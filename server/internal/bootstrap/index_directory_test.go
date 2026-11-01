package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"
)

type directoryType struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}
type directoryItem struct {
	ID       string   `json:"page_id"`
	ParentID *string  `json:"parent_id"`
	Title    string   `json:"title"`
	Type     *string  `json:"type"`
	Tags     []string `json:"tags"`
}
type directoryResponse struct {
	Data     []directoryItem `json:"data"`
	Types    []directoryType `json:"types"`
	Complete bool            `json:"types_complete"`
	Long     int64           `json:"unfilterable_type_pages"`
	Mode     string          `json:"mode"`
	Next     *string         `json:"next_cursor"`
	Source   *struct {
		RootID  string `json:"root_id"`
		Sources int64  `json:"sources"`
	} `json:"source_area"`
}

func TestDirectoryIndex(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	t.Run("rest_mcp", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Directory ordinary")
		a := tm.createPageWith(t, "alice", nb, "", "Alpha1", "---\ntype: alpha\n---\n1")
		b := tm.createPageWith(t, "alice", nb, "", "Alpha2", "---\ntype: alpha\n---\n2")
		c := tm.createPageWith(t, "alice", nb, a, "Beta.md", "---\ntype: Beta\n---\n3")
		d := tm.createPageWith(t, "alice", nb, "", "Untyped", "4")
		path := "/api/v0/notebooks/" + nb + "/index"
		var all directoryResponse
		tm.get(t, "alice", path, &all)
		ids := []string{}
		for _, it := range all.Data {
			ids = append(ids, it.ID)
		}
		if !slices.Equal(ids, []string{a, b, c, d}) || !reflect.DeepEqual(all.Types, []directoryType{{"alpha", 2}, {"Beta", 1}}) || !all.Complete || all.Mode != "standard" || all.Source != nil {
			t.Fatalf("ordinary directory=%+v", all)
		}
		if all.Data[2].Title != "Beta.md" || all.Data[2].ParentID == nil || *all.Data[2].ParentID != a || all.Data[0].ParentID != nil {
			t.Fatalf("stored metadata=%+v", all.Data)
		}
		m := tm.agent(t, "alice", nb)
		for _, scope := range []string{"notebook", "account"} {
			got := answerOf[mcpIndexAnswer](t, sourceMCPCall(t, m, scope, "index", map[string]any{}))
			if !slices.Equal(indexPaths(got.Pages), []string{"Alpha1.md", "Alpha2.md", "Alpha1/Beta.md.md", "Untyped.md"}) {
				t.Fatalf("%s ordering=%v", scope, indexPaths(got.Pages))
			}
		}
		var filtered directoryResponse
		tm.get(t, "alice", path+"?parent_id="+a, &filtered)
		if len(filtered.Data) != 1 || filtered.Data[0].ID != c || !reflect.DeepEqual(filtered.Types, all.Types) {
			t.Fatalf("parent directory=%+v", filtered)
		}
		tm.get(t, "alice", path+"?at_root=true", &filtered)
		if len(filtered.Data) != 3 {
			t.Fatalf("root directory=%+v", filtered)
		}
		// Page validation does not admit assets, other notebooks, or deleted pages.
		asset := tm.upload(t, "alice", nb, "", "cover.txt", "asset")
		other := tm.openNotebook(t, "alice", "Directory other")
		foreign := tm.createPage(t, "alice", other, "", "Other")
		gone := tm.createPage(t, "alice", nb, "", "Gone")
		status, body := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/nodes/"+gone, tm.tokens["alice"], "")
		if status != 204 {
			t.Fatalf("delete fixture=%d %s", status, body)
		}
		for _, parent := range []string{asset.ID, foreign, gone, uuid.NewV7().String()} {
			status, body = ask(t, tm.contract, http.MethodGet, tm.base+path+"?parent_id="+parent, tm.tokens["alice"], "")
			if status != 404 || problemCode(t, body) != "page.not_found" {
				t.Fatalf("parent %s=%d %s", parent, status, body)
			}
		}
		for _, c := range []struct {
			q      string
			status int
			code   string
		}{{"?cursor=bad&type=", 400, "bad_request"}, {"?type=", 422, "validation_failed"}, {"?limit=0", 422, "validation_failed"}, {"", 404, "notebook.not_found"}, {"?parent_id=" + a + "&at_root=true", 422, "validation_failed"}} {
			status, body = ask(t, tm.contract, http.MethodGet, tm.base+path+c.q, tm.tokens["dana"], "")
			if status != c.status || problemCode(t, body) != c.code {
				t.Fatalf("hidden %s=%d %s", c.q, status, body)
			}
		}
	})
	t.Run("exclude", func(t *testing.T) {
		nb := tm.llmWiki(t, "Directory sources")
		s := tm.createPageWith(t, "alice", nb.id, nb.raw, "Source", "---\ntype: source\n---\nsource body")
		tm.createPage(t, "alice", nb.id, nb.raw, "EmptySource")
		tm.upload(t, "alice", nb.id, nb.raw, "source.txt", "source asset")
		w := tm.createPageWith(t, "alice", nb.id, "", "Wiki", "---\ntype: topic\n---\nwiki body")
		empty := tm.createPage(t, "alice", nb.id, "", "Empty")
		path := "/api/v0/notebooks/" + nb.id + "/index"
		var all directoryResponse
		tm.get(t, "alice", path, &all)
		if len(all.Data) != 4 || !reflect.DeepEqual(all.Types, []directoryType{{"topic", 1}}) || all.Source == nil || all.Source.RootID != nb.raw || all.Source.Sources != 2 || all.Mode != "llm_wiki" {
			t.Fatalf("LLM directory=%+v", all)
		}
		foundRoot, foundWiki := false, false
		for _, it := range all.Data {
			if it.ID == s {
				t.Fatal("source page was listed")
			}
			foundRoot = foundRoot || it.ID == nb.raw
			foundWiki = foundWiki || it.ID == w
		}
		if !foundRoot || !foundWiki {
			t.Fatal("root/wiki disappeared")
		}
		for _, q := range []string{"?parent_id=" + empty, "?tag=a+b", "?tag=absent", "?type=absent"} {
			var got directoryResponse
			tm.get(t, "alice", path+q, &got)
			if len(got.Data) != 0 || got.Data == nil || got.Next != nil || !reflect.DeepEqual(got.Types, []directoryType{{"topic", 1}}) || !got.Complete || got.Long != 0 || got.Source == nil || got.Source.Sources != 2 || got.Mode != "llm_wiki" {
				t.Fatalf("empty response %s=%+v", q, got)
			}
		}
		m := tm.agent(t, "alice", nb.id)
		for _, scope := range []string{"notebook", "account"} {
			got := answerOf[mcpIndexAnswer](t, sourceMCPCall(t, m, scope, "index", map[string]any{}))
			if len(got.Pages) != 6 || !slices.Contains(indexPaths(got.Pages), "raw/Source.md") {
				t.Fatalf("%s real directory=%v", scope, indexPaths(got.Pages))
			}
		}
	})
	t.Run("stored_value_normalization", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Directory normalized")
		id := tm.createPageWith(t, "alice", nb, "", "Normalized", "---\ntype: \"a\\0b\"\n---\nreal Markdown")
		var stored string
		if err := tm.pool.QueryRow(t.Context(), `SELECT value #>> '{}' FROM page_properties WHERE source_id=$1 AND key='type'`, id).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != "a\ufffdb" {
			t.Fatalf("stored type=%q", stored)
		}
		path := "/api/v0/notebooks/" + nb + "/index"
		var got directoryResponse
		tm.get(t, "alice", path, &got)
		if !reflect.DeepEqual(got.Types, []directoryType{{"a\ufffdb", 1}}) || !got.Complete || got.Long != 0 {
			t.Fatalf("normalized catalogue=%+v", got)
		}
		tm.get(t, "alice", path+"?type="+url.QueryEscape("a\ufffdb"), &got)
		if len(got.Data) != 1 || got.Data[0].ID != id {
			t.Fatalf("normalized exact filter=%+v", got)
		}
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+path+"?type="+url.QueryEscape("a\x00b"), tm.tokens["alice"], "")
		if status != 422 || problemCode(t, body) != "validation_failed" {
			t.Fatalf("NUL filter=%d %s", status, body)
		}
	})
	t.Run("catalogue_bounds", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Directory JSON")
		for n := 0; n < 8; n++ {
			value := string(rune('a'+n)) + strings.Repeat("<", 1000)
			tm.createPageWith(t, "alice", nb, "", string(rune('A'+n)), "---\ntype: "+value+"\n---\n")
		}
		var got directoryResponse
		tm.get(t, "alice", "/api/v0/notebooks/"+nb+"/index", &got)
		encoded, err := json.Marshal(got.Types)
		if err != nil || len(got.Types) != 5 || len(encoded) > 32*1024 || got.Complete || got.Long != 0 || len(got.Data) != 8 {
			t.Fatalf("actual JSON bytes=%d result=%+v err=%v", len(encoded), got, err)
		}
	})
}
