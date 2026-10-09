package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is the fixed clock's time.
func now() time.Time { return time.Date(2026, 10, 9, 10, 30, 0, 0, time.UTC) }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func quiet() *slog.Logger { return slog.New(slog.DiscardHandler) }

// logs keeps a logger's text.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *logs) logger() *slog.Logger { return slog.New(slog.NewTextHandler(l, nil)) }

// recorder records the calls of the fakes that share it, in their order;
// a nil one records nothing.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(call string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

// of is the calls among names, in their order.
func (r *recorder) of(names ...string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, c := range r.calls {
		if slices.Contains(names, c) {
			out = append(out, c)
		}
	}
	return out
}

// direct runs fn in no transaction: the fakes keep no state a rollback
// would undo.
type direct struct {
	snapshots int
	rec       *recorder
}

func (d *direct) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	d.rec.add("WithinTx")
	return fn(ctx)
}

func (d *direct) WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error {
	d.rec.add("WithinSnapshot")
	d.snapshots++
	return fn(ctx)
}

// auth decides by roles: a user's role in a notebook, none not visible.
type auth struct {
	mu    sync.Mutex
	roles map[uuid.UUID]map[uuid.UUID]shared.NotebookRole
	asked []shared.Actor
	rec   *recorder
}

func (a *auth) Authorize(_ context.Context, actor shared.Actor, _ shared.Action, t shared.Target) (shared.Grant, error) {
	a.rec.add("Authorize")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asked = append(a.asked, actor)
	role, ok := a.roles[t.NotebookID][actor.UserID]
	if !ok {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: role}, nil
}

// workspaces lock every workspace but gone.
type workspaces struct {
	gone bool
	rec  *recorder
}

func (w workspaces) ShareByID(context.Context, uuid.UUID) (bool, error) {
	w.rec.add("Workspaces.ShareByID")
	return !w.gone, nil
}

// notebooks are the notebooks by id: each its workspace and name. One
// unshared is deleted as it locks.
type notebooks struct {
	names     map[uuid.UUID]string
	workspace uuid.UUID
	unshared  bool
	rec       *recorder
}

func (n notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	n.rec.add("Notebooks.WorkspaceOf")
	_, ok := n.names[id]
	return n.workspace, ok, nil
}

func (n notebooks) ShareByID(_ context.Context, id uuid.UUID) (bool, error) {
	n.rec.add("Notebooks.ShareByID")
	_, ok := n.names[id]
	return ok && !n.unshared, nil
}

func (n notebooks) NameOf(_ context.Context, id uuid.UUID) (string, bool, error) {
	n.rec.add("Notebooks.NameOf")
	name, ok := n.names[id]
	return name, ok, nil
}

type names map[uuid.UUID]string

func (n names) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if name, ok := n[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

// nodes is a notebook's tree: its nodes, and its pages' contents.
type nodes struct {
	all      []domain.Node
	contents map[uuid.UUID]string
	reads    int
}

func (n *nodes) Page(_ context.Context, _ uuid.UUID, id uuid.UUID) (string, bool, error) {
	for _, x := range n.all {
		if x.ID == id {
			return x.Name, !x.Asset, nil
		}
	}
	return "", false, nil
}

func (n *nodes) Scope(_ context.Context, _ uuid.UUID, root *uuid.UUID) ([]domain.Node, error) {
	if root == nil {
		return n.all, nil
	}
	in := map[uuid.UUID]bool{*root: true}
	var out []domain.Node
	for changed := true; changed; {
		changed = false
		for _, x := range n.all {
			if !in[x.ID] && x.ParentID != nil && in[*x.ParentID] {
				in[x.ID], changed = true, true
			}
		}
	}
	for _, x := range n.all {
		if in[x.ID] {
			if x.ID == *root && x.Asset {
				return nil, nil
			}
			out = append(out, x)
		}
	}
	return out, nil
}

func (n *nodes) Contents(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	n.reads++
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if c, ok := n.contents[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

// linked leads the links of any source to the pages ids.
type linked struct {
	ids     []uuid.UUID
	targets []uuid.UUID
}

func (l *linked) Linked(_ context.Context, _, targets []uuid.UUID) ([]uuid.UUID, error) {
	l.targets = targets
	var out []uuid.UUID
	for _, t := range targets {
		if slices.Contains(l.ids, t) {
			out = append(out, t)
		}
	}
	return out, nil
}

// blobWritten is when the blobs were written.
func blobWritten() time.Time { return time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC) }

// blobs are the attachments' blobs, written at blobWritten, and the blobs'
// files: open, when set, is called as a file opens, before its reader.
type blobs struct {
	of    map[uuid.UUID]uuid.UUID
	files map[uuid.UUID][]byte
	open  func(ctx context.Context) io.Reader
}

func (b *blobs) Of(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.Blob, error) {
	out := map[uuid.UUID]app.Blob{}
	for _, id := range ids {
		if blob, ok := b.of[id]; ok {
			out[id] = app.Blob{ID: blob, Created: blobWritten()}
		}
	}
	return out, nil
}

func (b *blobs) Open(ctx context.Context, blob uuid.UUID) (io.ReadCloser, error) {
	data, ok := b.files[blob]
	if !ok {
		return nil, app.ErrFileMissing
	}
	if b.open != nil {
		if r := b.open(ctx); r != nil {
			return io.NopCloser(r), nil
		}
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// entry is a file of an archive: a folder's has no data.
type entry struct {
	path     string
	modified time.Time
	stored   bool
	folder   bool
	data     string
}

// archives keeps the archives in memory: committed by job, the jobs
// created, aborted and deleted. full refuses a Create; fullAt fails the
// Add of that path; commitErr fails a Commit; onAdd runs as a file is
// added, onCommit as one commits, onOpen as one opens.
type archives struct {
	rec       *recorder
	mu        sync.Mutex
	committed map[uuid.UUID][]entry
	created   []uuid.UUID
	aborted   []uuid.UUID
	deleted   []uuid.UUID
	full      bool
	fullAt    string
	commitErr error
	free      int64
	deleteErr error
	listed    []uuid.UUID
	onAdd     func(path string)
	onCommit  func()
	onOpen    func(id uuid.UUID)
}

func newArchives() *archives { return &archives{committed: map[uuid.UUID][]entry{}, free: 1 << 40} }

func (a *archives) Create(_ context.Context, id uuid.UUID) (app.Archive, error) {
	if a.full {
		return nil, domain.ErrStorageFull
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.created = append(a.created, id)
	return &archive{a: a, id: id}, nil
}

func (a *archives) Open(_ context.Context, id uuid.UUID) (app.ArchiveFile, error) {
	if a.onOpen != nil {
		a.onOpen(id)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.committed[id]; !ok {
		return nil, app.ErrFileMissing
	}
	return file{Reader: bytes.NewReader(nil)}, nil
}

func (a *archives) Delete(_ context.Context, _ domain.Kind, id uuid.UUID) error {
	if a.deleteErr != nil {
		return a.deleteErr
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deleted = append(a.deleted, id)
	delete(a.committed, id)
	return nil
}

func (a *archives) List(_ context.Context, _ domain.Kind, _ time.Time, each func(uuid.UUID) error) error {
	for _, id := range a.listed {
		if err := each(id); err != nil {
			return err
		}
	}
	return nil
}

func (a *archives) Free(context.Context) (int64, error) {
	a.rec.add("Free")
	return a.free, nil
}

// entries are the committed archive of job id, by path.
func (a *archives) entries(id uuid.UUID) map[string]entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]entry{}
	for _, e := range a.committed[id] {
		out[e.path] = e
	}
	return out
}

func (a *archives) paths(id uuid.UUID) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, e := range a.committed[id] {
		out = append(out, e.path)
	}
	return out
}

type archive struct {
	a       *archives
	id      uuid.UUID
	entries []entry
}

func (x *archive) Add(path string, modified time.Time, stored bool, r io.Reader) error {
	if x.a.fullAt == path {
		return domain.ErrStorageFull
	}
	e := entry{path: path, modified: modified, stored: stored, folder: r == nil}
	if r != nil {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		e.data = string(data)
	}
	x.entries = append(x.entries, e)
	if x.a.onAdd != nil {
		x.a.onAdd(path)
	}
	return nil
}

func (x *archive) Commit() (int64, error) {
	if x.a.onCommit != nil {
		x.a.onCommit()
	}
	if x.a.commitErr != nil {
		return 0, x.a.commitErr
	}
	x.a.mu.Lock()
	defer x.a.mu.Unlock()
	x.a.committed[x.id] = x.entries
	return int64(len(x.entries)) * 100, nil
}

func (x *archive) Abort() error {
	x.a.mu.Lock()
	defer x.a.mu.Unlock()
	x.a.aborted = append(x.a.aborted, x.id)
	return nil
}

type file struct{ *bytes.Reader }

func (file) Close() error                         { return nil }
func (file) ModTime() time.Time                   { return now() }
func (f file) Size() int64                        { return f.Reader.Size() }
func (f file) Seek(o int64, w int) (int64, error) { return f.Reader.Seek(o, w) }

// queue records the jobs enqueued, or refuses with err.
type queue struct {
	enqueued []uuid.UUID
	err      error
	rec      *recorder
}

func (q *queue) Export(_ context.Context, id uuid.UUID) error {
	q.rec.add("Queue.Export")
	if q.err != nil {
		return q.err
	}
	q.enqueued = append(q.enqueued, id)
	return nil
}

// signer signs as adapter/mac does, an address's expiry part of its
// signature: the end of the hour after, or until when that comes first.
type signer struct{}

func (signer) Sign(now time.Time, id uuid.UUID, until time.Time) app.Signed {
	e := min(now.Truncate(time.Hour).Add(2*time.Hour).Unix(), until.Unix())
	return app.Signed{Expires: time.Unix(e, 0).UTC(), Signature: signature(id, e)}
}

func (signer) Valid(now time.Time, id uuid.UUID, e int64, sig string) bool {
	return e > now.Unix() && sig == signature(id, e)
}

// signature is the fake signer's signature of the job id's address,
// expiring at e.
func signature(id uuid.UUID, e int64) string {
	return fmt.Sprintf("sig-%s-%d", id, e)
}

// row is a job's row, deleted or not.
type row struct {
	job     domain.Job
	deleted bool
}

// rows keeps the jobs in memory, moving them as the statements do. beats
// counts the heartbeats, with the progress each wrote; failBeats fails as
// many heartbeats first; finishErr fails a FinishJob, and finishLeft is the
// time its context left it; onBeat runs at each heartbeat written.
type rows struct {
	rec        *recorder
	mu         sync.Mutex
	jobs       map[uuid.UUID]*row
	order      []uuid.UUID
	beats      []domain.Progress
	failBeats  int
	finishErr  error
	finishLeft time.Duration
	onBeat     func()
	finds      int
	locks      int
}

func newRows() *rows { return &rows{jobs: map[uuid.UUID]*row{}} }

func (r *rows) add(j domain.Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[j.ID] = &row{job: j}
	r.order = append(r.order, j.ID)
}

func (r *rows) get(id uuid.UUID) domain.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id].job
}

func (r *rows) set(id uuid.UUID, f func(*row)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f(r.jobs[id])
}

func (r *rows) CreateJob(_ context.Context, j domain.Job) error {
	r.rec.add("CreateJob")
	r.add(j)
	return nil
}

func (r *rows) LockQueue(context.Context) error {
	r.rec.add("LockQueue")
	r.locks++
	return nil
}

func (r *rows) CountActive(context.Context) (int, error) {
	r.rec.add("CountActive")
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.jobs {
		if !x.deleted && !x.job.State.Ended() {
			n++
		}
	}
	return n, nil
}

func (r *rows) Exporting(_ context.Context, notebookID, userID uuid.UUID) (bool, error) {
	r.rec.add("Exporting")
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.jobs {
		if !x.deleted && x.job.NotebookID == notebookID && x.job.CreatedBy == userID && x.job.Kind == domain.KindExport && !x.job.State.Ended() {
			return true, nil
		}
	}
	return false, nil
}

func (r *rows) FindJob(_ context.Context, id uuid.UUID) (domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finds++
	x, ok := r.jobs[id]
	if !ok || x.deleted {
		return domain.Job{}, app.ErrNoRow
	}
	return x.job, nil
}

func (r *rows) LockJob(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	return r.FindJob(ctx, id)
}

func (r *rows) ListJobs(_ context.Context, notebookID uuid.UUID, by *uuid.UUID, after *app.Cursor, limit int) ([]domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Job
	for i := len(r.order) - 1; i >= 0; i-- {
		x := r.jobs[r.order[i]]
		if x.deleted || x.job.NotebookID != notebookID || by != nil && x.job.CreatedBy != *by {
			continue
		}
		if after != nil && (x.job.CreatedAt.After(after.CreatedAt) || x.job.CreatedAt.Equal(after.CreatedAt) && x.job.ID.Compare(after.ID) >= 0) {
			continue
		}
		out = append(out, x.job)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (r *rows) StartJob(_ context.Context, id uuid.UUID, at time.Time) (domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.jobs[id]
	if !ok || x.deleted || x.job.State != domain.StateQueued {
		return domain.Job{}, app.ErrNoRow
	}
	x.job.State, x.job.Started, x.job.Heartbeat = domain.StateRunning, &at, &at
	return x.job, nil
}

func (r *rows) BeatJob(_ context.Context, id uuid.UUID, at time.Time, p domain.Progress) (app.Beat, error) {
	r.mu.Lock()
	if r.failBeats > 0 {
		r.failBeats--
		r.mu.Unlock()
		return app.Beat{}, errors.New("the database is gone")
	}
	x := r.jobs[id]
	if x.job.State != domain.StateRunning {
		r.mu.Unlock()
		return app.Beat{}, app.ErrNoRow
	}
	x.job.Heartbeat, x.job.Progress = &at, p
	r.beats = append(r.beats, p)
	b := app.Beat{CancelRequested: x.job.CancelRequested != nil, Deleted: x.deleted}
	onBeat := r.onBeat
	r.mu.Unlock()
	if onBeat != nil {
		onBeat()
	}
	return b, nil
}

func (r *rows) FinishJob(ctx context.Context, id uuid.UUID, e app.Ended) (bool, error) {
	r.rec.add("FinishJob")
	if r.finishErr != nil {
		return false, r.finishErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := ctx.Deadline(); ok {
		r.finishLeft = time.Until(d)
	}
	x := r.jobs[id]
	if x.deleted || x.job.State != domain.StateRunning {
		return false, nil
	}
	report := e.Report
	x.job.State, x.job.Finished, x.job.Report, x.job.ResultBytes, x.job.Name, x.job.Progress = e.State, &e.At, &report, e.ResultBytes, e.Name, e.Progress
	return true, nil
}

func (r *rows) ExpireOthers(_ context.Context, notebookID, userID, id uuid.UUID) ([]uuid.UUID, error) {
	r.rec.add("ExpireOthers")
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []uuid.UUID
	for _, other := range r.order {
		x := r.jobs[other]
		if other != id && !x.deleted && x.job.NotebookID == notebookID && x.job.CreatedBy == userID && x.job.Kind == domain.KindExport &&
			x.job.State == domain.StateSucceeded {
			x.job.State = domain.StateExpired
			out = append(out, other)
		}
	}
	return out, nil
}

func (r *rows) CancelQueued(_ context.Context, id uuid.UUID, at time.Time, report domain.Report) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := r.jobs[id]
	if x.job.State != domain.StateQueued {
		return false, nil
	}
	x.job.State, x.job.Finished, x.job.Report = domain.StateCancelled, &at, &report
	return true, nil
}

func (r *rows) RequestCancel(_ context.Context, id uuid.UUID, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := r.jobs[id]
	if x.job.State != domain.StateRunning {
		return false, nil
	}
	if x.job.CancelRequested == nil {
		x.job.CancelRequested = &at
	}
	return true, nil
}

// maintained are the maintained rows: what each was asked, and answers.
type maintained struct {
	rec         *recorder
	mu          sync.Mutex
	expire      [][]uuid.UUID
	expiredAt   []time.Time
	interrupted []app.Interrupted
	beatBefore  []*time.Time
	queued      []uuid.UUID
	failed      []uuid.UUID
	live        []uuid.UUID
	deletedOf   []uuid.UUID
	expiredJobs map[uuid.UUID]domain.Kind
	purged      []uuid.UUID
}

func (m *maintained) ExpireExports(_ context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expiredAt = append(m.expiredAt, before)
	if len(m.expire) == 0 {
		return nil, nil
	}
	next := m.expire[0]
	m.expire = m.expire[1:]
	if len(next) > batch {
		return nil, errors.New("more than the batch")
	}
	return next, nil
}

func (m *maintained) InterruptJobs(_ context.Context, beatBefore *time.Time, _ time.Time, r domain.Report) ([]app.Interrupted, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Failure != domain.FailureInterrupted {
		return nil, errors.New("the report is not an interruption's")
	}
	m.beatBefore = append(m.beatBefore, beatBefore)
	return m.interrupted, nil
}

func (m *maintained) QueuedExports(context.Context) ([]uuid.UUID, error) {
	m.rec.add("QueuedExports")
	return m.queued, nil
}

func (m *maintained) FailQueued(_ context.Context, ids []uuid.UUID, _ time.Time, r domain.Report) ([]app.Interrupted, error) {
	if r.Failure != domain.FailureInterrupted {
		return nil, errors.New("the report is not an interruption's")
	}
	m.failed = append(m.failed, ids...)
	out := make([]app.Interrupted, len(ids))
	for i, id := range ids {
		out[i] = app.Interrupted{ID: id}
	}
	return out, nil
}

// held are the jobs River holds, or err.
type held struct {
	ids []uuid.UUID
	err error
	rec *recorder
}

func (h held) Held(context.Context) ([]uuid.UUID, error) {
	h.rec.add("Held")
	return h.ids, h.err
}

func (m *maintained) LiveArchives(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if slices.Contains(m.live, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *maintained) DeleteJobsOfNotebooks(_ context.Context, ids []uuid.UUID, _ time.Time) error {
	m.deletedOf = append(m.deletedOf, ids...)
	return nil
}

func (m *maintained) ExpiredJobs(context.Context, time.Time, int) (map[uuid.UUID]domain.Kind, error) {
	return m.expiredJobs, nil
}

func (m *maintained) DeleteJobs(_ context.Context, ids []uuid.UUID) (int, error) {
	m.purged = append(m.purged, ids...)
	return len(ids), nil
}
