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

// The page module's actions the starts decide again, as bootstrap hands
// them.
const (
	restoreAction shared.Action = "node.restore"
	revertAction  shared.Action = "changeset.revert"
)

// starts are a world's background starts, through its tree, and its
// imports' start, all sharing its uploads.
type starts struct {
	restore *app.StartTrashRestore
	revert  *app.StartRevert
	imports *app.StartImport
}

func (w *world) starts(q *queue, tree *jobTree, maxQueued int) starts {
	q.rec = w.rec
	w.auth.rules = map[shared.Action][]shared.NotebookRole{restoreAction: {shared.NotebookEditor, shared.NotebookAdmin},
		revertAction: {shared.NotebookEditor, shared.NotebookAdmin}, domain.ActionImport: {shared.NotebookEditor, shared.NotebookAdmin}}
	d := app.StartDeps{Tx: w.tx, Authorizer: w.auth, Workspaces: w.workspaces, Notebooks: w.notebooks, Nodes: w.nodes, Rows: w.rows,
		Archives: w.archives, Queue: q, Names: names{w.bob: "Bob"}, Signer: signer{}, Clock: fixedClock{now()}, Logger: w.logger,
		Uploads: w.uploads, MaxQueued: maxQueued, MinFree: 100, ImportMaxBytes: 1 << 10}
	b := app.BackgroundDeps{Tree: tree, RestoreAction: restoreAction, RevertAction: revertAction}
	return starts{restore: app.NewStartTrashRestore(d, b), revert: app.NewStartRevert(d, b), imports: app.NewStartImport(d)}
}

// A restore starts as an import is created: the page module's check,
// unlocked, then the notebook claimed and its writing jobs read; the
// queue's room; then, under the workspace's lock and the notebook's, the
// restore decided again, the queue counted one at a time, and the row
// written, queued, named after the item, its root the item, the
// changeset it will write decided, enqueued last. Its log tells the item,
// not its name.
func TestARestoreStarts(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	q, tree := &queue{}, newJobTree(w.rec)
	item := uuid.NewV7()
	v, err := w.starts(q, tree, 20).restore.Execute(as(w.bob), app.RestoreRequest{NotebookID: w.eng, NodeID: item, Client: domain.ClientAPI})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Tree.CheckRestore", "Writing", "CountActive", "Notebooks.WorkspaceOf", "WithinTx", "Workspaces.ShareByID",
		"Notebooks.ShareByID", "Authorize", "LockQueue", "CountActive", "Writing", "CreateJob", "Queue.TrashRestore"}
	if !slices.Equal(w.rec.calls, want) {
		t.Errorf("calls = %v, want %v", w.rec.calls, want)
	}
	j := v.Job
	if j.Kind != domain.KindTrashRestore || j.State != domain.StateQueued || j.Name != "Box" || j.RootID == nil || *j.RootID != item ||
		j.TargetChangesetID != nil || j.Message != nil || j.CreatedBy != w.bob || j.Client != domain.ClientAPI || !j.CreatedAt.Equal(now()) ||
		v.CreatedByName != "Bob" || !slices.Equal(q.enqueued, []uuid.UUID{j.ID}) || w.rows.get(j.ID).ID != j.ID {
		t.Errorf("Execute() = %+v, enqueued %v", v, q.enqueued)
	}
	if j.ChangesetID == nil || *j.ChangesetID == (uuid.UUID{}) || *j.ChangesetID == j.ID || *w.rows.get(j.ID).ChangesetID != *j.ChangesetID {
		t.Errorf("the changeset decided %v, want one of its own on the row", j.ChangesetID)
	}
	if text := l.String(); !strings.Contains(text, "trash_restore queued") || !strings.Contains(text, "root_id="+item.String()) ||
		strings.Contains(text, "Box") {
		t.Errorf("logs %q", text)
	}
}

// A revert starts as a restore does, the page module's check claiming the
// notebook as it admits the revert; named after the changeset's kind,
// its target the changeset, with its message, which its log does not
// tell.
func TestARevertStarts(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	q, tree := &queue{}, newJobTree(w.rec)
	tree.onAdmit = func() { w.rec.add("admitted") }
	target, message := uuid.NewV7(), "Undo the vault"
	v, err := w.starts(q, tree, 20).revert.Execute(as(w.bob), app.RevertRequest{NotebookID: w.eng, ChangesetID: target, Message: &message,
		Client: domain.ClientWeb})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Tree.CheckRevert", "Writing", "admitted", "CountActive", "Notebooks.WorkspaceOf", "WithinTx", "Workspaces.ShareByID",
		"Notebooks.ShareByID", "Authorize", "LockQueue", "CountActive", "Writing", "CreateJob", "Queue.Revert"}
	if !slices.Equal(w.rec.calls, want) {
		t.Errorf("calls = %v, want %v", w.rec.calls, want)
	}
	j := v.Job
	if j.Kind != domain.KindRevert || j.State != domain.StateQueued || j.Name != "import" || j.RootID != nil || j.TargetChangesetID == nil ||
		*j.TargetChangesetID != target || j.Message == nil || *j.Message != message || j.Client != domain.ClientWeb ||
		j.ChangesetID == nil || *j.ChangesetID == target || !slices.Equal(q.enqueued, []uuid.UUID{j.ID}) {
		t.Errorf("Execute() = %+v, enqueued %v", v, q.enqueued)
	}
	if text := l.String(); !strings.Contains(text, "revert queued") || !strings.Contains(text, "target_changeset_id="+target.String()) ||
		strings.Contains(text, "Undo") {
		t.Errorf("logs %q", text)
	}
}

// What refuses a background start, each with its code, the restore's and
// the revert's alike: nothing enqueued, and the notebook left unclaimed.
func TestABackgroundStartIsRefused(t *testing.T) {
	enqueueErr, locked := errors.New("River refused"), &shared.Error{Kind: shared.KindConflict, Code: "page.locked"}
	for _, tt := range []struct {
		name  string
		setUp func(w *world, q *queue, tree *jobTree)
		want  error
	}{
		{"the page module's check", func(_ *world, _ *queue, tree *jobTree) {
			tree.refuse = &shared.Error{Kind: shared.KindConflict, Code: "page.not_in_trash"}
		}, &shared.Error{Kind: shared.KindConflict, Code: "page.not_in_trash"}},
		{"an import going, anyone's", func(w *world, _ *queue, _ *jobTree) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindImport, State: domain.StateRunning, CreatedBy: w.alice})
		}, domain.ErrWriting},
		{"a restore queued", func(w *world, _ *queue, _ *jobTree) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindTrashRestore, State: domain.StateQueued, CreatedBy: w.bob})
		}, domain.ErrWriting},
		{"a revert running", func(w *world, _ *queue, _ *jobTree) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindRevert, State: domain.StateRunning, CreatedBy: w.bob})
		}, domain.ErrWriting},
		{"an import uploading", func(w *world, _ *queue, _ *jobTree) {
			imports := w.starts(&queue{}, newJobTree(nil), 20).imports
			if err := imports.Check(as(w.bob), app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}); err != nil {
				panic(err)
			}
		}, domain.ErrWriting},
		{"the queue full", func(w *world, _ *queue, _ *jobTree) {
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindExport, State: domain.StateRunning, CreatedBy: w.bob})
			w.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Kind: domain.KindImport, State: domain.StateQueued, CreatedBy: w.bob})
		}, domain.ErrQueueFull},
		{"a reader as the row is written", func(w *world, _ *queue, _ *jobTree) {
			w.auth.roles[w.eng][w.bob] = shared.NotebookReader
		}, shared.Forbidden()},
		{"a workspace deleted as it locks", func(w *world, _ *queue, _ *jobTree) { w.workspaces.gone = true }, domain.ErrNotebookNotFound},
		{"a notebook deleted as it locks", func(w *world, _ *queue, _ *jobTree) { w.notebooks.unshared = true }, domain.ErrNotebookNotFound},
		{"River refusing", func(_ *world, q *queue, _ *jobTree) { q.err = enqueueErr }, enqueueErr},
	} {
		for _, kind := range []domain.Kind{domain.KindTrashRestore, domain.KindRevert} {
			t.Run(tt.name+", "+string(kind), func(t *testing.T) {
				w := newWorld()
				q, tree := &queue{}, newJobTree(w.rec)
				tt.setUp(w, q, tree)
				s := w.starts(q, tree, 2)
				if _, err := start(w, s, kind); !errors.Is(err, tt.want) {
					t.Errorf("Execute() = %v, want %v", err, tt.want)
				}
				if len(q.enqueued) != 0 {
					t.Errorf("enqueued %v", q.enqueued)
				}
				// An import's Check claims the notebook before it counts the
				// queue: only a claim left makes it transfer.busy. The fake
				// transaction keeps the row River refused, which would too.
				w.auth.roles[w.eng][w.bob] = shared.NotebookAdmin
				if !errors.Is(tt.want, domain.ErrWriting) && !errors.Is(tt.want, enqueueErr) {
					if err := s.imports.Check(as(w.bob), app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Size: -1}); errors.Is(err, domain.ErrWriting) {
						t.Errorf("an import's Check after it = %v, want the notebook released", err)
					}
				}
			})
		}
	}
	// A revert's refusal after its admit, its locks', releases the claim.
	w := newWorld()
	q, tree := &queue{}, newJobTree(w.rec)
	tree.late = locked
	s := w.starts(q, tree, 2)
	if _, err := start(w, s, domain.KindRevert); !errors.Is(err, locked) || len(q.enqueued) != 0 {
		t.Errorf("Execute() = %v, enqueued %v; want page.locked", err, q.enqueued)
	}
	if err := s.imports.Check(as(w.bob), app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Size: -1}); err != nil {
		t.Errorf("an import's Check after it = %v, want the notebook released", err)
	}
}

// start starts a background job of kind in eng, as bob.
func start(w *world, s starts, kind domain.Kind) (app.JobView, error) {
	if kind == domain.KindRevert {
		return s.revert.Execute(as(w.bob), app.RevertRequest{NotebookID: w.eng, ChangesetID: uuid.NewV7(), Client: domain.ClientWeb})
	}
	return s.restore.Execute(as(w.bob), app.RestoreRequest{NotebookID: w.eng, NodeID: uuid.NewV7(), Client: domain.ClientWeb})
}

// A start holds its notebook from its check's end, a revert's admit, until
// its row is written: an import's upload, or another start, is
// transfer.busy meanwhile, and after it the job's row is; another
// notebook's is not.
func TestABackgroundStartHoldsItsNotebook(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindTrashRestore, domain.KindRevert} {
		t.Run(string(kind), func(t *testing.T) {
			w := newWorld()
			ops := uuid.NewV7()
			w.auth.roles[ops] = map[uuid.UUID]shared.NotebookRole{w.bob: shared.NotebookAdmin}
			w.notebooks.names[ops] = "Ops"
			q, tree := &queue{}, newJobTree(w.rec)
			s := w.starts(q, tree, 20)
			upload := app.ImportRequest{NotebookID: w.eng, FileName: "v.zip", Client: domain.ClientWeb, Size: -1}
			other := upload
			other.NotebookID = ops
			var meanwhile, another, elsewhere error
			hold := func() {
				meanwhile = s.imports.Check(as(w.bob), upload)
				_, another = start(w, s, kind)
				elsewhere = s.imports.Check(as(w.bob), other)
				s.imports.Release(other)
			}
			if kind == domain.KindRevert {
				tree.onAdmit = hold
			} else {
				w.rows.onCount = hold
			}
			if _, err := start(w, s, kind); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(meanwhile, domain.ErrWriting) || !errors.Is(another, domain.ErrWriting) || elsewhere != nil {
				t.Errorf("as the start held eng: an upload %v, another start %v, an upload into another %v; want transfer.busy twice, then none",
					meanwhile, another, elsewhere)
			}
			if err := s.imports.Check(as(w.bob), upload); !errors.Is(err, domain.ErrWriting) {
				t.Errorf("an upload after the start = %v, want transfer.busy, of the job's row", err)
			}
			if len(q.enqueued) != 1 {
				t.Errorf("enqueued %v, want the first start's", q.enqueued)
			}
		})
	}
}
