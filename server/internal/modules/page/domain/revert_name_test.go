package domain_test

import (
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestPlanRevertNameCandidatesKeepConflictSemantics(t *testing.T) {
	t.Run("untouched holder outside the changeset", func(t *testing.T) {
		w := newWorld(t)
		id := w.asset("new.png", nil, 0)
		w.placed(id, domain.TreeState{Name: "old.png"})
		holder := w.asset("OLD.png", nil, 1)
		want := domain.RevertPlan{Count: 1, Conflicts: []domain.RevertConflict{{
			NodeID: id, Name: "old.png", Reason: domain.ConflictTitleTaken, HolderID: &holder,
		}}}
		if got := w.plan(); !reflect.DeepEqual(got, want) {
			t.Fatalf("plan = %+v; want %+v", got, want)
		}
	})
	t.Run("self and another parent do not hold the target name", func(t *testing.T) {
		w := newWorld(t)
		parent := w.page("Folder", nil, 0, 1, "folder")
		w.asset("same.png", &parent, 0)
		id := w.asset("same.png", nil, 2)
		w.placed(id, domain.TreeState{Name: "same.png", SortOrder: 1})
		want := domain.RevertPlan{Ops: []domain.RevertOp{{
			Kind: domain.RevertPlace, NodeID: id, To: domain.TreeState{Name: "same.png", SortOrder: 1}, Move: true,
		}}}
		if got := w.plan(); !reflect.DeepEqual(got, want) {
			t.Fatalf("plan = %+v; want %+v", got, want)
		}
	})
	for _, withMove := range []bool{false, true} {
		name := "children added with no target names"
		if withMove {
			name = "children added outside the target names"
		}
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			parent := w.page("Created", nil, 0, 1, "created")
			w.created(parent)
			child := w.asset("added.png", &parent, 0)
			if withMove {
				id := w.asset("new.png", nil, 1)
				w.placed(id, domain.TreeState{Name: "old.png", SortOrder: 1})
			}
			want := domain.RevertPlan{Count: 1, Conflicts: []domain.RevertConflict{{
				NodeID: child, Name: "added.png", Reason: domain.ConflictChildrenAdded,
			}}}
			if got := w.plan(); !reflect.DeepEqual(got, want) {
				t.Fatalf("plan = %+v; want %+v", got, want)
			}
		})
	}
	t.Run("a content-only item can hold the target name", func(t *testing.T) {
		w := newWorld(t)
		id := w.asset("new", nil, 0)
		w.placed(id, domain.TreeState{Name: "old"})
		// The changeset wrote this page; a later rename took the other
		// node's old name without changing the written content.
		holder := w.page("old", nil, 1, 2, "written")
		w.wrote(holder, 1, "before")
		state := w.state()
		state.Nodes[holder] = w.nodes[holder]
		want := domain.RevertPlan{Count: 1, Conflicts: []domain.RevertConflict{{
			NodeID: id, Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &holder,
		}}}
		got, err := domain.PlanRevert(state)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("plan = %+v, %v; want %+v", got, err, want)
		}
	})
}

// Conflicting read snapshots still choose the same holder: an untouched
// contender first, then ascending UUID, excluding the node itself. These
// deliberately conflicting states are not examples of a valid earlier tree.
func TestPlanRevertNameCandidatesChooseAStableHolder(t *testing.T) {
	ids := []uuid.UUID{
		uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f501"),
		uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f502"),
		uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f503"),
	}
	for _, tt := range []struct {
		name               string
		untouched, written bool
	}{
		{"touched contenders exclude self", false, false},
		{"larger untouched contender takes precedence", true, false},
		{"larger content-only contender takes precedence", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := domain.RevertState{Nodes: map[uuid.UUID]domain.NodeNow{}}
			for i, id := range ids {
				current := []string{"one", "two", "three"}[i]
				if tt.untouched && i == 2 {
					current = "old"
				}
				n := domain.Node{ID: id, Kind: domain.KindAsset, Name: current, NameKey: current}
				if tt.written && i == 2 {
					n.Kind = domain.KindPage
					state.Contents = map[uuid.UUID]domain.ContentNow{id: {Revision: 2, Hash: hash("written")}}
					state.Hashes = map[domain.VersionRef][]byte{{NodeID: id, Revision: 1}: hash("before")}
					state.Versions = []domain.ChangesetVersion{{NodeID: id, Base: new(1), Revision: 2, Hash: hash("written")}}
				}
				state.Nodes[id] = domain.NodeNow{Node: n}
				state.Tree = append(state.Tree, n)
				if !tt.untouched || i != 2 {
					state.Items = append(state.Items, domain.ChangesetItem{NodeID: id,
						Before: &domain.TreeState{Name: "old"}, After: &domain.TreeState{Name: current}})
				}
			}
			want := domain.RevertPlan{Count: 3, Conflicts: []domain.RevertConflict{
				{NodeID: ids[0], Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &ids[1]},
				{NodeID: ids[1], Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &ids[0]},
				{NodeID: ids[2], Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &ids[0]},
			}}
			if tt.untouched {
				want = domain.RevertPlan{Count: 2, Conflicts: []domain.RevertConflict{
					{NodeID: ids[0], Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &ids[2]},
					{NodeID: ids[1], Name: "old", Reason: domain.ConflictTitleTaken, HolderID: &ids[2]},
				}}
			}
			got, err := domain.PlanRevert(state)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("plan = %+v, %v; want %+v", got, err, want)
			}
		})
	}
}
