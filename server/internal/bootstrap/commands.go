package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/logging"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/webui"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Serve implements `nervewiki serve`: it logs the effective configuration
// (secrets masked) to logOut, applies pending migrations when
// database.auto_migrate is on, checks the database, and serves until ctx is
// done. Once the logger exists, a fatal error is also logged, as a
// structured record.
func Serve(ctx context.Context, cfg config.Config, logOut io.Writer) (err error) {
	logger, err := logging.New(logOut, cfg.Log)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "nervewiki serve failed", slog.Any("error", err))
		}
	}()
	logger.InfoContext(ctx, "configuration loaded", slog.Any("config", cfg))
	a, err := newApp(ctx, cfg, logger, migrations.FS(), webui.FS())
	if err != nil {
		return err
	}
	defer a.close()
	return a.run(ctx)
}

// MigrateUp implements `nervewiki migrate up`: it applies every pending
// migration, prints one "applied <file>" line each, and then checks the
// database.
func MigrateUp(ctx context.Context, cfg config.Config, out io.Writer) error {
	return runMigration(ctx, cfg, migrations.FS(), out, migrateUp)
}

// MigrateDown implements `nervewiki migrate down`: it rolls back the latest
// migration and prints "rolled back <file>".
func MigrateDown(ctx context.Context, cfg config.Config, out io.Writer) error {
	return runMigration(ctx, cfg, migrations.FS(), out, migrateDown)
}

// MigrateStatus implements `nervewiki migrate status`: it prints a table of
// every migration and whether it has been applied.
func MigrateStatus(ctx context.Context, cfg config.Config, out io.Writer) error {
	return runMigration(ctx, cfg, migrations.FS(), out, migrateStatus)
}

type migrationCommand func(context.Context, *pgxpool.Pool, *postgres.Migrator, io.Writer) error

func runMigration(ctx context.Context, cfg config.Config, files fs.FS, out io.Writer, run migrationCommand) (err error) {
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := awaitDatabase(ctx, pool, databaseWait); err != nil {
		return err
	}
	m, err := postgres.NewMigrator(pool, files)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.Close()) }()
	return run(ctx, pool, m, out)
}

func migrateUp(ctx context.Context, pool *pgxpool.Pool, m *postgres.Migrator, out io.Writer) error {
	applied, err := m.Up(ctx)
	// Print what was applied even when a later migration failed.
	for _, mig := range applied {
		if writeErr := writeLine(out, "applied "+mig.Source); writeErr != nil {
			return errors.Join(err, writeErr)
		}
	}
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		if err := writeLine(out, "no pending migrations"); err != nil {
			return err
		}
	}
	return postgres.CheckDatabase(ctx, pool)
}

func migrateDown(ctx context.Context, _ *pgxpool.Pool, m *postgres.Migrator, out io.Writer) error {
	rolledBack, err := m.Down(ctx)
	if err != nil {
		return err
	}
	if rolledBack == nil {
		return writeLine(out, "no applied migrations to roll back")
	}
	return writeLine(out, "rolled back "+rolledBack.Source)
}

func migrateStatus(ctx context.Context, _ *pgxpool.Pool, m *postgres.Migrator, out io.Writer) error {
	statuses, err := m.Status(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	// tabwriter buffers; write errors are reported by Flush.
	_, _ = fmt.Fprintln(tw, "VERSION\tSTATE\tAPPLIED AT\tSOURCE")
	for _, s := range statuses {
		state, appliedAt := "pending", "-"
		if s.Applied {
			state, appliedAt = "applied", s.AppliedAt.UTC().Format(time.RFC3339)
		}
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", s.Version, state, appliedAt, s.Source)
	}
	return tw.Flush()
}

func writeLine(w io.Writer, line string) error {
	_, err := io.WriteString(w, line+"\n")
	return err
}
