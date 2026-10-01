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
//
// A fourth, lab, is the notebook columns' (M3/P1 design 3.11): priv, a
// private notebook of an admin, an editor, a guest reader and an ended
// member; team, open to the workspace as editor, and wiki, as viewer, both
// of the workspace's admin outside priv, team with a guest reader; gone-nb,
// deleted, of the deleted notebook's column. other has other-nb, whose
// admin is the column outside lab: a role read in the wrong workspace or
// notebook lets it into priv. orphan, private, is ownerless (M3/P3 design
// 3.7): its admin, the ended member's column, left it, its editor stays;
// lab has one audit event, gone-nb deleted ownerless.

// matrixWorkspace is a seeded workspace and the column that is its admin.
type matrixWorkspace struct {
	slug  string
	admin caller
}

func matrixWorkspaces() []matrixWorkspace {
	return []matrixWorkspace{{"acme", callerAdmin}, {"gone", callerDeleted}, {"other", callerNever}, {"lab", callerOutsideAdmin}}
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
		{"other", callerOutsideWorkspace, shared.WorkspaceMember},
		{"lab", callerNotebookAdmin, shared.WorkspaceMember},
		{"lab", callerNotebookEditor, shared.WorkspaceMember},
		{"lab", callerNotebookReader, shared.WorkspaceGuest},
		{"lab", callerOutsideAdmin, shared.WorkspaceAdmin},
		{"lab", callerOutsideMember, shared.WorkspaceMember},
		{"lab", callerDefaultEditor, shared.WorkspaceMember},
		{"lab", callerDefaultReader, shared.WorkspaceAdmin},
		{"lab", callerOutsideGuest, shared.WorkspaceGuest},
		{"lab", callerNotebookEnded, shared.WorkspaceMember},
		{"lab", callerNotebookDeleted, shared.WorkspaceMember},
		{"lab", callerGuestReaderOfOpen, shared.WorkspaceGuest},
		{"lab", callerOwnerlessMember, shared.WorkspaceMember},
	}
}

// matrixNotebook is a seeded notebook, by its name.
type matrixNotebook struct {
	name    string
	slug    string
	access  shared.WorkspaceAccess
	deleted bool // with its members, through SQL
}

func matrixNotebooks() []matrixNotebook {
	return []matrixNotebook{
		{"priv", "lab", shared.AccessNone, false},
		{"team", "lab", shared.AccessEditor, false},
		{"wiki", "lab", shared.AccessViewer, false},
		{"gone-nb", "lab", shared.AccessEditor, true},
		{"other-nb", "other", shared.AccessNone, false},
		{"orphan", "lab", shared.AccessNone, false},
	}
}

// matrixNotebookMember is a seeded membership of a notebook.
type matrixNotebookMember struct {
	notebook string
	c        caller
	role     shared.NotebookRole
	ended    bool
}

func matrixNotebookMembers() []matrixNotebookMember {
	return []matrixNotebookMember{
		{"priv", callerNotebookAdmin, shared.NotebookAdmin, false},
		{"priv", callerNotebookEditor, shared.NotebookEditor, false},
		{"priv", callerNotebookReader, shared.NotebookReader, false},
		{"priv", callerNotebookEnded, shared.NotebookReader, true},
		{"team", callerOutsideAdmin, shared.NotebookAdmin, false},
		{"team", callerGuestReaderOfOpen, shared.NotebookReader, false},
		{"wiki", callerOutsideAdmin, shared.NotebookAdmin, false},
		{"gone-nb", callerNotebookDeleted, shared.NotebookAdmin, false},
		{"other-nb", callerOutsideWorkspace, shared.NotebookAdmin, false},
		{"orphan", callerOwnerlessMember, shared.NotebookEditor, false},
		{"orphan", callerNotebookEnded, shared.NotebookAdmin, true},
	}
}

// matrixOwnerless is the ownerless notebook and its former owner.
func matrixOwnerless() (notebook string, formerOwner caller) { return "orphan", callerNotebookEnded }

// matrixAuditEvent is lab's audit event: gone-nb, deleted by lab's admin
// while ownerless of its admin.
func matrixAuditEvent() (notebook string, formerOwner, actor caller) {
	return "gone-nb", callerNotebookDeleted, callerOutsideAdmin
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
	t               testing.TB
	workspaces      map[string]uuid.UUID // by slug
	memberships     map[string]uuid.UUID // by slug/caller
	invitations     map[string]uuid.UUID // by slug
	notebooks       map[string]uuid.UUID // by name
	notebookMembers map[string]uuid.UUID // by notebook/caller
	// accounts are the columns' account ids, which registering them through
	// the API gives: prepareMatrix fills the map, so they are known to the
	// rows' requests, not to the coverage test, which reads the paths alone.
	accounts map[caller]uuid.UUID
}

func newSeeded() seeded {
	s := seeded{workspaces: map[string]uuid.UUID{}, memberships: map[string]uuid.UUID{}, invitations: map[string]uuid.UUID{},
		notebooks: map[string]uuid.UUID{}, notebookMembers: map[string]uuid.UUID{}, accounts: map[caller]uuid.UUID{}}
	for _, n := range matrixNotebooks() {
		s.notebooks[n.name] = uuid.NewV7()
	}
	for _, m := range matrixNotebookMembers() {
		s.notebookMembers[m.notebook+"/"+string(m.c)] = uuid.NewV7()
	}
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

// notebook is the id of the notebook name.
func (s seeded) notebook(name string) uuid.UUID {
	id, ok := s.notebooks[name]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no notebook %s is seeded", name)
	}
	return id
}

// notebookMember is the id of c's membership of the notebook name, ended
// or not.
func (s seeded) notebookMember(name string, c caller) uuid.UUID {
	id, ok := s.notebookMembers[name+"/"+string(c)]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no membership of %s by %s is seeded", name, c)
	}
	return id
}

// workspaceOfRow is the slug of the workspace a seeded row's id is in.
func (s seeded) workspaceOfRow(id uuid.UUID) (string, bool) {
	for _, n := range matrixNotebooks() {
		if s.notebooks[n.name] == id {
			return n.slug, true
		}
	}
	for key, seededID := range s.notebookMembers {
		if seededID == id {
			name, _, _ := strings.Cut(key, "/")
			for _, n := range matrixNotebooks() {
				if n.name == name {
					return n.slug, true
				}
			}
		}
	}
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
// memberships, invitations and notebooks through SQL, with the ids newSeeded fixed
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
		for _, c := range allColumns() {
			d.tokens[c] = registerAccount(t, contract, base, emailOf(c)).AccessToken
		}
		pool := connect(t, d.url)
		for _, c := range allColumns() {
			var id uuid.UUID
			if err := pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", emailOf(c)).Scan(&id); err != nil {
				t.Fatal(err)
			}
			d.seeded.accounts[c] = id
		}
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
		for _, n := range matrixNotebooks() {
			exec("INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) "+
				"SELECT $1, w.id, $3, $4, w.created_by_id, w.created_by_id, $5, $5 FROM workspaces w WHERE w.slug = $2",
				d.seeded.notebooks[n.name], n.slug, n.name, string(n.access), now)
		}
		for _, m := range matrixNotebookMembers() {
			var ended *time.Time
			if m.ended {
				ended = &now
			}
			exec("INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, ended_at) "+
				"VALUES ($6, $1, "+account+", $3, "+account+", "+account+", $4, $4, $5)",
				d.seeded.notebooks[m.notebook], emailOf(m.c), string(m.role), now, ended, d.seeded.notebookMembers[m.notebook+"/"+string(m.c)])
		}
		for _, n := range matrixNotebooks() {
			if n.deleted {
				exec("UPDATE notebooks SET deleted_at = $2 WHERE id = $1", d.seeded.notebooks[n.name], now)
				exec("UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", d.seeded.notebooks[n.name], now)
			}
		}
		orphan, formerOwner := matrixOwnerless()
		exec("UPDATE notebooks SET ownerless_since = $3, former_owner_id = "+account+" WHERE id = $1",
			d.seeded.notebooks[orphan], emailOf(formerOwner), now)
		gone, goneOwner, actor := matrixAuditEvent()
		exec("INSERT INTO notebook_audit_events (id, workspace_id, notebook_id, notebook_name, action, former_owner_id, created_by_id, "+
			"updated_by_id, created_at, updated_at) SELECT $1, n.workspace_id, n.id, n.name, 'deleted', "+account+", a.id, a.id, $4, $4 "+
			"FROM notebooks n, users a WHERE n.id = $3 AND a.email = $5",
			uuid.NewV7(), emailOf(goneOwner), d.seeded.notebooks[gone], now, emailOf(actor))
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
	if len(d.tokens) != len(allColumns()) {
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
