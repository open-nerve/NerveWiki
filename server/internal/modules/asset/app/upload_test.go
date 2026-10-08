package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// tree stands in for the page module's writes: it records the nodes it
// is given, answers checkErr and createErr, and runs after with a node of
// its own unless createErr is set.
type tree struct {
	checked, created []app.NewNode
	checkErr         error
	createErr        error
	afterErr         error
	node             app.Node
}

func (t *tree) Check(_ context.Context, n app.NewNode) error {
	t.checked = append(t.checked, n)
	return t.checkErr
}

func (t *tree) CreateAsset(ctx context.Context, n app.NewNode, after func(context.Context, app.Node) error) (app.Node, error) {
	t.created = append(t.created, n)
	if t.createErr != nil {
		return app.Node{}, t.createErr
	}
	t.node = app.Node{ID: uuid.NewV7(), NotebookID: n.NotebookID, ParentID: n.ParentID, Asset: true, Name: n.Name,
		CreatedBy: uuid.NewV7(), CreatedAt: now()}
	if t.afterErr = after(ctx, t.node); t.afterErr != nil {
		return app.Node{}, t.afterErr
	}
	return t.node, nil
}

// uploader is an Upload over the fakes, its log in logs.
type uploader struct {
	tree  *tree
	files *memFiles
	rows  *memRows
	logs  *bytes.Buffer
	uc    *app.Upload
}

func newUploader() *uploader {
	u := &uploader{tree: &tree{}, files: newFiles(), rows: newRows(), logs: &bytes.Buffer{}}
	u.uc = app.NewUpload(app.UploadDeps{
		Tree: u.tree, Blobs: app.NewBlobs(u.files, u.rows, &sniffer{sniffed: "image/png"}, slog.New(slog.NewTextHandler(u.logs, nil))), Files: u.files,
		Signer: macadapter.New(signKey()), Logger: slog.New(slog.NewTextHandler(u.logs, nil)), MaxBytes: 8, MinFree: 100,
	})
	return u
}

func notebookID() uuid.UUID { return uuid.MustParse("0192b7c4-0000-7000-8000-00000000000a") }

// Check asks the page module about the node it would create, decided on
// asset.upload, then the store's free space: less than it keeps is
// storage_full; as much passes.
func TestCheckDecidesTheNodeThenTheFreeSpace(t *testing.T) {
	u := newUploader()
	parent := uuid.NewV7()
	req := app.Request{NotebookID: notebookID(), ParentID: &parent, Name: "a.png", Client: "api"}
	u.files.free = 100
	if err := u.uc.Check(context.Background(), req); err != nil {
		t.Errorf("Check() = %v with as much free space as is kept, want nil", err)
	}
	want := app.NewNode{NotebookID: notebookID(), ParentID: &parent, Name: "a.png", Action: domain.ActionUpload, Client: "api"}
	if len(u.tree.checked) != 1 || u.tree.checked[0].Name != want.Name || *u.tree.checked[0].ParentID != parent ||
		u.tree.checked[0].Action != want.Action || u.tree.checked[0].Client != "api" || u.tree.checked[0].NotebookID != notebookID() {
		t.Errorf("checked %+v, want %+v", u.tree.checked, want)
	}
	u.files.free = 99
	if err := u.uc.Check(context.Background(), req); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Check() = %v with less free space than is kept, want storage_full", err)
	}
	u.tree.checkErr = shared.Forbidden()
	u.files.free = 0
	if err := u.uc.Check(context.Background(), req); !errors.Is(err, shared.Forbidden()) {
		t.Errorf("Check() = %v, want the page module's forbidden before the space", err)
	}
}

// Store keeps the file up to asset.max_bytes; past it, payload too large.
func TestStoreBoundsTheFile(t *testing.T) {
	u := newUploader()
	req := app.Request{NotebookID: notebookID(), Name: "a.png"}
	if _, err := u.uc.Store(context.Background(), req, strings.NewReader("12345678")); err != nil {
		t.Errorf("Store(8 bytes) = %v, want it kept", err)
	}
	if _, err := u.uc.Store(context.Background(), req, strings.NewReader("123456789")); !errors.Is(err, domain.ErrTooLarge) {
		t.Errorf("Store(9 bytes) = %v, want ErrTooLarge", err)
	}
}

// Store tells the file's type by the name as the node keeps it: a blank
// after the extension, trimmed, is no part of it.
func TestStoreTellsTheTypeByTheNameAsKept(t *testing.T) {
	files := newFiles()
	uc := app.NewUpload(app.UploadDeps{Blobs: app.NewBlobs(files, newRows(), &sniffer{sniffed: domain.Octet}, slog.New(slog.DiscardHandler)),
		Files: files, MaxBytes: 8})
	blob, err := uc.Store(context.Background(), app.Request{NotebookID: notebookID(), Name: " report.pdf "}, strings.NewReader("%PDF"))
	if err != nil || blob.MIME != "application/pdf" {
		t.Errorf("Store( report.pdf ) = %q, %v; want application/pdf", blob.MIME, err)
	}
}

// Create writes the row in the node's unit, with the node's id, notebook,
// uploader and time, and answers the attachment signed; its log names the
// ids, the type and the size, never the name.
func TestCreateAttachesTheRowInTheUnit(t *testing.T) {
	u := newUploader()
	req := app.Request{NotebookID: notebookID(), Name: "secret plan.png", Client: "web"}
	blob, err := u.uc.Store(context.Background(), req, strings.NewReader("abc"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := u.uc.Create(context.Background(), req, blob)
	if err != nil {
		t.Fatal(err)
	}
	n := u.tree.node
	row := u.rows.rows[n.ID]
	if row.ID != blob.ID || row.NotebookID != notebookID() || row.CreatedBy != n.CreatedBy || !row.CreatedAt.Equal(now()) || row.MIME != "image/png" {
		t.Errorf("row = %+v, want the blob of node %+v", row, n)
	}
	created := u.tree.created[0]
	if created.Meta.MIME != "image/png" || created.Meta.Bytes != 3 || !bytes.Equal(created.Meta.SHA256, blob.SHA256) ||
		created.Action != domain.ActionUpload {
		t.Errorf("created %+v, want the file's type, size and SHA-256 for the guards", created)
	}
	if a.Node != n || a.Blob.ID != blob.ID || a.Blob.NodeID != n.ID ||
		a.Signed != macadapter.New(signKey()).Sign(n.CreatedAt, n.ID, blob.ID) {
		t.Errorf("answer = %+v, want the node, its blob and their address signed", a)
	}
	logs := u.logs.String()
	for _, want := range []string{"asset uploaded", n.ID.String(), notebookID().String(), blob.ID.String(), n.CreatedBy.String(),
		"mime=image/png", "bytes=3", "client=web"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q lacks %q", logs, want)
		}
	}
	if strings.Contains(logs, "secret") {
		t.Errorf("log %q tells the name", logs)
	}
}

// A unit that refuses the attachment, a *shared.Error, rolled its row
// back: the file is deleted. Any other failure, the row's or the commit's,
// may have kept the row: the file stays, for the orphan sweep.
func TestCreateDeletesTheFileOnARefusalOnly(t *testing.T) {
	for _, tt := range []struct {
		name      string
		createErr error
		rowErr    error
		deleted   bool
	}{
		{"a name taken since the check", shared.NewError(shared.KindConflict, "page.title_taken", "Taken."), nil, true},
		{"the guard's refusal", shared.NewError(shared.KindConflict, "page.locked", "Locked."), nil, true},
		{"a commit whose outcome is unknown", errors.New("commit: connection reset"), nil, false},
		{"a row that failed", nil, errors.New("insert: connection reset"), false},
		{"a deadline", context.DeadlineExceeded, nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u := newUploader()
			u.tree.createErr, u.rows.err = tt.createErr, tt.rowErr
			req := app.Request{NotebookID: notebookID(), Name: "a.png", Client: "web"}
			blob, err := u.uc.Store(context.Background(), req, strings.NewReader("abc"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := u.uc.Create(context.Background(), req, blob); err == nil {
				t.Fatal("Create() = nil, want the failure")
			}
			if deleted := len(u.files.keys()) == 0; deleted != tt.deleted {
				t.Errorf("the file deleted: %v, want %v", deleted, tt.deleted)
			}
			if strings.Contains(u.logs.String(), "asset uploaded") {
				t.Errorf("a failed upload logged %q", u.logs)
			}
		})
	}
}

// The signed time is the hour's: the answer of an upload at 10:30 lasts
// until 12:00.
func TestCreateSignsForTheHourAfter(t *testing.T) {
	u := newUploader()
	req := app.Request{NotebookID: notebookID(), Name: "a.png", Client: "web"}
	blob, _ := u.uc.Store(context.Background(), req, strings.NewReader("abc"))
	a, err := u.uc.Create(context.Background(), req, blob)
	if err != nil || !a.Signed.Expires.Equal(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("Create() = %+v, %v; want it signed until 12:00", a.Signed, err)
	}
}
