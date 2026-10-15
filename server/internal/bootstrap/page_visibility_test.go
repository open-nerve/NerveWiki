package bootstrap

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The tree and the reads agree (M4/P1 design 3.12): on the matrix's data,
// each column's trees of lab's notebooks hold exactly the pages and lab's
// attachments it reads one by one (M7/P2), by the same name under the same
// parent, and each page read's ancestors are its chain up the tree. A
// page's reading view answers as its read does (M4/P3 design 3.9). The
// tree is read by the notebook, the page by its node, each decided on by
// the access module: a read that drifts from the tree fails here. On this
// copy alone, team's page has a child and priv a deleted page, which
// neither shows; the copy keeps the pages' invariant. Then, through the
// API, team's default editor deletes its page with the child and its
// attachment (M4/P2 design 3.7): none is in a tree or read by any column.
func TestTheTreeIsWhatEachReadAllows(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	url := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	pages := slices.Collect(maps.Values(s.pages))
	extras := map[string]uuid.UUID{}
	pool := connect(t, url)
	for _, extra := range []struct {
		parent  string
		name    string
		deleted bool
	}{{"team-page", "team-child", false}, {"priv-root", "priv-trashed", true}} {
		id := uuid.NewV7()
		pages, extras[extra.name] = append(pages, id), id
		// A deleted page is a trash item of its own (M8/P2 design 3.3).
		if _, err := pool.Exec(context.Background(), `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
				created_by_id, updated_by_id, created_at, updated_at, deleted_at, trash_root, trash_pages, trash_assets, trash_height)
			SELECT $1, p.notebook_id, p.id, 'page', $3, $3, 1, p.created_by_id, p.created_by_id, now(), now(), CASE WHEN $4 THEN now() END,
				$4, CASE WHEN $4 THEN 1 END, CASE WHEN $4 THEN 0 END, CASE WHEN $4 THEN 1 END
			FROM nodes p WHERE p.id = $2`, id, s.page(extra.parent), extra.name, extra.deleted); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size,
				updated_by_id, updated_at, deleted_at)
			SELECT id, '', 1, sha256(''), 0, created_by_id, now(), deleted_at FROM nodes WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), seededPageHistory, id); err != nil {
			t.Fatal(err)
		}
	}
	queueSeeded(t, pool)
	checkPages(t, pool)
	base := startApp(t, d.config(t, url, nil), migrations.FS())
	teamPage := s.page("team-page")
	if status, answer := ask(t, contract, http.MethodDelete, base+"/api/v0/nodes/"+teamPage.String(), d.tokens[callerDefaultEditor], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE team's page = %d %s, want 204", status, answer)
	}
	checkPages(t, pool)
	gone := []string{teamPage.String(), extras["team-child"].String(), s.asset("team-page.png").String()}
	type place struct{ name, parent string }
	inLab := map[string]bool{} // the notebooks whose trees are read
	for _, n := range matrixNotebooks() {
		inLab[n.name] = n.slug == "lab"
	}
	for _, c := range allColumns() {
		tree := map[string]place{} // by id, of the trees c reads
		for _, n := range matrixNotebooks() {
			if !inLab[n.name] {
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
			if view, viewAnswer := ask(t, contract, http.MethodGet, base+"/api/v0/pages/"+id.String()+"/view", d.tokens[c], ""); view != status {
				t.Errorf("%s: a page's read = %d, its reading view = %d %s", c, status, view, viewAnswer)
			}
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
		for _, a := range matrixAssets() {
			if !inLab[a.notebook] {
				continue // other-nb's, which no tree here holds
			}
			status, answer := ask(t, contract, http.MethodGet, base+"/api/v0/assets/"+s.asset(a.name).String(), d.tokens[c], "")
			switch status {
			case http.StatusOK:
				var a struct {
					ID       string  `json:"id"`
					Name     string  `json:"name"`
					ParentID *string `json:"parent_id"`
				}
				decodeAnswer(t, answer, &a)
				read[a.ID] = place{a.Name, deref(a.ParentID)}
			case http.StatusNotFound:
			default:
				t.Fatalf("%s: GET an attachment = %d %s", c, status, answer)
			}
		}
		for _, id := range gone {
			if _, inTree := tree[id]; inTree || read[id] != (place{}) {
				t.Errorf("%s still has a deleted page %s: in the tree %v, read %v", c, id, inTree, read[id])
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

// The search and the reads agree (M8/P5 design 3.8; v0.1 design 13.1,
// item 3): on the matrix's data, each column's search by the mark of each
// seeded workspace it is a member of finds exactly the marked nodes it
// reads one by one, a page through GET /pages/{id}, an attachment through
// GET /assets/{id}: never one in the trash, nor one of a deleted notebook.
// Limited to a notebook it sees, the search finds those of that notebook;
// to one it does not, notebook.not_found. Twice, each on the state it
// relies on: before the job, every seeded node queued, deleted or not, and
// no row built, so that the reads' filter is what turns the deleted and
// the unseen away; then after a run, the queue empty, every node found
// through its row. Each runs on the search and the reads of the access
// module: a search that drifts from the reads fails here.
func TestTheSearchIsWhatEachReadAllows(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	db := pgtest.NewDatabaseFrom(t, d.url)
	s := d.seeded.in(t)
	pool := connect(t, db)
	base := startApp(t, d.config(t, db, nil), migrations.FS())
	notebookOfNode := map[string]string{} // the marked nodes' notebooks, by id
	for _, p := range slices.Concat(matrixPages(), matrixTrash()) {
		notebookOfNode[s.page(p.name).String()] = p.notebook
	}
	assets := map[string]bool{}
	for _, n := range matrixNotebooks() {
		id := s.asset(markedIn(n.name)).String()
		notebookOfNode[id], assets[id] = n.name, true
	}
	deleted := map[string]bool{}
	for _, p := range matrixTrash() {
		deleted[s.page(p.name).String()] = true
	}
	for id, nb := range notebookOfNode {
		if nb == "gone-nb" {
			deleted[id] = true
		}
	}
	search := func(t *testing.T, c caller, slug string, notebook *uuid.UUID) (int, searchAnswer, string) {
		t.Helper()
		more := url.Values{"limit": {"50"}}
		if notebook != nil {
			more.Set("notebook_id", notebook.String())
		}
		status, answer := ask(t, contract, http.MethodGet, base+searchPath(slug, matrixMark, more), d.tokens[c], "")
		var a searchAnswer
		switch status {
		case http.StatusOK:
			decodeAnswer(t, answer, &a)
			if a.NextCursor != nil || a.Building {
				t.Errorf("%s: search %s = %s, want one page, not building", c, slug, answer)
			}
			return status, a, ""
		case http.StatusNotFound:
			return status, a, problemCode(t, answer)
		}
		t.Fatalf("%s: search %s = %d %s", c, slug, status, answer)
		return 0, a, ""
	}
	check := func(t *testing.T) {
		for _, c := range allColumns() {
			var reads []string
			for id := range notebookOfNode {
				path := "/api/v0/pages/" + id
				if assets[id] {
					path = "/api/v0/assets/" + id
				}
				switch status, answer := ask(t, contract, http.MethodGet, base+path, d.tokens[c], ""); status {
				case http.StatusOK:
					reads = append(reads, id)
				case http.StatusNotFound:
				default:
					t.Fatalf("%s: GET %s = %d %s", c, path, status, answer)
				}
			}
			var found []string
			for _, w := range matrixWorkspaces() {
				status, a, code := search(t, c, w.slug, nil)
				if status == http.StatusNotFound && code != "workspace.not_found" {
					t.Errorf("%s: search %s = 404 %s, want workspace.not_found", c, w.slug, code)
				}
				found = append(found, a.ids()...)
				for _, n := range matrixNotebooks() {
					if n.slug != w.slug {
						continue
					}
					nb := s.notebook(n.name)
					inStatus, in, inCode := search(t, c, w.slug, &nb)
					var want []string
					for _, id := range reads {
						if notebookOfNode[id] == n.name {
							want = append(want, id)
						}
					}
					switch {
					case status == http.StatusNotFound:
						if inStatus != http.StatusNotFound || inCode != "workspace.not_found" {
							t.Errorf("%s: search %s in %s = %d %s, want workspace.not_found", c, w.slug, n.name, inStatus, inCode)
						}
					case inStatus == http.StatusNotFound:
						if inCode != "notebook.not_found" || len(want) != 0 {
							t.Errorf("%s: search %s in %s = 404 %s, want notebook.not_found, as it reads %d of its nodes", c, w.slug,
								n.name, inCode, len(want))
						}
					default:
						if got := slices.Sorted(slices.Values(in.ids())); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
							t.Errorf("%s: search %s in %s found %q, want those of it the column reads, %q", c, w.slug, n.name, got, want)
						}
					}
				}
			}
			for _, id := range found {
				if deleted[id] {
					t.Errorf("%s found %s, of the trash or of a deleted notebook", c, id)
				}
			}
			if got, want := slices.Sorted(slices.Values(found)), slices.Sorted(slices.Values(reads)); !slices.Equal(got, want) {
				t.Errorf("%s found %q, want the marked nodes it reads, %q", c, got, want)
			}
		}
	}
	t.Run("through the queue", func(t *testing.T) {
		if queued, rows, nodes := count(t, pool, "SELECT count(*) FROM page_search_queue WHERE NOT bulk"), count(t, pool,
			"SELECT count(*) FROM page_search"), count(t, pool, "SELECT count(*) FROM nodes"); queued != nodes || rows != 0 {
			t.Fatalf("%d nodes queued exact of %d, %d rows; want every node queued, no row", queued, nodes, rows)
		}
		check(t)
	})
	drainSearch(t, pool)
	t.Run("through the index", func(t *testing.T) {
		if queued, rows, live := count(t, pool, "SELECT count(*) FROM page_search_queue"), count(t, pool,
			"SELECT count(*) FROM page_search"), count(t, pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL"); queued != 0 ||
			rows != live {
			t.Fatalf("%d nodes queued, %d rows of %d live nodes; want none queued, a row for each", queued, rows, live)
		}
		check(t)
	})
	checkPages(t, pool)
}
