package harden

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Observe the actual registered parser's returned nodes. Keeping their
// pointers and block starts prevents allocator reuse from faking sharing.
// No allocation counter or observer is added to the product.
type linkArenaTrace struct {
	links
	t         *testing.T
	installed int
	images    int
	used      map[*ast.Link]bool
	blocks    map[*ast.Link]int
}

func (r *linkArenaTrace) SetParserOption(c *parser.Config) {
	for i := range c.InlineParsers {
		if _, ok := c.InlineParsers[i].Value.(links); ok {
			c.InlineParsers[i].Value = r
			r.installed++
		}
	}
}

func (r *linkArenaTrace) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before, _ := pc.Get(linkArenaKey).(*linkArena)
	var head *ast.Link
	cursor := 0
	if before != nil {
		head, cursor = &before.block[0], before.next
	}
	n := r.links.Parse(parent, block, pc)
	a, _ := pc.Get(linkArenaKey).(*linkArena)
	switch n := n.(type) {
	case *ast.Image:
		if a != before || a != nil && (a.next != cursor || &a.block[0] != head) {
			r.t.Fatal("an image temporary created or advanced the Link arena")
		}
		r.images++
		return n
	case *ast.Link:
		if a == nil || a.next < 1 || a.next > len(a.block) {
			r.t.Fatal("a successful real Link did not consume an arena slot")
		}
		last := &a.block[a.next-1]
		if n != last {
			r.t.Fatal("the real returned Link is not the newly acquired slot")
		}
		if r.used[last] {
			r.t.Fatal("a real Link reused a previously returned slot")
		}
		r.used[last] = true
		r.blocks[&a.block[0]] = len(a.block)
	}
	return n
}

func linkArenaMarkdown(t *testing.T) (goldmark.Markdown, *linkArenaTrace) {
	t.Helper()
	m := hardened()
	r := &linkArenaTrace{t: t, used: map[*ast.Link]bool{}, blocks: map[*ast.Link]int{}}
	m.Parser().AddOptions(r)
	if r.installed != 1 {
		t.Fatalf("forwarding observer replaced %d links parsers, want 1", r.installed)
	}
	return m, r
}

// Expectations come from the bytes written here, never from a parser,
// destination lookup, allocation helper or sort. All three constructors
// are reached: inline, full reference and shortcut reference, with images.
func linkArenaDocument(count int) (string, string, []labelSlotDestination) {
	var source strings.Builder
	var html []string
	var want []labelSlotDestination
	for i := range count {
		label, target, title := fmt.Sprintf("label-%04d", i), fmt.Sprintf("doc-%04d.md", i), fmt.Sprintf("title-%04d", i)
		pos := source.Len()
		kind := ast.KindLink
		start, stop := 0, 0
		switch i % 6 {
		case 0, 1:
			prefix := "[" + label + "]("
			if i%6 == 1 {
				prefix, kind = "!"+prefix, ast.KindImage
			}
			start, stop = pos+len(prefix), pos+len(prefix)+len(target)
			source.WriteString(prefix + target + " \"" + title + "\")\n")
		case 2, 3:
			if i%6 == 3 {
				source.WriteByte('!')
				kind = ast.KindImage
			}
			fmt.Fprintf(&source, "[%s][ref-%04d]\n", label, i)
		case 4, 5:
			if i%6 == 5 {
				source.WriteByte('!')
				kind = ast.KindImage
			}
			label = fmt.Sprintf("ref-%04d", i)
			source.WriteString("[" + label + "]\n")
		}
		if kind == ast.KindImage {
			html = append(html, "<img src=\""+target+"\" alt=\""+label+"\" title=\""+title+"\">")
		} else {
			html = append(html, "<a href=\""+target+"\" title=\""+title+"\">"+label+"</a>")
		}
		want = append(want, labelSlotDestination{kind, pos, target, start, stop})
	}
	source.WriteByte('\n')
	for i := range count {
		if i%6 < 2 {
			continue
		}
		prefix, target := fmt.Sprintf("[ref-%04d]: ", i), fmt.Sprintf("doc-%04d.md", i)
		want[i].start, want[i].stop = source.Len()+len(prefix), source.Len()+len(prefix)+len(target)
		source.WriteString(prefix + target + fmt.Sprintf(" \"title-%04d\"\n", i))
	}
	return source.String(), "<p>" + strings.Join(html, "\n") + "</p>\n", want
}

func TestLinkArenaKeepsCompleteTreesAcrossBlocks(t *testing.T) {
	for _, count := range []int{1, 1025, 10001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m, trace := linkArenaMarkdown(t)
			pc := parser.NewContext()
			source, wantHTML, wantDestinations := linkArenaDocument(count)
			root, gotHTML := labelSlotParse(t, m, pc, source)
			if gotHTML != wantHTML || gotHTML != render(t, original(), source) {
				t.Fatal("the complete real tree differs from literal HTML or original Goldmark")
			}
			if got := labelSlotDestinations(t, root, pc); !slices.Equal(got, wantDestinations) {
				t.Fatalf("complete destination positions differ: got %d nodes, want %d", len(got), count)
			}
			links := 0
			for _, expected := range wantDestinations {
				if expected.kind == ast.KindLink {
					links++
				}
			}
			if len(trace.used) != links || trace.images != count-links {
				t.Fatalf("real constructors acquired %d Link nodes / %d images, want %d / %d", len(trace.used), trace.images, links, count-links)
			}
			capacity := 0
			for _, size := range trace.blocks {
				if size > 256 {
					t.Fatalf("a real node belongs to an unbounded block of %d slots", size)
				}
				capacity += size
			}
			// A loose structural bound, not a copy of the growth algorithm:
			// many actual nodes must share storage, with at most one tail.
			if len(trace.blocks) > 16+(links+63)/64 || capacity > links+255 {
				t.Fatalf("%d real nodes used %d blocks / %d slots", count, len(trace.blocks), capacity)
			}
			if count == 1 && capacity != 1 {
				t.Fatalf("one real link reserved %d slots, want 1", capacity)
			}
		})
	}
}

func TestLinkArenaKeepsOpenAndFailedLabelsIndependent(t *testing.T) {
	m, trace := linkArenaMarkdown(t)
	pc := parser.NewContext()
	const tail = "[seed](s) [outer [middle ![inner](one) [ok](two)\n\n[last](three)\n"
	prefix := strings.Repeat("[fill](keep)\n", 300) + "\n"
	source := prefix + tail
	root, got := labelSlotParse(t, m, pc, source)
	const tailHTML = "<p><a href=\"s\">seed</a> [outer [middle <img src=\"one\" alt=\"inner\"> <a href=\"two\">ok</a></p>\n<p><a href=\"three\">last</a></p>\n"
	if !strings.HasSuffix(got, tailHTML) || got != render(t, original(), source) {
		t.Fatal("open labels or their later successful links changed after many earlier nodes")
	}
	destinations := labelSlotDestinations(t, root, pc)
	if len(destinations) != 304 || len(trace.used) != 303 || trace.images != 1 {
		t.Fatalf("got %d real nodes / %d constructor slots, want 304 / 303 (the image uses no slot)", len(destinations), len(trace.used))
	}
	want := []labelSlotDestination{
		{ast.KindLink, 0, "s", 7, 8}, {ast.KindImage, 25, "one", 34, 37},
		{ast.KindLink, 39, "two", 44, 47}, {ast.KindLink, 50, "three", 57, 62},
	}
	for i := range want {
		want[i].pos += len(prefix)
		want[i].start += len(prefix)
		want[i].stop += len(prefix)
	}
	if !slices.Equal(destinations[300:], want) {
		t.Fatalf("tail destinations=%+v want=%+v", destinations[300:], want)
	}
	for _, source := range []string{"plain text", "[open [more", "[bad](<open", "[missing][reference]"} {
		fresh := parser.NewContext()
		_, got := labelSlotParse(t, m, fresh, source)
		if got != render(t, original(), source) || fresh.Get(linkArenaKey) != nil {
			t.Fatalf("unsuccessful syntax allocated a Link arena or changed rendering: %q", source)
		}
	}
}

func TestLinkArenaDoesNotRetainImageTemporaries(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		images       int
	}{
		{"tiny", "![only](one)", 1},
		{"inline_full_and_shortcut", strings.Repeat("![a](inline) ![b][r] ![r]\n", 513) + "\n[r]: referred\n", 1539},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, trace := linkArenaMarkdown(t)
			pc := parser.NewContext()
			root, got := labelSlotParse(t, m, pc, tc.source)
			if got != render(t, original(), tc.source) {
				t.Fatal("real inline/reference image rendering differs from original Goldmark")
			}
			if pc.Get(linkArenaKey) != nil || len(trace.used) != 0 || len(trace.blocks) != 0 || trace.images != tc.images {
				t.Fatalf("%d real images created arena state or acquired slots", tc.images)
			}
			if got := len(labelSlotDestinations(t, root, pc)); got != tc.images {
				t.Fatalf("retained %d real image destinations, want %d", got, tc.images)
			}
		})
	}
}

func TestLinkArenaRetainsOldTreesAcrossContextUse(t *testing.T) {
	m, _ := linkArenaMarkdown(t)
	pc := parser.NewContext()
	source, wantHTML, wantDestinations := linkArenaDocument(1025)
	first, _ := labelSlotParse(t, m, pc, source)
	lookup := Destinations(pc)
	for _, context := range []parser.Context{pc, parser.NewContext(), pc} {
		const next = "[next](different) ![image](other) [empty]()"
		_, got := labelSlotParse(t, m, context, next)
		if want := "<p><a href=\"different\">next</a> <img src=\"other\" alt=\"image\"> <a href=\"\">empty</a></p>\n"; got != want {
			t.Fatalf("later tree=%q want=%q", got, want)
		}
	}
	if got := labelSlotRender(t, m, []byte(source), first); got != wantHTML {
		t.Fatal("a retained tree changed after reused or independent parse contexts")
	}
	if got := labelSlotDestinations(t, first, pc); !slices.Equal(got, wantDestinations) {
		t.Fatal("a retained tree's destinations changed after other parses")
	}
	for _, n := range writtenCompactNodes(first) {
		if _, ok := lookup(n); !ok {
			t.Fatal("the original destination closure lost a retained node")
		}
	}
}

func TestLinkArenaSeparatesConcurrentParseContexts(t *testing.T) {
	m := hardened()
	m.Parser().Parse(text.NewReader(nil)) // initialise the shared parser before concurrent calls
	sources := []string{strings.Repeat("[one](first)\n", 1025), strings.Repeat("[two](second)\n", 1025)}
	roots := make([]ast.Node, len(sources))
	contexts := make([]parser.Context, len(sources))
	var workers sync.WaitGroup
	for i, source := range sources {
		contexts[i] = parser.NewContext()
		workers.Go(func() {
			roots[i] = m.Parser().Parse(text.NewReader([]byte(source)), parser.WithContext(contexts[i]))
		})
	}
	workers.Wait()
	if contexts[0].Get(linkArenaKey) == contexts[1].Get(linkArenaKey) {
		t.Fatal("independent real parses shared their Link arena")
	}
	for i, root := range roots {
		if got, want := labelSlotRender(t, m, []byte(sources[i]), root), render(t, original(), sources[i]); got != want {
			t.Fatalf("concurrent context %d did not retain its own complete tree", i)
		}
		if got := len(labelSlotDestinations(t, root, contexts[i])); got != 1025 {
			t.Fatalf("concurrent context %d retained %d nodes, want 1025", i, got)
		}
	}
}
