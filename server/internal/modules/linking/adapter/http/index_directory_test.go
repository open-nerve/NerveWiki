package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestDirectoryIndex(t *testing.T) {
	t.Run("route", func(t *testing.T) {
		f := &fakes{}
		status, body := call(t, f.serve(t), notebookPath+"/index")
		want := `{"data":[],"mode":"standard","next_cursor":null,"source_area":null,"types":[],"types_complete":true,"unfilterable_type_pages":0}`
		if status != http.StatusOK || body != want {
			t.Fatalf("directory=%d %s want %s", status, body, want)
		}
		if !reflect.DeepEqual(f.got, []any{id(10), app.IndexQuery{Catalogue: true, ExcludeArea: true}}) {
			t.Fatalf("options=%+v", f.got)
		}
	})
	t.Run("required_fields", func(t *testing.T) {
		zero := uuid.UUID{}
		typ, summary := "topic", "summary"
		at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		f := &fakes{directory: &app.IndexPage{Mode: "llm_wiki", Next: "next", SourceArea: &app.IndexSourceArea{RootID: id(11), Sources: 0}, Types: []app.IndexType{{Type: "topic", Count: 2}}, TypesComplete: false, UnfilterableTypePages: 3,
			Items: []app.IndexItem{{PageID: id(12), Title: "a.md", ChangedAt: at}, {PageID: id(13), ParentID: &zero, Title: "Child", Type: &typ, Summary: &summary, Cut: []string{"summary"}, ChangedAt: at, Inbound: 1000}}}}
		status, body := call(t, f.serve(t), notebookPath+"/index?parent_id="+zero.String()+"&at_root=false&type=topic&tag=plan&limit=2&cursor=cursor")
		if status != http.StatusOK {
			t.Fatalf("status=%d %s", status, body)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatal(err)
		}
		rows := got["data"].([]any)
		a := rows[0].(map[string]any)
		b := rows[1].(map[string]any)
		if len(got) != 7 || len(a) != 10 || len(b) != 10 || a["parent_id"] != nil || a["type"] != nil || a["summary"] != nil || a["summary_cut"] != false || a["inbound"] != float64(0) || a["inbound_capped"] != false || !reflect.DeepEqual(a["tags"], []any{}) || a["title"] != "a.md" {
			t.Fatalf("required zero/null values: %s", body)
		}
		if b["parent_id"] != zero.String() || b["summary_cut"] != true || b["inbound_capped"] != true || got["types_complete"] != false || got["unfilterable_type_pages"] != float64(3) || got["source_area"].(map[string]any)["sources"] != float64(0) {
			t.Fatalf("required nonempty values: %s", body)
		}
		limit, cursor, tag := 2, "cursor", "plan"
		if !reflect.DeepEqual(f.got, []any{id(10), app.IndexQuery{ParentID: &zero, Type: &typ, Tag: &tag, Limit: &limit, Cursor: &cursor, Catalogue: true, ExcludeArea: true}}) {
			t.Fatalf("bound options=%+v", f.got)
		}
	})
	t.Run("binding", func(t *testing.T) {
		for _, q := range []string{"parent_id=bad", "at_root=maybe", "limit=no"} {
			f := &fakes{}
			status, body := call(t, f.serve(t), notebookPath+"/index?"+q)
			if status != 400 || f.got != nil {
				t.Fatalf("binding %s=%d %s got=%v", q, status, body, f.got)
			}
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, c := range []struct {
			err    error
			status int
			code   string
		}{{shared.InvalidCursor(), 400, "bad_request"}, {domain.ErrNotebookNotFound, 404, "notebook.not_found"}, {domain.ErrPageNotFound, 404, "page.not_found"}, {shared.Invalid(shared.FieldError{Field: "type", Code: shared.FieldTooLong, Message: "too long"}), 422, "validation_failed"}} {
			f := &fakes{err: c.err}
			status, body := call(t, f.serve(t), notebookPath+"/index")
			var problem map[string]any
			_ = json.Unmarshal([]byte(body), &problem)
			if status != c.status || problem["code"] != c.code {
				t.Fatalf("error=%d %s want %d %s", status, body, c.status, c.code)
			}
		}
	})
}
