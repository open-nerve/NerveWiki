package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
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
		MaxQueued: maxQueued, MinFree: 100, ImportMaxBytes: 1 << 10})
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
	stored, err := s.Store(as(w.bob), strings.NewReader("PK zip"))
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
			stored, err := w.startImport(q, 2).Store(as(w.bob), strings.NewReader("zip"))
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
	stored, err := s.Store(as(w.bob), strings.NewReader("zip"))
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
	stored, err := s.Store(as(w.bob), strings.NewReader("zip"))
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
			stored, err := w.startImport(&queue{}, 20).Store(as(w.bob), bytes.NewReader(tt.body()))
			if !tt.check(err) {
				t.Errorf("Store() = %+v, %v", stored, err)
			}
			if err != nil && len(w.archives.imports) != 0 {
				t.Errorf("archives %v left", w.archives.imports)
			}
		})
	}
	w := newWorld()
	_, err := w.startImport(&queue{}, 20).Store(as(w.bob), iotest.ErrReader(readErr))
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
	stored, err := s.Store(context.Background(), strings.NewReader("zip"))
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
