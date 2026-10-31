package page

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NodeKind distinguishes a page from an attachment in compact metadata.
type NodeKind = domain.Kind

const (
	KindPage  = domain.KindPage
	KindAsset = domain.KindAsset
)

type (
	// LintNode is the page-owned compact fact; no body or display path is read.
	LintNode = app.LintNode
	// SourceNode is a strict live source-area member, including empty pages.
	SourceNode = app.SourceNode
	// SourceData combines complete metadata with its strict source membership.
	SourceData = app.SourceData
)

// LintNodes reads all live candidates, independent of source/index eligibility.
type LintNodes struct {
	store *postgresadapter.Store
}

// NewLintNodes uses the caller's transaction when its context carries one.
func NewLintNodes(pool *pgxpool.Pool) LintNodes {
	return LintNodes{store: postgresadapter.New(pool)}
}

// Nodes returns compact metadata of the whole live notebook. Calling this and
// SourceArea.Nodes separately performs two metadata reads in the same snapshot.
func (l LintNodes) Nodes(ctx context.Context, notebook uuid.UUID) ([]LintNode, error) {
	return l.store.LintNodes(ctx, notebook)
}
