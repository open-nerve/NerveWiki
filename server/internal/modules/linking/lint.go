package linking

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// Lint's read projections contain no page content or display paths.
type (
	LintReads         = app.LintReads
	IndexedHead       = app.IndexedHead
	EdgePos           = app.EdgePos
	LintEdge          = app.LintEdge
	SourcePropertyPos = app.SourcePropertyPos
	SourceProperty    = app.SourceProperty
	SourceSpec        = app.SourceSpec
	SourceSlotEdge    = app.SourceSlotEdge
	BodyReference     = app.BodyReference
)

// NewLintReads reads the linking index in the caller's snapshot. Bootstrap
// supplies page.MaxContentBytes; this is not a new user-configurable limit.
func NewLintReads(pool *pgxpool.Pool, maxContentBytes int) *LintReads {
	return app.NewLintReads(postgresadapter.NewLintReads(pool, maxContentBytes), maxContentBytes)
}
