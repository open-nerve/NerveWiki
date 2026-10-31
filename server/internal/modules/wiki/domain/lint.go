package domain

import (
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LintRule names a check in the mode's fixed result order.
type LintRule string

const (
	RuleWanted             LintRule = "wanted"
	RuleOrphans            LintRule = "orphans"
	RuleAmbiguous          LintRule = "ambiguous"
	RuleInvalidFrontmatter LintRule = "invalid_frontmatter"
	RuleMissingType        LintRule = "missing_type"
	RuleMissingSummary     LintRule = "missing_summary"
	RuleUndigestedSources  LintRule = "undigested_sources"
	RuleInvalidSources     LintRule = "invalid_sources"
)

// ParseLintRule accepts only the eight rule names, without normalizing input.
func ParseLintRule(name string) (LintRule, bool) {
	switch rule := LintRule(name); rule {
	case RuleWanted, RuleOrphans, RuleAmbiguous, RuleInvalidFrontmatter,
		RuleMissingType, RuleMissingSummary, RuleUndigestedSources, RuleInvalidSources:
		return rule, true
	}
	return "", false
}

// LintRules returns a fresh ordered list, or nil for an unknown mode.
func LintRules(mode Mode) []LintRule {
	if mode != ModeStandard && mode != ModeLLMWiki {
		return nil
	}
	rules := []LintRule{RuleWanted, RuleOrphans, RuleAmbiguous, RuleInvalidFrontmatter}
	if mode == ModeLLMWiki {
		rules = append(rules, RuleMissingType, RuleMissingSummary, RuleUndigestedSources, RuleInvalidSources)
	}
	return rules
}

// NodeKind is the wiki's source kind; adapters convert other modules' values.
type NodeKind string

const (
	NodePage  NodeKind = "page"
	NodeAsset NodeKind = "asset"
)

// LintCount caps the reported count, not the items available for pagination.
type LintCount struct {
	Value  int64
	Capped bool
}

// LintQueryKind separates a summary from one rule's list. Zero is invalid.
type LintQueryKind uint8

const (
	LintSummary LintQueryKind = iota + 1
	LintItems
)

// LintPosition holds a decoded cursor's sorting fields. An ID pointer may
// contain the zero UUID; Begin is the distinct position before the first item.
type LintPosition struct {
	Begin     bool
	Pages     int64
	Prefix    []byte
	Hash      string
	ID        *uuid.UUID
	CreatedAt time.Time
}

// LintItem is the internal rule-tagged result, before HTTP or tool projection.
type LintItem struct {
	Rule            LintRule
	ItemKey, Cursor string
	PageID, NodeID  *uuid.UUID
	Target          string
	TargetCut       bool
	Pages           int64
	NotLink         bool
	Kind            NodeKind
	Bytes           int64
	CreatedAt       time.Time
	Path            *string
}

// LintRuleResult retains the start cursor for a later projection with no
// previews. It is distinct from NextCursor after an actual returned item.
type LintRuleResult struct {
	Rule        LintRule
	Count       LintCount
	Items       []LintItem
	NextCursor  *string
	StartCursor *string
}

// LintResult belongs to one caller; it is not itself a public wire schema.
type LintResult struct {
	Kind       LintQueryKind
	Mode       Mode
	ComputedAt time.Time
	StalePages int64
	Rules      []LintRuleResult
}

// CloneLintResult gives a caller independent slices and pointer values.
// Empty item lists remain non-nil for the protocol projections.
func CloneLintResult(result LintResult) LintResult {
	result.Rules = slices.Clone(result.Rules)
	for i := range result.Rules {
		rule := &result.Rules[i]
		items := make([]LintItem, len(rule.Items))
		copy(items, rule.Items)
		rule.Items = items
		rule.NextCursor = cloneLintPointer(rule.NextCursor)
		rule.StartCursor = cloneLintPointer(rule.StartCursor)
		for j := range rule.Items {
			item := &rule.Items[j]
			item.PageID = cloneLintPointer(item.PageID)
			item.NodeID = cloneLintPointer(item.NodeID)
			item.Path = cloneLintPointer(item.Path)
		}
	}
	return result
}

func cloneLintPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// ActionLintRead is consumed by Execute and registered by the module root.
const ActionLintRead shared.Action = "lint.read"

// Actions returns an independent list for the composition's rule table.
func Actions() []shared.Action { return []shared.Action{ActionLintRead} }
