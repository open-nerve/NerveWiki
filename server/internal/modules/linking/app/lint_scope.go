package app

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

var (
	errLintMetadata = errors.New("lint rules: inconsistent metadata")
	errLintQuery    = errors.New("lint rules: invalid preparation query")
	errLintRows     = errors.New("lint rules: inconsistent read result")
)

// ReadScope is private to one preparation. Its sets are complete; none of the
// rule caps or stream batch sizes restrict membership or kind presence.
type ReadScope struct {
	notebook     uuid.UUID
	mode         domain.LintMode
	binding      domain.CursorBinding
	nodes        map[uuid.UUID]CompactNode
	current      map[uuid.UUID]IndexedHead
	source       map[uuid.UUID]struct{}
	presence     lintPresence
	stale        int64
	excluded     []uuid.UUID
	orphans      []uuid.UUID
	frontmatter  []uuid.UUID
	missing      []uuid.UUID
	properties   []uuid.UUID
	emitters     []uuid.UUID
	pendingPages []uuid.UUID
	parents      []uuid.UUID
	assets       []uuid.UUID
}

type lintPresence map[string]uint8

func (p lintPresence) Has(key string, asset bool) bool {
	bit := uint8(1)
	if asset {
		bit = 2
	}
	return p[key]&bit != 0
}

func lintCopyID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	v := *id
	return &v
}

func lintCopyInput(ctx context.Context, in ReadInput) (ReadInput, error) {
	in.AreaRootID = lintCopyID(in.AreaRootID)
	in.SourceIDs = slices.Clone(in.SourceIDs)
	in.Nodes = slices.Clone(in.Nodes)
	for i := range in.Nodes {
		if err := ctx.Err(); err != nil {
			return ReadInput{}, err
		}
		in.Nodes[i].ParentID = lintCopyID(in.Nodes[i].ParentID)
	}
	return in, nil
}

func lintIDs(ids []uuid.UUID) {
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return a.Compare(b) })
}

func newReadScope(ctx context.Context, in ReadInput, heads []IndexedHead) (*ReadScope, error) {
	if in.EffectiveMode != domain.ModeStandard && in.EffectiveMode != domain.ModeLLM ||
		in.EffectiveMode == domain.ModeStandard && (in.AreaRootID != nil || len(in.SourceIDs) != 0) ||
		in.EffectiveMode == domain.ModeLLM && in.AreaRootID == nil {
		return nil, errLintMetadata
	}
	s := &ReadScope{
		notebook: in.NotebookID, mode: in.EffectiveMode,
		binding: domain.CursorBinding{NotebookID: in.NotebookID, Explicit: in.Explicit,
			Mode: in.EffectiveMode, AreaRootID: lintCopyID(in.AreaRootID)},
		nodes: make(map[uuid.UUID]CompactNode, len(in.Nodes)), current: make(map[uuid.UUID]IndexedHead, len(heads)),
		source: make(map[uuid.UUID]struct{}, len(in.SourceIDs)), presence: lintPresence{},
	}
	children := make(map[uuid.UUID][]uuid.UUID)
	ordered := make([]uuid.UUID, 0, len(in.Nodes))
	for _, n := range in.Nodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if n.Kind != domain.NodePage && n.Kind != domain.NodeAsset ||
			n.Kind == domain.NodePage && n.PageBytes < 0 || n.NameKey == "" {
			return nil, errLintMetadata
		}
		if _, exists := s.nodes[n.ID]; exists {
			return nil, errLintMetadata
		}
		s.nodes[n.ID] = n
		ordered = append(ordered, n.ID)
		if n.ParentID != nil {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
		if n.Kind == domain.NodePage {
			s.presence[n.NameKey] |= 1
		} else if n.Linkable {
			s.presence[n.NameKey] |= 2
		}
	}
	// Page owns the legal depth limit. Here the converted facts still must be
	// a complete acyclic live forest; no new relative-source depth cap is used.
	state := make(map[uuid.UUID]uint8, len(s.nodes))
	for _, id := range ordered {
		var path []uuid.UUID
		for at := &id; at != nil && state[*at] != 2; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			n, ok := s.nodes[*at]
			if !ok || state[*at] == 1 {
				return nil, errLintMetadata
			}
			state[*at] = 1
			path = append(path, *at)
			if n.ParentID != nil {
				parent, exists := s.nodes[*n.ParentID]
				if !exists || parent.Kind != domain.NodePage {
					return nil, errLintMetadata
				}
			}
			at = n.ParentID
		}
		for _, p := range path {
			state[p] = 2
		}
	}
	for _, id := range in.SourceIDs {
		if _, exists := s.source[id]; exists {
			return nil, errLintMetadata
		}
		if _, exists := s.nodes[id]; !exists {
			return nil, errLintMetadata
		}
		s.source[id] = struct{}{}
	}
	if in.AreaRootID != nil {
		root, ok := s.nodes[*in.AreaRootID]
		if !ok || root.Kind != domain.NodePage {
			return nil, errLintMetadata
		}
		if _, included := s.source[root.ID]; included {
			return nil, errLintMetadata
		}
		// Reconcile both page ports: all and only strict descendants must have
		// been converted, including assets and sources past the 10000 move cap.
		stack := slices.Clone(children[root.ID])
		visited := 0
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, included := s.source[id]; !included {
				return nil, errLintMetadata
			}
			visited++
			stack = append(stack, children[id]...)
		}
		if visited != len(s.source) {
			return nil, errLintMetadata
		}
	}
	seenHeads := make(map[uuid.UUID]struct{}, len(heads))
	for _, h := range heads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := seenHeads[h.ID]; exists {
			return nil, errLintRows
		}
		seenHeads[h.ID] = struct{}{}
		if h.Extractor != domain.Extractor {
			s.stale++
			continue
		}
		if n, live := s.nodes[h.ID]; live && n.Kind == domain.NodePage {
			s.current[h.ID] = h
		}
	}
	lintIDs(ordered)
	llm := s.mode == domain.ModeLLM
	for _, id := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := s.nodes[id]
		_, source := s.source[id]
		if source {
			if n.Kind == domain.NodeAsset {
				s.assets = append(s.assets, id)
			} else {
				s.excluded = append(s.excluded, id)
				if n.PageBytes > 0 {
					s.parents = append(s.parents, id)
					if _, current := s.current[id]; current {
						s.pendingPages = append(s.pendingPages, id)
					}
				}
			}
		}
		h, current := s.current[id]
		if !current {
			continue
		}
		root := in.AreaRootID != nil && id == *in.AreaRootID
		agents := n.ParentID == nil && n.NameKey == "agents"
		folder := n.PageBytes == 0 && len(children[id]) > 0
		outside := !llm || !source
		if outside {
			if h.FrontmatterValid {
				s.properties = append(s.properties, id)
			} else {
				s.frontmatter = append(s.frontmatter, id)
			}
		}
		if outside && !agents && (!llm || !root) && !folder {
			s.orphans = append(s.orphans, id)
		}
		if llm && outside && !root && !agents {
			// The nonempty condition belongs to source P/F parents, never to
			// the wiki emitter. Even a zero-byte current emitter can qualify.
			s.emitters = append(s.emitters, id)
			if h.FrontmatterValid {
				s.missing = append(s.missing, id)
			}
		}
	}
	return s, nil
}

func (s *ReadScope) parentKeys(ctx context.Context, id uuid.UUID) ([]string, error) {
	n, ok := s.nodes[id]
	if !ok {
		return nil, errLintMetadata
	}
	var keys []string
	for parent := n.ParentID; parent != nil; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, ok = s.nodes[*parent]
		if !ok || n.Kind != domain.NodePage {
			return nil, errLintMetadata
		}
		keys = append(keys, n.NameKey)
		parent = n.ParentID
	}
	slices.Reverse(keys)
	return keys, nil
}

func (s *ReadScope) sourceTarget(id *uuid.UUID, asset bool) (CompactNode, bool) {
	if id == nil {
		return CompactNode{}, false
	}
	if _, ok := s.source[*id]; !ok {
		return CompactNode{}, false
	}
	n, ok := s.nodes[*id]
	if !ok || (n.Kind == domain.NodeAsset) != asset {
		return CompactNode{}, false
	}
	return n, true
}

func (s *ReadScope) digestible(n CompactNode) bool {
	if n.Kind == domain.NodeAsset {
		return true
	}
	_, current := s.current[n.ID]
	return current && n.PageBytes > 0
}
