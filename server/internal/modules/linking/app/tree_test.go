package app_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The tree a rewrite reads of a move of a large folder is built in work as
// long as the links it reaches, not as their product with the folder's
// nodes (M9/P3 review B-I1): a move of a folder of 10,000 nodes, one change
// a node, reaching 40,000 links into it, four to each node, allocates some
// MiB; an index of the changes built again for each link allocated some
// tens of GB. It counts allocations, not time, so the race detector and
// -short change nothing of it.
func TestTheTreeOfALargeMoveIsBuiltOnceForItsLinks(t *testing.T) {
	const nodes, each = 10_000, 4
	tree := pathsTree{nodes: map[uuid.UUID]domain.Node{}, named: map[string][]domain.Node{}}
	top, g, f := tree.step("Top"), tree.step("G"), tree.step("F")
	tree.add([]domain.Step{g})
	tree.add([]domain.Step{g, f})
	changes := []domain.Change{{NodeID: f.ID, Before: &domain.Place{ParentID: &top.ID, Name: "F"},
		After: &domain.Place{ParentID: &g.ID, Name: "F"}}}
	targets := slices.Repeat([]uuid.UUID{f.ID}, each)
	for i := range nodes - 1 {
		x := tree.step(fmt.Sprint("x", i))
		tree.add([]domain.Step{g, f, x})
		// moveSubtree's change of a node the move carries: the same place.
		place := &domain.Place{ParentID: &f.ID, Name: x.Name}
		changes = append(changes, domain.Change{NodeID: x.ID, Before: place, After: place})
		targets = append(targets, slices.Repeat([]uuid.UUID{x.ID}, each)...)
	}
	m := app.Moved{NotebookID: uuid.NewV7(), Changes: changes, Parents: map[uuid.UUID][]domain.Step{top.ID: {top}}}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	got, _, err := app.TreeOf(context.Background(), tree, m, targets, nil)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("%d links to %d nodes moved: %.1f MiB allocated", len(targets), nodes, float64(allocated)/(1<<20))
	if allocated > 64<<20 {
		t.Errorf("the tree of %d links to %d nodes moved allocated %d MiB, want 64 at most", len(targets), nodes, allocated>>20)
	}
	x := changes[1].NodeID
	if len(got.Before) != nodes || len(got.After) != nodes || !slices.Equal(stepIDs(got.Before[x].Path), []uuid.UUID{top.ID, f.ID, x}) ||
		!slices.Equal(stepIDs(got.After[x].Path), []uuid.UUID{g.ID, f.ID, x}) {
		t.Errorf("the tree holds %d nodes before and %d after, %s at %v before and %v after; want %d, Top/F/x0 and G/F/x0", len(got.Before),
			len(got.After), x, got.Before[x].Path, got.After[x].Path, nodes)
	}
}

// pathsTree is a tree for Pages of many nodes, read by maps: each node at
// its path from the root, and by its title's key.
type pathsTree struct {
	nodes map[uuid.UUID]domain.Node
	named map[string][]domain.Node
}

// step is a node named name, with an id after every other.
func (pathsTree) step(name string) domain.Step {
	return domain.Step{ID: uuid.NewV7(), Key: shared.TitleKey(name), Name: name}
}

// add has the node at the end of path there.
func (p pathsTree) add(path []domain.Step) {
	n := domain.Node{ID: path[len(path)-1].ID, Path: path}
	p.nodes[n.ID] = n
	key := path[len(path)-1].Key
	p.named[key] = append(p.named[key], n)
}

func (p pathsTree) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	out := make([]domain.Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := p.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}

func (p pathsTree) ByKeys(_ context.Context, _ uuid.UUID, keys []string) ([]domain.Node, error) {
	var out []domain.Node
	for _, k := range keys {
		out = append(out, p.named[k]...)
	}
	return out, nil
}

func (pathsTree) Subtree(context.Context, uuid.UUID, uuid.UUID) ([]domain.Step, error) {
	return nil, errors.New("a subtree is not read here")
}

func (pathsTree) SourcePath(context.Context, uuid.UUID, uuid.UUID) (domain.Node, bool, error) {
	return domain.Node{}, false, errors.New("a source's path is not read here")
}

// stepIDs is the ids of path's steps.
func stepIDs(path []domain.Step) []uuid.UUID {
	out := make([]uuid.UUID, len(path))
	for i, s := range path {
		out[i] = s.ID
	}
	return out
}
