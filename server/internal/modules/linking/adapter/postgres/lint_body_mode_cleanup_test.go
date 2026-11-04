package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func TestLintBodyCachedRowsCleanup(t *testing.T) {
	for _, part := range []string{"first", "after"} {
		for _, failure := range []string{"scan", "query", "context-cancel", "database-cancel"} {
			t.Run(part+"/"+failure, func(t *testing.T) {
				f := newFixture(t)
				p, target := uuid.UUID{}, uuid.UUID{15: 99}
				f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
				lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT $1,k,k+1,$2,'wikilink','body','body',$3,(k%2=0),false FROM generate_series(0,513) k`, p, f.eng, target)
				lintExec(t, f, `CREATE TABLE body_mode_rollback (id integer PRIMARY KEY)`)
				if failure == "scan" {
					lintExec(t, f, `ALTER TABLE page_links ALTER COLUMN resolved_asset DROP NOT NULL`)
				}
				want := make([]app.BodyReference, 0, 514)
				for i := range 514 {
					want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: p, Start: int32(i)}, ResolvedID: target, ResolvedAsset: i%2 == 0})
				}
				tr := &lintTrace{}
				pool := lintBodyModePool(t, f, tr, 8, 8)
				conn, err := pool.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Release()
				pid := conn.Conn().PgConn().PID()
				bound := postgres.OnConn(t.Context(), conn)
				adapter := postgresadapter.NewLintReads(pool, 5<<20)
				reads := app.NewLintReads(adapter, 5<<20)
				for _, warm := range []struct {
					name  string
					after *app.EdgePos
					want  []app.BodyReference
				}{{"LintBodyReferencesFirst", nil, want[:512]}, {"LintBodyReferencesAfter", &want[511].EdgePos, want[512:]}} {
					rows, err := adapter.BodyReferences(bound, f.eng, []uuid.UUID{p}, warm.after, 512)
					if err != nil || !slices.Equal(rows, warm.want) {
						t.Fatalf("nonempty Body warmup %s: rows=%d err=%v", warm.name, len(rows), err)
					}
					calls := tr.observations()
					lintAssertMode(t, calls[len(calls)-1], warm.name, pgx.QueryExecModeCacheDescribe, pid, 8, 8)
				}
				stream := reads.Bodies(f.eng, []uuid.UUID{p})
				name, badStart := "LintBodyReferencesFirst", 300
				if part == "after" {
					rows, err := stream.Next(bound)
					if err != nil || !slices.Equal(rows, want[:512]) {
						t.Fatalf("Body stream first prefix: rows=%d err=%v", len(rows), err)
					}
					name, badStart = "LintBodyReferencesAfter", 513
				}
				if failure == "scan" {
					lintExec(t, f, `UPDATE page_links SET resolved_asset=NULL WHERE source_id=$1 AND range_start=$2`, p, badStart)
				}
				if failure == "query" {
					lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN resolved_asset TO hidden_asset`)
				}
				before, prepared := len(tr.observations()), len(tr.prepareObservations())
				var bodyErr error
				var partial []app.BodyReference
				run := func(ctx context.Context) error {
					return lintBodyTxManager(pool).WithinTx(postgres.OnConn(ctx, conn), func(ctx context.Context) error {
						if _, err := postgres.Planned(postgres.DB(ctx, pool)).Exec(ctx, `INSERT INTO body_mode_rollback(id) VALUES(1)`); err != nil {
							return fmt.Errorf("rollback marker: %w", err)
						}
						partial, bodyErr = stream.Next(ctx)
						return bodyErr
					})
				}
				if failure == "context-cancel" || failure == "database-cancel" {
					blocker, e := f.pool.Begin(t.Context())
					if e != nil {
						t.Fatal(e)
					}
					defer func() { _ = blocker.Rollback(context.Background()) }()
					if _, e := blocker.Exec(t.Context(), `LOCK TABLE page_links IN ACCESS EXCLUSIVE MODE`); e != nil {
						t.Fatal(e)
					}
					ctx, cancel := context.WithCancel(t.Context())
					result := make(chan error, 1)
					joined := false
					go func() { result <- run(ctx) }()
					// A wait-or-assertion failure still joins before releasing conn.
					defer func() {
						cancel()
						if !joined {
							<-result
						}
					}()
					pgtest.WaitForTableLockWaits(t, f.pool, "page_links", 1, time.Second)
					if failure == "context-cancel" {
						cancel()
					} else {
						var cancelled bool
						if e := f.pool.QueryRow(t.Context(), `SELECT pg_cancel_backend($1)`, pid).Scan(&cancelled); e != nil || !cancelled {
							t.Fatalf("real backend cancellation pid=%d returned %t %v", pid, cancelled, e)
						}
					}
					err, joined = <-result, true
					if e := blocker.Rollback(t.Context()); e != nil {
						t.Fatal(e)
					}
				} else {
					err = run(t.Context())
				}
				if bodyErr == nil || err == nil || partial != nil {
					t.Fatalf("Body fault returned partial data or success: rows=%v body=%v tx=%v", partial, bodyErr, err)
				}
				calls := tr.observations()
				if len(calls) != before+1 {
					t.Fatalf("Body fault retried or skipped query: %+v", calls[before:])
				}
				lintAssertMode(t, calls[before], name, pgx.QueryExecModeCacheDescribe, pid, 8, 8)
				if calls[before].Err == nil || !errors.Is(bodyErr, calls[before].Err) {
					t.Fatalf("Body lost actual query/Scan error identity: returned=%v observed=%v", bodyErr, calls[before].Err)
				}
				lintAssertPrepares(t, tr.prepareObservations()[prepared:], name, pid, 0)
				switch failure {
				case "scan":
					var scan pgx.ScanArgError
					if !errors.As(bodyErr, &scan) || scan.ColumnIndex != 3 || scan.FieldName != "resolved_asset" || !strings.Contains(scan.Error(), "cannot scan NULL") {
						t.Fatalf("not Body resolved_asset Scan failure: %T %v", bodyErr, bodyErr)
					}
					lintExec(t, f, `UPDATE page_links SET resolved_asset=(range_start%2=0) WHERE source_id=$1`, p)
				case "query", "database-cancel":
					var pgErr *pgconn.PgError
					code := "42703"
					if failure == "database-cancel" {
						code = "57014"
					}
					if !errors.As(bodyErr, &pgErr) || pgErr.Code != code {
						t.Fatalf("not actual Body PG error %s: %T %v", code, bodyErr, bodyErr)
					}
					if failure == "query" {
						lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN hidden_asset TO resolved_asset`)
					}
				case "context-cancel":
					if !errors.Is(bodyErr, context.Canceled) || !errors.Is(err, context.Canceled) {
						t.Fatalf("Body context error identity: body=%v tx=%v", bodyErr, err)
					}
				}
				if failure != "context-cancel" && !errors.Is(err, bodyErr) {
					t.Fatalf("transaction lost Body error after rollback: %v / %v", err, bodyErr)
				}
				// LintBatches preserves the first error and performs no later query.
				if rows, e := stream.Next(t.Context()); rows != nil || !errors.Is(e, bodyErr) || len(tr.observations()) != before+1 {
					t.Fatalf("failed Body stream was not terminal: rows=%v err=%v", rows, e)
				}
				var markers int
				if e := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM body_mode_rollback`).Scan(&markers); e != nil || markers != 0 {
					t.Fatalf("Body error transaction did not roll back: markers=%d err=%v", markers, e)
				}
				conn.Release()
				// This real acquire is also the asynchronous cancelled-connection barrier.
				recoveryCtx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				recovered, e := pool.Acquire(recoveryCtx)
				if e != nil {
					t.Fatal(e)
				}
				defer recovered.Release()
				recoveryPID := recovered.Conn().PgConn().PID()
				recovery := reads.Bodies(f.eng, []uuid.UUID{p})
				start := len(tr.observations())
				var got []app.BodyReference
				for _, count := range []int{512, 2, 0} {
					rows, e := recovery.Next(postgres.OnConn(recoveryCtx, recovered))
					if e != nil || len(rows) != count {
						t.Fatalf("new Body request after %s: rows=%d want=%d err=%v", failure, len(rows), count, e)
					}
					got = append(got, rows...)
				}
				if !slices.Equal(got, want) {
					t.Fatal("new Body request changed independent DTO sequence")
				}
				calls = tr.observations()[start:]
				if len(calls) != 3 {
					t.Fatalf("recovery query count=%d want 3", len(calls))
				}
				for i, call := range calls {
					name := "LintBodyReferencesAfter"
					if i == 0 {
						name = "LintBodyReferencesFirst"
					}
					lintAssertMode(t, call, name, pgx.QueryExecModeCacheDescribe, recoveryPID, 8, 8)
					if call.Err != nil {
						t.Fatal(call.Err)
					}
				}
				lintNoNamedStatements(t, postgres.OnConn(recoveryCtx, recovered), pool)
				recovered.Release()
				if pool.Stat().AcquiredConns() != 0 {
					t.Fatalf("Body retained pool after recovery: %d", pool.Stat().AcquiredConns())
				}
				t.Logf("Body %s %s: original PID=%d error=%T %v; terminal stream, rollback markers=%d; recovery PID=%d rows=%d queries=%d; no wire claim", part, failure, pid, bodyErr, bodyErr, markers, recoveryPID, len(got), len(calls))
			})
		}
	}
}
