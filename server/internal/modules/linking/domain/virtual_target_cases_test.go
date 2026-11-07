package domain_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// This is the same hand-written file consumed by the Web test. The public
// production WantedTarget path retains its real grammar, normalization and
// virtual exclusion; no test-side target parser supplies its expectations.
func TestVirtualTargetSharedCases(t *testing.T) {
	data, err := os.ReadFile("../../../../../testdata/m10/virtual-targets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name      string   `json:"name"`
		Raw       string   `json:"raw"`
		Parents   []string `json:"parents"`
		Virtual   *string  `json:"virtual"`
		Wanted    string   `json:"wanted"`
		Qualified bool     `json:"qualified"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 39 {
		t.Fatalf("shared cases = %d, want 39", len(cases))
	}
	positive, negative := 0, 0
	for _, tt := range cases {
		if tt.Virtual == nil {
			negative++
		} else {
			positive++
		}
		t.Run(tt.Name, func(t *testing.T) {
			got, qualified, err := domain.WantedTarget(context.Background(), tt.Raw, "wikilink", tt.Parents, true, nil)
			if err != nil || got != tt.Wanted || qualified != tt.Qualified {
				t.Fatalf("WantedTarget(%q, %v) = (%q, %v, %v), want (%q, %v, nil)", tt.Raw, tt.Parents, got, qualified, err, tt.Wanted, tt.Qualified)
			}
			// A standard notebook has no generated files: what only the LLM
			// Wiki leaves out of its wanted pages is exactly the virtual target.
			plain, wanted, err := domain.WantedTarget(context.Background(), tt.Raw, "wikilink", tt.Parents, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if (tt.Virtual != nil) != (wanted && !qualified) {
				t.Fatalf("virtual %v, but qualified %v standard and %v in an LLM Wiki", tt.Virtual, wanted, qualified)
			}
			if tt.Virtual != nil && strings.TrimSuffix(plain, ".md") != *tt.Virtual {
				t.Fatalf("standard target %q is not the virtual %q", plain, *tt.Virtual)
			}
		})
	}
	if positive != 12 || negative != 27 {
		t.Fatalf("shared split = %d/%d, want 12/27", positive, negative)
	}
}
