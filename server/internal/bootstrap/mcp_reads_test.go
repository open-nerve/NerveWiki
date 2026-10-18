package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The MCP reads through serve (M9/P1 design 3.13): list and read through
// both entries reach page's use cases, an attachment's last hop reaches
// asset's files, and no string argument answers internal_error.

// publicOrigin is server.public_url in the reads' tests: no address the
// test's server listens on, so an address made from the request's Host
// would not start with it.
const publicOrigin = "http://wiki.example"

// readsApp is an MCP app of startMCPApp's, of server.public_url set as
// asked, with its store and database at hand.
type readsApp struct {
	*mcpApp
	storage string
	pool    *pgxpool.Pool
}

// startReadsApp runs a readsApp of publicURL, with the modules' tools
// alone.
func startReadsApp(t *testing.T, publicURL string) readsApp {
	t.Helper()
	db := pgtest.NewDatabase(t)
	logs := &syncBuffer{}
	cfg := testConfig(t, db, false)
	cfg.Server.PublicURL = publicURL
	m := runMCPApp(t, wireMCPApp(t, cfg, logs, nil), db, logs)
	return readsApp{mcpApp: m, storage: cfg.Storage.Dir, pool: connect(t, db)}
}

// createPage creates a page titled title with content in alice's
// notebook, under parent ("" for the root), and returns its id.
func (r readsApp) createPage(t *testing.T, parent, title, content string) string {
	t.Helper()
	body := map[string]any{"parent_id": nil, "title": title, "content": content}
	if parent != "" {
		body["parent_id"] = parent
	}
	b, _ := json.Marshal(body)
	status, got := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/notebooks/"+r.notebook+"/pages", r.access, string(b))
	if status != http.StatusCreated {
		t.Fatalf("create %s = %d %s", title, status, got)
	}
	return idOf(t, answer{body: got})
}

// createPageFirst creates a page titled title in alice's notebook, first
// among parent's children, and returns its id.
func (r readsApp) createPageFirst(t *testing.T, parent, title string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"parent_id": parent, "title": title, "after_id": nil})
	status, got := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/notebooks/"+r.notebook+"/pages", r.access, string(b))
	if status != http.StatusCreated {
		t.Fatalf("create %s first = %d %s", title, status, got)
	}
	return idOf(t, answer{body: got})
}

// upload uploads content named name into alice's notebook under parent and
// answers the attachment.
func (r readsApp) upload(t *testing.T, parent, name string, content []byte) uploadedAsset {
	t.Helper()
	c := assetUpload(t, "alice", r.notebook, parent, name, string(content))
	status, got := askTyped(t, r.contract, c.method, r.base+c.path, r.access, c.contentType, c.body)
	if status != http.StatusCreated {
		t.Fatalf("upload %s = %d %s", name, status, got)
	}
	var a uploadedAsset
	if err := json.Unmarshal([]byte(got), &a); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(a.ContentURL)
	if err != nil {
		t.Fatal(err)
	}
	a.Blob = u.Query().Get("b")
	return a
}

// entryCall is tool's call with args through entry, the notebook entry of
// alice's notebook or the account entry with it as the notebook argument.
func (r readsApp) entryCall(t *testing.T, entry mcpserver.Entry, tool string, args map[string]any) map[string]any {
	t.Helper()
	path, all := atNotebook(r.notebook), map[string]any{}
	for k, v := range args {
		all[k] = v
	}
	if entry == mcpserver.AccountEntry {
		path, all["notebook"] = mcpserver.Pattern, r.notebook
	}
	res := r.call(t, path, tool, all).result(t)
	if res["isError"] == true {
		t.Fatalf("%s %v through the %s entry = %v, want a result", tool, args, entry, res)
	}
	return res
}

// blocksOf is a text tool's content blocks.
func blocksOf(t *testing.T, res map[string]any) []map[string]any {
	t.Helper()
	content, _ := res["content"].([]any)
	out := make([]map[string]any, len(content))
	for i, c := range content {
		out[i], _ = c.(map[string]any)
	}
	return out
}

// metaOf is the JSON of a read's first block.
func metaOf(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	blocks := blocksOf(t, res)
	if len(blocks) == 0 || blocks[0]["type"] != "text" {
		t.Fatalf("read answered %v, want a first text block", res)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(blocks[0]["text"].(string)), &m); err != nil {
		t.Fatalf("the first block %v: %v", blocks[0], err)
	}
	return m
}

// pathsOf is the paths of a list's items.
func pathsOf(res map[string]any) []string {
	sc, _ := res["structuredContent"].(map[string]any)
	items, _ := sc["items"].([]any)
	out := []string{}
	for _, item := range items {
		path, _ := item.(map[string]any)["path"].(string)
		out = append(out, path)
	}
	return out
}

// imageOf is an image of 2 × 3 pixels in format: png, jpeg or gif.
func imageOf(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 2, 3), color.Palette{color.Black, color.White})
	var b bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&b, img)
	case "jpeg":
		err = jpeg.Encode(&b, img, nil)
	case "gif":
		err = gif.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// webpOf is a lossless WebP of 1 × 1 pixel (VP8L), as e2e's webpBytes.
func webpOf(t *testing.T) []byte {
	t.Helper()
	b, err := hex.DecodeString("524946461a000000574542505650384c0e0000002f00000000071011fd0f4444ff03")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// list and read reach page's use cases through both entries: the root's
// children, a page's children in the tree's order, which is neither the
// names' nor the ids', a page's content exactly; null for an optional
// argument is as if it were not there.
func TestTheMCPReadsReachTheUseCases(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	content := "\ufeff---\r\ntitle: Guide\r\n---\r\nLine one\r\n"
	guide := r.createPage(t, "", "Guide", content)
	r.createPage(t, guide, "Setup", "")
	r.createPage(t, guide, "Alpha", "")
	r.createPageFirst(t, guide, "Beta")
	for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
		for _, args := range []map[string]any{{}, {"pattern": nil, "limit": nil, "cursor": nil}} {
			res := r.entryCall(t, entry, "list", args)
			sc, _ := res["structuredContent"].(map[string]any)
			items, _ := sc["items"].([]any)
			if len(items) != 1 || sc["next_cursor"] != nil {
				t.Fatalf("list %v through the %s entry = %v, want Guide.md alone", args, entry, sc)
			}
			item := items[0].(map[string]any)
			if item["path"] != "Guide.md" || item["kind"] != "page" || item["has_children"] != true || item["version"] != guide+":1" ||
				item["byte_size"] != float64(len(content)) {
				t.Errorf("list %v through the %s entry: %v", args, entry, item)
			}
		}
		if got := pathsOf(r.entryCall(t, entry, "list", map[string]any{"pattern": "guide/"})); !slices.Equal(got,
			[]string{"Guide/Beta.md", "Guide/Setup.md", "Guide/Alpha.md"}) {
			t.Errorf("list guide/ through the %s entry = %v, want the tree's order", entry, got)
		}
		if got := pathsOf(r.entryCall(t, entry, "list", map[string]any{"pattern": "**/*.MD"})); !slices.Equal(got,
			[]string{"Guide.md", "Guide/Alpha.md", "Guide/Beta.md", "Guide/Setup.md"}) {
			t.Errorf("list **/*.MD through the %s entry = %v, want the paths' order", entry, got)
		}
		for _, args := range []map[string]any{{"path": "/guide.md"}, {"path": "Guide.md", "offset": nil, "column": nil, "limit": nil}} {
			res := r.entryCall(t, entry, "read", args)
			blocks, meta := blocksOf(t, res), metaOf(t, res)
			if _, ok := res["structuredContent"]; ok || len(blocks) != 2 || blocks[1]["text"] != content {
				t.Fatalf("read %v through the %s entry = %v, want the metadata and the content", args, entry, res)
			}
			if meta["path"] != "Guide.md" || meta["version"] != guide+":1" || meta["total_lines"] != 4.0 || meta["next_offset"] != nil {
				t.Errorf("read %v through the %s entry: %v", args, entry, meta)
			}
		}
	}
}

// An attachment's last hop (v0.1 design 13.1 item 21; M9/P1 design 3.13):
// the four images uploaded come as image blocks of their bytes, the sides
// the upload read; a WebP whose row lost its sides, as one uploaded
// before M9/P1, as well, the sides read from its bytes; a PDF and an SVG
// as links at server.public_url, which the test's address downloads; a
// PNG whose file went as its metadata and a note, logged at warn.
func TestTheMCPReadsGiveAttachments(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	guide := r.createPage(t, "", "Guide", "")
	images := []struct {
		name, mime string
		bytes      []byte
		w, h       float64
	}{
		{"x.png", "image/png", imageOf(t, "png"), 2, 3},
		{"x.jpg", "image/jpeg", imageOf(t, "jpeg"), 2, 3},
		{"x.gif", "image/gif", imageOf(t, "gif"), 2, 3},
		{"x.webp", "image/webp", webpOf(t), 1, 1},
		{"old.webp", "image/webp", webpOf(t), 1, 1},
	}
	uploaded := map[string]uploadedAsset{}
	for _, img := range images {
		uploaded[img.name] = r.upload(t, guide, img.name, img.bytes)
	}
	if n := count(t, r.pool, `SELECT count(*) FROM asset_blobs WHERE mime = 'image/webp' AND width = 1 AND height = 1`); n != 2 {
		t.Fatalf("%d WebP rows with their sides, want the upload to have read both", n)
	}
	if _, err := r.pool.Exec(context.Background(), `UPDATE asset_blobs SET width = NULL, height = NULL WHERE node_id = $1`,
		uploaded["old.webp"].ID); err != nil {
		t.Fatal(err)
	}
	for _, img := range images {
		res := r.entryCall(t, mcpserver.NotebookEntry, "read", map[string]any{"path": "Guide/" + img.name})
		blocks, meta := blocksOf(t, res), metaOf(t, res)
		sum := sha256.Sum256(img.bytes)
		switch {
		case len(blocks) != 2 || blocks[1]["type"] != "image" || blocks[1]["mimeType"] != img.mime ||
			blocks[1]["data"] != base64.StdEncoding.EncodeToString(img.bytes):
			t.Errorf("read %s = %v, want its image", img.name, blocks)
		case meta["kind"] != "attachment" || meta["mime"] != img.mime || meta["byte_size"] != float64(len(img.bytes)) ||
			meta["sha256"] != hex.EncodeToString(sum[:]) || meta["width"] != img.w || meta["height"] != img.h ||
			meta["link"] != uploaded[img.name].Link || !strings.HasPrefix(meta["url"].(string), publicOrigin+"/api/v0/assets/") ||
			meta["expires_at"] == nil || meta["note"] != nil:
			t.Errorf("read %s: the metadata %v", img.name, meta)
		}
	}
	files := map[string]struct {
		mime    string
		content []byte
	}{"spec.pdf": {"application/pdf", []byte("%PDF-1.4\n%%EOF\n")},
		"logo.svg": {"image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`)}}
	for name, file := range files {
		content := file.content
		r.upload(t, guide, name, content)
		blocks := blocksOf(t, r.entryCall(t, mcpserver.AccountEntry, "read", map[string]any{"path": "Guide/" + name}))
		if len(blocks) != 2 || blocks[1]["type"] != "resource_link" || blocks[1]["name"] != name || blocks[1]["size"] != float64(len(content)) ||
			blocks[1]["mimeType"] != file.mime {
			t.Fatalf("read %s = %v, want a link of %s", name, blocks, file.mime)
		}
		uri, _ := blocks[1]["uri"].(string)
		u, err := url.Parse(uri)
		if err != nil || !strings.HasPrefix(uri, publicOrigin+"/api/v0/assets/") || u.Query().Get("d") != "1" {
			t.Fatalf("the link's address %q, want the download's at %s", uri, publicOrigin)
		}
		req := newRequest(t, http.MethodGet, r.base+strings.TrimPrefix(uri, publicOrigin), "", nil)
		res, body := sendRequest(t, req)
		r.contract.CheckResponse(t, req, res)
		if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") || !bytes.Equal(body, content) {
			t.Errorf("the download of %s = %d %q, %d bytes; want the file as an attachment", name, res.StatusCode,
				res.Header.Get("Content-Disposition"), len(body))
		}
	}
	// list answers each attachment's type and size, as uploaded.
	want := map[string][2]any{}
	for _, img := range images {
		want["Guide/"+img.name] = [2]any{img.mime, float64(len(img.bytes))}
	}
	for name, file := range files {
		want["Guide/"+name] = [2]any{file.mime, float64(len(file.content))}
	}
	sc, _ := r.entryCall(t, mcpserver.NotebookEntry, "list", map[string]any{"pattern": "Guide/"})["structuredContent"].(map[string]any)
	items, _ := sc["items"].([]any)
	for _, it := range items {
		item, _ := it.(map[string]any)
		path, _ := item["path"].(string)
		if w, ok := want[path]; !ok || item["kind"] != "attachment" || item["mime"] != w[0] || item["byte_size"] != w[1] {
			t.Errorf("list of Guide/ has %v, want an attachment of %v", item, w)
		}
	}
	if len(items) != len(want) {
		t.Errorf("list of Guide/ has %d items, want %d", len(items), len(want))
	}
	gone := r.upload(t, guide, "gone.png", imageOf(t, "png"))
	if err := os.Remove(storedBlobs(t, r.storage)[gone.Blob]); err != nil {
		t.Fatal(err)
	}
	res := r.entryCall(t, mcpserver.NotebookEntry, "read", map[string]any{"path": "Guide/gone.png"})
	if blocks, meta := blocksOf(t, res), metaOf(t, res); len(blocks) != 1 || meta["note"] != "The attachment's file is missing from the server's storage." {
		t.Errorf("read of a PNG whose file went = %v, want its metadata and the note", res)
	}
	if e := r.logged(t, "asset file missing"); e == nil || e["level"] != "WARN" || e["blob_id"] != gone.Blob {
		t.Errorf("the missing file logged %v, want a warning naming its blob", e)
	}
}

// Without server.public_url an image comes all the same, which needs no
// address; another attachment is its metadata and a note; neither has an
// address (M9/P1 design 3.11.2, as fix round 1 of S2 reads it).
func TestTheMCPReadsWithoutAPublicURL(t *testing.T) {
	r := startReadsApp(t, "")
	png := imageOf(t, "png")
	r.upload(t, "", "x.png", png)
	r.upload(t, "", "spec.pdf", []byte("%PDF-1.4\n%%EOF\n"))
	res := r.entryCall(t, mcpserver.NotebookEntry, "read", map[string]any{"path": "x.png"})
	blocks, meta := blocksOf(t, res), metaOf(t, res)
	if len(blocks) != 2 || blocks[1]["type"] != "image" || blocks[1]["mimeType"] != "image/png" ||
		blocks[1]["data"] != base64.StdEncoding.EncodeToString(png) || meta["url"] != nil || meta["expires_at"] != nil ||
		meta["width"] != 2.0 || meta["note"] != nil {
		t.Errorf("read of a PNG without public_url = %v, want its metadata and its image", res)
	}
	res = r.entryCall(t, mcpserver.AccountEntry, "read", map[string]any{"path": "spec.pdf"})
	blocks, meta = blocksOf(t, res), metaOf(t, res)
	if len(blocks) != 1 || meta["url"] != nil || meta["expires_at"] != nil ||
		meta["note"] != "server.public_url is not set, so there is no address to download it; ask the server's admin." {
		t.Errorf("read of a PDF without public_url = %v, want its metadata and the note", res)
	}
}

// server.public_url's trailing "/" is not its addresses': an attachment's
// link is the origin and the download's path, one "/" between (M9/P1
// design 3.12; S2 review, B-M3).
func TestTheMCPReadsWithAPublicURLEndingInASlash(t *testing.T) {
	r := startReadsApp(t, publicOrigin+"/")
	r.upload(t, "", "spec.pdf", []byte("%PDF-1.4\n%%EOF\n"))
	res := r.entryCall(t, mcpserver.NotebookEntry, "read", map[string]any{"path": "spec.pdf"})
	blocks, meta := blocksOf(t, res), metaOf(t, res)
	link, _ := meta["url"].(string)
	if len(blocks) != 2 || blocks[1]["uri"] != link || !strings.HasPrefix(link, publicOrigin+"/api/v0/assets/") ||
		strings.Contains(strings.TrimPrefix(link, "http://"), "//") {
		t.Errorf("read with public_url %q = %v, want a link at the origin and one slash", publicOrigin+"/", res)
	}
}

// Each error an MCP tool answers carries its code, one its tool's row of
// mcpToolCodes lists, which the contract declares (M9 design 4.7;
// mcpAnswer.toolError checks both): the arguments' schema, bad_request; a
// cursor that decodes into no payload, bad_request; a pattern's, a path's
// or a window's value, validation_failed; a page's children or a path
// that names nothing, page.not_found; the account entry's notebook not
// seen, notebook.not_found, before any path in it is looked up. Each code
// of every row of mcpToolCodes is answered here: a tool's row comes with
// its calls here (S2 fix check C). Neither list nor read binds to one
// REST operation: list answers what listNodes and listAssets do, read
// what getPageContent and the notebook's decision do, and their arguments
// are their own. S3's notebooks refuses only an argument it does not
// take; open_notebook a call without its notebook or with an argument it
// does not take, and a notebook not seen, deleted, of none or no UUID
// (M9/P1 design 3.17). P2's writes answer each code of the REST writes
// they are: the reads' app's calls, a reader's forbidden, and server_busy
// with the parse budget held (writeCodeCalls, carolsWrites, busyWrites).
func TestTheMCPToolsErrorsCarryTheirCodes(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	guide := r.createPage(t, "", "Guide", "a\n")
	unseen := uuid.NewV7().String()
	answered := answeredCodes{}
	calls := []codeCall{
		{atNotebook(r.notebook), "list", map[string]any{"limit": "two"}, "bad_request"},
		{atNotebook(r.notebook), "list", map[string]any{"cursor": "nope"}, "bad_request"},
		{atNotebook(r.notebook), "list", map[string]any{"limit": 0}, "validation_failed"},
		{atNotebook(r.notebook), "list", map[string]any{"pattern": "a/./b"}, "validation_failed"},
		{atNotebook(r.notebook), "list", map[string]any{"pattern": "Missing/"}, "page.not_found"},
		{mcpserver.Pattern, "list", map[string]any{"notebook": unseen}, "notebook.not_found"},
		{atNotebook(r.notebook), "read", map[string]any{}, "bad_request"},
		{atNotebook(r.notebook), "read", map[string]any{"path": "Guide/"}, "validation_failed"},
		{atNotebook(r.notebook), "read", map[string]any{"path": "Guide.md", "offset": 0}, "validation_failed"},
		{atNotebook(r.notebook), "read", map[string]any{"path": "Guide.md", "column": 9}, "validation_failed"},
		{atNotebook(r.notebook), "read", map[string]any{"path": "Missing.md"}, "page.not_found"},
		{mcpserver.Pattern, "read", map[string]any{"notebook": unseen, "path": "Guide.md"}, "notebook.not_found"},
		// bob's notebook, which alice does not see: the notebook before the
		// path, whether a node is there or not.
		{mcpserver.Pattern, "list", map[string]any{"notebook": r.hidden, "pattern": "Missing/"}, "notebook.not_found"},
		{mcpserver.Pattern, "read", map[string]any{"notebook": r.hidden, "path": "Missing.md"}, "notebook.not_found"},
		{mcpserver.Pattern, "notebooks", map[string]any{"notebook": r.notebook}, "bad_request"},
		{mcpserver.Pattern, "open_notebook", map[string]any{}, "bad_request"},
		{mcpserver.Pattern, "open_notebook", map[string]any{"notebook": r.notebook, "pattern": "/"}, "bad_request"},
		{mcpserver.Pattern, "open_notebook", map[string]any{"notebook": r.hidden}, "notebook.not_found"},
		{mcpserver.Pattern, "open_notebook", map[string]any{"notebook": r.deleted}, "notebook.not_found"},
		{mcpserver.Pattern, "open_notebook", map[string]any{"notebook": unseen}, "notebook.not_found"},
		{mcpserver.Pattern, "open_notebook", map[string]any{"notebook": "nope"}, "notebook.not_found"},
	}
	for _, tt := range append(calls, writeCodeCalls(t, r, guide)...) {
		member := r.call(t, tt.path, tt.tool, tt.args).toolError(t)
		if member["code"] != tt.want {
			t.Errorf("%s %v at %s = %v, want %s", tt.tool, tt.args, tt.path, member, tt.want)
			continue
		}
		answered.add(tt.tool, tt.want)
	}
	r.carolsWrites(t, answered)
	busyWrites(t, answered)
	for tool, codes := range mcpToolCodes() {
		for _, code := range codes {
			if !answered[tool][code] {
				t.Errorf("%s's row lists %s, which no call here answers", tool, code)
			}
		}
	}
}

// asset's errors are page's to read, which asked for a path (M9/P1 design
// 3.11.2): asset.not_found, wrapped or not, page.not_found; a missing
// file page's; any other as it is (M9/P1 S2 review, B-M3 and C-M3).
func TestPageAttachmentErrors(t *testing.T) {
	boom := errors.New("boom")
	for _, tt := range []struct{ err, want error }{
		{asset.ErrNotFound, page.ErrNotFound},
		{fmt.Errorf("read: %w", asset.ErrNotFound), page.ErrNotFound},
		{asset.ErrNoFile, page.ErrNoAttachmentFile},
		{fmt.Errorf("open: %w", asset.ErrNoFile), page.ErrNoAttachmentFile},
		{boom, boom},
		{nil, nil},
	} {
		if got := pageAttachmentErr(tt.err); got != tt.want { //nolint:errorlint // the very error, mapped or not
			t.Errorf("pageAttachmentErr(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

// A NUL in read's path or list's pattern is 422 invalid_format on the
// argument, which no title holds, not internal_error (M9/P1 design 3.13).
func TestTheMCPReadsRefuseANul(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	for _, c := range []struct{ tool, field string }{{"read", "path"}, {"list", "pattern"}} {
		member := r.call(t, atNotebook(r.notebook), c.tool, map[string]any{c.field: "a\x00b.md"}).toolError(t)
		fields, _ := member["fields"].([]any)
		if member["code"] != "validation_failed" || len(fields) != 1 ||
			fields[0].(map[string]any)["field"] != c.field || fields[0].(map[string]any)["code"] != "invalid_format" {
			t.Errorf("%s with a NUL = %v, want 422 invalid_format on %s", c.tool, member, c.field)
		}
	}
}

// stringParameters are the string parameters of a tool's input schema,
// as the paths a call's arguments reach them by: a property whose type is
// string or holds it, and, within an object or an array's items, its
// own, "ops[].path" for path in the objects of the array ops.
func stringParameters(schema map[string]any) []string {
	var out []string
	var walk func(s map[string]any, prefix string)
	walk = func(s map[string]any, prefix string) {
		props, _ := s["properties"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(props)) {
			p, _ := props[name].(map[string]any)
			switch {
			case typeHolds(p, "string"):
				out = append(out, prefix+name)
			case typeHolds(p, "object"):
				walk(p, prefix+name+".")
			case typeHolds(p, "array"):
				if items, ok := p["items"].(map[string]any); ok {
					if typeHolds(items, "string") {
						out = append(out, prefix+name+"[]")
					} else {
						walk(items, prefix+name+"[].")
					}
				}
			}
		}
	}
	walk(schema, "")
	return out
}

// typeHolds reports whether s's type is want or a list holding it.
func typeHolds(s map[string]any, want string) bool {
	switch t := s["type"].(type) {
	case string:
		return t == want
	case []any:
		return slices.Contains(t, any(want))
	}
	return false
}

// withArgumentAt is a deep copy of args holding value at path, a
// stringParameters path: the objects on the way kept or made, ops[].path
// the first op's path, an op made when there is none (M9/P2 design
// 3.13.1).
func withArgumentAt(args map[string]any, path string, value any) map[string]any {
	out := clonedArgs(args)
	parts := strings.Split(path, ".")
	at := out
	for i, part := range parts {
		name, inArray := strings.CutSuffix(part, "[]")
		switch last := i == len(parts)-1; {
		case last && inArray:
			at[name] = []any{value}
		case last:
			at[name] = value
		case inArray:
			items, _ := at[name].([]any)
			if len(items) == 0 {
				items = []any{map[string]any{}}
			}
			first, ok := items[0].(map[string]any)
			if !ok {
				first = map[string]any{}
				items[0] = first
			}
			at[name], at = items, first
		default:
			next, ok := at[name].(map[string]any)
			if !ok {
				next = map[string]any{}
				at[name] = next
			}
			at = next
		}
	}
	return out
}

// clonedArgs is a deep copy of args, as JSON has them.
func clonedArgs(args map[string]any) map[string]any {
	var out map[string]any
	b, _ := json.Marshal(args)
	if _ = json.Unmarshal(b, &out); out == nil {
		out = map[string]any{}
	}
	return out
}

// argumentAt is the value args hold at path, as withArgumentAt puts it.
func argumentAt(args map[string]any, path string) any {
	var v any = args
	for _, part := range strings.Split(path, ".") {
		name, inArray := strings.CutSuffix(part, "[]")
		object, _ := v.(map[string]any)
		v = object[name]
		if inArray {
			items, _ := v.([]any)
			if len(items) == 0 {
				return nil
			}
			v = items[0]
		}
	}
	return v
}

// The derivation finds a string parameter wherever an argument holds one:
// on a synthetic schema whose one string is in ops[].path (the shape of
// P3's batch), it finds it, and the arguments it builds reach it, beside
// the others, which are kept.
func TestTheStringParametersAreDerivedFromTheSchema(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(`{"type":"object","properties":{"limit":{"type":["null","integer"]},
		"ops":{"type":"array","items":{"type":"object","properties":{"path":{"type":"string"},"n":{"type":"integer"}}}}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	if got := stringParameters(schema); !slices.Equal(got, []string{"ops[].path"}) {
		t.Fatalf("stringParameters = %v, want ops[].path", got)
	}
	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{nil, `{"ops":[{"path":"x"}]}`},
		{map[string]any{"limit": 2, "ops": []any{map[string]any{"path": "a", "n": 1}, map[string]any{"path": "b"}}},
			`{"limit":2,"ops":[{"n":1,"path":"x"},{"path":"b"}]}`},
	} {
		got := withArgumentAt(tt.args, "ops[].path", "x")
		if b, _ := json.Marshal(got); string(b) != tt.want || argumentAt(got, "ops[].path") != "x" {
			t.Errorf("withArgumentAt(%v, ops[].path) = %s, want %s", tt.args, b, tt.want)
		}
	}
	if b, _ := json.Marshal(withArgumentAt(map[string]any{"s": "a"}, "t", "x")); string(b) != `{"s":"a","t":"x"}` {
		t.Errorf("withArgumentAt kept %s", b)
	}
}

// validToolArguments are, for each tool, arguments that reach its use
// case in alice's notebook, where Strings.md holds "x" (M9/P2 design
// 3.13.1): a tool listed without them fails the check, so that P3's and
// P4's tools add theirs. The account entry's notebook is added to them.
func validToolArguments() map[string]map[string]any {
	return map[string]map[string]any{
		"notebooks":     {},
		"open_notebook": {},
		"list":          {},
		"read":          {"path": "Strings.md"},
		"write":         {"path": "Strings.md", "content": "x"},
		"edit":          {"path": "Strings.md", "old_string": "x", "new_string": "y"},
		"move":          {"from": "Strings.md", "to": "Moved.md"},
		"delete":        {"path": "Gone.md"},
	}
}

// schemaRefusal starts the text of the arguments' refusal by the tool's
// input schema (mcpserver's argumentsError), which no use case answers.
const schemaRefusal = "bad_request: The arguments do not match the tool's input schema."

// nul is what each string parameter is given, its argument checked
// before the call.
const nul = "a\x00b"

// toolStringProblems are the ways a NUL in a tool's string argument fails
// to reach its use case (M9/P2 design 3.13.1): for each tool lists has,
// each string parameter of its schema, given nul in a copy of the tool's
// valid arguments, the account entry's notebook among them first, and
// sent by call: a tool of no valid arguments; arguments that do not hold
// the NUL where they should; internal_error; and the schema's refusal,
// which a call that reaches its use case never answers (a use case's 400
// of a bad cursor, or its 422, does).
func toolStringProblems(t *testing.T, lists toolLists, valid map[string]map[string]any, notebook string,
	call func(entry, tool string, args map[string]any) mcpAnswer,
) []string {
	t.Helper()
	var problems []string
	for _, entry := range slices.Sorted(maps.Keys(lists)) {
		for _, tool := range lists[entry] {
			object, _ := tool.(map[string]any)
			name, _ := object["name"].(string)
			schema, _ := object["inputSchema"].(map[string]any)
			args, ok := valid[name]
			if !ok {
				problems = append(problems, fmt.Sprintf("the %s entry's %s has no valid arguments", entry, name))
				continue
			}
			props, _ := schema["properties"].(map[string]any)
			base := clonedArgs(args)
			if _, takes := props["notebook"]; takes && mcpEntries()[entry] == mcpserver.AccountEntry {
				base["notebook"] = notebook
			}
			for _, param := range stringParameters(schema) {
				sent := withArgumentAt(base, param, nul)
				if argumentAt(sent, param) != nul {
					problems = append(problems, fmt.Sprintf("the %s entry's %s: the arguments %v do not hold the NUL at %s", entry, name, sent, param))
					continue
				}
				a := call(entry, name, sent)
				switch r := a.result(t); {
				case mcpCellOf(t, a).code == "internal_error":
					problems = append(problems, fmt.Sprintf("the %s entry's %s with a NUL in %s = internal_error", entry, name, param))
				case r["isError"] == true && strings.HasPrefix(fmt.Sprint(a.toolError(t)["message"]), schemaRefusal):
					problems = append(problems, fmt.Sprintf("the %s entry's %s: %s did not reach its use case", entry, name, param))
				}
			}
		}
	}
	return problems
}

// No string argument of any tool answers internal_error, and each reaches
// its use case (M9/P1 design 3.13; M9/P2 design 3.13.1): each string
// parameter that both entries' tools/list tell, NUL in it among the
// tool's valid arguments, is answered by the tool's use case, not by the
// schema; a lone surrogate's escape in it is the edge's 400.
func TestToolStringsDoNotAnswerInternalErrors(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	r.createPage(t, "", "Strings", "x")
	lists := r.listedTools(t)
	pathOf := func(entry string) string {
		if mcpEntries()[entry] == mcpserver.NotebookEntry {
			return atNotebook(r.notebook)
		}
		return mcpserver.Pattern
	}
	cases := 0
	call := func(entry, tool string, args map[string]any) mcpAnswer {
		cases++
		return r.call(t, pathOf(entry), tool, args)
	}
	for _, p := range toolStringProblems(t, lists, validToolArguments(), r.notebook, call) {
		t.Error(p)
	}
	// read's path is a string in both entries: none found means the
	// derivation broke.
	if cases == 0 {
		t.Fatal("no string parameter derived from the tools' lists")
	}
	for _, entry := range slices.Sorted(maps.Keys(lists)) {
		for _, tool := range lists[entry] {
			object, _ := tool.(map[string]any)
			name, _ := object["name"].(string)
			schema, _ := object["inputSchema"].(map[string]any)
			for _, param := range stringParameters(schema) {
				const mark = "lone-surrogate-here"
				args := withArgumentAt(validToolArguments()[name], param, mark)
				if _, ok := args["notebook"]; !ok && pathOf(entry) == mcpserver.Pattern {
					args["notebook"] = r.notebook
				}
				body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
					"params": map[string]any{"name": name, "arguments": args}})
				raw := strings.Replace(string(body), mark, "a"+`\`+"ud800b", 1)
				if a := r.post(t, context.Background(), pathOf(entry), r.pat, raw, afterHandshake()); a.status != http.StatusBadRequest {
					t.Errorf("%s %s with a lone surrogate in %s = %d %s, want 400", entry, name, param, a.status, a.body)
				}
			}
		}
	}
}

// strictArgs are the seam tool strict's arguments: two strings it needs.
type strictArgs struct {
	Must string `json:"must" jsonschema:"Required."`
	S    string `json:"s" jsonschema:"Required too."`
}

// The check catches each way a string's call does not reach its use case
// (M9/P2 design 3.13.1, reverse controls): beside the modules' tools, the
// seam tool strict needs must and s; given strict's valid arguments
// without must, s's call is the schema's refusal, and only it; without
// strict's arguments, strict has none. The problems are exactly those:
// a check that put the NUL alone in the arguments would add must's.
func TestToolStringsCatchASchemaRefusal(t *testing.T) {
	strict := mcpserver.NewTool(mcpserver.Spec{Name: "strict", Title: "Strict", Description: "Needs must and s.",
		Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.AccountEntry},
		func(context.Context, mcpserver.Call, strictArgs) (mcpserver.Result, error) {
			return mcpserver.Result{Structured: map[string]any{}}, nil
		})
	db := pgtest.NewDatabase(t)
	logs := &syncBuffer{}
	m := runMCPApp(t, wireMCPApp(t, testConfig(t, db, false), logs, []mcpserver.Tool{strict}), db, logs)
	r := readsApp{mcpApp: m}
	r.createPage(t, "", "Strings", "x")
	lists := r.listedTools(t)
	call := func(entry, tool string, args map[string]any) mcpAnswer {
		path := mcpserver.Pattern
		if mcpEntries()[entry] == mcpserver.NotebookEntry {
			path = atNotebook(r.notebook)
		}
		return r.call(t, path, tool, args)
	}
	valid := validToolArguments()
	valid["strict"] = map[string]any{"s": "x"}
	if got, want := toolStringProblems(t, lists, valid, r.notebook, call), []string{"the account entry's strict: s did not reach its use case"}; !slices.Equal(got, want) {
		t.Errorf("the problems of strict without must = %q, want %q", got, want)
	}
	delete(valid, "strict")
	if got, want := toolStringProblems(t, lists, valid, r.notebook, call), []string{"the account entry's strict has no valid arguments"}; !slices.Equal(got, want) {
		t.Errorf("the problems of strict without its arguments = %q, want %q", got, want)
	}
}
