package domain_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Restoring one attachment in a large notebook must not allocate a name
// candidate list for every unrelated node. Fixture construction is outside
// the measurement; the complete real planner, including its tree simulation,
// runs inside it. This memory bound does not replace the HTTP time guards.
func TestPlanRevertNameCandidatesDoNotAllocatePerUnrelatedNode(t *testing.T) {
	id := uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f5a6")
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	node := domain.Node{ID: id, Kind: domain.KindAsset, Name: "restored.png", NameKey: "restored.png"}
	state := domain.RevertState{
		Items:  []domain.ChangesetItem{{NodeID: id, Before: &domain.TreeState{Name: "restored.png"}}},
		Nodes:  map[uuid.UUID]domain.NodeNow{id: {Node: node, DeletedAt: &deletedAt, TrashRoot: true, TrashAssets: 1}},
		Groups: map[uuid.UUID][]domain.TrashMember{id: {{Node: node, Level: 1}}},
		At:     deletedAt.Add(time.Hour), Retention: retention,
	}
	for i := range 100_000 {
		name := fmt.Sprintf("unrelated-%06d.png", i)
		state.Tree = append(state.Tree, domain.Node{ID: uuid.NewV7(), Kind: domain.KindAsset, Name: name, NameKey: name})
	}
	want := domain.RevertPlan{Ops: []domain.RevertOp{{
		Kind: domain.RevertRestore, NodeID: id, To: domain.TreeState{Name: "restored.png"},
		Group:  []domain.TrashMember{{Node: node, Level: 1}},
		States: []domain.TreeState{{Name: "restored.png"}}, DeletedAt: deletedAt,
	}}}
	var got domain.RevertPlan
	var err error
	allocs := testing.AllocsPerRun(1, func() {
		got, err = domain.PlanRevert(state)
	})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanRevert = %+v, %v; want %+v", got, err, want)
	}
	t.Logf("one restored attachment among 100000 unrelated nodes: %.0f allocations", allocs)
	if allocs > 25_000 {
		t.Fatalf("PlanRevert allocated %.0f times; budget 25000 excludes a candidate list per unrelated name", allocs)
	}
}
