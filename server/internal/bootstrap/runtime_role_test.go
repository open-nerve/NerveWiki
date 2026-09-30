package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The role nervewiki serves with when another role owns the tables and runs
// the migrations (M1/P4 design 3.9, README "部署"): a login role in the group
// role nervewiki_runtime, which deploy/runtime-grants.sql grants to. These
// tests run nervewiki on such a role, and hold the file to every relation
// and function of the schema.

// splitRoles is a database whose tables belong to an owner role that ran the
// migrations and then deploy/runtime-grants.sql, and a login role in
// nervewiki_runtime to serve with.
type splitRoles struct {
	owner      *pgxpool.Pool // the owner of the tables
	server     *pgxpool.Pool // the login role nervewiki serves with
	serverName string
	serverURL  string
}

func newSplitRoles(t *testing.T) splitRoles {
	t.Helper()
	ctx := context.Background()
	adminURL := pgtest.NewEmptyDatabase(t)
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	db := strings.TrimPrefix(u.Path, "/")
	// The roles belong to the cluster, which the tests of this binary share:
	// named after the database, which is new; nervewiki_runtime once. The
	// owner owns the database, as pg_trgm, a trusted extension, requires.
	ownerName, serverName := db+"_owner", db+"_server"
	admin := connect(t, adminURL)
	for _, sql := range []string{
		"CREATE ROLE " + pgx.Identifier{ownerName}.Sanitize() + " LOGIN PASSWORD 'owner'",
		"ALTER DATABASE " + pgx.Identifier{db}.Sanitize() + " OWNER TO " + pgx.Identifier{ownerName}.Sanitize(),
		"DO $$ BEGIN CREATE ROLE nervewiki_runtime NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$",
		"CREATE ROLE " + pgx.Identifier{serverName}.Sanitize() + " LOGIN PASSWORD 'server' IN ROLE nervewiki_runtime",
	} {
		if _, err := admin.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	as := func(name, password string) string {
		v := *u
		v.User = url.UserPassword(name, password)
		return v.String()
	}
	owner := connect(t, as(ownerName, "owner"))
	m, err := postgres.NewMigrator(owner, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("migrate as the owner: %v", err)
	}
	grants, err := os.ReadFile(grantsFile())
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // as after every migration: again changes nothing
		if _, err := owner.Exec(ctx, string(grants)); err != nil {
			t.Fatalf("run deploy/runtime-grants.sql as the owner: %v", err)
		}
	}
	serverURL := as(serverName, "server")
	return splitRoles{owner: owner, server: connect(t, serverURL), serverName: serverName, serverURL: serverURL}
}

// grantsFile is deploy/runtime-grants.sql, found from this file, three
// directories below the repository root (tests run without -trimpath).
func grantsFile() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "deploy", "runtime-grants.sql")
}

// call sends a JSON request and fails the test unless it answers want; it
// returns the body.
func call(t *testing.T, method, url, token, body string, want int) []byte {
	t.Helper()
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	res, got := sendRequest(t, newRequest(t, method, url, token, b))
	if res.StatusCode != want {
		t.Fatalf("%s %s = %d %s, want %d", method, url, res.StatusCode, got, want)
	}
	return got
}

// nervewiki serves on the role and its administrator's commands run on it:
// ready; the session cleanup runs as the jobs start; every statement of the
// account's API and of the commands goes through; River's daily reindex
// goes through, as River runs it, index by index. Nothing logs a permission
// denied, the shutdown included.
func TestTheRuntimeRoleServesWithTheGrantsFile(t *testing.T) {
	roles := newSplitRoles(t)
	ctx := context.Background()
	if out, logs, err := runUsers(t, roles.serverURL, CreateUser("admin@example.com", "Tr0ub4dor&3")); err != nil {
		t.Fatalf("create as %s = %q, %v: %s", roles.serverName, out, err, logs)
	}
	if _, err := roles.owner.Exec(ctx, `INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, updated_at)
		SELECT gen_random_uuid(), id, sha256('expired'), now() - interval '1 minute', now() - interval '1 hour', now() - interval '1 hour'
		FROM users`); err != nil {
		t.Fatal(err)
	}
	var logs syncBuffer
	// Registered before the app's: it runs once the app has stopped.
	t.Cleanup(func() {
		if all := logs.String(); !strings.Contains(all, `msg="jobs started"`) || strings.Contains(all, "permission denied") {
			t.Errorf("logs want the jobs started and no permission denied:\n%s", all)
		}
	})
	base := runApp(t, buildAppWith(t, testConfig(t, roles.serverURL, false), migrations.FS(), slog.New(slog.NewTextHandler(&logs, nil))))

	if status := getStatus(t, base+"/readyz"); status != http.StatusOK {
		t.Errorf("/readyz = %d, want 200", status)
	}
	const cleanups = "SELECT count(*) FROM river_job WHERE kind = 'identity.cleanup_expired_sessions' AND state = 'completed'"
	for deadline := time.Now().Add(15 * time.Second); count(t, roles.owner, cleanups) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the session cleanup did not complete as %s; logs:\n%s", roles.serverName, logs.String())
		}
	}
	if n := count(t, roles.owner, "SELECT count(*) FROM auth_sessions"); n != 0 {
		t.Errorf("%d sessions after the cleanup, want the expired one deleted", n)
	}

	contract := apitest.Load(t)
	session := registerAccount(t, contract, base, "alice@example.com")
	var signedIn authTokens
	if err := json.Unmarshal(call(t, http.MethodPost, base+"/api/v0/auth/login", "",
		`{"email":"alice@example.com","password":"Tr0ub4dor&3"}`, http.StatusOK), &signedIn); err != nil {
		t.Fatal(err)
	}
	call(t, http.MethodPost, base+"/api/v0/auth/refresh", "", `{"refresh_token":"`+signedIn.RefreshToken+`"}`, http.StatusOK)
	token := createToken(t, contract, base, session.AccessToken)
	call(t, http.MethodGet, base+"/api/v0/me", token, "", http.StatusOK)
	call(t, http.MethodPost, base+"/api/v0/me/onboarding-steps", session.AccessToken, `{"step":"welcome"}`, http.StatusOK)
	call(t, http.MethodPost, base+"/api/v0/me/change-password", session.AccessToken,
		`{"current_password":"Tr0ub4dor&3","new_password":"N3w-Passw0rd!"}`, http.StatusNoContent)
	call(t, http.MethodPost, base+"/api/v0/me/deactivate", session.AccessToken, "", http.StatusNoContent)
	for _, cmd := range []UserCommand{
		ActivateUser("alice@example.com"),
		ResetPassword("alice@example.com", "Correct-Horse-9"),
		SetEmail("alice@example.com", "alice@example.org"),
		DeactivateUser("alice@example.org"),
	} {
		if out, logs, err := runUsers(t, roles.serverURL, cmd); err != nil {
			t.Errorf("a command as %s = %q, %v: %s", roles.serverName, out, err, logs)
		}
	}

	for _, index := range river.ReindexerIndexNamesDefault() {
		if _, err := roles.server.Exec(ctx, "REINDEX INDEX CONCURRENTLY "+pgx.Identifier{index}.Sanitize()); err != nil {
			t.Errorf("REINDEX INDEX CONCURRENTLY %s as %s: %v", index, roles.serverName, err)
		}
	}
}

// Every table, view, materialized view, sequence and function the
// migrations leave in the schema public has its grant, and no more: a
// migration that adds one fails here until deploy/runtime-grants.sql grants
// it. The runtime role reads and writes every table but goose's record,
// which it only reads; it reads the views; it uses the sequences of the
// tables it writes; it may reindex River's jobs; it runs the functions,
// River's river_job_state_in_bitmask among them, which it may through
// PUBLIC's default EXECUTE. Types are left to PUBLIC's default USAGE.
func TestTheGrantsFileCoversEveryRelationAndFunction(t *testing.T) {
	roles := newSplitRoles(t)
	rows, err := roles.owner.Query(context.Background(), `
		SELECT c.relname, c.relkind::text,
		       CASE WHEN c.relkind = 'S' THEN ARRAY[has_sequence_privilege($1, c.oid, 'USAGE'),
		                       has_sequence_privilege($1, c.oid, 'SELECT'), has_sequence_privilege($1, c.oid, 'UPDATE')]
		            ELSE ARRAY[has_table_privilege($1, c.oid, 'SELECT'), has_table_privilege($1, c.oid, 'INSERT'),
		                       has_table_privilege($1, c.oid, 'UPDATE'), has_table_privilege($1, c.oid, 'DELETE'),
		                       has_table_privilege($1, c.oid, 'TRUNCATE'), has_table_privilege($1, c.oid, 'REFERENCES'),
		                       has_table_privilege($1, c.oid, 'TRIGGER'), has_table_privilege($1, c.oid, 'MAINTAIN')] END
		  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'S')
		UNION ALL
		SELECT p.oid::regprocedure::text, 'f', ARRAY[has_function_privilege($1, p.oid, 'EXECUTE')]
		  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = 'public'
		 ORDER BY 1`, roles.serverName)
	if err != nil {
		t.Fatal(err)
	}
	privileges := map[string][]string{
		"S": {"USAGE", "SELECT", "UPDATE"},
		"f": {"EXECUTE"},
	}
	tablePrivileges := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"}
	dml := []string{"SELECT", "INSERT", "UPDATE", "DELETE"}
	seen := map[string]int{}
	for rows.Next() {
		var name, kind string
		var has []bool
		if err := rows.Scan(&name, &kind, &has); err != nil {
			t.Fatal(err)
		}
		seen[kind]++
		names, ok := privileges[kind]
		if !ok {
			names = tablePrivileges
		}
		var got, want []string
		for i, ok := range has {
			if ok {
				got = append(got, names[i])
			}
		}
		switch {
		case name == "goose_db_version_id_seq":
		case kind == "S":
			want = []string{"USAGE"}
		case kind == "f":
			want = []string{"EXECUTE"}
		case kind == "v", kind == "m", name == "goose_db_version":
			want = []string{"SELECT"}
		case name == "river_job":
			want = append(slices.Clone(dml), "MAINTAIN")
		default:
			want = dml
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %s has %v, want %v: deploy/runtime-grants.sql grants each of them", name, roles.serverName, got, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen["r"] == 0 || seen["S"] == 0 || seen["f"] == 0 {
		t.Fatalf("the schema public has %v tables (r), sequences (S) and functions (f), want some of each", seen)
	}
}
