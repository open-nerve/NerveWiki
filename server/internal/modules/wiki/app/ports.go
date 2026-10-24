// Package app holds the wiki module's use cases and the ports they need.
package app

import (
	"context"
	"time"
	"uuid"
)

// Seeds are the page module's template writes, as the composition root
// passes them (M10 design 4.4): a notebook's pages in its creation's
// transaction, one unit; and a template's contents parsed once.
type Seeds interface {
	Seed(ctx context.Context, s Seed) error
	Parse(content string) (Parsed, error)
}

// Seed is a template's unit: notebook, time, client, the changeset's
// message, its pages in order.
type Seed struct {
	NotebookID uuid.UUID
	At         time.Time
	Client     string
	Message    string
	Pages      []SeedPage
}

// SeedPage is a page of a seed: its id, the zero one for a new id, its
// title and content, and the facts Parse answered for it.
type SeedPage struct {
	ID      uuid.UUID
	Title   string
	Content string
	Parsed  Parsed
}

// Parsed is a parsed content as the page module answers it, which the
// wiki module passes back and never reads (page.Facts, itself an opaque
// any).
type Parsed struct{ Facts any }
