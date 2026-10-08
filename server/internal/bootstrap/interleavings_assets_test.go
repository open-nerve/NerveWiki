package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// The interleavings of the attachments (M7/P2 design 3.12), harnessed as
// the pages' (interleavings_page_test.go): an upload checks unlocked,
// stores its file, then runs its unit, which shares its workspace's row
// and locks its notebook's as a page's creation does; the test holds the
// row the two steps wait for, and they run in the order they came, the
// upload's file stored before it waits. A unit's refusal deletes the file.
// The purge skips a row another transaction holds; a write that follows a
// deletion passes it by. Each ends by checking the pages' invariant and
// the attachments'.

// checkRefused fails t when what an upload refused under its unit left: a
// node of its name, or a file no row holds (checkAssets), which the
// refusal deletes (M7/P2 design 3.4).
func (tm acmeTeam) checkRefused(t *testing.T, name string) {
	t.Helper()
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE name = $1", name); n != 0 {
		t.Errorf("%d nodes named %s, want none", n, name)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// Uploading under a page while its subtree is deleted. The deletion
// first: the parent is gone, 422; the file is deleted. The upload first:
// the attachment goes with the subtree, its row at the deletion's time.
func TestDeletingASubtreeAndUploadingInIt(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, nb, top, child string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		top = tm.createPage(t, "alice", nb, "", "Top")
		return tm, nb, top, tm.createPage(t, "alice", nb, top, "Child")
	}
	t.Run("the deletion first", func(t *testing.T) {
		tm, nb, top, child := setup(t)
		deleted, uploaded := tm.interleaveOn(t, sharedNotebookRow(nb), nodeDeletion("alice", top),
			assetUpload(t, "bob", nb, child, "late.txt", "late"))
		if !deleted.is(http.StatusNoContent, "") || !uploaded.is(http.StatusUnprocessableEntity, "validation_failed") {
			t.Errorf("the deletion = %d, then the upload = %d %s; want 204, then 422 validation_failed", deleted.status, uploaded.status,
				uploaded.code)
		}
		tm.checkRefused(t, "late.txt")
	})
	t.Run("the upload first", func(t *testing.T) {
		tm, nb, top, child := setup(t)
		uploaded, deleted := tm.interleaveOn(t, sharedNotebookRow(nb), assetUpload(t, "bob", nb, child, "late.txt", "late"),
			nodeDeletion("alice", top))
		if !uploaded.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("the upload = %d %s, then the deletion = %d; want 201, then 204", uploaded.status, uploaded.code, deleted.status)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM asset_blobs b JOIN nodes x ON x.id = b.node_id JOIN nodes t ON t.id = $1
			WHERE x.name = 'late.txt' AND x.deleted_at = t.deleted_at AND b.deleted_at = t.deleted_at`, top); n != 1 {
			t.Errorf("%d attachments deleted with Top at its time, their rows too; want late.txt", n)
		}
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
	})
}

// Uploading into a notebook while it is deleted. The deletion first: the
// upload finds the notebook deleted once it has its row, 404; the file is
// deleted. The upload first: the attachment goes with the notebook, its
// row at the notebook's time.
func TestDeletingANotebookAndUploadingInIt(t *testing.T) {
	deletion := func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") }
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		deleted, uploaded := tm.interleaveOn(t, notebookRow(nb), deletion(nb), assetUpload(t, "bob", nb, "", "late.txt", "late"))
		if !deleted.is(http.StatusNoContent, "") || !uploaded.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("DELETE the notebook = %d, then the upload = %d %s; want 204, then 404 notebook.not_found", deleted.status,
				uploaded.status, uploaded.code)
		}
		tm.checkRefused(t, "late.txt")
		checkNotebooks(t, tm.pool)
	})
	t.Run("the upload first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		uploaded, deleted := tm.interleaveOn(t, notebookRow(nb), assetUpload(t, "bob", nb, "", "late.txt", "late"), deletion(nb))
		if !uploaded.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("the upload = %d %s, then DELETE the notebook = %d; want 201, then 204", uploaded.status, uploaded.code, deleted.status)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM asset_blobs b JOIN nodes x ON x.id = b.node_id JOIN notebooks n ON n.id = x.notebook_id
			WHERE x.deleted_at = n.deleted_at AND b.deleted_at = n.deleted_at`); n != 1 {
			t.Errorf("%d attachments deleted with the notebook at its time, their rows too; want late.txt", n)
		}
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
		checkNotebooks(t, tm.pool)
	})
}

// bob uploads while alice removes him from acme. The removal first: the
// upload's unit decides under acme's lock that he sees the notebook no
// more, 404; the file is deleted. The upload first: the attachment stays,
// and he reads it no more.
func TestUploadingAndRemovingTheWriterFromTheWorkspace(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		removed, uploaded := tm.interleave(t, tm.removal("alice", "bob"), assetUpload(t, "bob", nb, "", "late.txt", "late"))
		if !removed.is(http.StatusNoContent, "") || !uploaded.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("remove bob = %d, then his upload = %d %s; want 204, then 404 notebook.not_found", removed.status, uploaded.status,
				uploaded.code)
		}
		tm.checkRefused(t, "late.txt")
	})
	t.Run("the upload first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		uploaded, removed := tm.interleave(t, assetUpload(t, "bob", nb, "", "late.txt", "late"), tm.removal("alice", "bob"))
		if !uploaded.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
			t.Errorf("his upload = %d %s, then remove bob = %d; want 201, then 204", uploaded.status, uploaded.code, removed.status)
		}
		a := tm.asset(t, uploaded.body)
		if n := count(t, tm.pool, "SELECT count(*) FROM asset_blobs WHERE node_id = $1 AND deleted_at IS NULL", a.ID); n != 1 {
			t.Errorf("bob's attachment is gone, want it kept")
		}
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+a.ID, tm.tokens["bob"], ""); status != http.StatusNotFound {
			t.Errorf("bob reads his attachment after his removal = %d %s, want 404", status, answer)
		}
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
	})
}

// bob, who writes only by the workspace's default role, uploads while
// alice makes the workspace's role viewer; both wait for the notebook's
// row. The change first: the upload's unit decides under the row that he
// writes no more, 403 forbidden; the file is deleted. The upload first:
// the attachment stays, and he still reads it.
func TestUploadingAndDemotingTheWriter(t *testing.T) {
	demotion := func(nb string) step {
		return request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb, `{"workspace_access":"viewer"}`)
	}
	t.Run("the change first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		changed, uploaded := tm.interleaveOn(t, notebookRow(nb), demotion(nb), assetUpload(t, "bob", nb, "", "late.txt", "late"))
		if !changed.is(http.StatusOK, "") || !uploaded.is(http.StatusForbidden, "forbidden") {
			t.Errorf("the change = %d %s, then bob's upload = %d %s; want 200, then 403 forbidden", changed.status, changed.code,
				uploaded.status, uploaded.code)
		}
		tm.checkRefused(t, "late.txt")
		checkNotebooks(t, tm.pool)
	})
	t.Run("the upload first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		uploaded, changed := tm.interleaveOn(t, notebookRow(nb), assetUpload(t, "bob", nb, "", "late.txt", "late"), demotion(nb))
		if !uploaded.is(http.StatusCreated, "") || !changed.is(http.StatusOK, "") {
			t.Errorf("bob's upload = %d %s, then the change = %d %s; want 201, then 200", uploaded.status, uploaded.code, changed.status,
				changed.code)
		}
		a := tm.asset(t, uploaded.body)
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+a.ID, tm.tokens["bob"], ""); status != http.StatusOK {
			t.Errorf("bob reads his attachment after the change = %d %s, want 200", status, answer)
		}
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
		checkNotebooks(t, tm.pool)
	})
}

// purge is what runs the purgers in order, as the purge job does, on
// what was deleted before it runs, and returns the first failure. Nothing
// uploads while it is made: opening its store deletes the files being
// written.
func (tm acmeTeam) purge(t *testing.T) func() error {
	t.Helper()
	store, err := storage.OpenLocal(tm.storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	all := purgers(tm.pool, postgres.NewTxManager(tm.pool, interleavingWait), store, slog.New(slog.DiscardHandler))
	return func() error {
		before := time.Now()
		for _, p := range all {
			for {
				n, err := p.Purge(context.Background(), before, 1000)
				if err != nil {
					return fmt.Errorf("%s: %w", p.Table, err)
				}
				if n < 1000 {
					break
				}
			}
		}
		return nil
	}
}

// deletedAsset is an attachment alice uploaded into a notebook of acme
// and deleted, the purge's to delete.
func deletedAsset(t *testing.T) (tm acmeTeam, nb string, a uploadedAsset) {
	t.Helper()
	tm = newAcmeTeam(t, "member", "")
	nb = tm.openNotebook(t, "alice", "Eng")
	a = tm.upload(t, "alice", nb, "", "gone.txt", "gone")
	tm.send(t, nodeDeletion("alice", a.ID), http.StatusNoContent)
	return tm, nb, a
}

// holdRow begins a transaction that holds the row of blob as a purge's
// batch does, FOR UPDATE, and returns it; the test ends it.
func (tm acmeTeam) holdRow(t *testing.T, blob string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	var n int
	if err := holder.QueryRow(ctx, "SELECT count(*) FROM (SELECT 1 FROM asset_blobs WHERE id = $1 FOR UPDATE) held", blob).Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows of %s held: %v; want 1", n, blob, err)
	}
	return holder
}

// A purge meets an attachment's row another purge holds: it skips the row
// and the file, and its deletion of the node waits for the row (the
// node's key restricts it). The other deletes the file and the row: the
// node goes after them. The other lets the row go: the node's deletion
// fails, leaving the node, the row and the file, and a later run deletes
// all three, the file first.
func TestPurgingAnAttachmentAnotherPurgeHolds(t *testing.T) {
	for _, tt := range []struct {
		name    string
		release func(t *testing.T, tm acmeTeam, holder pgx.Tx, blob string)
		fails   bool
	}{
		{"the other deletes it", func(t *testing.T, tm acmeTeam, holder pgx.Tx, blob string) {
			ctx := context.Background()
			if err := os.Remove(fileOf(t, tm.storage, uuid.MustParse(blob))); err != nil {
				t.Fatal(err)
			}
			if _, err := holder.Exec(ctx, "DELETE FROM asset_blobs WHERE id = $1", blob); err != nil {
				t.Fatal(err)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"the other lets it go", func(t *testing.T, _ acmeTeam, holder pgx.Tx, _ string) {
			if err := holder.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm, _, a := deletedAsset(t)
			purge := tm.purge(t)
			holder := tm.holdRow(t, a.Blob)
			done := make(chan error, 1)
			go func() { done <- purge() }()
			pgtest.WaitForLockWaitsOn(t, tm.pool, "asset_blobs", 1, interleavingWait)
			if _, ok := storedBlobs(t, tm.storage)[a.Blob]; !ok {
				t.Errorf("the purge deleted the file of the row another held, want it skipped")
			}
			tt.release(t, tm, holder, a.Blob)
			var err error
			select {
			case err = <-done:
			case <-time.After(interleavingWait):
				t.Fatal("the purge did not end")
			}
			left := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1", a.ID)
			if tt.fails != (err != nil) || left != count(t, tm.pool, "SELECT count(*) FROM asset_blobs WHERE id = $1", a.Blob) {
				t.Errorf("the purge: %v, the node left: %d; want it to fail %t, the node left with its row", err, left, tt.fails)
			}
			checkAssets(t, tm.pool, tm.storage)
			if err := purge(); err != nil {
				t.Errorf("the next purge: %v, want none", err)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes") + count(t, tm.pool, "SELECT count(*) FROM asset_blobs"); n != 0 ||
				len(storedBlobs(t, tm.storage)) != 0 {
				t.Errorf("%d nodes and rows, the files of %v; want none, the attachment purged", n, storedBlobs(t, tm.storage))
			}
			checkPages(t, tm.pool)
		})
	}
}

// A notebook's deletion while a purge holds the row of one of its
// attachments, deleted before: the deletion takes the rows not deleted
// without waiting for the held one, and leaves its time.
func TestDeletingANotebookWhileThePurgeHoldsAnAttachment(t *testing.T) {
	tm, nb, gone := deletedAsset(t)
	kept := tm.upload(t, "alice", nb, "", "kept.txt", "kept")
	was := queryStrings(t, tm.pool, "SELECT deleted_at::text FROM asset_blobs WHERE id = $1", gone.Blob)
	holder := tm.holdRow(t, gone.Blob)
	deletion := tm.sender(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""))
	done := make(chan answer, 1)
	go func() { done <- deletion() }()
	select {
	case got := <-done:
		if !got.is(http.StatusNoContent, "") {
			t.Errorf("DELETE the notebook = %d %s, want 204", got.status, got.body)
		}
	case <-time.After(interleavingWait):
		_ = holder.Rollback(context.Background())
		<-done
		t.Fatal("DELETE the notebook waited for the row the purge holds")
	}
	if err := holder.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := queryStrings(t, tm.pool, "SELECT deleted_at::text FROM asset_blobs WHERE id = $1", gone.Blob); !slices.Equal(got, was) {
		t.Errorf("the held row deleted at %q, want %q as before", got, was)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM asset_blobs b JOIN notebooks n ON n.id = b.notebook_id
		WHERE b.id = $1 AND b.deleted_at = n.deleted_at`, kept.Blob); n != 1 {
		t.Errorf("kept.txt's row not deleted at the notebook's time")
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
	checkNotebooks(t, tm.pool)
}
