package bootstrap

import (
	"bytes"
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
	"uuid"

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
// ready; the sessions' and the edit sessions' cleanups and the purge run
// as the jobs start; every statement of the account's API and of the
// commands goes through, nervewiki reindex's too, which rekeys the nodes
// and rebuilds the link index; an export runs, enqueued with its job's row
// and read in its snapshot, and its archive downloads; an import runs, its
// archive uploaded, its nodes written, the statistics of the tables it
// fills refreshed; River's daily reindex goes through, as River runs it,
// index by index. Nothing logs a permission denied, the shutdown included.
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
	if _, err := roles.owner.Exec(ctx, `INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at)
		SELECT gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), id, 'web', now() - interval '1 hour', now() - interval '1 minute'
		FROM users`); err != nil {
		t.Fatal(err)
	}
	if _, err := roles.owner.Exec(ctx, `WITH w AS (
			INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), 'gone', 'Gone', id, id, now(), now(), now() - interval '61 days' FROM users RETURNING id, created_by_id, deleted_at
		), m AS (
			INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, created_by_id, 'admin', created_by_id, created_by_id, now(), now(), deleted_at FROM w
		)
		INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
		SELECT gen_random_uuid(), id, 'dana@example.com', 'member', created_by_id, created_by_id, now(), now(), deleted_at FROM w`); err != nil {
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
	const editCleanups = "SELECT count(*) FROM river_job WHERE kind = 'page.cleanup_expired_edit_sessions' AND state = 'completed'"
	for deadline := time.Now().Add(15 * time.Second); count(t, roles.owner, editCleanups) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the edit sessions' cleanup did not complete as %s; logs:\n%s", roles.serverName, logs.String())
		}
	}
	if n := count(t, roles.owner, "SELECT count(*) FROM edit_sessions"); n != 0 {
		t.Errorf("%d edit sessions after the cleanup, want the expired one deleted", n)
	}
	const purges = "SELECT count(*) FROM river_job WHERE kind = 'platform.purge_soft_deleted' AND state = 'completed'"
	for deadline := time.Now().Add(15 * time.Second); count(t, roles.owner, purges) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not complete as %s; logs:\n%s", roles.serverName, logs.String())
		}
	}
	if n := count(t, roles.owner, "SELECT (SELECT count(*) FROM workspaces) + (SELECT count(*) FROM workspace_members) + "+
		"(SELECT count(*) FROM workspace_invitations)"); n != 0 {
		t.Errorf("%d rows of the workspace deleted 61 days ago after the purge, want none", n)
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
	for _, cmd := range []WorkspaceCommand{CreateWorkspace("acme", "Acme", "admin@example.com"), ReactivateMember("acme", "admin@example.com")} {
		if out, logs, err := runWorkspaces(t, roles.serverURL, cmd); err != nil {
			t.Errorf("a workspaces command as %s = %q, %v: %s", roles.serverName, out, err, logs)
		}
	}
	if _, err := roles.owner.Exec(ctx, `WITH n AS (
			INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
			SELECT gen_random_uuid(), w.id, 'Notes', 'editor', w.created_by_id, w.created_by_id, now(), now() FROM workspaces w WHERE w.slug = 'acme'
			RETURNING id, created_by_id
		), p AS (
			INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			SELECT gen_random_uuid(), id, 'page', 'Note', 'old', 0, created_by_id, created_by_id, now(), now() FROM n
			RETURNING id, created_by_id
		)
		INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, c, 1, sha256(convert_to(c, 'UTF8')), octet_length(c), created_by_id, now()
		FROM p, (SELECT E'---\naliases: [N]\ntags: [t]\n---\n[[Note]]' AS c) content`); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Reindex(ctx, testConfig(t, roles.serverURL, false), &stderr, &stdout, uuid.Nil()); err != nil ||
		count(t, roles.owner, `SELECT (SELECT count(*) FROM page_links WHERE resolved_id IS NOT NULL) + (SELECT count(*) FROM page_aliases)
			+ (SELECT count(*) FROM page_tags) + (SELECT count(*) FROM page_properties)`) != 5 {
		t.Errorf("reindex as %s = %q, %v: %s", roles.serverName, stdout.String(), err, stderr.String())
	}

	var admin authTokens
	if err := json.Unmarshal(call(t, http.MethodPost, base+"/api/v0/auth/login", "",
		`{"email":"admin@example.com","password":"Tr0ub4dor&3"}`, http.StatusOK), &admin); err != nil {
		t.Fatal(err)
	}
	var notes string
	if err := roles.owner.QueryRow(ctx, "SELECT id::text FROM notebooks WHERE name = 'Notes'").Scan(&notes); err != nil {
		t.Fatal(err)
	}
	var export transferJob
	if err := json.Unmarshal(call(t, http.MethodPost, base+"/api/v0/notebooks/"+notes+"/exports", admin.AccessToken, `{}`, http.StatusAccepted),
		&export); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(15 * time.Second); export.State == "queued" || export.State == "running"; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the export is still %s as %s; logs:\n%s", export.State, roles.serverName, logs.String())
		}
		if err := json.Unmarshal(call(t, http.MethodGet, base+"/api/v0/transfer-jobs/"+export.ID, admin.AccessToken, "", http.StatusOK), &export); err != nil {
			t.Fatal(err)
		}
	}
	if export.State != "succeeded" || export.Download == nil {
		t.Fatalf("the export as %s = %+v, want succeeded; logs:\n%s", roles.serverName, export, logs.String())
	}
	call(t, http.MethodGet, base+export.Download.URL, "", "", http.StatusOK)

	contentType, body := importBody(t, "", "vault.zip", vaultZip(t, vaultFile{"Imported.md", "[[Note]] ![[pic.png]]"},
		vaultFile{"Imported/pic.png", pngHead}))
	req := newRequest(t, http.MethodPost, base+"/api/v0/notebooks/"+notes+"/imports", admin.AccessToken, []byte(body))
	req.Header.Set("Content-Type", contentType)
	res, answer := sendRequest(t, req)
	var imported transferJob
	if res.StatusCode != http.StatusAccepted || json.Unmarshal(answer, &imported) != nil {
		t.Fatalf("an import as %s = %d %s", roles.serverName, res.StatusCode, answer)
	}
	for deadline := time.Now().Add(15 * time.Second); imported.State == "queued" || imported.State == "running"; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the import is still %s as %s; logs:\n%s", imported.State, roles.serverName, logs.String())
		}
		if err := json.Unmarshal(call(t, http.MethodGet, base+"/api/v0/transfer-jobs/"+imported.ID, admin.AccessToken, "", http.StatusOK),
			&imported); err != nil {
			t.Fatal(err)
		}
	}
	if imported.State != "succeeded" || imported.Report == nil || imported.Report.Counts["pages"] != 1 || imported.Report.Counts["attachments"] != 1 {
		t.Fatalf("the import as %s = %+v, want succeeded; logs:\n%s", roles.serverName, imported, logs.String())
	}
	if got := analyzed(t, roles.owner, importAnalyzes()); len(got) != len(importAnalyzes()) {
		t.Errorf("analyzed as %s %q, want %q", roles.serverName, got, importAnalyzes())
	}

	for _, index := range river.ReindexerIndexNamesDefault() {
		if _, err := roles.server.Exec(ctx, "REINDEX INDEX CONCURRENTLY "+pgx.Identifier{index}.Sanitize()); err != nil {
			t.Errorf("REINDEX INDEX CONCURRENTLY %s as %s: %v", index, roles.serverName, err)
		}
	}
}

// importAnalyzes are the tables an import analyzes, each module its own
// (M7/P6 design 3.6), which take MAINTAIN.
func importAnalyzes() []string {
	return []string{
		"nodes", "page_contents", "page_revisions", "changesets", "changeset_items",
		"indexed_pages", "page_links", "page_tags", "page_properties", "page_aliases", "asset_blobs",
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
	// The tables an import analyzes (M7/P6 design 3.6).
	analyzed := map[string]bool{}
	for _, table := range importAnalyzes() {
		analyzed[table] = true
	}
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
		case name == "river_job" || analyzed[name]:
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

// A role without River's grants cannot start the jobs: River's start reads
// river_queue, fails, and serve stops with that error rather than serve
// without its jobs (M1/P4 design 3.4); HTTP stops with it. README "部署"
// says which missing grant does this and which only log.
func TestServeStopsWhenTheJobsCannotStart(t *testing.T) {
	roles := newSplitRoles(t)
	if _, err := roles.owner.Exec(context.Background(), "REVOKE ALL ON river_queue FROM nervewiki_runtime"); err != nil {
		t.Fatal(err)
	}
	a := buildApp(t, testConfig(t, roles.serverURL, false), migrations.FS())
	done := make(chan error, 1)
	go func() { done <- a.run(context.Background()) }()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "start the jobs: ") || !strings.Contains(err.Error(), "permission denied for table river_queue") {
			t.Errorf("run() = %v, want the jobs' start refused on river_queue", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve still runs 15s later, want it stopped by the jobs' failure")
	}
}
