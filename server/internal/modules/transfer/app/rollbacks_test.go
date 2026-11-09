package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// changeMode is the notebook module's notebook.change_mode, an admin's.
const changeMode shared.Action = "notebook.change_mode"

// rollbackOf adds by's rollback of an upgrade of eng in state.
func (w *world) rollbackOf(by uuid.UUID, state domain.State) domain.Job {
	j := w.job(by, now(), state)
	up, changeset := uuid.NewV7(), uuid.NewV7()
	w.rows.set(j.ID, func(r *row) {
		r.job.Kind, r.job.Name, r.job.UpgradeID, r.job.ChangesetID = domain.KindRollback, "upgrade", &up, &changeset
	})
	return w.rows.get(j.ID)
}

// An upgrade's rollback is its notebook's admins' to cancel (M11/P1 design
// 3.19): its starter, an admin no longer, is forbidden, the job left as it
// is; an admin cancels another's, queued or running, and an ended one is
// not cancellable; another who sees the notebook is not found, as for any
// job of another's, and so is one who cannot see it. A job of another
// kind its starter cancels as ever.
func TestOnlyAdminsCancelARollback(t *testing.T) {
	w := newWorld()
	carol, dave := uuid.NewV7(), uuid.NewV7()
	w.auth.roles[w.eng][carol] = shared.NotebookEditor
	w.auth.rules = map[shared.Action][]shared.NotebookRole{changeMode: {shared.NotebookAdmin}}
	c := app.NewCancel(app.CancelDeps{Tx: w.tx, Rows: w.rows, Authorizer: w.auth, Notebooks: w.notebooks, Names: names{}, Signer: signer{},
		Clock: fixedClock{now()}, Logger: w.logger, ExportTTL: ttl, RollbackAction: changeMode})

	queued := w.rollbackOf(carol, domain.StateQueued)
	if _, err := c.Run(as(carol), queued.ID); !errors.Is(err, shared.Forbidden()) || w.rows.get(queued.ID).State != domain.StateQueued {
		t.Errorf("its starter, an editor now, cancelling = %v, the job %s; want forbidden, queued", err, w.rows.get(queued.ID).State)
	}
	for name, who := range map[string]uuid.UUID{"a reader": w.alice, "an account that cannot see it": dave} {
		if _, err := c.Run(as(who), queued.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s cancelling another's rollback = %v, want transfer.not_found", name, err)
		}
	}
	if got, err := c.Run(as(w.bob), queued.ID); err != nil || got.Job.State != domain.StateCancelled {
		t.Errorf("the admin cancelling it = %+v, %v; want cancelled", got.Job, err)
	}
	running := w.rollbackOf(carol, domain.StateRunning)
	if got, err := c.Run(as(w.bob), running.ID); err != nil || got.Job.CancelRequested == nil {
		t.Errorf("the admin cancelling a running one = %+v, %v; want its cancel asked", got.Job, err)
	}
	ended := w.rollbackOf(w.bob, domain.StateFailed)
	if _, err := c.Run(as(w.bob), ended.ID); !errors.Is(err, domain.ErrNotCancellable) {
		t.Errorf("the admin cancelling an ended one = %v, want transfer.not_cancellable", err)
	}
	export := w.job(carol, now(), domain.StateQueued)
	if got, err := c.Run(as(carol), export.ID); err != nil || got.Job.State != domain.StateCancelled {
		t.Errorf("an editor cancelling her export = %+v, %v; want cancelled", got.Job, err)
	}
}

// An upgrade's rollback starts in a notebook rolling it back, the one job
// the freeze lets start (M11/P1 design 3.19): the row written queued, of
// the kind rollback, named upgrade, of the upgrade, from the client, by
// its caller, a changeset decided, enqueued last, the queue's room read
// before and under the locks. The
// rollback of another upgrade, or of one upgrading, is notebook.upgrading;
// a restore of a notebook rolling back is too. A caller no admin is
// forbidden; a full queue server_busy.
func TestARollbackStartsInItsUpgradeAlone(t *testing.T) {
	up := uuid.NewV7()
	rolling := func(rollingBack bool) (*world, *queue, app.Rollbacks) {
		w := newWorld()
		var l logs
		w.logger = l.logger()
		w.notebooks.upgrades, w.notebooks.rollingBack = map[uuid.UUID]uuid.UUID{w.eng: up}, rollingBack
		w.auth.rules = map[shared.Action][]shared.NotebookRole{changeMode: {shared.NotebookAdmin}}
		q := &queue{rec: w.rec}
		d := app.StartDeps{Tx: w.tx, Authorizer: w.auth, Workspaces: w.workspaces, Notebooks: w.notebooks, Nodes: w.nodes, Rows: w.rows,
			Archives: w.archives, Queue: q, Names: names{}, Signer: signer{}, Clock: fixedClock{now()}, Logger: w.logger,
			Uploads: w.uploads, MaxQueued: 20, MinFree: 100}
		return w, q, app.NewRollbacks(d, changeMode)
	}

	w, q, r := rolling(true)
	id, err := r.Start(as(w.bob), w.eng, up, domain.ClientWeb)
	if err != nil {
		t.Fatal(err)
	}
	got := w.rows.get(id)
	if got.Kind != domain.KindRollback || got.State != domain.StateQueued || got.Name != "upgrade" || got.UpgradeID == nil ||
		*got.UpgradeID != up || got.CreatedBy != w.bob || got.Client != domain.ClientWeb || got.ChangesetID == nil ||
		!got.CreatedAt.Equal(now()) {
		t.Errorf("the job %+v, want bob's rollback of the upgrade, queued, named upgrade, its changeset decided", got)
	}
	if want := []string{"CountActive", "Notebooks.WorkspaceOf", "WithinTx", "Workspaces.ShareByID", "Notebooks.ShareByID", "Authorize", "LockQueue",
		"CountActive", "Writing", "CreateJob", "Queue.Rollback"}; !slices.Equal(w.rec.of(want...), want) || !slices.Equal(q.enqueued,
		[]uuid.UUID{id}) {
		t.Errorf("calls %v, enqueued %v; want %v, the job enqueued last", w.rec.calls, q.enqueued, want)
	}

	for name, start := range map[string]func(w *world, r app.Rollbacks) error{
		"another upgrade's": func(w *world, r app.Rollbacks) error {
			_, err := r.Start(as(w.bob), w.eng, uuid.NewV7(), domain.ClientWeb)
			return err
		},
	} {
		w, q, r := rolling(true)
		if err := start(w, r); !errors.Is(err, domain.ErrUpgrading) || len(q.enqueued) != 0 {
			t.Errorf("the rollback of %s = %v, enqueued %v; want notebook.upgrading, none", name, err, q.enqueued)
		}
	}
	w, q, r = rolling(false)
	if _, err := r.Start(as(w.bob), w.eng, up, domain.ClientWeb); !errors.Is(err, domain.ErrUpgrading) || len(q.enqueued) != 0 {
		t.Errorf("the rollback of an upgrade upgrading = %v, enqueued %v; want notebook.upgrading, none", err, q.enqueued)
	}
	w, _, _ = rolling(true)
	tree := newJobTree(w.rec)
	s := w.starts(&queue{}, tree, 20)
	if _, err := start(w, s, domain.KindTrashRestore); !errors.Is(err, domain.ErrUpgrading) {
		t.Errorf("a restore of a notebook rolling back = %v, want notebook.upgrading", err)
	}
	w, q, r = rolling(true)
	w.auth.roles[w.eng][w.bob] = shared.NotebookEditor
	if _, err := r.Start(as(w.bob), w.eng, up, domain.ClientWeb); !errors.Is(err, shared.Forbidden()) || len(q.enqueued) != 0 {
		t.Errorf("an editor's rollback = %v, want forbidden", err)
	}
	w, q, r = rolling(true)
	for range 20 {
		w.job(w.alice, now().Add(-time.Minute), domain.StateQueued)
	}
	var busy *shared.Error
	if _, err := r.Start(as(w.bob), w.eng, up, domain.ClientWeb); !errors.As(err, &busy) || busy.Code != "server_busy" ||
		len(q.enqueued) != 0 {
		t.Errorf("a rollback with the queue full = %v, want server_busy", err)
	}
}
