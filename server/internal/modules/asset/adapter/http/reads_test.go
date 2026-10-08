package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// assetAnswer is an Asset answer, as much as the tests check.
type assetAnswer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentURL  string `json:"content_url"`
	DownloadURL string `json:"download_url"`
}

// An attachment reads as the API answers it, its addresses those that
// download it.
func TestGetAssetAnswersTheAttachment(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abc")
	res, body := h.get(t, http.MethodGet, "/api/v0/assets/"+n.ID.String(), "session")
	var a assetAnswer
	_ = json.Unmarshal(body, &a)
	if res.StatusCode != http.StatusOK || a.ID != n.ID.String() || a.Name != "a.png" || a.ContentURL != h.address(n, b, false) ||
		a.DownloadURL != h.address(n, b, true) {
		t.Errorf("getAsset = %d %s, want a.png and its addresses", res.StatusCode, body)
	}
	if res, body := h.get(t, http.MethodGet, "/api/v0/assets/"+uuid.NewV7().String(), "session"); res.StatusCode != http.StatusNotFound ||
		code(body) != "asset.not_found" {
		t.Errorf("getAsset(none) = %d %s, want 404 asset.not_found", res.StatusCode, body)
	}
}

// The list answers a page of the attachments under its parent, by name,
// and the cursor of the next; the last page's is null.
func TestListAssetsPagesTheAttachments(t *testing.T) {
	h := newHarness(t)
	h.attach("c.png", "image/png", "abc")
	h.attach("b.png", "image/png", "abc")
	h.attach("a.png", "image/png", "abc")
	list := "/api/v0/notebooks/" + notebookID().String() + "/assets"
	var page struct {
		Data       []assetAnswer `json:"data"`
		NextCursor *string       `json:"next_cursor"`
	}
	res, body := h.get(t, http.MethodGet, list+"?limit=2", "session")
	if err := json.Unmarshal(body, &page); err != nil || res.StatusCode != http.StatusOK || len(page.Data) != 2 || page.Data[0].Name != "a.png" ||
		page.Data[1].Name != "b.png" || page.NextCursor == nil {
		t.Fatalf("listAssets = %d %s, want a.png, b.png and a cursor", res.StatusCode, body)
	}
	res, body = h.get(t, http.MethodGet, list+"?limit=2&cursor="+url.QueryEscape(*page.NextCursor), "session")
	page.NextCursor = nil
	if err := json.Unmarshal(body, &page); err != nil || res.StatusCode != http.StatusOK || len(page.Data) != 1 || page.Data[0].Name != "c.png" ||
		page.NextCursor != nil {
		t.Errorf("listAssets(the cursor) = %d %s, want c.png, the last page", res.StatusCode, body)
	}
}

// Each failure of the list answers its code.
func TestListAssetsAnswersEachFailure(t *testing.T) {
	h := newHarness(t)
	asset, _ := h.attach("a.png", "image/png", "abc")
	h.nodes.add(app.Node{ID: parentID(), NotebookID: notebookID(), Name: "Intro", NameKey: "intro"})
	list := "/api/v0/notebooks/" + notebookID().String() + "/assets"
	for _, tt := range []struct {
		name, path string
		status     int
		code       string
	}{
		{"a cursor it cannot read", list + "?cursor=nope", http.StatusBadRequest, "bad_request"},
		{"a notebook it cannot see", "/api/v0/notebooks/" + uuid.NewV7().String() + "/assets", http.StatusNotFound, "notebook.not_found"},
		{"a parent that is no node", list + "?parent_id=" + uuid.NewV7().String(), http.StatusNotFound, "page.not_found"},
		{"a parent that is an attachment", list + "?parent_id=" + asset.ID.String(), http.StatusNotFound, "page.not_found"},
		{"a limit of 0", list + "?limit=0", http.StatusUnprocessableEntity, "validation_failed"},
		{"a page's attachments", list + "?parent_id=" + parentID().String(), http.StatusOK, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if res, body := h.get(t, http.MethodGet, tt.path, "session"); res.StatusCode != tt.status || code(body) != tt.code {
				t.Errorf("listAssets = %d %s, want %d %s", res.StatusCode, body, tt.status, tt.code)
			}
		})
	}
}
