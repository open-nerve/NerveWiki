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
// A file system without migrations is valid; every operation is then a no-op.
type Migrator struct {
	provider *goose.Provider // nil when there are no migrations
}

// NewMigrator prepares the migrations in fsys for the database behind pool.
// Close releases the connection it borrows from the pool.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true), // no package-level Go migrations
	)
	if err != nil {
		_ = db.Close() // nothing has been borrowed from the pool yet
		if errors.Is(err, goose.ErrNoMigrations) {
			return &Migrator{}, nil
		}
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return &Migrator{provider: provider}, nil
}

// Up applies every pending migration in version order and returns them. When
// a migration fails, the ones applied before it stay applied: Up returns them
// together with the error.
func (m *Migrator) Up(ctx context.Context) ([]Migration, error) {
	if m.provider == nil {
		return nil, nil
	}
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
	if m.provider == nil {
		return nil, nil
	}
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
	if m.provider == nil {
		return nil, nil
	}
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
	if m.provider == nil {
		return nil
	}
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
	if m.provider == nil {
		return nil
	}
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
