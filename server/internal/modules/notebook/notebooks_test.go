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
	states := map[string]func(context.Context, uuid.UUID) (notebook.State, bool, error){
		"ShareState": nbs.ShareState, "LockState": nbs.LockState,
	}
	locks := map[string]func(context.Context, uuid.UUID) (bool, error){"ShareByID": nbs.ShareByID, "LockByID": nbs.LockByID}
	for name, state := range states {
		locks[name] = func(ctx context.Context, id uuid.UUID) (bool, error) {
			_, ok, err := state(ctx, id)
			return ok, err
		}
	}
	root := uuid.NewV7()
	f.exec(t, "UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", f.outside, root)
	wantStates := []struct {
		id   uuid.UUID
		want notebook.State
		ok   bool
	}{
		{f.outside, notebook.State{Mode: notebook.ModeLLMWiki, SourceRootID: root}, true},
		{f.eng, notebook.State{Mode: notebook.ModeStandard}, true},
		{f.ops, notebook.State{}, false},
		{uuid.NewV7(), notebook.State{}, false},
	}

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

	t.Run("NameOf", func(t *testing.T) {
		var want string
		if err := f.pool.QueryRow(ctx, "SELECT name FROM notebooks WHERE id = $1", f.eng).Scan(&want); err != nil {
			t.Fatal(err)
		}
		if got, ok, err := nbs.NameOf(ctx, f.eng); err != nil || !ok || got != want {
			t.Errorf("NameOf(eng) = %q, %v, %v; want %q", got, ok, err, want)
		}
		for _, id := range []uuid.UUID{f.ops, uuid.NewV7()} {
			if got, ok, err := nbs.NameOf(ctx, id); err != nil || ok || got != "" {
				t.Errorf("NameOf(%s) = %q, %v, %v; want none", id, got, ok, err)
			}
		}
	})

	// An LLM Wiki's state, a standard notebook's, and none of a deleted
	// notebook, unlocked, outside a transaction too (M10/P1 design 3.3).
	t.Run("StateOf", func(t *testing.T) {
		for _, tt := range wantStates {
			if got, ok, err := nbs.StateOf(ctx, tt.id); err != nil || ok != tt.ok || got != tt.want {
				t.Errorf("StateOf(%s) = %+v, %v, %v; want %+v, %v", tt.id, got, ok, err, tt.want, tt.ok)
			}
		}
	})

	// The same states read under each lock, in a transaction: a unit's
	// (M10/P1 design 3.18).
	for name, state := range states {
		t.Run(name+" reads the state", func(t *testing.T) {
			for _, tt := range wantStates {
				if err := tx.WithinTx(ctx, func(ctx context.Context) error {
					got, ok, err := state(ctx, tt.id)
					if err != nil || ok != tt.ok || got != tt.want {
						t.Errorf("%s(%s) = %+v, %v, %v; want %+v, %v", name, tt.id, got, ok, err, tt.want, tt.ok)
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

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
