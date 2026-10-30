package linking

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
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
	LintRules         = app.Rules
	PreparedRules     = app.Prepared
	ReadInput         = app.ReadInput
	CompactNode       = app.CompactNode
	RuleQuery         = app.RuleQuery
	AssetSizes        = app.AssetSizes
	Rule              = domain.Rule
	Count             = domain.Count
	Item              = domain.Item
	Position          = domain.Position
	NodeKind          = domain.NodeKind
	LintMode          = domain.LintMode
	CursorBinding     = domain.CursorBinding
	LintCursor        = domain.LintCursor
)

const (
	Wanted             = domain.Wanted
	Orphans            = domain.Orphans
	Ambiguous          = domain.Ambiguous
	InvalidFrontmatter = domain.InvalidFrontmatter
	MissingType        = domain.MissingType
	MissingSummary     = domain.MissingSummary
	UndigestedSources  = domain.UndigestedSources
	InvalidSources     = domain.InvalidSources
	NodePage           = domain.NodePage
	NodeAsset          = domain.NodeAsset
	ModeStandard       = domain.ModeStandard
	ModeLLM            = domain.ModeLLM
)

// NewLintReads reads the linking index in the caller's snapshot. Bootstrap
// supplies page.MaxContentBytes; this is not a new user-configurable limit.
func NewLintReads(pool *pgxpool.Pool, maxContentBytes int) *LintReads {
	return app.NewLintReads(postgresadapter.NewLintReads(pool, maxContentBytes), maxContentBytes)
}

// NewLintRules prepares index rules inside the caller's snapshot. Its caller
// converts page metadata and asset infos at the module boundary; this entry
// does not construct a wiki runtime or register an HTTP/MCP operation.
func NewLintRules(pool *pgxpool.Pool, maxContentBytes int, assets AssetSizes) *LintRules {
	return app.NewRules(NewLintReads(pool, maxContentBytes), postgresadapter.New(pool), assets)
}

// DecodeLintCursor validates pure shape before a caller performs its own
// authorization and invokes PositionFor to check the requested binding.
func DecodeLintCursor(raw string) (LintCursor, error) {
	return domain.DecodeLintCursor(raw)
}

// EncodeLintCursor encodes an actual returned position or the dedicated start.
func EncodeLintCursor(binding CursorBinding, rule Rule, position Position) (string, error) {
	return domain.EncodeLintCursor(binding, rule, position)
}
