package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type ingestBudgetMetadata struct {
	real      linking.IngestMetadata
	armed     *atomic.Bool
	healthy   *atomic.Bool
	remaining *atomic.Int64
	elapsed   *atomic.Int64
}

func (m ingestBudgetMetadata) Now(ctx context.Context, nb, root uuid.UUID, ids []uuid.UUID) ([]linking.IngestHead, error) {
	return m.real.Now(ctx, nb, root, ids)
}
func (m ingestBudgetMetadata) Heads(ctx context.Context, nb, root uuid.UUID, after *uuid.UUID, take int) (linking.IngestHeadBatch, error) {
	b, err := m.real.Heads(ctx, nb, root, after, take)
	if err != nil || after != nil || !m.armed.CompareAndSwap(true, false) {
		return b, err
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return b, errors.New("budget fixture lost original transaction")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return b, errors.New("budget fixture lost request deadline")
	}
	m.remaining.Store(int64(time.Until(deadline)))
	// A successful real query consumes time without cancellation or PG abort.
	// The original 4s request deadline remains in force.
	started := time.Now()
	if _, err = tx.Exec(ctx, "SELECT pg_sleep(2.1)"); err != nil {
		return b, err
	}
	m.elapsed.Store(int64(time.Since(started)))
	m.healthy.Store(ctx.Err() == nil)
	return b, nil
}
func testIngestBudgetBetween(t *testing.T) {
	trace := &ingestQueryTrace{}
	armed, healthy := &atomic.Bool{}, &atomic.Bool{}
	remaining, elapsed := &atomic.Int64{}, &atomic.Int64{}
	w := wiring{queryTracer: trace, page: func(d *page.Deps) {
		found := 0
		for i, obs := range d.Observers {
			if original, ok := obs.(linkIndex); ok {
				found++
				deps := *original.index.Ingest
				deps.Metadata = ingestBudgetMetadata{deps.Metadata, armed, healthy, remaining, elapsed}
				original.index.Ingest = &deps
				d.Observers[i] = original
			}
		}
		if found != 1 {
			t.Fatalf("normal linkIndex count%d", found)
		}
	}}
	f := newIngestFixture(t, &w)
	pages := make([]sqlPage, 1024)
	for i := range pages {
		pages[i] = sqlPage{"", fmt.Sprintf("Budget filler%04d", i), ""}
	}
	seedPagesSQL(t, f.tm.pool, f.nb, pages)
	q := f.tm.createPageWith(t, "alice", f.nb, "", "Tail Q", "[[raw/T]]")
	if n := count(t, f.tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND id<$2", f.nb, q); n < 1024 {
		t.Fatal("blocking Q is not beyond the first raw batch")
	}
	m := f.tm.agent(t, "alice", f.nb)
	m.handshakeAs(t, "ingest-budget")
	op := opWriteOn("P.md", "[[raw/T]]", versionOf(f.p, f.tm.content(t, "alice", f.p).Revision))
	trace.start()
	armed.Store(true)
	result := answerOf[mcpBatched](t, m.batch(t, op))
	if !result.Changed || result.ChangesetID == nil {
		t.Fatalf("budget real batch: %+v", result)
	}
	cs := *result.ChangesetID
	calls, rows := trace.stop()
	if left := time.Duration(remaining.Load()); left <= 2100*time.Millisecond || left > 4*time.Second {
		t.Fatalf("fixture did not use the original 4s MCP request deadline: remaining=%s", left)
	}
	t.Logf("C09 real deadline before successful delay=%s query=%s remaining_after_query~%s", time.Duration(remaining.Load()), time.Duration(elapsed.Load()), time.Duration(remaining.Load()-elapsed.Load()))
	if !healthy.Load() {
		t.Fatal("soft stop followed an unhealthy/canceled transaction")
	}
	if rows["IngestHeadsFirst"] != 512 || calls["IngestHeadsAfter"] != 0 || calls["IngestNovelSources"] != 0 || calls["AddIngestMarks"] != 0 {
		t.Fatalf("C09 stop must leave an incomplete raw prefix and no Novel/Add: calls=%v rows=%v", calls, rows)
	}
	assertIngestMarks(t, f.tm, cs)
	if count(t, f.tm.pool, "SELECT revision FROM indexed_pages WHERE node_id=$1", f.p) != 2 {
		t.Fatal("healthy edit/index did not commit")
	}
	f.tm.resolves(t, q, f.t)
	f.tm.resolves(t, f.p, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}
