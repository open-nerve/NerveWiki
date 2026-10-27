package domain_test

import (
	"bytes"
	"crypto/sha256"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A final merged content cannot stand in for each item: the creation
// writes A, its same-byte write changes nothing, and the outside edit
// later writes BBB without adding another source change.
func TestBatchSourceFactsKeepEachContentStep(t *testing.T) {
	nb := newNotebook("raw.md")
	raw := nb.id(t, "raw.md")
	p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw},
		writeOp("raw/new/leaf.md", "A"), writeOp("raw/new/leaf.md", "A"),
		moveOp("raw/new/leaf.md", "leaf.md"), editOp("leaf.md", "A", "BBB"))
	contentsOf(t, p, nil, nil, &fakeRewriter{})
	if p.Failed != nil {
		t.Fatalf("plan failed: %v", p.Failed)
	}
	if len(p.Steps[0].Facts) != 2 {
		t.Fatalf("creation facts = %+v, want new then leaf", p.Steps[0].Facts)
	}
	for i, want := range []struct {
		content string
		merged  int
	}{{"", 0}, {"A", 1}} {
		fact := p.Steps[0].Facts[i]
		if fact.Operation != domain.OpCreate || len(fact.Nodes) != 1 || fact.Nodes[0].Kind != domain.KindPage || fact.Nodes[0].Before != nil || !fact.Nodes[0].After.InArea {
			t.Errorf("creation %d = %+v, want a source page creation", i, fact)
		}
		checkSourceContent(t, fact, want.content, 0, want.merged)
	}
	if len(p.Steps[1].Facts) != 1 || len(p.Steps[2].Facts) != 1 || len(p.Steps[3].Facts) != 1 {
		t.Fatalf("later facts = %+v, %+v, %+v", p.Steps[1].Facts, p.Steps[2].Facts, p.Steps[3].Facts)
	}
	checkSourceContent(t, p.Steps[1].Facts[0], "A", 1, 1)
	move := p.Steps[2].Facts[0]
	if move.Operation != domain.OpMove || len(move.Nodes) != 1 || !move.Nodes[0].Before.InArea || move.Nodes[0].After.InArea {
		t.Errorf("move facts = %+v, want source to outside", move)
	}
	last := p.Steps[3].Facts[0]
	checkSourceContent(t, last, "BBB", 1, 1)
	if last.Nodes[0].Before.InArea || last.Nodes[0].After.InArea {
		t.Errorf("outside edit marked source: %+v", last.Nodes[0])
	}
	if pages := p.Pages(); len(pages) != 1 || pages[0].Content != "BBB" || pages[0].Merged != 1 {
		t.Errorf("pages = %+v, want final BBB and one source change", pages)
	}
}

// A source page read only by its head still has an instantaneous before
// size; its same-byte second write must not consume another change.
func TestBatchSourceContentFactsReadHeadBeforeBytes(t *testing.T) {
	nb := newNotebook("raw.md", "raw/empty.md")
	raw, page := nb.id(t, "raw.md"), nb.id(t, "raw/empty.md")
	p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw},
		with(writeOp("raw/empty.md", "first"), func(o *domain.BatchArgs) { o.Base = tokenOf(page, 1) }),
		writeOp("raw/empty.md", "first"))
	contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{page: headOf("", 1)}, &fakeRewriter{})
	if p.Failed != nil {
		t.Fatalf("plan failed: %v", p.Failed)
	}
	for i, before := range []int{0, 5} {
		if len(p.Steps[i].Facts) != 1 {
			t.Fatalf("step %d facts = %+v", i, p.Steps[i].Facts)
		}
		checkSourceContent(t, p.Steps[i].Facts[0], "first", before, 1)
	}
}

// SourceRootID can be preallocated in standard mode; it must not turn
// ordinary user edits into source changes. The root's own body is also
// outside in LLM mode.
func TestBatchSourceMergedIgnoresStandardAndRootBodies(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode domain.Mode
		path string
	}{{"standard preset root", domain.ModeStandard, "raw/page.md"}, {"source root body", domain.ModeLLMWiki, "raw.md"}} {
		t.Run(tt.name, func(t *testing.T) {
			nb := newNotebook("raw.md", "raw/page.md")
			raw, page := nb.id(t, "raw.md"), nb.id(t, tt.path)
			p, _ := planIn(t, nb, domain.NotebookState{Mode: tt.mode, SourceRootID: raw},
				with(writeOp(tt.path, "A"), func(o *domain.BatchArgs) { o.Base = tokenOf(page, 1) }), editOp(tt.path, "A", "BB"))
			contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{page: headOf("", 1)}, &fakeRewriter{})
			if p.Failed != nil {
				t.Fatalf("plan failed: %v", p.Failed)
			}
			if pages := p.Pages(); len(pages) != 1 || pages[0].Merged != 0 {
				t.Errorf("pages = %+v, want zero source changes", pages)
			}
			for i, want := range []struct {
				content string
				before  int
			}{{"A", 0}, {"BB", 1}} {
				if len(p.Steps[i].Facts) != 1 {
					t.Fatalf("step %d facts = %+v", i, p.Steps[i].Facts)
				}
				checkSourceContent(t, p.Steps[i].Facts[0], want.content, want.before, 0)
			}
		})
	}
}

// A source attachment moved in and out keeps its kind and each side's
// membership; deletion also retains its outside position and kind.
func TestBatchSourceTreeFactsKeepAttachmentKindAndBothSides(t *testing.T) {
	nb := newNotebook("raw.md", "a.png")
	p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")},
		moveOp("a.png", "raw/a.png"), moveOp("raw/a.png", "a.png"))
	if p.Failed != nil {
		t.Fatalf("plan failed: %v", p.Failed)
	}
	for i, want := range [][2]bool{{false, true}, {true, false}} {
		if len(p.Steps[i].Facts) != 1 || len(p.Steps[i].Facts[0].Nodes) != 1 {
			t.Fatalf("step %d facts = %+v", i, p.Steps[i].Facts)
		}
		node := p.Steps[i].Facts[0].Nodes[0]
		if node.Kind != domain.KindAsset || node.Before.InArea != want[0] || node.After.InArea != want[1] {
			t.Errorf("step %d node = %+v, want attachment source %v", i, node, want)
		}
	}
	p, _ = planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")}, deleteOp("a.png"))
	if p.Failed != nil || len(p.Steps[0].Facts) != 1 {
		t.Fatalf("delete facts = %+v, failure %v", p.Steps[0].Facts, p.Failed)
	}
	node := p.Steps[0].Facts[0].Nodes[0]
	if node.Kind != domain.KindAsset || node.Before.InArea || node.After != nil {
		t.Errorf("deleted node = %+v", node)
	}
}

// A preceding automatic rewrite sets the byte size the user's write
// compares with. Writing those same bytes consumes no source change.
func TestBatchSourceContentFactsUseRewrittenBefore(t *testing.T) {
	nb := newNotebook("A.md", "raw.md", "raw/P.md")
	raw, page := nb.id(t, "raw.md"), nb.id(t, "raw/P.md")
	p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw},
		moveOp("A.md", "Long.md"), with(writeOp("raw/P.md", "[[Long]]"), func(o *domain.BatchArgs) { o.Base = tokenOf(page, 1) }))
	p.ReadWhole(page)
	contentsOf(t, p, map[uuid.UUID]domain.PageText{page: {Content: "[[A]]", Revision: 1}}, nil,
		&fakeRewriter{subs: map[string]string{"[[A]]": "[[Long]]"}})
	if p.Failed != nil || len(p.Steps[1].Facts) != 1 {
		t.Fatalf("facts = %+v, failure %v", p.Steps[1].Facts, p.Failed)
	}
	checkSourceContent(t, p.Steps[1].Facts[0], "[[Long]]", 8, 0)
	if pages := p.Pages(); len(pages) != 1 || pages[0].Merged != 0 {
		t.Errorf("pages = %+v, want zero user changes", pages)
	}
}

func checkSourceContent(t *testing.T, fact domain.StepFacts, content string, before, merged int) {
	t.Helper()
	if fact.Content == nil {
		t.Error("content facts missing")
		return
	}
	hash := sha256.Sum256([]byte(content))
	if !bytes.Equal(fact.Content.Hash, hash[:]) || fact.Content.Bytes != len(content) || fact.Content.BeforeBytes != before || fact.Content.Merged != merged {
		t.Errorf("content facts = %+v, want %q, BeforeBytes %d, Merged %d", fact.Content, content, before, merged)
	}
}
