package bootstrap

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// mcpSearchAnswer is search's answer, as these tests read it.
type mcpSearchAnswer struct {
	Results []struct {
		Path      string    `json:"path"`
		Kind      string    `json:"kind"`
		Excerpt   *string   `json:"excerpt"`
		ChangedAt time.Time `json:"changed_at"`
	} `json:"results"`
	Count *struct {
		Value int    `json:"value"`
		Kind  string `json:"kind"`
	} `json:"count"`
	NextCursor *string `json:"next_cursor"`
	Building   bool    `json:"building"`
}

// TestTheMCPSearchIsTheNotebooksSearch: MCP's search of a notebook finds
// what REST's search of its workspace finds limited to that notebook, the
// same nodes in the same order, the same count (M9/P4 design 6.1): three
// titles, then twelve contents under a chain of nine pages of long titles,
// whose paths, "&" six bytes each in an answer, cut the first answer at
// 48 KiB among the contents' tier; each answer's next_cursor, the last
// kept hit's of its tier, goes on after it, none twice, none left out.
func TestTheMCPSearchIsTheNotebooksSearch(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	for i := range 3 {
		r.createPage(t, "", fmt.Sprintf("Alpha %d", i), "a title's page\n")
	}
	parent := ""
	for depth := range 9 {
		parent = r.createPage(t, parent, strings.Repeat("&", 100)+fmt.Sprint(depth), "")
	}
	for i := range 12 {
		r.createPage(t, parent, strings.Repeat("&", 100)+fmt.Sprintf(" leaf %02d", i), fmt.Sprintf("an alpha in leaf %d\n", i))
	}
	status, body := ask(t, r.contract, http.MethodGet, r.base+searchPath("acme", "alpha",
		url.Values{"notebook_id": {r.notebook}, "limit": {"50"}}), r.access, "")
	if status != http.StatusOK {
		t.Fatalf("REST's search = %d %s", status, body)
	}
	var rest searchAnswer
	decodeAnswer(t, body, &rest)
	var want []string
	for _, h := range rest.Data {
		var names []string
		for _, a := range h.Ancestors {
			names = append(names, a.Name)
		}
		path := strings.Join(append(names, h.Node.Name), "/")
		if h.Node.Kind == "page" {
			path += ".md"
		}
		want = append(want, path)
	}
	if len(want) != 15 || rest.NextCursor != nil || rest.Count == nil || rest.Count.Value != 15 || rest.Count.Kind != "exact" {
		t.Fatalf("REST's search = %s, want the 15 pages on one page, counted exact", body)
	}
	var got []string
	var pages []mcpSearchAnswer
	args := map[string]any{"q": "alpha", "limit": 50}
	for range 20 {
		a := answerOf[mcpSearchAnswer](t, r.use(t, "search", args))
		pages = append(pages, a)
		for _, res := range a.Results {
			got = append(got, res.Path)
		}
		if a.NextCursor == nil {
			break
		}
		args["cursor"] = *a.NextCursor
	}
	first := pages[0]
	switch {
	case strings.Join(got, "\n") != strings.Join(want, "\n"):
		t.Errorf("MCP's search found\n%q\nwant REST's\n%q", got, want)
	case len(first.Results) <= 3 || len(first.Results) >= 15 || !strings.HasPrefix(first.Results[len(first.Results)-1].Path, "&"):
		t.Errorf("the first answer holds %d results, want the titles and some of the contents, cut among them", len(first.Results))
	case first.Count == nil || *first.Count != *rest.Count || pages[1].Count != nil:
		t.Errorf("the counts are %v, then %v; want REST's on the first page alone", first.Count, pages[1].Count)
	}
}
