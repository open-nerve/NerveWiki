// Package postgresadapter is the wiki module's tables (M11 design 4.2):
// its notebooks' upgrades.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Store is the module's tables over the pool, each statement in the
// transaction ctx carries when there is one.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// Insert writes u, upgrading, started by by at at (M11 design 4.4 step
// 14).
func (s *Store) Insert(ctx context.Context, u domain.Upgrade, by uuid.UUID, at time.Time) error {
	err := s.queries(ctx).InsertUpgrade(ctx, gen.InsertUpgradeParams{
		ID: u.ID, NotebookID: u.NotebookID, SourceRootID: u.SourceRootID, SeededRoot: u.SeededRoot,
		SeededAgentsID: u.SeededAgentsID, Locale: string(u.Locale), BaselineNodes: int32(u.BaselineNodes),
		BaselinePages: int32(u.BaselinePages), BaselineWanted: int32(u.BaselineWanted),
		BaselineWantedCapped: u.BaselineWantedCapped, By: by, Now: at,
	})
	if err != nil {
		return fmt.Errorf("insert an upgrade: %w", err)
	}
	return nil
}

// Current is the notebook's upgrade under way; false for none.
func (s *Store) Current(ctx context.Context, notebookID uuid.UUID) (domain.Upgrade, bool, error) {
	r, err := s.queries(ctx).CurrentUpgrade(ctx, notebookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Upgrade{}, false, nil
	}
	if err != nil {
		return domain.Upgrade{}, false, fmt.Errorf("read the notebook's upgrade: %w", err)
	}
	u := domain.Upgrade{
		ID: r.ID, NotebookID: r.NotebookID, State: domain.UpgradeState(r.State), SourceRootID: r.SourceRootID,
		SeededRoot: r.SeededRoot, SeededAgentsID: r.SeededAgentsID, Locale: domain.Locale(r.Locale),
		BaselineNodes: int(r.BaselineNodes), BaselinePages: int(r.BaselinePages), BaselineWanted: int(r.BaselineWanted),
		BaselineWantedCapped: r.BaselineWantedCapped, SubmittedBy: r.SubmittedByID, SubmittedAt: r.SubmittedAt,
		RollbackJobID: r.RollbackJobID, RollbackComplete: r.RollbackComplete, CreatedBy: r.CreatedByID, CreatedAt: r.CreatedAt,
	}
	if r.Ending != nil {
		u.Ending = domain.UpgradeEnding(*r.Ending)
	}
	if r.RollbackLeftNodes != nil && r.RollbackLeftPages != nil && r.RollbackLeftCreated != nil {
		u.RollbackLeft = &domain.RollbackLeft{Nodes: int(*r.RollbackLeftNodes), Pages: int(*r.RollbackLeftPages),
			Created: int(*r.RollbackLeftCreated)}
	}
	return u, true, nil
}

// SoftDeleteOf deletes the notebooks' upgrades, under way or ended, by by
// at at: their deletion's (M11/P1 design 3.15).
func (s *Store) SoftDeleteOf(ctx context.Context, notebookIDs []uuid.UUID, by uuid.UUID, at time.Time) error {
	err := s.queries(ctx).SoftDeleteUpgradesOf(ctx, gen.SoftDeleteUpgradesOfParams{NotebookIds: notebookIDs, By: by, At: at})
	if err != nil {
		return fmt.Errorf("delete the notebooks' upgrades: %w", err)
	}
	return nil
}

// PurgeUpgrades purges up to batch upgrades deleted before before, those
// of the deleted notebooks, skipping those another transaction holds, and
// returns how many it deleted (v0.1 design 13.1, item 6).
func (s *Store) PurgeUpgrades(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeUpgrades(ctx, gen.PurgeUpgradesParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge upgrades: %w", err)
	}
	return int(n), nil
}
