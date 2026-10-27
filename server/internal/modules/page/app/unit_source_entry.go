package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// subtreeForMove checks the known root's protection and chooses the finite
// reader only for an existing subtree entering sources. The caller has checked
// parameters, cycle and title using its destination line. Overflow precedes
// the depth of unread descendants, and all hash, asset and write work.
func (u *Unit) subtreeForMove(ctx context.Context, before, after domain.Node, line []domain.Ancestor) (domain.Subtree, *StepLines, error) {
	if u.write.Notebook.Mode != domain.ModeLLMWiki {
		sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, before.ID)
		return sub, nil, err
	}
	lines := &StepLines{After: line}
	if domain.SameParent(before.ParentID, after.ParentID) {
		lines.Before = line
	} else if before.ParentID != nil {
		var err error
		lines.Before, err = u.w.d.Nodes.Ancestors(ctx, before.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	from, to := before.State(), after.State()
	facts := domain.FactsOf(domain.OpMove, u.write.Kind, []domain.Change{{NodeID: before.ID, Before: &from, After: &to}})
	facts.Nodes[0].Kind = before.Kind
	facts = domain.SourceFacts(facts, u.write.Notebook.SourceRootID, lines.Before, lines.After, nil)
	if verdict := domain.CheckContract(u.write.Notebook, facts); verdict.Err != nil {
		return nil, nil, domain.Refuse(verdict.Err, verdict.NodeID)
	}
	root := facts.Nodes[0]
	if !root.Before.InArea && root.After.InArea {
		sub, complete, err := u.w.d.Nodes.SubtreeWithin(ctx, u.write.NotebookID, before.ID, domain.MaxUnitNodes)
		if err != nil {
			return nil, nil, err
		}
		if !complete {
			return nil, nil, domain.Refuse(domain.ErrSourceTooLarge, before.ID)
		}
		return sub, lines, nil
	}
	sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, before.ID)
	return sub, lines, err
}
