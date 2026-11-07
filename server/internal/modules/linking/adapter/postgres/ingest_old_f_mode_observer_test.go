package postgresadapter_test

import (
	"context"
	"hash/fnv"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const (
	ingestOldBodyFirst = "IngestOldBodyReferencesFirst"
	ingestOldBodyAfter = "IngestOldBodyReferencesAfter"
)

// Reuse the existing observation storage and End callbacks, with a separate
// exact-name cohort. The original Lint starts and 70/7 assertions do not change.
// These are Query/Prepare callbacks, not wire or physical-work measurements.
type ingestOldBodyTrace struct{ lintTrace }

func ingestOldBodySQLName(sql string) string {
	for _, name := range []string{ingestOldBodyFirst, ingestOldBodyAfter} {
		if strings.HasPrefix(sql, "-- name: "+name+" ") {
			return name
		}
	}
	return ""
}

func (tr *ingestOldBodyTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	name := ingestOldBodySQLName(data.SQL)
	if name == "" {
		return ctx
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls = append(tr.calls, name)
	i := len(tr.observed)
	tr.observed = append(tr.observed, lintObservedMode(conn, data))
	return context.WithValue(ctx, lintModeQueryKey{}, i)
}

func (tr *ingestOldBodyTrace) TracePrepareStart(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	name := ingestOldBodySQLName(data.SQL)
	if name == "" {
		return ctx
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	i := len(tr.prepared)
	tr.prepared = append(tr.prepared, lintPrepareObservation{Query: name, Name: data.Name, PID: conn.PgConn().PID()})
	return context.WithValue(ctx, lintModePrepareKey{}, i)
}

func ingestOldBodyNoNamed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var count int
	err := postgres.Planned(postgres.DB(ctx, pool)).QueryRow(ctx,
		`SELECT count(*) FROM pg_prepared_statements
         WHERE statement LIKE '-- name: IngestOldBodyReferencesFirst %'
            OR statement LIKE '-- name: IngestOldBodyReferencesAfter %'`).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("named private old-F statements: count=%d err=%v", count, err)
	}
}

// Adapter fixtures have real current linking heads, not page-owned Before DTOs.
// Source-role/Before authorization is tested through the production app path.
// Keep the actual index lock and validate complete heads before using this port.
func ingestOldBodyFixtureHeads(t *testing.T, ctx context.Context, f fixture, r *postgresadapter.IngestReads, emitters []uuid.UUID) {
	t.Helper()
	// Store.Lock uses the configured default mode. These fixtures deliberately
	// disable that statement cache, so take its exact index-lock key through
	// Planned without changing any default or the tested Body query options.
	h := fnv.New32a()
	_, _ = h.Write(f.eng[:])
	if _, err := postgres.Planned(postgres.DB(ctx, f.pool)).Exec(ctx,
		"SELECT pg_advisory_xact_lock($1::integer,$2::integer)", int32(0x6c696e6b), int32(h.Sum32())); err != nil {
		t.Fatal(err)
	}
	ids := slices.Clone(emitters)
	slices.SortFunc(ids, uuid.UUID.Compare)
	ids = slices.Compact(ids)
	for len(ids) > 0 {
		n := min(len(ids), 512)
		revisions := make([]app.IngestRevision, n)
		for i, id := range ids[:n] {
			revisions[i] = app.IngestRevision{ID: id, Revision: 1}
		}
		heads, err := r.IndexedHeads(ctx, f.eng, revisions)
		if err != nil || len(heads) != n {
			t.Fatalf("fixture current heads: %+v %v", heads, err)
		}
		for i, h := range heads {
			if h.ID != ids[i] || !h.Present || h.PageRevision != 1 || h.IndexedRevision != 1 || h.Extractor != 2 {
				t.Fatalf("fixture emitter is not current: %+v", h)
			}
		}
		ids = ids[n:]
	}
}

var _ pgx.QueryTracer = (*ingestOldBodyTrace)(nil)
var _ pgx.PrepareTracer = (*ingestOldBodyTrace)(nil)
