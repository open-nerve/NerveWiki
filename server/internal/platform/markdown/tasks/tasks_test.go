package tasks_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

func newMarkdown(t testing.TB) *markdown.Markdown {
	t.Helper()
	m, err := markdown.New([]markdown.Extension{tasks.Extension()})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func extracted(t testing.TB, m *markdown.Markdown, src []byte) []tasks.Task {
	t.Helper()
	got, ok := m.Parse(src).Extracted(tasks.Name).([]tasks.Task)
	if !ok {
		t.Fatalf("extracted %T", m.Parse(src).Extracted(tasks.Name))
	}
	return got
}

// Every fixture's tasks are what its JSON says, none when it says none
// (the fixture set's rule 11).
func TestTheFixturesTasksAreTheirs(t *testing.T) {
	m := newMarkdown(t)
	found := 0
	for _, f := range markdowntest.Fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			var want struct {
				Tasks []tasks.Task `json:"tasks"`
			}
			if err := json.Unmarshal(f.JSON, &want); err != nil {
				t.Fatal(err)
			}
			found += len(want.Tasks)
			if got := extracted(t, m, f.Content); !slices.Equal(got, want.Tasks) {
				t.Errorf("tasks %v, want %v", got, want.Tasks)
			}
		})
	}
	if found < 10 {
		t.Errorf("the fixtures hold %d tasks", found)
	}
}

// The checkbox is goldmark's, disabled for everyone, with its position; the
// HTML passes the check with the extension's markup, and not without.
func TestTheCheckboxCarriesItsPosition(t *testing.T) {
	m := newMarkdown(t)
	src := []byte("- [ ] a\n- [x] b\n")
	got, err := m.Render(context.Background(), m.Parse(src), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	want := "<ul>\n<li><input disabled=\"\" type=\"checkbox\" data-task=\"3\"> a</li>\n" +
		"<li><input checked=\"\" disabled=\"\" type=\"checkbox\" data-task=\"11\"> b</li>\n</ul>\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if err := markdowntest.CheckHTML(got, tasks.Extension()); err != nil {
		t.Errorf("with the extension's markup: %v", err)
	}
	if err := markdowntest.CheckHTML(got); err == nil {
		t.Error("the checkbox passed without the extension's markup")
	}
}

// ours is the hardened parser with the extension, rendered as goldmark
// renders the rest; theirs is goldmark's GFM, task lists and all.
func ours(src []byte) (string, []tasks.Task, error) {
	ext := tasks.Extension()
	root := harden.NewParser(ext.Parser...).Parse(text.NewReader(src))
	r := renderer.NewRenderer(renderer.WithNodeRenderers(append([]util.PrioritizedValue{
		util.Prioritized(html.NewRenderer(html.WithUnsafe()), 1000),
		util.Prioritized(extension.NewTableHTMLRenderer(), 500),
		util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
		util.Prioritized(extension.NewFootnoteHTMLRenderer(), 500),
	}, ext.Renderer(nil)...)...))
	var b bytes.Buffer
	err := r.Render(&b, src, root)
	got, _ := ext.Extract(root, src).([]tasks.Task)
	return b.String(), got, err
}

func theirs(src []byte) (string, error) {
	var b bytes.Buffer
	err := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithRendererOptions(html.WithUnsafe())).Convert(src, &b)
	return b.String(), err
}

var position = regexp.MustCompile(` data-task="(\d+)"`)

// sameAsGoldmark checks that src renders as goldmark renders it but for the
// checkboxes' positions, and that each position is a task's character, in
// the order Extract gives.
func sameAsGoldmark(t *testing.T, src string) bool {
	t.Helper()
	got, found, err := ours([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want, err := theirs([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var shown []tasks.Task
	for _, m := range position.FindAllStringSubmatchIndex(got, -1) {
		o, _ := strconv.Atoi(got[m[2]:m[3]])
		box := got[strings.LastIndex(got[:m[0]], "<input"):m[0]]
		shown = append(shown, tasks.Task{Offset: o, Checked: strings.Contains(box, "checked")})
	}
	ok := true
	if stripped := position.ReplaceAllString(got, ""); stripped != want {
		t.Errorf("input %q\nours   %q\ntheirs %q", src, stripped, want)
		ok = false
	}
	if !slices.Equal(shown, found) {
		t.Errorf("input %q: rendered %v, extracted %v", src, shown, found)
		ok = false
	}
	for _, task := range found {
		o := task.Offset
		if o < 1 || o+1 >= len(src) || src[o-1] != '[' || src[o+1] != ']' ||
			task.Checked != (src[o] == 'x' || src[o] == 'X') {
			t.Errorf("input %q: %+v is not a task's character", src, task)
			ok = false
		}
	}
	return ok
}

// The extension finds the task items goldmark finds, on goldmark's examples
// of task lists and on CommonMark's of lists.
func TestTheExamplesParseAsGoldmarkParsesThem(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/yuin/goldmark").Output()
	if err != nil {
		t.Fatalf("go list goldmark: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	inputs := caseFile(t, filepath.Join(dir, "extension", "_test", "tasklist.txt"))
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
	tasksSeen := 0
	for _, in := range inputs {
		sameAsGoldmark(t, in)
		_, found, _ := ours([]byte(in))
		tasksSeen += len(found)
	}
	if tasksSeen < 5 {
		t.Errorf("the examples hold %d tasks", tasksSeen)
	}
}

// caseFile reads the Markdown of each case in a goldmark test file: the
// Markdown is between the first two separators of a case.
func caseFile(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	const sep = "//- - - - - - - - -//"
	var out, buf []string
	in := false
	for s := bufio.NewScanner(bytes.NewReader(raw)); s.Scan(); {
		switch line := s.Text(); {
		case line == sep && !in:
			in, buf = true, nil
		case line == sep:
			out, in = append(out, strings.Join(buf, "\n")), false
		case in:
			buf = append(buf, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s: no cases", name)
	}
	return out
}

// The extension finds the task items goldmark finds on inputs made of
// lists, quotes, brackets and what may follow them, with a fixed seed.
func TestRandomInputsParseAsGoldmarkParsesThem(t *testing.T) {
	pieces := []string{
		"- ", "* ", "1. ", "2) ", "> ", "  ", "    ", "\t", " ", "\n", "\n\n", "\r\n", "a", "中",
		"[", "]", "[ ]", "[x]", "[X]", "[\t]", "[-]", "[]", "[xx]", "- [ ] ", "- [x] ", "- [X]",
		"`", "*", "\\", "(u)", ": /u", "[x]: /u\n", "[^1]", "[^1]: ", "|", "|-|", "<b>", "```\n", "#",
	}
	r := rand.New(rand.NewPCG(20261004, 6))
	n := 20000
	if testing.Short() {
		n = 2000
	}
	fails, tasksSeen := 0, 0
	for range n {
		var b strings.Builder
		for range 1 + r.IntN(30) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		if !sameAsGoldmark(t, b.String()) {
			if fails++; fails == 10 {
				t.FailNow()
			}
		}
		_, found, _ := ours([]byte(b.String()))
		tasksSeen += len(found)
	}
	if tasksSeen < n/5 {
		t.Errorf("%d inputs hold %d tasks", n, tasksSeen)
	}
}
