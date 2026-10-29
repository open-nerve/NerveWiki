package app

import (
	"context"
	"iter"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (w *JobWrites) planRestoreSource(ctx context.Context, spec JobSpec, root domain.TrashRoot, notebook domain.NotebookState) (JobPlan, error) {
	precheck := w.writer.d.SourcePrecheck
	if precheck == nil {
		return w.planRestoreRead(ctx, spec, root)
	}
	var plan JobPlan
	err := precheck.d.Snapshots.WithinSnapshot(ctx, func(ctx context.Context) error {
		var err error
		plan, err = w.planRestoreRead(ctx, spec, root)
		if err != nil {
			return err
		}
		return checkRestoreSources(ctx, precheck, w.reads, spec.NotebookID, notebook, root, nil)
	})
	return plan, err
}

// The root's time and exact group are checked again within this snapshot;
// the mode was read once after admission. No parser, locks or queue join it.
func checkRestoreSources(ctx context.Context, precheck *SourcePrecheck, reads TrashReads, notebookID uuid.UUID, notebook domain.NotebookState, root domain.TrashRoot, expected []domain.TrashMember) error {
	current, err := reads.Nodes.FindAnyNode(ctx, root.Node.ID)
	if err != nil {
		return found(err, domain.ErrTrashChanged)
	}
	if current.NotebookID != notebookID || current.DeletedAt == nil || !current.DeletedAt.Equal(root.DeletedAt) || !current.TrashRoot {
		return domain.ErrTrashChanged
	}
	group, err := reads.Trash.TrashGroup(ctx, notebookID, root.Node.ID, root.DeletedAt)
	if err != nil {
		return found(err, domain.ErrTrashChanged)
	}
	if expected != nil && !sameGroup(group, expected) {
		return domain.ErrTrashChanged
	}
	if len(group) != root.Pages+root.Assets {
		return domain.ErrTrashChanged
	}
	lines, err := reads.Trash.TrashLines(ctx, []uuid.UUID{root.Node.ID})
	if err != nil {
		return err
	}
	place := domain.PlacementOf(lines[root.Node.ID], groupHeight(group))
	line := restoredLines(place.ParentID, lines[root.Node.ID])
	facts := domain.StepFacts{Operation: domain.OpRestore, Kind: domain.ChangesetTrashRestore, Nodes: make([]domain.NodeFacts, len(group))}
	for i, m := range group {
		parent := m.Node.ParentID
		if i == 0 {
			parent = place.ParentID
		}
		facts.Nodes[i] = domain.NodeFacts{ID: m.Node.ID, Kind: m.Node.Kind, After: &domain.NodePlace{ParentID: parent, Key: shared.TitleKey(m.Node.Name)}}
	}
	facts = domain.SourceFacts(facts, notebook.SourceRootID, nil, line.After, nil)
	sequence := iter.Seq[domain.StepFacts](func(yield func(domain.StepFacts) bool) { yield(facts) })
	// Restore arrivals never need manage_source; keep the contract's decision
	// explicit should a future pure rule require it.
	return precheck.check(ctx, notebookID, notebook, sequence, func(shared.Action) error { return shared.Forbidden() })
}
