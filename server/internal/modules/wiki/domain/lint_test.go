package domain

import (
	"reflect"
	"testing"
	"time"
	"uuid"
)

func TestLintRuleNames(t *testing.T) {
	standard := []LintRule{"wanted", "orphans", "ambiguous", "invalid_frontmatter"}
	wiki := []LintRule{"wanted", "orphans", "ambiguous", "invalid_frontmatter", "missing_type", "missing_summary", "undigested_sources", "invalid_sources"}
	for _, tc := range []struct {
		mode Mode
		want []LintRule
	}{{ModeStandard, standard}, {ModeLLMWiki, wiki}} {
		got := LintRules(tc.mode)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("rules for %q = %v, want %v", tc.mode, got, tc.want)
		}
		got[0] = "changed"
		if !reflect.DeepEqual(LintRules(tc.mode), tc.want) {
			t.Fatal("a caller changed the mode's rule list")
		}
	}
	for _, want := range wiki {
		if got, ok := ParseLintRule(string(want)); !ok || got != want {
			t.Errorf("parse %q = %q, %v", want, got, ok)
		}
	}
	for _, name := range []string{"", "Wanted", " wanted", "sources", "wanted\x00"} {
		if got, ok := ParseLintRule(name); ok || got != "" {
			t.Errorf("unknown rule %q = %q, %v", name, got, ok)
		}
	}
	if got := LintRules("upgrading"); got != nil {
		t.Fatalf("unknown mode has rules: %v", got)
	}
}

func TestLintResultOwnsItsValues(t *testing.T) {
	zero := uuid.UUID{}
	node := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	path, next, start := "folder/page", "after", "begin"
	at := time.Date(2026, 10, 30, 2, 0, 0, 123, time.UTC)
	input := LintResult{
		Kind: LintItems, Mode: ModeLLMWiki, ComputedAt: at, StalePages: 3,
		Rules: []LintRuleResult{{
			Rule: RuleUndigestedSources, Count: LintCount{Value: 10000, Capped: true},
			Items: []LintItem{{Rule: RuleUndigestedSources, ItemKey: "id", Cursor: "cursor", PageID: &zero, NodeID: &node,
				Target: "target", TargetCut: true, Pages: 7, NotLink: true, Kind: NodeAsset, Bytes: 42, CreatedAt: at, Path: &path}},
			NextCursor: &next, StartCursor: &start,
		}},
	}
	a, b := CloneLintResult(input), CloneLintResult(input)
	if !reflect.DeepEqual(a, input) || !reflect.DeepEqual(b, input) {
		t.Fatal("clone lost result fields or the present zero UUID")
	}
	*a.Rules[0].Items[0].PageID = node
	*a.Rules[0].Items[0].NodeID = zero
	*a.Rules[0].Items[0].Path = "changed"
	*a.Rules[0].NextCursor = "changed"
	*a.Rules[0].StartCursor = "changed"
	a.Rules[0].Items[0].Target = "changed"
	a.Rules[0].Count.Value = 1
	a.Rules[0].Items = append(a.Rules[0].Items, LintItem{})
	a.Rules = append(a.Rules, LintRuleResult{})
	if !reflect.DeepEqual(b, input) || *input.Rules[0].Items[0].PageID != (uuid.UUID{}) ||
		*input.Rules[0].Items[0].NodeID != uuid.MustParse("11111111-1111-4111-8111-111111111111") ||
		input.Rules[0].Items[0].Target != "target" || input.Rules[0].Count.Value != 10000 ||
		path != "folder/page" || next != "after" || start != "begin" {
		t.Fatal("one caller's projection changed another caller or the owner's result")
	}
	for _, items := range [][]LintItem{nil, {}} {
		got := CloneLintResult(LintResult{Rules: []LintRuleResult{{Items: items}}})
		if got.Rules[0].Items == nil || len(got.Rules[0].Items) != 0 || got.Rules[0].NextCursor != nil || got.Rules[0].StartCursor != nil {
			t.Fatal("empty items must be an owned empty array without invented cursors")
		}
	}
	got := CloneLintResult(LintResult{Rules: []LintRuleResult{{Items: []LintItem{{}}}}})
	item := got.Rules[0].Items[0]
	if item.PageID != nil || item.NodeID != nil || item.Path != nil {
		t.Fatal("absent fields became present zero values")
	}
}
