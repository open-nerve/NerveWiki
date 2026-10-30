package app_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A blanket root/AGENTS/folder exemption would change the independent
// relations below: folders are not orphans but still need type/summary.
func TestLintPreparePerRuleEligibility(t *testing.T) {
	for _, mode := range []domain.LintMode{domain.ModeStandard, domain.ModeLLM} {
		name := "standard"
		if mode == domain.ModeLLM {
			name = "llm"
		}
		t.Run(name, func(t *testing.T) {
			root, folder := lintID(10), lintID(20)
			f, in := lintFixture(
				lintNode(10, nil, "raw", domain.NodePage, 0),
				lintNode(19, nil, "asset-folder", domain.NodePage, 0),
				lintNode(20, nil, "folder", domain.NodePage, 0),
				lintNode(21, nil, "empty-leaf", domain.NodePage, 0),
				lintNode(22, nil, "agents", domain.NodePage, 1),
				lintNode(23, &folder, "agents", domain.NodePage, 1),
				lintNode(24, nil, "stale", domain.NodePage, 1),
				lintNode(25, nil, "bad", domain.NodePage, 1),
				lintNode(30, &root, "source", domain.NodePage, 1),
				lintNode(39, lintPtr(lintID(19)), "only-child.pdf", domain.NodeAsset, 0),
				lintNode(40, &folder, "child.pdf", domain.NodeAsset, 0),
			)
			for i := range f.heads {
				h := &f.heads[i]
				if h.ID == lintID(10) || h.ID == lintID(22) || h.ID == lintID(25) {
					h.FrontmatterValid = false
				}
				if h.ID == lintID(24) {
					h.Extractor = 1
				}
			}
			in.EffectiveMode = mode
			orphanIDs := []uuid.UUID{lintID(21), lintID(23), lintID(25)}
			if mode == domain.ModeLLM {
				in.AreaRootID, in.SourceIDs = &root, []uuid.UUID{lintID(30)}
			} else {
				orphanIDs = append(orphanIDs, lintID(30))
			}
			f.candidates[domain.Orphans] = orphanIDs
			f.sqlIDs[domain.Orphans] = []uuid.UUID{lintID(10), lintID(19), lintID(20), lintID(21), lintID(22), lintID(23), lintID(24), lintID(25), lintID(30)}
			f.sqlIDs[domain.InvalidFrontmatter] = []uuid.UUID{lintID(10), lintID(22), lintID(25)}
			missingIDs := []uuid.UUID{lintID(19), lintID(20), lintID(21), lintID(23)}
			for _, r := range []domain.Rule{domain.MissingType, domain.MissingSummary} {
				f.candidates[r], f.sqlIDs[r] = missingIDs, missingIDs
			}
			p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{})
			if err != nil {
				t.Fatal(err)
			}
			items, _ := lintAnswer(t, p, domain.Orphans, int64(len(orphanIDs)), false)
			var got []uuid.UUID
			for _, item := range items {
				got = append(got, *item.PageID)
			}
			if !slices.Equal(got, orphanIDs) {
				t.Fatalf("orphans=%v, want %v", got, orphanIDs)
			}
			lintAnswer(t, p, domain.InvalidFrontmatter, 3, false)
			if mode == domain.ModeLLM {
				lintAnswer(t, p, domain.MissingType, 4, false)
				lintAnswer(t, p, domain.MissingSummary, 4, false)
			} else if _, err := p.RuleCount(t.Context(), domain.MissingType); err == nil {
				t.Fatal("standard preparation exposed an LLM-only rule")
			}
		})
	}
}

func TestLintPrepareAssetsOnlyAfterPageSelection(t *testing.T) {
	f, in := lintSourceFixture()
	p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.UndigestedSources), Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	items, more := lintAnswer(t, p, domain.UndigestedSources, 2, false)
	if len(items) != 1 || !more || *items[0].NodeID != lintID(33) || len(f.assetCalls) != 0 {
		t.Fatalf("first page=%+v more=%t asset calls=%v; the asset lookahead must not be read", items, more, f.assetCalls)
	}
	cursor, err := domain.DecodeLintCursor(items[0].Cursor)
	if err != nil {
		t.Fatal(err)
	}
	after, err := cursor.PositionFor(domain.CursorBinding{NotebookID: f.nb, Mode: domain.ModeLLM, AreaRootID: in.AreaRootID}, domain.UndigestedSources)
	if err != nil {
		t.Fatal(err)
	}
	f, in = lintSourceFixture()
	p, err = f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.UndigestedSources), Limit: 1, After: &after})
	if err != nil {
		t.Fatal(err)
	}
	items, more = lintAnswer(t, p, domain.UndigestedSources, 2, false)
	if len(items) != 1 || more || *items[0].NodeID != lintID(42) || items[0].Bytes != 420 ||
		len(f.assetCalls) != 1 || !slices.Equal(f.assetCalls[0], []uuid.UUID{lintID(42)}) {
		t.Fatalf("second page=%+v more=%t asset calls=%v, want only asset 42 once", items, more, f.assetCalls)
	}
}
