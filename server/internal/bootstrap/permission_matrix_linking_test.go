package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The linking module's rows (M6/P5 design 7), by the notebook columns, as
// the page module's reads: any role reads, the rest do not see the
// notebook; a link's landing is its writers' (M6/P6 design 2). The seeded
// pages' contents are empty, so the index's answers are; the link targets
// are the notebook's pages.
func linkingMatrixRows() []matrixRow {
	notebookNotFound := cell{http.StatusNotFound, "notebook.not_found"}
	pageNotFound := cell{http.StatusNotFound, "page.not_found"}
	notebookPath := func(c caller, s seeded) string { return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() }
	pagePath := func(c caller, s seeded) string { return "/api/v0/pages/" + s.page(pageOf(c)).String() }
	readers := func(notFound cell) map[caller]cell {
		cells := map[caller]cell{}
		for _, c := range notebookColumns() {
			cells[c] = notFound
			if _, seen := roleIn(c); seen {
				cells[c] = cellOK()
			}
		}
		return cells
	}
	answers := func(want string) func(t *testing.T, c caller, s seeded, answer string) {
		return func(t *testing.T, _ caller, _ seeded, answer string) {
			t.Helper()
			if answer = strings.TrimSpace(answer); answer != want {
				t.Errorf("answered %s, want %s", answer, want)
			}
		}
	}
	return []matrixRow{
		{
			op:      "listBacklinks",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, pagePath(c, s) + "/backlinks", ""
			},
			cells: readers(pageNotFound),
			check: answers(`{"data":[],"next_cursor":null}`),
		},
		{
			op:      "getPageProperties",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, pagePath(c, s) + "/properties", ""
			},
			cells: readers(pageNotFound),
			check: answers(`{"links":[],"properties":[],"valid":true}`),
		},
		{
			op:      "listTags",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, notebookPath(c, s) + "/tags", ""
			},
			cells: readers(notebookNotFound),
			check: answers(`{"data":[]}`),
		},
		{
			op:      "getTag",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, notebookPath(c, s) + "/tags/a%2Fb", ""
			},
			cells: readers(notebookNotFound),
			check: answers(`{"data":[]}`),
		},
		{
			op:      "listLinkTargets",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, notebookPath(c, s) + "/link-targets", ""
			},
			cells: readers(notebookNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct {
					Data []struct {
						Name string `json:"name"`
						Link string `json:"link"`
					}
				}
				decodeAnswer(t, answer, &list)
				var names []string
				for _, n := range list.Data {
					names = append(names, n.Name)
					if n.Link != n.Name {
						t.Errorf("%s is written %q, not by its title, its own in the notebook", n.Name, n.Link)
					}
				}
				want := pagesOf(notebookOf(c))
				slices.Sort(names)
				slices.Sort(want)
				if !slices.Equal(names, want) {
					t.Errorf("link targets %q, want the notebook's pages %q", names, want)
				}
			},
		},
		{
			// A page new to every notebook lands beside the caller's page.
			op:      "getLinkLanding",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, pagePath(c, s) + "/link-landing?target=New", ""
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				parent := "null"
				if up := ancestorsOf(pageOf(c)); len(up) > 0 {
					parent = `"` + s.page(up[len(up)-1]).String() + `"`
				}
				answers(`{"landing":{"parent_id":`+parent+`,"title":"New"},"node_id":null,"reason":null}`)(t, c, s, answer)
			},
		},
	}
}
