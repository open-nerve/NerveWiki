package harden

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func emailGuardBlock(src []byte) *ast.Paragraph {
	p := ast.NewParagraph()
	from := 0
	for _, line := range strings.SplitAfter(string(src), "\n") {
		if len(line) != 0 {
			p.Lines().Append(text.NewSegment(from, from+len(line)))
			from += len(line)
		}
	}
	return p
}

func TestEmailGuardIndexesOnlyLinesThatCanContainEmail(t *testing.T) {
	const line = "[](asset.bin) words_and_symbols\n"
	src := []byte(strings.Repeat(line, 10000))
	ix := indexOf(emailGuardBlock(src), src, parser.NewContext())
	if len(ix.emails) != 0 {
		t.Fatalf("line with no possible email built %d candidates", len(ix.emails))
	}
	// An email on a later line still gets its complete original index. The
	// previous 10k lines do not gain candidate storage because of that tail.
	src = append(src, []byte("mail@example.test\n")...)
	ix = indexOf(emailGuardBlock(src), src, parser.NewContext())
	if len(ix.emails) != 2 || ix.emails[0] != (email{len(line) * 10000, len(line)*10000 + 4, true}) {
		t.Fatalf("email tail lost or unrelated lines indexed: %+v", ix.emails)
	}
}

type emailGuardInner struct{ calls int }

func (*emailGuardInner) Trigger() []byte { return []byte{' '} }
func (p *emailGuardInner) Parse(ast.Node, text.Reader, parser.Context) ast.Node {
	p.calls++
	return nil
}

func TestEmailGuardDoesNotFallBackOutsideIndexedRuns(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		calls      int
	}{
		{"plain", "words_without_address", 0},
		{"invalid", "local@no-dot", 0},
		{"later-line", "plain\nmail@example.test", 0},
		{"email", "mail@example.test", 1},
		{"email-after-space", " mail@example.test", 1},
		{"url", "https://example.test", 1},
		{"www", "www.example.test", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.text)
			inner := &emailGuardInner{}
			guard := &linkify{inner: inner}
			guard.Parse(emailGuardBlock(src), text.NewReader(src), parser.NewContext())
			if inner.calls != tc.calls {
				t.Fatalf("inner parser calls=%d want%d", inner.calls, tc.calls)
			}
			if got, want := render(t, hardened(), tc.text), render(t, original(), tc.text); got != want {
				t.Fatalf("email guard rendering differs: got%q want%q", got, want)
			}
		})
	}
}
