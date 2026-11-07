package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// RebuildMetadata is borrowed only for the metadata write of one parsed page.
// It never carries the links or retains the parser's Facts after that call.
type RebuildMetadata struct {
	FrontmatterValid bool
	Tags             []domain.Tag
	Properties       []domain.Property
	Aliases          []domain.Alias
}

// RebuildWrites is private to the Rebuild maintenance protocol. The caller
// holds the notebook row and index locks and has deleted its old index rows.
// Merely being in a transaction does not establish that lifecycle.
// Ordinary Store.AddPage keeps its complete, unresolved insertion contract.
type RebuildWrites interface {
	AddRebuildMetadata(context.Context, Page, RebuildMetadata) error
	// A staged page already has its metadata; drain only its original links.
	AddRebuildUnresolvedLinks(context.Context, Page, []domain.Link) error
	// Every link has one fresh result, including those resolving to nothing.
	// Both slices are borrowed for this call and have the same length.
	AddRebuildResolvedLinks(context.Context, Page, []domain.Link, []domain.Resolution) error
}
