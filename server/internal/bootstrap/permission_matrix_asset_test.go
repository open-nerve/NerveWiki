package bootstrap

import (
	"net/http"
	"testing"
)

// The asset module's rows (M7/P2 design 3.12), by the notebook columns: an
// upload aims at its notebook (notebookOf), under its page in it (pageOf).
// The editors and admins upload, the readers are refused; the rest do not
// see the notebook.

// pngHead is the PNG signature: a file the server takes for a PNG.
const pngHead = "\x89PNG\r\n\x1a\n"

// uploadForm is the form of an upload of pic.png, the PNG signature alone,
// under parent, its boundary "matrix".
func uploadForm(parent string) string {
	return "--matrix\r\nContent-Disposition: form-data; name=\"parent_id\"\r\n\r\n" + parent +
		"\r\n--matrix\r\nContent-Disposition: form-data; name=\"file\"; filename=\"pic.png\"\r\n" +
		"Content-Type: application/octet-stream\r\n\r\n" + pngHead + "\r\n--matrix--\r\n"
}

func assetMatrixRows() []matrixRow {
	return []matrixRow{
		{
			op:          "uploadAsset",
			columns:     notebookColumns(),
			write:       true,
			contentType: "multipart/form-data; boundary=matrix",
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() + "/assets",
					uploadForm(s.page(pageOf(c)).String())
			},
			cells: editorsOnly(cellCreated(), cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var a struct {
					Name     string `json:"name"`
					ParentID string `json:"parent_id"`
					Mime     string `json:"mime"`
					ByteSize int    `json:"byte_size"`
				}
				decodeAnswer(t, answer, &a)
				if a.Name != "pic.png" || a.ParentID != s.page(pageOf(c)).String() || a.Mime != "image/png" || a.ByteSize != len(pngHead) {
					t.Errorf("uploaded %+v, want pic.png, a PNG of %d bytes, under %s", a, len(pngHead), pageOf(c))
				}
			},
		},
	}
}
