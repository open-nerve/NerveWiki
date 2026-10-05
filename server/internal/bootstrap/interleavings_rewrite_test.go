package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The rewrite's interleavings (M6/P4 design 4.2, 8): a rename holds its
// notebook's row FOR NO KEY UPDATE, and its rewrite reads the index the
// units before it left. Each ends with the index its pages'.

// A rename and a content write of a page that links to the renamed page,
// one waiting for the other at the notebook's row: the write first, the
// rename writes the written content again; the rename first, the write on
// the base it read is page.revision_mismatch, and the rename's content
// stays.
func TestARenameAndAContentWriteOfALinkingPageInEitherOrder(t *testing.T) {
	for _, writeFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("the write first: %v", writeFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			a := tm.createPage(t, "alice", nb, "", "A")
			src := tm.createPageWith(t, "alice", nb, "", "Src", "[[A]]\n")
			first, second := contentWrite("bob", src, "[[A]] and more\n", 1, ""), nodeRename("alice", a, "B")
			if !writeFirst {
				first, second = second, first
			}
			x, y := tm.interleaveOn(t, notebookRow(nb), first, second)
			write, rename := x, y
			if !writeFirst {
				write, rename = y, x
			}
			if rename.status != http.StatusOK {
				t.Fatalf("the rename = %d %s, want 200", rename.status, rename.body)
			}
			switch {
			case writeFirst && write.status != http.StatusOK:
				t.Fatalf("the write = %d %s, want 200", write.status, write.body)
			case writeFirst:
				tm.wrote(t, src, "[[B]] and more\n", 3)
			case write.status != http.StatusConflict || write.code != "page.revision_mismatch":
				t.Fatalf("the write = %d %s, want 409 page.revision_mismatch", write.status, write.body)
			default:
				tm.wrote(t, src, "[[B]]\n", 2)
			}
			tm.resolves(t, src, a)
			checkLinks(t, tm.pool)
			checkPages(t, tm.pool)
		})
	}
}

// Two renames of two pages a third links to write its links one after the
// other at the notebook's row, in either order: the second reads the index
// the first's rewrite left.
func TestTwoRenamesWriteALinkingPageOneAfterTheOther(t *testing.T) {
	for _, aFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("A first: %v", aFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			a := tm.createPage(t, "alice", nb, "", "A")
			c := tm.createPage(t, "alice", nb, "", "C")
			src := tm.createPageWith(t, "alice", nb, "", "Src", "[[A]] [[C]]\n")
			first, second := nodeRename("alice", a, "B"), nodeRename("bob", c, "D")
			if !aFirst {
				first, second = second, first
			}
			x, y := tm.interleaveOn(t, notebookRow(nb), first, second)
			if x.status != http.StatusOK || y.status != http.StatusOK {
				t.Fatalf("the renames = %d %s, %d %s; want both 200", x.status, x.body, y.status, y.body)
			}
			tm.wrote(t, src, "[[B]] [[D]]\n", 3)
			tm.resolves(t, src, a, c)
			checkLinks(t, tm.pool)
			checkPages(t, tm.pool)
		})
	}
}

// A rename and an opening of a page that links to the renamed page, one
// waiting for the other at the notebook's row: the opening first, the
// rename is linking.pages_locked by its editor, and nothing is written;
// the rename first, the session opens on the content the rename wrote.
func TestARenameAndAnOpeningOfALinkingPageInEitherOrder(t *testing.T) {
	for _, openFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("the opening first: %v", openFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			a := tm.createPage(t, "alice", nb, "", "A")
			src := tm.createPageWith(t, "alice", nb, "", "Src", "[[A]]\n")
			first, second := sessionOpening("bob", src), nodeRename("alice", a, "B")
			if !openFirst {
				first, second = second, first
			}
			x, y := tm.interleaveOn(t, notebookRow(nb), first, second)
			opening, rename := x, y
			if !openFirst {
				opening, rename = y, x
			}
			if opening.status != http.StatusCreated {
				t.Fatalf("the opening = %d %s, want 201", opening.status, opening.body)
			}
			if openFirst {
				pagesLocked(t, "the rename", rename, map[string]string{src: "bob"})
				tm.wrote(t, src, "[[A]]\n", 1)
			} else {
				if rename.status != http.StatusOK {
					t.Fatalf("the rename = %d %s, want 200", rename.status, rename.body)
				}
				tm.wrote(t, src, "[[B]]\n", 2)
			}
			tm.resolves(t, src, a)
			checkLinks(t, tm.pool)
			checkPages(t, tm.pool)
		})
	}
}

// A session that a heartbeat keeps alive after the rename's precheck found
// it expired (M6 design 4.6, item 4; M6/P4 design 4.1, step 8): a
// heartbeat whose clock was read before the session expired. The whole
// program has one clock, so the test writes the expiry as that heartbeat
// would, in a transaction of its own that holds the linking page's content
// row: the rename's rewrite waits for the row, then the edit lock refuses
// its write, and the rename reads the locks again, 409
// linking.pages_locked by the editor, nothing written.
func TestAHeartbeatAfterTheRenamesPrecheckRefusesTheRename(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	src := tm.createPageWith(t, "alice", nb, "", "Src", "[[A]]\n")
	session := tm.openSession(t, "bob", src)
	ctx := context.Background()
	if _, err := tm.pool.Exec(ctx, `UPDATE edit_sessions SET created_at = now() - interval '1 minute',
		expires_at = now() - interval '1 second' WHERE id = $1`, session); err != nil {
		t.Fatal(err)
	}
	was := tm.snapshot(t, nb)
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM page_contents WHERE node_id = $1 FOR NO KEY UPDATE", src); err != nil {
		t.Fatal(err)
	}
	rename := nodeRename("alice", a, "B")
	send, answered := tm.sender(t, rename), make(chan answer, 1)
	go func() { answered <- send() }()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "page_contents", 1, interleavingWait)
	if _, err := holder.Exec(ctx, "UPDATE edit_sessions SET expires_at = now() + interval '1 minute' WHERE id = $1", session); err != nil {
		t.Fatal(err)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-answered:
		if got.res == nil {
			t.Fatalf("the rename failed: %v", got.err)
		}
		tm.contract.CheckResponse(t, got.req, got.res)
		pagesLocked(t, "the rename", got, map[string]string{src: "bob"})
	case <-time.After(interleavingWait):
		t.Fatal("the rename did not answer")
	}
	if now := tm.snapshot(t, nb); now != was {
		t.Errorf("the refused rename changed the notebook:\n%s\nwas\n%s", now, was)
	}
	checkLinks(t, tm.pool)
}
