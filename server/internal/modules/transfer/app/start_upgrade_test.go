package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// A notebook being upgraded refuses the starts of the jobs that write its
// tree, each notebook.upgrading (M11/P1 design 3.5): an import's Check,
// claiming nothing; an import Check let by before the upgrade's start, at
// its Create, under the notebook's lock, its archive deleted; a restore's
// start and a revert's, under the lock, no row written and the notebook
// left unclaimed. An export reads, and starts as ever.
func TestTheStartsRefuseAnUpgradingNotebook(t *testing.T) {
	upgrading := func() *world {
		w := newWorld()
		w.notebooks.upgrades = map[uuid.UUID]uuid.UUID{w.eng: uuid.NewV7()}
		return w
	}
	req := func(w *world) app.ImportRequest {
		return app.ImportRequest{NotebookID: w.eng, ParentID: &w.spec, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}
	}

	t.Run("an import's Check", func(t *testing.T) {
		w := upgrading()
		s := w.startImport(&queue{}, 2)
		if err := s.Check(as(w.bob), req(w)); !errors.Is(err, domain.ErrUpgrading) {
			t.Errorf("Check() = %v, want notebook.upgrading", err)
		}
		if calls := w.rec.calls; calls[len(calls)-1] != "Notebooks.StateOf" {
			t.Errorf("calls = %v, want none after the state's read", calls)
		}
		delete(w.notebooks.upgrades, w.eng)
		if err := s.Check(as(w.bob), req(w)); err != nil {
			t.Errorf("Check() once the upgrade ended = %v, want the notebook left unclaimed", err)
		}
	})

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
