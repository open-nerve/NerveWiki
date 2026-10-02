package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// write writes text as the page id's content at revision, at at.
func (f fixture) write(t *testing.T, id uuid.UUID, text string, revision int, at time.Time) {
	t.Helper()
	sum := sha256.Sum256([]byte(text))
	if err := f.s.WriteContent(context.Background(), app.Content{NodeID: id, Content: text, Revision: revision, Hash: sum[:],
		ByteSize: len(text), By: f.alice, At: at}); err != nil {
		t.Fatal(err)
	}
}

// A content write leaves the content, its version, hash and size, by
// whom and when; the reads see it.
func TestWriteContent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	later := now().Add(time.Minute)
	f.write(t, a.ID, "# A\r\n", 2, later)
	sum := sha256.Sum256([]byte("# A\r\n"))
	got, err := f.s.PageContent(ctx, a.ID)
	if want := (app.PageContent{Content: "# A\r\n", Revision: 2, Hash: sum[:]}); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("PageContent(A) = %+v, %v; want %+v", got, err, want)
	}
	meta, err := f.s.ContentMeta(ctx, a.ID)
	if err != nil || meta != (app.ContentMeta{Revision: 2, ByteSize: 5, UpdatedBy: f.alice, UpdatedAt: later}) {
		t.Errorf("ContentMeta(A) = %+v, %v; want revision 2 of 5 bytes, by alice, a minute later", meta, err)
	}
}

// The gate of a page is its content row, locked FOR NO KEY UPDATE: a
// second lock waits for the first's transaction. Its node's row is not
// locked: a tree's write could take it at once. A page deleted, of
// another notebook, or missing has no gate.
func TestLockContent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	f.write(t, a.ID, "abc", 3, now())
	tx := postgres.NewTxManager(f.pool, time.Second)

	held := make(chan struct{})
	release := make(chan struct{})
	// The holder lets go when the test ends, failed or not.
	let := sync.OnceFunc(func() { close(release) })
	defer let()
	done := make(chan error, 1)
	go func() {
		done <- tx.WithinTx(ctx, func(ctx context.Context) error {
			got, err := f.s.LockContent(ctx, f.eng, a.ID)
			sum := sha256.Sum256([]byte("abc"))
			if want := (app.ContentLock{Revision: 3, Hash: sum[:], ByteSize: 3}); err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("LockContent(A) = %+v, %v; want %+v", got, err, want)
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	if _, err := f.pool.Exec(ctx, "SELECT 1 FROM nodes WHERE id = $1 FOR NO KEY UPDATE NOWAIT", a.ID); err != nil {
		t.Errorf("lock A's node while its content is locked = %v, want the node's row free", err)
	}
	second := make(chan error, 1)
	go func() {
		second <- tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := f.s.LockContent(ctx, f.eng, a.ID)
			return err
		})
	}()
	pgtest.WaitForLockWaitsOn(t, f.pool, "page_contents", 1, 10*time.Second)
	let()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Errorf("the second LockContent = %v, want it once the first let go", err)
	}

	b := f.page(t, f.eng, nil, "B", 1)
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{b.ID}, f.alice, now()); err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string][2]uuid.UUID{
		"of another notebook": {f.ops, a.ID}, "deleted": {f.eng, b.ID}, "missing": {f.eng, uuid.NewV7()},
	} {
		err := tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := f.s.LockContent(ctx, id[0], id[1])
			return err
		})
		if !errors.Is(err, app.ErrNotFound) {
			t.Errorf("LockContent(a page %s) = %v, want ErrNotFound", name, err)
		}
	}
}

// A changeset written again by an edit session moves its updated_at on,
// and keeps its created_at.
func TestTouchChangeset(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	later := now().Add(time.Hour)
	if err := f.s.TouchChangeset(ctx, cs.ID, later); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND created_at = $2 AND updated_at = $3", cs.ID, now(), later); got != 1 {
		t.Error("the changeset's updated_at did not move to its later write, its created_at kept")
	}
}

// A notebook's activity from its pages: the bytes of its pages not
// deleted, and its changesets' latest updated_at, a deleted changeset's
// not counted. A notebook without changesets is not in the answer.
func TestNotebookActivities(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b, gone := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1), f.page(t, f.eng, nil, "Gone", 2)
	f.write(t, a.ID, "12345", 2, now())
	f.write(t, b.ID, "名", 2, now())
	f.write(t, gone.ID, "1234567", 2, now())
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{gone.ID}, f.alice, now()); err != nil {
		t.Fatal(err)
	}
	latest := now().Add(2 * time.Hour)
	for _, c := range []struct {
		at      time.Time
		deleted bool
	}{{now(), false}, {latest, false}, {now().Add(3 * time.Hour), true}} {
		cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientAPI, By: f.alice, At: c.at}
		if err := f.s.CreateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
		if c.deleted {
			f.exec(t, "UPDATE changesets SET deleted_at = $2 WHERE id = $1", cs.ID, c.at)
		}
	}
	got, err := f.s.NotebookActivities(ctx, []uuid.UUID{f.eng, f.ops})
	want := map[uuid.UUID]app.NotebookActivity{f.eng: {Bytes: 5 + 3, LastWriteAt: latest}}
	if err != nil || len(got) != 1 || got[f.eng].Bytes != want[f.eng].Bytes || !got[f.eng].LastWriteAt.Equal(latest) {
		t.Errorf("NotebookActivities(eng, ops) = %+v, %v; want %+v, ops without changesets left out", got, err, want)
	}
}
