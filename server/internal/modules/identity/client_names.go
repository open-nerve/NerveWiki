package identity

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
)

// ClientNames remembers the client name each personal access token's MCP
// client last told (M9 design 4.5): a call that tells none, after a
// restart too, is still its client's. Each read and write is one
// statement, unlocked, outside the lock order, as TouchAPIToken is.
type ClientNames struct{ store *postgresadapter.Store }

// NewClientNames returns ClientNames over pool alone: bootstrap wires it
// to mcpserver, cleaning the names with shared.CleanClientName first.
func NewClientNames(pool *pgxpool.Pool) ClientNames {
	return ClientNames{store: postgresadapter.New(pool)}
}

// Recorded is the name tokenID last told; "" for none, and for no such
// token.
func (c ClientNames) Recorded(ctx context.Context, tokenID uuid.UUID) (string, error) {
	return c.store.APITokenClientName(ctx, tokenID)
}

// Record writes name, cleaned and not empty, as tokenID's when it
// differs: written is false when no row was written, as when the row
// already holds name, or another transaction holds it and it is skipped
// as TouchAPIToken skips it, or the token is gone.
func (c ClientNames) Record(ctx context.Context, tokenID uuid.UUID, name string) (written bool, err error) {
	return c.store.RecordAPITokenClientName(ctx, tokenID, name)
}
