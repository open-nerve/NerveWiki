package domain_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Each later operation must see the moves, restores and deletions before
// it, including descendants that joined or left a subtree meanwhile.
func TestARevertsFactsFollowItsSteps(t *testing.T) {
	// The descendant has the smaller id: sorting all nodes by id would
	// incorrectly put it before its root.
	box, doc, raw, notes := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	state := domain.RevertState{Items: []domain.ChangesetItem{{NodeID: box, After: &domain.TreeState{Name: "Box"}}}, Tree: []domain.Node{
		{ID: notes, Kind: domain.KindPage, Name: "Notes", NameKey: "notes"},
		{ID: box, Kind: domain.KindPage, Name: "Box", NameKey: "box"},
		{ID: raw, Kind: domain.KindPage, Name: "sources", NameKey: "sources", ParentID: &box},
	}}
	plan := domain.RevertPlan{Ops: []domain.RevertOp{
		{Kind: domain.RevertPlace, NodeID: raw, Move: true, Rename: true, To: domain.TreeState{Name: "raw"}},
		{Kind: domain.RevertDelete, NodeID: box, Subtree: []uuid.UUID{box}},
		{Kind: domain.RevertRestore, NodeID: doc, Group: []domain.TrashMember{{Node: domain.Node{ID: doc, Kind: domain.KindPage}}}, States: []domain.TreeState{{ParentID: &raw, Name: "Doc"}}},
		{Kind: domain.RevertAside, NodeID: raw, Move: true, Rename: true, To: domain.TreeState{Name: "raw aside", ParentID: &notes}},
		{Kind: domain.RevertContent, NodeID: doc},
		{Kind: domain.RevertDelete, NodeID: raw, Subtree: []uuid.UUID{raw, doc}},
	}}
	got := slices.Collect(domain.RevertFacts(state, plan))
	if len(got) != 6 {
		t.Fatalf("facts = %+v, want six operations", got)
	}
	if got[0].Operation != domain.OpMove || len(got[0].Nodes) != 1 || got[0].Nodes[0].Before.Key != "sources" || got[0].Nodes[0].After.Key != "raw" {
		t.Errorf("first move = %+v, want sources moved and renamed together", got[0])
	}
	if got[1].Operation != domain.OpDelete || len(got[1].Nodes) != 1 || got[1].Nodes[0].ID != box || got[1].Nodes[0].After != nil {
		t.Errorf("deletion = %+v, want Box only after raw left", got[1])
	}
	if got[2].Operation != domain.OpRestore || got[2].Nodes[0].Before != nil || got[2].Nodes[0].After.Key != "doc" {
		t.Errorf("restore = %+v", got[2])
	}
	if got[3].Operation != domain.OpMove || len(got[3].Nodes) != 2 || got[3].Nodes[0].ID != raw || got[3].Nodes[1].ID != doc || got[3].Nodes[0].Before.Key != "raw" {
		t.Errorf("later move = %+v, want raw first and newly restored Doc beneath it", got[3])
	}
	if got[4].Operation != domain.OpContent || got[4].Nodes[0].Before.Key != "doc" || *got[4].Nodes[0].After.ParentID != raw {
		t.Errorf("content = %+v", got[4])
	}
	if got[5].Operation != domain.OpDelete || len(got[5].Nodes) != 2 || got[5].Nodes[0].ID != raw || got[5].Nodes[1].ID != doc || got[5].Nodes[0].Before.Key != "raw aside" {
		t.Errorf("later deletion = %+v, want renamed raw then restored Doc", got[5])
	}
	for _, f := range got {
		if f.Kind != domain.ChangesetRevert {
			t.Errorf("kind = %s", f.Kind)
		}
	}
}

// A content-only changeset reads the current nodes without reading a tree.
func TestAContentOnlyRevertsFactsNeedNoTree(t *testing.T) {
	id, parent := uuid.NewV7(), uuid.NewV7()
	state := domain.RevertState{Versions: []domain.ChangesetVersion{{NodeID: id}}, Nodes: map[uuid.UUID]domain.NodeNow{
		id: {Node: domain.Node{ID: id, ParentID: &parent, Name: "Body", NameKey: "body", Kind: domain.KindPage}},
	}}
	plan := domain.RevertPlan{Ops: []domain.RevertOp{{Kind: domain.RevertContent, NodeID: id}}}
	got := slices.Collect(domain.RevertFacts(state, plan))
	if len(got) != 1 || len(got[0].Nodes) != 1 {
		t.Fatalf("facts = %+v, want one content step", got)
	}
	node := got[0].Nodes[0]
	if got[0].Operation != domain.OpContent || node.ID != id || node.Before.Key != "body" || node.After.Key != "body" || node.Before.ParentID == nil || *node.Before.ParentID != parent {
		t.Errorf("content = %+v, want Body at its current parent", got[0])
	}
}
