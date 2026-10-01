package bootstrap

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// What the matrix prepares. Three workspaces: acme, which the columns
// target; gone, deleted by its admin, the deleted column's caller, with a
// member and an invitation the deleted column's rows aim at; and other,
// whose admin is the column that was never a member of acme and where the
// ended member of acme is still a member: a role read in the wrong
// workspace lets either into acme. acme and gone each have an invitation
// pending to an address of no account.

// matrixWorkspace is a seeded workspace and the column that is its admin.
type matrixWorkspace struct {
	slug  string
	admin caller
}

func matrixWorkspaces() []matrixWorkspace {
	return []matrixWorkspace{{"acme", callerAdmin}, {"gone", callerDeleted}, {"other", callerNever}}
}

// matrixMembership is a seeded membership. The ended member's of acme is
// ended, and gone's are deleted, by prepareMatrix through the API.
type matrixMembership struct {
	slug string
	c    caller
	role shared.WorkspaceRole
}

func matrixMemberships() []matrixMembership {
	return []matrixMembership{
		{"acme", callerAdmin, shared.WorkspaceAdmin},
		{"acme", callerMember, shared.WorkspaceMember},
		{"acme", callerGuest, shared.WorkspaceGuest},
		{"acme", callerEnded, shared.WorkspaceMember},
		{"gone", callerDeleted, shared.WorkspaceAdmin},
		{"gone", callerMember, shared.WorkspaceMember},
		{"other", callerNever, shared.WorkspaceAdmin},
		{"other", callerEnded, shared.WorkspaceMember},
	}
}

// matrixInvitee is the address acme's and gone's invitations are sent to.
const matrixInvitee = "invitee@example.com"

// invitedWorkspaces are the workspaces with an invitation seeded.
func invitedWorkspaces() []string { return []string{"acme", "gone"} }

// emailOf is a column's account.
func emailOf(c caller) string {
	return strings.ReplaceAll(string(c), " ", "-") + "@example.com"
}

// seeded are the ids of what prepareMatrix seeds, fixed before it runs:
// from P2 on, the rows that aim at a row by its id build their requests
// from them, and the coverage test checks what they aim at without a
// database.
type seeded struct {
	t           testing.TB
	workspaces  map[string]uuid.UUID // by slug
	memberships map[string]uuid.UUID // by slug/caller
	invitations map[string]uuid.UUID // by slug
}

func newSeeded() seeded {
	s := seeded{workspaces: map[string]uuid.UUID{}, memberships: map[string]uuid.UUID{}, invitations: map[string]uuid.UUID{}}
	for _, w := range matrixWorkspaces() {
		s.workspaces[w.slug] = uuid.NewV7()
	}
	for _, m := range matrixMemberships() {
		s.memberships[m.slug+"/"+string(m.c)] = uuid.NewV7()
	}
	for _, slug := range invitedWorkspaces() {
		s.invitations[slug] = uuid.NewV7()
	}
	return s
}

// in is s failing t when a row asks for what is not seeded.
func (s seeded) in(t testing.TB) seeded {
	s.t = t
	return s
}

// membership is the id of c's membership of slug.
func (s seeded) membership(slug string, c caller) uuid.UUID {
	id, ok := s.memberships[slug+"/"+string(c)]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no membership of %s by %s is seeded", slug, c)
	}
	return id
}

// adminMembership is the id of the membership of slug's admin.
func (s seeded) adminMembership(slug string) uuid.UUID {
	for _, w := range matrixWorkspaces() {
		if w.slug == slug {
			return s.membership(slug, w.admin)
		}
	}
	s.t.Helper()
	s.t.Fatalf("no workspace %s is seeded", slug)
	return uuid.UUID{}
}

// invitation is the id of slug's invitation.
func (s seeded) invitation(slug string) uuid.UUID {
	id, ok := s.invitations[slug]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no invitation of %s is seeded", slug)
	}
	return id
}

// workspaceOfRow is the slug of the workspace a seeded row's id is in.
func (s seeded) workspaceOfRow(id uuid.UUID) (string, bool) {
	for key, seededID := range s.memberships {
		if seededID == id {
			slug, _, _ := strings.Cut(key, "/")
			return slug, true
		}
	}
	for slug, seededID := range s.invitations {
		if seededID == id {
			return slug, true
		}
	}
	return "", false
}

// matrixData is the prepared database, the signing key every app on a copy
// of it shares, each column's access token, and the seeded ids.
type matrixData struct {
	url     string
	keyFile string
	tokens  map[caller]string
	seeded  seeded
}

// config is the configuration of an app on the database at url, with
// change applied when it is not nil.
func (d matrixData) config(t *testing.T, url string, change func(*config.Config)) config.Config {
	t.Helper()
	cfg := testConfig(t, url, false)
	cfg.Auth.JWT.PrivateKeyFile = d.keyFile
	if change != nil {
		change(&cfg)
	}
	return cfg
}

// prepareMatrix fills a database for the matrix: an account for each
// column, registered through the API for its token; the workspaces,
// memberships and invitations through SQL, with the ids newSeeded fixed
// (the coverage check needs them before any database: members joining by
// invitation would get theirs from the server, M2/P3 design 3.10); then,
// through the API, acme's admin removes the ended member, and gone's admin
// deletes it. Everything that connected to
// the database is closed when it returns, so that it can be copied. A -run
// that leaves out prepare fails here, not with a 401 in every cell.
func prepareMatrix(t *testing.T) matrixData {
	t.Helper()
	d := matrixData{url: pgtest.NewDatabase(t), keyFile: signingKeyFile(t), tokens: map[caller]string{}, seeded: newSeeded()}
	prepared := t.Run("prepare", func(t *testing.T) {
		contract := apitest.Load(t)
		base := startApp(t, d.config(t, d.url, nil), migrations.FS())
		for _, c := range workspaceColumns() {
			d.tokens[c] = registerAccount(t, contract, base, emailOf(c)).AccessToken
		}
		pool := connect(t, d.url)
		exec := func(sql string, args ...any) {
			t.Helper()
			if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
				t.Fatalf("%s: %v", sql, err)
			}
		}
		account := "(SELECT id FROM users WHERE email = $2)"
		now := time.Now()
		for _, w := range matrixWorkspaces() {
			exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) "+
				"VALUES ($1, $3, $3, "+account+", "+account+", $4, $4)",
				d.seeded.workspaces[w.slug], emailOf(w.admin), w.slug, now)
		}
		for _, m := range matrixMemberships() {
			exec("INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
				"VALUES ($1, $3, "+account+", $4, "+account+", "+account+", $5, $5)",
				d.seeded.memberships[m.slug+"/"+string(m.c)], emailOf(m.c), d.seeded.workspaces[m.slug], string(m.role), now)
		}
		for _, slug := range invitedWorkspaces() {
			exec("INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at) "+
				"SELECT $1, w.id, $3, 'member', w.created_by_id, w.created_by_id, $4, $4 FROM workspaces w WHERE w.slug = $2",
				d.seeded.invitations[slug], slug, matrixInvitee, now)
		}
		for _, end := range []struct {
			by   caller
			path string
		}{
			{callerAdmin, "/api/v0/workspace-members/" + d.seeded.memberships["acme/"+string(callerEnded)].String()},
			{callerDeleted, "/api/v0/workspaces/gone"},
		} {
			if status, answer := ask(t, contract, http.MethodDelete, base+end.path, d.tokens[end.by], ""); status != http.StatusNoContent {
				t.Fatalf("DELETE %s = %d %s, want 204", end.path, status, answer)
			}
		}
	})
	if !prepared {
		t.FailNow()
	}
	if len(d.tokens) != len(workspaceColumns()) {
		t.Fatal("prepare did not run: a -run of some cells must select prepare too, e.g. -run 'TestPermissionMatrix/(prepare|getWorkspace)'")
	}
	return d
}

// signingKeyFile writes a new Ed25519 key in the format of openssl genpkey
// and returns its path: every app of the matrix signs and verifies the
// columns' tokens with it.
func signingKeyFile(t *testing.T) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
