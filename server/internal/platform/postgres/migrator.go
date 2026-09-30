package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// ErrPendingMigrations is returned by CheckUpToDate when the schema is behind.
var ErrPendingMigrations = errors.New("database has pending migrations")

// Migration identifies one migration file.
type Migration struct {
	Version int64
	Source  string // file name, e.g. "00001_platform_pg_trgm.sql"
}

// MigrationStatus tells whether a migration has been applied.
type MigrationStatus struct {
	Migration
	Applied   bool
	AppliedAt time.Time // zero while pending
}

// Migrator applies the goose SQL migrations at the root of a file system.
type Migrator struct {
	provider *goose.Provider
}

// NewMigrator prepares the migrations in fsys for the database behind pool;
// fsys must hold at least one. Close releases the connection it borrows from
// the pool.
//
// Up, Down and Status hold a session-level advisory lock while they run, so
// processes that migrate the same database at once, such as several
// instances starting with database.auto_migrate, take turns instead of
// failing: a process waits up to 5 minutes for the lock, polling every
// second.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 300))
	if err != nil {
		return nil, fmt.Errorf("migration lock: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true), // no package-level Go migrations
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		_ = db.Close() // nothing has been borrowed from the pool yet
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return &Migrator{provider: provider}, nil
}

// Up applies every pending migration in version order and returns them. When
// a migration fails, the ones applied before it stay applied: Up returns them
// together with the error.
func (m *Migrator) Up(ctx context.Context) ([]Migration, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		var partial *goose.PartialError
		if !errors.As(err, &partial) {
			return nil, fmt.Errorf("migrate up: %w", err)
		}
		return migrationsOf(partial.Applied), fmt.Errorf("migrate up: %w", err)
	}
	return migrationsOf(results), nil
}

// Down rolls back the most recently applied migration and returns it, or nil
// when nothing is applied.
func (m *Migrator) Down(ctx context.Context) (*Migration, error) {
	result, err := m.provider.Down(ctx)
	if errors.Is(err, goose.ErrNoNextVersion) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("migrate down: %w", err)
	}
	rolledBack := migrationOf(result.Source)
	return &rolledBack, nil
}

// Status lists every known migration in version order.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}
	out := make([]MigrationStatus, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, MigrationStatus{
			Migration: migrationOf(s.Source),
			Applied:   s.State == goose.StateApplied,
			AppliedAt: s.AppliedAt,
		})
	}
	return out, nil
}

// CheckUpToDate returns ErrPendingMigrations if any migration is pending. Its
// signature fits a readiness check.
func (m *Migrator) CheckUpToDate(ctx context.Context) error {
	pending, err := m.provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check migrations: %w", err)
	}
	if pending {
		return ErrPendingMigrations
	}
	return nil
}

// Close releases the database handle. Call it before closing the pool.
func (m *Migrator) Close() error {
	return m.provider.Close()
}

func migrationOf(s *goose.Source) Migration {
	return Migration{Version: s.Version, Source: path.Base(s.Path)}
}

func migrationsOf(results []*goose.MigrationResult) []Migration {
	out := make([]Migration, 0, len(results))
	for _, r := range results {
		out = append(out, migrationOf(r.Source))
	}
	return out
}
