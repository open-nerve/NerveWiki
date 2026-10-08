package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// AssetLinks writes how a wikilink is written to lead to attachments alone
// from anywhere in their notebook (M7/P3 design 4.6), as the link targets
// write it (domain.Linktext): the asset module's link of each, which
// bootstrap wires. It reads in the transaction ctx carries, so that an
// upload's unit sees its own node, or on the pool outside one.
type AssetLinks struct {
	Pages Pages
}

// Of is the link of each of ids that is an attachment of notebookID not
// deleted, by id: its name when no other attachment of the notebook has
// its title key, its path from the root otherwise, the path too for one
// without an extension, which nothing leads to.
func (a AssetLinks) Of(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	ids = slices.SortedFunc(slices.Values(ids), uuid.UUID.Compare)
	nodes, err := a.Pages.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	nodes = slices.DeleteFunc(nodes, func(n domain.Node) bool { return !n.Asset })
	if len(nodes) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	keys := make([]string, len(nodes))
	for i, n := range nodes {
		keys[i] = n.Path[len(n.Path)-1].Key
	}
	slices.Sort(keys)
	named, err := a.Pages.ByKeys(ctx, notebookID, slices.Compact(keys))
	if err != nil {
		return nil, err
	}
	tree := domain.Tree{Named: make(map[string][]domain.Node)}
	for _, n := range named {
		key := n.Path[len(n.Path)-1].Key
		tree.Named[key] = append(tree.Named[key], n)
	}
	out := make(map[uuid.UUID]string, len(nodes))
	for _, n := range nodes {
		text, ok := domain.Linktext(n, nil, tree)
		if !ok {
			text = domain.Linktexts([]domain.Node{n})[0]
		}
		out[n.ID] = text
	}
	return out, nil
}
