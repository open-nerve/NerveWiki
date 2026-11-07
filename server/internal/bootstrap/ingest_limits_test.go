package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"uuid"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

func ingestManyAssets(t *testing.T, f ingestFixture, n int) ([]string, string) {
	t.Helper()
	names := make([]string, n)
	var body strings.Builder
	for i := range n {
		names[i] = fmt.Sprintf("A%05d.txt", i)
		fmt.Fprintf(&body, "[](raw/%s)\n", names[i])
	}
	ingestExec(t, f.tm, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,created_by_id,updated_by_id,created_at,updated_at)
 SELECT ('00000000-0000-0000-0000-'||lpad(to_hex(ord),12,'0'))::uuid,b.id,$2,'asset',n,lower(n),ord,sha256(convert_to(n,'UTF8')),b.created_by_id,b.created_by_id,now(),now()
 FROM notebooks b,unnest($3::text[]) WITH ORDINALITY a(n,ord) WHERE b.id=$1`, f.nb, f.raw, names)
	ingestExec(t, f.tm, `INSERT INTO asset_blobs(id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
 SELECT gen_random_uuid(),id,notebook_id,'application/octet-stream',octet_length(name),source_hash,created_by_id,created_at FROM nodes WHERE notebook_id=$1 AND kind='asset'`, f.nb)
	ingestExec(t, f.tm, `INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE notebook_id=$1 AND kind='asset'`, f.nb)
	ids := queryStrings(t, f.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND kind='asset' ORDER BY id", f.nb)
	if len(ids) != n {
		t.Fatalf("asset count%d want%d", len(ids), n)
	}
	if uuid.MustParse(ids[len(ids)-1]).Compare(uuid.MustParse(f.t)) >= 0 {
		t.Fatal("independent novel T must follow all10000 sorted asset candidates")
	}
	checkPages(t, f.tm.pool)
	return ids, body.String()
}

type ingestCandidateCount struct {
	linkingapp.IngestCandidateReads
	count *atomic.Int64
	calls *atomic.Int64
}

func (r ingestCandidateCount) NovelSources(ctx context.Context, q linkingapp.IngestCandidateQuery) (linkingapp.IngestCandidateEvidence, error) {
	r.calls.Add(1)
	r.count.Store(int64(len(q.Candidates)))
	return r.IngestCandidateReads.NovelSources(ctx, q)
}
func testIngestCompleteCandidates(t *testing.T) {
	candidates, calls := &atomic.Int64{}, &atomic.Int64{}
	f := newIngestFixture(t, &wiring{page: func(d *page.Deps) {
		for i, obs := range d.Observers {
			if original, ok := obs.(linkIndex); ok {
				deps := *original.index.Ingest
				deps.Candidates = ingestCandidateCount{deps.Candidates, candidates, calls}
				original.index.Ingest = &deps
				d.Observers[i] = original
			}
		}
	}})
	ids, body := ingestManyAssets(t, f, 10000)
	// One old unchanged real wiki digests every asset, independently of marks.
	q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", body)
	if n := count(t, f.tm.pool, "SELECT count(DISTINCT resolved_id) FROM page_links WHERE source_id=$1 AND resolved_asset", q); n != 10000 {
		t.Fatalf("old real Q digested%d assets", n)
	}
	tail := f.tm.createPage(t, "alice", f.nb, "", "Tail")
	m := f.tm.agent(t, "alice", f.nb)
	m.handshakeAs(t, "ingest-candidate-boundary")
	calls.Store(0)
	candidates.Store(0)
	result := answerOf[mcpBatched](t, m.batch(t, opWriteOn("P.md", body, versionOf(f.p, 1)), opWriteOn("Tail.md", "[[raw/T]]", versionOf(tail, 1))))
	if !result.Changed || result.ChangesetID == nil || calls.Load() != 1 || candidates.Load() != 10001 {
		t.Fatalf("complete candidate proof: result%+v calls%d candidates%d", result, calls.Load(), candidates.Load())
	}
	assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
	if n := count(t, f.tm.pool, "SELECT count(DISTINCT resolved_id) FROM page_links WHERE source_id=$1 AND resolved_asset", f.p); n != len(ids) {
		t.Fatalf("new actual candidate references%d want%d", n, len(ids))
	}
	f.tm.resolves(t, tail, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}
func testIngestMarksCap(t *testing.T) {
	f := newIngestFixture(t, nil)
	ids, body := ingestManyAssets(t, f, 10000)
	split := strings.LastIndex(body[:len(body)-1], "\n") + 1
	initial := body[:split] // exact first9999
	session := f.tm.openSession(t, "alice", f.p)
	f.tm.send(t, contentWrite("alice", f.p, initial, 1, session), http.StatusOK)
	cs := ingestCS(t, f.tm, f.p, 2)
	if n := count(t, f.tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", cs); n != 9999 {
		t.Fatalf("initial real producer count%d want9999", n)
	}
	// Existing first asset plus two true new IDs reaches Add with only1 slot.
	next := "[](raw/A00000.txt) [](raw/A09999.txt) [[raw/T]]"
	f.tm.send(t, contentWrite("alice", f.p, next, 2, session), http.StatusOK)
	if got := ingestCS(t, f.tm, f.p, 3); got != cs {
		t.Fatal("real session did not reuse changeset")
	}
	var n, distinctIDs, distinctSeq, low, high int
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT count(*),count(DISTINCT node_id),count(DISTINCT seq),min(seq),max(seq) FROM changeset_sources WHERE changeset_id=$1", cs).Scan(&n, &distinctIDs, &distinctSeq, &low, &high); err != nil {
		t.Fatal(err)
	}
	if n != 10000 || distinctIDs != 10000 || distinctSeq != 10000 || low != 1 || high != 10000 || count(t, f.tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != 10000 {
		t.Fatalf("cap/sequence actual rows%d distinctIDs%d distinctSeq%d range%d..%d", n, distinctIDs, distinctSeq, low, high)
	}
	if got := queryStrings(t, f.tm.pool, "SELECT node_id::text FROM changeset_sources WHERE changeset_id=$1 AND seq=10000", cs); len(got) != 1 || got[0] != ids[9999] {
		t.Fatalf("saturated append order%v want%s", got, ids[9999])
	}
	if n := count(t, f.tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1 AND node_id=$2", cs, f.t); n != 0 {
		t.Fatal("saturated changeset exceeded its capacity")
	}
	// Removing current T does not remove the old session marks. A fresh edit
	// can now independently consume T and must not inherit the saturated cap.
	f.tm.send(t, contentWrite("alice", f.p, "[](raw/A00000.txt) [](raw/A09999.txt)", 3, session), http.StatusOK)
	fresh := f.tm.createPage(t, "alice", f.nb, "", "Fresh")
	other := f.write(t, fresh, "[[raw/T]]")
	if other == cs {
		t.Fatal("fresh edit accidentally reused full session")
	}
	assertIngestMarks(t, f.tm, other, f.t)
	if count(t, f.tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != 10000 {
		t.Fatal("saturated original marks changed")
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}
