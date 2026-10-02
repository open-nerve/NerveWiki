package notebook_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The page module's port on a real database (M4/P1 design 3.3).
func TestNotebooksFindAndLock(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", f.ops, testNow())
	nbs := notebook.NewNotebooks(f.pool)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	locks := map[string]func(context.Context, uuid.UUID) (bool, error){"ShareByID": nbs.ShareByID, "LockByID": nbs.LockByID}

	t.Run("WorkspaceOf", func(t *testing.T) {
		for _, tt := range []struct {
			id, want uuid.UUID // want zero: none
		}{{f.eng, f.acme}, {f.outside, f.other}, {f.ops, uuid.UUID{}}, {uuid.NewV7(), uuid.UUID{}}} {
			got, ok, err := nbs.WorkspaceOf(ctx, tt.id)
			if err != nil || ok != (tt.want != uuid.UUID{}) || got != tt.want {
				t.Errorf("WorkspaceOf(%s) = %s, %v, %v; want %s", tt.id, got, ok, err, tt.want)
			}
		}
	})

	for name, lock := range locks {
		t.Run(name, func(t *testing.T) {
			if _, err := lock(ctx, f.eng); err == nil {
				t.Errorf("%s outside a transaction = nil error", name)
			}
			for _, tt := range []struct {
				id   uuid.UUID
				want bool
			}{{f.eng, true}, {f.ops, false}, {uuid.NewV7(), false}} {
				if err := tx.WithinTx(ctx, func(ctx context.Context) error {
					ok, err := lock(ctx, tt.id)
					if err != nil || ok != tt.want {
						t.Errorf("%s(%s) = %v, %v; want %v", name, tt.id, ok, err, tt.want)
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

	// Each lock waits for a management write's FOR NO KEY UPDATE, and sees
	// the deletion it commits.
	for name, lock := range locks {
		t.Run(name+" waits and sees a deletion", func(t *testing.T) {
			id := f.newNotebook(t)
			holder, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(ctx) }()
			if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", id); err != nil {
				t.Fatal(err)
			}
			got := make(chan bool, 1)
			go func() {
				var ok bool
				if err := tx.WithinTx(ctx, func(ctx context.Context) (err error) {
					ok, err = lock(ctx, id)
					return err
				}); err != nil {
					t.Error(err)
				}
				got <- ok
			}()
			pgtest.WaitForLockWaitsOn(t, f.pool, "notebooks", 1, 10*time.Second)
			if _, err := holder.Exec(ctx, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", id, testNow()); err != nil {
				t.Fatal(err)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if <-got {
				t.Errorf("%s after a deletion committed while it waited = true, want none", name)
			}
		})
	}

	// Two page writes that change no tree run side by side; a tree write
	// waits for them.
	t.Run("shares do not wait for each other; a lock waits for a share", func(t *testing.T) {
		holder, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Rollback(ctx) }()
		if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR SHARE", f.eng); err != nil {
			t.Fatal(err)
		}
		quick, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := tx.WithinTx(quick, func(ctx context.Context) error {
			ok, err := nbs.ShareByID(ctx, f.eng)
			if !ok {
				t.Error("ShareByID beside a share = false, want the notebook")
			}
			return err
		}); err != nil {
			t.Errorf("ShareByID beside a share: %v, want no wait", err)
		}
		if err := tx.WithinTx(quick, func(ctx context.Context) error {
			_, err := nbs.LockByID(ctx, f.eng)
			return err
		}); err == nil {
			t.Error("LockByID beside a share = nil error, want it to wait past the deadline")
		}
	})
}

// newNotebook adds a notebook to acme, administered by alice.
func (f fixture) newNotebook(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, 'Notes', $3, $3, $4, $4)", id, f.acme, f.alice, testNow())
	return id
}
