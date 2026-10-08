package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LinkTarget is a page or an attachment as a notebook's completion offers
// it (M6/P5 design 6; M7/P3 design 4.5): its title or name, how a wikilink
// is written to lead to it alone, the rename's rewrite's writing, and a
// page's aliases, by key; Asset tells an attachment.
type LinkTarget struct {
	ID      uuid.UUID
	Asset   bool
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

// Execute returns the pages and the attachments with an extension of the
// notebook id, by id, after the decision: the tree and the aliases in two
// statements, which the events have read again.
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
	out := make([]LinkTarget, 0, len(nodes))
	for i, n := range nodes {
		// An attachment without an extension is no link's target (M7/P3
		// design 4.5): its path may lead to a page of that path.
		if !n.Linkable() {
			continue
		}
		t := LinkTarget{ID: n.ID, Asset: n.Asset, Name: n.Path[len(n.Path)-1].Name, Link: links[i], Aliases: aliases[n.ID]}
		if t.Aliases == nil {
			t.Aliases = []string{}
		}
		out = append(out, t)
	}
	return out, nil
}
