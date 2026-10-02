package harden

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// The edges of each replaced or guarded part parse as goldmark parses them.
// The random inputs reach most of them; these name them.
func TestThePartsEdgesParseAsGoldmarkParsesThem(t *testing.T) {
	inputs := map[string][]string{
		"code spans": {
			"`a` b", "``a`", "`a``", "`` a ` b ``", "\\``a`", "a `b\nc` d", "```a``b```", "`a` `` b ``` c",
			"| `a\\|b` |\n|---|\n| c |",
		},
		"raw HTML": {
			"a <!-- b --> c", "a <!-- b", "a <!--> b", "a <!---> b", "a <?b?> c", "a <? b", "a <?> b",
			"a <!A b> c", "a <!A b", "a <![CDATA[ b ]]> c", "a <![CDATA[ b", "<!-- a\nb --> c",
			"<!--x@y.z>", "a <b c=\"<!--\"> d",
		},
		"links": {
			"[a](<b\\>c>)", "[a](<b\\\\>c>)", "[a](<b>c)", "[a](<b\nc>)", "[a](b(c)d)", "[a](b(c)",
			"[a [b](c)](d)", "![a [b](c)](d)", "[a ![b](c)](d)", "[a\n\n](b)", "[a](b \"c\")",
			"[a](b 'c')", "[a](b (c))", "[a](b \"c\nd\")", "[a]: /u\n\n[a][] [a] [b][a] [A]",
			"[a]: /u\n\n[*a*]", "[\n[a](b)", "[a](\n<b>)",
		},
		"autolinks": {
			"foo a@b.c", "foo a.b", "x www.a.b y", "[www.a.b](c)", "_a@b.c_", "*www.a.b*", "(www.a.b)",
			"a.b.c@d.e.", "a@b.c_d", "http://a.b/c?d", "a-b@c.d", "`x`a@b.c",
			"_www.1_www.a", "~www.1.a", "www.a", "www.a.B", "www.-.a/x", "www..a", "www.a.b@c.d", "www.1@a.b",
			"www." + strings.Repeat("1", 255) + ".a", "www." + strings.Repeat("1", 256) + ".a",
			"www." + strings.Repeat("1", 257) + ".a", "www.a.b/" + strings.Repeat(")", 9),
			"a*b@c.d", "a*b*c@d.e", "a*b@c", "a*b@c.d-", "a*b@c.d_", "a_b@-c.d", "a*b@.c", "a`b`@c.d",
			"a*b@c.d.", "a*b@c-.d", "x a*b@c.d.e-f", "a@b.c@d.e", "a@b@c.d",
		},
		"tables": {
			"| `a\\|b` | `c\\|d\\|e` |\n|---|---|\n| `\\|` x `\\|` | y |",
			"| a \\| `b\\|c` |\n|-|\n| `\\|\\|` |", "| `a` \\| b |\n|-|", "|`\\|`|`\\|`|\n|-|-|\n|`\\|`|",
			"| *`a\\|b`* | [`c\\|d`](e) |\n|-|-|", "| `a\nb\\|c` |\n|-|", "|a|\n|:-|\n|b|c|",
			"|a|b|\n|-:|:-:|\n|c|", "a|b\n-|-\nc", "|a|\n| - |\n", "|a|\n|-- -|\n", "|a|\n|:|\n",
			"|a|\n|::-|\n", "|a|\n|-\v|\n", "|a|\n|\t-\t|\n", "|a|\n    |-|\n", "|a|\n|-|-|\n",
			"p\n|a|\n|-|\n|b|", "> |a|\n> |-|\n> |b|", "- |a|\n  |-|\n  |b|",
			// A wider delimiter row in the body is a row: the first one counts.
			"|a|\n|-|\n" + strings.Repeat("|-", 50) + "\n" + strings.Repeat("x\n", 20),
		},
		"emphasis": {
			"*[a*](b)", "*a [b* c](d)", "**a [b](c) d**", "***a***", "**a*", "*a**", "a**b**c",
			"_a_b", "__a__b", "*a _b* c_", "~~a~~", "~a~", "~~a~", "~~~a~~~", "**a ~~b** c~~",
			"*a\n\n*b", "* a *", "*(*a*)*", "_(_a_)_",
		},
		"footnotes": {
			"a[^1] b[^1]\n\n[^1]: c", "a[^1]\n\n[^1]: c\n[^2]: d", "a[^2] b[^1]\n\n[^1]: c\n[^2]: d",
			"a![^1]\n\n[^1]: c", "a[^x]\n\n[^x]: c\n[^x]: d", "a[^1]\n\n[^1]: c\n\n    d",
		},
		"reference definitions": {
			"[a]: /u\n[b]: /v\n\n[a] [b]", "[a]: /u \"t\"\npara [a]", "[a]: /u\n\"t\" junk\n\n[a]",
			"[a]: /u\n[b]: /v\npara", "  [a]: /u\n\n[a]", "[a]:\n/u\n'''t'''\n\n[a]", "[a]: <u v> \"t\"\n\n[a]",
			"[a]: /u 't\n\nx'\n\n[a]", "[a]: /u\n[b]:\n\n[a]",
			"[a]: /u\n\"t\nt\" [b]: /v\n\"t\nt\" [c]: /w\n\n[a] [b] [c]",
			"[a]: /u\n\"t\nt\nt\" [b]: /v\n\"t\nt\" [c]: /w\n\"t\nt\" [d]: /x\n\n[a] [b] [c] [d]",
			"[a]:\n/u\n\"t\nt\" [b]: /v\n\"t\nt\" [c]: /w\n\n[a] [b] [c]",
			"[a]: /" + strings.Repeat("(", MaxDestinationParens+1) + strings.Repeat(")", MaxDestinationParens+1) + "\n\n[a]",
		},
	}
	// CommonMark's label is at most 999 characters; goldmark measures the
	// open labels from the first to the last still open when one closes.
	for _, n := range []int{997, 998, 999, 1000} {
		label := strings.Repeat("a", n)
		def := "[" + label + "]: /u\n\n"
		inputs["labels"] = append(inputs["labels"], def+"["+label+"]", def+"["+label+"][]", def+"[x]["+label+"]",
			"[x]: /u\n\n["+label[1:]+"[x]]", "[x]: /u\n\n[[x]"+label[2:]+"](/v)", "[x]: /u\n\n["+label[3:]+"[[x]")
	}
	h, o := hardened(), original()
	for part, ins := range inputs {
		for _, in := range ins {
			if got, want := render(t, h, in), render(t, o, in); got != want {
				t.Errorf("%s %q\nhardened %q\noriginal %q", part, in, got, want)
			}
		}
	}
}

// isWWW is goldmark's pattern of a "www." link, wherever the pattern
// takes the head of a line.
func TestWWWIsGoldmarksPattern(t *testing.T) {
	pattern := regexp.MustCompile(`^www\.[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?:[/#?][-a-zA-Z0-9@:%_\+.~#!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)
	r := rand.New(rand.NewPCG(20261002, 4))
	const alphabet = "w.aZ1-_@~/ *"
	for range 20000 {
		b := []byte("www.")
		for range r.IntN(300) {
			c := alphabet[r.IntN(len(alphabet))]
			if r.IntN(8) != 0 {
				c = "1a"[r.IntN(2)]
			}
			b = append(b, c)
		}
		if got, want := isWWW(b), pattern.Match(b); got != want {
			t.Fatalf("%q: isWWW %v, goldmark's pattern %v", b, got, want)
		}
	}
	for c := range 256 {
		b := []byte{'w', 'w', 'w', '.', byte(c), '.', 'a'}
		if got, want := isWWW(b), pattern.Match(b); got != want {
			t.Errorf("%q: isWWW %v, goldmark's pattern %v", b, got, want)
		}
	}
}

// isEmailChar is the local part goldmark's linkify accepts: a run of these
// before '@' and a domain is an address.
func TestEmailCharactersAreGoldmarks(t *testing.T) {
	for c := range 256 {
		if c == '@' {
			continue
		}
		b := []byte{byte(c), '@', 'a', '.', 'b'}
		if goldmark := util.FindEmailIndex(b) > 0; goldmark != isEmailChar(byte(c)) {
			t.Errorf("%q: goldmark %v, isEmailChar %v", rune(c), goldmark, isEmailChar(byte(c)))
		}
	}
}

// An opener with no closer in its block turns into text at once: its block
// holds nothing that closes it, the next one does.
func TestAGuardLooksOnlyInItsBlock(t *testing.T) {
	h := hardened()
	if got := render(t, h, "a `b\n\nc` d"); strings.Contains(got, "<code>") {
		t.Errorf("a code span across blocks: %q", got)
	}
	if got := render(t, h, "a <!-- b\n\nc --> d"); strings.Contains(got, "<!--") {
		t.Errorf("a comment across blocks: %q", got)
	}
}

// reached counts the backtick strings that get past the code span guard.
type reached struct{ n int }

func (*reached) Trigger() []byte { return []byte{'`'} }

func (r *reached) Parse(ast.Node, text.Reader, parser.Context) ast.Node {
	r.n++
	return nil
}

// goldmark's code span parser scans to the end of the block for a closer:
// only a backtick string with a string of its length after it in its block
// gets to it. The costs at 256 KB do not tell the scans apart from a linear
// parse (they grow as the size to the power 1.5).
func TestOnlyACodeSpanThatClosesReachesGoldmarks(t *testing.T) {
	tests := []struct {
		src     string
		reached int
	}{
		{"`a``b", 0}, {"a `b\n\nc` d", 0}, {"`a`", 1}, {"``a`b``", 1}, {"`a ``b`", 1}, {"``a` `b", 1},
		{"` `` ``` ````", 0},
	}
	for _, tt := range tests {
		r := &reached{}
		p := NewParser(parser.WithInlineParsers(util.Prioritized(r, 95)))
		p.Parse(text.NewReader([]byte(tt.src)))
		if r.n != tt.reached {
			t.Errorf("%q: %d reached goldmark's parser, want %d", tt.src, r.n, tt.reached)
		}
	}
}
