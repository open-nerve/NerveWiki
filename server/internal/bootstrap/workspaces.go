package bootstrap

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// WorkspaceCommand is one `nervewiki workspaces` command on the workspace
// module's administrator use cases: it runs and returns the line it
// prints.
type WorkspaceCommand func(ctx context.Context, admin *workspace.Admin) (string, error)

// Workspaces runs cmd on the command line's composition (M2/P4 design 3.3):
// a pool, identity's accounts and the workspace module's administrator use
// cases, with the restore's registrants; no HTTP server, authorizer,
// signing key or jobs client. The command's line goes to out, the logs to
// logOut. An error is one line for the administrator, and the database is
// unchanged.
func Workspaces(ctx context.Context, cfg config.Config, logOut, out io.Writer, cmd WorkspaceCommand) error {
	c, err := openAdminCommand(ctx, cfg, logOut)
	if err != nil {
		return err
	}
	defer c.close()
	ext := workspaceRegistrants(c.pool)
	admin := workspace.NewAdmin(workspace.AdminDeps{
		Pool:                         c.pool,
		Tx:                           postgres.NewTxManager(c.pool, cfg.Database.CommitTimeout),
		Clock:                        clock.System{},
		Logger:                       c.logger,
		Accounts:                     identity.NewAccounts(c.pool),
		MembershipRestoreSubscribers: ext.restoreSubscribers,
	})
	line, err := cmd(ctx, admin)
	return printResult(out, line, err)
}

// CreateWorkspace is `nervewiki workspaces create`.
func CreateWorkspace(slug, name, adminEmail string) WorkspaceCommand {
	return func(ctx context.Context, admin *workspace.Admin) (string, error) {
		w, err := admin.CreateWorkspace(ctx, name, slug, adminEmail)
		return fmt.Sprintf("created workspace %s (%s) with admin %s", w.Slug, w.ID, shared.NormalizeEmail(adminEmail)), err
	}
}

// ReactivateMember is `nervewiki workspaces reactivate-member`.
func ReactivateMember(slug, email string) WorkspaceCommand {
	return func(ctx context.Context, admin *workspace.Admin) (string, error) {
		r, err := admin.ReactivateMember(ctx, slug, email)
		email := shared.NormalizeEmail(email)
		if r.Already {
			return email + " is already a member of " + r.Slug, err
		}
		return fmt.Sprintf("reactivated %s in %s as %s; the membership had ended at %s",
			email, r.Slug, r.Role, r.EndedAt.UTC().Format(time.RFC3339)), err
	}
}
