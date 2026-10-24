package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SeedWrites are the template's writes (M10 design 4.4): a notebook's pages
// written in its creation's transaction, their contents parsed once, at
// start-up.
type SeedWrites struct {
	writer *Writer
	md     Markdown
}

// NewSeedWrites returns them, writing through writer, parsing with md.
func NewSeedWrites(writer *Writer, md Markdown) *SeedWrites {
	return &SeedWrites{writer: writer, md: md}
}

// SeedSpec is a template's unit: in NotebookID, at At, from Client, its
// changeset's message Message, writing Pages in order, each created last
// among its siblings. The actor is ctx's, as Run's (v0.1 design 13.1
// item 1; M10 design review A2-N3).
type SeedSpec struct {
	NotebookID uuid.UUID
	At         time.Time
	Client     domain.Client
	Message    *string
	Pages      []PageDraft
}

// Seed writes spec's pages in one unit that changes the tree, of the edit
// kind, deciding page.create, within the transaction ctx carries: one
// changeset, one time. It answers the ids of the pages it wrote, in
// spec's order. A notebook the caller cannot see is notebook.not_found;
// any refusal of the unit, a guard's too, is answered as it is, its
// caller's transaction then to roll back.
func (s *SeedWrites) Seed(ctx context.Context, spec SeedSpec) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	_, err := s.writer.Seed(ctx, UnitSpec{
		NotebookID: spec.NotebookID, Action: domain.ActionCreate, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Message: spec.Message,
	}, spec.At, func(ctx context.Context, u *Unit) error {
		ids = ids[:0]
		for _, p := range spec.Pages {
			n, err := u.CreatePage(ctx, p)
			if err != nil {
				return err
			}
			ids = append(ids, n.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ParseTemplate checks and parses content, a template's page that the
// server writes at each creation of its kind, once, at start-up: outside
// the parse budget, before any decision (v0.1 design 13.3 item 3's
// exception for it; 13.1 item 19: the creation's transaction only locks,
// compares and writes). Its facts are static: a unit's guards,
// participants and observers read them, never write them. 422 for a
// content CheckContent refuses.
func (s *SeedWrites) ParseTemplate(content string) (Facts, error) {
	if err := domain.CheckContent("content", content); err != nil {
		return nil, err
	}
	return s.md.Facts(content), nil
}
