package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A notebook being upgraded refuses the starts of the jobs that write its
// tree, each notebook.upgrading right after the decision (M11/P1 design
// 3.5, 4.5): an import's Check, before the parent is read, claiming
// nothing, notebook.not_found for a notebook deleted as its state is read;
// an import Check let by before the upgrade's start, at its Create, under
// the notebook's lock, its archive deleted; a restore's start and a
// revert's, under the lock, no row written and the notebook left
// unclaimed, a reader's forbidden. An export reads, and starts as ever.
func TestTheStartsRefuseAnUpgradingNotebook(t *testing.T) {
	upgrading := func() *world {
		w := newWorld()
		w.notebooks.upgrades = map[uuid.UUID]uuid.UUID{w.eng: uuid.NewV7()}
		return w
	}
	req := func(w *world) app.ImportRequest {
		return app.ImportRequest{NotebookID: w.eng, ParentID: &w.spec, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}
	}
	// nowhere is req under a parent no page: page.not_found, once the
	// parent is read.
	nowhere := func(w *world) app.ImportRequest {
		r, id := req(w), uuid.NewV7()
		r.ParentID = &id
		return r
	}

	for _, tt := range []struct {
		name  string
		setUp func(w *world)
		want  error
	}{
		{"", func(*world) {}, domain.ErrUpgrading},
		{", the notebook deleted as its state is read", func(w *world) { w.notebooks.goneAsRead = true }, domain.ErrNotebookNotFound},
	} {
		t.Run("an import's Check"+tt.name, func(t *testing.T) {
			w := upgrading()
			tt.setUp(w)
			s := w.startImport(&queue{}, 2)
			if err := s.Check(as(w.bob), nowhere(w)); !errors.Is(err, tt.want) {
				t.Errorf("Check() = %v, want %v", err, tt.want)
			}
			if want := []string{"Notebooks.WorkspaceOf", "Authorize", "Notebooks.StateOf"}; !slices.Equal(w.rec.calls, want) {
				t.Errorf("calls = %v, want %v and nothing after the state's read", w.rec.calls, want)
			}
			w.notebooks.goneAsRead = false
			delete(w.notebooks.upgrades, w.eng)
			s = w.startImport(&queue{}, 2)
			if err := s.Check(as(w.bob), nowhere(w)); !errors.Is(err, domain.ErrRootNotFound) {
				t.Errorf("Check() under no page once the upgrade ended = %v, want page.not_found", err)
			}
			if err := s.Check(as(w.bob), req(w)); err != nil {
				t.Errorf("Check() once the upgrade ended = %v, want the notebook left unclaimed", err)
			}
		})
	}

	t.Run("an import's Create", func(t *testing.T) {
		w := upgrading()
		w.notebooks.upgradedAsLocked = true
		q := &queue{}
		s := w.startImport(q, 2)
		if err := s.Check(as(w.bob), req(w)); err != nil {
			t.Fatalf("Check() before the upgrade's start = %v", err)
		}
		stored, err := s.Store(as(w.bob), req(w), strings.NewReader("zip"))
		if err != nil {
			t.Fatal(err)
		}
		w.rec.calls = nil
		if _, err := s.Create(as(w.bob), req(w), stored); !errors.Is(err, domain.ErrUpgrading) {
			t.Errorf("Create() = %v, want notebook.upgrading", err)
		}
		if slices.Contains(w.rec.calls, "CreateJob") || len(q.enqueued) != 0 || w.archives.imported(stored.ID) {
			t.Errorf("calls %v, enqueued %v, archive kept %v; want no row, nothing enqueued, the archive deleted", w.rec.calls,
				q.enqueued, w.archives.imported(stored.ID))
		}
	})

	for _, kind := range []domain.Kind{domain.KindTrashRestore, domain.KindRevert} {
		t.Run("a "+string(kind)+"'s start", func(t *testing.T) {
			w := upgrading()
			q, tree := &queue{}, newJobTree(w.rec)
			s := w.starts(q, tree, 20)
			if _, err := start(w, s, kind); !errors.Is(err, domain.ErrUpgrading) {
				t.Errorf("Execute() = %v, want notebook.upgrading", err)
			}
			if slices.Contains(w.rec.calls, "CreateJob") || len(q.enqueued) != 0 {
				t.Errorf("calls %v, enqueued %v; want no row and nothing enqueued", w.rec.calls, q.enqueued)
			}
			delete(w.notebooks.upgrades, w.eng)
			if err := s.imports.Check(as(w.bob), req(w)); err != nil {
				t.Errorf("an import's Check once the upgrade ended = %v, want the notebook left unclaimed", err)
			}
		})

		// The page module's check let a reader by: the decision under
		// the lock refuses it before the upgrade does.
		t.Run("a "+string(kind)+"'s start by a reader", func(t *testing.T) {
			w := upgrading()
			q, tree := &queue{}, newJobTree(w.rec)
			s := w.starts(q, tree, 20)
			w.auth.roles[w.eng][w.bob] = shared.NotebookReader
			if _, err := start(w, s, kind); !errors.Is(err, shared.Forbidden()) {
				t.Errorf("Execute() = %v, want forbidden", err)
			}
			if slices.Contains(w.rec.calls, "CreateJob") || len(q.enqueued) != 0 {
				t.Errorf("calls %v, enqueued %v; want no row and nothing enqueued", w.rec.calls, q.enqueued)
			}
		})
	}

	t.Run("an export", func(t *testing.T) {
		w := upgrading()
		q := &queue{}
		v, err := w.start(q, 20).Run(as(w.alice), w.eng, nil, domain.ClientWeb)
		if err != nil {
			t.Fatalf("an export of a notebook being upgraded = %v", err)
		}
		if !slices.Equal(q.enqueued, []uuid.UUID{v.Job.ID}) {
			t.Errorf("enqueued %v, want the export's job", q.enqueued)
		}
	})
}
