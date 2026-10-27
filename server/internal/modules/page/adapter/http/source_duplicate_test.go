package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceDuplicateAnswersCarryOnlyStableTargets(t *testing.T) {
	for _, tt := range []struct{ name, method, path, body string }{
		{"createPage", http.MethodPost, pagesPath, `{"parent_id":null,"title":"New","content":"same"}`},
		{"putPageContent", http.MethodPut, contentPath, `{"content":"same","base_revision":1}`},
		{"moveNode", http.MethodPost, movePath, `{"parent_id":null}`},
		{"restoreNode", http.MethodPost, restoreNodePath, ""},
		{"revertChangeset", http.MethodPost, revertPath, ""},
	} {
		for _, target := range []*uuid.UUID{nil, new(id(14))} {
			name := "within_operation"
			if target != nil {
				name = "baseline_target"
			}
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				status, body := call(t, (&fakes{err: domain.Refuse(domain.SourceDuplicate(target), id(12))}).serve(t), "session", tt.method, tt.path, tt.body)
				var p map[string]json.RawMessage
				if err := json.Unmarshal([]byte(body), &p); err != nil {
					t.Fatal(err)
				}
				if status != http.StatusConflict || string(p["code"]) != `"page.source_duplicate"` {
					t.Fatalf("answer = %d %s", status, body)
				}
				got, exists := p["duplicate_of"]
				if target == nil && exists || target != nil && string(got) != `"`+target.String()+`"` {
					t.Errorf("duplicate_of = %s present %v; want %v", got, exists, target)
				}
			})
		}
	}
}
