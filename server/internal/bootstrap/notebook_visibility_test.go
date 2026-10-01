package bootstrap

import (
	"maps"
	"net/http"
	"testing"

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
func TestTheNotebookListIsWhatEachReadAllows(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	base := startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), nil), migrations.FS())
	s := d.seeded.in(t)
	for _, c := range allColumns() {
		read := map[string]string{} // name → role, of the notebooks c reads
		for _, n := range matrixNotebooks() {
			if n.slug != "lab" {
				continue
			}
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+s.notebook(n.name).String(), d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var got notebookAnswer
				decodeAnswer(t, answer, &got)
				read[got.Name] = got.Role
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s = %d %s", c, n.name, status, answer)
			}
		}
		status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/workspaces/lab/notebooks", d.tokens[c], "")
		listed := map[string]string{}
		switch status {
		case http.StatusOK:
			var list struct{ Data []notebookAnswer }
			decodeAnswer(t, answer, &list)
			for _, n := range list.Data {
				listed[n.Name] = n.Role
			}
		case http.StatusNotFound:
		default:
			t.Fatalf("%s: GET lab's notebooks = %d %s", c, status, answer)
		}
		if !maps.Equal(listed, read) {
			t.Errorf("%s lists %v, reads %v; want the same notebooks, the same roles", c, listed, read)
		}
	}
}
