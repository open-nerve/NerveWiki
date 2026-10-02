package harden

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// hardened is the parser of this package with goldmark's renderers for its
// nodes; original is goldmark with the same syntax as it comes. Raw HTML is
// rendered so that the two are compared on it too.
func hardened() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParser(NewParser()),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(extension.NewTableHTMLRenderer(), 500),
				util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
				util.Prioritized(extension.NewTaskCheckBoxHTMLRenderer(), 500),
				util.Prioritized(extension.NewFootnoteHTMLRenderer(), 500),
			),
		),
	)
}

func original() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
}

func render(t testing.TB, m goldmark.Markdown, src string) string {
	t.Helper()
	var b bytes.Buffer
	if err := m.Convert([]byte(src), &b); err != nil {
		t.Fatalf("convert %q: %v", src, err)
	}
	return b.String()
}

// goldmarkDir is goldmark's directory in the module cache: its examples are
// read from there, not copied into the repository.
func goldmarkDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/yuin/goldmark").Output()
	if err != nil {
		t.Fatalf("go list goldmark: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestTheExamplesParseAsGoldmarkParsesThem compares the two on CommonMark's
// examples and goldmark's examples of its extensions.
func TestTheExamplesParseAsGoldmarkParsesThem(t *testing.T) {
	dir := goldmarkDir(t)
	var inputs []string
	raw, err := os.ReadFile(filepath.Join(dir, "_test", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec []struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	for _, c := range spec {
		inputs = append(inputs, c.Markdown)
	}
	if len(inputs) < 600 {
		t.Fatalf("spec.json has %d examples", len(inputs))
	}
	for _, f := range []string{"footnote.txt", "linkify.txt", "strikethrough.txt", "table.txt", "tasklist.txt"} {
		inputs = append(inputs, caseFile(t, filepath.Join(dir, "extension", "_test", f))...)
	}
	h, o := hardened(), original()
	for _, in := range inputs {
		if got, want := render(t, h, in), render(t, o, in); got != want {
			t.Errorf("input %q\nhardened %q\noriginal %q", in, got, want)
		}
	}
}

// caseFile reads the Markdown of each case in a goldmark test file: a
// header, options maybe, the separator, the Markdown, the separator, the
// HTML, the case separator.
func caseFile(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	const sep = "//- - - - - - - - -//"
	var out []string
	var buf []string
	in := false
	s := bufio.NewScanner(bytes.NewReader(raw))
	for s.Scan() {
		line := s.Text()
		switch {
		case line == sep && !in:
			in, buf = true, nil
		case line == sep && in:
			out = append(out, strings.Join(buf, "\n"))
			in = false
		case in:
			buf = append(buf, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s: no cases", name)
	}
	return out
}

// pieces are what the random inputs are made of: every construct the
// hardening touches, its openers and closers apart.
func pieces() []string {
	return []string{
		"*", "**", "***", "_", "__", "~", "~~", "~~~", "`", "``", "[", "]", "(", ")", "![", "](", "][", "<", ">",
		"<!--", "-->", "<?", "?>", "<!A", "<![CDATA[", "]]>", "<b>", "</b>", `<a href="x">`, "\\", "\\>", "\\(",
		"a", "b ", " ", "  ", "\t", "\n", "\n\n", "> ", "- ", "1. ", "* ", "# ", "|", "|-|", "- [ ] ", "- [x] ",
		"[^1]", "[^1]: ", "[^x]", "[^x]: note", "[x]: /u", "[x]: <u> \"t\"", "[x]", "[x][]", "[x][x]",
		"http://a.b", "www.a.b", "a@b.c", "x.y@a.b", "&amp;", "&#42;", "\"", "'", ":", "    ", "```", "~~~\n",
		"a*", "*a", "_a_", "x_y", "a**b", "c* ", "(<u>)", "[a](b \"c\")", "[a](<b>)", "[a](b(c)d)", "中文",
	}
}

func randomInput(r *rand.Rand, pieces []string) string {
	var b strings.Builder
	for range 1 + r.IntN(40) {
		b.WriteString(pieces[r.IntN(len(pieces))])
	}
	return b.String()
}

// TestRandomInputsParseAsGoldmarkParsesThem compares the two on inputs made
// of the pieces, with a fixed seed: none nests near the limits.
func TestRandomInputsParseAsGoldmarkParsesThem(t *testing.T) {
	r := rand.New(rand.NewPCG(20261002, 3))
	h, o, ps := hardened(), original(), pieces()
	n := 50000
	if testing.Short() {
		n = 5000
	}
	fails := 0
	for range n {
		in := randomInput(r, ps)
		if got, want := render(t, h, in), render(t, o, in); got != want {
			t.Errorf("input %q\nhardened %q\noriginal %q", in, got, want)
			if fails++; fails == 10 {
				t.FailNow()
			}
		}
	}
}
