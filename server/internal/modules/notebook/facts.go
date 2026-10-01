package notebook

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// Fact is what an account has of a notebook: whether it is there and not
// deleted, its workspace, its workspace access, and the role of the
// account's active membership of it, "" for none.
type Fact = app.Fact

// Facts reads the facts of the access module's notebook level, which
// bootstrap wires to it.
type Facts interface {
	// NotebookFacts returns what userID has of notebookID, in the
	// transaction ctx carries.
	NotebookFacts(ctx context.Context, notebookID, userID uuid.UUID) (Fact, error)
}

// NewFacts returns Facts over pool alone: bootstrap builds the access
// module, and the Authorizer the notebook module takes, before the module
// itself.
func NewFacts(pool *pgxpool.Pool) Facts {
	return postgresadapter.New(pool)
}
