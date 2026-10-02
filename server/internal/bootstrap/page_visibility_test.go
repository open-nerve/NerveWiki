package bootstrap

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The tree and the reads agree (M4/P1 design 3.12): on the matrix's data,
// each column's trees of lab's notebooks hold exactly the pages it reads
// one by one, by the same name under the same parent, and each read's
// ancestors are its chain up the tree. The tree is read by the notebook,
// the page by its node, each decided on by the access module: a read that
// drifts from the tree fails here. On this copy alone, team's page has a
// child and priv a deleted page, which neither shows.
func TestTheTreeIsWhatEachReadAllows(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	pages := slices.Collect(maps.Values(s.pages))
	pool := connect(t, url)
	for _, extra := range []struct {
		parent  string
		name    string
		deleted bool
	}{{"team-page", "team-child", false}, {"priv-root", "priv-trashed", true}} {
		id := uuid.NewV7()
		pages = append(pages, id)
		if _, err := pool.Exec(context.Background(), `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
				created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT $1, p.notebook_id, p.id, 'page', $3, $3, 1, p.created_by_id, p.created_by_id, now(), now(), CASE WHEN $4 THEN now() END
			FROM nodes p WHERE p.id = $2`, id, s.page(extra.parent), extra.name, extra.deleted); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size,
				updated_by_id, updated_at, deleted_at)
			SELECT id, '', 1, sha256(''), 0, created_by_id, now(), deleted_at FROM nodes WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	type place struct{ name, parent string }
	for _, c := range allColumns() {
		tree := map[string]place{} // by id, of the trees c reads
		for _, n := range matrixNotebooks() {
			if n.slug != "lab" {
				continue
			}
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/notebooks/"+s.notebook(n.name).String()+"/nodes", d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var list struct {
					Data []struct {
						ID       string  `json:"id"`
						Name     string  `json:"name"`
						ParentID *string `json:"parent_id"`
					}
				}
				decodeAnswer(t, answer, &list)
				for _, node := range list.Data {
					tree[node.ID] = place{node.Name, deref(node.ParentID)}
				}
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET %s's tree = %d %s", c, n.name, status, answer)
			}
		}
		read := map[string]place{}
		for _, id := range pages {
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/pages/"+id.String(), d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var p struct {
					ID        string  `json:"id"`
					Name      string  `json:"name"`
					ParentID  *string `json:"parent_id"`
					Ancestors []struct {
						ID string `json:"id"`
					} `json:"ancestors"`
				}
				decodeAnswer(t, answer, &p)
				read[p.ID] = place{p.Name, deref(p.ParentID)}
				var chain []string
				for parent := tree[p.ID].parent; parent != ""; parent = tree[parent].parent {
					chain = append([]string{parent}, chain...)
				}
				var ancestors []string
				for _, a := range p.Ancestors {
					ancestors = append(ancestors, a.ID)
				}
				if !slices.Equal(ancestors, chain) {
					t.Errorf("%s reads %s under %v, want the tree's %v", c, p.Name, ancestors, chain)
				}
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET a page = %d %s", c, status, answer)
			}
		}
		if !maps.Equal(tree, read) {
			t.Errorf("%s's trees hold %v, it reads %v; want the same pages, in the same places", c, tree, read)
		}
	}
}

// deref is s, or "" for none.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
