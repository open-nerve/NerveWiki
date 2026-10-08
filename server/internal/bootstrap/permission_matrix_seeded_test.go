package bootstrap

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
//
// Each notebook column has an edit session of its own, alive, whatever its
// role (M4/P4 design 3.9), on a page of its own at its notebook's root, its
// draft: what the session's operations answer is decided when they are
// called. Each notebook of the columns has one more draft, with a session of
// acme's admin, an account outside lab: someone else's for every column. So
// a page has at most one alive session, as the edit lock allows (M5/P1), and
// the pages the other rows target have none. gone-nb's are deleted with it.

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

// matrixPage is a seeded page, by its title: under parent, a page before
// it, or at its notebook's root, its siblings in the list's order. A
// deleted notebook's are deleted with it.
type matrixPage struct {
	name     string
	notebook string
	parent   string
}

func matrixPages() []matrixPage {
	pages := []matrixPage{
		{"priv-root", "priv", ""},
		{"priv-child", "priv", "priv-root"},
		{"team-page", "team", ""},
		{"wiki-page", "wiki", ""},
		{"gone-nb-page", "gone-nb", ""},
		{"orphan-page", "orphan", ""},
	}
	for _, c := range notebookColumns() {
		pages = append(pages, matrixPage{draftOf(c), notebookOf(c), ""})
	}
	for _, nb := range sessionNotebooks() {
		pages = append(pages, matrixPage{othersDraftIn(nb), nb, ""}, matrixPage{tasksIn(nb), nb, ""})
	}
	return pages
}

// tasksIn is a notebook's page of task items, its content matrixTasks;
// every other seeded page's is empty.
func tasksIn(notebook string) string {
	return notebook + "-tasks"
}

// matrixTasks is the content of the pages of task items: one open at 3.
const matrixTasks = "- [ ] a\n"

// draftOf is the page of a notebook column's own edit session.
func draftOf(c caller) string {
	return "draft-" + strings.ReplaceAll(string(c), " ", "-")
}

// othersDraftIn is the page of someone else's edit session in a notebook.
func othersDraftIn(notebook string) string {
	return notebook + "-others-draft"
}

// sessionNotebooks are the notebooks of the notebook columns, once each.
func sessionNotebooks() []string {
	var out []string
	for _, c := range notebookColumns() {
		if nb := notebookOf(c); !slices.Contains(out, nb) {
			out = append(out, nb)
		}
	}
	return out
}

// matrixSession is a seeded edit session: owner's, of page.
type matrixSession struct {
	page  string
	owner caller
}

// someoneElse is the owner of the session someone else's for every column.
const someoneElse = callerAdmin

func matrixSessions() []matrixSession {
	var out []matrixSession
	for _, c := range notebookColumns() {
		out = append(out, matrixSession{draftOf(c), c})
	}
	for _, nb := range sessionNotebooks() {
		out = append(out, matrixSession{othersDraftIn(nb), someoneElse})
	}
	return out
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
	pages           map[string]uuid.UUID // by title
	assets          map[string]uuid.UUID // by name
	sessions        map[string]uuid.UUID // by page/owner
	// accounts are the columns' account ids, which registering them through
	// the API gives: prepareMatrix fills the map, so they are known to the
	// rows' requests, not to the coverage test, which reads the paths alone.
	accounts map[caller]uuid.UUID
}

func newSeeded() seeded {
	s := seeded{workspaces: map[string]uuid.UUID{}, memberships: map[string]uuid.UUID{}, invitations: map[string]uuid.UUID{},
		notebooks: map[string]uuid.UUID{}, notebookMembers: map[string]uuid.UUID{}, pages: map[string]uuid.UUID{},
		assets: map[string]uuid.UUID{}, sessions: map[string]uuid.UUID{}, accounts: map[caller]uuid.UUID{}}
	for _, p := range matrixPages() {
		s.pages[p.name] = uuid.NewV7()
	}
	for _, a := range matrixAssets() {
		s.assets[a.name] = uuid.NewV7()
	}
	for _, n := range matrixNotebooks() {
		s.notebooks[n.name] = uuid.NewV7()
	}
	for _, e := range matrixSessions() {
		s.sessions[e.page+"/"+string(e.owner)] = uuid.NewV7()
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

// page is the id of the page name.
func (s seeded) page(name string) uuid.UUID {
	id, ok := s.pages[name]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no page %s is seeded", name)
	}
	return id
}

// asset is the id of the attachment name's node.
func (s seeded) asset(name string) uuid.UUID {
	id, ok := s.assets[name]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no attachment %s is seeded", name)
	}
	return id
}

// session is the id of owner's edit session of the page name.
func (s seeded) session(name string, owner caller) uuid.UUID {
	id, ok := s.sessions[name+"/"+string(owner)]
	if !ok {
		s.t.Helper()
		s.t.Fatalf("no session of %s by %s is seeded", name, owner)
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
	for _, p := range slices.Concat(matrixPages(), matrixAssets()) {
		if s.pages[p.name] == id || s.assets[p.name] == id {
			return s.workspaceOfRow(s.notebooks[p.notebook])
		}
	}
	for key, seededID := range s.sessions {
		if seededID == id {
			name, _, _ := strings.Cut(key, "/")
			return s.workspaceOfRow(s.pages[name])
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

// seededPageHistory writes, beside the page node $1 and its content
// seeded by SQL, what its creation through the API would: a changeset of
// its own, its item and its first version, at the node's time and in its
// state, and, for a page not deleted, its index at that version, so that
// the pages' invariant (checkPages) holds on the seeded data. The seeded
// contents have no frontmatter and no link.
const seededPageHistory = `WITH n AS (SELECT * FROM nodes WHERE id = $1), c AS (SELECT * FROM page_contents WHERE node_id = $1),
	x AS (INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
		SELECT id, notebook_id, 1, 1, true FROM n WHERE deleted_at IS NULL),
	s AS (INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, deleted_at)
		SELECT gen_random_uuid(), notebook_id, 'edit', 'web', created_by_id, created_at, created_at, deleted_at FROM n RETURNING id),
	i AS (INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order,
			created_at, updated_at, deleted_at)
		SELECT gen_random_uuid(), s.id, n.id, n.parent_id, n.name, n.sort_order, n.created_at, n.created_at, n.deleted_at FROM n, s)
	INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at, deleted_at)
	SELECT gen_random_uuid(), s.id, n.id, 1, c.content, c.content_hash, c.byte_size, n.created_at, n.created_at, n.deleted_at
	FROM n, s, c`

// prepareMatrix fills a database for the matrix: an account for each
// column, registered through the API for its token; the workspaces,
// memberships, invitations, notebooks, pages and edit sessions through SQL, with the ids newSeeded fixed
// (the coverage check needs them before any database: members joining by
// invitation would get theirs from the server, M2/P3 design 3.10); then,
// through the API, acme's admin removes the ended member, and gone's admin
// deletes it. Everything that connected to
// the database is closed when it returns, so that it can be copied. The
// attachments' rows are seeded through SQL too, without their files. A -run
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
		for i, p := range matrixPages() {
			var parent *uuid.UUID
			if p.parent != "" {
				id := d.seeded.pages[p.parent]
				parent = &id
			}
			exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, "+
				"created_at, updated_at) SELECT $1, n.id, $3, 'page', $4, $4, $6, n.created_by_id, n.created_by_id, $5, $5 "+
				"FROM notebooks n WHERE n.id = $2", d.seeded.pages[p.name], d.seeded.notebooks[p.notebook], parent, p.name, now, i)
			content := ""
			if p.name == tasksIn(p.notebook) {
				content = matrixTasks
			}
			exec("INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) "+
				"SELECT id, $3, 1, sha256(convert_to($3, 'UTF8')), octet_length($3), created_by_id, $2 FROM nodes WHERE id = $1",
				d.seeded.pages[p.name], now, content)
			exec(seededPageHistory, d.seeded.pages[p.name])
		}
		for i, a := range matrixAssets() {
			exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, "+
				"created_at, updated_at) SELECT $1, n.id, $3, 'asset', $4, $4, $6, n.created_by_id, n.created_by_id, $5, $5 "+
				"FROM notebooks n WHERE n.id = $2", d.seeded.assets[a.name], d.seeded.notebooks[a.notebook], d.seeded.pages[a.parent], a.name,
				now, len(matrixPages())+i)
			exec("INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at) "+
				"SELECT gen_random_uuid(), id, notebook_id, 'image/png', $3, sha256('abc'), created_by_id, $2 FROM nodes WHERE id = $1",
				d.seeded.assets[a.name], now, matrixAssetBytes)
		}
		for _, e := range matrixSessions() {
			exec("INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at) "+
				"SELECT $1, n.id, n.notebook_id, "+account+", 'web', $4, $5 FROM nodes n WHERE n.id = $3",
				d.seeded.sessions[e.page+"/"+string(e.owner)], emailOf(e.owner), d.seeded.pages[e.page], now, now.Add(time.Hour))
		}
		for _, n := range matrixNotebooks() {
			if n.deleted {
				exec("UPDATE notebooks SET deleted_at = $2 WHERE id = $1", d.seeded.notebooks[n.name], now)
				exec("UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", d.seeded.notebooks[n.name], now)
				exec("UPDATE nodes SET deleted_at = $2 WHERE notebook_id = $1", d.seeded.notebooks[n.name], now)
				for _, table := range []string{"page_contents", "page_revisions", "changeset_items"} {
					exec("UPDATE "+table+" SET deleted_at = $2 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)",
						d.seeded.notebooks[n.name], now)
				}
				exec("UPDATE changesets SET deleted_at = $2 WHERE notebook_id = $1", d.seeded.notebooks[n.name], now)
				exec("UPDATE asset_blobs SET deleted_at = $2 WHERE notebook_id = $1", d.seeded.notebooks[n.name], now)
				exec("DELETE FROM edit_sessions WHERE notebook_id = $1", d.seeded.notebooks[n.name])
				exec("DELETE FROM indexed_pages WHERE notebook_id = $1", d.seeded.notebooks[n.name])
			}
		}
		// orphan is ownerless of an account still active in lab, a state no
		// end makes (its end would have ended its membership of lab): the
		// rows read the notebook's columns alone.
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
