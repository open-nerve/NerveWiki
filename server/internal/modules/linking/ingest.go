package linking

import (
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// The composition root converts the page owner's transaction-bound ports.
type (
	IngestNotebook   = app.IngestNotebook
	IngestNodeBefore = app.IngestNodeBefore
	IngestBefore     = app.IngestBefore
	IngestHead       = app.IngestHead
	IngestHeadBatch  = app.IngestHeadBatch
	IngestMetadata   = app.IngestMetadata
	IngestMarks      = app.IngestMarks
	IngestMarkResult = app.IngestMarkResult
)
type IngestDeps struct {
	Metadata        IngestMetadata
	Marks           IngestMarks
	MaxContentBytes int
	Logger          *slog.Logger
}
