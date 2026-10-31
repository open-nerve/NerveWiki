package mcpadapter

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The four expected objects were written before the first tools/list call.
// They include the nullable scalar schemas and each entry's notebook binding.
func TestLintToolObjects(t *testing.T) {
	raw, err := os.ReadFile("testdata/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string][]map[string]any
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	server := lintServer(t, nil)
	for _, entry := range []string{"notebook", "account"} {
		t.Run(entry, func(t *testing.T) {
			path := "/mcp"
			if entry == "notebook" {
				path += "?notebook=" + lintID(10).String()
			}
			body := lintRPC(t, server, path, "tools/list", map[string]any{})
			var listed struct {
				Tools []map[string]any `json:"tools"`
			}
			if err := json.Unmarshal(body, &listed); err != nil {
				t.Fatal(err)
			}
			if len(listed.Tools) != 2 || !reflect.DeepEqual(listed.Tools, expected[entry]) {
				t.Fatalf("complete %s tool objects = %s; want %s", entry, body, raw)
			}
		})
	}
}
