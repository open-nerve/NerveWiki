package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The link index's reads through serve (M6/P5 design 9): what the index
// keeps of the pages' writes, read by a member of the notebook.

// get answers GET path as by, which must be 200, decoded into out.
func (tm acmeTeam) get(t *testing.T, by, path string, out any) {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+path, tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET %s as %s = %d %s", path, by, status, body)
	}
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("GET %s: %s: %v", path, body, err)
	}
}

// backlinkPage is a listBacklinks answer.
type backlinkPage struct {
	Data []struct {
		ID       string   `json:"id"`
		Count    int      `json:"count"`
		Contexts []string `json:"contexts"`
	} `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

// A page's backlinks come a page at a time, by the id of the page with the
// links, its own links left out; each with how many links lead there and
// their lines, a long one cut around the link; a link by an alias counts.
// A page whose index is of another revision than its content gives no
// lines (M6/P5 design 3).
func TestAPagesBacklinksThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	target := tm.createPageWith(t, "alice", nb, "", "Target", "---\naliases: [Goal]\n---\n[[Target]] [[#H]]\n## H\n")
	han := func(n int) string { return strings.Repeat("中", n) }
	near := tm.createPageWith(t, "alice", nb, "", "Near", "see [[Target]] here\n")
	long := tm.createPageWith(t, "alice", nb, "", "Long", han(200)+"[[Goal]]"+han(200))
	many := tm.createPageWith(t, "alice", nb, "", "Many", "[[Target]] [[Target]]\r\n[[Target#H]]")
	tm.createPageWith(t, "alice", nb, "", "Elsewhere", "[[Near]]")

	path := "/api/v0/pages/" + target + "/backlinks"
	var first backlinkPage
	tm.get(t, "bob", path+"?limit=2", &first)
	if first.NextCursor == nil || len(first.Data) != 2 || first.Data[0].ID != near || first.Data[1].ID != long {
		t.Fatalf("the first page: %+v", first)
	}
	if got := first.Data[0]; got.Count != 1 || !slices.Equal(got.Contexts, []string{"see [[Target]] here"}) {
		t.Errorf("Near: %+v", got)
	}
	if got, want := first.Data[1], "…"+han(26)+"[[Goal]]"+han(51)+"…"; got.Count != 1 || !slices.Equal(got.Contexts, []string{want}) {
		t.Errorf("Long: %+v, want the line around the link %q", got, want)
	}
	var last backlinkPage
	tm.get(t, "bob", path+"?limit=2&cursor="+url.QueryEscape(*first.NextCursor), &last)
	if last.NextCursor != nil || len(last.Data) != 1 || last.Data[0].ID != many || last.Data[0].Count != 3 ||
		!slices.Equal(last.Data[0].Contexts, []string{"[[Target]] [[Target]]", "[[Target#H]]"}) {
		t.Errorf("the last page: %+v", last)
	}
	var all backlinkPage
	tm.get(t, "bob", path+"?limit=3", &all)
	if all.NextCursor != nil || len(all.Data) != 3 {
		t.Errorf("an exactly full last page: %+v", all)
	}

	// A rename takes the links to the page by its old title elsewhere, or
	// to none: they are written again (M6/P4), and lead to it still; a
	// page renamed to the title of the links of another now has them.
	other := tm.createPageWith(t, "alice", nb, "", "Other", "[[Renamed]]\n")
	tm.send(t, nodeRename("alice", target, "Renamed"), http.StatusOK)
	tm.get(t, "bob", path+"?limit=5", &all)
	if len(all.Data) != 4 || all.Data[0].ID != near || all.Data[3].ID != other ||
		!slices.Equal(all.Data[0].Contexts, []string{"see [[Renamed]] here"}) || !slices.Equal(all.Data[3].Contexts, []string{"[[Renamed]]"}) {
		t.Errorf("after the rename: %+v", all)
	}

	if _, err := tm.pool.Exec(t.Context(), `UPDATE indexed_pages SET revision = revision + 1 WHERE node_id = $1`, near); err != nil {
		t.Fatal(err)
	}
	tm.get(t, "bob", path+"?limit=1", &first)
	if len(first.Data) != 1 || first.Data[0].ID != near || first.Data[0].Count != 1 || len(first.Data[0].Contexts) != 0 {
		t.Errorf("a page written since the index read it: %+v", first)
	}
}

// A page's properties are its frontmatter's, in the order written, with
// where each property link resolves, null for none; a page without a
// frontmatter has a valid one and none; an invalid one has none (M6/P5
// design 4).
func TestAPagesPropertiesThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	note := tm.createPage(t, "alice", nb, a, "Note")
	page := tm.createPageWith(t, "alice", nb, "", "Page",
		"---\nup: \"[[A]]\"\nsources:\n  - \"[[Missing]]\"\n  - \"[[A/Note]]\"\nn: 1000000000000000000000\n---\n[[A]]\n")
	plain := tm.createPageWith(t, "alice", nb, "", "Plain", "[[A]]\n")
	invalid := tm.createPageWith(t, "alice", nb, "", "Invalid", "---\nup: [[[A]]\n---\n")

	var got struct {
		Valid      bool              `json:"valid"`
		Properties []json.RawMessage `json:"properties"`
		Links      []struct {
			Key    string  `json:"key"`
			NodeID *string `json:"node_id"`
		} `json:"links"`
	}
	tm.get(t, "bob", "/api/v0/pages/"+page+"/properties", &got)
	var props []string
	for _, p := range got.Properties {
		props = append(props, string(p))
	}
	if want := []string{`{"key":"up","value":"[[A]]"}`, `{"key":"sources","value":["[[Missing]]","[[A/Note]]"]}`,
		`{"key":"n","value":1000000000000000000000}`}; !got.Valid || !slices.Equal(props, want) {
		t.Errorf("properties %v, %q; want %q", got.Valid, props, want)
	}
	var links []string
	for _, l := range got.Links {
		to := "null"
		if l.NodeID != nil {
			to = *l.NodeID
		}
		links = append(links, l.Key+" "+to)
	}
	if want := []string{"up " + a, "sources.0 null", "sources.1 " + note}; !slices.Equal(links, want) {
		t.Errorf("property links %q, want %q", links, want)
	}
	for id, valid := range map[string]bool{plain: true, invalid: false} {
		tm.get(t, "bob", "/api/v0/pages/"+id+"/properties", &got)
		if got.Valid != valid || len(got.Properties) != 0 || len(got.Links) != 0 {
			t.Errorf("properties of %s: %+v, want valid %t and none", id, got, valid)
		}
	}
}

// A notebook's tags are its pages', the body's and the frontmatter's, one
// a tag in any case, as most of its pages write it; a tag's pages are those
// with it or a tag under it, the nested one's '/' escaped in the path; a
// name no tag has has none (M6/P5 design 5).
func TestANotebooksTagsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	tagged := tm.createPageWith(t, "alice", nb, "", "Tagged", "---\ntags: [Proj/Sub]\n---\n#Proj and #proj\n")
	other := tm.createPageWith(t, "alice", nb, "", "Other", "#proj #中文\n")
	tm.createPageWith(t, "alice", nb, "", "Untagged", "# Proj\n")

	var tags struct {
		Data []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"data"`
	}
	tm.get(t, "bob", "/api/v0/notebooks/"+nb+"/tags", &tags)
	var got []string
	for _, tag := range tags.Data {
		got = append(got, tag.Tag+" "+string(rune('0'+tag.Count)))
	}
	if want := []string{"Proj 2", "Proj/Sub 1", "中文 1"}; !slices.Equal(got, want) {
		t.Errorf("tags %q, want %q", got, want)
	}
	for tag, want := range map[string][]string{
		"proj":                   {tagged, other},
		"PROJ%2Fsub":             {tagged},
		url.PathEscape("中文"):     {other},
		"Proj%2FSub%2FDeeper":    {},
		url.PathEscape("#proj"):  {},
		url.PathEscape("a b"):    {},
		url.PathEscape("\xffab"): {},
		url.PathEscape("a\x00b"): {},
	} {
		var pages struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		tm.get(t, "bob", "/api/v0/notebooks/"+nb+"/tags/"+tag, &pages)
		var ids []string
		for _, p := range pages.Data {
			ids = append(ids, p.ID)
		}
		if !reflect.DeepEqual(ids, want) && (len(ids) > 0 || len(want) > 0) {
			t.Errorf("the pages of %s: %q, want %q", tag, ids, want)
		}
	}
}

// A notebook's link targets are its pages, each with how a wikilink is
// written to lead to it alone, its path where another page has its title,
// and its aliases; a rename that leaves a title its page's alone writes
// it by the title again (M6/P5 design 6).
func TestANotebooksLinkTargetsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	b := tm.createPage(t, "alice", nb, "", "B")
	aNote := tm.createPage(t, "alice", nb, a, "Note")
	bNote := tm.createPageWith(t, "alice", nb, b, "note", "---\naliases: [Nb, Alpha]\n---\n")

	targets := func() map[string]string {
		t.Helper()
		var list struct {
			Data []struct {
				ID      string   `json:"id"`
				Kind    string   `json:"kind"`
				Name    string   `json:"name"`
				Link    string   `json:"link"`
				Aliases []string `json:"aliases"`
			} `json:"data"`
		}
		tm.get(t, "bob", "/api/v0/notebooks/"+nb+"/link-targets", &list)
		out := map[string]string{}
		var ids []string
		for _, l := range list.Data {
			out[l.ID] = l.Kind + " " + l.Name + " " + l.Link + " " + strings.Join(l.Aliases, ",")
			ids = append(ids, l.ID)
		}
		// By id: the pages in the order created, not the tree's (review r3-5).
		if want := []string{a, b, aNote, bNote}; !slices.Equal(ids, want) {
			t.Errorf("link targets %q, want by id %q", ids, want)
		}
		return out
	}
	want := map[string]string{a: "page A A ", b: "page B B ", aNote: "page Note A/Note ", bNote: "page note B/note Alpha,Nb"}
	if got := targets(); !reflect.DeepEqual(got, want) {
		t.Errorf("link targets %q\nwant %q", got, want)
	}
	tm.send(t, nodeRename("alice", aNote, "Only"), http.StatusOK)
	want[aNote], want[bNote] = "page Only Only ", "page note note Alpha,Nb"
	if got := targets(); !reflect.DeepEqual(got, want) {
		t.Errorf("after the rename: %q\nwant %q", got, want)
	}
}
