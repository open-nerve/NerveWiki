package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The asset module's rows (M7/P2 design 3.12), by the notebook columns:
// each aims at its notebook (notebookOf), at its page in it (pageOf), and
// at that page's attachment (assetOf). Any role reads; the editors and
// admins upload, the readers are refused; the rest do not see the
// notebook.

// matrixAssets are the seeded attachments: one under each notebook
// column's page, named after it, of matrixAssetBytes, its file not
// written.
func matrixAssets() []matrixPage {
	var out []matrixPage
	for _, c := range notebookColumns() {
		a := matrixPage{assetOf(c), notebookOf(c), pageOf(c)}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// matrixAssetBytes is the size of each seeded attachment.
const matrixAssetBytes = 3

// assetOf is the attachment under a notebook column's page.
func assetOf(c caller) string {
	return pageOf(c) + ".png"
}

// pngHead is the PNG signature: a file the server takes for a PNG.
const pngHead = "\x89PNG\r\n\x1a\n"

// uploadForm is the form of an upload of pic.png, the PNG signature alone,
// under parent, its boundary "matrix".
func uploadForm(parent string) string {
	return "--matrix\r\nContent-Disposition: form-data; name=\"parent_id\"\r\n\r\n" + parent +
		"\r\n--matrix\r\nContent-Disposition: form-data; name=\"file\"; filename=\"pic.png\"\r\n" +
		"Content-Type: application/octet-stream\r\n\r\n" + pngHead + "\r\n--matrix--\r\n"
}

// assetAnswer is an Asset answer, as much as the rows check.
type assetAnswer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ParentID   string `json:"parent_id"`
	Mime       string `json:"mime"`
	ByteSize   int    `json:"byte_size"`
	ContentURL string `json:"content_url"`
}

// seenOrNot answers seen to a notebook column with a role in its notebook,
// and notFound to the rest.
func seenOrNot(seen, notFound cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range notebookColumns() {
		cells[c] = notFound
		if _, ok := roleIn(c); ok {
			cells[c] = seen
		}
	}
	return cells
}

func assetMatrixRows() []matrixRow {
	assets := func(c caller, s seeded) string {
		return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() + "/assets"
	}
	return []matrixRow{
		{
			op:          "uploadAsset",
			columns:     notebookColumns(),
			write:       true,
			contentType: "multipart/form-data; boundary=matrix",
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, assets(c, s), uploadForm(s.page(pageOf(c)).String())
			},
			cells: editorsOnly(cellCreated(), cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var a assetAnswer
				decodeAnswer(t, answer, &a)
				if a.Name != "pic.png" || a.ParentID != s.page(pageOf(c)).String() || a.Mime != "image/png" || a.ByteSize != len(pngHead) {
					t.Errorf("uploaded %+v, want pic.png, a PNG of %d bytes, under %s", a, len(pngHead), pageOf(c))
				}
			},
		},
		{
			op:      "getAsset",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/assets/" + s.asset(assetOf(c)).String(), ""
			},
			cells: seenOrNot(cellOK(), cell{http.StatusNotFound, "asset.not_found"}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var a assetAnswer
				decodeAnswer(t, answer, &a)
				if a.Name != assetOf(c) || !strings.HasPrefix(a.ContentURL, "/api/v0/assets/"+a.ID+"/content?b=") {
					t.Errorf("read %+v, want %s and its address", a, assetOf(c))
				}
			},
		},
		{
			op:      "listAssets",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, assets(c, s) + "?parent_id=" + s.page(pageOf(c)).String(), ""
			},
			cells: seenOrNot(cellOK(), cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []assetAnswer }
				decodeAnswer(t, answer, &list)
				if len(list.Data) != 1 || list.Data[0].Name != assetOf(c) {
					t.Errorf("listed %+v, want %s", list.Data, assetOf(c))
				}
			},
		},
	}
}
