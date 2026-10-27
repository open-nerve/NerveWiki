package app

import (
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The checked payload is one connected restore group with a page's
// write-back. These tests isolate the ordering adapter; real RunBatch
// fixtures separately verify locks, history and persistence.
func TestSourceJobFactsUseNominalCutsAndActualActions(t *testing.T) {
	raw, f, a, b := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	node := func(id uuid.UUID, name string, parent *uuid.UUID, order float64) domain.Node {
		return domain.Node{ID: id, Kind: domain.KindPage, Name: name, NameKey: name, ParentID: parent, SortOrder: order}
	}
	group := []domain.TrashMember{{Node: node(f, "F", &raw, 0), Revision: 1},
		{Node: node(a, "A", &f, 0), Level: 1, Revision: 3, Bytes: 1}, {Node: node(b, "B", &f, 1), Level: 1, Revision: 1, Bytes: 4}}
	states := make([]domain.TreeState, len(group))
	for i, m := range group {
		states[i] = m.Node.State()
	}
	checked := revertChecked{state: domain.RevertState{Tree: []domain.Node{node(raw, "raw", nil, 0)},
		Hashes: map[domain.VersionRef][]byte{{NodeID: a, Revision: 3}: []byte("A"), {NodeID: b, Revision: 1}: []byte("BBBB")}},
		plan: domain.RevertPlan{Ops: []domain.RevertOp{
			{Kind: domain.RevertRestore, NodeID: f, Group: group, States: states},
			{Kind: domain.RevertContent, NodeID: a, Base: 2, Bytes: 2, Hash: []byte("CC"), Rewrite: true},
		}}}
	job := revertSteps(uuid.NewV7(), checked)
	names := map[uuid.UUID]string{f: "F", a: "A", b: "B"}
	for _, tt := range []struct {
		name string
		cuts [][2]int
		want []string
	}{
		{"one_batch", [][2]int{{0, 4}}, []string{"restore F A B", "content A"}},
		{"legal_tail", [][2]int{{0, 3}, {3, 4}}, []string{"restore F A", "content A", "restore B"}},
		{"halved_prefix_then_remainder", [][2]int{{0, 1}, {1, 4}}, []string{"restore F", "restore A B", "content A"}},
		{"three_batches", [][2]int{{0, 1}, {1, 3}, {3, 4}}, []string{"restore F", "restore A", "content A", "restore B"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := checkedBatches(job.Steps(), func([]domain.JobStep) [][2]int { return tt.cuts }); err != nil {
				t.Fatal(err)
			}
			facts := slices.Collect(jobRevertFacts(checked, job, tt.cuts, raw))
			var got []string
			for _, s := range facts {
				line := string(s.Operation)
				for _, n := range s.Nodes {
					line += " " + names[n.ID]
					if n.After == nil || !n.After.InArea || n.Kind != domain.KindPage {
						t.Errorf("lost current source membership: %+v", n)
					}
				}
				got = append(got, line)
				if s.Operation == domain.OpContent && (s.Content == nil || s.Content.BeforeBytes != 1 || s.Content.Bytes != 2 || !s.Rewrite) {
					t.Errorf("content facts=%+v rewrite=%v", s.Content, s.Rewrite)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("actions=%v, want hand-written trace %v", got, tt.want)
			}
		})
	}

	// Independent groups stay independent even when one nominal batch holds
	// both. Compact interleaves each restored page's own write-back.
	g, c := uuid.NewV7(), uuid.NewV7()
	other := []domain.TrashMember{{Node: node(g, "G", &raw, 1), Revision: 1}, {Node: node(c, "C", &g, 0), Level: 1, Revision: 3, Bytes: 5}}
	checked.plan.Ops = []domain.RevertOp{checked.plan.Ops[0],
		{Kind: domain.RevertRestore, NodeID: g, Group: other, States: []domain.TreeState{other[0].Node.State(), other[1].Node.State()}},
		checked.plan.Ops[1], {Kind: domain.RevertContent, NodeID: c, Base: 2, Bytes: 6, Hash: []byte("target"), Rewrite: true}}
	job = revertSteps(uuid.NewV7(), checked)
	names[g], names[c] = "G", "C"
	var got []string
	for fact := range jobRevertFacts(checked, job, [][2]int{{0, len(job.Steps())}}, raw) {
		line := string(fact.Operation)
		for _, n := range fact.Nodes {
			line += " " + names[n.ID]
		}
		got = append(got, line)
	}
	want := []string{"restore F A B", "content A", "restore G C", "content C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("independent groups=%v, want %v", got, want)
	}
}
