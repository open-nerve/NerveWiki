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
	got := slices.Collect(domain.RevertFacts(state, plan, uuid.Nil()))
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
	got := slices.Collect(domain.RevertFacts(state, plan, uuid.Nil()))
	if len(got) != 1 || len(got[0].Nodes) != 1 {
		t.Fatalf("facts = %+v, want one content step", got)
	}
	node := got[0].Nodes[0]
	if got[0].Operation != domain.OpContent || node.ID != id || node.Before.Key != "body" || node.After.Key != "body" || node.Before.ParentID == nil || *node.Before.ParentID != parent {
		t.Errorf("content = %+v, want Body at its current parent", got[0])
	}
}

// Later facts must follow the simulated parent graph, not the initial
// tree or the moving root's single source flag.
func TestRevertFactsTracksSourceMembershipAndKinds(t *testing.T) {
	raw, outside, folder, asset := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	state := domain.RevertState{Tree: []domain.Node{
		{ID: raw, Kind: domain.KindPage, Name: "raw", NameKey: "raw"},
		{ID: outside, Kind: domain.KindPage, Name: "outside", NameKey: "outside"},
		{ID: folder, Kind: domain.KindPage, Name: "folder", NameKey: "folder"},
		{ID: asset, ParentID: &folder, Kind: domain.KindAsset, Name: "a.png", NameKey: "a.png"},
	}}
	plan := domain.RevertPlan{Ops: []domain.RevertOp{
		{Kind: domain.RevertPlace, NodeID: folder, Move: true, To: domain.TreeState{ParentID: &raw, Name: "folder"}},
		{Kind: domain.RevertAside, NodeID: asset, Rename: true, To: domain.TreeState{ParentID: &folder, Name: "aside.png"}},
		{Kind: domain.RevertPlace, NodeID: raw, Move: true, To: domain.TreeState{ParentID: &outside, Name: "raw"}},
		{Kind: domain.RevertPlace, NodeID: folder, Move: true, To: domain.TreeState{Name: "folder"}},
		{Kind: domain.RevertDelete, NodeID: folder},
	}}
	got := slices.Collect(domain.RevertFacts(state, plan, raw))
	want := [][][2]bool{
		{{false, true}, {false, true}},
		{{true, true}},
		{{false, false}, {true, true}, {true, true}},
		{{true, false}, {true, false}},
		{{false, false}, {false, false}},
	}
	if len(got) != len(want) {
		t.Fatalf("facts = %d steps, want %d", len(got), len(want))
	}
	for i, step := range got {
		if len(step.Nodes) != len(want[i]) {
			t.Fatalf("step %d nodes = %d, want %d", i, len(step.Nodes), len(want[i]))
		}
		for j, node := range step.Nodes {
			before, after := node.Before != nil && node.Before.InArea, node.After != nil && node.After.InArea
			if before != want[i][j][0] || after != want[i][j][1] {
				t.Errorf("step %d node %d source = %v -> %v, want %v", i, j, before, after, want[i][j])
			}
			kind := domain.KindPage
			if node.ID == asset {
				kind = domain.KindAsset
			}
			if node.Kind != kind {
				t.Errorf("step %d node %d kind = %q, want %q", i, j, node.Kind, kind)
			}
		}
	}
}

// A restored member is placed by the plan and starts at its trash
// content; each write-back then becomes the next write's BeforeBytes.
func TestRevertFactsRestoresThenUpdatesContentState(t *testing.T) {
	raw, outside, page, asset := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	state := domain.RevertState{
		Tree:     []domain.Node{{ID: raw, Kind: domain.KindPage, Name: "raw", NameKey: "raw"}},
		Contents: map[uuid.UUID]domain.ContentNow{page: {Revision: 8, Bytes: 99, Hash: []byte("stale")}},
		Hashes:   map[domain.VersionRef][]byte{{NodeID: page, Revision: 3}: []byte("trash")},
	}
	plan := domain.RevertPlan{Ops: []domain.RevertOp{
		{Kind: domain.RevertRestore, NodeID: page, Group: []domain.TrashMember{
			{Node: domain.Node{ID: page, ParentID: &outside, Kind: domain.KindPage}, Revision: 3, Bytes: 11},
			{Node: domain.Node{ID: asset, ParentID: &page, Kind: domain.KindAsset}},
		}, States: []domain.TreeState{{ParentID: &raw, Name: "page"}, {ParentID: &page, Name: "a.png"}}},
		{Kind: domain.RevertContent, NodeID: page, Hash: []byte("first"), Bytes: 44, Rewrite: true},
		{Kind: domain.RevertContent, NodeID: page, Hash: []byte("second"), Bytes: 2},
	}}
	got := slices.Collect(domain.RevertFacts(state, plan, raw))
	if len(got) != 3 || len(got[0].Nodes) != 2 {
		t.Fatalf("facts = %+v, want restore and two writes", got)
	}
	for i, node := range got[0].Nodes {
		if node.Before != nil || node.After == nil || !node.After.InArea {
			t.Errorf("restored node %d = %+v, want planned source After", i, node)
		}
	}
	if got[0].Nodes[1].Kind != domain.KindAsset {
		t.Errorf("restored attachment kind = %q", got[0].Nodes[1].Kind)
	}
	for i, want := range []struct {
		hash          string
		before, after int
		rewrite       bool
	}{{"first", 11, 44, true}, {"second", 44, 2, false}} {
		step := got[i+1]
		if step.Content == nil {
			t.Errorf("write %d content facts missing", i)
			continue
		}
		if string(step.Content.Hash) != want.hash || step.Content.BeforeBytes != want.before || step.Content.Bytes != want.after || step.Content.Merged != 1 || step.Rewrite != want.rewrite {
			t.Errorf("write %d content = %+v, rewrite %v; want %+v and Merged 1", i, step.Content, step.Rewrite, want)
		}
	}
	if state.Contents[page].Bytes != 99 || string(state.Contents[page].Hash) != "stale" {
		t.Errorf("facts changed caller's initial content: %+v", state.Contents[page])
	}
}

// Content-only reverts have current nodes and parent-keyed lines; asking
// the absent full tree alone would lose a nested source's membership.
func TestContentOnlyRevertSourceFactsUseParentLines(t *testing.T) {
	raw, parent, page := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	state := domain.RevertState{
		Versions: []domain.ChangesetVersion{{NodeID: page}, {NodeID: raw}},
		Nodes: map[uuid.UUID]domain.NodeNow{
			page: {Node: domain.Node{ID: page, ParentID: &parent, Kind: domain.KindPage, Name: "page", NameKey: "page"}},
			raw:  {Node: domain.Node{ID: raw, Kind: domain.KindPage, Name: "raw", NameKey: "raw"}},
		},
		Lines:    map[uuid.UUID][]domain.Ancestor{parent: {{ID: raw}, {ID: parent}}, uuid.Nil(): nil},
		Contents: map[uuid.UUID]domain.ContentNow{page: {Bytes: 17}, raw: {Bytes: 3}},
	}
	plan := domain.RevertPlan{Ops: []domain.RevertOp{
		{Kind: domain.RevertContent, NodeID: page, Hash: []byte("target"), Bytes: 5, Rewrite: true},
		{Kind: domain.RevertContent, NodeID: raw, Hash: []byte("root"), Bytes: 9},
	}}
	got := slices.Collect(domain.RevertFacts(state, plan, raw))
	if len(got) != 2 {
		t.Fatalf("facts = %+v", got)
	}
	for i, wantSource := range []bool{true, false} {
		node := got[i].Nodes[0]
		if node.Kind != domain.KindPage || node.Before.InArea != wantSource || node.After.InArea != wantSource {
			t.Errorf("step %d node = %+v, want page with source %v on both sides", i, node, wantSource)
		}
	}
	if c := got[0].Content; c == nil || c.BeforeBytes != 17 || c.Bytes != 5 || string(c.Hash) != "target" || !got[0].Rewrite {
		t.Errorf("source content = %+v, rewrite %v", c, got[0].Rewrite)
	}
}
