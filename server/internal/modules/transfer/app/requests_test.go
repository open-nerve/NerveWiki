package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func as(user uuid.UUID) context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: user, SessionID: uuid.NewV7()})
}

func (w *world) start(q *queue, maxQueued int) *app.StartExport {
	return app.NewStartExport(app.StartDeps{Tx: w.tx, Authorizer: w.auth, Workspaces: workspaces{}, Notebooks: w.notebooks, Nodes: w.nodes,
		Rows: w.rows, Archives: w.archives, Queue: q, Clock: fixedClock{now()}, Logger: quiet(), MaxQueued: maxQueued, MinFree: 100})
}

// An export starts queued, named after what it exports, from the client,
// enqueued; under the notebook's lock, after the decision, the queue
// counted one at a time.
func TestAnExportStarts(t *testing.T) {
	w := newWorld()
	q := &queue{}
	j, err := w.start(q, 20).Run(as(w.alice), w.eng, nil, domain.ClientAPI)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != domain.StateQueued || j.Name != "Eng" || j.CreatedBy != w.alice || j.Client != domain.ClientAPI || j.RootID != nil ||
		!j.CreatedAt.Equal(now()) || !slices.Equal(q.enqueued, []uuid.UUID{j.ID}) || w.rows.get(j.ID).ID != j.ID || w.rows.locks != 1 {
		t.Errorf("Run() = %+v, enqueued %v, %d locks", j, q.enqueued, w.rows.locks)
	}
	sub, err := w.start(&queue{}, 20).Run(as(w.bob), w.eng, &w.folder, domain.ClientWeb)
	if err != nil || sub.Name != "Folder" || *sub.RootID != w.folder {
		t.Errorf("Run(Folder) = %+v, %v", sub, err)
	}
}

// What refuses an export, each with its code.
func TestAnExportIsRefused(t *testing.T) {
	enqueueErr := errors.New("River refused")
	for _, tt := range []struct {
		name  string
		setUp func(w *world, q *queue) (uuid.UUID, *uuid.UUID)
		want  error
	}{
		{"no such notebook", func(*world, *queue) (uuid.UUID, *uuid.UUID) { return uuid.NewV7(), nil }, domain.ErrNotebookNotFound},
		{"a notebook not seen", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) {
			delete(w.auth.roles[w.eng], w.alice)
			return w.eng, nil
		}, domain.ErrNotebookNotFound},
		{"a notebook deleted as it locks", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) {
			w.notebooks.unshared = true
			return w.eng, nil
		}, domain.ErrNotebookNotFound},
		{"an attachment", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) { return w.eng, &w.png }, domain.ErrRootNotFound},
		{"no such page", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) { id := uuid.NewV7(); return w.eng, &id }, domain.ErrRootNotFound},
		{"the queue full", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateQueued, CreatedBy: w.bob})
			return w.eng, nil
		}, domain.ErrQueueFull},
		{"an export of hers going", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) {
			w.queued(nil)
			return w.eng, nil
		}, domain.ErrBusy},
		{"the store full", func(w *world, _ *queue) (uuid.UUID, *uuid.UUID) { w.archives.free = 99; return w.eng, nil }, domain.ErrStorageFull},
		{"River refusing", func(w *world, q *queue) (uuid.UUID, *uuid.UUID) { q.err = enqueueErr; return w.eng, nil }, enqueueErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			q := &queue{}
			notebook, root := tt.setUp(w, q)
			if _, err := w.start(q, 2).Run(as(w.alice), notebook, root, domain.ClientWeb); !errors.Is(err, tt.want) {
				t.Errorf("Run() = %v, want %v", err, tt.want)
			}
			if len(q.enqueued) != 0 {
				t.Errorf("enqueued %v", q.enqueued)
			}
		})
	}
	var problem *shared.Error
	if !errors.As(domain.ErrQueueFull, &problem) || problem.ProblemStatus() != 503 || problem.RetryAfter() <= 0 {
		t.Errorf("ErrQueueFull = %+v, want 503 with Retry-After", domain.ErrQueueFull)
	}
}

func (w *world) reads() *app.Reads {
	return app.NewReads(app.ReadsDeps{Authorizer: w.auth, Notebooks: w.notebooks, Names: names{w.alice: "Alice", w.bob: "Bob"}, Signer: signer{},
		Clock: fixedClock{now()}, Rows: w.rows})
}

// job adds a job of eng by by, made at, in state.
func (w *world) job(by uuid.UUID, at time.Time, state domain.State) domain.Job {
	j := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindExport, State: state, Name: "Eng", CreatedBy: by, CreatedAt: at}
	w.rows.add(j)
	return j
}

// A job is its starter's to read, or the notebook's admin's; a reader of
// another's, or of a notebook they cannot see, finds none. A succeeded
// export's address is signed.
func TestReadingAJob(t *testing.T) {
	w := newWorld()
	alices := w.job(w.alice, now(), domain.StateSucceeded)
	bobs := w.job(w.bob, now(), domain.StateRunning)
	r := w.reads()

	got, err := r.Get(as(w.alice), alices.ID)
	if err != nil || got.CreatedByName != "Alice" || got.Download == nil || got.Download.Signature != "sig-"+alices.ID.String() {
		t.Errorf("Get(her own) = %+v, %v", got, err)
	}
	if got, err := r.Get(as(w.bob), alices.ID); err != nil || got.Job.ID != alices.ID {
		t.Errorf("Get() by the admin = %+v, %v", got, err)
	}
	if got, err := r.Get(as(w.bob), bobs.ID); err != nil || got.Download != nil {
		t.Errorf("Get(running) = %+v, %v; want no address", got, err)
	}
	for name, ctx := range map[string]context.Context{"another's": as(w.alice), "a stranger": as(uuid.NewV7())} {
		id := bobs.ID
		if name == "a stranger" {
			id = alices.ID
		}
		if _, err := r.Get(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Get(%s) = %v, want transfer.not_found", name, err)
		}
	}
	if _, err := r.Get(as(w.alice), uuid.NewV7()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get(none) = %v", err)
	}
}

// A notebook's jobs list newest first, a page at a time: a reader's own,
// every one for its admin.
func TestListingTheJobs(t *testing.T) {
	w := newWorld()
	a1 := w.job(w.alice, now(), domain.StateFailed)
	b1 := w.job(w.bob, now().Add(time.Minute), domain.StateSucceeded)
	a2 := w.job(w.alice, now().Add(2*time.Minute), domain.StateSucceeded)
	r := w.reads()
	ids := func(p app.JobPage) []uuid.UUID {
		var out []uuid.UUID
		for _, j := range p.Jobs {
			out = append(out, j.Job.ID)
		}
		return out
	}

	mine, err := r.List(as(w.alice), w.eng, nil, nil)
	if err != nil || !slices.Equal(ids(mine), []uuid.UUID{a2.ID, a1.ID}) || mine.NextCursor != "" {
		t.Errorf("List(alice) = %v, %q, %v", ids(mine), mine.NextCursor, err)
	}
	one := 1
	first, err := r.List(as(w.bob), w.eng, &one, nil)
	if err != nil || !slices.Equal(ids(first), []uuid.UUID{a2.ID}) || first.NextCursor == "" {
		t.Fatalf("List(admin, 1) = %v, %q, %v", ids(first), first.NextCursor, err)
	}
	two := 2
	rest, err := r.List(as(w.bob), w.eng, &two, &first.NextCursor)
	if err != nil || !slices.Equal(ids(rest), []uuid.UUID{b1.ID, a1.ID}) || rest.NextCursor != "" {
		t.Errorf("the next page = %v, %q, %v", ids(rest), rest.NextCursor, err)
	}
	if _, err := r.List(as(uuid.NewV7()), w.eng, nil, nil); !errors.Is(err, domain.ErrNotebookNotFound) {
		t.Errorf("List(stranger) = %v", err)
	}
	bad := "x"
	if _, err := r.List(as(w.alice), w.eng, nil, &bad); err == nil {
		t.Error("List(a bad cursor) = nil error")
	}
	zero := 0
	if _, err := r.List(as(w.alice), w.eng, &zero, nil); err == nil {
		t.Error("List(limit 0) = nil error")
	}
}

func (w *world) cancel() *app.Cancel {
	return app.NewCancel(app.CancelDeps{Tx: w.tx, Rows: w.rows, Authorizer: w.auth, Notebooks: w.notebooks, Names: names{}, Signer: signer{},
		Clock: fixedClock{now()}})
}

// A queued job is cancelled at once; a running one is asked to stop, once;
// an ended one cannot be. Another's is the admin's to cancel.
func TestCancellingAJob(t *testing.T) {
	w := newWorld()
	queued, running, ended := w.job(w.alice, now(), domain.StateQueued), w.job(w.alice, now(), domain.StateRunning), w.job(w.alice, now(), domain.StateFailed)
	c := w.cancel()

	if got, err := c.Run(as(w.alice), queued.ID); err != nil || got.Job.State != domain.StateCancelled || got.Job.Report == nil {
		t.Errorf("Run(queued) = %+v, %v", got.Job, err)
	}
	got, err := c.Run(as(w.bob), running.ID)
	if err != nil || got.Job.State != domain.StateRunning || got.Job.CancelRequested == nil {
		t.Errorf("Run(running) by the admin = %+v, %v", got.Job, err)
	}
	if _, err := c.Run(as(w.alice), ended.ID); !errors.Is(err, domain.ErrNotCancellable) {
		t.Errorf("Run(ended) = %v", err)
	}
	bobs := w.job(w.bob, now(), domain.StateQueued)
	if _, err := c.Run(as(w.alice), bobs.ID); !errors.Is(err, domain.ErrNotFound) || w.rows.get(bobs.ID).State != domain.StateQueued {
		t.Errorf("Run(another's) = %v", err)
	}
}

// A download opens the archive its signature signs, of an export that
// succeeded; anything else is not found, the signature checked first.
func TestDownloadingAnArchive(t *testing.T) {
	w := newWorld()
	ok, running, missing := w.job(w.alice, now(), domain.StateSucceeded), w.job(w.alice, now(), domain.StateRunning), w.job(w.alice, now(), domain.StateSucceeded)
	w.archives.committed[ok.ID] = nil
	w.archives.committed[running.ID] = nil
	d := app.NewDownload(w.rows, w.archives, signer{}, fixedClock{now()}, quiet())
	address := func(id uuid.UUID) app.Address {
		s := signer{}.Sign(now(), id)
		return app.Address{JobID: id, Expires: s.Expires.Unix(), Signature: s.Signature}
	}

	got, err := d.Open(context.Background(), address(ok.ID))
	if err != nil || got.Name != "Eng.zip" || got.File == nil {
		t.Errorf("Open() = %+v, %v", got, err)
	}
	finds := w.rows.finds
	forged := address(ok.ID)
	forged.Signature = "sig-" + running.ID.String()
	if _, err := d.Open(context.Background(), forged); !errors.Is(err, domain.ErrDownloadNotFound) || w.rows.finds != finds {
		t.Errorf("Open(forged) = %v after %d reads, want not_found before any", err, w.rows.finds-finds)
	}
	for name, id := range map[string]uuid.UUID{"running": running.ID, "file missing": missing.ID, "no job": uuid.NewV7()} {
		if _, err := d.Open(context.Background(), address(id)); !errors.Is(err, domain.ErrDownloadNotFound) {
			t.Errorf("Open(%s) = %v, want not_found", name, err)
		}
	}
	expired := address(ok.ID)
	expired.Expires = now().Unix()
	if _, err := d.Open(context.Background(), expired); !errors.Is(err, domain.ErrDownloadNotFound) {
		t.Errorf("Open(expired) = %v", err)
	}
}

// The exports expire a batch at a time, their archives deleted.
func TestExportsExpireTheirArchives(t *testing.T) {
	m := &maintained{expire: [][]uuid.UUID{make([]uuid.UUID, 100), {uuid.NewV7()}}}
	for i := range m.expire[0] {
		m.expire[0][i] = uuid.NewV7()
	}
	a := newArchives()
	n, err := app.NewExpire(m, a, fixedClock{now()}, quiet(), 24*time.Hour).Run(context.Background())
	if err != nil || n != 101 || len(a.deleted) != 101 || !m.expiredAt[0].Equal(now().Add(-24*time.Hour)) {
		t.Errorf("Run() = %d, %v, deleted %d, before %v", n, err, len(a.deleted), m.expiredAt)
	}
}

// The rescue fails every running job as the server starts, then those
// whose heartbeat is older than the timeout.
func TestTheRescue(t *testing.T) {
	m := &maintained{interrupted: []app.Interrupted{{ID: uuid.NewV7()}}}
	r := app.NewRescue(m, fixedClock{now()}, quiet(), 5*time.Minute)
	if err := r.AtStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(m.beatBefore) != 2 || m.beatBefore[0] != nil || !m.beatBefore[1].Equal(now().Add(-5*time.Minute)) {
		t.Errorf("asked %v, want all, then before five minutes ago", m.beatBefore)
	}
}

// The sweep deletes the old archives no job keeps.
func TestTheSweepDeletesOrphanArchives(t *testing.T) {
	kept, orphan := uuid.NewV7(), uuid.NewV7()
	m := &maintained{live: []uuid.UUID{kept}}
	a := newArchives()
	a.listed = []uuid.UUID{kept, orphan}
	n, err := app.NewSweep(m, a, fixedClock{now()}, quiet()).Run(context.Background())
	if err != nil || n != 1 || !slices.Equal(a.deleted, []uuid.UUID{orphan}) {
		t.Errorf("Run() = %d, %v, deleted %v; want the orphan", n, err, a.deleted)
	}
	a.deleteErr = errors.New("denied")
	if _, err := app.NewSweep(m, a, fixedClock{now()}, quiet()).Run(context.Background()); err == nil {
		t.Error("Run() with a file not deleted = nil error")
	}
}

// The purge deletes the archives, then the rows; an archive not deleted
// keeps them.
func TestThePurgeDeletesArchivesThenRows(t *testing.T) {
	id := uuid.NewV7()
	m := &maintained{expiredJobs: map[uuid.UUID]domain.Kind{id: domain.KindExport}}
	a := newArchives()
	p := app.NewPurge(&direct{}, m, a, quiet())
	if n, err := p.Batch(context.Background(), now(), 10); n != 1 || err != nil || !slices.Equal(a.deleted, []uuid.UUID{id}) ||
		!slices.Equal(m.purged, []uuid.UUID{id}) {
		t.Errorf("Batch() = %d, %v; deleted %v, purged %v", n, err, a.deleted, m.purged)
	}
	m.purged = nil
	a.deleteErr = errors.New("denied")
	if n, err := p.Batch(context.Background(), now(), 10); n != 0 || err == nil || len(m.purged) != 0 {
		t.Errorf("Batch() with an archive not deleted = %d, %v, purged %v", n, err, m.purged)
	}
	if err := app.NewFollow(m).NotebooksDeleted(context.Background(), []uuid.UUID{id}, now()); err != nil || !slices.Equal(m.deletedOf, []uuid.UUID{id}) {
		t.Errorf("NotebooksDeleted() = %v, %v", err, m.deletedOf)
	}
}
