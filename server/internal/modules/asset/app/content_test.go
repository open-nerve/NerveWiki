package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// server is Content over the fakes, with one attachment, photo.png, its
// file holding "abc"; its addresses signed now.
type server struct {
	nodes   *treeNodes
	rows    *memRows
	files   *memFiles
	clock   *clock
	logs    *bytes.Buffer
	content *app.Content
	photo   app.Node
	blob    domain.Blob
	signed  app.Signed
}

func newServer() *server {
	s := &server{rows: newRows(), files: newFiles(), clock: &clock{}, logs: &bytes.Buffer{}}
	s.photo = app.Node{ID: uuid.NewV7(), NotebookID: uuid.NewV7(), Asset: true, Name: "photo.png"}
	s.nodes = &treeNodes{nodes: map[uuid.UUID]app.Node{s.photo.ID: s.photo}}
	s.blob = domain.Blob{ID: uuid.NewV7(), NodeID: s.photo.ID, MIME: "image/png", Bytes: 3}
	s.rows.rows[s.photo.ID] = s.blob
	s.files.files[domain.Key(s.blob.ID)] = []byte("abc")
	signer := macadapter.New(signKey())
	s.signed = signer.Sign(now(), s.photo.ID, s.blob.ID)
	logger := slog.New(slog.NewTextHandler(s.logs, nil))
	s.content = app.NewContent(s.nodes, app.NewBlobs(s.files, s.rows, &sniffer{}, logger), signer, s.clock, logger)
	return s
}

// address is the address signed, shown or downloaded.
func (s *server) address(download bool) app.Address {
	a := app.Address{Node: s.photo.ID, Blob: s.blob.ID, Expires: s.signed.Expires.Unix(), Download: download, Signature: s.signed.Inline}
	if download {
		a.Signature = s.signed.Download
	}
	return a
}

// A signed address opens its file, with its row, the attachment's name,
// and the time it has left, of one read of the clock; shown or
// downloaded.
func TestContentOpensTheFileOfItsAddress(t *testing.T) {
	for _, download := range []bool{false, true} {
		s := newServer()
		s.clock.step = time.Hour
		o, err := s.content.Open(context.Background(), s.address(download))
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(o.File)
		if string(got) != "abc" || o.Blob.ID != s.blob.ID || o.Name != "photo.png" || o.Left != 90*time.Minute || s.clock.reads != 1 {
			t.Errorf("Open(download %v) = %q, %+v, %q, %v left after %d reads of the clock; want abc, its row, photo.png, 90 min, one",
				download, got, o.Blob, o.Name, o.Left, s.clock.reads)
		}
	}
}

// A read of the row that fails is the download's failure, not not_found.
func TestContentAnswersAReadThatFails(t *testing.T) {
	s := newServer()
	s.rows.readErr = errPort
	if _, err := s.content.Open(context.Background(), s.address(false)); !errors.Is(err, errPort) {
		t.Errorf("Open() = %v, want the rows' failure", err)
	}
}

// Each address that is not one signed, or that leads to nothing, is
// not_found; a file gone is logged as a warning. The signature is checked
// before the node is read.
func TestContentAnswersNotFound(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(s *server, a *app.Address)
		read   bool
		warned bool
	}{
		{"another signature", func(_ *server, a *app.Address) { a.Signature = strings.Repeat("A", 22) }, false, false},
		{"the shown signature downloaded", func(_ *server, a *app.Address) { a.Download = true }, false, false},
		{"a later expiry", func(_ *server, a *app.Address) { a.Expires += 3600 }, false, false},
		{"another file", func(_ *server, a *app.Address) { a.Blob = uuid.NewV7() }, false, false},
		{"an address expired", func(s *server, a *app.Address) {
			logger := slog.New(slog.NewTextHandler(s.logs, nil))
			s.content = app.NewContent(s.nodes, app.NewBlobs(s.files, s.rows, &sniffer{}, logger), macadapter.New(signKey()),
				fixedClock{s.signed.Expires}, logger)
		}, false, false},
		{"a node deleted", func(s *server, _ *app.Address) { delete(s.nodes.nodes, s.photo.ID) }, true, false},
		{"a page's node", func(s *server, _ *app.Address) {
			p := s.photo
			p.Asset = false
			s.nodes.nodes[p.ID] = p
		}, true, false},
		{"a row deleted", func(s *server, _ *app.Address) { delete(s.rows.rows, s.photo.ID) }, true, false},
		{"a row of another file, signed", func(s *server, a *app.Address) {
			a.Blob = uuid.NewV7()
			a.Signature = macadapter.New(signKey()).Sign(now(), s.photo.ID, a.Blob).Inline
		}, true, false},
		{"a file gone", func(s *server, _ *app.Address) { delete(s.files.files, domain.Key(s.blob.ID)) }, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newServer()
			a := s.address(false)
			tt.change(s, &a)
			if _, err := s.content.Open(context.Background(), a); !errors.Is(err, domain.ErrContentNotFound) {
				t.Errorf("Open() = %v, want not_found", err)
			}
			if read := s.nodes.reads > 0; read != tt.read {
				t.Errorf("the node read: %v, want %v", read, tt.read)
			}
			l := s.logs.String()
			if warned := strings.Contains(l, "level=WARN") && strings.Contains(l, "blob_id="+s.blob.ID.String()) &&
				strings.Contains(l, "node_id="+s.photo.ID.String()); warned != tt.warned {
				t.Errorf("logs %q, want a warning naming the file and its node: %v", l, tt.warned)
			}
		})
	}
}
