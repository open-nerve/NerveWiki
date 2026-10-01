package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// byCLI marks in the logs what the server's administrator did with
// nervewiki workspaces (v0.1 design 13.1, item 22).
const byCLI = "cli"

// CreateWorkspaceForDeps are what CreateWorkspaceFor needs.
type CreateWorkspaceForDeps struct {
	Workspaces WorkspaceCreator
	Accounts   AccountsByEmail
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
}

// CreateWorkspaceFor creates a workspace whose admin is an account named
// by its address, for the server's administrator: nervewiki workspaces
// create (M2/P4 design 3.3). workspace.creation_enabled does not apply:
// while it is off, workspaces are made this way.
type CreateWorkspaceFor struct {
	d CreateWorkspaceForDeps
}

// NewCreateWorkspaceFor returns the use case.
func NewCreateWorkspaceFor(d CreateWorkspaceForDeps) *CreateWorkspaceFor {
	return &CreateWorkspaceFor{d: d}
}

// Execute creates the workspace name and slug with the account of
// adminEmail as its admin. The values' problems come together (422),
// before the transaction, whose first statement shares the account's row:
// identity.account_not_found or identity.account_deactivated, and nothing
// is written.
func (c *CreateWorkspaceFor) Execute(ctx context.Context, name, slug, adminEmail string) (domain.Workspace, error) {
	draft, err := domain.CheckDraft(name, slug)
	if err != nil {
		return domain.Workspace{}, err
	}
	w, admin := newWorkspace(draft, uuid.Nil(), c.d.Clock.Now()) // the admin's account is known under its lock
	err = c.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if admin.UserID, err = c.d.Accounts.ShareActiveAccountByEmail(ctx, adminEmail); err != nil {
			return err
		}
		return create(ctx, c.d.Workspaces, w, admin)
	})
	if err != nil {
		return domain.Workspace{}, err
	}
	c.d.Logger.InfoContext(ctx, "workspace created", slog.String("workspace_id", w.ID.String()),
		slog.String("user_id", admin.UserID.String()), slog.String("by", byCLI))
	return w, nil
}
