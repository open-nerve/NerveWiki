package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Deliberate test-only schema faults exercise hot-description error cleanup.
// This does not extend the private capture proof to arbitrary concurrent DDL,
// or replace the separate request cancellation and production rollback tests.
func TestIngestOldBodyCachedRowsCleanup(t *testing.T) {
	for _, part := range []string{"first", "after"} {
		for _, failure := range []string{"scan", "query", "search-path"} {
			t.Run(part+"/"+failure, func(t *testing.T) {
				f, p, want := lintBodyModeSeed(t)
				lintExec(t, f, `CREATE TABLE old_f_mode_rollback(id integer PRIMARY KEY)`)
				if failure == "scan" {
					lintExec(t, f, `ALTER TABLE page_links ALTER COLUMN resolved_asset DROP NOT NULL`)
				}
				if failure == "search-path" {
					// Only this session's failing transaction sees this alternate
					// relation. The selected output keeps a deliberate wrong type.
					lintExec(t, f, `CREATE SCHEMA old_f_shadow`)
					lintExec(t, f, `CREATE TABLE old_f_shadow.page_links(
                        source_id uuid, range_start integer, notebook_id uuid,
                        property_key text, resolved_id uuid, resolved_asset text)`)
					lintExec(t, f, `INSERT INTO old_f_shadow.page_links
                        SELECT source_id,range_start,notebook_id,property_key,resolved_id,'not-a-boolean'
                        FROM page_links`)
				}
				tr := &ingestOldBodyTrace{}
				pool := lintBodyModePool(t, f, tr, 0, 8)
				conn, err := pool.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Release()
				pid := conn.Conn().PgConn().PID()
				bound := postgres.OnConn(t.Context(), conn)
				r := postgresadapter.NewIngestReads(pool, 5<<20)
				run := func(fn func(context.Context) error) error { return lintBodyTxManager(pool).WithinTx(bound, fn) }
				err = run(func(ctx context.Context) error {
					ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
					for _, cursor := range []*app.EdgePos{nil, &want[0].EdgePos} {
						expected := want
						name := ingestOldBodyFirst
						if cursor != nil {
							expected, name = want[1:], ingestOldBodyAfter
						}
						rows, err := r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, cursor, 512)
						if err != nil || !slices.Equal(rows, expected) {
							t.Fatalf("nonempty warmup: %+v %v", rows, err)
						}
						calls := tr.observations()
						lintAssertMode(t, calls[len(calls)-1], name, pgx.QueryExecModeCacheDescribe, pid, 0, 8)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if failure == "scan" {
					lintExec(t, f, `UPDATE page_links SET resolved_asset=NULL WHERE source_id=$1 AND range_start=30`, p)
				}
				if failure == "query" {
					lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN resolved_asset TO hidden_asset`)
				}
				name := ingestOldBodyFirst
				var cursor *app.EdgePos
				if part == "after" {
					name, cursor = ingestOldBodyAfter, &want[0].EdgePos
				}
				before, prepared := len(tr.observations()), len(tr.prepareObservations())
				var bodyErr error
				var partial []app.BodyReference
				err = run(func(ctx context.Context) error {
					ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
					db := postgres.Planned(postgres.DB(ctx, pool))
					if _, err := db.Exec(ctx, `INSERT INTO old_f_mode_rollback(id) VALUES(1)`); err != nil {
						return err
					}
					if failure == "search-path" {
						if _, err := db.Exec(ctx, `SET LOCAL search_path TO old_f_shadow,public`); err != nil {
							return err
						}
					}
					partial, bodyErr = r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, cursor, 512)
					return bodyErr
				})
				if bodyErr == nil || err == nil || partial != nil || !errors.Is(err, bodyErr) {
					t.Fatalf("hot old-F lost error/returned partial rows: %+v body=%v tx=%v", partial, bodyErr, err)
				}
				calls := tr.observations()
				if len(calls) != before+1 {
					t.Fatalf("hot old-F retried/skipped: %+v", calls[before:])
				}
				lintAssertMode(t, calls[before], name, pgx.QueryExecModeCacheDescribe, pid, 0, 8)
				if calls[before].Err == nil || !errors.Is(bodyErr, calls[before].Err) {
					t.Fatalf("lost real query/Scan error: %v / %v", bodyErr, calls[before].Err)
				}
				lintAssertPrepares(t, tr.prepareObservations()[prepared:], name, pid, 0)
				if failure == "query" {
					var pgErr *pgconn.PgError
					if !errors.As(bodyErr, &pgErr) || pgErr.Code != "42703" {
						t.Fatalf("not real missing-column error: %T %v", bodyErr, bodyErr)
					}
					lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN hidden_asset TO resolved_asset`)
				} else {
					var scan pgx.ScanArgError
					// Actual generated scan order: UUID,int32,*UUID,bool.
					if !errors.As(bodyErr, &scan) || scan.ColumnIndex != 3 || scan.FieldName != "resolved_asset" {
						t.Fatalf("not real resolved_asset Scan error: %T %v", bodyErr, bodyErr)
					}
					if failure == "scan" {
						lintExec(t, f, `UPDATE page_links SET resolved_asset=(range_start=20) WHERE source_id=$1`, p)
						lintExec(t, f, `ALTER TABLE page_links ALTER COLUMN resolved_asset SET NOT NULL`)
					}
				}
				var markers int
				if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM old_f_mode_rollback`).Scan(&markers); err != nil || markers != 0 {
					t.Fatalf("failed old-F transaction did not roll back: markers=%d err=%v", markers, err)
				}
				// New call, same actual connection; error recovery must not rely
				// on an internal fallback or a replaced executor. The failed Tx's
				// SET LOCAL is gone, and a complete normal result includes EOF.
				err = run(func(ctx context.Context) error {
					ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
					ingestOldBodyConsume(t, ctx, f, r, tr, []uuid.UUID{p}, want, pgx.QueryExecModeCacheDescribe, 0, 8)
					ingestOldBodyNoNamed(t, ctx, pool)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				conn.Release()
				if pool.Stat().AcquiredConns() != 0 {
					t.Fatalf("old-F retained connection after recovery: %d", pool.Stat().AcquiredConns())
				}
			})
		}
	}
}
