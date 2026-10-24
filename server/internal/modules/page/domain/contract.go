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
	Operation Operation
	Kind      ChangesetKind
	Nodes     []NodeFacts
}

// NodeFacts is a node of a step: Before nil for a creation, After nil for
// a deletion. M10/P2/S1 adds whether each is in the source area, with
// the line of ancestors it is read from; P2/S2 the content's hash and
// size, the rewrite's mark and the edit session.
type NodeFacts struct {
	ID     uuid.UUID
	Before *NodePlace
	After  *NodePlace
}

// NodePlace is where a node is: its parent, nil at the notebook's root,
// and its name's title key. (Place is an order among siblings.)
type NodePlace struct {
	ParentID *uuid.UUID
	Key      string
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
// the order of their codes: page.protected, page.virtual_file, then the
// need of page.manage_source.
//
//   - Its AGENTS page, the node at its root keyed agents, is not renamed,
//     moved (reordered, moved and renamed) or deleted; its source root is
//     not deleted, nor any page above it: page.protected.
//   - No node comes to its root under a reserved key (ReservedKeys) by a
//     creation, a rename or a move; an import's creations number theirs
//     instead (M10/P1/S3): page.virtual_file.
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
	if needsManageSource(n, s) {
		return Verdict{Needs: ActionManageSource, NodeID: n.SourceRootID}
	}
	return Verdict{}
}

// needsManageSource reports whether s moves n's source area: a rename or a
// move whose first node, the one it is about, is the source root, or a move
// that takes the source root, among the first node's descendants, to
// another parent.
func needsManageSource(n NotebookState, s StepFacts) bool {
	first := s.Nodes[0]
	switch s.Operation {
	case OpRename:
		return first.ID == n.SourceRootID
	case OpMove:
		if first.ID == n.SourceRootID {
			return true
		}
		moved := first.Before != nil && first.After != nil && !SameParent(first.Before.ParentID, first.After.ParentID)
		return moved && slices.ContainsFunc(s.Nodes[1:], func(x NodeFacts) bool { return x.ID == n.SourceRootID })
	}
	return false
}
