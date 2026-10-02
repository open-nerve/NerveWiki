package bootstrap

import (
	"context"
	"maps"
	"net/http"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
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

// listedAnswer is a notebook as GET answers it, by id.
type listedAnswer struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}
