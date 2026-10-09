package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (w *world) startImport(q *queue, maxQueued int) *app.StartImport {
	q.rec = w.rec
	w.auth.rules = map[shared.Action][]shared.NotebookRole{domain.ActionImport: {shared.NotebookEditor, shared.NotebookAdmin}}
	return app.NewStartImport(app.StartDeps{Tx: w.tx, Authorizer: w.auth, Workspaces: w.workspaces, Notebooks: w.notebooks, Nodes: w.nodes,
		Rows: w.rows, Archives: w.archives, Queue: q, Names: names{w.bob: "Bob"}, Signer: signer{}, Clock: fixedClock{now()}, Logger: w.logger,
		Uploads: w.uploads, MaxQueued: maxQueued, MinFree: 100, ImportMaxBytes: 1 << 10})
}

// An import starts in three steps: Check decides, unlocked; Store writes
// the archive; Create, under the workspace's lock and the notebook's,
// decides again, the queue counted one at a time, and writes the row,
// queued, named after the file, under the parent, enqueued last. Its log
// tells the bytes, not the file's name.
func TestAnImportStarts(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	q := &queue{}
	s := w.startImport(q, 20)
	req := app.ImportRequest{NotebookID: w.eng, ParentID: &w.spec, FileName: "a:b.zip", Client: domain.ClientAPI}
	if err := s.Check(as(w.bob), req); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Notebooks.WorkspaceOf", "Authorize", "Importing", "CountActive", "Free"}; !slices.Equal(w.rec.calls, want) {
		t.Errorf("Check's calls = %v, want %v", w.rec.calls, want)
	}
	stored, err := s.Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, strings.NewReader("PK zip"))
	if err != nil || stored.Bytes != 6 || string(w.archives.imports[stored.ID]) != "PK zip" {
		t.Fatalf("Store() = %+v, %v; archive %q", stored, err, w.archives.imports[stored.ID])
	}
	w.rec.calls = nil
	v, err := s.Create(as(w.bob), req, stored)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Notebooks.WorkspaceOf", "WithinTx", "Workspaces.ShareByID", "Notebooks.ShareByID", "Authorize", "LockQueue", "CountActive",
		"Importing", "CreateJob", "Queue.Import"}
	if !slices.Equal(w.rec.calls, want) {
		t.Errorf("Create's calls = %v, want %v", w.rec.calls, want)
	}
	j := v.Job
	if j.ID != stored.ID || j.Kind != domain.KindImport || j.State != domain.StateQueued || j.Name != "a_b.zip" || *j.RootID != w.spec ||
		j.CreatedBy != w.bob || j.Client != domain.ClientAPI || v.CreatedByName != "Bob" || v.Download != nil ||
		!slices.Equal(q.enqueued, []uuid.UUID{j.ID}) || w.rows.get(j.ID).ID != j.ID || !w.archives.imported(j.ID) {
		t.Errorf("Create() = %+v, enqueued %v", v, q.enqueued)
	}
	if text := l.String(); !strings.Contains(text, "import queued") || !strings.Contains(text, "bytes=6") || strings.Contains(text, "a_b") ||
		strings.Contains(text, "a:b") {
		t.Errorf("logs %q", text)
	}
	if _, err := s.Create(as(w.bob), app.ImportRequest{NotebookID: w.eng, FileName: " ", Client: domain.ClientWeb}, stored); !errors.Is(err, domain.ErrImportBusy) {
		t.Errorf("a second Create() = %v, want ErrImportBusy", err)
	}
}

// What refuses an import, each with its code: Check finds all but those
// of a deletion as Create locks and of River, Create all but the store's
// room, which the stored archive took; Create's refusal deletes the
// archive.
func TestAnImportIsRefused(t *testing.T) {
	enqueueErr := errors.New("River refused")
	const both, create, check = 0, 1, 2
	for _, tt := range []struct {
		name  string
		setUp func(w *world, q *queue, req *app.ImportRequest)
		want  error
		by    int
	}{
		{"no such notebook", func(_ *world, _ *queue, req *app.ImportRequest) { req.NotebookID = uuid.NewV7() }, domain.ErrNotebookNotFound, both},
		{"a notebook not seen", func(w *world, _ *queue, _ *app.ImportRequest) { delete(w.auth.roles[w.eng], w.bob) }, domain.ErrNotebookNotFound, both},
		{"a reader", func(w *world, _ *queue, _ *app.ImportRequest) { w.auth.roles[w.eng][w.bob] = shared.NotebookReader }, shared.Forbidden(), both},
		{"an attachment", func(w *world, _ *queue, req *app.ImportRequest) { req.ParentID = &w.png }, domain.ErrRootNotFound, both},
		{"no such page", func(_ *world, _ *queue, req *app.ImportRequest) { id := uuid.NewV7(); req.ParentID = &id }, domain.ErrRootNotFound, both},
		{"an import going, anyone's", func(w *world, _ *queue, _ *app.ImportRequest) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindImport, State: domain.StateRunning, CreatedBy: w.alice})
		}, domain.ErrImportBusy, both},
		{"the queue full", func(w *world, _ *queue, _ *app.ImportRequest) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindImport, State: domain.StateQueued, CreatedBy: w.bob})
		}, domain.ErrQueueFull, both},
		{"the store full", func(w *world, _ *queue, _ *app.ImportRequest) { w.archives.free = 99 }, domain.ErrStorageFull, check},
		{"a workspace deleted as it locks", func(w *world, _ *queue, _ *app.ImportRequest) { w.workspaces.gone = true }, domain.ErrNotebookNotFound, create},
		{"a notebook deleted as it locks", func(w *world, _ *queue, _ *app.ImportRequest) { w.notebooks.unshared = true }, domain.ErrNotebookNotFound, create},
		{"River refusing", func(_ *world, q *queue, _ *app.ImportRequest) { q.err = enqueueErr }, enqueueErr, create},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			q := &queue{}
			req := app.ImportRequest{NotebookID: w.eng, ParentID: &w.spec, FileName: "v.zip", Client: domain.ClientWeb}
			stored, err := w.startImport(q, 2).Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, strings.NewReader("zip"))
			if err != nil {
				t.Fatal(err)
			}
			tt.setUp(w, q, &req)
			s := w.startImport(q, 2)
			if err := s.Check(as(w.bob), req); tt.by != create && !errors.Is(err, tt.want) || tt.by == create && err != nil {
				t.Errorf("Check() = %v", err)
			}
			_, err = s.Create(as(w.bob), req, stored)
			if tt.by != check && !errors.Is(err, tt.want) || tt.by == check && err != nil {
				t.Errorf("Create() = %v", err)
			}
			if tt.by != check && (len(q.enqueued) != 0 || w.archives.imported(stored.ID)) {
				t.Errorf("enqueued %v, archive kept %v; want neither", q.enqueued, w.archives.imported(stored.ID))
			}
		})
	}
}

// An import being uploaded holds its notebook from Check until Release:
// another is transfer.busy until then, and a refused Check holds nothing;
// each counts toward MaxQueued; the bytes its body declares must leave
// MinFree in the store, or it is 507.
func TestAnImportsUploadHoldsItsNotebook(t *testing.T) {
	w := newWorld()
	ops := uuid.NewV7()
	w.auth.roles[ops] = map[uuid.UUID]shared.NotebookRole{w.bob: shared.NotebookAdmin}
	w.notebooks.names[ops] = "Ops"
	s := w.startImport(&queue{}, 2)
	eng := app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}
	other := eng
	other.NotebookID = ops
	check := func(req app.ImportRequest, want error) {
		t.Helper()
		if err := s.Check(as(w.bob), req); want == nil && err != nil || want != nil && !errors.Is(err, want) {
			t.Errorf("Check(%s) = %v, want %v", w.notebooks.names[req.NotebookID], err, want)
		}
	}
	check(eng, nil)
	check(eng, domain.ErrImportBusy)
	w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
	check(other, domain.ErrQueueFull)
	s.Release(eng)
	check(other, nil)
	s.Release(other)
	w.archives.free = 149
	eng.Size = 50
	check(eng, domain.ErrStorageFull)
	w.archives.free = 150
	check(eng, nil)
}

// The uploads under way count for every start, the exports' too, until
// their jobs' rows are written: each as a job in the queue, its declared
// bytes as written in the store. A panic in Check releases its claim.
func TestTheUploadsUnderWayCountForEveryStart(t *testing.T) {
	w := newWorld()
	ops := uuid.NewV7()
	w.auth.roles[ops] = map[uuid.UUID]shared.NotebookRole{w.bob: shared.NotebookAdmin}
	w.notebooks.names[ops] = "Ops"
	s, exports := w.startImport(&queue{}, 3), w.start(&queue{}, 3)
	w.archives.free = 200
	eng := app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: 30}
	other := eng
	other.NotebookID, other.Size = ops, 71
	if err := s.Check(as(w.bob), eng); err != nil {
		t.Fatal(err)
	}
	if err := s.Check(as(w.bob), other); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Check() of 71 bytes beside an upload of 30, 200 free = %v, want ErrStorageFull", err)
	}
	other.Size = 70
	if err := s.Check(as(w.bob), other); err != nil {
		t.Fatalf("Check() of 70 bytes beside an upload of 30, 200 free = %v", err)
	}
	w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
	if _, err := exports.Run(as(w.alice), w.eng, nil, domain.ClientWeb); !errors.Is(err, domain.ErrQueueFull) {
		t.Errorf("an export with a job running and two uploads, of 3 = %v, want ErrQueueFull", err)
	}
	// Each upload's row, once written, counts in its stead.
	for _, req := range []app.ImportRequest{eng, other} {
		stored, err := s.Store(as(w.bob), req, strings.NewReader("zip"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Create(as(w.bob), req, stored); err != nil {
			t.Errorf("Create(%s) = %v", w.notebooks.names[req.NotebookID], err)
		}
	}
	s.Release(eng)
	s.Release(other)

	w.rows = newRows()
	s = w.startImport(&queue{}, 3)
	w.archives.onFree = func() { panic("archives: the store's disk cannot be read") }
	func() {
		defer func() { _ = recover() }()
		_ = s.Check(as(w.bob), eng)
		t.Error("Check() did not panic")
	}()
	w.archives.onFree = nil
	if err := s.Check(as(w.bob), eng); err != nil {
		t.Errorf("Check() after one that panicked = %v, want the claim released", err)
	}
}

// An upload counts in the store the bytes it declared and has not stored
// yet: those it stored are the store's own. A Check under way counts for
// no other start until it admits its upload, and the next Check waits for
// it.
func TestAnUploadCountsWhatItHasStillToStore(t *testing.T) {
	w := newWorld()
	ops, dev := uuid.NewV7(), uuid.NewV7()
	for _, nb := range []uuid.UUID{ops, dev} {
		w.auth.roles[nb] = map[uuid.UUID]shared.NotebookRole{w.bob: shared.NotebookAdmin}
		w.notebooks.names[nb] = "Ops"
	}
	s := w.startImport(&queue{}, 2)
	w.archives.free = 200
	eng := app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: 60}
	other := eng
	other.NotebookID, other.Size = ops, 40
	if err := s.Check(as(w.bob), eng); err != nil {
		t.Fatal(err)
	}
	stored, err := s.Store(as(w.bob), eng, strings.NewReader(strings.Repeat("z", 50)))
	if err != nil {
		t.Fatal(err)
	}
	w.archives.free = 150 // the 50 bytes stored
	if err := s.Check(as(w.bob), other); err != nil {
		t.Errorf("Check() of 40 bytes, 150 free, 10 bytes of another upload still to store = %v", err)
	}
	s.Release(other)

	// other's Check stops in its decision, past the queue's.
	reached, resume := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	w.archives.onFree = func() {
		if first.CompareAndSwap(false, true) {
			close(reached)
			<-resume
		}
	}
	checked := make(chan error, 1)
	go func() { checked <- s.Check(as(w.bob), other) }()
	<-reached
	third := make(chan error, 1)
	go func() {
		third <- s.Check(as(w.bob), app.ImportRequest{NotebookID: dev, FileName: "v.zip", Client: domain.ClientWeb})
	}()
	w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
	if _, err := s.Create(as(w.bob), eng, stored); err != nil {
		t.Errorf("Create() beside a Check under way, the queue one short = %v", err)
	}
	select {
	case err := <-third:
		t.Errorf("a Check beside one under way = %v, want it to wait", err)
		third <- err
	case <-time.After(50 * time.Millisecond):
	}
	close(resume)
	if err := <-checked; !errors.Is(err, domain.ErrQueueFull) && err != nil {
		t.Errorf("the Check under way = %v", err)
	}
	<-third
}

// Create counts the uploads into other notebooks in the queue again: one
// whose upload ended as the queue filled is refused, server_busy.
func TestAnImportsCreateCountsTheOtherUploads(t *testing.T) {
	w := newWorld()
	ops := uuid.NewV7()
	w.auth.roles[ops] = map[uuid.UUID]shared.NotebookRole{w.bob: shared.NotebookAdmin}
	w.notebooks.names[ops] = "Ops"
	s := w.startImport(&queue{}, 2)
	eng := app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}
	other := eng
	other.NotebookID = ops
	for _, req := range []app.ImportRequest{eng, other} {
		if err := s.Check(as(w.bob), req); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := s.Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, strings.NewReader("zip"))
	if err != nil {
		t.Fatal(err)
	}
	w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
	if _, err := s.Create(as(w.bob), eng, stored); !errors.Is(err, domain.ErrQueueFull) {
		t.Errorf("Create() with another upload under way and a job running = %v, want ErrQueueFull", err)
	}
	s.Release(other)
	if _, err := s.Create(as(w.bob), eng, stored); err != nil {
		t.Errorf("Create() once the other upload ended = %v", err)
	}
}

// A commit whose answer was lost leaves the archive, which the sweep
// deletes when no job keeps it.
func TestAnImportWhoseCommitFailsKeepsItsArchive(t *testing.T) {
	w := newWorld()
	s := w.startImport(&queue{}, 20)
	stored, err := s.Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, strings.NewReader("zip"))
	if err != nil {
		t.Fatal(err)
	}
	w.tx.commitErr = errors.New("connection lost")
	if _, err := s.Create(as(w.bob), app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb}, stored); err == nil ||
		!w.archives.imported(stored.ID) {
		t.Errorf("Create() = %v, archive kept %v; want an error, the archive kept", err, w.archives.imported(stored.ID))
	}
}

// Store writes at most ImportMaxBytes; a file larger, a body that fails,
// a store out of room leave no archive; a commit that fails is its error.
func TestStoringAnImportsArchive(t *testing.T) {
	readErr := errors.New("connection reset")
	commitErr := errors.New("fsync failed")
	for _, tt := range []struct {
		name  string
		body  func() []byte
		r     func(b []byte) *bytes.Reader
		setUp func(a *archives)
		check func(err error) bool
	}{
		{"its largest", func() []byte { return bytes.Repeat([]byte("z"), 1<<10) }, nil, nil, func(err error) bool { return err == nil }},
		{"too large", func() []byte { return bytes.Repeat([]byte("z"), 1<<10+1) }, nil, nil, func(err error) bool { return errors.Is(err, app.ErrTooLarge) }},
		{"the store full", func() []byte { return []byte("zip") }, nil, func(a *archives) { a.uploadFull = true },
			func(err error) bool { return errors.Is(err, domain.ErrStorageFull) }},
		{"the commit failing", func() []byte { return []byte("zip") }, nil, func(a *archives) { a.commitErr = commitErr },
			func(err error) bool { return errors.Is(err, commitErr) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			if tt.setUp != nil {
				tt.setUp(w.archives)
			}
			stored, err := w.startImport(&queue{}, 20).Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, bytes.NewReader(tt.body()))
			if !tt.check(err) {
				t.Errorf("Store() = %+v, %v", stored, err)
			}
			if err != nil && len(w.archives.imports) != 0 {
				t.Errorf("archives %v left", w.archives.imports)
			}
		})
	}
	w := newWorld()
	_, err := w.startImport(&queue{}, 20).Store(as(w.bob), app.ImportRequest{NotebookID: w.eng}, iotest.ErrReader(readErr))
	var read *app.ReadError
	if !errors.As(err, &read) || !errors.Is(err, readErr) || len(w.archives.aborted) != 1 || len(w.archives.imports) != 0 {
		t.Errorf("Store() of a body failing = %v, aborted %v", err, w.archives.aborted)
	}
}

// Discard deletes a stored archive; one it cannot delete is logged.
func TestDiscardingAnImportsArchive(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	s := w.startImport(&queue{}, 20)
	stored, err := s.Store(context.Background(), app.ImportRequest{NotebookID: w.eng}, strings.NewReader("zip"))
	if err != nil {
		t.Fatal(err)
	}
	s.Discard(context.Background(), stored)
	if w.archives.imported(stored.ID) {
		t.Error("the archive is kept")
	}
	w.archives.deleteErr = errors.New("denied")
	s.Discard(context.Background(), stored)
	if !strings.Contains(l.String(), "import archive not deleted") {
		t.Errorf("logs %q", l.String())
	}
}
