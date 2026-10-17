package asset_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	pngenc "image/png"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// readNodes are the page module's nodes as the reads see them: an
// attachment's node by id.
type readNodes map[uuid.UUID]asset.Node

func (n readNodes) Node(_ context.Context, id uuid.UUID) (asset.Node, bool, error) {
	node, ok := n[id]
	return node, ok, nil
}

func (readNodes) Parent(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil }

func (readNodes) Assets(context.Context, uuid.UUID, *uuid.UUID, *asset.Cursor, int) ([]asset.Node, error) {
	return nil, nil
}

// readLinks writes an attachment's link by its name, none without an
// extension.
type readLinks map[uuid.UUID]asset.Node

func (l readLinks) Of(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if n, ok := l[id]; ok {
			link := ""
			if strings.Contains(n.Name, ".") {
				link = "![[" + n.Name + "]]"
			}
			out[id] = link
		}
	}
	return out, nil
}

// readNotebooks has every notebook in one workspace.
type readNotebooks struct{ workspace uuid.UUID }

func (n readNotebooks) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return n.workspace, true, nil
}

// readAuthorizer lets alice read, no one else.
type readAuthorizer struct{ alice uuid.UUID }

func (a readAuthorizer) Authorize(_ context.Context, actor shared.Actor, _ shared.Action, _ shared.Target) (shared.Grant, error) {
	if actor.UserID != a.alice {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookReader}, nil
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// attachments is a database and a store of eng's attachments, each a node
// and a row, with or without its file.
type attachments struct {
	t     *testing.T
	pool  *pgxpool.Pool
	store storage.Store
	alice uuid.UUID
	eng   uuid.UUID
	nodes readNodes
}

func newAttachments(t *testing.T) *attachments {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	a := &attachments{t: t, pool: pool, store: store, alice: uuid.NewV7(), eng: uuid.NewV7(), nodes: readNodes{}}
	acme := uuid.NewV7()
	a.exec("INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())",
		a.alice)
	a.exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())",
		acme, a.alice)
	a.exec("INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now())",
		a.eng, acme, a.alice)
	return a
}

func (a *attachments) exec(sql string, args ...any) {
	a.t.Helper()
	if _, err := a.pool.Exec(context.Background(), sql, args...); err != nil {
		a.t.Fatalf("%s: %v", sql, err)
	}
}

// add adds the attachment name of type mime: its node, its row of bytes
// (rowBytes when it is not -1) and sides (none for 0), and its file of data
// unless data is nil.
func (a *attachments) add(name, mime string, data []byte, rowBytes int64, width, height int) (uuid.UUID, uuid.UUID) {
	a.t.Helper()
	node, blob := uuid.NewV7(), uuid.NewV7()
	a.exec(`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'asset', $3, lower($3), 0, $4, $4, now(), now())`, node, a.eng, name, a.alice)
	if rowBytes < 0 {
		rowBytes = int64(len(data))
	}
	sum := sha256.Sum256(data)
	var w, h *int
	if width > 0 {
		w, h = &width, &height
	}
	a.exec(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, width, height, created_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())`, blob, node, a.eng, mime, rowBytes, sum[:], w, h, a.alice)
	a.nodes[node] = asset.Node{ID: node, NotebookID: a.eng, Asset: true, Name: name, NameKey: strings.ToLower(name)}
	if data != nil {
		f, err := a.store.Create(context.Background(), "blobs/"+blob.String())
		if err != nil {
			a.t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			a.t.Fatal(err)
		}
		if err := f.Commit(); err != nil {
			a.t.Fatal(err)
		}
	}
	return node, blob
}

// reads are the attachments' module's reads, its addresses signed as of
// at from publicURL, its logs to logs.
func (a *attachments) reads(publicURL string, at time.Time, logs *bytes.Buffer) asset.AttachmentReads {
	return asset.New(asset.Deps{
		Pool: a.pool, Store: a.store, Clock: fixedClock(at), Logger: slog.New(slog.NewTextHandler(logs, nil)),
		Authorizer: readAuthorizer{alice: a.alice}, Notebooks: readNotebooks{workspace: uuid.NewV7()}, Nodes: a.nodes,
		Links: readLinks(a.nodes), ContentKey: []byte("content key of the tests"), MaxBytes: 1 << 20, MinRate: 1, PublicURL: publicURL,
	}).AttachmentReads()
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := pngenc.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Read is the GET of an attachment's: decided on asset.read, its row, its
// link and its content's download address signed, absolute from
// server.public_url, with when it expires; none without public_url.
func TestAttachmentReadsReadTheRowAndSignTheAddress(t *testing.T) {
	a := newAttachments(t)
	png := pngOf(t, 7, 5)
	node, blob := a.add("diagram.png", "image/png", png, -1, 7, 5)
	data, _ := a.add("data", "application/octet-stream", []byte("abc"), -1, 0, 0)
	at := time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC)
	alice := shared.WithActor(context.Background(), shared.Actor{UserID: a.alice, APITokenID: uuid.NewV7()})
	var logs bytes.Buffer
	got, err := a.reads("http://wiki.example", at, &logs).Read(alice, node)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png)
	switch {
	case got.NodeID != node || got.BlobID != blob || got.Name != "diagram.png" || got.MIME != "image/png" || got.Bytes != int64(len(png)) ||
		!bytes.Equal(got.SHA256, sum[:]) || got.Width != 7 || got.Height != 5 || got.Link != "![[diagram.png]]":
		t.Errorf("Read = %+v", got)
	case !got.Expires.Equal(time.Date(2026, 10, 17, 11, 0, 0, 0, time.UTC)):
		t.Errorf("the address expires at %v, want the end of the hour after", got.Expires)
	}
	u, err := url.Parse(got.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme+"://"+u.Host != "http://wiki.example" || u.Path != "/api/v0/assets/"+node.String()+"/content" || q.Get("b") != blob.String() ||
		q.Get("e") != strconv.FormatInt(got.Expires.Unix(), 10) || q.Get("d") != "1" || q.Get("s") == "" {
		t.Errorf("the address is %s", got.URL)
	}
	if none, err := a.reads("", at, &logs).Read(alice, node); err != nil || none.URL != "" || !none.Expires.IsZero() || none.Name != "diagram.png" {
		t.Errorf("Read without public_url = %+v, %v", none, err)
	}
	if plain, err := a.reads("", at, &logs).Read(alice, data); err != nil || plain.Link != "" {
		t.Errorf("Read of an attachment without an extension = %+v, %v", plain, err)
	}
	bob := shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), APITokenID: uuid.NewV7()})
	for name, id := range map[string]uuid.UUID{"one bob cannot read": node, "no attachment": uuid.NewV7()} {
		ctx := bob
		if name == "no attachment" {
			ctx = alice
		}
		if _, err := a.reads("", at, &logs).Read(ctx, id); !errors.Is(err, asset.ErrNotFound) {
			t.Errorf("Read of %s = %v, want asset.not_found", name, err)
		}
	}
}

// Content reads the file whole within its bound, the row's sides, or a
// WebP's from its header when its row has none, as before M9/P1; a file
// gone from the store is ErrNoFile, logged at warn; a file of another size
// than its row's is an error; a row gone since Read is asset.not_found.
func TestAttachmentReadsReadTheContent(t *testing.T) {
	a := newAttachments(t)
	png := pngOf(t, 7, 5)
	webp, err := os.ReadFile("adapter/sniff/testdata/vp8l.webp")
	if err != nil {
		t.Fatal(err)
	}
	alice := shared.WithActor(context.Background(), shared.Actor{UserID: a.alice, APITokenID: uuid.NewV7()})
	var logs bytes.Buffer
	reads := a.reads("", time.Now(), &logs)
	read := func(node uuid.UUID) asset.Attachment {
		t.Helper()
		at, err := reads.Read(alice, node)
		if err != nil {
			t.Fatal(err)
		}
		return at
	}
	for _, tt := range []struct {
		name          string
		mime          string
		data          []byte
		width, height int
		wantW, wantH  int
	}{
		{"a PNG", "image/png", png, 7, 5, 7, 5},
		{"a PNG whose row has no sides", "image/png", png, 0, 0, 7, 5},
		{"a WebP whose row has no sides", "image/webp", webp, 0, 0, 7, 5},
		{"a PDF", "application/pdf", []byte("%PDF-1.4"), 0, 0, 0, 0},
	} {
		node, _ := a.add(tt.name, tt.mime, tt.data, -1, tt.width, tt.height)
		data, w, h, err := reads.Content(alice, read(node), 3<<20)
		if err != nil || !bytes.Equal(data, tt.data) || w != tt.wantW || h != tt.wantH {
			t.Errorf("Content(%s) = %d bytes, %d×%d, %v; want %d×%d", tt.name, len(data), w, h, err, tt.wantW, tt.wantH)
		}
	}
	missing, blob := a.add("missing.png", "image/png", nil, 10, 7, 5)
	if _, _, _, err := reads.Content(alice, read(missing), 3<<20); !errors.Is(err, asset.ErrNoFile) ||
		!strings.Contains(logs.String(), "level=WARN msg=\"asset file missing\" blob_id="+blob.String()) {
		t.Errorf("Content of a file not in the store = %v, logging %s", err, logs.String())
	}
	short, _ := a.add("short.png", "image/png", png, int64(len(png))+1, 7, 5)
	if _, _, _, err := reads.Content(alice, read(short), 3<<20); err == nil || errors.Is(err, asset.ErrNoFile) {
		t.Errorf("Content of a file shorter than its row = %v, want an error", err)
	}
	long, _ := a.add("long.png", "image/png", png, -1, 7, 5)
	at := read(long)
	if _, _, _, err := reads.Content(alice, at, int64(len(png))-1); err == nil {
		t.Error("Content past its bound read the file")
	}
	a.exec("UPDATE asset_blobs SET deleted_at = now() WHERE node_id = $1", long)
	if _, _, _, err := reads.Content(alice, at, 3<<20); !errors.Is(err, asset.ErrNotFound) {
		t.Errorf("Content of a row deleted since Read = %v, want asset.not_found", err)
	}
}
