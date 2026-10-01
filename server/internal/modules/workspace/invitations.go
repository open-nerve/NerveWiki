package workspace

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/mac"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
)

// InvitationKeyInfo is the HKDF info bootstrap derives the invitations' MAC
// key with from the instance's signing key (M2/P3 design 3.2).
const InvitationKeyInfo = "nervewiki workspace-invitation mac v1"

// InvitationCheck is what identity's sign-up policy asks of the module,
// through bootstrap (M2/P3 design 3.6).
type InvitationCheck interface {
	// Admits reports whether token is the invitation id's, the invitation
	// is pending, its workspace not deleted, and it was sent to email, a
	// normalized address.
	Admits(ctx context.Context, id uuid.UUID, token, email string) (bool, error)
}

// NewInvitationCheck returns the InvitationCheck over pool alone, with key,
// the invitations' MAC key: bootstrap builds it before identity, whose
// sign-up policy holds it.
func NewInvitationCheck(pool *pgxpool.Pool, key []byte) InvitationCheck {
	return app.CheckInvitation{Tokens: macadapter.New(key), Invitations: postgresadapter.New(pool)}
}
