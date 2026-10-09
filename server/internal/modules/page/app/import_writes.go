package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ImportWrites are an import's writes of the tree (M7/P6 design 3.3): its
// pages' contents checked and parsed before their units, and its units,
// which create pages and attachments alone, each of the import kind, the
// later ones merged into the first one's changeset.
type ImportWrites struct {
	writer *Writer
	parser *ContentParser
	md     Markdown
}

// NewImportWrites returns them, writing through writer, parsing with
// parser, counting the links of md's facts.
func NewImportWrites(writer *Writer, parser *ContentParser, md Markdown) *ImportWrites {
	return &ImportWrites{writer: writer, parser: parser, md: md}
}

// ImportSpec is an import's unit: in NotebookID, deciding on Action, from
// Client, the job's; merged into Changeset when it is set.
type ImportSpec struct {
	NotebookID uuid.UUID
	Action     shared.Action
	Client     domain.Client
	Changeset  uuid.UUID
}

// Parsed is a page's content parsed for an import's unit: its facts, how
// many links the content holds, and the release of the parse budget its
// facts keep, which the import calls once the unit is over.
type Parsed struct {
	Facts   Facts
	Links   int
	release func()
}

// Release gives back what the facts keep of the parse budget.
func (p Parsed) Release() {
	if p.release != nil {
		p.release()
	}
}

// CheckContent checks a page's content as its unit would: 422 on content
// past 5 MiB, not UTF-8, or holding NUL. It reads nothing.
func (w *ImportWrites) CheckContent(content string) error {
	return domain.CheckContent("content", content)
}

// Parse parses content, which CheckContent passed, for a unit of the
// import, outside it (v0.1 design 13.1, item 19): the parse budget holds
// its bytes as it is parsed, and the facts' share until Release; a budget
// that does not free up in time is 503 server_busy, which the import tries
// again.
func (w *ImportWrites) Parse(ctx context.Context, content string) (Parsed, error) {
	facts, release, err := w.parser.Decided(ctx, content)
	if err != nil {
		return Parsed{}, err
	}
	return Parsed{Facts: facts, Links: w.md.Links(facts), release: release}, nil
}

// Import runs do in a unit of spec that changes the tree, of the import
// kind, and answers its changeset: the one it merged into, the one it
// created, or none when it wrote nothing. A notebook the job cannot see
// is notebook.not_found.
func (w *ImportWrites) Import(ctx context.Context, spec ImportSpec, do func(ctx context.Context, u *ImportUnit) error) (uuid.UUID, error) {
	outcome, err := w.writer.Run(ctx, UnitSpec{
		NotebookID: spec.NotebookID, Action: spec.Action, Tree: true, Client: spec.Client, NotFound: domain.ErrNotebookNotFound,
		Kind: domain.ChangesetImport, Changeset: spec.Changeset,
	}, func(ctx context.Context, u *Unit) error {
		return do(ctx, &ImportUnit{u: u, under: map[uuid.UUID]*children{}, lines: map[uuid.UUID][]domain.Ancestor{}})
	})
	return outcome.ChangesetID, err
}
