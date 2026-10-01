package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/logging"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What the administrator's commands share (v0.1 design 13.1, item 22):
// nervewiki users and nervewiki workspaces each compose a module's
// administrator use cases on this, statically, for the composition check
// to follow.

// adminCommand is the start of a command's composition: its logger and a
// pool on which the database answered.
type adminCommand struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
}

// openAdminCommand builds the logger on logOut and the pool, and waits for
// the database. close releases the pool.
func openAdminCommand(ctx context.Context, cfg config.Config, logOut io.Writer) (adminCommand, error) {
	logger, err := logging.New(logOut, cfg.Log)
	if err != nil {
		return adminCommand{}, err
	}
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return adminCommand{}, err
	}
	if err := awaitDatabase(ctx, pool, databaseWait); err != nil {
		pool.Close()
		return adminCommand{}, err
	}
	return adminCommand{logger: logger, pool: pool}, nil
}

func (c adminCommand) close() { c.pool.Close() }

// printResult writes the command's line to out, or returns its error as
// one line for the administrator.
func printResult(out io.Writer, line string, err error) error {
	if err != nil {
		return commandError(err)
	}
	return writeLine(out, line)
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
	case "name", "slug":
		return "--" + field
	default:
		return field
	}
}
