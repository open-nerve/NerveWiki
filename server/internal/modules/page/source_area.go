package page

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
)

// SourceArea is the source area's cleanup port. Bootstrap hands it to the
// notebook downgrade without constructing the page module or its writer.
type SourceArea struct {
	store *postgresadapter.Store
}

// NewSourceArea returns the source port over pool. Its statements use the
// caller's transaction, including the downgrade's notebook lock.
func NewSourceArea(pool *pgxpool.Pool) SourceArea {
	return SourceArea{store: postgresadapter.New(pool)}
}

// Forget clears the live source hashes before the mode's subscribers run.
func (s SourceArea) Forget(ctx context.Context, notebook uuid.UUID) error {
	return s.store.ForgetSources(ctx, notebook)
}

// SourceLockKey exposes the notebook key for observed lock interleavings.
// The first advisory key is 0x73726365 ("srce").
func SourceLockKey(notebook uuid.UUID) int32 {
	return postgresadapter.SourceLockKey(notebook)
}
