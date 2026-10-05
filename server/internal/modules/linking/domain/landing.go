package domain

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Reason is why a link's target has no landing.
type Reason string

// The reasons a target has no landing (M6/P6 design 2).
const (
	// TargetInvalid: the target cuts into no segments a page could have.
	TargetInvalid Reason = "target_invalid"
	// TitleInvalid: its last segment is no title.
	TitleInvalid Reason = "title_invalid"
	// ParentMissing: no page is where its segments before the last lead.
	ParentMissing Reason = "parent_missing"
	// TooDeep: the page would be deeper than pages nest.
	TooDeep Reason = "too_deep"
	// NotResolvable: the target would not lead to the page made, or not to
	// it alone.
	NotResolvable Reason = "not_resolvable"
)

// MaxLandingTarget is the longest target, in bytes, whose landing is read
// (M6/P6 design 2).
const MaxLandingTarget = 4096

// Landing is where a page made for a link's target would go, so that the
// link leads to it (M6/P6 design 2): under Parent, the zero id for the
// root, titled Title. Node is the page the target leads to already; Reason
// why there is none. Exactly one of Node, Title and Reason is set.
type Landing struct {
	Node   uuid.UUID
	Parent uuid.UUID
	Title  string
	Reason Reason
}

// Land is the landing of t, written in the page at from (itself last),
// among candidates and aliased, as Resolve reads them, and parents, the
// pages whose title key is that of t's segment before its last, if it has
// one; a page is at most maxDepth deep (M6/P6 design 2):
//
//   - the page t resolves to, if it resolves;
//   - else its title, the last segment as written without its ".md", as
//     titles are checked;
//   - under the source folder for a name alone; under the page its
//     segments before the last lead to, read exactly from the folder up
//     t.Up for a relative target and from the root for a rooted one, and
//     else by the resolution's first three steps, not by aliases, which
//     lead from a name alone only;
//   - and only if t would then resolve to the page made, and to it alone.
func Land(t Target, from []Step, candidates, parents []Node, aliased map[string][]Node, maxDepth int) Landing {
	if r := Resolve(t, from, candidates, aliased); r.ID != (uuid.UUID{}) {
		return Landing{Node: r.ID}
	}
	title, problem := shared.CheckTitle("title", t.Name)
	if problem != nil {
		return Landing{Reason: TitleInvalid}
	}
	parent, ok := t.parent(from, parents)
	if !ok {
		return Landing{Reason: ParentMissing}
	}
	if len(parent)+1 > maxDepth {
		return Landing{Reason: TooDeep}
	}
	// The page made loses a tie, the least id winning one: resolving to it,
	// t resolves to it alone, and Ambiguous only guards a rule that would
	// break ties otherwise.
	made := Node{ID: uuid.Max(), Path: append(slices.Clip(parent), Step{ID: uuid.Max(), Key: shared.TitleKey(title), Name: title})}
	if r := Resolve(t, from, append(slices.Clip(candidates), made), aliased); r.ID != made.ID || r.Ambiguous {
		return Landing{Reason: NotResolvable}
	}
	var id uuid.UUID
	if len(parent) > 0 {
		id = parent[len(parent)-1].ID
	}
	return Landing{Parent: id, Title: title}
}

// parent is the path from the root of the page a page made for t, written
// in the page at from, would go under, empty for the root, among parents:
// see Land.
func (t Target) parent(from []Step, parents []Node) ([]Step, bool) {
	folder := from[:max(len(from)-1, 0)]
	lead := t.Keys[:len(t.Keys)-1]
	switch {
	case t.Relative:
		return exactPath(folder[:max(len(folder)-t.Up, 0)], lead, parents)
	case t.Rooted:
		return exactPath(nil, lead, parents)
	case len(lead) == 0:
		return folder, true
	}
	r := Resolve(Target{Keys: lead}, from, parents, nil)
	if r.ID == (uuid.UUID{}) {
		return nil, false
	}
	at := slices.IndexFunc(parents, func(n Node) bool { return n.ID == r.ID })
	return parents[at].Path, true
}

// exactPath is base, then the page among parents whose path is base's then
// keys, if keys are any.
func exactPath(base []Step, keys []string, parents []Node) ([]Step, bool) {
	if len(keys) == 0 {
		return base, true
	}
	id, ok := exactly(base, keys, parents)
	if !ok {
		return nil, false
	}
	at := slices.IndexFunc(parents, func(n Node) bool { return n.ID == id })
	return parents[at].Path, true
}
