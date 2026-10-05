package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LinkTarget is a page as a notebook's completion offers it (M6/P5 design
// 6): its title, how a wikilink is written to lead to it alone, the
// rename's rewrite's writing, and its aliases, by key.
type LinkTarget struct {
	ID      uuid.UUID
	Name    string
	Link    string
	Aliases []string
}

// ListLinkTargets lists what a notebook's links may lead to, for the
// editor's completion: GET /api/v0/notebooks/{notebook_id}/link-targets
// (M6/P5 design 6).
type ListLinkTargets struct {
	Access Access
	Reads  Reads
}

// Execute returns the pages of the notebook id, by id, after the decision:
// the tree and the aliases in two statements, which the events have read
// again.
func (l ListLinkTargets) Execute(ctx context.Context, id uuid.UUID) ([]LinkTarget, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.Access.notebook(ctx, actor, id, domain.ActionListLinkTargets); err != nil {
		return nil, err
	}
	nodes, err := l.Access.Pages.All(ctx, id)
	if err != nil {
		return nil, err
	}
	aliases, err := l.Reads.NotebookAliases(ctx, id)
	if err != nil {
		return nil, err
	}
	links := domain.Linktexts(nodes)
	out := make([]LinkTarget, len(nodes))
	for i, n := range nodes {
		out[i] = LinkTarget{ID: n.ID, Name: n.Path[len(n.Path)-1].Name, Link: links[i], Aliases: aliases[n.ID]}
		if out[i].Aliases == nil {
			out[i].Aliases = []string{}
		}
	}
	return out, nil
}
