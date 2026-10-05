package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The link index's interleavings (M6/P3 design 5; M6 design 4.5).

// interleaveOnIndex is interleaveWith on the notebook nb's lock of the
// index, the advisory lock linking's store takes: its key space "link",
// and the FNV-1a of the notebook's id. Each step waits for it in its
// unit's observer.
func (tm acmeTeam) interleaveOnIndex(t *testing.T, nb string, first, second step) (answer, answer) {
	t.Helper()
	h := fnv.New32a()
	id := uuid.MustParse(nb)
	_, _ = h.Write(id[:])
	space, key := int32(0x6c696e6b), int32(h.Sum32())
	return tm.interleaveWith(t, fmt.Sprintf("SELECT pg_advisory_xact_lock(%d, %d)", space, key), func(i int) {
		pgtest.WaitForAdvisoryLockWaits(t, tm.pool, space, key, i+1, interleavingWait)
	}, first, second)
}

// Two content writes of a notebook, which hold its row FOR SHARE and may
// run at once, keep its index one after the other at its lock of the
// index: one page gains the alias x while another writes [[x]], and
// whichever comes first, the link resolves to the alias's page, the second
// writer reading what the first committed. Without the lock, nothing would
// order them, and each could miss the other's rows.
func TestTwoContentWritesKeepTheIndexOneAfterTheOther(t *testing.T) {
	for _, aliasFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("the alias first: %v", aliasFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			named := tm.createPage(t, "alice", nb, "", "Named")
			src := tm.createPage(t, "alice", nb, "", "Src")
			first, second := contentWrite("alice", named, "---\naliases: [x]\n---\n", 1, ""), contentWrite("alice", src, "[[x]]", 1, "")
			if !aliasFirst {
				first, second = second, first
			}
			a, b := tm.interleaveOnIndex(t, nb, first, second)
			if a.status != http.StatusOK || b.status != http.StatusOK {
				t.Fatalf("the writes = %d %s, %d %s; want both 200", a.status, a.body, b.status, b.body)
			}
			tm.resolves(t, src, named)
			checkPages(t, tm.pool)
		})
	}
}

// A content written with a link to B and B's deletion keep the index in
// either order, one waiting for the other at the notebook's row: the link
// resolves to none, and no link points to a page deleted.
func TestAContentAndItsTargetsDeletionKeepTheIndexInEitherOrder(t *testing.T) {
	for _, writeFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("the write first: %v", writeFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			b := tm.createPage(t, "alice", nb, "", "B")
			src := tm.createPage(t, "alice", nb, "", "Src")
			first, second := contentWrite("alice", src, "[[B]]", 1, ""), nodeDeletion("alice", b)
			if !writeFirst {
				first, second = second, first
			}
			x, y := tm.interleaveOn(t, notebookRow(nb), first, second)
			write, deletion := x, y
			if !writeFirst {
				write, deletion = y, x
			}
			if write.status != http.StatusOK || deletion.status != http.StatusNoContent {
				t.Fatalf("the write = %d %s, the deletion = %d %s; want 200 and 204", write.status, write.body, deletion.status, deletion.body)
			}
			tm.resolves(t, src, "")
			checkPages(t, tm.pool)
		})
	}
}

// A notebook deleted while reindex goes through the notebooks, between its
// listing and its rebuild, which waits for its row, is skipped: the others
// are rebuilt, and the command succeeds.
func TestReindexSkipsANotebookDeletedMeanwhile(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	first := tm.openNotebook(t, "alice", "Eng")
	second := tm.openNotebook(t, "alice", "Ops")
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", second); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, tm.url, false)
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- Reindex(ctx, cfg, &stderr, &stdout, uuid.Nil()) }()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	if _, err := holder.Exec(ctx, "UPDATE notebooks SET deleted_at = now() WHERE id = $1", second); err != nil {
		t.Fatal(err)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil || stdout.String() != fmt.Sprintf("notebook %s: 0 pages, 0 links, 0 unresolved\n", first) {
		t.Errorf("reindex = %q, %v: %s; want the first notebook's line alone", stdout.String(), err, stderr.String())
	}
}

// A reindex of a notebook behind a content write of it in flight, which
// holds the notebook's row FOR SHARE and waits at its content's row, waits
// at the notebook's row before it takes the index's lock: the write then
// takes that lock, and both end. Taken the other way round, the reindex
// would hold the index's lock and wait for the row, the write would wait
// for the index's lock, and the two would deadlock.
func TestAReindexWaitsAtTheNotebooksRowBeforeTheIndexsLock(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	b := tm.createPage(t, "alice", nb, "", "B")
	src := tm.createPage(t, "alice", nb, "", "Src")
	cfg := testConfig(t, tm.url, false)
	var stdout, stderr bytes.Buffer
	reindex := step{command: func() error { return Reindex(context.Background(), cfg, &stderr, &stdout, uuid.MustParse(nb)) }}
	row := held{"page_contents", fmt.Sprintf("SELECT 1 FROM page_contents WHERE node_id = '%s' FOR NO KEY UPDATE", src)}
	write, re := tm.interleaveBehind(t, row, contentWrite("alice", src, "[[B]]", 1, ""), reindex, "notebooks")
	if write.status != http.StatusOK || re.err != nil {
		t.Fatalf("the write = %d %s, the reindex = %v %s; want both to end", write.status, write.body, re.err, stderr.String())
	}
	tm.resolves(t, src, b)
	checkPages(t, tm.pool)
}
