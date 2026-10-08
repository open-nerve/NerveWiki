package httpadapter_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
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
// is given and answers checkErr and createErr; created, it runs after
// with a node of its own.
type tree struct {
	mu        sync.Mutex
	checked   []app.NewNode
	created   []app.NewNode
	checkErr  error
	createErr error
}

func (t *tree) Check(_ context.Context, n app.NewNode) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.checked = append(t.checked, n)
	return t.checkErr
}

func (t *tree) CreateAsset(ctx context.Context, n app.NewNode, after func(context.Context, app.Node) error) (app.Node, error) {
	t.mu.Lock()
	t.created = append(t.created, n)
	err := t.createErr
	t.mu.Unlock()
	if err != nil {
		return app.Node{}, err
	}
	node := app.Node{ID: uuid.MustParse("0199a2b4-0000-7000-8000-000000000020"), NotebookID: n.NotebookID, ParentID: n.ParentID,
		Asset: true, Name: n.Name, CreatedBy: alice(), CreatedAt: now()}
	if err := after(ctx, node); err != nil {
		return app.Node{}, err
	}
	return node, nil
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

// harness is the module's routes on a real server, which a stream route
// needs for its deadlines, over the fakes; the largest file is maxBytes.
type harness struct {
	tree     *tree
	files    *memFiles
	rows     *memRows
	logs     *syncBuffer
	contract *apitest.Contract
	router   *httpserver.Router
	base     string
	client   *http.Client
}

const maxBytes = 1 << 10

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{tree: &tree{}, files: newFiles(), rows: &memRows{rows: map[uuid.UUID]domain.Blob{}}, logs: &syncBuffer{},
		contract: apitest.Load(t), client: &http.Client{Timeout: 10 * time.Second}}
	logger := slog.New(slog.NewTextHandler(h.logs, nil))
	upload := app.NewUpload(app.UploadDeps{
		Tree: h.tree, Blobs: app.NewBlobs(h.files, h.rows, sniffer{}), Files: h.files, Signer: app.NewSigner([]byte("key"), fixedClock{}),
		Logger: logger, MaxBytes: maxBytes, MinFree: 100,
	})
	h.router = httpserver.NewRouter(slog.New(slog.DiscardHandler))
	httpadapter.Register(h.router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}}),
		httpadapter.UseCases{Upload: upload}, httpadapter.Limits{MaxBytes: maxBytes, MinRate: 64 << 10}, logger)
	srv := httptest.NewServer(h.router)
	t.Cleanup(srv.Close)
	h.base = srv.URL
	return h
}

// serve runs the routes on the platform's server, which tells a stream
// that shutdown began, until stop: it answers stop and the server's end.
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
