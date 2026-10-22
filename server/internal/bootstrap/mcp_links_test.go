package bootstrap

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// mcpBacklink is a page of backlinks' answer, as these tests read it.
type mcpBacklink struct {
	Path     string   `json:"path"`
	Links    int      `json:"links"`
	Contexts []string `json:"contexts"`
}

// mcpBacklinksAnswer is backlinks' answer.
type mcpBacklinksAnswer struct {
	Pages      []mcpBacklink `json:"pages"`
	NextCursor *string       `json:"next_cursor"`
}

// mcpIndexPage is a page of index's answer.
type mcpIndexPage struct {
	Path         string    `json:"path"`
	Type         *string   `json:"type"`
	Summary      *string   `json:"summary"`
	Tags         []string  `json:"tags"`
	ChangedAt    time.Time `json:"changed_at"`
	InboundLinks int       `json:"inbound_links"`
	Parent       *string   `json:"parent"`
	Cut          []string  `json:"cut"`
}

// mcpIndexAnswer is index's answer.
type mcpIndexAnswer struct {
	Pages      []mcpIndexPage `json:"pages"`
	NextCursor *string        `json:"next_cursor"`
}

// restBacklink is an item of REST's listBacklinks.
type restBacklink struct {
	ID       string   `json:"id"`
	Count    int      `json:"count"`
	Contexts []string `json:"contexts"`
}

// backlinksThrough is backlinks of args, answer after answer while
// next_cursor is set, 50 at most: each answer.
func (r readsApp) backlinksThrough(t *testing.T, args map[string]any) []mcpBacklinksAnswer {
	t.Helper()
	var out []mcpBacklinksAnswer
	args = maps.Clone(args)
	for range 50 {
		a := answerOf[mcpBacklinksAnswer](t, r.use(t, "backlinks", args))
		if out = append(out, a); a.NextCursor == nil {
			return out
		}
		args["cursor"] = *a.NextCursor
	}
	t.Fatalf("backlinks %v answered 50 times with a cursor", args)
	return nil
}

// indexThrough is index of args, answer after answer while next_cursor is
// set, 50 at most: their pages.
func (r readsApp) indexThrough(t *testing.T, args map[string]any) []mcpIndexPage {
	t.Helper()
	var out []mcpIndexPage
	args = maps.Clone(args)
	for range 50 {
		a := answerOf[mcpIndexAnswer](t, r.use(t, "index", args))
		if out = append(out, a.Pages...); a.NextCursor == nil {
			return out
		}
		args["cursor"] = *a.NextCursor
	}
	t.Fatalf("index %v answered 50 times with a cursor", args)
	return nil
}

// indexPaths are the pages' paths, in their order.
func indexPaths(pages []mcpIndexPage) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.Path
	}
	return out
}

// restGet is the REST read of path, which must answer 200, decoded into v.
func (r readsApp) restGet(t *testing.T, path string, v any) {
	t.Helper()
	status, body := ask(t, r.contract, http.MethodGet, r.base+path, r.access, "")
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, status, body)
	}
	decodeAnswer(t, body, v)
}

// restBacklinks is REST's listBacklinks of the page id, on one page.
func (r readsApp) restBacklinks(t *testing.T, id string) []restBacklink {
	t.Helper()
	var b struct {
		Data       []restBacklink `json:"data"`
		NextCursor *string        `json:"next_cursor"`
	}
	r.restGet(t, "/api/v0/pages/"+id+"/backlinks?limit=100", &b)
	if b.NextCursor != nil {
		t.Fatalf("the backlinks of %s are more than a page", id)
	}
	return b.Data
}

// Each method of linking's NodePaths reaches page's (M9/P4 design 3.9.1,
// 6.1), as bootstrap wires them: one that answers nothing fails here.
// Check refuses a path before the notebook is decided, a notebook alice
// does not see; FindPage finds a page by its path in any case, and
// answers page's words for nothing there and for an attachment; Paths
// writes backlinks' sources' paths; Pages index's items' paths and their
// last changes, list's; ChildPages a parent's child pages, an attachment
// not among them, and the root's.
func TestTheMCPLinksReachTheirUseCases(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	projects := r.createPage(t, "", "Projects", "")
	r.createPage(t, projects, "Roadmap", "")
	r.upload(t, projects, "diagram.png", imageOf(t, "png"))
	r.createPage(t, "", "Inbox", "see [[Projects/Roadmap]]\n")

	for _, tool := range []string{"backlinks", "index"} {
		args := map[string]any{"notebook": r.hidden, "path": "a//b.md"}
		if tool == "index" {
			args = map[string]any{"notebook": r.hidden, "parent": "a//b.md"}
		}
		member := r.call(t, mcpserver.Pattern, tool, args).toolError(t)
		if fields, _ := member["fields"].([]any); member["code"] != "validation_failed" || len(fields) != 1 {
			t.Errorf("%s of a path of an empty segment in a notebook not seen = %v, want validation_failed first", tool, member)
		}
	}
	b := answerOf[mcpBacklinksAnswer](t, r.use(t, "backlinks", map[string]any{"path": "projects/roadmap.md"}))
	want := []mcpBacklink{{Path: "Inbox.md", Links: 1, Contexts: []string{"see [[Projects/Roadmap]]"}}}
	if !slices.EqualFunc(b.Pages, want, backlinkEqual) || b.NextCursor != nil {
		t.Errorf("backlinks of projects/roadmap.md = %+v, want %+v", b, want)
	}
	for path, text := range map[string]string{
		"Projects/diagram.png": `page.not_found: "Projects/diagram.png" is an attachment; a page's path ends with .md.`,
		"Projects/Missing.md":  `page.not_found: Nothing is at "Projects/Missing.md".`,
	} {
		for tool, field := range map[string]string{"backlinks": "path", "index": "parent"} {
			member := r.use(t, tool, map[string]any{field: path}).toolError(t)
			if message, _ := member["message"].(string); member["code"] != "page.not_found" || !strings.HasPrefix(message, text) {
				t.Errorf("%s of %s = %v, want %q", tool, path, member, text)
			}
		}
	}

	var listed struct {
		Items []struct {
			Path      string    `json:"path"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"items"`
	}
	structuredOf(t, r.use(t, "list", map[string]any{"pattern": "**/*.md"}), &listed)
	changed := map[string]time.Time{}
	for _, it := range listed.Items {
		changed[it.Path] = it.UpdatedAt
	}
	all := r.indexThrough(t, map[string]any{})
	if got := indexPaths(all); !slices.Equal(got, []string{"Projects.md", "Projects/Roadmap.md", "Inbox.md"}) {
		t.Errorf("index = %q, want the three pages by id", got)
	}
	for _, p := range all {
		if !p.ChangedAt.Equal(changed[p.Path]) || p.ChangedAt.IsZero() {
			t.Errorf("index's %s changed at %v, list's at %v", p.Path, p.ChangedAt, changed[p.Path])
		}
	}
	for parent, want := range map[string][]string{"Projects.md": {"Projects/Roadmap.md"}, "/": {"Projects.md", "Inbox.md"}} {
		if got := indexPaths(r.indexThrough(t, map[string]any{"parent": parent})); !slices.Equal(got, want) {
			t.Errorf("index of the parent %s = %q, want %q", parent, got, want)
		}
	}
}

// backlinkEqual is whether two backlinks are the same.
func backlinkEqual(a, b mcpBacklink) bool {
	return a.Path == b.Path && a.Links == b.Links && slices.Equal(a.Contexts, b.Contexts)
}

// MCP's backlinks of a page are REST's listBacklinks of it (M9/P4 design
// 3.9.2, 6.1): the same pages in the same order, each with its count and
// its contexts, its path for its id; a page that links to itself, and a
// source in the trash, not among them. Twelve of the sources are under a
// chain of nine pages of long titles, whose paths, "&" six bytes each in
// an answer, cut the first answer at 48 KiB; each answer's next_cursor
// goes on after its last page, and so does each of limit 1.
func TestTheMCPBacklinksAreRESTs(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	target := r.createPage(t, "", "Roadmap", "# Roadmap\nsee [[Roadmap]]\n")
	paths := map[string]string{
		r.createPage(t, "", "Inbox", "see [[Roadmap]] first\nthen [[Roadmap|the plan]] again\n"): "Inbox.md",
	}
	parent, prefix := "", ""
	for depth := range 9 {
		title := strings.Repeat("&", 100) + fmt.Sprint(depth)
		parent, prefix = r.createPage(t, parent, title, ""), prefix+title+"/"
	}
	for i := range 12 {
		title := fmt.Sprintf("Leaf %02d", i)
		paths[r.createPage(t, parent, title, fmt.Sprintf("leaf %d links [[Roadmap]]\n", i))] = prefix + title + ".md"
	}
	r.createPage(t, "", "Gone", "[[Roadmap]]\n")
	answerOf[map[string]any](t, r.use(t, "delete", map[string]any{"path": "Gone.md"}))

	var want []mcpBacklink
	for _, b := range r.restBacklinks(t, target) {
		path, ok := paths[b.ID]
		if !ok {
			t.Fatalf("REST's backlinks list %s, which is no source", b.ID)
		}
		want = append(want, mcpBacklink{Path: path, Links: b.Count, Contexts: b.Contexts})
	}
	if len(want) != 13 || want[0].Links != 2 || len(want[0].Contexts) != 2 {
		t.Fatalf("REST's backlinks are %+v, want Inbox's two links, then the twelve leaves'", want)
	}
	for _, args := range []map[string]any{{"path": "Roadmap.md"}, {"path": "Roadmap.md", "limit": 1}} {
		answers := r.backlinksThrough(t, args)
		var got []mcpBacklink
		for _, a := range answers {
			got = append(got, a.Pages...)
		}
		if !slices.EqualFunc(got, want, backlinkEqual) {
			t.Errorf("MCP's backlinks of %v are\n%+v\nwant REST's\n%+v", args, got, want)
		}
		if n := len(answers[0].Pages); args["limit"] == nil && (n <= 1 || n >= len(want)) {
			t.Errorf("the first answer holds %d pages, want some, cut at the bound", n)
		}
	}
}

// asciiLower is s with its ASCII letters in lower case alone, as the
// index's translate compares a property's key.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// The index is each page's reads (M9/P4 design 3.9.3, 6.1): the pages
// with a type first, by type, the others after them, each by id, a page
// in the trash not among them; each page as REST and list read it one by
// one. Its type and summary are the first property of that key but for
// ASCII case, getPageProperties', when its value is text, cut to 256
// bytes, cut naming it; its tags the page's own, by key, twenty at most,
// cut naming the rest, each listed by getTag and each of listTags' whose
// getTag lists the page its own or one under it (a page tagged plan/q4
// alone has no tag plan, S2 review B-n2; Plan and apple are ordered by
// key, C-m1); its inbound links the pages of listBacklinks, a page that
// links twice once; its path and its last change list's; its parent its
// path's parent page's.
func TestTheIndexIsEachPagesReads(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	summary := "The roadmap of " + strings.Repeat("计划", 50)
	// Tags of both cases, 25: their first 20 by key are not their first 20
	// by text. The seed's tags are ASCII: a tag's key is its lower case.
	var many []string
	for i := 25; i > 0; i-- {
		many = append(many, fmt.Sprintf([]string{"t%02d", "T%02d"}[i%2], i))
	}
	own := map[string][]string{"Projects/Roadmap.md": {"plan", "plan/q4"}, "Projects/Spec.md": {"plan"}, "Inbox.md": {"plan/q4"},
		"Mixed.md": {"Plan", "apple"}, "Tagged.md": many}
	key := strings.ToLower
	projects := r.createPage(t, "", "Projects", "---\ntype: area\n---\n")
	r.createPage(t, projects, "Roadmap", "---\ntype: project\ntags: [plan, plan/q4]\nsummary: "+summary+"\n---\n# Roadmap\n")
	r.createPage(t, projects, "Spec", "---\ntype: project\ntags: [plan]\n---\n")
	r.createPage(t, projects, "Notes", "[[Projects/Roadmap]] and [[Roadmap]]\n")
	r.createPage(t, "", "Inbox", "---\ntags: [plan/q4]\n---\nsee [[Projects/Roadmap]] and [[Projects/Spec]]\n")
	r.createPage(t, "", "Mixed", "---\nTYPE: Mixed\ntype: later\nSummary: [a, b]\nsummary: text\ntags: [Plan, apple]\n---\n")
	r.createPage(t, "", "Tagged", "---\ntags: ["+strings.Join(many, ", ")+"]\n---\n")
	r.createPage(t, "", "Gone", "---\ntype: project\n---\n")
	answerOf[map[string]any](t, r.use(t, "delete", map[string]any{"path": "Gone.md"}))

	var listed struct {
		Items []struct {
			Path      string    `json:"path"`
			Version   string    `json:"version"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"items"`
	}
	structuredOf(t, r.use(t, "list", map[string]any{"pattern": "**/*.md"}), &listed)
	ids, changed := map[string]string{}, map[string]time.Time{}
	for _, it := range listed.Items {
		ids[it.Path], _, _ = strings.Cut(it.Version, ":")
		changed[it.Path] = it.UpdatedAt
	}
	var tags struct {
		Data []struct {
			Tag string `json:"tag"`
		} `json:"data"`
	}
	r.restGet(t, "/api/v0/notebooks/"+r.notebook+"/tags", &tags)
	tagsOf := map[string][]string{}
	for _, tag := range tags.Data {
		var pages struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		r.restGet(t, "/api/v0/notebooks/"+r.notebook+"/tags/"+url.PathEscape(tag.Tag), &pages)
		for _, p := range pages.Data {
			tagsOf[p.ID] = append(tagsOf[p.ID], tag.Tag)
		}
	}

	got := r.indexThrough(t, map[string]any{"limit": 3})
	// By type, then by id; the pages without a type by id: an id's text is
	// in its bytes' order.
	byID := func(paths ...string) []string {
		return slices.SortedFunc(slices.Values(paths), func(a, b string) int { return strings.Compare(ids[a], ids[b]) })
	}
	order := slices.Concat([]string{"Projects.md", "Mixed.md"}, byID("Projects/Roadmap.md", "Projects/Spec.md"),
		byID("Projects/Notes.md", "Inbox.md", "Tagged.md"))
	if !slices.Equal(indexPaths(got), order) {
		t.Fatalf("index = %q, want %q", indexPaths(got), order)
	}
	inbound := map[string]int{}
	for _, p := range got {
		id := ids[p.Path]
		var props struct {
			Properties []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"properties"`
		}
		r.restGet(t, "/api/v0/pages/"+id+"/properties", &props)
		var cut []string
		first := func(key string) *string {
			for _, prop := range props.Properties {
				if asciiLower(prop.Key) != key {
					continue
				}
				var s string
				if json.Unmarshal(prop.Value, &s) != nil {
					return nil
				}
				if v, c := domain.CutIndexString(s); c {
					cut, s = append(cut, key), v
				}
				return &s
			}
			return nil
		}
		typ, sum := first("type"), first("summary")
		for _, x := range own[p.Path] {
			if !slices.ContainsFunc(tagsOf[id], func(t string) bool { return key(t) == key(x) }) {
				t.Errorf("getTag of %s does not list %s, which has it", x, p.Path)
			}
		}
		for _, tag := range tagsOf[id] {
			if !slices.ContainsFunc(own[p.Path], func(x string) bool { return key(x) == key(tag) || strings.HasPrefix(key(x), key(tag)+"/") }) {
				t.Errorf("getTag of %s lists %s, which has neither it nor one under it", tag, p.Path)
			}
		}
		want := slices.SortedFunc(slices.Values(own[p.Path]), func(a, b string) int { return strings.Compare(key(a), key(b)) })
		if len(want) > domain.MaxIndexTags {
			want, cut = want[:domain.MaxIndexTags], append(cut, "tags")
		}
		var parent *string
		if i := strings.LastIndex(p.Path, "/"); i >= 0 {
			parent = new(p.Path[:i] + ".md")
		}
		inbound[p.Path] = len(r.restBacklinks(t, id))
		if !equalText(p.Type, typ) || !equalText(p.Summary, sum) || !slices.Equal(p.Tags, want) || !slices.Equal(p.Cut, cut) ||
			p.InboundLinks != inbound[p.Path] || !equalText(p.Parent, parent) || !p.ChangedAt.Equal(changed[p.Path]) {
			t.Errorf("index's %s = %+v; want type %v, summary %v, tags %q, cut %q, %d inbound, parent %v, changed at %v", p.Path, p,
				deref(typ), deref(sum), want, cut, inbound[p.Path], deref(parent), changed[p.Path])
		}
	}
	// The rows above tell what they check: the seed has each case, a
	// summary cut, a type of the first key, a summary of a list, more than
	// twenty tags, a page that links twice, a page getTag of plan lists that
	// has plan/q4 alone, tags whose order by key is not their order by text.
	at := map[string]mcpIndexPage{}
	for _, p := range got {
		at[p.Path] = p
	}
	roadmap, mixed, tagged := at["Projects/Roadmap.md"], at["Mixed.md"], at["Tagged.md"]
	if inbound["Projects/Roadmap.md"] != 2 || inbound["Projects/Spec.md"] != 1 || !slices.Equal(roadmap.Cut, []string{"summary"}) ||
		mixed.Type == nil || *mixed.Type != "Mixed" || mixed.Summary != nil || len(tagged.Tags) != domain.MaxIndexTags ||
		!slices.Equal(tagged.Cut, []string{"tags"}) || !slices.Equal(at["Inbox.md"].Tags, []string{"plan/q4"}) ||
		!slices.ContainsFunc(tagsOf[ids["Inbox.md"]], func(t string) bool { return key(t) == "plan" }) ||
		!slices.Equal(mixed.Tags, []string{"apple", "Plan"}) || slices.Equal(tagged.Tags, slices.Sorted(slices.Values(many))[:domain.MaxIndexTags]) {
		t.Errorf("the seed lacks its cases: inbound %v, the pages %+v", inbound, got)
	}
}

// equalText is whether two strings that may be none are the same.
func equalText(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
