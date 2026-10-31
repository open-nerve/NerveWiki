package app_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestLintPrepareTimings(t *testing.T) {
	standard := []domain.Rule{domain.Wanted, domain.Orphans, domain.Ambiguous, domain.InvalidFrontmatter}
	llm := append(slices.Clone(standard), domain.MissingType, domain.MissingSummary, domain.UndigestedSources, domain.InvalidSources)
	for _, tc := range []struct {
		name   string
		rule   *domain.Rule
		llm    bool
		shared bool
		want   []domain.Rule
	}{
		{"standard-summary", nil, false, false, standard},
		{"llm-summary", nil, true, true, llm},
		{"wanted-only", lintPtr(domain.Wanted), true, false, []domain.Rule{domain.Wanted}},
		{"invalid-only", lintPtr(domain.InvalidSources), true, true, []domain.Rule{domain.InvalidSources}},
		{"pending-only", lintPtr(domain.UndigestedSources), true, true, []domain.Rule{domain.UndigestedSources}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, in := lintSourceFixture()
			if !tc.llm {
				in.EffectiveMode, in.AreaRootID, in.SourceIDs = domain.ModeStandard, nil, nil
				f.candidates = nil
				f.excluded = nil
			}
			p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: tc.rule})
			if err != nil {
				t.Fatal(err)
			}
			beforeCalls := slices.Clone(f.calls)
			got := p.Timings()
			if len(got.Rules) != len(tc.want) || (got.SharedSources != nil) != tc.shared {
				t.Fatalf("timing coverage=%+v, want rules=%v shared=%t", got, tc.want, tc.shared)
			}
			for i, rule := range tc.want {
				if got.Rules[i].Rule != rule || got.Rules[i].Duration < 0 {
					t.Fatalf("timing %d=%+v, want %s with a measured nonnegative duration", i, got.Rules[i], rule)
				}
			}
			if got.SharedSources != nil && *got.SharedSources < 0 {
				t.Fatal("negative shared duration")
			}
			wantValues := 0
			if tc.shared {
				wantValues = 1
			}
			values := 0
			for _, name := range f.calls {
				if name == "source-values" {
					values++
				}
			}
			if values != wantValues {
				t.Fatalf("sources value calls=%d, want %d shared phase", values, wantValues)
			}
			original := app.PreparationTimings{Rules: slices.Clone(got.Rules)}
			if got.SharedSources != nil {
				value := *got.SharedSources
				original.SharedSources = &value
			}
			got.Rules[0].Rule, got.Rules[0].Duration = domain.InvalidSources, -1
			if got.SharedSources != nil {
				*got.SharedSources = -1
			}
			if !reflect.DeepEqual(p.Timings(), original) || !slices.Equal(f.calls, beforeCalls) {
				t.Fatal("timing projection exposed mutable storage or repeated a read")
			}
		})
	}
	t.Run("late-failure-has-no-prepared-timings", func(t *testing.T) {
		f, in := lintSourceFixture()
		f.fail = "asset-bytes"
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{})
		if err == nil || p != nil {
			t.Fatalf("failed Prepare returned timings-bearing result=%v error=%v", p, err)
		}
	})
}
