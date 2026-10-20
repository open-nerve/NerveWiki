package domain_test

import (
	"crypto/sha256"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// fakeRewriter is a LinkRewriter for a plan's tests: a content's links
// lead where subs tell, the text of each before the moves to its text
// after; each parse is the plan's to admit (Admit), then done once a page
// and content, as the plan's parse session keeps facts.
type fakeRewriter struct {
	p         *domain.BatchPlan
	subs      map[string]string
	stale     map[uuid.UUID]bool // pages whose index is not current
	ambiguous bool               // each link leads ambiguously, before and after
	grow      int                // bytes a rewrite adds to what it writes
	calls     []string
	asked     map[string]int // a page and content's parses asked for
	parsed    map[string]int // and done
}

// parse is the parse of content, the page id's, admitted by the plan.
func (f *fakeRewriter) parse(id uuid.UUID, content string) error {
	if err := f.p.Admit(id, content); err != nil {
		return err
	}
	if f.asked == nil {
		f.asked, f.parsed = map[string]int{}, map[string]int{}
	}
	key := id.String() + "\x00" + content
	if f.asked[key]++; f.asked[key] == 1 {
		f.parsed[key]++
	}
	return nil
}

func (f *fakeRewriter) lead() domain.LinksBefore {
	return domain.LinksBefore{Leads: map[int]domain.Lead{0: {ID: uuid.NewV7(), Ambiguous: f.ambiguous}}}
}

func (f *fakeRewriter) Indexed(id uuid.UUID, revision int) (domain.LinksBefore, bool, error) {
	f.calls = append(f.calls, "indexed "+strconv.Itoa(revision))
	return f.lead(), !f.stale[id], nil
}

func (f *fakeRewriter) Resolve(id uuid.UUID, content string, at int) (domain.LinksBefore, error) {
	f.calls = append(f.calls, "resolve "+strconv.Itoa(at))
	if err := f.parse(id, content); err != nil {
		return domain.LinksBefore{}, err
	}
	return f.lead(), nil
}

func (f *fakeRewriter) Rewrite(id uuid.UUID, content string, before domain.LinksBefore, since, until int) (string, error) {
	f.calls = append(f.calls, "rewrite "+strconv.Itoa(since)+" "+strconv.Itoa(until))
	if err := f.parse(id, content); err != nil {
		return "", err
	}
	out := content
	if !before.Leads[0].Ambiguous {
		for from, to := range f.subs {
			out = strings.ReplaceAll(out, from, to)
		}
		out += strings.Repeat("+", f.grow)
	}
	if out != content {
		if err := f.parse(id, out); err != nil { // the read back of what it wrote
			return "", err
		}
	}
	return out, nil
}

// headOf is the head of a page holding content at revision.
func headOf(content string, revision int) domain.PageHead {
	sum := sha256.Sum256([]byte(content))
	return domain.PageHead{Revision: revision, Bytes: len(content), Hash: sum[:]}
}

// contentsOf is the plan's contents' pass with rw, its error fatal.
func contentsOf(t *testing.T, p *domain.BatchPlan, texts map[uuid.UUID]domain.PageText, heads map[uuid.UUID]domain.PageHead,
	rw *fakeRewriter,
) {
	t.Helper()
	rw.p = p
	if err := p.Contents(texts, heads, rw); err != nil {
		t.Fatalf("Contents: %v", err)
	}
}

// The contents' pass (M9/P3 design 3.4.4): tokens against the revision
// read, a write of the bytes there unchanged whatever its token, by the
// head's size and hash; a page read by its head alone; an edit on its
// page's content rewritten first for the moves before it, from the index
// before the batch or from its last content op; the parses' bounds.
func TestBatchPlanContents(t *testing.T) {
	nb := newNotebook("A.md", "P.md", "Q.md", "X.md")
	pID, qID := nb.id(t, "P.md"), nb.id(t, "Q.md")
	text := func(content string, revision int) map[uuid.UUID]domain.PageText {
		return map[uuid.UUID]domain.PageText{pID: {Content: content, Revision: revision}}
	}
	t.Run("an edit's token past", func(t *testing.T) {
		p, _ := planOf(t, nb, with(editOp("P.md", "a", "b"), func(o *domain.BatchArgs) { o.Base = tokenOf(pID, 1) }))
		contentsOf(t, p, text("a", 2), nil, &fakeRewriter{})
		if got := failed(t, p); got != (refusal{"page.revision_mismatch", "", "", 0}) {
			t.Fatalf("Failed = %+v", got)
		}
		if m := mismatchOf(p.Failed); m == nil || m.Why != domain.Changed || m.Current != (domain.VersionToken{Node: pID, Revision: 2}) {
			t.Errorf("the mismatch = %+v", m)
		}
	})
	t.Run("a write's token past, of the bytes there and of others", func(t *testing.T) {
		stale := func(o *domain.BatchArgs) { o.Base = tokenOf(pID, 1) }
		heads := map[uuid.UUID]domain.PageHead{pID: headOf("same", 2)}
		p, _ := planOf(t, nb, with(writeOp("P.md", "same"), stale))
		contentsOf(t, p, nil, heads, &fakeRewriter{})
		if p.Failed != nil || p.Steps[0].Changed {
			t.Errorf("the bytes there: Failed = %v, changed %v; want unchanged", p.Failed, p.Steps[0].Changed)
		}
		p, _ = planOf(t, nb, with(writeOp("P.md", "other"), stale))
		contentsOf(t, p, nil, heads, &fakeRewriter{})
		if got := failed(t, p); got != (refusal{"page.revision_mismatch", "", "", 0}) {
			t.Errorf("other bytes: Failed = %+v", got)
		}
	})
	t.Run("a page read by its head alone", func(t *testing.T) {
		p, _ := planOf(t, nb, with(writeOp("P.md", "new"), func(o *domain.BatchArgs) { o.Base = tokenOf(pID, 2) }), editOp("P.md", "new", "newer"))
		if texts, heads := p.Reads(); len(texts) != 0 || !slices.Equal(heads, []uuid.UUID{pID}) {
			t.Fatalf("Reads = %v, %v", texts, heads)
		}
		contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{pID: headOf("old", 2)}, &fakeRewriter{})
		pages := p.Pages()
		if p.Failed != nil || !p.Steps[0].Changed || len(pages) != 1 || pages[0].Content != "newer" || pages[0].Revision != 2 ||
			pages[0].First != 0 || pages[0].Last != 1 {
			t.Errorf("Failed = %v, pages = %+v", p.Failed, pages)
		}
	})
	t.Run("a write of the bytes there, its page's content op", func(t *testing.T) {
		p, _ := planOf(t, nb, writeOp("P.md", "same"), editOp("P.md", "same", "else"))
		contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{pID: headOf("same", 3)}, &fakeRewriter{})
		pages := p.Pages()
		if p.Failed != nil || p.Steps[0].Changed || len(pages) != 1 || pages[0].First != 0 || pages[0].Content != "else" {
			t.Errorf("Failed = %v, pages = %+v", p.Failed, pages)
		}
		p, _ = planOf(t, nb, writeOp("P.md", "other"))
		contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{pID: headOf("same", 3)}, &fakeRewriter{})
		if got := failed(t, p); got != (refusal{"page.title_taken", "", "", 0}) {
			t.Errorf("a write of other bytes: Failed = %+v", got)
		}
	})
	t.Run("an edit that leaves the bytes as they were", func(t *testing.T) {
		// The CRLF rule makes old_string and new_string the page's CRLF,
		// one string: unchanged, as the edit alone answers (M9/P3 review
		// A-M3); the page's next edit changes it.
		p, _ := planOf(t, nb, editOp("P.md", "a\n", "a\r\n"), editOp("P.md", "b", "c"))
		contentsOf(t, p, text("a\r\nb\r\n", 2), nil, &fakeRewriter{})
		if p.Failed != nil || p.Steps[0].Changed || p.Steps[0].Replacements != 1 || !p.Steps[1].Changed ||
			p.Pages()[0].Content != "a\r\nc\r\n" {
			t.Errorf("Failed = %v, changed %v and %v, replacements %d, content %q", p.Failed, p.Steps[0].Changed, p.Steps[1].Changed,
				p.Steps[0].Replacements, p.Pages()[0].Content)
		}
	})
	t.Run("an edit on its content rewritten for a move before it", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text("see [[A]]", 4), nil, rw)
		if p.Failed != nil || p.Pages()[0].Content != "see [[C]]" || !p.Steps[1].Moved {
			t.Errorf("Failed = %v, content %q, moved %v", p.Failed, p.Pages()[0].Content, p.Steps[1].Moved)
		}
		if want := []string{"indexed 4", "rewrite -1 1"}; !slices.Equal(rw.calls, want) {
			t.Errorf("calls = %q, want %q: before the batch, by its index", rw.calls, want)
		}
	})
	t.Run("its before from its last content op", func(t *testing.T) {
		p, _ := planOf(t, nb, editOp("P.md", "x", "y [[A]]"), moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text("x", 4), nil, rw)
		if p.Failed != nil || p.Pages()[0].Content != "y [[C]]" {
			t.Errorf("Failed = %v, content %q", p.Failed, p.Pages()[0].Content)
		}
		if want := []string{"resolve 0", "rewrite 0 2"}; !slices.Equal(rw.calls, want) {
			t.Errorf("calls = %q, want %q", rw.calls, want)
		}
		// Resolve parsed the content, which Rewrite found parsed.
		if key := pID.String() + "\x00y [[A]]"; rw.asked[key] != 2 || rw.parsed[key] != 1 {
			t.Errorf("the content was asked %d and parsed %d times, want 2 and 1", rw.asked[key], rw.parsed[key])
		}
	})
	// A write after a move that reaches its page is judged on the page as
	// the move leaves it, as separate calls are (M9/P3 review A-I1): its
	// page read whole (ReadWhole), its content rewritten from the index
	// before the batch. 1a: the bytes the move leaves, without a token,
	// unchanged; 1d: with the current token, unchanged; the bytes from
	// before the move, without a token, the page's 409, as write alone
	// answers. A write that no move reaches is judged on its head.
	t.Run("a write after a move that reaches its page", func(t *testing.T) {
		heads := map[uuid.UUID]domain.PageHead{pID: headOf("[[A]] p", 4)}
		for _, tt := range []struct {
			name    string
			base    *string
			content string
			want    refusal
		}{
			{"1a, without a token", nil, "[[B]] p", refusal{}},
			{"1d, with the current token", tokenOf(pID, 4), "[[B]] p", refusal{}},
			{"the bytes from before the move", nil, "[[A]] p", refusal{"page.title_taken", "", "", 1}},
		} {
			p, _ := planOf(t, nb, moveOp("A.md", "B.md"), with(writeOp("P.md", tt.content), func(o *domain.BatchArgs) { o.Base = tt.base }))
			p.ReadWhole(pID)
			if texts, heads := p.Reads(); !slices.Equal(texts, []uuid.UUID{pID}) || len(heads) != 0 {
				t.Fatalf("%s: Reads = %v, %v; want P whole", tt.name, texts, heads)
			}
			rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
			contentsOf(t, p, text("[[A]] p", 4), heads, rw)
			if p.Failed == nil && tt.want != (refusal{}) || p.Failed != nil && failed(t, p) != tt.want {
				t.Errorf("%s: Failed = %v, want %+v", tt.name, p.Failed, tt.want)
			}
			if p.Failed == nil && (p.Steps[1].Changed || !p.Steps[1].Moved || p.Pages()[0].Content != tt.content) {
				t.Errorf("%s: changed %v, moved %v, content %q; want unchanged, its content rewritten first", tt.name, p.Steps[1].Changed,
					p.Steps[1].Moved, p.Pages()[0].Content)
			}
			if want := []string{"indexed 4", "rewrite -1 1"}; !slices.Equal(rw.calls, want) {
				t.Errorf("%s: calls = %q, want %q", tt.name, rw.calls, want)
			}
		}
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), writeOp("P.md", "[[A]] p"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, nil, heads, rw)
		if p.Failed != nil || p.Steps[1].Changed || len(rw.calls) != 0 {
			t.Errorf("a write no move reaches: Failed = %v, changed %v, calls %q; want its head's bytes, unchanged", p.Failed,
				p.Steps[1].Changed, rw.calls)
		}
	})
	t.Run("a move without update_links", func(t *testing.T) {
		p, _ := planOf(t, nb, with(moveOp("A.md", "B.md"), func(o *domain.BatchArgs) { o.UpdateLinks = flag(false) }),
			editOp("P.md", "[[A]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text("see [[A]]", 4), nil, rw)
		if p.Failed != nil || len(rw.calls) != 0 || p.Steps[1].Moved {
			t.Errorf("Failed = %v, calls %q, moved %v", p.Failed, rw.calls, p.Steps[1].Moved)
		}
	})
	t.Run("an index not current", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[A]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}, stale: map[uuid.UUID]bool{pID: true}}
		contentsOf(t, p, text("see [[A]]", 4), nil, rw)
		if p.Failed != nil || p.Pages()[0].Content != "see [[C]]" || !slices.Equal(rw.calls, []string{"indexed 4"}) {
			t.Errorf("Failed = %v, content %q, calls %q", p.Failed, p.Pages()[0].Content, rw.calls)
		}
	})
	t.Run("an ambiguous link before and after", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("X.md", "Y.md"), editOp("P.md", "[[X]]", "[[Z]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[X]]": "[[Y]]"}, ambiguous: true}
		contentsOf(t, p, text("see [[X]]", 4), nil, rw)
		if p.Failed != nil || p.Pages()[0].Content != "see [[Z]]" {
			t.Errorf("Failed = %v, content %q", p.Failed, p.Pages()[0].Content)
		}
	})
	big := "[[A]] " + strings.Repeat("x", domain.MaxContentBytes-len("[[A]] "))
	t.Run("the plan's parses at their bound and past it", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text(big, 4), nil, rw)
		if p.Failed != nil {
			t.Errorf("10 MiB parsed: Failed = %v", p.Failed)
		}
		// A lone page has the plan's bound too, which the unit's bounds do
		// not reach: the read back of a byte more is not parsed.
		p, _ = planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"))
		rw = &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}, grow: 1}
		contentsOf(t, p, text(big, 4), nil, rw)
		if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) || len(rw.parsed) != 1 ||
			!strings.Contains(p.Failed.Error(), "bytes of contents the plan parses") {
			t.Errorf("a lone page a byte more: Failed = %+v (%v), %d contents parsed; want the plan's bound at 1, the first alone parsed",
				got, p.Failed, len(rw.parsed))
		}
		p, _ = planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"), editOp("Q.md", "q", "r"))
		rw = &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}, grow: 1}
		contentsOf(t, p, map[uuid.UUID]domain.PageText{pID: {Content: big, Revision: 4}, qID: {Content: "q", Revision: 1}}, nil, rw)
		if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) {
			t.Errorf("a byte more: Failed = %+v", got)
		}
		if len(rw.parsed) != 1 {
			t.Errorf("%d contents parsed, want the first alone: the parse past the bound is not done", len(rw.parsed))
		}
	})
	t.Run("a content resolved, then rewritten, counted once", func(t *testing.T) {
		// 10 MiB: the content, resolved and rewritten, once, and the read
		// back of what the rewrite wrote; counted twice it is 15.
		edited := "x[[A]] " + strings.Repeat("x", domain.MaxContentBytes-len("x[[A]] "))
		p, _ := planOf(t, nb, editOp("P.md", "x[[A]]", "y[[A]]"), moveOp("A.md", "B.md"), editOp("P.md", "y[[B]]", "y[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text(edited, 4), nil, rw)
		if p.Failed != nil || len(rw.parsed) != 2 {
			t.Errorf("Failed = %v, %d contents parsed, want 2", p.Failed, len(rw.parsed))
		}
	})
	t.Run("the unit's bounds on a batch of more pages than one", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), editOp("P.md", "[[B]]", "[[C]]"), editOp("Q.md", "q", "r"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, map[uuid.UUID]domain.PageText{pID: {Content: big, Revision: 4}, qID: {Content: "q", Revision: 1}}, nil, rw)
		if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) {
			t.Errorf("Failed = %+v, want the unit's bytes at 1", got)
		}
	})
	t.Run("nothing at or after the failed op", func(t *testing.T) {
		p, _ := planOf(t, nb, moveOp("A.md", "B.md"), editOp("Nope.md", "o", "n"), editOp("P.md", "[[B]]", "[[C]]"))
		rw := &fakeRewriter{subs: map[string]string{"[[A]]": "[[B]]"}}
		contentsOf(t, p, text("see [[A]]", 4), nil, rw)
		if got := failed(t, p); got.at != 1 || len(rw.calls) != 0 || len(rw.asked) != 0 {
			t.Errorf("Failed = %+v, calls %q, parses %d", got, rw.calls, len(rw.asked))
		}
	})
}

// mismatchOf is err's *VersionMismatch, nil for none.
func mismatchOf(err error) *domain.VersionMismatch {
	var m *domain.VersionMismatch
	if !errors.As(err, &m) {
		return nil
	}
	return m
}

// The reads' bound (M9/P3 design 3.4.4): the pages read whole, by their
// heads' sizes in their first ops' order, at MaxBatchReadBytes and a byte
// past it, at the op of the page that passes it; pages read by their heads
// alone are not counted, but those whose writes the moves before them
// reach are read whole and counted (ReadWhole, M9/P3 review A-I1).
func TestBatchPlanChecksItsReads(t *testing.T) {
	nb := newNotebook("P1.md", "P2.md", "P3.md")
	ids := []uuid.UUID{nb.id(t, "P1.md"), nb.id(t, "P2.md"), nb.id(t, "P3.md")}
	heads := func(sizes ...int) map[uuid.UUID]domain.PageHead {
		out := map[uuid.UUID]domain.PageHead{}
		for i, n := range sizes {
			out[ids[i]] = domain.PageHead{Revision: 1, Bytes: n}
		}
		return out
	}
	five := domain.MaxContentBytes
	p, _ := planOf(t, nb, editOp("P1.md", "o", "n"), editOp("P2.md", "o", "n"), editOp("P3.md", "o", "n"))
	if failed := p.CheckReads(heads(five, five, domain.MaxBatchReadBytes-2*five)); failed != nil {
		t.Errorf("at the bound: %v", failed)
	}
	p, _ = planOf(t, nb, editOp("P1.md", "o", "n"), editOp("P2.md", "o", "n"), editOp("P3.md", "o", "n"))
	if failed := p.CheckReads(heads(five, five, domain.MaxBatchReadBytes-2*five+1)); failed == nil || failed.Op != 2 || p.Failed != failed {
		t.Errorf("a byte past: %v", failed)
	}
	if err := p.Contents(nil, nil, &fakeRewriter{p: p}); err != nil || p.Failed.Op != 2 {
		t.Errorf("Contents ran past the reads' bound: %v, %v", err, p.Failed)
	}
	write := func(path string, id uuid.UUID) domain.BatchArgs {
		return with(writeOp(path, "x"), func(o *domain.BatchArgs) { o.Base = tokenOf(id, 1) })
	}
	p, _ = planOf(t, nb, write("P1.md", ids[0]), write("P2.md", ids[1]), write("P3.md", ids[2]))
	if failed := p.CheckReads(heads(five, five, five)); failed != nil {
		t.Errorf("heads alone: %v", failed)
	}
	for _, id := range ids {
		p.ReadWhole(id)
	}
	if failed := p.CheckReads(heads(five, five, five)); failed == nil || failed.Op != 2 {
		t.Errorf("writes read whole: %v, want the bound at ops[2]", failed)
	}
}

// The unit's bounds on the plan (M9/P3 design 3.4.4, 3.8): the final
// contents, each page and hash once with the plan's parses, past
// MaxUnitBytes for more pages than one, never for a lone page.
func TestBatchPlanChecksTheUnit(t *testing.T) {
	nb := newNotebook("P.md", "Q.md")
	pID, qID := nb.id(t, "P.md"), nb.id(t, "Q.md")
	four := "x" + strings.Repeat("a", 4<<20-1)
	texts := map[uuid.UUID]domain.PageText{pID: {Content: four, Revision: 1}, qID: {Content: four, Revision: 1}}
	p, _ := planOf(t, nb, editOp("P.md", "x", "y"), editOp("Q.md", "x", "yz"))
	contentsOf(t, p, texts, nil, &fakeRewriter{})
	if err := p.CheckUnit(); err == nil {
		t.Error("two pages a byte past the bytes passed")
	} else if got := refusalOf(t, err); got != (refusal{"page.batch_too_large", "", "", -1}) {
		t.Errorf("two pages past the bytes = %+v", got)
	}
	p, _ = planOf(t, nb, editOp("P.md", "x", "y"), editOp("Q.md", "x", "y"))
	contentsOf(t, p, texts, nil, &fakeRewriter{})
	if err := p.CheckUnit(); err != nil {
		t.Errorf("two pages at the bytes: %v", err)
	}
	big := "x" + strings.Repeat("a", domain.MaxContentBytes-1)
	p, _ = planOf(t, nb, moveOp("P.md", "P2.md"), editOp("Q.md", "x", "y"))
	rw := &fakeRewriter{subs: map[string]string{"x": "z"}}
	contentsOf(t, p, map[uuid.UUID]domain.PageText{nb.id(t, "Q.md"): {Content: big, Revision: 1}}, nil, rw)
	if p.Failed == nil {
		t.Fatal("the edit found its old string rewritten away, and passed")
	}
	p, _ = planOf(t, nb, moveOp("P.md", "P2.md"), editOp("Q.md", "z", "y"))
	rw = &fakeRewriter{subs: map[string]string{"x": "z"}}
	contentsOf(t, p, map[uuid.UUID]domain.PageText{nb.id(t, "Q.md"): {Content: big, Revision: 1}}, nil, rw)
	if p.Failed != nil {
		t.Fatalf("a lone page rewritten: %v", p.Failed)
	}
	if err := p.CheckUnit(); err != nil {
		t.Errorf("a lone page's 15 MiB: %v", err)
	}
}

// The renames and moves between two ops, merged node by node (M9/P3
// design 3.7.1): the first before and the last after; a node moved
// without update_links left out, one back where it was, one deleted; one
// created among them before as created; and the nodes moved without
// update_links, frozen (M9/P3 review B-M1).
func TestBatchPlanMovesBetween(t *testing.T) {
	nb := newNotebook("A.md", "B.md", "F.md", "F/X.md", "W.md")
	p, ids := planOf(t, nb,
		moveOp("F.md", "G.md"),   // 0: a rename
		moveOp("G/X.md", "X.md"), // 1: a move to the root
		with(moveOp("A.md", "A2.md"), // 2: without update_links
			func(o *domain.BatchArgs) { o.UpdateLinks = flag(false) }),
		writeOp("N.md", "n"),      // 3: created
		moveOp("N.md", "A2/N.md"), // 4: and moved
		moveOp("B.md", "B2.md"),   // 5
		moveOp("B2.md", "B.md"),   // 6: back
		moveOp("X.md", "X2.md"),   // 7: X again
		deleteOp("W.md"),          // 8
	)
	if p.Failed != nil {
		t.Fatal(p.Failed)
	}
	f, x, a2 := nb.id(t, "F.md"), nb.id(t, "F/X.md"), nb.id(t, "A.md")
	n := ids[3][0]
	got := p.MovesBetween(-1, 8)
	want := []struct {
		id           uuid.UUID
		beforeParent *uuid.UUID
		beforeName   string
		afterParent  *uuid.UUID
		afterName    string
	}{
		{f, nil, "F", nil, "G"},
		{x, &f, "X", nil, "X2"},
		{n, nil, "N", &a2, "N"},
	}
	if len(got) != len(want) {
		t.Fatalf("MovesBetween = %d changes, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		c := got[i]
		if c.NodeID != w.id || c.Revision != 0 || !domain.SameParent(c.Before.ParentID, w.beforeParent) || c.Before.Name != w.beforeName ||
			!domain.SameParent(c.After.ParentID, w.afterParent) || c.After.Name != w.afterName {
			t.Errorf("change %d = %+v %+v -> %+v, want %+v", i, c.NodeID, *c.Before, *c.After, w)
		}
	}
	if got := p.MovesBetween(3, 8); len(got) != 2 || got[0].NodeID != n || got[0].Before.Name != "N" || got[1].NodeID != x ||
		got[1].Before.Name != "X" || !domain.SameParent(got[1].Before.ParentID, nil) {
		t.Errorf("MovesBetween(3, 8) = %+v", got)
	}
	if got := p.MovesBetween(-1, 0); len(got) != 1 || got[0].NodeID != f {
		t.Errorf("MovesBetween(-1, 0) = %+v", got)
	}
	if got := p.FrozenBetween(-1, 8); !slices.Equal(got, []uuid.UUID{a2}) {
		t.Errorf("FrozenBetween(-1, 8) = %v, want A, moved without update_links", got)
	}
	if got := p.FrozenBetween(2, 8); len(got) != 0 {
		t.Errorf("FrozenBetween(2, 8) = %v, want none after A's move", got)
	}
	if got := p.FrozenBetween(-1, 1); len(got) != 0 {
		t.Errorf("FrozenBetween(-1, 1) = %v, want none before A's move", got)
	}
}

// The plan's tree after each op (M9/P3 design 3.4.1, 3.7.4): its nodes by
// key and by id with their paths, a subtree, and the nodes deleted by
// then.
func TestBatchPlanTreeAt(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "A/x.png", "F.md")
	p, ids := planOf(t, nb, moveOp("A.md", "F/C.md"), writeOp("F/C/N.md", "n"), deleteOp("F/C/x.png"))
	if p.Failed != nil {
		t.Fatal(p.Failed)
	}
	a, b := nb.id(t, "A.md"), nb.id(t, "A/B.md")
	names := func(path []domain.PathStep) string {
		var out []string
		for _, s := range path {
			out = append(out, s.Name)
		}
		return strings.Join(out, "/")
	}
	before := p.TreeAt(-1)
	if got := before.ByKeys([]string{"a"}); len(got) != 1 || got[0].ID != a || names(got[0].Path) != "A" {
		t.Errorf("ByKeys(a) before = %+v", got)
	}
	after := p.TreeAt(0)
	if got := after.ByKeys([]string{"a"}); len(got) != 0 {
		t.Errorf("ByKeys(a) after the move = %+v", got)
	}
	if got := after.Paths([]uuid.UUID{b}); len(got) != 1 || names(got[0].Path) != "F/C/B" || got[0].Path[1].Key != "c" {
		t.Errorf("Paths(B) after the move = %+v", got)
	}
	if got := p.TreeAt(1).Paths([]uuid.UUID{ids[1][2]}); len(got) != 1 || names(got[0].Path) != "F/C/N" {
		t.Errorf("Paths(N) once created = %+v", got)
	}
	if got := after.Subtree(a); len(got) != 3 || got[0].ID != a {
		t.Errorf("Subtree(A) = %+v", got)
	}
	if got := after.Paths([]uuid.UUID{nb.id(t, "A/x.png")}); len(got) != 1 || !got[0].Asset {
		t.Errorf("Paths(x.png) = %+v", got)
	}
	last, x := p.TreeAt(2), nb.id(t, "A/x.png")
	if gone := last.Gone(); !slices.Equal(gone, []uuid.UUID{x}) || len(last.Paths([]uuid.UUID{x})) != 0 || len(after.Gone()) != 0 {
		t.Errorf("Gone after the delete = %v, before %v", gone, after.Gone())
	}
	if got := last.Subtree(a); len(got) != 3 { // A, B and N
		t.Errorf("Subtree(A) after the delete = %+v", got)
	}
}
