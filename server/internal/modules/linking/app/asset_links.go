package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Attachments is what an attachment's link reads of its notebook's nodes
// (M7/P3 design 4.6): bootstrap wires the page module's.
type Attachments interface {
	// Attachments is the attachments not deleted of notebookID among ids,
	// each with its path from the root and the number of notebookID's
	// attachments not deleted with its title key, itself among them, read
	// at once.
	Attachments(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]Attachment, error)
}

// Attachment is an attachment, and Alike the number of its notebook's
// attachments with its title key, itself among them.
type Attachment struct {
	Node  domain.Node
	Alike int
}

// AssetLinks writes how a wikilink is written to lead to attachments alone
// from anywhere in their notebook (M7/P3 design 4.6), as the link targets
// write it (domain.AssetLinktext): the asset module's link of each, which
// bootstrap wires. It reads in the transaction ctx carries, so that an
// upload's unit sees its own node, or on the pool outside one.
type AssetLinks struct {
	Attachments Attachments
}

// Of is the link of each of ids that is an attachment of notebookID not
// deleted, by id: its name when no other attachment of the notebook has
// its title key, its path from the root otherwise; empty for one without
// an extension, which no link leads to.
func (a AssetLinks) Of(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	ids = slices.SortedFunc(slices.Values(ids), uuid.UUID.Compare)
	attachments, err := a.Attachments.Attachments(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(attachments))
	for _, at := range attachments {
		out[at.Node.ID], _ = domain.AssetLinktext(at.Node, at.Alike)
	}
	return out, nil
}
