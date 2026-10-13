package bootstrap

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The interleavings of the trash (M8/P2 design 3.10), harnessed as the
// pages' (interleavings_page_test.go): a restore decides unlocked, then
// locks its notebook's row FOR NO KEY UPDATE as every tree's write does,
// and reads its item again there. Each ends by checking the pages', the
// attachments' and the notebooks' invariants.

// holdNotebook begins a transaction of the test's own that holds the row
// of the notebook nb as a tree's write does; the test ends it.
func (tm acmeTeam) holdNotebook(t *testing.T, nb string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, notebookRow(nb).lock); err != nil {
		t.Fatal(err)
	}
	return holder
}

// waitingFor sends c, waits until it waits for a row of table, which the
// test holds, runs then, and returns c's answer, checked against the
// contract.
func (tm acmeTeam) waitingFor(t *testing.T, table string, c step, then func()) answer {
	t.Helper()
	send := tm.sender(t, c)
	done := make(chan answer, 1)
	go func() { done <- send() }()
	pgtest.WaitForLockWaitsOn(t, tm.pool, table, 1, interleavingWait)
	then()
	select {
	case got := <-done:
		if got.res == nil {
			t.Fatalf("%s failed: %v", c.name(), got.err)
		}
		tm.contract.CheckResponse(t, got.req, got.res)
		return got
	case <-time.After(interleavingWait):
		t.Fatalf("%s did not answer", c.name())
		return answer{}
	}
}

// restoredBy is what the answer a of a restore says it did.
func restoredBy(t *testing.T, a answer) nodeRestored {
	t.Helper()
	var r nodeRestored
	decodeAnswer(t, a.body, &r)
	return r
}

// commit ends the test's hold of a row.
func commit(t *testing.T, holder pgx.Tx) {
	t.Helper()
	if err := holder.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// A restore and the deletion of the parent it goes back under. The
// restore first: D goes back under P, then goes with it at its time, a
// node of P's item. The deletion first: D goes back at the root, its
// parent in the trash.
func TestRestoringAndDeletingItsParent(t *testing.T) {
	orders(t, "restore", "deletion", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPage(t, "alice", nb, "", "P")
		d := tm.createPage(t, "alice", nb, p, "D")
		tm.send(t, nodeDeletion("alice", d), http.StatusNoContent)
		steps := map[string]step{"restore": nodeRestore("bob", d), "deletion": nodeDeletion("alice", p)}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		restore, deletion := got["restore"], got["deletion"]
		if restore.status != http.StatusOK || deletion.status != http.StatusNoContent {
			t.Fatalf("the restore = %d %s, the deletion = %d %s; want 200 and 204", restore.status, restore.body, deletion.status,
				deletion.body)
		}
		r := restoredBy(t, restore)
		inP := count(t, tm.pool, `SELECT count(*) FROM nodes d JOIN nodes p ON p.id = d.parent_id
			WHERE d.id = $1 AND p.id = $2 AND d.deleted_at = p.deleted_at AND NOT d.trash_root AND p.trash_pages = 2`, d, p)
		switch {
		case first == "restore" && (r.Placed.at() != p+" original" || inP != 1):
			t.Errorf("restored %+v, D in P's item %d; want D back under P, then deleted with it", r, inP)
		case first == "deletion" && (r.Placed.at() != " parent_in_trash" ||
			count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id IS NULL AND deleted_at IS NULL", d) != 1):
			t.Errorf("restored %+v, want D back at the root, P in the trash", r)
		}
		tm.checkTrash(t)
	})
}

// A restore and a page created where it goes back, by its name. The
// creation first: the restore numbers its name. The restore first: the
// creation is page.title_taken.
func TestRestoringAndCreatingItsName(t *testing.T) {
	orders(t, "restore", "creation", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPage(t, "alice", nb, "", "P")
		n := tm.createPage(t, "alice", nb, p, "N")
		tm.send(t, nodeDeletion("alice", n), http.StatusNoContent)
		steps := map[string]step{"restore": nodeRestore("bob", n), "creation": childCreation("alice", nb, p, "N")}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		restore, creation := got["restore"], got["creation"]
		r := restoredBy(t, restore)
		switch {
		case first == "creation" && (creation.status != http.StatusCreated || restore.status != http.StatusOK || r.Name != "N 2" || !r.Renamed):
			t.Errorf("the creation = %d, the restore = %d %+v; want 201, then N back as N 2", creation.status, restore.status, r)
		case first == "restore" && (restore.status != http.StatusOK || r.Name != "N" || !creation.is(http.StatusConflict, "page.title_taken")):
			t.Errorf("the restore = %d %+v, the creation = %d %s; want N back, then 409 page.title_taken", restore.status, r,
				creation.status, creation.code)
		}
		tm.checkTrash(t)
	})
}

// Two restores of one item: the second finds it restored under the lock,
// page.trash_changed; one trash_restore changeset.
func TestTwoRestoresOfAnItem(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	r := tm.createPage(t, "alice", nb, "", "R")
	tm.createPage(t, "alice", nb, r, "A")
	tm.send(t, nodeDeletion("alice", r), http.StatusNoContent)
	x, y := tm.interleaveOn(t, notebookRow(nb), nodeRestore("alice", r), nodeRestore("bob", r))
	if x.status != http.StatusOK || !y.is(http.StatusConflict, "page.trash_changed") {
		t.Errorf("the restores = %d, then %d %s; want 200, then 409 page.trash_changed", x.status, y.status, y.code)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE kind = 'trash_restore'"); n != 1 {
		t.Errorf("%d restores' changesets, want 1", n)
	}
	tm.checkTrash(t)
}

// A restore and the purge, on either side of the item's period, a day's
// retention. An item restorable a minute longer: while its restore waits
// for the notebook, the purge at its cutoff takes nothing; the restore
// brings it back. An item past its period, not yet the purge's: its
// restore is page.restore_expired before it waits, and the same purge
// leaves it, an hour clear.
func TestRestoringAndPurgingAtTheEndOfThePeriod(t *testing.T) {
	retention := 24 * time.Hour
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) { c.Jobs.PurgeRetention = retention })
	nb := tm.openNotebook(t, "alice", "Eng")
	within, past := tm.createPage(t, "alice", nb, "", "Within"), tm.createPage(t, "alice", nb, "", "Past")
	tm.createPage(t, "alice", nb, within, "Under")
	for _, id := range []string{within, past} {
		tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	}
	tm.trashedAgo(t, within, retention-domain.TrashMargin-time.Minute)
	tm.trashedAgo(t, past, retention-30*time.Minute)
	holder := tm.holdNotebook(t, nb)
	got := tm.waitingFor(t, "notebooks", nodeRestore("bob", within), func() {
		if err := tm.purgeBefore(t, time.Now().Add(-retention))(); err != nil {
			t.Errorf("the purge: %v", err)
		}
		c := nodeRestore("bob", past)
		if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], ""); status != http.StatusConflict ||
			problemCode(t, answer) != "page.restore_expired" {
			t.Errorf("the restore past the period = %d %s, want 409 page.restore_expired", status, answer)
		}
		commit(t, holder)
	})
	if got.status != http.StatusOK {
		t.Errorf("the restore within the period = %d %s, want 200", got.status, got.body)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NOT NULL"); n != 1 {
		t.Errorf("%d nodes deleted, want Past alone, the purge's later", n)
	}
	tm.checkTrash(t)
}

// A restore and a purge whose clock is ahead, which takes the item's
// items, versions and contents while the restore waits for the notebook:
// under the lock the restore finds them gone, page.trash_changed, and
// restores nothing.
func TestRestoringWhatThePurgeTakes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	r := tm.createPageWith(t, "alice", nb, "", "R", "# R")
	tm.send(t, nodeDeletion("alice", r), http.StatusNoContent)
	holder := tm.holdNotebook(t, nb)
	got := tm.waitingFor(t, "notebooks", nodeRestore("bob", r), func() {
		if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour), "changeset_items", "page_revisions", "page_contents")(); err != nil {
			t.Errorf("the purge: %v", err)
		}
		commit(t, holder)
	})
	if !got.is(http.StatusConflict, "page.trash_changed") {
		t.Errorf("the restore = %d %s, want 409 page.trash_changed", got.status, got.body)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL", r) +
		count(t, tm.pool, "SELECT count(*) FROM changesets WHERE kind = 'trash_restore'"); n != 1 {
		t.Errorf("R deleted and the restores' changesets: %d, want R alone", n)
	}
	tm.checkTrash(t)
}

// A restore and the purge of an attachment's row (M7 handoff, item 2): the
// restore waits for the row another purge's batch holds. The other deletes
// the file and the row: the restore is page.trash_changed, nothing of the
// item back. The other lets the row go: the restore brings it back, and
// the attachment downloads.
func TestRestoringAnAttachmentAPurgeHolds(t *testing.T) {
	for _, tt := range []struct {
		name    string
		release func(t *testing.T, tm acmeTeam, holder pgx.Tx, blob string)
		want    int
	}{
		{"the other deletes it", func(t *testing.T, tm acmeTeam, holder pgx.Tx, blob string) {
			if err := os.Remove(fileOf(t, tm.storage, uuid.MustParse(blob))); err != nil {
				t.Fatal(err)
			}
			if _, err := holder.Exec(context.Background(), "DELETE FROM asset_blobs WHERE id = $1", blob); err != nil {
				t.Fatal(err)
			}
			commit(t, holder)
		}, http.StatusConflict},
		{"the other lets it go", func(t *testing.T, _ acmeTeam, holder pgx.Tx, _ string) {
			if err := holder.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
		}, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			p := tm.createPage(t, "alice", nb, "", "P")
			a := tm.upload(t, "alice", nb, p, "a.txt", "file")
			tm.send(t, nodeDeletion("alice", p), http.StatusNoContent)
			holder := tm.holdRow(t, a.Blob)
			got := tm.waitingFor(t, "asset_blobs", nodeRestore("bob", p), func() { tt.release(t, tm, holder, a.Blob) })
			if got.status != tt.want {
				t.Fatalf("the restore = %d %s, want %d", got.status, got.body, tt.want)
			}
			alive := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL AND id IN ($1, $2)", p, a.ID)
			switch {
			case tt.want == http.StatusConflict && (got.code != "page.trash_changed" || alive != 0):
				t.Errorf("the restore = %s with %d nodes back, want page.trash_changed and none", got.code, alive)
			case tt.want == http.StatusOK:
				status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+a.ID, tm.tokens["alice"], "")
				if status != http.StatusOK || alive != 2 {
					t.Fatalf("GET a.txt = %d %s with %d nodes back, want 200 and both", status, body, alive)
				}
				if status, file := tm.download(t, tm.asset(t, body).ContentURL); status != http.StatusOK || file != "file" {
					t.Errorf("a.txt's address = %d %q, want its file", status, file)
				}
			}
			tm.checkTrash(t)
		})
	}
}

// A restore of an item and of the item of its parent, deleted after it.
// The parent first: D goes back under it. D first: D goes back at the
// root, its parent in the trash, and P where it was.
func TestRestoringAnItemAndItsParentsItem(t *testing.T) {
	orders(t, "D", "P", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := map[string]string{"P": tm.createPage(t, "alice", nb, "", "P")}
		ids["D"] = tm.createPage(t, "alice", nb, ids["P"], "D")
		tm.send(t, nodeDeletion("alice", ids["D"]), http.StatusNoContent)
		tm.send(t, nodeDeletion("alice", ids["P"]), http.StatusNoContent)
		x, y := tm.interleaveOn(t, notebookRow(nb), nodeRestore("alice", ids[first]), nodeRestore("bob", ids[second]))
		got := map[string]answer{first: x, second: y}
		if got["D"].status != http.StatusOK || got["P"].status != http.StatusOK {
			t.Fatalf("the restores of D = %d %s, of P = %d %s; want both 200", got["D"].status, got["D"].body, got["P"].status, got["P"].body)
		}
		want := map[string]string{"P": ids["P"] + " original", "D": " parent_in_trash"}[first]
		if d := restoredBy(t, got["D"]); d.Placed.at() != want {
			t.Errorf("D restored %s, want %s", d.Placed.at(), want)
		}
		tm.checkTrash(t)
	})
}

// A restore and a move that takes its parent deeper. The move first: the
// item, too deep there, goes back at the root, the unit deciding under
// the lock. The restore first: it goes back where it was, and the move,
// deciding on its own depth, is page.too_deep.
func TestRestoringAndMovingItsParentDeeper(t *testing.T) {
	orders(t, "restore", "move", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		var eighth, levelA string
		for i := range domain.MaxDepth - 2 {
			eighth = tm.createPage(t, "alice", nb, eighth, "Level "+string(rune('A'+i)))
			if i == 0 {
				levelA = eighth
			}
		}
		x := tm.createPage(t, "alice", nb, eighth, "X")
		tm.createPage(t, "alice", nb, x, "Y")
		top := tm.createPage(t, "alice", nb, "", "Top")
		tm.send(t, nodeDeletion("alice", x), http.StatusNoContent)
		steps := map[string]step{"restore": nodeRestore("bob", x), "move": nodeMove("alice", levelA, top)}
		a, b := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: a, second: b}
		restore, move := got["restore"], got["move"]
		r := restoredBy(t, restore)
		switch {
		case first == "move" && (move.status != http.StatusOK || restore.status != http.StatusOK || r.Placed.at() != " too_deep"):
			t.Errorf("the move = %d, the restore = %d %+v; want 200, then X back at the root, too deep", move.status, restore.status, r)
		case first == "restore" && (restore.status != http.StatusOK || r.Placed.at() != eighth+" original" ||
			move.code != "page.too_deep"):
			t.Errorf("the restore = %d %+v, the move = %d %s; want X back, then page.too_deep", restore.status, r, move.status, move.code)
		}
		tm.checkTrash(t)
	})
}

// A deletion of a page of an item while the item's restore waits: the
// page is deleted already, page.not_found at once, without waiting; the
// restore brings it back; deleted again, the page is an item of its own,
// with its counts.
func TestDeletingAPageOfAnItemBeingRestored(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	r := tm.createPage(t, "alice", nb, "", "R")
	a := tm.createPage(t, "alice", nb, r, "A")
	tm.send(t, nodeDeletion("alice", r), http.StatusNoContent)
	holder := tm.holdNotebook(t, nb)
	got := tm.waitingFor(t, "notebooks", nodeRestore("bob", r), func() {
		send := tm.sender(t, nodeDeletion("alice", a))
		done := make(chan answer, 1)
		go func() { done <- send() }()
		select {
		case d := <-done:
			if !d.is(http.StatusNotFound, "page.not_found") {
				t.Errorf("the deletion of A = %d %s, want 404 page.not_found", d.status, d.body)
			}
		case <-time.After(interleavingWait):
			t.Error("the deletion of A waited for the notebook")
		}
		commit(t, holder)
	})
	if got.status != http.StatusOK {
		t.Fatalf("the restore = %d %s, want 200", got.status, got.body)
	}
	tm.send(t, nodeDeletion("alice", a), http.StatusNoContent)
	if n := count(t, tm.pool, `SELECT count(*) FROM nodes WHERE id = $1 AND trash_root AND trash_pages = 1 AND trash_assets = 0
		AND trash_height = 1`, a); n != 1 {
		t.Error("A deleted again is no item of its own with its counts")
	}
	tm.checkTrash(t)
}
