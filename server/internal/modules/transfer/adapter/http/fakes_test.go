package httpadapter_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeAuth takes "session" for a sign-in session's access token and "pat"
// for a personal access token, both alice's, and "bob" for bob's session.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	actor := shared.Actor{UserID: alice()}
	switch token {
	case "session":
		actor.SessionID = uuid.NewV7()
	case "pat":
		actor.APITokenID = uuid.NewV7()
	case "bob":
		actor.UserID, actor.SessionID = bob(), uuid.NewV7()
	default:
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, actor), token, nil
}

func alice() uuid.UUID       { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000001") }
func bob() uuid.UUID         { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000002") }
func workspaceID() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000009") }
func notebookID() uuid.UUID  { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000010") }
func pageID() uuid.UUID      { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000011") }

func now() time.Time { return time.Date(2026, 10, 9, 10, 30, 0, 0, time.UTC) }

// exportTTL is transfer.export_ttl in the tests.
const exportTTL = 24 * time.Hour

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now() }

// direct runs fn in no transaction: the fakes keep no state a rollback
// would undo.
type direct struct{}

func (direct) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// roles decides by each account's role in notebookID(): alice reads it,
// bob is its admin; the rest do not see it.
type roles struct{}

func (roles) Authorize(_ context.Context, actor shared.Actor, _ shared.Action, t shared.Target) (shared.Grant, error) {
	role, ok := map[uuid.UUID]shared.NotebookRole{alice(): shared.NotebookReader, bob(): shared.NotebookAdmin}[actor.UserID]
	if !ok || t.NotebookID != notebookID() {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: role}, nil
}

type workspaces struct{}

func (workspaces) ShareByID(context.Context, uuid.UUID) (bool, error) { return true, nil }

// notebooks has notebookID() alone, named Eng.
type notebooks struct{}

func (notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return workspaceID(), id == notebookID(), nil
}

func (notebooks) ShareByID(_ context.Context, id uuid.UUID) (bool, error) {
	return id == notebookID(), nil
}

func (notebooks) NameOf(_ context.Context, id uuid.UUID) (string, bool, error) {
	return "Eng", id == notebookID(), nil
}

type names struct{}

func (names) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = map[uuid.UUID]string{alice(): "Alice", bob(): "Bob"}[id]
	}
	return out, nil
}

// nodes has the page pageID(), named Spec, in notebookID(); the start
// reads its name alone.
type nodes struct{}

func (nodes) Page(_ context.Context, notebook, id uuid.UUID) (string, bool, error) {
	return "Spec", notebook == notebookID() && id == pageID(), nil
}

func (nodes) Scope(context.Context, uuid.UUID, *uuid.UUID) ([]domain.Node, error) { return nil, nil }

func (nodes) Contents(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) { return nil, nil }

// rows keeps the jobs in memory, as the statements move them; FindJob
// waits on gate when it is set, once it said so on asked.
type rows struct {
	mu    sync.Mutex
	jobs  map[uuid.UUID]*domain.Job
	order []uuid.UUID
	gate  chan struct{}
	asked chan struct{}
}

func (r *rows) add(j domain.Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[j.ID] = &j
	r.order = append(r.order, j.ID)
}

// get is the job id as it is now.
func (r *rows) get(id uuid.UUID) domain.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return *r.jobs[id]
}

// clear drops every job.
func (r *rows) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs, r.order = map[uuid.UUID]*domain.Job{}, nil
}

func (r *rows) CreateJob(_ context.Context, j domain.Job) error {
	r.add(j)
	return nil
}

func (r *rows) LockQueue(context.Context) error { return nil }

func (r *rows) CountActive(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, j := range r.jobs {
		if !j.State.Ended() {
			n++
		}
	}
	return n, nil
}

func (r *rows) Exporting(_ context.Context, notebook, user uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		if j.NotebookID == notebook && j.CreatedBy == user && j.Kind == domain.KindExport && !j.State.Ended() {
			return true, nil
		}
	}
	return false, nil
}

func (r *rows) FindJob(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	if r.gate != nil {
		r.asked <- struct{}{}
		select {
		case <-r.gate:
		case <-ctx.Done():
			return domain.Job{}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return domain.Job{}, app.ErrNoRow
	}
	return *j, nil
}

func (r *rows) LockJob(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	return r.FindJob(ctx, id)
}

func (r *rows) ListJobs(_ context.Context, notebook uuid.UUID, by *uuid.UUID, after *app.Cursor, limit int) ([]domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Job
	for i := len(r.order) - 1; i >= 0 && len(out) < limit; i-- {
		j := r.jobs[r.order[i]]
		if j.NotebookID != notebook || by != nil && j.CreatedBy != *by ||
			after != nil && (j.CreatedAt.After(after.CreatedAt) || j.CreatedAt.Equal(after.CreatedAt) && j.ID.Compare(after.ID) >= 0) {
			continue
		}
		out = append(out, *j)
	}
	return out, nil
}

func (r *rows) StartJob(context.Context, uuid.UUID, time.Time) (domain.Job, error) {
	panic("rows: no job runs here")
}

func (r *rows) BeatJob(context.Context, uuid.UUID, time.Time, domain.Progress) (app.Beat, error) {
	panic("rows: no job runs here")
}

func (r *rows) FinishJob(context.Context, uuid.UUID, app.Ended) (bool, error) {
	panic("rows: no job runs here")
}

func (r *rows) ExpireOthers(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	panic("rows: no job runs here")
}

func (r *rows) CancelQueued(_ context.Context, id uuid.UUID, at time.Time, report domain.Report) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[id]
	if j.State != domain.StateQueued {
		return false, nil
	}
	j.State, j.Finished, j.Report = domain.StateCancelled, &at, &report
	return true, nil
}

func (r *rows) RequestCancel(_ context.Context, id uuid.UUID, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[id]
	if j.State != domain.StateRunning {
		return false, nil
	}
	if j.CancelRequested == nil {
		j.CancelRequested = &at
	}
	return true, nil
}

// archives keeps the exports' archives in memory: free is the store's
// room; each opened is counted until it closes.
type archives struct {
	mu    sync.Mutex
	files map[uuid.UUID][]byte
	free  int64
	open  int
}

func (a *archives) Create(context.Context, uuid.UUID) (app.Archive, error) {
	panic("archives: no export runs here")
}

func (a *archives) Open(_ context.Context, id uuid.UUID) (app.ArchiveFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, ok := a.files[id]
	if !ok {
		return nil, app.ErrFileMissing
	}
	a.open++
	return &file{Reader: bytes.NewReader(data), archives: a}, nil
}

func (a *archives) Delete(context.Context, domain.Kind, uuid.UUID) error { return nil }

func (a *archives) List(context.Context, domain.Kind, time.Time, func(uuid.UUID) error) error {
	return nil
}

func (a *archives) Free(context.Context) (int64, error) { return a.free, nil }

func (a *archives) Upload(context.Context, uuid.UUID) (app.Upload, error) {
	return nil, errors.New("no import's upload in the exports' tests")
}

func (a *archives) OpenImport(context.Context, uuid.UUID, int) (app.ImportArchive, error) {
	return nil, errors.New("no import's archive in the exports' tests")
}

func (a *archives) unclosed() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.open
}

type file struct {
	*bytes.Reader
	archives *archives
	once     sync.Once
}

func (f *file) Close() error {
	f.once.Do(func() {
		f.archives.mu.Lock()
		f.archives.open--
		f.archives.mu.Unlock()
	})
	return nil
}

func (*file) ModTime() time.Time { return now().Add(-time.Hour) }

// queue records the exports it enqueued.
type queue struct {
	mu  sync.Mutex
	ids []uuid.UUID
}

// enqueued are the exports enqueued so far.
func (q *queue) enqueued() []uuid.UUID {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.ids)
}

func (q *queue) Export(_ context.Context, id uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ids = append(q.ids, id)
	return nil
}

// bucket allows left requests, recording each key.
type bucket struct {
	mu   sync.Mutex
	left int
	keys []string
}

func (b *bucket) Allow(key string) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.keys = append(b.keys, key)
	if b.left == 0 {
		return time.Minute, false
	}
	b.left--
	return 0, true
}

func (b *bucket) Reserve(key string) (func(), time.Duration, bool) {
	wait, ok := b.Allow(key)
	return func() {}, wait, ok
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// harness is the module's routes on the platform's server, which the
// download needs for its deadlines and which sets the fixed headers, over
// the fakes: at most maxQueued jobs, and a store of free bytes, minFree
// kept.
type harness struct {
	rows      *rows
	archives  *archives
	queue     *queue
	signer    macadapter.Signer
	signedAt  time.Time // the time the tests sign addresses at
	logs      *syncBuffer
	contract  *apitest.Contract
	router    *httpserver.Router
	base      string
	client    *http.Client
	anonymous *bucket
}

const (
	maxQueued = 3
	minFree   = 100
)

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, httpservertest.APIOptions{})
}

// newHarnessWith is newHarness with the per-route middlewares of o, its
// authenticator and public operations the module's, its logger the
// harness's, its anonymous bucket the harness's unless o sets one.
func newHarnessWith(t *testing.T, o httpservertest.APIOptions) *harness {
	t.Helper()
	h := &harness{rows: &rows{jobs: map[uuid.UUID]*domain.Job{}}, archives: &archives{files: map[uuid.UUID][]byte{}, free: 1 << 30},
		queue: &queue{}, signer: macadapter.New([]byte("key")), signedAt: now(), logs: &syncBuffer{}, contract: apitest.Load(t),
		client: &http.Client{Timeout: 10 * time.Second}, anonymous: &bucket{left: 1000}}
	logger := slog.New(slog.NewTextHandler(h.logs, nil))
	uc := httpadapter.UseCases{
		Start: app.NewStartExport(app.StartDeps{Tx: direct{}, Authorizer: roles{}, Workspaces: workspaces{}, Notebooks: notebooks{}, Nodes: nodes{},
			Rows: h.rows, Archives: h.archives, Queue: h.queue, Names: names{}, Signer: h.signer, Clock: fixedClock{}, Logger: logger,
			MaxQueued: maxQueued, MinFree: minFree}),
		Reads: app.NewReads(app.ReadsDeps{Authorizer: roles{}, Notebooks: notebooks{}, Names: names{}, Signer: h.signer, Clock: fixedClock{},
			Rows: h.rows, ExportTTL: exportTTL}),
		Cancel: app.NewCancel(app.CancelDeps{Tx: direct{}, Rows: h.rows, Authorizer: roles{}, Notebooks: notebooks{}, Names: names{},
			Signer: h.signer, Clock: fixedClock{}, Logger: logger, ExportTTL: exportTTL}),
		Download: app.NewDownload(h.rows, h.archives, h.signer, fixedClock{}, logger, exportTTL),
	}
	h.router = httpserver.NewRouter(slog.New(slog.DiscardHandler))
	o.Authenticator, o.PublicOperations, o.Logger = fakeAuth{}, httpadapter.PublicOperations(), logger
	if o.Anonymous == nil {
		o.Anonymous = h.anonymous
	}
	httpadapter.Register(h.router, httpservertest.NewAPI(t, o), uc, 64<<10)
	h.serve(t)
	return h
}

// serve runs the routes on a server of the platform's of their own, until
// stop or the test's end: it answers stop and the server's end.
func (h *harness) serve(t *testing.T) (stop func(), done chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{Addr: ln.Addr().String(), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second,
		WriteTimeout: 15 * time.Second, ShutdownTimeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done = make(chan error, 1)
	go func() { done <- httpserver.NewServer(cfg, h.router, slog.New(slog.DiscardHandler)).Serve(ctx, ln) }()
	h.base = "http://" + ln.Addr().String()
	return cancel, done
}

// job puts a job of notebookID() in state, started by by at created: an
// export that succeeded has its archive, holding content, unless content
// is empty.
func (h *harness) job(by uuid.UUID, state domain.State, created time.Time, content string) domain.Job {
	j := domain.Job{ID: uuid.NewV7(), NotebookID: notebookID(), Kind: domain.KindExport, State: state, Name: "Eng", CreatedBy: by,
		Client: domain.ClientWeb, CreatedAt: created}
	if state != domain.StateQueued {
		started := created.Add(time.Second)
		j.Started, j.Heartbeat = &started, &started
	}
	if state.Ended() {
		finished, report := created.Add(time.Minute), domain.Report{Counts: domain.Counts{Pages: 2}}
		if state == domain.StateFailed {
			report.Failure = domain.FailureInterrupted
		}
		j.Finished, j.Report = &finished, &report
		if state == domain.StateSucceeded {
			report.Add(domain.Problem{Path: "Eng/A.md", Code: domain.ProblemRenamed, To: "Eng/A 2.md"})
			size := int64(len(content))
			j.ResultBytes = &size
		}
	}
	h.rows.add(j)
	if content != "" {
		h.archives.files[j.ID] = []byte(content)
	}
	return j
}

// send sends method path with token, none when empty, a JSON body when it
// is not empty, and the headers; checks the answer against the contract,
// but for a HEAD, which the router answers on every GET route; and
// answers it with its body.
func (h *harness) send(t *testing.T, method, path, token, body string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, h.base+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	got, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(got))
	if method != http.MethodHead {
		h.contract.CheckResponse(t, req, res)
	}
	return res, got
}

// waitFor polls cond until it holds, or fails t after 5 s.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatal("the condition did not hold within 5 s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
