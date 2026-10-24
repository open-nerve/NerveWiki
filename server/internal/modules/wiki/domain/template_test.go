package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// The template's two languages say the same (M10/P1 design 3.9): their
// pages split into the same blocks, block by block of one kind, with the
// same inline code. The blocks are what blank lines outside a fence part.

// block is a page's block: its kind, and its inline code, sorted.
type block struct {
	kind string
	code []string
}

// blocksOf splits content into its blocks: a heading (its level), a fence
// (its info string and its lines), a list (its items) or a paragraph.
func blocksOf(content string) []block {
	var (
		blocks []block
		lines  []string
		fence  string
	)
	flush := func() {
		if len(lines) > 0 {
			blocks = append(blocks, blockOf(lines))
			lines = nil
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		switch {
		case fence != "":
			lines = append(lines, line)
			if line == fence {
				fence = ""
				flush()
			}
		case strings.HasPrefix(line, "```"):
			flush()
			fence = strings.TrimRight(line, "abcdefghijklmnopqrstuvwxyz")
			lines = append(lines, line)
		case line == "":
			flush()
		default:
			lines = append(lines, line)
		}
	}
	flush()
	return blocks
}

func blockOf(lines []string) block {
	var kind string
	switch first := lines[0]; {
	case strings.HasPrefix(first, "```"):
		kind = fmt.Sprintf("fence %s, %d lines", strings.TrimLeft(first, "`"), len(lines))
	case strings.HasPrefix(first, "#"):
		kind = fmt.Sprintf("heading %d", len(first)-len(strings.TrimLeft(first, "#")))
	case strings.HasPrefix(first, "- "):
		kind = fmt.Sprintf("list of %d", len(lines))
	default:
		kind = fmt.Sprintf("paragraph of %d lines", len(lines))
	}
	b := block{kind: kind}
	if !strings.HasPrefix(lines[0], "```") {
		b.code = inlineCode(strings.Join(lines, "\n"))
	}
	return b
}

// inlineCode is the text between each pair of backticks, sorted.
func inlineCode(text string) []string {
	parts := strings.Split(text, "`")
	var code []string
	for i := 1; i < len(parts); i += 2 {
		code = append(code, parts[i])
	}
	slices.Sort(code)
	return code
}

// correspond is nil when a and b split into blocks alike, else what
// differs.
func correspond(a, b string) error {
	x, y := blocksOf(a), blocksOf(b)
	if len(x) != len(y) {
		return fmt.Errorf("%d blocks against %d", len(x), len(y))
	}
	for i := range x {
		if x[i].kind != y[i].kind || !slices.Equal(x[i].code, y[i].code) {
			return fmt.Errorf("block %d: %s %q against %s %q", i, x[i].kind, x[i].code, y[i].kind, y[i].code)
		}
	}
	return nil
}

// Every page of a mode's template corresponds in the two languages; the
// comparison sees a paragraph gone and one more at the end (the blocks'
// count), a list item gone and one inline code changed (the code), and,
// the code kept, each block's shape: two list items on one line (a list's
// items), a heading a level down (its level), a fence a line short (a
// fence's lines) and a paragraph in two lines (a paragraph's) (v0.1
// design 13.4 item 1).
func TestTheTemplatesCorrespond(t *testing.T) {
	for _, mode := range []Mode{ModeStandard, ModeLLMWiki} {
		c, _ := ContractOf(mode)
		en, zh := c.Templates[LocaleEn], c.Templates[LocaleZhCN]
		if len(en) != len(zh) {
			t.Fatalf("%s: %d pages in en, %d in zh-CN", mode, len(en), len(zh))
		}
		for i := range en {
			if en[i].Title != zh[i].Title || en[i].SourceRoot != zh[i].SourceRoot {
				t.Errorf("%s: page %d is %q in en, %q in zh-CN", mode, i, en[i].Title, zh[i].Title)
			}
			if err := correspond(en[i].Content, zh[i].Content); err != nil {
				t.Errorf("%s: %s: %v", mode, en[i].Title, err)
			}
		}
	}
	en, zh := template(LocaleEn)[1].Content, template(LocaleZhCN)[1].Content
	generated := "## 生成的页\n\n"
	at := strings.Index(zh, generated)
	if at < 0 {
		t.Fatal("zh-CN's AGENTS has no heading 生成的页")
	}
	end := at + len(generated) + strings.Index(zh[at+len(generated):], "\n\n") + 2
	item := "- `lint` 检查这本 wiki，能直接修的就修。\n"
	for name, changed := range map[string]string{
		"a paragraph gone":       zh[:at+len(generated)] + zh[end:],
		"one more at the end":    zh + "\n多出的一段。\n",
		"a list item gone":       strings.Replace(zh, item, "", 1),
		"an inline code changed": strings.Replace(zh, "`log.md`", "`log`", 1),
		"two items on one line":  strings.Replace(zh, "一次改完。\n- `query`", "一次改完；`query`", 1),
		"a heading a level down": strings.Replace(zh, "## 生成的页\n", "### 生成的页\n", 1),
		"a fence a line short":   strings.Replace(zh, "summary: 一句话说这一页讲什么。\n", "", 1),
		"a paragraph in two lines": strings.Replace(zh, "这一页是双方都遵守的约定。",
			"这一页是双方都遵守的约定。\n", 1),
	} {
		if changed == zh {
			t.Fatalf("%s: the change changed nothing", name)
		}
		if err := correspond(en, changed); err == nil {
			t.Errorf("%s: the two languages correspond, want them told apart", name)
		}
	}
}

// links is what text holds of a link or a tag outside code: [[, ![, ](,
// or # before a letter or a digit, a tag's start.
func links(text string) []string {
	var outside strings.Builder
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if !fenced {
			parts := strings.Split(line, "`")
			for i := 0; i < len(parts); i += 2 {
				outside.WriteString(parts[i])
				outside.WriteString(" ")
			}
			outside.WriteString("\n")
		}
	}
	s := outside.String()
	var found []string
	for _, mark := range []string{"[[", "![", "]("} {
		if strings.Contains(s, mark) {
			found = append(found, mark)
		}
	}
	for i, r := range s {
		if r != '#' {
			continue
		}
		if next, _ := utf8.DecodeRuneInString(s[i+1:]); unicode.IsLetter(next) || unicode.IsDigit(next) {
			found = append(found, "#"+string(next))
		}
	}
	return found
}

// The templates link nothing and tag nothing outside code, so that a new
// LLM Wiki's links and tags are none (M10/P1 design 3.9): a link and a tag
// written in the text are seen.
func TestTheTemplatesLinkNothingOutsideCode(t *testing.T) {
	c, _ := ContractOf(ModeLLMWiki)
	for l, pages := range c.Templates {
		for _, p := range pages {
			if found := links(p.Content); len(found) != 0 {
				t.Errorf("%s: %s links or tags %q outside code", l, p.Title, found)
			}
		}
	}
	agents := template(LocaleEn)[1].Content
	for _, added := range []string{"see [[raw/x]]", "a #ingest of it"} {
		if found := links(agents + "\n" + added + "\n"); len(found) == 0 {
			t.Errorf("%q added: no link or tag found", added)
		}
	}
}

// checkPage is nil for a template's page the server may write: at most
// MaxTemplateBytes of UTF-8 without NUL, ending with a line break, its
// first line a heading, not frontmatter.
func checkPage(content string) error {
	switch {
	case len(content) > MaxTemplateBytes:
		return fmt.Errorf("%d bytes, more than %d", len(content), MaxTemplateBytes)
	case !utf8.ValidString(content) || strings.ContainsRune(content, 0):
		return errors.New("not UTF-8 text without NUL")
	case !strings.HasSuffix(content, "\n"):
		return errors.New("no line break at its end")
	case !strings.HasPrefix(content, "# "):
		return errors.New("its first line is no heading")
	}
	return nil
}

// Each page of the template is small, text, and starts with its heading,
// never frontmatter; the check takes a page of 8 KiB, 00-M10 4.18's bound
// written out here, not read from MaxTemplateBytes, and refuses a byte
// past it and a page that starts with ---.
func TestTheTemplatesAreSmall(t *testing.T) {
	c, _ := ContractOf(ModeLLMWiki)
	for l, pages := range c.Templates {
		for _, p := range pages {
			if err := checkPage(p.Content); err != nil {
				t.Errorf("%s: %s: %v", l, p.Title, err)
			}
		}
	}
	const bound = 8 << 10
	for _, tt := range []struct {
		name, content string
		ok            bool
	}{
		{"8 KiB", "# A\n" + strings.Repeat("a", bound-5) + "\n", true},
		{"a byte past 8 KiB", "# A\n" + strings.Repeat("a", bound-4) + "\n", false},
		{"frontmatter", "---\ntype: concept\n---\n# A\n", false},
	} {
		if err := checkPage(tt.content); (err == nil) != tt.ok {
			t.Errorf("%s (%d bytes): checked as a template's page %t, want %t (%v)", tt.name, len(tt.content), err == nil, tt.ok, err)
		}
	}
}
