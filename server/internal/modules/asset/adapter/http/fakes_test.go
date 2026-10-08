package httpadapter_test

import (
	"bytes"
	"context"
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

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeAuth takes "session" for a sign-in session's access token and "pat"
// for a personal access token, both alice's.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	actor := shared.Actor{UserID: alice()}
	switch token {
	case "session":
		actor.SessionID = uuid.NewV7()
	case "pat":
		actor.APITokenID = uuid.NewV7()
	default:
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, actor), token, nil
}

func alice() uuid.UUID      { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000001") }
func notebookID() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000010") }
func parentID() uuid.UUID   { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000011") }

func now() time.Time { return time.Date(2026, 10, 8, 10, 30, 0, 0, time.UTC) }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now() }

// tree stands in for the page module's writes: it records the nodes it
// is given and answers checkErr and createErr, or waits for the end of
// its context when stalled; created, it runs after with a node of its
// own, which nodes then hold.
type tree struct {
	mu          sync.Mutex
	nodes       *memNodes
	checked     []app.NewNode
	created     []app.NewNode
	checkErr    error
	createErr   error
	stallCheck  bool
	stallCreate bool
}

func (t *tree) Check(ctx context.Context, n app.NewNode) error {
	t.mu.Lock()
	t.checked = append(t.checked, n)
	err, stall := t.checkErr, t.stallCheck
	t.mu.Unlock()
	if stall {
		<-ctx.Done()
		return ctx.Err()
	}
	return err
}

func (t *tree) CreateAsset(ctx context.Context, n app.NewNode, after func(context.Context, app.Node) error) (app.Node, error) {
	t.mu.Lock()
	t.created = append(t.created, n)
	err, stall := t.createErr, t.stallCreate
	t.mu.Unlock()
	if stall {
		<-ctx.Done()
		return app.Node{}, ctx.Err()
	}
	if err != nil {
		return app.Node{}, err
	}
	node := app.Node{ID: uuid.MustParse("0199a2b4-0000-7000-8000-000000000020"), NotebookID: n.NotebookID, ParentID: n.ParentID,
		Asset: true, Name: n.Name, CreatedBy: alice(), CreatedAt: now()}
	if err := after(ctx, node); err != nil {
		return app.Node{}, err
	}
	t.nodes.add(node)
	return node, nil
}

// memNodes stands in for the page module's reads of the trees: the nodes
// not deleted. A gate, when set, holds each Node until it is closed, its
// call told on asked.
type memNodes struct {
	mu    sync.Mutex
	nodes map[uuid.UUID]app.Node
	gate  chan struct{}
	asked chan struct{}
}

func (m *memNodes) add(n app.Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[n.ID] = n
}

func (m *memNodes) Node(ctx context.Context, id uuid.UUID) (app.Node, bool, error) {
	if m.gate != nil {
		m.asked <- struct{}{}
		select {
		case <-m.gate:
		case <-ctx.Done():
			return app.Node{}, false, ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[id]
	return n, ok, nil
}

func (m *memNodes) Parent(_ context.Context, notebookID, parentID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[parentID]
	return ok && !n.Asset && n.NotebookID == notebookID, nil
}

func (m *memNodes) Assets(_ context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *app.Cursor, limit int) ([]app.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []app.Node
	for _, n := range m.nodes {
		under := parentID == nil && n.ParentID == nil || parentID != nil && n.ParentID != nil && *parentID == *n.ParentID
		if n.Asset && n.NotebookID == notebookID && under &&
			(after == nil || n.NameKey > after.NameKey || n.NameKey == after.NameKey && n.ID.Compare(after.ID) > 0) {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b app.Node) int {
		if c := strings.Compare(a.NameKey, b.NameKey); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	return out[:min(limit, len(out))], nil
}

// sees lets every caller read in notebookID() alone; it hides the rest.
type sees struct{}

func (sees) Authorize(_ context.Context, _ shared.Actor, _ shared.Action, t shared.Target) (shared.Grant, error) {
	if t.NotebookID != notebookID() {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{NotebookRole: shared.NotebookReader}, nil
}

// books has every notebook in one workspace.
type books struct{}

func (books) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return uuid.MustParse("0199a2b4-0000-7000-8000-000000000002"), true, nil
}

// bucket lets left requests through, then refuses each, a minute to wait.
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
	retry, ok := b.Allow(key)
	return func() {}, retry, ok
}

// memFiles is the store in memory. full refuses a Create; fullAfter fails
// a write past so many bytes; creating signals each Create.
type memFiles struct {
	mu        sync.Mutex
	files     map[string][]byte
	deleted   []string
	open      int
	full      bool
	fullAfter int
	free      int64
	creating  chan struct{}
}

func newFiles() *memFiles {
	return &memFiles{files: map[string][]byte{}, free: 1 << 40, creating: make(chan struct{}, 8)}
}

func (f *memFiles) Create(_ context.Context, key string) (app.FileWriter, error) {
	f.creating <- struct{}{}
	if f.full {
		return nil, domain.ErrStorageFull
	}
	f.mu.Lock()
	f.open++
	f.mu.Unlock()
	return &memWriter{f: f, key: key}, nil
}

func (f *memFiles) Open(_ context.Context, key string) (app.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.files[key]
	if !ok {
		return nil, app.ErrNoFile
	}
	return memFile{bytes.NewReader(b)}, nil
}

func (f *memFiles) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.files, key)
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *memFiles) Free(context.Context) (int64, error) { return f.free, nil }

// state is the committed files' keys, the deleted, and the writers open.
func (f *memFiles) state() (keys, deleted []string, open int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.files {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, slices.Clone(f.deleted), f.open
}

type memWriter struct {
	f   *memFiles
	key string
	buf bytes.Buffer
}

func (w *memWriter) Write(p []byte) (int, error) {
	if w.f.fullAfter > 0 && w.buf.Len()+len(p) > w.f.fullAfter {
		return 0, domain.ErrStorageFull
	}
	return w.buf.Write(p)
}

func (w *memWriter) Commit() error { return w.close(true) }
func (w *memWriter) Abort() error  { return w.close(false) }

func (w *memWriter) close(keep bool) error {
	w.f.mu.Lock()
	defer w.f.mu.Unlock()
	w.f.open--
	if keep {
		w.f.files[w.key] = w.buf.Bytes()
	}
	return nil
}

type memFile struct{ *bytes.Reader }

func (memFile) Close() error       { return nil }
func (memFile) ModTime() time.Time { return now() }

// sniffer is net/http's sniffing; it reads no image's size.
type sniffer struct{}

func (sniffer) Sniff(head []byte) string        { return http.DetectContentType(head) }
func (sniffer) Size(io.Reader) (int, int, bool) { return 0, 0, false }

// memRows keeps the rows by node.
type memRows struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.Blob
}

func (r *memRows) CreateBlob(_ context.Context, b domain.Blob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[b.NodeID] = b
	return nil
}

func (r *memRows) BlobOfNode(_ context.Context, nodeID uuid.UUID) (domain.Blob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.rows[nodeID]
	if !ok {
		return domain.Blob{}, app.ErrNoRow
	}
	return b, nil
}

func (r *memRows) BlobsOfNodes(_ context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]domain.Blob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[uuid.UUID]domain.Blob{}
	for _, id := range nodeIDs {
		if b, ok := r.rows[id]; ok {
			out[id] = b
		}
	}
	return out, nil
}

// syncBuffer is a log the server's goroutines write to.
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

// harness is the module's routes on the platform's server, which a stream
// route needs for its deadlines and which sets the fixed headers, over the
// fakes; the largest file is maxBytes.
// The downloads' bucket lets downloads requests through.
type harness struct {
	tree      *tree
	nodes     *memNodes
	files     *memFiles
	rows      *memRows
	signer    macadapter.Signer
	signedAt  time.Time // the time the tests sign addresses at
	logs      *syncBuffer
	contract  *apitest.Contract
	router    *httpserver.Router
	base      string
	client    *http.Client
	downloads *bucket
}

const maxBytes = 1 << 10

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, httpservertest.APIOptions{})
}

// newHarnessWith is newHarness with the per-route middlewares of o, its
// authenticator and public operations the module's, its logger the
// harness's.
func newHarnessWith(t *testing.T, o httpservertest.APIOptions) *harness {
	t.Helper()
	h := &harness{nodes: &memNodes{nodes: map[uuid.UUID]app.Node{}}, files: newFiles(), rows: &memRows{rows: map[uuid.UUID]domain.Blob{}},
		signer: macadapter.New([]byte("key")), signedAt: now(), logs: &syncBuffer{}, contract: apitest.Load(t),
		client: &http.Client{Timeout: 10 * time.Second}, downloads: &bucket{left: 1000}}
	h.tree = &tree{nodes: h.nodes}
	logger := slog.New(slog.NewTextHandler(h.logs, nil))
	blobs := app.NewBlobs(h.files, h.rows, sniffer{}, logger)
	uc := httpadapter.UseCases{
		Upload: app.NewUpload(app.UploadDeps{Tree: h.tree, Blobs: blobs, Files: h.files, Signer: h.signer, Logger: logger,
			MaxBytes: maxBytes, MinFree: 100}),
		Reads: app.NewReads(app.ReadsDeps{Authorizer: sees{}, Notebooks: books{}, Nodes: h.nodes, Rows: h.rows, Signer: h.signer,
			Clock: fixedClock{}, Logger: logger}),
		Content: app.NewContent(h.nodes, blobs, h.signer, fixedClock{}, logger),
	}
	h.router = httpserver.NewRouter(slog.New(slog.DiscardHandler))
	o.Authenticator, o.PublicOperations, o.Logger = fakeAuth{}, httpadapter.PublicOperations(), logger
	api := httpservertest.NewAPI(t, o)
	httpadapter.Register(h.router, api, uc, httpadapter.Limits{MaxBytes: maxBytes, MinRate: 64 << 10, ContentBucket: h.downloads}, logger)
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
