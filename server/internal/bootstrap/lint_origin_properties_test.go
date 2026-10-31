package bootstrap

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

type originPropertyLink struct {
	Key    string  `json:"key"`
	NodeID *string `json:"node_id"`
	Kind   *string `json:"kind"`
	URL    *string `json:"url"`
	Inline *bool   `json:"inline"`
}

// Empty paths pass through the real parser, writer, REST projection and
// attachment signer. A named path and a body link cannot stand in for it.
func TestTheLintOriginProperties(t *testing.T) {
	t.Run("empty-key", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		target := tm.createPage(t, "alice", nb, "", "Target")
		asset := tm.upload(t, "alice", nb, "", "x.png", pngFile)
		for _, kind := range []string{"page", "unresolved", "asset"} {
			t.Run(kind, func(t *testing.T) {
				name := "Target"
				var id *string
				var resolvedKind *string
				if kind == "page" {
					id = &target
					resolvedKind = &kind
				}
				if kind == "unresolved" {
					name = "Missing"
				}
				if kind == "asset" {
					name = "x.png"
					id = &asset.ID
					resolvedKind = &kind
				}
				content := "---\n\"\": '[[" + name + "]]'\nnamed: '[[Target]]'\n---\n[[Target]]\n"
				src := tm.createPageWith(t, "alice", nb, "", "Source "+kind, content)
				var got struct {
					Valid          bool                 `json:"valid"`
					Links          []originPropertyLink `json:"links"`
					AssetsExpireAt *time.Time           `json:"assets_expire_at"`
				}
				tm.get(t, "bob", "/api/v0/pages/"+src+"/properties", &got)
				if !got.Valid || len(got.Links) != 2 {
					t.Fatalf("properties = %+v; want empty and named links only", got)
				}
				empty, named := got.Links[0], got.Links[1]
				pageKind := "page"
				if empty.Key != "" || !reflect.DeepEqual(empty.NodeID, id) || !reflect.DeepEqual(empty.Kind, resolvedKind) ||
					!reflect.DeepEqual(named, originPropertyLink{Key: "named", NodeID: &target, Kind: &pageKind}) {
					t.Fatalf("exact property paths and kinds = %+v", got.Links)
				}
				if kind != "asset" {
					if empty.URL != nil || empty.Inline != nil || got.AssetsExpireAt != nil {
						t.Errorf("non-asset signed: %+v", got)
					}
					return
				}
				if empty.URL == nil || empty.Inline == nil || !*empty.Inline || got.AssetsExpireAt == nil || !got.AssetsExpireAt.After(time.Now()) {
					t.Fatalf("asset was not signed: %+v", got)
				}
				if status, body := tm.download(t, *empty.URL); status != http.StatusOK || body != pngFile {
					t.Errorf("signed asset = %d %q", status, body)
				}
			})
		}
	})
}
