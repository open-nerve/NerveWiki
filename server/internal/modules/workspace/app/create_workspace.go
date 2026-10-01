package app

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CreateWorkspaceDeps are what CreateWorkspace needs.
type CreateWorkspaceDeps struct {
	Workspaces WorkspaceCreator
	Accounts   Accounts
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
	// CreationEnabled is workspace.creation_enabled.
	CreationEnabled bool
}

// CreateWorkspace creates a workspace whose admin is the caller:
// POST /api/v0/workspaces (M2/P1 design 3.7).
type CreateWorkspace struct {
	d CreateWorkspaceDeps
}

// NewCreateWorkspace returns the use case.
func NewCreateWorkspace(d CreateWorkspaceDeps) *CreateWorkspace {
	return &CreateWorkspace{d: d}
}

// Execute creates the workspace name and slug. While creation is off it
// answers workspace.creation_disabled before the values are checked; the
// values' problems come together (422), before the transaction. The
// transaction's first statement shares the caller's account row: a
// concurrent deactivation waits for the new membership, or the creation
// sees the account deactivated (v0.1 design 13.1, item 18).
func (c *CreateWorkspace) Execute(ctx context.Context, name, slug string) (Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Membership{}, err
	}
	if !c.d.CreationEnabled {
		return Membership{}, domain.ErrCreationDisabled
	}
	draft, err := domain.CheckDraft(name, slug)
	if err != nil {
		return Membership{}, err
	}
	w, admin := newWorkspace(draft, actor.UserID, c.d.Clock.Now())
	err = c.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := c.d.Accounts.ShareActiveAccount(ctx, actor.UserID); err != nil {
			return err
		}
		return create(ctx, c.d.Workspaces, w, admin)
	})
	if err != nil {
		return Membership{}, err
	}
	c.d.Logger.InfoContext(ctx, "workspace created",
		slog.String("workspace_id", w.ID.String()), slog.String("user_id", actor.UserID.String()))
	return Membership{Workspace: w, Role: admin.Role}, nil
}

// newWorkspace is the workspace of draft created at now, and its admin's
// membership, of the account userID: their ids come before the
// transaction.
func newWorkspace(draft domain.Draft, userID uuid.UUID, now time.Time) (domain.Workspace, domain.Member) {
	w := domain.Workspace{ID: uuid.NewV7(), Slug: draft.Slug, Name: draft.Name, CreatedAt: now, UpdatedAt: now}
	return w, domain.Member{ID: uuid.NewV7(), WorkspaceID: w.ID, UserID: userID, Role: shared.WorkspaceAdmin, CreatedAt: now}
}

// create writes w and its admin's membership, by the admin: what follows
// the share of the admin's account in a creation's transaction.
func create(ctx context.Context, store WorkspaceCreator, w domain.Workspace, admin domain.Member) error {
	if err := store.CreateWorkspace(ctx, w, admin.UserID); err != nil {
		return err
	}
	return store.AddMember(ctx, admin, admin.UserID)
}
