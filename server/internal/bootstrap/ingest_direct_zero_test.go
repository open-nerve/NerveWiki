package bootstrap

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"uuid"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

func testIngestNewDirectOnly(t *testing.T) {
	f := newIngestFixture(t, nil)
	a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "followed A")
	b := f.tm.upload(t, "alice", f.nb, f.raw, "b.txt", "direct B")
	source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "[A](a.txt)")
	empty := f.tm.createPage(t, "alice", f.nb, f.raw, "Empty")
	f.tm.resolves(t, source, a.ID)
	if count(t, f.tm.pool, "SELECT byte_size FROM page_contents WHERE node_id=$1", empty) != 0 {
		t.Fatal("empty source fixture is not empty")
	}
	cs := f.write(t, f.p, "[[raw/Follow]] [B](raw/b.txt) [[raw/Empty]] [[raw]]")
	// A belongs to F(S), not the direct new candidates. Empty and source root
	// are deliberately real resolved page IDs, not unresolved link negatives.
	f.tm.resolves(t, f.p, source, b.ID, empty, f.raw)
	assertIngestMarks(t, f.tm, cs, source, b.ID)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

type ingestZeroRawTrace struct {
	mu    sync.Mutex
	armed bool
	calls []ingestZeroRawCall
}
type ingestZeroRawCall struct {
	after *uuid.UUID
	ids   []uuid.UUID
	last  *uuid.UUID
	done  bool
	take  int
}
type ingestZeroRawMetadata struct {
	linkingapp.IngestMetadata
	trace *ingestZeroRawTrace
}

func (r ingestZeroRawMetadata) Heads(ctx context.Context, nb, root uuid.UUID, after *uuid.UUID, take int) (linkingapp.IngestHeadBatch, error) {
	rows, err := r.IngestMetadata.Heads(ctx, nb, root, after, take)
	if err != nil {
		return rows, err
	}
	r.trace.mu.Lock()
	defer r.trace.mu.Unlock()
	if r.trace.armed {
		call := ingestZeroRawCall{done: rows.Done, take: take}
		if after != nil {
			id := *after
			call.after = &id
		}
		if rows.Last != nil {
			id := *rows.Last
			call.last = &id
		}
		for _, row := range rows.Rows {
			call.ids = append(call.ids, row.ID)
		}
		r.trace.calls = append(r.trace.calls, call)
	}
	return rows, nil
}
func testIngestZeroRawEOF(t *testing.T) {
	trace := &ingestZeroRawTrace{}
	f := newIngestFixture(t, &wiring{page: func(d *page.Deps) {
		found := 0
		for i, o := range d.Observers {
			if original, ok := o.(linkIndex); ok {
				deps := *original.index.Ingest
				deps.Metadata = ingestZeroRawMetadata{deps.Metadata, trace}
				original.index.Ingest = &deps
				d.Observers[i] = original
				found++
			}
		}
		if found != 1 {
			t.Fatalf("zero coverage production observer%d", found)
		}
	}})
	// SQL seeds the same legal asset/content/queue shape as existing fixtures.
	// It supplies an ordinary literal zero raw ID, which HTTP-generated IDs do
	// not produce, and leaves every page's current index proof intact.
	ingestExec(t, f.tm, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,created_by_id,updated_by_id,created_at,updated_at)
 SELECT $1,b.id,$3,'asset','zero.bin','zero.bin',1,sha256('zero'::bytea),b.created_by_id,b.created_by_id,now(),now() FROM notebooks b WHERE b.id=$2`, uuid.Nil(), f.nb, f.raw)
	ingestExec(t, f.tm, `INSERT INTO asset_blobs(id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
 SELECT gen_random_uuid(),id,notebook_id,'application/octet-stream',4,source_hash,created_by_id,now() FROM nodes WHERE id=$1`, uuid.Nil())
	ingestExec(t, f.tm, `INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE id=$1`, uuid.Nil())
	missing := 512 - count(t, f.tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1", f.nb)
	if missing <= 0 {
		t.Fatal("zero coverage fixture is not below512")
	}
	pages := make([]sqlPage, missing)
	for i := range pages {
		pages[i] = sqlPage{f.raw, fmt.Sprintf("Empty %03d", i), ""}
	}
	seedPagesSQL(t, f.tm.pool, f.nb, pages)
	wanted := queryStrings(t, f.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 ORDER BY id", f.nb)
	if len(wanted) != 512 {
		t.Fatalf("hand raw count%d want512", len(wanted))
	}
	if wanted[0] != uuid.Nil().String() {
		t.Fatalf("hand raw first%s wantzero", wanted[0])
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	trace.mu.Lock()
	trace.armed = true
	trace.mu.Unlock()
	cs := f.write(t, f.p, "[[raw/T]]")
	trace.mu.Lock()
	calls := slices.Clone(trace.calls)
	trace.armed = false
	trace.mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("real app coverage calls%d want full512 then emptyEOF", len(calls))
	}
	first, last := calls[0], calls[1]
	var got []string
	for _, id := range first.ids {
		got = append(got, id.String())
	}
	if first.after != nil || first.take != 512 || first.done || !slices.Equal(got, wanted) || first.last == nil || first.last.String() != wanted[511] {
		t.Fatalf("actual app first raw512 lost zero/full independent IDs or prematureEOF: %+v got%v want%v", first, got, wanted)
	}
	if last.after == nil || last.after.String() != wanted[511] || last.take != 512 || len(last.ids) != 0 || last.last != nil || !last.done {
		t.Fatalf("actual app full512 did not require exact emptyEOF: %+v", last)
	}
	assertIngestMarks(t, f.tm, cs, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}
