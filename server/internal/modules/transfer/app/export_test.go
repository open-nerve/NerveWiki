package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// world is an export's surroundings: eng, alice's to read, with its tree
//
//	Spec (content)          Spec.md
//	  Folder (empty)        Spec/Folder/        only children
//	    Deep (content)      Spec/Folder/Deep.md
//	  Linked (empty)        Spec/Linked.md      a link leads to it
//	    Child (empty)       Spec/Linked/Child.md
//	  x.png                 Spec/x.png
//	gone.pdf                gone.pdf            its file is missing
type world struct {
	eng, alice, bob                       uuid.UUID
	spec, folder, deep, linkd, child, png uuid.UUID
	gone                                  uuid.UUID
	tx                                    *direct
	auth                                  *auth
	workspaces                            workspaces
	notebooks                             notebooks
	nodes                                 *nodes
	linked                                *linked
	blobs                                 *blobs
	archives                              *archives
	rows                                  *rows
	contributors                          []app.Contributor
	rec                                   *recorder
	logger                                *slog.Logger
}

func newWorld() *world {
	rec := &recorder{}
	w := &world{eng: uuid.NewV7(), alice: uuid.NewV7(), bob: uuid.NewV7(), spec: uuid.NewV7(), folder: uuid.NewV7(), deep: uuid.NewV7(),
		linkd: uuid.NewV7(), child: uuid.NewV7(), png: uuid.NewV7(), gone: uuid.NewV7(), tx: &direct{rec: rec}, rec: rec, logger: quiet()}
	w.auth = &auth{roles: map[uuid.UUID]map[uuid.UUID]shared.NotebookRole{w.eng: {w.alice: shared.NotebookReader, w.bob: shared.NotebookAdmin}}, rec: rec}
	w.workspaces = workspaces{rec: rec}
	w.notebooks = notebooks{names: map[uuid.UUID]string{w.eng: "Eng"}, workspace: uuid.NewV7(), rec: rec}
	modified := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	node := func(id uuid.UUID, parent *uuid.UUID, asset bool, name string, order float64, empty bool) domain.Node {
		n := domain.Node{ID: id, ParentID: parent, Asset: asset, Name: name, SortOrder: order, Modified: modified}
		if !asset && !empty {
			n.Bytes = 4
		}
		return n
	}
	w.nodes = &nodes{
		all: []domain.Node{
			node(w.spec, nil, false, "Spec", 1, false),
			node(w.folder, &w.spec, false, "Folder", 1, true),
			node(w.deep, &w.folder, false, "Deep", 1, false),
			node(w.linkd, &w.spec, false, "Linked", 2, true),
			node(w.child, &w.linkd, false, "Child", 1, true),
			node(w.png, &w.spec, true, "x.png", 3, false),
			node(w.gone, nil, true, "gone.pdf", 2, false),
		},
		contents: map[uuid.UUID]string{w.spec: "# Spec\n[[Linked]]", w.deep: "deep", w.folder: "", w.linkd: "", w.child: ""},
	}
	w.linked = &linked{ids: []uuid.UUID{w.linkd}}
	pngBlob, goneBlob := uuid.NewV7(), uuid.NewV7()
	w.blobs = &blobs{of: map[uuid.UUID]uuid.UUID{w.png: pngBlob, w.gone: goneBlob}, files: map[uuid.UUID][]byte{pngBlob: []byte("PNG")}}
	w.archives = newArchives()
	w.archives.rec = rec
	w.rows = newRows()
	w.rows.rec = rec
	return w
}

func (w *world) export() *app.Export {
	return app.NewExport(app.ExportDeps{Tx: w.tx, Snapshots: w.tx, Authorizer: w.auth, Notebooks: w.notebooks, Nodes: w.nodes, Linked: w.linked,
		Blobs: w.blobs, Archives: w.archives, Rows: w.rows, Contributors: w.contributors, Clock: fixedClock{now()}, Logger: w.logger,
		Beat: 5 * time.Millisecond})
}

// queued adds alice's export of eng, of root when set, queued.
func (w *world) queued(root *uuid.UUID) domain.Job {
	j := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, RootID: root, Kind: domain.KindExport, State: domain.StateQueued, Name: "Eng",
		CreatedBy: w.alice, Client: domain.ClientWeb, CreatedAt: now()}
	w.rows.add(j)
	return j
}

// The whole notebook's archive: a root folder named after it, the vault
// in it as the plan maps it, each page's content as written, the
// attachments' files, meta.json; the attachment whose file is missing in
// the report, not the archive. The job succeeds with its counts, as the
// starter acting through the job.
func TestAnExportWritesTheNotebooksArchive(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)

	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}

	got := w.rows.get(j.ID)
	if got.State != domain.StateSucceeded || got.ResultBytes == nil || got.Name != "Eng" || got.Progress != (domain.Progress{Done: 7, Total: 7}) {
		t.Errorf("job = %+v, want succeeded with 7 of 7 nodes", got)
	}
	r := got.Report
	if r == nil || r.Failure != "" || r.Counts != (domain.Counts{Pages: 5, Attachments: 1, Missing: 1}) ||
		!slices.Equal(r.Problems, []domain.Problem{{Path: "gone.pdf", Code: domain.ProblemFileMissing}}) {
		t.Errorf("report = %+v", r)
	}
	want := []string{"Eng/Spec.md", "Eng/Spec/Folder/", "Eng/Spec/Folder/Deep.md", "Eng/Spec/Linked.md", "Eng/Spec/Linked/Child.md",
		"Eng/Spec/x.png", "Eng/.nerve/meta.json"}
	if paths := w.archives.paths(j.ID); !slices.Equal(paths, want) {
		t.Errorf("archive = %q, want %q", paths, want)
	}
	entries := w.archives.entries(j.ID)
	if e := entries["Eng/Spec.md"]; e.data != "# Spec\n[[Linked]]" || e.stored || !e.modified.Equal(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Spec.md = %+v", e)
	}
	if e := entries["Eng/Spec/x.png"]; e.data != "PNG" || !e.stored || !e.modified.Equal(blobWritten()) {
		t.Errorf("x.png = %+v, want its file stored, modified as its blob was written", e)
	}
	if e := entries["Eng/Spec/Folder/"]; !e.folder {
		t.Errorf("Folder/ = %+v, want a folder's entry", e)
	}
	var meta domain.Meta
	if err := json.Unmarshal([]byte(entries["Eng/.nerve/meta.json"].data), &meta); err != nil {
		t.Fatal(err)
	}
	var metaPaths []string
	for _, n := range meta.Nodes {
		metaPaths = append(metaPaths, n.Path)
	}
	if wantMeta := []string{"Spec.md", "Spec/Folder/", "Spec/Folder/Deep.md", "Spec/Linked.md", "Spec/Linked/Child.md", "Spec/x.png"}; !slices.Equal(metaPaths, wantMeta) ||
		meta.Notebook.Name != "Eng" || meta.Root != nil || !meta.ExportedAt.Equal(now()) {
		t.Errorf("meta = %+v (paths %q)", meta, metaPaths)
	}
	if !slices.Equal(w.linked.targets, []uuid.UUID{w.folder, w.linkd}) {
		t.Errorf("asked of the links %v, want the empty pages with children", w.linked.targets)
	}
	if w.tx.snapshots != 1 {
		t.Errorf("%d snapshots, want one", w.tx.snapshots)
	}
	if len(w.auth.asked) != 1 || w.auth.asked[0] != (shared.Actor{UserID: w.alice, JobID: j.ID}) {
		t.Errorf("decided for %+v, want alice through the job", w.auth.asked)
	}
}

// A subtree's archive is named after its page, which is at the vault's
// root; meta.json names it.
func TestAnExportOfASubtree(t *testing.T) {
	w := newWorld()
	j := w.queued(&w.folder)

	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}

	if got := w.rows.get(j.ID); got.State != domain.StateSucceeded || got.Name != "Folder" {
		t.Errorf("job = %+v, want succeeded, named Folder", got)
	}
	want := []string{"Folder/Folder/", "Folder/Folder/Deep.md", "Folder/.nerve/meta.json"}
	if paths := w.archives.paths(j.ID); !slices.Equal(paths, want) {
		t.Errorf("archive = %q, want %q", paths, want)
	}
	var meta domain.Meta
	_ = json.Unmarshal([]byte(w.archives.entries(j.ID)["Folder/.nerve/meta.json"].data), &meta)
	if meta.Root == nil || meta.Root.ID != w.folder || meta.Root.Name != "Folder" {
		t.Errorf("meta's root = %+v", meta.Root)
	}
}

// A success expires the starter's earlier export of the notebook, and
// deletes its archive; another's stays.
func TestASuccessKeepsTheLatestExportAlone(t *testing.T) {
	w := newWorld()
	ctx := context.Background()
	first := w.queued(nil)
	if err := w.export().Run(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	bobs := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindExport, State: domain.StateSucceeded, Name: "Eng", CreatedBy: w.bob, CreatedAt: now()}
	w.rows.add(bobs)
	second := w.queued(nil)
	if err := w.export().Run(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if w.rows.get(first.ID).State != domain.StateExpired || w.rows.get(bobs.ID).State != domain.StateSucceeded {
		t.Errorf("first %s, bob's %s; want expired, succeeded", w.rows.get(first.ID).State, w.rows.get(bobs.ID).State)
	}
	if !slices.Equal(w.archives.deleted, []uuid.UUID{first.ID}) {
		t.Errorf("deleted %v, want the first's archive", w.archives.deleted)
	}
}

// A job no longer running as it succeeds, the rescue having failed it,
// writes nothing: the starter's earlier export stays, its archive too; the
// job's archive is dropped, its stop logged.
func TestAJobRescuedAsItSucceedsDropsItsArchive(t *testing.T) {
	w := newWorld()
	ctx := context.Background()
	first := w.queued(nil)
	if err := w.export().Run(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	var l logs
	w.logger = l.logger()
	second := w.queued(nil)
	w.archives.onCommit = func() { w.rows.set(second.ID, func(r *row) { r.job.State = domain.StateFailed }) }

	if err := w.export().Run(ctx, second.ID); err != nil {
		t.Fatal(err)
	}

	if w.rows.get(first.ID).State != domain.StateSucceeded || w.rows.get(second.ID).State != domain.StateFailed ||
		!slices.Equal(w.archives.deleted, []uuid.UUID{second.ID}) || !strings.Contains(l.String(), "export stopped") {
		t.Errorf("first %s, second %s, deleted %v, logs %q; want the first kept, the second's archive dropped, its stop logged",
			w.rows.get(first.ID).State, w.rows.get(second.ID).State, w.archives.deleted, l.String())
	}
}

// A job no longer queued, cancelled or deleted before it ran, is left.
func TestAJobNoLongerQueuedIsLeft(t *testing.T) {
	w := newWorld()
	cancelled, deleted := w.queued(nil), w.queued(nil)
	w.rows.set(cancelled.ID, func(r *row) { r.job.State = domain.StateCancelled })
	w.rows.set(deleted.ID, func(r *row) { r.deleted = true })
	for _, id := range []uuid.UUID{cancelled.ID, deleted.ID} {
		if err := w.export().Run(context.Background(), id); err != nil {
			t.Error(err)
		}
	}
	if len(w.archives.created) != 0 || w.rows.get(cancelled.ID).State != domain.StateCancelled {
		t.Errorf("created %v, the cancelled one %s; want nothing done", w.archives.created, w.rows.get(cancelled.ID).State)
	}
}

// blocking makes the attachment's file read wait until the run's context
// ends, after started is called once it waits.
func blocking(w *world, started func()) {
	w.blobs.open = func(ctx context.Context) io.Reader {
		started()
		return readerFunc(func([]byte) (int, error) {
			<-ctx.Done()
			return 0, errors.New("read cut")
		})
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// A running job stops at its heartbeat once its cancel was asked: it ends
// cancelled, with what it did, its archive dropped; once deleted, it
// stops without a word, its archive dropped.
func TestARunningJobStops(t *testing.T) {
	for _, tt := range []struct {
		name string
		stop func(r *row)
		want domain.State
	}{
		{"cancelled", func(r *row) { at := now(); r.job.CancelRequested = &at }, domain.StateCancelled},
		{"deleted", func(r *row) { r.deleted = true }, domain.StateRunning},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			j := w.queued(nil)
			blocking(w, func() { w.rows.set(j.ID, tt.stop) })

			// The job's context would end it, timed out, were the heartbeat
			// not to stop it first.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := w.export().Run(ctx, j.ID); err != nil {
				t.Fatal(err)
			}

			got := w.rows.get(j.ID)
			if got.State != tt.want || len(w.archives.committed) != 0 || !slices.Equal(w.archives.aborted, []uuid.UUID{j.ID}) {
				t.Errorf("job %s, committed %v, aborted %v; want %s, the archive dropped", got.State, w.archives.committed, w.archives.aborted, tt.want)
			}
			if tt.want == domain.StateCancelled && (got.Report == nil || got.Report.Failure != "" || got.Report.Counts.Pages != 5 || got.Progress.Done != 5) {
				t.Errorf("report %+v, progress %+v; want the five pages done", got.Report, got.Progress)
			}
		})
	}
}

// River's context ends the job: its timeout fails it so, the end written
// in its own time; the server's stop as interrupted, the end written
// within River's grace. Either end counts what the job wrote.
func TestRiversContextEndsTheJob(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		want domain.Failure
		left func(time.Duration) bool
	}{
		{"timeout", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 50*time.Millisecond)
		}, domain.FailureTimeout, func(d time.Duration) bool { return d < 10*time.Second }},
		{"stop", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(50*time.Millisecond, cancel)
			return ctx, cancel
		}, domain.FailureInterrupted, func(d time.Duration) bool { return d > time.Second }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			j := w.queued(nil)
			blocking(w, func() {})
			ctx, cancel := tt.ctx()
			defer cancel()

			if err := w.export().Run(ctx, j.ID); err != nil {
				t.Fatal(err)
			}

			got := w.rows.get(j.ID)
			if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != tt.want || got.Report.Counts.Pages != 5 ||
				len(w.archives.aborted) != 1 {
				t.Errorf("job %s, report %+v, aborted %v; want failed %s, the five pages counted, dropped", got.State, got.Report, w.archives.aborted, tt.want)
			}
			if left := w.rows.finishLeft; tt.left(left) {
				t.Errorf("the end written with %v left", left)
			}
		})
	}
}

// A stop that comes as the archive's last file is written ends the job
// before its commit: it fails, its archive dropped, never committed.
func TestAStopAsTheArchiveEndsDropsIt(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.archives.onAdd = func(path string) {
		if strings.HasSuffix(path, "/"+domain.MetaPath) {
			cancel()
		}
	}

	if err := w.export().Run(ctx, j.ID); err != nil {
		t.Fatal(err)
	}

	got := w.rows.get(j.ID)
	if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != domain.FailureInterrupted || len(w.archives.committed) != 0 ||
		!slices.Equal(w.archives.aborted, []uuid.UUID{j.ID}) {
		t.Errorf("job %s, report %+v, committed %v, aborted %v; want failed interrupted, the archive dropped before its commit",
			got.State, got.Report, w.archives.committed, w.archives.aborted)
	}
}

// What fails an export, and how its report says it.
func TestAnExportFails(t *testing.T) {
	for _, tt := range []struct {
		name   string
		root   bool
		setUp  func(w *world)
		want   domain.Failure
		create bool
	}{
		{"no longer a reader", false, func(w *world) { delete(w.auth.roles[w.eng], w.alice) }, domain.FailureForbidden, false},
		{"the page gone", true, func(w *world) { w.nodes.all = w.nodes.all[1:] }, domain.FailureRootNotFound, true},
		{"no room for the archive", false, func(w *world) { w.archives.full = true }, domain.FailureStorageFull, false},
		{"no room for a file", false, func(w *world) { w.archives.fullAt = "Eng/Spec/x.png" }, domain.FailureStorageFull, true},
		{"no room to commit", false, func(w *world) { w.archives.commitErr = domain.ErrStorageFull }, domain.FailureStorageFull, true},
		{"a contributor's conflict", false, func(w *world) { w.contributors = []app.Contributor{adds{"Spec.md"}} }, domain.FailureContributorConflict, true},
		{"a contributor's error", false, func(w *world) { w.contributors = []app.Contributor{fails{}} }, domain.FailureInternal, true},
		{"two siblings of one key", false, func(w *world) { w.nodes.all[6].Name = "SPEC" }, domain.FailureInternal, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			var root *uuid.UUID
			if tt.root {
				root = &w.spec
			}
			j := w.queued(root)
			tt.setUp(w)

			if err := w.export().Run(context.Background(), j.ID); err != nil {
				t.Fatal(err)
			}

			got := w.rows.get(j.ID)
			if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != tt.want || len(w.archives.committed) != 0 {
				t.Errorf("job %s, report %+v; want failed %s, nothing committed", got.State, got.Report, tt.want)
			}
			if created := len(w.archives.created) > 0; created != tt.create || created && len(w.archives.deleted)+len(w.archives.aborted) == 0 {
				t.Errorf("created %v, aborted %v, deleted %v; want an archive %v, dropped", w.archives.created, w.archives.aborted, w.archives.deleted, tt.create)
			}
		})
	}
}

// adds is a contributor that adds its paths, each "contributed".
type adds []string

func (a adds) Contribute(_ context.Context, _ app.Scope, sink app.Sink) error {
	for _, p := range a {
		if err := sink.Add(p, []byte("contributed")); err != nil {
			return err
		}
	}
	return nil
}

type fails struct{}

func (fails) Contribute(context.Context, app.Scope, app.Sink) error {
	return errors.New("the contributor failed")
}

// scopes records the scope it is given.
type scopes struct{ got *app.Scope }

func (s scopes) Contribute(_ context.Context, scope app.Scope, _ app.Sink) error {
	*s.got = scope
	return nil
}

// The contributors run in their order in the snapshot, given the scope;
// their files go in the archive and in meta.json, the first error stops
// the rest.
func TestTheContributorsAddTheirFiles(t *testing.T) {
	w := newWorld()
	var scope app.Scope
	w.contributors = []app.Contributor{scopes{&scope}, adds{"index.md"}, adds{"log/2026.md"}}
	j := w.queued(nil)
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	entries := w.archives.entries(j.ID)
	if entries["Eng/index.md"].data != "contributed" || entries["Eng/log/2026.md"].data != "contributed" {
		t.Errorf("archive = %q, want the contributors' files", w.archives.paths(j.ID))
	}
	var meta domain.Meta
	_ = json.Unmarshal([]byte(entries["Eng/.nerve/meta.json"].data), &meta)
	if !slices.Equal(meta.Contributed, []string{"index.md", "log/2026.md"}) {
		t.Errorf("contributed = %q", meta.Contributed)
	}
	if scope.NotebookID != w.eng || scope.RootID != nil || len(scope.Nodes) != 7 || scope.Nodes[1] != (app.ScopeNode{ID: w.folder, Path: "Spec/Folder/"}) {
		t.Errorf("scope = %+v", scope)
	}

	w = newWorld()
	second := adds{"never.md"}
	w.contributors = []app.Contributor{fails{}, second}
	j = w.queued(nil)
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	if w.rows.get(j.ID).State != domain.StateFailed {
		t.Errorf("job %s, want failed at the first error", w.rows.get(j.ID).State)
	}
}

// beats calls onBeat at each heartbeat written, and answers a channel
// that tells them, the latest kept.
func beats(w *world) <-chan struct{} {
	beaten := make(chan struct{}, 1)
	w.rows.onBeat = func() {
		select {
		case beaten <- struct{}{}:
		default:
		}
	}
	return beaten
}

// A running job writes its heartbeat with its progress.
func TestARunningJobBeats(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	beaten := beats(w)
	pages := domain.Progress{Done: 5, Total: 7}
	w.blobs.open = func(context.Context) io.Reader {
		for limit := time.After(10 * time.Second); ; {
			select {
			case <-beaten:
			case <-limit:
				return nil
			}
			w.rows.mu.Lock()
			written := slices.Contains(w.rows.beats, pages)
			w.rows.mu.Unlock()
			if written {
				return nil
			}
		}
	}
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	w.rows.mu.Lock()
	written := slices.Clone(w.rows.beats)
	w.rows.mu.Unlock()
	if !slices.Contains(written, pages) {
		t.Errorf("beats %+v, want the progress of the five pages", written)
	}
}

// The heartbeat goes on until the archive is committed: a long commit
// leaves no heartbeat old.
func TestTheHeartbeatLastsThroughTheCommit(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	beaten := beats(w)
	beat := false
	w.archives.onCommit = func() {
		select {
		case <-beaten:
		default:
		}
		select {
		case <-beaten:
			beat = true
		case <-time.After(5 * time.Second):
		}
	}
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	if !beat || w.rows.get(j.ID).State != domain.StateSucceeded {
		t.Errorf("a heartbeat during the commit %v, the job %s; want one, succeeded", beat, w.rows.get(j.ID).State)
	}
}

// A heartbeat that fails is tried again at the next: the first failure of
// a run of them is logged, and the write that ends it.
func TestAHeartbeatNotWrittenIsLoggedOnce(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	j := w.queued(nil)
	w.rows.failBeats = 3
	beaten := beats(w)
	w.blobs.open = func(context.Context) io.Reader {
		<-beaten
		return nil
	}
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	text := l.String()
	if strings.Count(text, "export heartbeat not written") != 1 || strings.Count(text, "export heartbeat written again") != 1 ||
		w.rows.get(j.ID).State != domain.StateSucceeded {
		t.Errorf("logs %q, the job %s; want the failures once, the write again once, succeeded", text, w.rows.get(j.ID).State)
	}
}

// A heartbeat that, after failing, finds its job no longer running stops
// the job as gone: no write is logged as written again.
func TestAHeartbeatThatFindsItsJobGoneIsNoWrite(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	j := w.queued(nil)
	w.rows.failBeats = 1 << 30
	w.blobs.open = func(ctx context.Context) io.Reader {
		for limit := time.Now().Add(10 * time.Second); !strings.Contains(l.String(), "export heartbeat not written") && time.Now().Before(limit); {
			time.Sleep(time.Millisecond)
		}
		w.rows.set(j.ID, func(r *row) { r.job.State = domain.StateFailed })
		w.rows.mu.Lock()
		w.rows.failBeats = 0
		w.rows.mu.Unlock()
		<-ctx.Done()
		return nil
	}
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	text := l.String()
	if strings.Count(text, "export heartbeat not written") != 1 || strings.Contains(text, "export heartbeat written again") ||
		!strings.Contains(text, "export stopped") || w.rows.get(j.ID).State != domain.StateFailed {
		t.Errorf("logs %q, the job %s; want the failure once, no write again, the stop", text, w.rows.get(j.ID).State)
	}
}

// The pages' contents are read 200 pages or 16 MiB at a time, whichever
// comes first, a larger page alone; each page as written.
func TestThePagesAreReadInBatches(t *testing.T) {
	ones := func(n int) []int64 { return slices.Repeat([]int64{1}, n) }
	for _, tt := range []struct {
		name  string
		bytes []int64
		reads int
	}{
		{"200 pages", ones(200), 1},
		{"201 pages", ones(201), 2},
		{"16 MiB", []int64{8 << 20, 8 << 20}, 1},
		{"past 16 MiB", []int64{8 << 20, 8 << 20, 1}, 2},
		{"a page past it alone", []int64{1, 20 << 20, 1}, 3},
		{"one page past it", []int64{20 << 20}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			w.nodes.all, w.nodes.contents = nil, map[uuid.UUID]string{}
			for i, b := range tt.bytes {
				id := uuid.NewV7()
				w.nodes.all = append(w.nodes.all, domain.Node{ID: id, Name: fmt.Sprintf("p%03d", i), SortOrder: float64(i), Bytes: b})
				w.nodes.contents[id] = fmt.Sprintf("page %d", i)
			}
			j := w.queued(nil)
			if err := w.export().Run(context.Background(), j.ID); err != nil {
				t.Fatal(err)
			}
			got := w.rows.get(j.ID)
			if w.nodes.reads != tt.reads || got.Report == nil || got.Report.Counts.Pages != int64(len(tt.bytes)) {
				t.Errorf("%d reads, report %+v; want %d, every page", w.nodes.reads, got.Report, tt.reads)
			}
			entries := w.archives.entries(j.ID)
			for i := range tt.bytes {
				if e := entries[fmt.Sprintf("Eng/p%03d.md", i)]; e.data != fmt.Sprintf("page %d", i) {
					t.Errorf("p%03d.md = %+v", i, e)
				}
			}
		})
	}
}

// A report counts what was written: an export that fails midway counts
// the pages before, as its progress does.
func TestAReportCountsWhatWasWritten(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	w.archives.fullAt = "Eng/Spec/Linked.md"
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	got := w.rows.get(j.ID)
	if got.Report == nil || got.Report.Counts != (domain.Counts{Pages: 3}) || got.Progress.Done != 3 {
		t.Errorf("report %+v, progress %+v; want the three pages before Linked.md", got.Report, got.Progress)
	}
}

// A success locks the notebook's row before the job's, as a deletion of
// the notebook does: neither waits for a row the other holds. A notebook
// deleted meanwhile took the job's row with it: nothing is written, the
// archive is dropped.
func TestASuccessLocksTheNotebookFirst(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{"WithinTx", "Notebooks.ShareByID", "FinishJob", "ExpireOthers"}
	if got := w.rec.of(want...); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}

	w = newWorld()
	var l logs
	w.logger = l.logger()
	j = w.queued(nil)
	w.notebooks.unshared = true
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	if got := w.rows.get(j.ID); got.State != domain.StateRunning || slices.Contains(w.rec.calls, "FinishJob") || len(w.archives.committed) != 0 ||
		!slices.Equal(w.archives.deleted, []uuid.UUID{j.ID}) || !strings.Contains(l.String(), "export stopped") {
		t.Errorf("job %s, calls %v, deleted %v, logs %q; want nothing written, the archive dropped, the stop logged", got.State, w.rec.calls,
			w.archives.deleted, l.String())
	}
}

// A job whose end cannot be written fails: the rescue fails it later.
func TestAnEndNotWrittenFails(t *testing.T) {
	w := newWorld()
	j := w.queued(nil)
	w.rows.finishErr = errors.New("the database is gone")
	if err := w.export().Run(context.Background(), j.ID); err == nil || !strings.Contains(err.Error(), "the database is gone") {
		t.Errorf("Run() = %v, want the write's failure", err)
	}
}

// The jobs' logs tell ids, the client, numbers and codes; never a
// notebook's, a page's or a file's name, a path of the vault, nor a
// signature (v0.1 design 13.1, item 10).
func TestTheJobsLogTheIDsNotTheNames(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	ctx := context.Background()
	v, err := w.start(&queue{}, 20).Run(as(w.alice), w.eng, &w.spec, domain.ClientAPI)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.export().Run(ctx, v.Job.ID); err != nil {
		t.Fatal(err)
	}
	refused := w.queued(nil)
	delete(w.auth.roles[w.eng], w.bob)
	w.rows.set(refused.ID, func(r *row) { r.job.CreatedBy = w.bob })
	if err := w.export().Run(ctx, refused.ID); err != nil {
		t.Fatal(err)
	}
	running := w.job(w.alice, now(), domain.StateRunning)
	if _, err := w.cancel().Run(as(w.alice), running.ID); err != nil {
		t.Fatal(err)
	}

	text := l.String()
	for _, want := range []string{"export queued", "export started", "export ended", "job cancel asked", v.Job.ID.String(), refused.ID.String(),
		running.ID.String(), w.eng.String(), w.alice.String(), w.bob.String(), "client=api", "state=succeeded", "failure=forbidden"} {
		if !strings.Contains(text, want) {
			t.Errorf("the logs lack %q:\n%s", want, text)
		}
	}
	for _, never := range []string{"Eng", "Spec", "Folder", "Linked", "x.png", "gone.pdf", "sig-"} {
		if strings.Contains(text, never) {
			t.Errorf("the logs tell %q:\n%s", never, text)
		}
	}
}

// contributorFunc is a contributor of a function.
type contributorFunc func(ctx context.Context, scope app.Scope, sink app.Sink) error

func (f contributorFunc) Contribute(ctx context.Context, scope app.Scope, sink app.Sink) error {
	return f(ctx, scope, sink)
}

// A job deleted as it fails has no end written: its stop is logged, no
// end. Its heartbeats fail, so that none reads the deletion first.
func TestAJobDeletedAsItFailsLogsItsStop(t *testing.T) {
	w := newWorld()
	var l logs
	w.logger = l.logger()
	j := w.queued(nil)
	w.rows.failBeats = 1 << 30
	w.contributors = []app.Contributor{contributorFunc(func(context.Context, app.Scope, app.Sink) error {
		w.rows.set(j.ID, func(r *row) { r.deleted = true })
		return errors.New("the contributor failed")
	})}
	if err := w.export().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	if got, text := w.rows.get(j.ID), l.String(); got.State != domain.StateRunning || !strings.Contains(text, "export stopped") ||
		strings.Contains(text, "export ended") {
		t.Errorf("job %s, logs %q; want no end written, the stop logged", got.State, text)
	}
}
