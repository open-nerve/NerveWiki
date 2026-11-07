package domain_test

import (
	"context"
	"encoding/json"
	"os"
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
	if len(cases) != 36 {
		t.Fatalf("shared cases = %d, want 36", len(cases))
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
		})
	}
	if positive != 12 || negative != 24 {
		t.Fatalf("shared split = %d/%d, want 12/24", positive, negative)
	}
}
