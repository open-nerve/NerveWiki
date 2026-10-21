package bootstrap

import (
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// linkingMCPRows are linking's rows, backlinks and index, as the readers'
// too, in both entries (M9/P4 design 3.13): a column that cannot see its
// notebook is its 404, as page's rows. backlinks of the column's page
// answers no page, as REST's listBacklinks does: the seeded pages have no
// links, and the cell checks the code, TestTheMCPBacklinksAreRESTs the
// contents. index lists the column's notebook's seeded pages not deleted,
// each indexed by seededPageHistory, none linked to; the facts are
// TestTheIndexIsEachPagesReads'.
func linkingMCPRows() []mcpRow {
	readers := func(c caller) mcpCell {
		if _, seen := roleIn(c); seen {
			return mcpCell{}
		}
		return mcpCell{code: "notebook.not_found"}
	}
	return []mcpRow{
		{
			tool: "backlinks", entries: mcpserver.BothEntries, want: readers,
			args: func(c caller, _ seeded) map[string]any { return map[string]any{"path": pagePathOf(c)} },
			check: func(t *testing.T, _ caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var b struct {
					Pages      []any   `json:"pages"`
					NextCursor *string `json:"next_cursor"`
				}
				structuredOf(t, a, &b)
				if b.Pages == nil || len(b.Pages) != 0 || b.NextCursor != nil {
					t.Errorf("backlinks answered %.500s, want no page", a.body)
				}
			},
		},
		{
			tool: "index", entries: mcpserver.BothEntries, want: readers,
			check: func(t *testing.T, c caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var x struct {
					Pages []struct {
						Path         string `json:"path"`
						InboundLinks int    `json:"inbound_links"`
					} `json:"pages"`
					NextCursor *string `json:"next_cursor"`
				}
				structuredOf(t, a, &x)
				var got, want []string
				for _, p := range x.Pages {
					got = append(got, p.Path)
					if p.InboundLinks != 0 {
						t.Errorf("index answered %s with %d inbound links, want none", p.Path, p.InboundLinks)
					}
				}
				for _, p := range matrixPages() {
					if p.notebook == notebookOf(c) {
						want = append(want, matrixPathOf(p, ".md"))
					}
				}
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) || x.NextCursor != nil {
					t.Errorf("index answered %.800s, want the pages %q", a.body, want)
				}
			},
		},
	}
}
