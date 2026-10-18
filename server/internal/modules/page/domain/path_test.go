package domain_test

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// acute is U+0301 in UTF-8, which composes with the e before it in NFC.
const acute = "\xcc\x81"

// pathOrProblem is p as titles, kind and dir, or err's problem.
func pathOrProblem(p domain.Path, err error) string {
	if err != nil {
		return problemOf(err)
	}
	out := strings.Join(p.Titles, "|") + " " + string(p.Kind)
	if p.Dir {
		out += " dir"
	}
	return out
}

// A path's last segment tells its kind by ".md", in any case; its
// ancestors are titles as they are; one leading "/" goes, and a trailing
// one names a page's children in list alone (M9 design 4.8, the four
// cases besides the links' rules).
func TestParsePath(t *testing.T) {
	long := strings.Repeat("a", 255)
	for _, tt := range []struct {
		path string
		dir  bool
		want string
	}{
		{"Guide/Setup.md", false, "Guide|Setup page"},
		{"/Guide/Setup.md", false, "Guide|Setup page"},
		{"Guide/Setup.MD", false, "Guide|Setup page"},
		{"Guide/Setup.mD", false, "Guide|Setup page"},
		{"A/x.png", false, "A|x.png asset"},          // only an attachment: a page x.png is A/x.png.md
		{"A/x.png.md", false, "A|x.png page"},        // the page x.png
		{"A/x.md", false, "A|x page"},                // only the page x
		{"A/x.md.md", false, "A|x.md page"},          // the page x.md
		{"A/data", false, "A|data asset"},            // an attachment without an extension
		{"A.md/B.md", false, "A.md|B page"},          // an ancestor's ".md" is its title's
		{"Cafe" + acute + ".md", false, "Café page"}, // NFD in, NFC out
		{"a b/c d.md", false, "a b|c d page"},        // inner white space stays
		{"A/", true, "A page dir"},                   // list's children of A
		{"/A/B/", true, "A|B page dir"},              // list's children of A/B
		{"", true, " page dir"},                      // the root's children
		{"/", true, " page dir"},                     // the root's children
		{"A", true, "A asset"},                       // list never asks this of a path without a "/"
		{strings.Repeat("a/", 10) + "b.md", false, strings.Repeat("a|", 10) + "b page"},
		{long + "/" + long + ".md", false, long + "|" + long + " page"},
		{"", false, "validation_failed path required"},
		{"/", false, "validation_failed path required"},
		{"A/", false, "validation_failed path invalid_format"},
		{"//", true, "validation_failed path invalid_format"},
		{"A//B.md", false, "validation_failed path invalid_format"},
		{"./A.md", false, "validation_failed path invalid_format"},
		{"A/../B.md", false, "validation_failed path invalid_format"},
		{"A/./B.md", false, "validation_failed path invalid_format"},
		{".md", false, "validation_failed path required"}, // a page of no title
		{"A /B.md", false, "validation_failed path invalid_format"},
		{"A/ B.md", false, "validation_failed path invalid_format"},
		{"A/B .md", false, "validation_failed path invalid_format"},
		{"A/B\x00.md", false, "validation_failed path invalid_format"},
		{"A/B:C.md", false, "validation_failed path invalid_format"},
		{"A/B*.md", false, "validation_failed path invalid_format"},
		{"A/.hidden", false, "validation_failed path invalid_format"},
		{"A/CON.md", false, "validation_failed path not_allowed"},
		{"A/" + long + "a.md", false, "validation_failed path too_long"},
		{"A/\xff.md", false, "validation_failed path invalid_format"},
		{strings.Repeat("a/", 11) + "b.md", false, "validation_failed path too_long"},
		{strings.Repeat("a", 4097), false, "validation_failed path too_long"},
	} {
		got, err := domain.ParsePath("path", tt.path, tt.dir)
		if s := pathOrProblem(got, err); s != tt.want {
			t.Errorf("ParsePath(%q, %v) = %q, want %q", tt.path, tt.dir, s, tt.want)
		}
	}
}

// A segment's problem says which segment: the second's here, with
// CheckTitle's code; one that only CheckTitle's trim would keep is its own
// invalid_format, its title the segment's NFC but for the white space. A
// trailing "/" says that only list takes it.
func TestParsePathNamesTheSegment(t *testing.T) {
	for path, want := range map[string]string{
		"A/":         "must not end with /: only list takes a path ending with /",
		"A/B:C.md":   "segment 2 must not contain",
		"A/ B.md":    "segment 2 must not start or end with white space",
		"A/B.md /C":  "segment 2 must not start or end with white space",
		"A/CON/B.md": "segment 2 is a name Windows reserves",
		"A/.md":      "segment 2 is required",
	} {
		_, err := domain.ParsePath("path", path, false)
		var e *shared.Error
		if !errors.As(err, &e) || len(e.Fields) != 1 || !strings.HasPrefix(e.Fields[0].Message, want) {
			t.Errorf("ParsePath(%q) = %v, want a message starting %q", path, err, want)
		}
		if seg := strings.Split(path, "/")[1]; strings.TrimSpace(seg) != seg {
			if title, f := shared.CheckTitle("path", strings.TrimSuffix(seg, ".md")); f != nil || title != strings.TrimSpace(strings.TrimSuffix(seg, ".md")) {
				t.Errorf("CheckTitle(%q) = %q, %v: more than the white space differs", seg, title, f)
			}
		}
	}
}

// The keys of a path are its titles' keys: NFD and any case find what
// NFC does.
func TestParsePathKeys(t *testing.T) {
	a, err := domain.ParsePath("path", "Café/STRASSE.md", false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := domain.ParsePath("path", "cafe"+acute+"/straße.MD", false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(a.Keys, b.Keys) || a.Keys[0] != shared.TitleKey("Café") {
		t.Errorf("the keys %q and %q differ", a.Keys, b.Keys)
	}
}

// PathOf writes the ancestors' titles, then the node's, a page's with
// ".md"; ParsePath reads back the titles and the kind, for any titles.
func TestPathOfIsWhatParsePathReads(t *testing.T) {
	page := func(name string) domain.Node { return domain.Node{Kind: domain.KindPage, Name: name} }
	for _, tt := range []struct {
		ancestors []domain.Node
		node      domain.Node
		want      string
	}{
		{nil, page("Guide"), "Guide.md"},
		{[]domain.Node{page("Guide")}, page("Setup"), "Guide/Setup.md"},
		{[]domain.Node{page("A")}, domain.Node{Kind: domain.KindAsset, Name: "x.png"}, "A/x.png"},
		{[]domain.Node{page("A")}, page("x.png"), "A/x.png.md"},
		{[]domain.Node{page("A.md")}, page("x.md"), "A.md/x.md.md"},
	} {
		if got := domain.PathOf(tt.ancestors, tt.node); got != tt.want {
			t.Errorf("PathOf = %q, want %q", got, tt.want)
		}
	}
	rnd := rand.New(rand.NewPCG(3, 5))
	alphabet := []string{"a", "B", "é", "漢", ".md", ".MD", ".png", " ", "-", "ß", "!", "x"}
	for range 2000 {
		var ancestors []domain.Node
		for range rnd.IntN(10) {
			ancestors = append(ancestors, page(randomTitle(rnd, alphabet)))
		}
		node := page(randomTitle(rnd, alphabet))
		if rnd.IntN(2) == 0 {
			node.Kind = domain.KindAsset
			if strings.HasSuffix(strings.ToLower(node.Name), ".md") {
				node.Name += "x"
			}
		}
		path := domain.PathOf(ancestors, node)
		got, err := domain.ParsePath("path", path, false)
		var titles []string
		for _, a := range ancestors {
			titles = append(titles, a.Name)
		}
		titles = append(titles, node.Name)
		if err != nil || !slices.Equal(got.Titles, titles) || got.Kind != node.Kind {
			t.Fatalf("ParsePath(%q) = %+v, %v; want %q, %s", path, got, err, titles, node.Kind)
		}
	}
}

// randomTitle is a title of the alphabet's pieces that CheckTitle keeps
// as it is.
func randomTitle(rnd *rand.Rand, alphabet []string) string {
	for {
		var b strings.Builder
		for range 1 + rnd.IntN(4) {
			b.WriteString(alphabet[rnd.IntN(len(alphabet))])
		}
		s := b.String()
		if got, f := shared.CheckTitle("t", s); f == nil && got == s {
			return s
		}
	}
}

// PathNotFound is page.not_found.
func TestPathNotFoundIsNotFound(t *testing.T) {
	var err error = &domain.PathNotFound{Path: "A/x", Hint: domain.PageHint("A/x.md")}
	if !errors.Is(err, domain.ErrNotFound) || err.Error() != domain.ErrNotFound.Error() {
		t.Errorf("PathNotFound is %v", err)
	}
}

// A version token is the node's canonical id and the revision in decimal.
func TestVersionTokenString(t *testing.T) {
	id := uuid.NewV7()
	if got := (domain.VersionToken{Node: id, Revision: 12}).String(); got != id.String()+":12" || strings.ToLower(got) != got {
		t.Errorf("the token is %q", got)
	}
}

// A version token parses back as String writes it, and in no other
// spelling: the round trip refuses what uuid.Parse and strconv.Atoi take
// besides, upper case, braces, 32 hexadecimal digits, a sign and leading
// zeros (M9/P2 design 3.4). Each refusal is invalid_format on the field,
// the value not repeated.
func TestParseVersionToken(t *testing.T) {
	rnd := rand.New(rand.NewPCG(4, 1))
	for range 1000 {
		want := domain.VersionToken{Node: uuid.NewV7(), Revision: 1 + rnd.IntN(domain.MaxRevision)}
		if got, err := domain.ParseVersionToken("base_version", want.String()); err != nil || got != want {
			t.Fatalf("ParseVersionToken(%q) = %v, %v", want.String(), got, err)
		}
	}
	id := uuid.NewV7().String()
	if got, err := domain.ParseVersionToken("base_version", id+":2147483647"); err != nil || got.Revision != domain.MaxRevision {
		t.Errorf("ParseVersionToken(MaxRevision) = %v, %v", got, err)
	}
	if got, err := domain.ParseVersionToken("base_version", uuid.UUID{}.String()+":1"); err != nil || got.Node != (uuid.UUID{}) {
		t.Errorf("ParseVersionToken(the zero id) = %v, %v: it parses, and is no node", got, err)
	}
	for _, bad := range []string{
		"", ":", "x:1", id, id + ":", id + ":0", id + ":-1", id + ":+1", id + ":01", strings.ToUpper(id) + ":1",
		"{" + id + "}:1", "urn:uuid:" + id + ":1", strings.ReplaceAll(id, "-", "") + ":1", id + ":2147483648",
		id + ": 1", id + ":1 ", id + ":" + strings.Repeat("1", 11), id + ":1\x00", strings.Repeat(id+":1", 30000),
	} {
		_, err := domain.ParseVersionToken("base_version", bad)
		var e *shared.Error
		if !errors.As(err, &e) || problemOf(err) != "validation_failed base_version invalid_format" ||
			e.Fields[0].Message != `is not a version, "<node id>:<revision>" as read, list and write give it` {
			t.Errorf("ParseVersionToken(%.60q) = %v, want 422 on base_version", bad, err)
		}
	}
}

// PathOf is PathOfNames of the ancestors' names and the node's.
func TestPathOfNames(t *testing.T) {
	for _, tt := range []struct {
		names []string
		kind  domain.Kind
		want  string
	}{
		{[]string{"A"}, domain.KindPage, "A.md"},
		{[]string{"A", "b.md", "c"}, domain.KindPage, "A/b.md/c.md"},
		{[]string{"A", "x.png"}, domain.KindAsset, "A/x.png"},
	} {
		if got := domain.PathOfNames(tt.names, tt.kind); got != tt.want {
			t.Errorf("PathOfNames(%q, %s) = %q, want %q", tt.names, tt.kind, got, tt.want)
		}
		var ancestors []domain.Node
		for _, n := range tt.names[:len(tt.names)-1] {
			ancestors = append(ancestors, domain.Node{Kind: domain.KindPage, Name: n})
		}
		leaf := domain.Node{Kind: tt.kind, Name: tt.names[len(tt.names)-1]}
		if got := domain.PathOf(ancestors, leaf); got != tt.want {
			t.Errorf("PathOf(%q) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

// The hints quote their paths as they are, in double quotes: a title may
// hold U+3000, U+00A0, U+200D and U+00AD, which %q would write as escapes,
// and no " or \, so what the quotes hold is the path (M9/P2 design
// 3.10.4).
func TestTheHintsQuoteThePathAsItIs(t *testing.T) {
	p := "A\u3000B/x\u00a0y/z\u200dw/so\u00adft"
	for _, tt := range []struct{ got, want string }{
		{domain.Quoted(p), `"` + p + `"`},
		{domain.PageHint(p + ".md"), "A page's path ends with .md: \"" + p + ".md\"."},
		{domain.AttachmentHint(p), "That is an attachment: its path is \"" + p + "\"."},
		{domain.TitleHint(p + ".md.md"), "A page there is titled with the .md: its path is \"" + p + ".md.md\"."},
		{domain.ChildrenHint(p + ".md"), p + ".md is a page; list \"" + p + "/\" for its children."},
	} {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}
