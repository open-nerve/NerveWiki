package bootstrap

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// U09: the actual connection selected by pgx is independent of the ctx's Tx.
// Only the designated real Add carries the marker. Fixture writes are untouched.
type ingestMarksTxProbe struct {
	t      *testing.T
	active atomic.Bool
	mu     sync.Mutex
	rows   []ingestMarksTxObservation
}
type ingestMarksTxObservation struct {
	OwnerPID, ActualPID                              uint32
	ParentHealthy, CleanupIssued, ParentStillHealthy bool
}
type ingestMarksTxKey struct{}
type ingestMarksTxExpected struct {
	probe *ingestMarksTxProbe
	owner uint32
}
type ingestMarksTxWitness struct {
	linking.IngestMarks
	probe *ingestMarksTxProbe
}

func (w ingestMarksTxWitness) Add(ctx context.Context, nb, cs uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	if !w.probe.active.Load() {
		return w.IngestMarks.Add(ctx, nb, cs, ids)
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return linking.IngestMarkResult{}, fmt.Errorf("U09 observation setup: no Writer Tx")
	}
	ctx = context.WithValue(ctx, ingestMarksTxKey{}, ingestMarksTxExpected{w.probe, tx.Conn().PgConn().PID()})
	return w.IngestMarks.Add(ctx, nb, cs, ids)
}
func (p *ingestMarksTxProbe) TraceQueryStart(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	want, ok := ctx.Value(ingestMarksTxKey{}).(ingestMarksTxExpected)
	if !ok || want.probe != p || !strings.HasPrefix(q.SQL, "-- name: AddIngestMarks ") {
		return ctx
	}
	seen := ingestMarksTxObservation{OwnerPID: want.owner, ActualPID: conn.PgConn().PID(), ParentHealthy: ctx.Err() == nil}
	p.mu.Lock()
	at := len(p.rows)
	p.rows = append(p.rows, seen)
	p.mu.Unlock()
	if seen.ParentHealthy && seen.OwnerPID != 0 && seen.ActualPID != 0 && seen.OwnerPID != seen.ActualPID {
		// This is the FIRST assertion, based on two actual connection identities.
		// Errorf is allowed from this server goroutine; Fatal/FailNow is not.
		// Cancellation below is cleanup AFTER positive business failure evidence.
		p.t.Errorf("U09_MARKS_WRONG_TX: actual AddIngestMarks backend=%d, Writer backend=%d; caller context was live", seen.ActualPID, seen.OwnerPID)
		cleanup, cancel := context.WithCancel(ctx)
		cancel() // only this query's child; not the Writer, pool, or another backend
		p.mu.Lock()
		p.rows[at].CleanupIssued, p.rows[at].ParentStillHealthy = true, ctx.Err() == nil
		p.mu.Unlock()
		return cleanup
	}
	return ctx
}
func (*ingestMarksTxProbe) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (p *ingestMarksTxProbe) observations() []ingestMarksTxObservation {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.rows)
}

var _ pgx.QueryTracer = (*ingestMarksTxProbe)(nil)

// U11: preserve the original already-marked reindex case and add an unmarked
// historical reference. A literal v1 source is ineligible during a REAL write;
// no mark/history row is deleted or fabricated to prepare this state.
func testIngestReindexLegacyNoHistory(t *testing.T) {
	f := newIngestFixture(t, nil)
	ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", f.s)
	oldCS := f.write(t, f.p, "[[raw/S]]")
	assertIngestMarks(t, f.tm, oldCS)
	want := []string{f.s}
	refs := func() []string {
		return queryStrings(t, f.tm.pool, "SELECT resolved_id::text FROM page_links WHERE source_id=$1 AND property_key IS NULL ORDER BY range_start", f.p)
	}
	if got := refs(); !slices.Equal(got, want) {
		t.Fatalf("legacy real reference=%v want%v", got, want)
	}
	ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE notebook_id=$1", f.nb)
	if n := count(t, f.tm.pool, "SELECT count(*) FROM indexed_pages WHERE node_id IN ($1,$2) AND extractor=1", f.s, f.p); n != 2 {
		t.Fatalf("literal v1 legacy source/wiki heads=%d want2", n)
	}
	before := ingestReindexHistoryImage(t, f)
	f.tm.reindex(t, f.nb) // original reindexAdmin -> actual Admin.Rebuild transaction
	if got := refs(); !slices.Equal(got, want) {
		t.Fatalf("reindex lost real legacy reference=%v want%v", got, want)
	}
	if n := count(t, f.tm.pool, `SELECT count(*) FROM indexed_pages i JOIN page_contents c ON c.node_id=i.node_id
		WHERE i.node_id IN ($1,$2) AND i.extractor=2 AND i.revision=c.revision`, f.s, f.p); n != 2 {
		t.Fatalf("reindex did not actually repair both heads: %d", n)
	}
	after := ingestReindexHistoryImage(t, f)
	if after != before {
		t.Fatalf("U11_REINDEX_BACKFILLED_LEGACY_HISTORY: real v1 P->S had no mark; history changed across successful rebuild: before=%s after=%s", before, after)
	}
	assertIngestMarks(t, f.tm, oldCS)
	newCS := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
	if newCS == oldCS {
		t.Fatal("independent real edit unexpectedly reused old non-session changeset")
	}
	assertIngestMarks(t, f.tm, newCS, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

// Snapshot persisted history only. Rebuild's index repairs, search queue and
// legitimate links event are deliberately not mistaken for edit history.
func ingestReindexHistoryImage(t *testing.T, f ingestFixture) string {
	t.Helper()
	var out string
	err := f.tm.pool.QueryRow(t.Context(), `SELECT jsonb_build_object(
		'changesets',(SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM changesets c WHERE c.notebook_id=$1),
		'items',(SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.id),'[]'::jsonb) FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id WHERE c.notebook_id=$1),
		'revisions',(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]'::jsonb) FROM page_revisions r JOIN changesets c ON c.id=r.changeset_id WHERE c.notebook_id=$1),
		'marks',(SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.changeset_id,s.seq),'[]'::jsonb) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1)
	)::text`, f.nb).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
