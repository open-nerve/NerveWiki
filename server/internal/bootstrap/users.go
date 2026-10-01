package bootstrap

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// UserCommand is one `nervewiki users` command on the administrator's use
// cases: it runs and returns the line it prints.
type UserCommand func(ctx context.Context, admin *identity.Admin) (string, error)

// Users runs cmd on the command line's composition (M1/P4 design 3.6): a
// pool and identity's administrator use cases, with the deactivation's
// registrants; no HTTP server, rate limiter or jobs client. The command's
// line goes to out, the logs to logOut. An error is one line for the
// administrator.
func Users(ctx context.Context, cfg config.Config, logOut, out io.Writer, cmd UserCommand) error {
	c, err := openAdminCommand(ctx, cfg, logOut)
	if err != nil {
		return err
	}
	defer c.close()
	vetoers, subscribers := deactivationRegistrants(c.pool)
	admin := identity.NewAdmin(identity.AdminDeps{
		Pool:                    c.pool,
		Tx:                      postgres.NewTxManager(c.pool, cfg.Database.CommitTimeout),
		Clock:                   clock.System{},
		Logger:                  c.logger,
		Password:                passwordHashing(cfg.Auth.Password),
		DeactivationVetoers:     vetoers,
		DeactivationSubscribers: subscribers,
	})
	line, err := cmd(ctx, admin)
	return printResult(out, line, err)
}

// CreateUser is `nervewiki users create`.
func CreateUser(email, password string) UserCommand {
	return func(ctx context.Context, admin *identity.Admin) (string, error) {
		u, err := admin.CreateUser(ctx, email, password)
		return fmt.Sprintf("created %s (%s)", u.Email, u.ID), err
	}
}

// ResetPassword is `nervewiki users reset-password`.
func ResetPassword(email, password string) UserCommand {
	return func(ctx context.Context, admin *identity.Admin) (string, error) {
		r, err := admin.ResetPassword(ctx, email, password)
		return fmt.Sprintf("password reset for %s: revoked %s, %s", r.Email, counted(r.Sessions, "session"), counted(r.APITokens, "API token")), err
	}
}

// SetEmail is `nervewiki users set-email`.
func SetEmail(email, newEmail string) UserCommand {
	return func(ctx context.Context, admin *identity.Admin) (string, error) {
		r, err := admin.SetEmail(ctx, email, newEmail)
		return fmt.Sprintf("e-mail changed to %s: revoked %s", r.Email, counted(r.Sessions, "session")), err
	}
}

// DeactivateUser is `nervewiki users deactivate`.
func DeactivateUser(email string) UserCommand {
	return func(ctx context.Context, admin *identity.Admin) (string, error) {
		r, err := admin.Deactivate(ctx, email)
		if r.Already {
			return r.Email + " is already deactivated", err
		}
		return fmt.Sprintf("deactivated %s: revoked %s", r.Email, counted(r.Sessions, "session")), err
	}
}

// ActivateUser is `nervewiki users activate`.
func ActivateUser(email string) UserCommand {
	return func(ctx context.Context, admin *identity.Admin) (string, error) {
		r, err := admin.Activate(ctx, email)
		if r.Already {
			return r.Email + " is already active", err
		}
		verb := "are"
		if r.APITokens == 1 {
			verb = "is"
		}
		return fmt.Sprintf("activated %s: %s %s usable again", r.Email, counted(r.APITokens, "API token"), verb), err
	}
}

// counted is n and noun, in the plural unless n is 1.
func counted(n int, noun string) string {
	if n != 1 {
		noun += "s"
	}
	return strconv.Itoa(n) + " " + noun
}
