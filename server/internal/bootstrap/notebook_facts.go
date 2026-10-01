package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
)

// notebookFacts is the notebook module's Facts as the access module reads
// them: the two modules do not import each other, so their types, alike
// field by field, meet here.
type notebookFacts struct {
	notebook.Facts
}

// NotebookFacts implements access.NotebookFacts.
func (f notebookFacts) NotebookFacts(ctx context.Context, notebookID, userID uuid.UUID) (access.NotebookFact, error) {
	fact, err := f.Facts.NotebookFacts(ctx, notebookID, userID)
	return access.NotebookFact(fact), err
}
