// Package pgtest gives integration tests their own PostgreSQL database. Only
// test code may import it (enforced by internal/archtest).
//
// The first call in a test binary starts one PostgreSQL container, shared by
// every test of that package, and migrates a template database with the
// production migrations. Each test then gets a copy made with
// CREATE DATABASE ... TEMPLATE, dropped when the test ends. The container is
// removed by the testcontainers reaper (Ryuk) after the test binary exits.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The image and cluster settings of the development database
// (deploy/compose.dev.yaml, v0.1 design 7.1): every database created with
// the defaults gets the builtin C.UTF-8 locale.
const (
	image      = "postgres:18.6-trixie"
	initdbArgs = "--locale-provider=builtin --locale=C.UTF-8"
)

const templateDB = "nervewiki_template"

// shared is the one container of this test binary, started on first use.
// A package-level singleton is inherent to "one container per test binary";
// pgtest is test-only code.
var shared = sync.OnceValues(startCluster)

type cluster struct {
	admin   *pgxpool.Pool // connected to the maintenance database "postgres"
	base    url.URL       // URL of "postgres"; per-test URLs swap the path
	counter atomic.Int64
}

// NewDatabase returns the URL of a new database with every production
// migration applied. The database is dropped when the test ends. Under
// go test -short the test is skipped instead.
func NewDatabase(t testing.TB) string {
	return newDatabase(t, "TEMPLATE "+templateDB)
}

// NewEmptyDatabase returns the URL of a new database without any migration,
// for tests that bring their own schema, such as the migrator's.
func NewEmptyDatabase(t testing.TB) string {
	return newDatabase(t, "TEMPLATE template0")
}

// NewEmptyDatabaseWith is NewEmptyDatabase with the given CREATE DATABASE
// options, such as ENCODING or a locale, for tests of databases nervewiki
// must refuse.
func NewEmptyDatabaseWith(t testing.TB, options string) string {
	return newDatabase(t, "TEMPLATE template0 "+options)
}

func newDatabase(t testing.TB, options string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: needs Docker")
	}
	c, err := shared()
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	name := fmt.Sprintf("test_%d", c.counter.Add(1))
	create := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize() + " " + options
	if _, err := c.admin.Exec(context.Background(), create); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}
	t.Cleanup(func() {
		drop := "DROP DATABASE " + pgx.Identifier{name}.Sanitize() + " WITH (FORCE)"
		if _, err := c.admin.Exec(context.Background(), drop); err != nil {
			t.Errorf("pgtest: drop database %s: %v", name, err)
		}
	})
	return c.url(name)
}

func (c *cluster) url(database string) string {
	u := c.base
	u.Path = "/" + database
	return u.String()
}

func startCluster() (*cluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("postgres"),
		testcontainers.WithEnv(map[string]string{"POSTGRES_INITDB_ARGS": initdbArgs}),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start %s container (is Docker running?): %w", image, err)
	}
	raw, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("container address: %w", err)
	}
	base, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("container address: %w", err)
	}
	admin, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: raw, MaxConns: 4})
	if err != nil {
		return nil, err
	}
	c := &cluster{admin: admin, base: *base}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+templateDB); err != nil {
		return nil, fmt.Errorf("create template database: %w", err)
	}
	if err := migrate(ctx, c.url(templateDB)); err != nil {
		return nil, fmt.Errorf("migrate template database: %w", err)
	}
	return c, nil
}

// migrate applies the production migrations, then closes every connection:
// CREATE DATABASE ... TEMPLATE needs a template nobody is connected to.
func migrate(ctx context.Context, dbURL string) error {
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: dbURL, MaxConns: 2})
	if err != nil {
		return err
	}
	defer pool.Close()
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		return err
	}
	defer func() { _ = m.Close() }()
	_, err = m.Up(ctx)
	return err
}
