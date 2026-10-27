package domain_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// These are the four permitted sequences and three refused sequences from
// P2 section 3.5. Removing per-item checks lets every negative case through;
// judging unchanged bytes or the final location breaks the positive cases.
func TestBatchSourceContractJudgesEachActualChange(t *testing.T) {
	for _, tt := range []struct {
		name           string
		ops            func(uuid.UUID) []domain.BatchArgs
		before         string
		path           string
		wantOp, needOp int
		wantContent    string
	}{
		{"admin create then move out then edit", func(_ uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{writeOp("raw/new.md", "A"), moveOp("raw/new.md", "new.md"), editOp("new.md", "A", "B")}
		}, "", "", -1, 1, "B"},
		{"admin empty first then move out then edit", func(id uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{with(writeOp("raw/P.md", "A"), func(o *domain.BatchArgs) { o.Base = tokenOf(id, 1) }), moveOp("raw/P.md", "P.md"), editOp("P.md", "A", "B")}
		}, "", "raw/P.md", -1, 1, "B"},
		{"writer create then same bytes", func(_ uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{writeOp("raw/new.md", "A"), writeOp("raw/new.md", "A")}
		}, "", "", -1, -1, "A"},
		{"writer empty creation then first bytes", func(_ uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{writeOp("raw/new.md", ""), writeOp("raw/new.md", "B")}
		}, "", "", -1, -1, "B"},
		{"create then change rejects second item", func(_ uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{writeOp("raw/new.md", "A"), editOp("raw/new.md", "A", "B")}
		}, "", "", 1, -1, ""},
		{"empty first then change rejects second item", func(id uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{with(writeOp("raw/P.md", "A"), func(o *domain.BatchArgs) { o.Base = tokenOf(id, 1) }), editOp("raw/P.md", "A", "B")}
		}, "", "raw/P.md", 1, -1, ""},
		{"outside edit then move in then edit rejects third item", func(_ uuid.UUID) []domain.BatchArgs {
			return []domain.BatchArgs{editOp("P.md", "A", "B"), moveOp("P.md", "raw/P.md"), editOp("raw/P.md", "B", "C")}
		}, "A", "P.md", 2, -1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nb := newNotebook("raw.md")
			var page uuid.UUID
			texts := map[uuid.UUID]domain.PageText{}
			heads := map[uuid.UUID]domain.PageHead{}
			if tt.path != "" {
				page = nb.add(tt.path)
				texts[page] = domain.PageText{Content: tt.before, Revision: 1}
				heads[page] = headOf(tt.before, 1)
			}
			ops := tt.ops(page)
			p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")}, ops...)
			if p.Failed != nil {
				t.Fatalf("tree pass failed: %v", p.Failed)
			}
			contentsOf(t, p, texts, heads, &fakeRewriter{})
			if tt.wantOp >= 0 {
				if got := failed(t, p); got != (refusal{"page.source_immutable", "", "", tt.wantOp}) {
					t.Fatalf("failure = %+v, want source_immutable at %d", got, tt.wantOp)
				}
				var refused *domain.Refused
				if !errors.As(p.Failed, &refused) || refused.NodeID != p.Steps[tt.wantOp].Node || p.Failed.Tool != domain.OpKindEdit {
					t.Fatalf("failure = %+v, want actual edit and node %s", p.Failed, p.Steps[tt.wantOp].Node)
				}
				return
			}
			if p.Failed != nil {
				t.Fatalf("content pass failed: %v", p.Failed)
			}
			if pages := p.Pages(); len(pages) != 1 || pages[0].Content != tt.wantContent || pages[0].Merged != 1 {
				t.Fatalf("pages = %+v, want %q and one actual source change", pages, tt.wantContent)
			}
			if tt.needOp < 0 {
				if p.Needs != nil {
					t.Fatalf("writer sequence needs %+v", p.Needs)
				}
			} else if p.Needs == nil || p.Needs.Op != tt.needOp || p.Needs.NodeID != p.Steps[tt.needOp].Node || p.Needs.Action != domain.ActionManageSource {
				t.Fatalf("need = %+v, want manage_source of moved node at %d", p.Needs, tt.needOp)
			}
		})
	}
}

// Clearing a page outside source makes its next first write valid item by
// item. Merged still carries both in-source changes to the final guard,
// which retains the explicitly accepted conservative fallback.
func TestBatchSourceContractKeepsMergedGuardFallback(t *testing.T) {
	nb := newNotebook("raw.md", "raw/P.md")
	page := nb.id(t, "raw/P.md")
	write := func(path, content string) domain.BatchArgs {
		return with(writeOp(path, content), func(o *domain.BatchArgs) { o.Base = tokenOf(page, 1) })
	}
	p, _ := planIn(t, nb, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")},
		write("raw/P.md", "A"), moveOp("raw/P.md", "P.md"), write("P.md", ""), moveOp("P.md", "raw/P.md"), write("raw/P.md", "B"))
	contentsOf(t, p, nil, map[uuid.UUID]domain.PageHead{page: headOf("", 1)}, &fakeRewriter{})
	if p.Failed != nil {
		t.Fatalf("per-item plan failed before merged guard fallback: %v", p.Failed)
	}
	if pages := p.Pages(); len(pages) != 1 || pages[0].Content != "B" || pages[0].Merged != 2 {
		t.Fatalf("pages = %+v, want final B and two source changes for guard fallback", pages)
	}
}

// Ordinary errors belong before source policy, and an actual earlier source
// content failure wins over a later tree refusal already recorded in PlanBatch.
func TestBatchSourceContractKeepsOrdinaryErrorsAndItemOrder(t *testing.T) {
	nb := newNotebook("raw.md", "raw/P.md", "AGENTS.md")
	page := nb.id(t, "raw/P.md")
	n := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")}
	for _, tt := range []struct {
		name string
		ops  []domain.BatchArgs
		code string
		op   int
	}{
		{"stale base", []domain.BatchArgs{with(writeOp("raw/P.md", "B"), func(o *domain.BatchArgs) { o.Base = tokenOf(page, 2) })}, "page.revision_mismatch", 0},
		{"missing replacement", []domain.BatchArgs{editOp("raw/P.md", "absent", "B")}, "validation_failed", 0},
		{"earlier immutable before later protected", []domain.BatchArgs{editOp("raw/P.md", "A", "B"), deleteOp("AGENTS.md")}, "page.source_immutable", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := planIn(t, nb, n, tt.ops...)
			contentsOf(t, p, map[uuid.UUID]domain.PageText{page: {Content: "A", Revision: 1}}, map[uuid.UUID]domain.PageHead{page: headOf("A", 1)}, &fakeRewriter{})
			if got := failed(t, p); got.code != tt.code || got.at != tt.op {
				t.Fatalf("failure = %+v, want %s at %d", got, tt.code, tt.op)
			}
		})
	}
}

// The first-body exemption is confined to a source page created empty by this
// batch. Adjacent cases retain write's no-base path-taken contract.
func TestBatchSourceFirstBodyKeepsNoBaseBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mode    domain.Mode
		path    string
		exists  bool
		created string
	}{
		{"existing empty source", domain.ModeLLMWiki, "raw/P.md", true, ""},
		{"new nonempty source", domain.ModeLLMWiki, "raw/P.md", false, "A"},
		{"standard new empty page", domain.ModeStandard, "raw/P.md", false, ""},
		{"outside new empty page", domain.ModeLLMWiki, "P.md", false, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nb := newNotebook("raw.md")
			var ops []domain.BatchArgs
			heads := map[uuid.UUID]domain.PageHead{}
			if tt.exists {
				id := nb.add(tt.path)
				heads[id] = headOf("", 1)
			} else {
				ops = append(ops, writeOp(tt.path, tt.created))
			}
			ops = append(ops, writeOp(tt.path, "B"))
			p, _ := planIn(t, nb, domain.NotebookState{Mode: tt.mode, SourceRootID: nb.id(t, "raw.md")}, ops...)
			contentsOf(t, p, nil, heads, &fakeRewriter{})
			if got := failed(t, p); got != (refusal{"page.title_taken", "", "", len(ops) - 1}) {
				t.Fatalf("failure = %+v, want original no-base path-taken refusal", got)
			}
		})
	}
}
