package domain

import (
	"time"
	"uuid"
)

// Rule names one of the lint checks, in the order returned by ApplicableRules.
type Rule string

const (
	Wanted             Rule = "wanted"
	Orphans            Rule = "orphans"
	Ambiguous          Rule = "ambiguous"
	InvalidFrontmatter Rule = "invalid_frontmatter"
	MissingType        Rule = "missing_type"
	MissingSummary     Rule = "missing_summary"
	UndigestedSources  Rule = "undigested_sources"
	InvalidSources     Rule = "invalid_sources"
)

// Count caps a report count, never the number of items that may be paged.
type Count struct {
	Value  int64
	Capped bool
}

// NodeKind is the kind of a pending source, independent of page's types.
type NodeKind string

const (
	NodePage  NodeKind = "page"
	NodeAsset NodeKind = "asset"
)

// LintMode is the effective contract. Its zero value is not a valid mode.
type LintMode uint8

const (
	ModeStandard LintMode = iota + 1
	ModeLLM
)

// CursorBinding identifies the notebook and the caller's scope contract.
// Implicit cursors bind only the notebook, not a previous effective mode/root.
type CursorBinding struct {
	NotebookID uuid.UUID
	Explicit   bool
	Mode       LintMode
	AreaRootID *uuid.UUID
}

// Position holds only the sorting fields of its rule. Begin is the unique
// start position and cannot be combined with any sorting field. A nil ID
// differs from the valid all-zero UUID. Prefix is raw bytes, not display text.
type Position struct {
	Begin     bool
	Pages     int64
	Prefix    []byte
	Hash      string
	ID        *uuid.UUID
	CreatedAt time.Time
}

// Item is an internal rule-tagged value. Constructors compute identities
// from full inputs before truncating Target. The app fills Cursor with its
// request binding; public REST/MCP branches are defined outside this type.
type Item struct {
	Rule            Rule
	ItemKey, Cursor string
	PageID, NodeID  *uuid.UUID
	Target          string
	TargetCut       bool
	Pages           int64
	NotLink         bool
	Kind            NodeKind
	Bytes           int64
	CreatedAt       time.Time
}
