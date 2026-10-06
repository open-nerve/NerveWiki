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
// connection is set up by afterConnect.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, errUnusableURL
	}
	pc.MaxConns = cfg.MaxConns
	pc.AfterConnect = afterConnect
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return pool, nil
}

// afterConnect sets a new connection up: it scans timestamptz values in UTC,
// and the server compiles none of its statements to machine code. They read
// tens of thousands of rows at most, and compiling took longer than it
// saved: the link targets of a page of 2,000 links, planned with them, took
// 49 ms, 6 ms without (M6 closeout FA5-M1). A SET, not a parameter at
// startup: a connection pooler refuses those it does not know (FA5-M2).
func afterConnect(ctx context.Context, conn *pgx.Conn) error {
	if err := scanTimestamptzInUTC(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "SET jit = off"); err != nil {
		return fmt.Errorf("turn jit off: %w", err)
	}
	return nil
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
