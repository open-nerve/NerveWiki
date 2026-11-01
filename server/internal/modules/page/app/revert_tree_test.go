package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// This reader refuses the wide public read. Rest must use the independent
// structural port, preserve its error, and keep an empty read distinct from
// the unread tree of the content-only path.
type revertTreeReads struct {
	History
	Trash
	notebook, node, parent uuid.UUID
	treeCalls, lineCalls   int
	err                    error
}

func (r *revertTreeReads) ContentsNow(context.Context, []uuid.UUID) (map[uuid.UUID]domain.ContentNow, error) {
	return nil, nil
}

func (r *revertTreeReads) VersionHashes(context.Context, []domain.VersionRef) (map[domain.VersionRef][]byte, error) {
	return nil, nil
}

func (r *revertTreeReads) ListRevertTree(_ context.Context, notebook uuid.UUID) ([]domain.RevertTreeNode, error) {
	r.treeCalls++
	if notebook != r.notebook {
		return nil, errors.New("wrong notebook for revert tree")
	}
	if r.err != nil {
		return nil, r.err
	}
	return []domain.RevertTreeNode{}, nil
}

func (r *revertTreeReads) TrashLines(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.TrashAncestor, error) {
	r.lineCalls++
	if len(ids) != 1 || ids[0] != r.node {
		return nil, errors.New("wrong content-only ancestry request")
	}
	return map[uuid.UUID][]domain.TrashAncestor{r.node: {{ID: r.parent, Name: "raw"}}}, nil
}

func TestRevertReadsDistinguishUnreadAndEmptyTree(t *testing.T) {
	for _, items := range []bool{false, true} {
		name := "content only"
		if items {
			name = "empty tree read"
		}
		t.Run(name, func(t *testing.T) {
			r := &revertTreeReads{notebook: uuid.NewV7(), node: uuid.NewV7(), parent: uuid.NewV7()}
			reads := revertReads{history: r, trash: r, trees: r}
			state := domain.RevertState{Nodes: map[uuid.UUID]domain.NodeNow{r.node: {Node: domain.Node{
				ID: r.node, ParentID: &r.parent, Kind: domain.KindPage}}}}
			if items {
				state.Items = []domain.ChangesetItem{{NodeID: r.node, After: &domain.TreeState{Name: "Created"}}}
			}
			got, err := reads.rest(t.Context(), r.notebook, state)
			if err != nil || (got.Tree != nil) != items || len(got.Tree) != 0 {
				t.Fatalf("tree = %#v, %v; read=%v", got.Tree, err, items)
			}
			got, err = reads.sourceLines(t.Context(), got, domain.NotebookState{Mode: domain.ModeLLMWiki})
			if err != nil {
				t.Fatal(err)
			}
			if items {
				if r.treeCalls != 1 || r.lineCalls != 0 || got.Lines != nil {
					t.Fatalf("empty tree reread ancestry: trees=%d lines=%d state=%+v", r.treeCalls, r.lineCalls, got)
				}
			} else if r.treeCalls != 0 || r.lineCalls != 1 || !reflect.DeepEqual(got.Lines[r.parent], []domain.Ancestor{{ID: r.parent, Name: "raw"}}) {
				t.Fatalf("content-only ancestors = %+v; trees=%d lines=%d", got.Lines, r.treeCalls, r.lineCalls)
			}
		})
	}
}

func TestRevertReadsKeepTreeError(t *testing.T) {
	want := errors.New("tree read failed")
	r := &revertTreeReads{notebook: uuid.NewV7(), err: want}
	reads := revertReads{history: r, trees: r}
	state, err := reads.rest(t.Context(), r.notebook, domain.RevertState{Items: []domain.ChangesetItem{{NodeID: uuid.NewV7(), After: &domain.TreeState{Name: "Created"}}}})
	if !errors.Is(err, want) || state.Tree != nil {
		t.Fatalf("tree failure = %+v, %v; want original error and no partial tree", state, err)
	}
}
