// Package postgres connects nervewiki to PostgreSQL, runs its schema
// migrations and checks the database it is given.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// errUnusableURL replaces pgx's parse error, which quotes the connection
// string and masks the password only on a best-effort basis (it misses the
// legal key/value form "password = secret"); even its inner causes can quote
// pieces of the string. pgx also rejects well-formed strings, e.g. when a
// file they name cannot be read, so the message points at every source.
var errUnusableURL = errors.New("database.url: pgx cannot use it; check its syntax, the files it names " +
	"(sslrootcert, sslcert, sslkey) and any PG* environment variables (details not shown, as they may contain the password)")

// NewPool creates a connection pool for database.url with at most
// database.max_conns connections. It connects lazily, on first use. Every
// connection scans timestamptz values in UTC (scanTimestamptzInUTC), and has
// the server plan each statement with its arguments, though pgx caches it:
// from a cached statement's sixth run the server may plan it once for any
// arguments, and such a plan compares each row with an array argument's
// elements one by one where a plan with the array hashes them. A subtree's
// level of 50,000 parents took 3.5 s (M6 closeout FA4-M1).
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, errUnusableURL
	}
	pc.MaxConns = cfg.MaxConns
	pc.AfterConnect = scanTimestamptzInUTC
	pc.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_custom_plan"
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return pool, nil
}

// scanTimestamptzInUTC makes the connection return timestamptz values in UTC.
// pgx returns them in time.Local by default; the API writes every time in UTC
// (v0.1 design 6.1).
func scanTimestamptzInUTC(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
	return nil
}
