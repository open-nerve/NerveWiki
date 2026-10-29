package domain

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A notebook's mode's contract (M10 design 4.6, 4.7, 4.9): what an LLM
// Wiki's writes may not do to its AGENTS page, its source root and the
// titles of its generated pages, judged on a step's facts alike by the mode
// guard in each unit and by each precheck before one.

// StepFacts are what CheckContract judges of a step (M10 design 4.6): its
// operation, its unit's kind, and each node it changes, where the node was
// and where it is after. FactsOf builds them for the guard from a Step,
// and each precheck from the step it plans, so that both judge alike.
type StepFacts struct {
	Operation   Operation
	Kind        ChangesetKind
	Nodes       []NodeFacts
	Content     *ContentFacts
	Rewrite     bool
	EditSession bool
}

// ContentFacts is the content a step writes, its hash and byte size,
// the byte size immediately before it, and its merged user changes.
// M10/P2/S1 carries these facts; S2 adds source-hash deduplication.
type ContentFacts struct {
	Hash                       []byte
	Bytes, BeforeBytes, Merged int
}

// NodeFacts is a node of a step: its kind, Before nil for a creation,
// After nil for a deletion. M10/P2/S1 carries each side's source-area
// membership alongside content, rewrite and edit-session facts.
type NodeFacts struct {
	ID     uuid.UUID
	Kind   Kind
	Before *NodePlace
	After  *NodePlace
}

// NodePlace is where a node is: its parent, nil at the notebook's root,
// and its name's title key. InArea marks a strict source descendant.
// (Place is an order among siblings.)
type NodePlace struct {
	ParentID *uuid.UUID
	Key      string
	InArea   bool
}

// atRoot reports whether p is a place at the notebook's root.
func (p *NodePlace) atRoot() bool { return p != nil && p.ParentID == nil }

// FactsOf is a step's facts from its operation, its unit's kind and its
// changes, each TreeState's name by its title key, in the changes'
// order: the first is the node the step is about (the node moved, its
// descendants after it; the root of a deleted subtree; the node renamed).
func FactsOf(op Operation, kind ChangesetKind, changes []Change) StepFacts {
	s := StepFacts{Operation: op, Kind: kind, Nodes: make([]NodeFacts, len(changes))}
	for i, c := range changes {
		s.Nodes[i] = NodeFacts{ID: c.NodeID, Before: placeOf(c.Before), After: placeOf(c.After)}
	}
	return s
}

// placeOf is the place of the tree state t, nil for none.
func placeOf(t *TreeState) *NodePlace {
	if t == nil {
		return nil
	}
	return &NodePlace{ParentID: t.ParentID, Key: shared.TitleKey(t.Name)}
}

// Verdict is CheckContract's answer: Err the step's refusal and NodeID the
// node it is about; or Needs an action the step needs beyond its unit's
// own decision, of NodeID, for the caller to ask; neither when the
// contract lets the step by.
type Verdict struct {
	Err    error
	NodeID uuid.UUID
	Needs  shared.Action
}

// ReservedKeys are the title keys no node may take at an LLM Wiki's root
// (M10 design 4.9): its generated index.md and log.md, by either name.
func ReservedKeys() []string { return []string{"index", "log", "index.md", "log.md"} }

// Reserved reports whether key is one of ReservedKeys.
func Reserved(key string) bool { return slices.Contains(ReservedKeys(), key) }

// agentsKey is the title key of an LLM Wiki's AGENTS page, at its root: the
// root's keys are one a node across pages and attachments, and checkPages
// keeps an alive LLM Wiki's AGENTS a page there, so the root's node of
// this key is it.
const agentsKey = "agents"

// CheckContract judges a step of the notebook n by its facts (M10 design
// 4.6, 4.9): a standard notebook's at once, reading nothing; an LLM
// Wiki's by the rules below, the first one a node breaks answering, in
// the order of their codes: page.protected, page.virtual_file,
// page.source_too_large, page.source_immutable, then page.manage_source.
//
//   - Its AGENTS page, the node at its root keyed agents, is not renamed,
//     moved (reordered, moved and renamed) or deleted; its source root is
//     not deleted, nor any page above it: page.protected.
//   - No node comes to its root under a reserved key (ReservedKeys) by a
//     creation, a rename or a move; an import's creations number theirs
//     instead (M10/P1/S3): page.virtual_file.
//   - One existing subtree moved from outside into sources has at most
//     MaxUnitNodes nodes, including empty pages and attachments. Restore,
//     import, source-out and membership-preserving moves keep their bounds.
//   - A source page may receive its first body once. Existing content is
//     immutable except for system rewrites; source edit sessions and version
//     restores are always refused, including rewrites (M10/P2 3.3).
//   - Existing sources' rename, move (including reorder) and delete need
//     page.manage_source of the changed source node. New arrivals do not.
//   - The source root's own rename and move (a reorder, a move and a
//     rename), and the move of a page above it to another parent, which
//     moves the source area, need page.manage_source of the source root,
//     its admins'; a reorder or a rename of a page above it does not, the
//     source area staying where it is (M10 design 4.7).
func CheckContract(n NotebookState, s StepFacts) Verdict {
	if n.Mode != ModeLLMWiki || len(s.Nodes) == 0 {
		return Verdict{}
	}
	for _, x := range s.Nodes {
		agents := x.Before.atRoot() && x.Before.Key == agentsKey
		switch {
		case agents && (s.Operation == OpRename || s.Operation == OpMove || s.Operation == OpDelete):
			return Verdict{Err: ErrProtected, NodeID: x.ID}
		case x.ID == n.SourceRootID && s.Operation == OpDelete:
			return Verdict{Err: ErrProtected, NodeID: x.ID}
		}
	}
	if s.Operation == OpCreate && s.Kind != ChangesetImport || s.Operation == OpRename || s.Operation == OpMove {
		for _, x := range s.Nodes {
			if x.After.atRoot() && Reserved(x.After.Key) && (!x.Before.atRoot() || x.Before.Key != x.After.Key) {
				return Verdict{Err: ErrVirtualFile, NodeID: x.ID}
			}
		}
	}
	first := s.Nodes[0]
	if s.Operation == OpMove && len(s.Nodes) > MaxUnitNodes && first.Before != nil && first.After != nil &&
		!first.Before.InArea && first.After.InArea && first.ID != n.SourceRootID {
		return Verdict{Err: ErrSourceTooLarge, NodeID: first.ID}
	}
	if sourceImmutable(n, s) {
		return Verdict{Err: ErrSourceImmutable, NodeID: s.Nodes[0].ID}
	}
	if id, needs := needsManageSource(n, s); needs {
		return Verdict{Needs: ActionManageSource, NodeID: id}
	}
	return Verdict{}
}

// sourceImmutable judges the page whose body the step writes. Content is
// absent in the tree pass and when opening an edit session; the latter is
// refused on membership alone. The source root's own body remains ordinary.
func sourceImmutable(n NotebookState, s StepFacts) bool {
	first := s.Nodes[0]
	if first.ID == n.SourceRootID || first.Kind != KindPage || first.After == nil || !first.After.InArea {
		return false
	}
	if s.EditSession || s.Kind == ChangesetVersionRestore {
		return true
	}
	return s.Content != nil && !s.Rewrite && (s.Content.BeforeBytes > 0 || s.Content.Merged > 1)
}

// needsManageSource names the existing source being managed, or the source
// root when its own place or an ancestor's parent changes. Looking only at
// the first node's source membership preserves an outside ancestor's rename
// and reorder exception, even when its subtree includes source descendants.
func needsManageSource(n NotebookState, s StepFacts) (uuid.UUID, bool) {
	first := s.Nodes[0]
	if (s.Operation == OpRename || s.Operation == OpMove || s.Operation == OpDelete) &&
		first.Before != nil && first.Before.InArea && first.ID != n.SourceRootID {
		return first.ID, true
	}
	switch s.Operation {
	case OpRename:
		return n.SourceRootID, first.ID == n.SourceRootID
	case OpMove:
		if first.ID == n.SourceRootID {
			return n.SourceRootID, true
		}
		moved := first.Before != nil && first.After != nil && !SameParent(first.Before.ParentID, first.After.ParentID)
		return n.SourceRootID, moved && slices.ContainsFunc(s.Nodes[1:], func(x NodeFacts) bool { return x.ID == n.SourceRootID })
	}
	return uuid.Nil(), false
}
