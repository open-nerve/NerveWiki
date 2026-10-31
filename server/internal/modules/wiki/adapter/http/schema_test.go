package httpadapter_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintSchemaRejectsCrossedAndExtraFields(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "../../../../../../api/dist/openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	f := lintRichFixture(t, domain.ModeLLMWiki)
	response, body := lintCall(t, f.handler, lintPath)
	if response.StatusCode != 200 {
		t.Fatalf("schema positive: %s", body)
	}
	var summary map[string]any
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatal(err)
	}
	schema := doc.Components.Schemas["LintSummary"].Value
	if err := schema.VisitJSON(summary); err != nil {
		t.Fatalf("actual positive summary: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"extra-summary-path", func(v map[string]any) { v["path"] = "private" }},
		{"extra-item-path", func(v map[string]any) {
			v["rules"].([]any)[1].(map[string]any)["items"].([]any)[0].(map[string]any)["path"] = "private"
		}},
		{"crossed-rule", func(v map[string]any) {
			v["rules"].([]any)[1].(map[string]any)["items"].([]any)[0].(map[string]any)["rule"] = "missing_type"
		}},
		{"missing-false", func(v map[string]any) {
			delete(v["rules"].([]any)[7].(map[string]any)["items"].([]any)[0].(map[string]any), "not_link")
		}},
		{"missing-cursor", func(v map[string]any) {
			delete(v["rules"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any), "cursor")
		}},
		{"missing-identity", func(v map[string]any) {
			delete(v["rules"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any), "item_key")
		}},
		{"null-items", func(v map[string]any) { v["rules"].([]any)[0].(map[string]any)["items"] = nil }},
		{"missing-null-cursor", func(v map[string]any) { delete(v["rules"].([]any)[0].(map[string]any), "next_cursor") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var changed map[string]any
			if err := json.Unmarshal(body, &changed); err != nil {
				t.Fatal(err)
			}
			tc.change(changed)
			if err := schema.VisitJSON(changed); err == nil {
				t.Fatal("strict rule schema accepted invalid public shape")
			}
		})
	}
}
