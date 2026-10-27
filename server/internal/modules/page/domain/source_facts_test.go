package domain_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Copying the root's membership to every node, or sharing the two sides'
// ancestor line, would miss a subtree entering or leaving the source area.
func TestSourceFactsKeepsMoveSidesSeparate(t *testing.T) {
	raw, nested, outside, folder, asset := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, tt := range []struct {
		name          string
		from, to      uuid.UUID
		before, after []domain.Ancestor
		wantBefore    bool
		wantAfter     bool
	}{
		{"into", outside, nested, []domain.Ancestor{{ID: outside}}, []domain.Ancestor{{ID: raw}, {ID: nested}}, false, true},
		{"out", nested, outside, []domain.Ancestor{{ID: raw}, {ID: nested}}, []domain.Ancestor{{ID: outside}}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{
				{ID: folder, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &tt.from}, After: &domain.NodePlace{ParentID: &tt.to}},
				{ID: asset, Kind: domain.KindAsset, Before: &domain.NodePlace{ParentID: &folder}, After: &domain.NodePlace{ParentID: &folder}},
			}}
			got := domain.SourceFacts(facts, raw, tt.before, tt.after, nil)
			for i, node := range got.Nodes {
				if node.Before.InArea != tt.wantBefore || node.After.InArea != tt.wantAfter {
					t.Errorf("node %d source = %v -> %v, want %v -> %v", i, node.Before.InArea, node.After.InArea, tt.wantBefore, tt.wantAfter)
				}
			}
			if got.Nodes[1].Kind != domain.KindAsset || got.Content != nil {
				t.Errorf("attachment facts = %+v, content = %+v", got.Nodes[1], got.Content)
			}
		})
	}
}

// A source root and its ancestors are outside; moving either carries
// source descendants whose membership remains true, including assets.
func TestSourceFactsRootAndAncestorMovesKeepStrictDescendants(t *testing.T) {
	outer, raw, page, asset, sibling, elsewhere := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, tt := range []struct {
		name  string
		nodes []domain.NodeFacts
		want  []bool
	}{
		{"source root", []domain.NodeFacts{
			{ID: raw, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &outer}, After: &domain.NodePlace{ParentID: &elsewhere}},
			{ID: page, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &raw}, After: &domain.NodePlace{ParentID: &raw}},
			{ID: asset, Kind: domain.KindAsset, Before: &domain.NodePlace{ParentID: &page}, After: &domain.NodePlace{ParentID: &page}},
		}, []bool{false, true, true}},
		{"source ancestor", []domain.NodeFacts{
			{ID: outer, Kind: domain.KindPage, Before: &domain.NodePlace{}, After: &domain.NodePlace{ParentID: &elsewhere}},
			{ID: raw, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &outer}, After: &domain.NodePlace{ParentID: &outer}},
			{ID: sibling, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &outer}, After: &domain.NodePlace{ParentID: &outer}},
			{ID: page, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &raw}, After: &domain.NodePlace{ParentID: &raw}},
			{ID: asset, Kind: domain.KindAsset, Before: &domain.NodePlace{ParentID: &page}, After: &domain.NodePlace{ParentID: &page}},
		}, []bool{false, false, false, true, true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.SourceFacts(domain.StepFacts{Operation: domain.OpMove, Nodes: tt.nodes}, raw,
				[]domain.Ancestor{{ID: outer}}, []domain.Ancestor{{ID: elsewhere}}, nil)
			for i, node := range got.Nodes {
				if node.Before.InArea != tt.want[i] || node.After.InArea != tt.want[i] {
					t.Errorf("node %d source = %v -> %v, want %v on both sides", i, node.Before.InArea, node.After.InArea, tt.want[i])
				}
			}
		})
	}
}

// Restoration has no Before; its source identity comes from the planned
// After, even when no node in the operation is currently alive.
func TestSourceFactsRestorationUsesAfter(t *testing.T) {
	raw, parent, page, asset := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	got := domain.SourceFacts(domain.StepFacts{Operation: domain.OpRestore, Nodes: []domain.NodeFacts{
		{ID: page, Kind: domain.KindPage, After: &domain.NodePlace{ParentID: &parent}},
		{ID: asset, Kind: domain.KindAsset, After: &domain.NodePlace{ParentID: &page}},
	}}, raw, nil, []domain.Ancestor{{ID: raw}, {ID: parent}}, nil)
	for i, node := range got.Nodes {
		if node.Before != nil || node.After == nil || !node.After.InArea {
			t.Errorf("restored node %d = %+v, want no Before and source After", i, node)
		}
	}
}

func TestSourceFactsRestoredForestUsesEachExternalParent(t *testing.T) {
	raw, p, q, a, b := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	line := map[uuid.UUID][]domain.Ancestor{p: {{ID: p}}, q: {{ID: raw}, {ID: q}}}
	for _, reverse := range []bool{false, true} {
		nodes := []domain.NodeFacts{{ID: a, Kind: domain.KindPage, After: &domain.NodePlace{ParentID: &p}}, {ID: b, Kind: domain.KindAsset, After: &domain.NodePlace{ParentID: &q}}}
		if reverse {
			nodes[0], nodes[1] = nodes[1], nodes[0]
		}
		facts := domain.SourceFacts(domain.StepFacts{Operation: domain.OpRestore, Nodes: nodes}, raw, nil, nil, line)
		for _, n := range facts.Nodes {
			if n.After.InArea != (n.ID == b) {
				t.Errorf("reverse=%v node=%s source=%v, want %v", reverse, n.ID, n.After.InArea, n.ID == b)
			}
		}
	}
	// Q is itself changed in this step. Its new outside place overrides the
	// cached old line, and B follows Q's new place.
	facts := domain.SourceFacts(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{
		{ID: q, After: &domain.NodePlace{ParentID: &p}}, {ID: b, After: &domain.NodePlace{ParentID: &q}},
	}}, raw, nil, nil, line)
	if facts.Nodes[0].After.InArea || facts.Nodes[1].After.InArea {
		t.Fatalf("stale parent line overrode changed parent: %+v", facts.Nodes)
	}
}
