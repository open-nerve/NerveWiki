package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/logging"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UserCommand is one `nervewiki users` command on the administrator's use
// cases: it runs and returns the line it prints.
type UserCommand func(ctx context.Context, admin *identity.Admin) (string, error)

// Users runs cmd on the command line's composition (M1/P4 design 3.6): a
// pool and identity's administrator use cases; no HTTP server, rate limiter
// or jobs client. The command's line goes to out, the logs to logOut. An
// error is one line for the administrator.
func Users(ctx context.Context, cfg config.Config, logOut, out io.Writer, cmd UserCommand) error {
	logger, err := logging.New(logOut, cfg.Log)
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := awaitDatabase(ctx, pool, databaseWait); err != nil {
		return err
	}
	vetoers, subscribers := deactivationRegistrants()
	admin := identity.NewAdmin(identity.AdminDeps{
		Pool:                    pool,
		Tx:                      postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Clock:                   clock.System{},
		Logger:                  logger,
		Password:                passwordHashing(cfg.Auth.Password),
		DeactivationVetoers:     vetoers,
		DeactivationSubscribers: subscribers,
	})
	line, err := cmd(ctx, admin)
	if err != nil {
		return commandError(err)
	}
	return writeLine(out, line)
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

// commandError is err as one line for the administrator (M1/P4 design 3.7):
// the invalid fields of a domain error, each as "<name> <problem>" with the
// name the command line knows it by, or else the error's detail.
func commandError(err error) error {
	var se *shared.Error
	if !errors.As(err, &se) || len(se.Fields) == 0 {
		return err
	}
	problems := make([]string, len(se.Fields))
	for i, f := range se.Fields {
		problems[i] = cliFieldName(f.Field) + " " + f.Message
	}
	return errors.New(strings.Join(problems, "; "))
}

// cliFieldName is how the command line names a use case's field.
func cliFieldName(field string) string {
	switch field {
	case "email":
		return "--email"
	case "new_email":
		return "--new-email"
	case "password":
		return "the password"
	default:
		return field
	}
}
