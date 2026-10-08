package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The attachments through serve (M7/P2 design 5): uploaded through the
// API, their rows and files checked against the database and the store.

// assetUpload is by's upload of a file named name holding content into the
// notebook nb, under the page parent, at the root when it is "".
func assetUpload(t *testing.T, by, nb, parent, name, content string) step {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if parent != "" {
		if err := w.WriteField("parent_id", parent); err != nil {
			t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return step{by: by, method: http.MethodPost, path: "/api/v0/notebooks/" + nb + "/assets", body: body.String(),
		contentType: w.FormDataContentType()}
}

// uploadedAsset is an upload's answer, as much as the tests read: the
// node, its file's id, its addresses and its time.
type uploadedAsset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentURL  string    `json:"content_url"`
	DownloadURL string    `json:"download_url"`
	CreatedAt   time.Time `json:"created_at"`
	Blob        string    `json:"-"`
}

// upload uploads as assetUpload does, checks the invariants and answers
// the attachment.
func (tm acmeTeam) upload(t *testing.T, by, nb, parent, name, content string) uploadedAsset {
	t.Helper()
	c := assetUpload(t, by, nb, parent, name, content)
	status, answer := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.contentType, c.body)
	if status != http.StatusCreated {
		t.Fatalf("upload %s as %s = %d %s", name, by, status, answer)
	}
	return tm.asset(t, answer)
}

// asset reads an Asset answer, its file's id from its address.
func (tm acmeTeam) asset(t *testing.T, answer string) uploadedAsset {
	t.Helper()
	var a uploadedAsset
	if err := json.Unmarshal([]byte(answer), &a); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(a.ContentURL)
	if err != nil {
		t.Fatal(err)
	}
	a.Blob = u.Query().Get("b")
	checkAssets(t, tm.pool, tm.storage)
	return a
}

// download gets the content at an attachment's address, with no token.
func (tm acmeTeam) download(t *testing.T, address string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodGet, tm.base+address, "", "")
}

// checkAssets fails t when the attachments break an invariant (M7/P2
// design 3.12): an attachment's node not deleted without exactly one row
// not deleted; a deleted one with a row not deleted; a row of a page's
// node; a row whose file is not in the store at dir; a file there that no
// row holds, which an upload a unit refused deletes, and only the sweep's
// test seeds.
func checkAssets(t *testing.T, pool *pgxpool.Pool, dir string) {
	t.Helper()
	for what, query := range map[string]string{
		"not deleted without exactly one row not deleted": `SELECT count(*) FROM nodes n WHERE n.kind = 'asset' AND n.deleted_at IS NULL
			AND (SELECT count(*) FROM asset_blobs b WHERE b.node_id = n.id AND b.deleted_at IS NULL) <> 1`,
		"deleted with a row not deleted": `SELECT count(*) FROM nodes n WHERE n.kind = 'asset' AND n.deleted_at IS NOT NULL
			AND EXISTS (SELECT 1 FROM asset_blobs b WHERE b.node_id = n.id AND b.deleted_at IS NULL)`,
		"that are pages, with a row": `SELECT count(*) FROM asset_blobs b JOIN nodes n ON n.id = b.node_id WHERE n.kind <> 'asset'`,
	} {
		if n := count(t, pool, query); n != 0 {
			t.Errorf("%d attachments' nodes %s, want none", n, what)
		}
	}
	files := storedBlobs(t, dir)
	for _, id := range queryStrings(t, pool, "SELECT id::text FROM asset_blobs ORDER BY id") {
		if _, ok := files[id]; !ok {
			t.Errorf("the file of the row %s is not in the store", id)
		}
		delete(files, id)
	}
	if len(files) != 0 {
		t.Errorf("the files %q in the store, no row holding them; want none", slices.Sorted(maps.Keys(files)))
	}
}

// storedBlobs maps the name of each file committed to the blobs area of
// the store at dir, its blob's id, to its path. It walks the directory
// rather than open a store there, which would delete the running app's
// files being written.
func storedBlobs(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(dir, "blobs"), func(path string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".tmp":
			return fs.SkipDir
		case d.Type().IsRegular():
			files[d.Name()] = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A file name that is not UTF-8, as filename* may spell one, is 422 on
// the name before the file is read: no node, no row, no file.
func TestAnUploadNamedOutsideUTF8IsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename*=UTF-8''%FF.png`}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(pngFile)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	status, answer := askTyped(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/assets", tm.tokens["alice"],
		w.FormDataContentType(), body.String())
	if status != http.StatusUnprocessableEntity || problemCode(t, answer) != "validation_failed" || !strings.Contains(answer, `"field":"name"`) {
		t.Errorf("upload = %d %s, want 422 validation_failed on the name", status, answer)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE kind = 'asset'"); n != 0 || len(storedBlobs(t, tm.storage)) != 0 {
		t.Errorf("%d attachments' nodes, files %v; want none", n, storedBlobs(t, tm.storage))
	}
	checkAssets(t, tm.pool, tm.storage)
}

// A file far larger than what the upload reads before it, and than the
// parser's buffers, goes up and comes back whole, a range of it too.
func TestALargeFileUploadsAndDownloadsWhole(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	content := make([]byte, 3<<20+7)
	for i := range content {
		content[i] = byte(i*31 + i>>8)
	}
	a := tm.upload(t, "alice", nb, "", "big.bin", string(content))
	if status, body := tm.download(t, a.ContentURL); status != http.StatusOK || body != string(content) {
		t.Errorf("download = %d, %d bytes; want 200, the %d bytes uploaded", status, len(body), len(content))
	}
	req, err := http.NewRequest(http.MethodGet, tm.base+a.ContentURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=2000000-2000009")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	got, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != http.StatusPartialContent || !bytes.Equal(got, content[2000000:2000010]) {
		t.Errorf("a range = %d %v, %v; want 206, those 10 bytes", res.StatusCode, got, err)
	}
}
