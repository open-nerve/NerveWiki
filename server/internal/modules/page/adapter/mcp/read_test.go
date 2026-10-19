package mcpadapter

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// pageReader is a ReadPath over one page, path and content, that cuts as
// ReadPath does: the window checked, the room the measure leaves the path.
func pageReader(id uuid.UUID, path, content string) readFunc {
	return func(_ uuid.UUID, _ string, offset, column, limit *int, m app.TextMeasure) (app.Read, error) {
		w, err := domain.CheckWindow(offset, column, limit)
		if err != nil {
			return app.Read{}, err
		}
		slice, err := domain.Cut(content, w, m.Room(path), m.Bytes)
		if err != nil {
			return app.Read{}, err
		}
		return app.Read{Path: path, Node: domain.Node{ID: id}, Page: &app.PageRead{
			Version: domain.VersionToken{Node: id, Revision: 7}, Window: w, Slice: slice, Bytes: len(content)}}, nil
	}
}

// readAll reads path through t from its start, part after part as the
// answers say, and answers the parts' text joined; each answer within the
// bound, its first block never wider than the widest.
func readAll(t *testing.T, tools tools, path string) string {
	t.Helper()
	var b strings.Builder
	in := readIn{Path: path}
	for range 1000 {
		res, err := tools.read(context.Background(), mcpserver.Call{}, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Blocks) != 2 {
			t.Fatalf("a page's read is %d blocks, want two", len(res.Blocks))
		}
		widest, _ := line(widest(path))
		meta, text := res.Blocks[0].Text, res.Blocks[1].Text
		if n := mcpserver.TextBytes(meta) + mcpserver.TextBytes(text); n > mcpserver.MaxAnswerBytes ||
			mcpserver.TextBytes(meta) > mcpserver.TextBytes(widest) {
			t.Fatalf("the answer holds %d bytes, its first block %d, the widest %d", n, mcpserver.TextBytes(meta), mcpserver.TextBytes(widest))
		}
		b.WriteString(text)
		m := blockJSON(t, res.Blocks[0])
		if m["next_offset"] == nil {
			return b.String()
		}
		offset := int(m["next_offset"].(float64))
		column := 0
		if m["next_column"] != nil {
			column = int(m["next_column"].(float64))
		}
		in.Offset, in.Column = &offset, &column
	}
	t.Fatal("the reads did not end")
	return ""
}

// A page is two text blocks: a line of JSON metadata, <, > and & as they
// are, next_offset and next_column null when nothing follows or the next
// part starts a line; then the slice, an empty block when there are no
// lines (M9/P1 design 3.11.1).
func TestTheReadOfAPageIsTwoBlocks(t *testing.T) {
	id := uuid.NewV7()
	for _, tt := range []struct {
		name  string
		slice domain.Slice
		meta  string
	}{
		{"all of it", domain.Slice{Text: "a\n", Lines: 1, TotalLines: 1},
			`"lines":1,"total_lines":1,"byte_size":12,"next_offset":null,"next_column":null}`},
		{"more lines", domain.Slice{Text: "a\n", Lines: 1, TotalLines: 3, NextOffset: 2},
			`"lines":1,"total_lines":3,"byte_size":12,"next_offset":2,"next_column":null}`},
		{"a line cut", domain.Slice{Text: "ab", Lines: 1, TotalLines: 3, NextOffset: 1, NextColumn: 2},
			`"lines":1,"total_lines":3,"byte_size":12,"next_offset":1,"next_column":2}`},
		{"past the end", domain.Slice{TotalLines: 3},
			`"lines":0,"total_lines":3,"byte_size":12,"next_offset":null,"next_column":null}`},
	} {
		tools := tools{uc: UseCases{Read: readFunc(func(uuid.UUID, string, *int, *int, *int, app.TextMeasure) (app.Read, error) {
			return app.Read{Path: "A<b>&c.md", Node: domain.Node{ID: id}, Page: &app.PageRead{
				Version: domain.VersionToken{Node: id, Revision: 7}, Window: domain.Window{Offset: 1, Limit: 2000},
				Slice: tt.slice, Bytes: 12}}, nil
		})}}
		res, err := tools.read(context.Background(), mcpserver.Call{}, readIn{Path: "a<b>&c.md"})
		want := `{"path":"A<b>&c.md","kind":"page","version":"` + id.String() + `:7","offset":1,"column":0,` + tt.meta
		switch {
		case err != nil:
			t.Errorf("%s: %v", tt.name, err)
		case res.Structured != nil || len(res.Blocks) != 2:
			t.Errorf("%s: %+v, want two blocks", tt.name, res)
		case res.Blocks[0] != (mcpserver.Block{Text: want}) || res.Blocks[1] != (mcpserver.Block{Text: tt.slice.Text}):
			t.Errorf("%s: the blocks are\n%q\n%q\nwant\n%q", tt.name, res.Blocks[0].Text, res.Blocks[1].Text, want)
		}
	}
}

// The widest first block is at least as wide as any page's: one whose
// every integer has twenty characters and whose next_offset and
// next_column are both set (M9/P1 design 3.11.1).
func TestTheWidestFirstBlockIsTheWidest(t *testing.T) {
	most := math.MinInt64
	for _, path := range []string{"A.md", strings.Repeat("&", 255) + ".md"} {
		widest, err := line(widest(path))
		if err != nil {
			t.Fatal(err)
		}
		wide, err := line(pageMeta{Path: path, Kind: "page", Version: domain.VersionToken{Node: uuid.NewV7(), Revision: most}.String(),
			Offset: most, Column: most, Lines: most, TotalLines: most, Bytes: most, NextOffset: &most, NextColumn: &most})
		if err != nil {
			t.Fatal(err)
		}
		if mcpserver.TextBytes(wide) > mcpserver.TextBytes(widest) {
			t.Errorf("a first block of %d bytes is wider than the widest, %d:\n%s\n%s", mcpserver.TextBytes(wide), mcpserver.TextBytes(widest), wide, widest)
		}
	}
}

// The slice has the room the widest first block leaves: on the longest
// path, all &, with a content of the costliest characters, each answer is
// within the bound and the parts read in turn are the content; on a short
// path a part fills nearly all of the answer (M9/P1 design 3.11.1).
func TestTheReadFitsItsAnswer(t *testing.T) {
	segment := strings.Repeat("&", 255)
	longest := strings.Repeat(segment+"/", 10) + segment + ".md"
	costly := strings.Repeat("\"<&>\n\u2028\x01é漢\r\n", 20000)
	wide := strings.Repeat("&", 200000) + "\nend\n"
	for _, tt := range []struct{ name, path, content string }{
		{"the longest path", longest, costly},
		{"one wide line", longest, wide},
		{"a short path", "A.md", costly},
	} {
		tools := tools{uc: UseCases{Read: pageReader(uuid.NewV7(), tt.path, tt.content)}}
		if got := readAll(t, tools, tt.path); got != tt.content {
			t.Errorf("%s: the parts read in turn are %d bytes, want the content's %d", tt.name, len(got), len(tt.content))
		}
	}
	tools := tools{uc: UseCases{Read: pageReader(uuid.NewV7(), "A.md", strings.Repeat("x", 100000))}}
	res, err := tools.read(context.Background(), mcpserver.Call{}, readIn{Path: "A.md"})
	if err != nil {
		t.Fatal(err)
	}
	if n := mcpserver.TextBytes(res.Blocks[0].Text) + mcpserver.TextBytes(res.Blocks[1].Text); n < mcpserver.MaxAnswerBytes-200 {
		t.Errorf("a part of a short path's page holds %d bytes, want nearly %d", n, mcpserver.MaxAnswerBytes)
	}
}

// fakeAttachments reads one attachment, its content's answer set, and
// counts the content's reads.
type fakeAttachments struct {
	a          app.Attachment
	readErr    error
	data       []byte
	w, h       int
	contentErr error
	contents   int
	max        int64
}

func (f *fakeAttachments) Read(_ context.Context, nodeID uuid.UUID) (app.Attachment, error) {
	if nodeID != f.a.NodeID {
		return app.Attachment{}, errors.New("another node")
	}
	return f.a, f.readErr
}

func (f *fakeAttachments) Content(_ context.Context, a app.Attachment, maxBytes int64) ([]byte, int, int, error) {
	f.contents++
	f.max = maxBytes
	if a.NodeID != f.a.NodeID {
		return nil, 0, 0, errors.New("another attachment")
	}
	return f.data, f.w, f.h, f.contentErr
}

// An attachment is its metadata, then the image for one of the four types
// within 3 MiB and 8,000 pixels a side, the sides those its content tells
// when it was read, else a link; without server.public_url the image all
// the same, which needs no address, else a note; its file gone a note; its
// row's sides unknown read from its bytes (M9/P1 design 3.11.2, the inline
// order).
func TestTheReadOfAnAttachment(t *testing.T) {
	expires := time.Date(2026, 10, 17, 12, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	png := []byte("\x89PNG")
	base := app.Attachment{NodeID: uuid.NewV7(), BlobID: uuid.NewV7(), Name: "diagram.png", MIME: "image/png",
		Link: "![[Guide/diagram.png]]", Bytes: 4, SHA256: []byte{0x9f, 0x2c}, Width: 640, Height: 480,
		URL: "http://wiki.example/api/v0/assets/x/content?d=1", Expires: expires}
	with := func(f func(a *app.Attachment)) app.Attachment {
		a := base
		f(&a)
		return a
	}
	noAddress := func(m map[string]any) bool {
		return m["link"] == base.Link && m["url"] == nil && m["expires_at"] == nil
	}
	tests := []struct {
		name      string
		a         app.Attachment
		w, h      int   // the content's sides
		err       error // the content's error
		read      bool  // whether the content is read
		second    string
		width     any
		note      any
		otherwise func(m map[string]any) bool
	}{
		{name: "a PNG", a: base, w: 640, h: 480, read: true, second: "image", width: 640.0},
		{name: "a WebP whose row has no sides", a: with(func(a *app.Attachment) { a.MIME, a.Width, a.Height = "image/webp", 0, 0 }),
			w: 1, h: 1, read: true, second: "image", width: 1.0},
		{name: "a GIF read of no sides", a: with(func(a *app.Attachment) { a.MIME, a.Width, a.Height = "image/gif", 0, 0 }),
			read: true, second: "link", width: nil},
		{name: "a JPEG whose bytes tell 8,001 pixels", a: with(func(a *app.Attachment) { a.MIME, a.Width, a.Height = "image/jpeg", 0, 0 }),
			w: 8001, h: 10, read: true, second: "link", width: 8001.0},
		{name: "a PNG of 8,000 pixels a side", a: with(func(a *app.Attachment) { a.Width, a.Height = 8000, 8000 }),
			w: 8000, h: 8000, read: true, second: "image", width: 8000.0},
		{name: "a PNG whose row has 8,001 pixels", a: with(func(a *app.Attachment) { a.Width = 8001 }), second: "link", width: 8001.0},
		{name: "a PNG of 3 MiB", a: with(func(a *app.Attachment) { a.Bytes = domain.MaxInlineImageBytes }),
			w: 640, h: 480, read: true, second: "image", width: 640.0},
		{name: "a PNG over 3 MiB", a: with(func(a *app.Attachment) { a.Bytes = domain.MaxInlineImageBytes + 1 }), second: "link", width: 640.0},
		{name: "an SVG", a: with(func(a *app.Attachment) { a.MIME, a.Width, a.Height = "image/svg+xml", 0, 0 }), second: "link", width: nil},
		{name: "a PDF", a: with(func(a *app.Attachment) { a.MIME, a.Width, a.Height = "application/pdf", 0, 0 }), second: "link", width: nil},
		{name: "a PNG without public_url", a: with(func(a *app.Attachment) { a.URL, a.Expires = "", time.Time{} }),
			w: 640, h: 480, read: true, second: "image", width: 640.0, otherwise: noAddress},
		{name: "a JPEG of 8,001 pixels without public_url", a: with(func(a *app.Attachment) {
			a.MIME, a.Width, a.Height, a.URL, a.Expires = "image/jpeg", 0, 0, "", time.Time{}
		}), w: 8001, h: 10, read: true, second: "none", width: 8001.0, note: noPublicURL, otherwise: noAddress},
		{name: "a PDF without public_url", a: with(func(a *app.Attachment) {
			a.MIME, a.Width, a.Height, a.URL, a.Expires = "application/pdf", 0, 0, "", time.Time{}
		}), second: "none", width: nil, note: noPublicURL, otherwise: noAddress},
		{name: "the file gone without public_url", a: with(func(a *app.Attachment) { a.URL, a.Expires = "", time.Time{} }),
			err: app.ErrNoAttachmentFile, read: true, second: "none", width: 640.0, note: noFile, otherwise: noAddress},
		{name: "the file gone", a: base, err: app.ErrNoAttachmentFile, read: true, second: "none", width: 640.0, note: noFile},
		{name: "no extension", a: with(func(a *app.Attachment) {
			a.Name, a.MIME, a.Link, a.Width, a.Height = "data", "application/octet-stream", "", 0, 0
		}), second: "link", width: nil, otherwise: func(m map[string]any) bool { _, ok := m["link"]; return ok && m["link"] == nil }},
	}
	for _, tt := range tests {
		f := &fakeAttachments{a: tt.a, data: png, w: tt.w, h: tt.h, contentErr: tt.err}
		tools := tools{uc: UseCases{Read: readFunc(func(uuid.UUID, string, *int, *int, *int, app.TextMeasure) (app.Read, error) {
			return app.Read{Path: "Guide/" + tt.a.Name, Node: domain.Node{ID: tt.a.NodeID}}, nil
		})}, attachments: f}
		res, err := tools.read(context.Background(), mcpserver.Call{}, readIn{Path: "Guide/" + tt.a.Name})
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		m := blockJSON(t, res.Blocks[0])
		second := "none"
		if len(res.Blocks) == 2 {
			switch b := res.Blocks[1]; {
			case b.Image != nil && bytes.Equal(b.Image.Data, png) && b.Image.MIME == tt.a.MIME:
				second = "image"
			case b.Link != nil && *b.Link == mcpserver.Link{URI: tt.a.URL, Name: tt.a.Name, MIME: tt.a.MIME, Size: tt.a.Bytes}:
				second = "link"
			default:
				second = "another block"
			}
		}
		switch {
		case second != tt.second || len(res.Blocks) > 2:
			t.Errorf("%s: the second block is %s, want %s", tt.name, second, tt.second)
		case (f.contents == 1) != tt.read || f.contents > 1 || (tt.read && f.max != domain.MaxInlineImageBytes):
			t.Errorf("%s: the content was read %d times, at most %d bytes", tt.name, f.contents, f.max)
		case m["width"] != tt.width || (tt.width == nil) != (m["height"] == nil):
			t.Errorf("%s: the sides are %v and %v, want the width %v", tt.name, m["width"], m["height"], tt.width)
		case m["note"] != tt.note:
			t.Errorf("%s: the note is %v, want %v", tt.name, m["note"], tt.note)
		case m["path"] != "Guide/"+tt.a.Name || m["kind"] != "attachment" || m["mime"] != tt.a.MIME ||
			m["byte_size"] != float64(tt.a.Bytes) || m["sha256"] != "9f2c":
			t.Errorf("%s: the metadata is %v", tt.name, m)
		case tt.otherwise != nil && !tt.otherwise(m):
			t.Errorf("%s: the metadata is %v", tt.name, m)
		case tt.otherwise == nil && (m["link"] != tt.a.Link || m["url"] != tt.a.URL || m["expires_at"] != "2026-10-17T10:00:00Z"):
			t.Errorf("%s: the link and the address are %v, %v, %v", tt.name, m["link"], m["url"], m["expires_at"])
		}
	}
}

// An attachment's errors are its reader's: page.not_found when it or its
// access went since the node was found, in the module's words for the path
// as written; a content's failure as it is.
func TestTheReadOfAnAttachmentFails(t *testing.T) {
	a := app.Attachment{NodeID: uuid.NewV7(), Name: "x.png", MIME: "image/png", Bytes: 4, URL: "http://wiki.example/x"}
	boom := errors.New("boom")
	for _, tt := range []struct {
		f    *fakeAttachments
		want error
		text string
	}{
		{&fakeAttachments{a: a, readErr: domain.ErrNotFound}, domain.ErrNotFound, `page.not_found: Nothing is at "/guide/X.png".`},
		// Gone between its metadata and its bytes (M9/P1 S2 fix checks B and C).
		{&fakeAttachments{a: a, contentErr: domain.ErrNotFound}, domain.ErrNotFound, `page.not_found: Nothing is at "/guide/X.png".`},
		{&fakeAttachments{a: a, contentErr: boom}, boom, ""},
	} {
		tools := tools{uc: UseCases{Read: readFunc(func(uuid.UUID, string, *int, *int, *int, app.TextMeasure) (app.Read, error) {
			return app.Read{Path: "Guide/x.png", Node: domain.Node{ID: a.NodeID}}, nil
		})}, attachments: tt.f}
		_, err := tools.read(context.Background(), mcpserver.Call{}, readIn{Path: "/guide/X.png"})
		if text, _ := wordsOf(err); !errors.Is(err, tt.want) || text != tt.text {
			t.Errorf("read = %v, want %v, %q", err, tt.want, tt.text)
		}
	}
}
