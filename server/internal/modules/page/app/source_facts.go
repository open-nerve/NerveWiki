package app

import (
	"context"
	"maps"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// withSourceLines completes only the operation root's missing ancestry.
// Children take their parents from Changes; standard notebooks do no reads.
func (u *Unit) withSourceLines(ctx context.Context, s Step) (Step, error) {
	if s.Notebook.Mode != domain.ModeLLMWiki || len(s.Changes) == 0 {
		return s, nil
	}
	if s.Operation == domain.OpRestore {
		return u.restoreSourceLines(ctx, s)
	}
	if s.Lines != nil {
		return s, nil
	}
	root := s.Changes[0]
	lines := &StepLines{}
	if root.Before != nil && root.Before.ParentID != nil {
		line, err := u.w.d.Nodes.Ancestors(ctx, root.NodeID)
		if err != nil {
			return Step{}, err
		}
		lines.Before = line
	}
	if root.After != nil {
		if root.Before != nil && domain.SameParent(root.Before.ParentID, root.After.ParentID) {
			lines.After = lines.Before
		} else if root.After.ParentID != nil {
			line, err := u.lineUnder(ctx, root.After.ParentID)
			if err != nil {
				return Step{}, err
			}
			lines.After = line
		}
	}
	s.Lines = lines
	return s, nil
}

// contractFacts carries already-read facts into the single domain contract.
func (s Step) contractFacts() domain.StepFacts {
	facts := domain.FactsOf(s.Operation, s.Kind, s.Changes)
	for i := range facts.Nodes {
		facts.Nodes[i].Kind = s.Kinds[facts.Nodes[i].ID]
	}
	if s.Content != nil {
		facts.Content = &domain.ContentFacts{Hash: s.Content.Hash, Bytes: s.Content.Bytes, BeforeBytes: s.Content.BeforeBytes, Merged: s.Content.Merged}
	}
	facts.Rewrite, facts.EditSession = s.Rewrite, s.EditSessionID != uuid.Nil()
	if s.Notebook.Mode == domain.ModeLLMWiki && s.Lines != nil {
		facts = domain.SourceFacts(facts, s.Notebook.SourceRootID, s.Lines.Before, s.Lines.After, s.Lines.AfterParents)
	}
	return facts
}

// restoreSourceLines handles the forest left when a batch skips members that
// are already alive. Each external parent keeps its own line; the nodes this
// step restores obtain their parent relation from Changes. Existing placement
// and continuation reads are reused, and only an unknown external root reads.
func (u *Unit) restoreSourceLines(ctx context.Context, s Step) (Step, error) {
	lines := &StepLines{AfterParents: map[uuid.UUID][]domain.Ancestor{}}
	if s.Lines != nil {
		lines.Before, lines.After = s.Lines.Before, s.Lines.After
		maps.Copy(lines.AfterParents, s.Lines.AfterParents)
	}
	first := s.Changes[0].After
	if first != nil && first.ParentID != nil && len(lines.After) > 0 {
		lines.AfterParents[*first.ParentID] = lines.After
	}
	changed := make(map[uuid.UUID]bool, len(s.Changes))
	for _, c := range s.Changes {
		changed[c.NodeID] = true
	}
	for _, c := range s.Changes {
		if c.After == nil || c.After.ParentID == nil || changed[*c.After.ParentID] {
			continue
		}
		parent := *c.After.ParentID
		if _, ok := lines.AfterParents[parent]; ok {
			continue
		}
		line, err := u.lineUnder(ctx, &parent)
		if err != nil {
			return Step{}, err
		}
		lines.AfterParents[parent] = line
	}
	if first != nil && first.ParentID != nil {
		lines.After = lines.AfterParents[*first.ParentID]
	}
	s.Lines = lines
	return s, nil
}
