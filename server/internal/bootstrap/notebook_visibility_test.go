package bootstrap

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The list and the decision agree (M3 design 4, Nerve's 3.4): on the
// matrix's data, each column's list of lab is exactly the notebooks of lab
// it reads one by one, each with the role the read answers. The list
// filters in SQL, the decision in the access module, both by
// shared.EffectiveNotebookRole: a query that drifts from it fails here. A
// column that is no member of lab sees no list and reads none.
//
// On this copy alone, lab's member who edits priv is also an explicit
// reader of team, which its access opens to lab's members to edit: the
// higher, editor, is that member's role in both (M3 Codex review R5); the
// guest reader of team stays a reader. Notebooks are told apart by id:
// names repeat.
func TestTheNotebookListIsWhatEachReadAllows(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	reader := callerNotebookEditor
	if _, err := connect(t, url).Exec(context.Background(),
		"INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
			"VALUES ($1, $2, $3, 'reader', $3, $3, now(), now())",
		uuid.NewV7(), s.notebook("team"), s.accounts[reader]); err != nil {
		t.Fatal(err)
	}
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	for _, c := range allColumns() {
		read := map[string]string{} // id → role, of the notebooks c reads
		for _, n := range matrixNotebooks() {
			if n.slug != "lab" {
				continue
			}
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+s.notebook(n.name).String(), d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var got listedAnswer
				decodeAnswer(t, answer, &got)
				read[got.ID] = got.Role
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s = %d %s", c, n.name, status, answer)
			}
		}
		status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/workspaces/lab/notebooks", d.tokens[c], "")
		listed := map[string]string{}
		switch status {
		case http.StatusOK:
			var list struct{ Data []listedAnswer }
			decodeAnswer(t, answer, &list)
			for _, n := range list.Data {
				listed[n.ID] = n.Role
			}
		case http.StatusNotFound:
		default:
			t.Fatalf("%s: GET lab's notebooks = %d %s", c, status, answer)
		}
		if !maps.Equal(listed, read) {
			t.Errorf("%s lists %v, reads %v; want the same notebooks, the same roles", c, listed, read)
		}
		team := s.notebook("team").String()
		if want := map[caller]string{reader: "editor", callerGuestReaderOfOpen: "reader"}[c]; want != "" && listed[team] != want {
			t.Errorf("%s lists team as %q, want %s", c, listed[team], want)
		}
	}
}

// The event stream sees what each read allows (M5/P2 design 3.7): on the
// matrix's data, the workspaces of each column are those it has a role
// in, each with that role, and the notebooks the stream sees in them are
// exactly those the column reads one by one. A deleted notebook is in
// neither, a workspace the column left or was removed from neither.
func TestTheStreamSeesWhatEachReadAllows(t *testing.T) {
	ctx := context.Background()
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	pool := connect(t, url)
	members := workspace.NewMemberships(pool)
	visibility := eventsVisibility{workspaces: members, notebooks: notebook.NewVisibleNotebooks(pool)}
	for _, c := range allColumns() {
		user := s.accounts[c]
		read := map[uuid.UUID]bool{}
		for _, n := range matrixNotebooks() {
			id := s.notebook(n.name)
			switch status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+id.String(), d.tokens[c], ""); status {
			case http.StatusOK:
				read[id] = true
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s = %d %s", c, n.name, status, answer)
			}
		}
		memberships, err := visibility.WorkspacesOf(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[uuid.UUID]bool{}
		listed := map[uuid.UUID]string{}
		for _, m := range memberships {
			listed[m.WorkspaceID] = string(m.Role)
			ids, err := visibility.NotebooksIn(ctx, m.WorkspaceID, user, m.Role)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range ids {
				seen[id] = true
			}
		}
		for slug, id := range s.workspaces {
			role, ok, err := members.RoleOf(ctx, id, user)
			if err != nil || ok != (listed[id] != "") || string(role) != listed[id] {
				t.Errorf("%s: the stream lists %s as %q, RoleOf answers %q, %v, %v", c, slug, listed[id], role, ok, err)
			}
		}
		if !maps.Equal(seen, read) {
			t.Errorf("%s: the stream sees %v, reads %v; want the same notebooks", c, seen, read)
		}
	}
}

// listedAnswer is a notebook as GET answers it, by id.
type listedAnswer struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// accountNotebooksDiff is how the notebooks MCP lists, by id with their
// roles, differ from those read one by one: one listed and not read, one
// read and not listed, one listed with a role other than the read's.
func accountNotebooksDiff(listed, read map[string]string) []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(listed)) {
		switch role, ok := read[id]; {
		case !ok:
			out = append(out, id+" is listed, not read")
		case role != listed[id]:
			out = append(out, id+" is listed as "+listed[id]+", read as "+role)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(read)) {
		if _, ok := listed[id]; !ok {
			out = append(out, id+" is read, not listed")
		}
	}
	return out
}

// The account's notebooks are what each read allows (M9/P1 design 3.17.4,
// v0.1 design 13.1 item 3): on the matrix's data, each column's
// notebooks, as MCP's notebooks lists them across its workspaces, are
// exactly the notebooks it reads one by one through REST, each with the
// role the read answers. On this copy alone, two columns are in two
// workspaces of notebooks: lab's notebook editor is also a member of
// other and a reader of other-nb, and the column outside lab, other-nb's
// admin, also a member of lab, where it sees what lab opens to its
// members.
func TestTheAccountsNotebooksAreWhatEachReadAllows(t *testing.T) {
	ctx := context.Background()
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	pool := connect(t, url)
	for _, m := range []struct {
		slug string
		c    caller
	}{{"other", callerNotebookEditor}, {"lab", callerOutsideWorkspace}} {
		if _, err := pool.Exec(ctx, `INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id,
				created_at, updated_at) VALUES ($1, $2, $3, 'member', $3, $3, now(), now())`,
			uuid.NewV7(), s.workspaces[m.slug], s.accounts[m.c]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at,
			updated_at) VALUES ($1, $2, $3, 'reader', $3, $3, now(), now())`,
		uuid.NewV7(), s.notebook("other-nb"), s.accounts[callerNotebookEditor]); err != nil {
		t.Fatal(err)
	}
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	for _, c := range allColumns() {
		read := map[string]string{} // id → role, of the notebooks c reads
		for _, n := range matrixNotebooks() {
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+s.notebook(n.name).String(), d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var got listedAnswer
				decodeAnswer(t, answer, &got)
				read[got.ID] = got.Role
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s = %d %s", c, n.name, status, answer)
			}
		}
		m := &mcpApp{base: base, pat: createToken(t, contract, base, d.tokens[c])}
		var got struct {
			Notebooks []accountNotebook `json:"notebooks"`
			Truncated bool              `json:"truncated"`
		}
		structuredOf(t, m.call(t, mcpserver.Pattern, "notebooks", map[string]any{}), &got)
		listed := map[string]string{}
		for _, n := range got.Notebooks {
			listed[n.ID] = n.Role
		}
		for _, v := range accountNotebooksDiff(listed, read) {
			t.Errorf("%s: %s", c, v)
		}
		for name, role := range map[caller]map[string]string{
			callerNotebookEditor:   {"priv": "editor", "other-nb": "reader"},
			callerOutsideWorkspace: {"team": "editor", "other-nb": "admin"},
		}[c] {
			if listed[s.notebook(name).String()] != role {
				t.Errorf("%s lists %s as %q, want %s: the copy's two workspaces", c, name, listed[s.notebook(name).String()], role)
			}
		}
	}
}

// Each kind of difference is reported: synthetic lists and reads, one
// difference at a time.
func TestAccountNotebooksDiffReportsEachDifference(t *testing.T) {
	read := map[string]string{"a": "admin", "b": "reader"}
	if found := accountNotebooksDiff(maps.Clone(read), read); len(found) != 0 {
		t.Fatalf("the same notebooks: %q", found)
	}
	for _, tt := range []struct {
		name   string
		listed map[string]string
		want   string
	}{
		{"one listed more", map[string]string{"a": "admin", "b": "reader", "c": "editor"}, "c is listed, not read"},
		{"one listed less", map[string]string{"a": "admin"}, "b is read, not listed"},
		{"a role other", map[string]string{"a": "admin", "b": "editor"}, "b is listed as editor, read as reader"},
	} {
		if found := accountNotebooksDiff(tt.listed, read); !slices.Equal(found, []string{tt.want}) {
			t.Errorf("%s: %q, want %q", tt.name, found, tt.want)
		}
	}
}
