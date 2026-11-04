package app

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A memo entry is the first tree state, independent of the source-hash memo.
// It keeps no content, and later writes never replace its locked content facts.
type ingestBeforeNode struct {
	state *domain.TreeState
	head  IngestNodeBefore
}

func (u *Unit) ingestEnabled() bool {
	return u.write.Kind == domain.ChangesetEdit && u.write.Notebook.Mode == domain.ModeLLMWiki
}

func ingestIDs(ids []uuid.UUID) error {
	if len(ids) > 512 {
		return errors.New("ingest before: more than 512 IDs")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1].Compare(ids[i]) >= 0 {
			return errors.New("ingest before: IDs must be sorted and unique")
		}
	}
	return nil
}

// rememberIngestBefore runs after every guard and before any write, including
// every node of a deletion and every participant's system content rewrite.
func (u *Unit) rememberIngestBefore(ctx context.Context, s Step) error {
	if !u.ingestEnabled() {
		return nil
	}
	if u.w.d.IngestNow == nil {
		return errors.New("ingest before: missing metadata")
	}
	if u.ingest == nil {
		u.ingest = map[uuid.UUID]ingestBeforeNode{}
	}
	var need []uuid.UUID
	for _, c := range s.Changes {
		if _, seen := u.ingest[c.NodeID]; seen {
			continue
		}
		kind, ok := s.Kinds[c.NodeID]
		if !ok || kind != domain.KindPage && kind != domain.KindAsset {
			return errors.New("ingest before: missing node kind")
		}
		n := ingestBeforeNode{head: IngestNodeBefore{ID: c.NodeID, Exists: c.Before != nil, Kind: kind}}
		if c.Before != nil {
			state := *c.Before
			if state.ParentID != nil {
				parent := *state.ParentID
				state.ParentID = &parent
			}
			n.state = &state
			if kind == domain.KindPage && s.Content != nil {
				n.head.Revision, n.head.RevisionKnown = s.Content.BeforeRevision, true
				n.head.PageBytes, n.head.PageBytesKnown = int64(s.Content.BeforeBytes), true
			} else if kind == domain.KindPage && u.write.Tree {
				need = append(need, c.NodeID)
			}
		} else {
			n.head.RevisionKnown, n.head.PageBytesKnown = true, true
		}
		u.ingest[c.NodeID] = n
	}
	slices.SortFunc(need, uuid.UUID.Compare)
	for len(need) > 0 {
		count := min(512, len(need))
		ids := need[:count]
		need = need[count:]
		heads, err := u.w.d.IngestNow.Now(ctx, u.write.NotebookID, u.write.Notebook.SourceRootID, ids)
		if err != nil {
			return err
		}
		if len(heads) != len(ids) {
			return errors.New("ingest before: incomplete deletion metadata")
		}
		for i, h := range heads {
			if h.ID != ids[i] || !h.Exists || !h.Live || h.Kind != domain.KindPage || !h.HasContent || h.Revision < 1 {
				return errors.New("ingest before: invalid locked page metadata")
			}
			n := u.ingest[h.ID]
			n.head.Revision, n.head.PageBytes = h.Revision, h.PageBytes
			n.head.RevisionKnown, n.head.PageBytesKnown = true, true
			u.ingest[h.ID] = n
		}
	}
	return nil
}

// NodesBefore reconstructs the initial ancestry from first states, never from
// final membership flags. The current paths supply only nodes the unit did not
// change. Missing required ancestors and cycles are errors, not final-tree fallbacks.
func (u *Unit) NodesBefore(ctx context.Context, ids []uuid.UUID) ([]IngestNodeBefore, error) {
	if err := u.inUnit(ctx); err != nil {
		return nil, err
	}
	if err := ingestIDs(ids); err != nil {
		return nil, err
	}
	if !u.ingestEnabled() || u.w.d.IngestNow == nil {
		return nil, errors.New("ingest before: unavailable unit")
	}
	var untouched []uuid.UUID
	for _, id := range ids {
		if _, ok := u.ingest[id]; !ok {
			untouched = append(untouched, id)
		}
	}
	now, err := u.w.d.IngestNow.Now(ctx, u.write.NotebookID, u.write.Notebook.SourceRootID, untouched)
	if err != nil {
		return nil, err
	}
	if len(now) != len(untouched) {
		return nil, errors.New("ingest before: incomplete target metadata")
	}
	heads := make(map[uuid.UUID]IngestHead, len(now))
	for i, h := range now {
		if h.ID != untouched[i] {
			return nil, errors.New("ingest before: unordered target metadata")
		}
		heads[h.ID] = h
	}
	// Read the paths once per bounded request. Deleted nodes are in the memo;
	// their unchanged initial parents may need a separate path in this request.
	pathIDs := slices.Clone(untouched)
	for _, id := range ids {
		if n, ok := u.ingest[id]; ok && n.state != nil && n.state.ParentID != nil {
			if _, changed := u.ingest[*n.state.ParentID]; !changed {
				pathIDs = append(pathIDs, *n.state.ParentID)
			}
		}
	}
	slices.SortFunc(pathIDs, uuid.UUID.Compare)
	pathIDs = slices.Compact(pathIDs)
	current := map[uuid.UUID]domain.TreeState{}
	if len(pathIDs) > 0 {
		if u.w.d.Paths == nil {
			return nil, errors.New("ingest before: missing path reader")
		}
		paths, err := u.w.d.Paths.LinePaths(ctx, u.write.NotebookID, pathIDs)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			for i, p := range path {
				state := domain.TreeState{Name: p.Name}
				if i > 0 {
					parent := path[i-1].ID
					state.ParentID = &parent
				}
				current[p.ID] = state
			}
		}
	}
	initial := func(id uuid.UUID) (*domain.TreeState, domain.Kind, error) {
		if n, ok := u.ingest[id]; ok {
			return n.state, n.head.Kind, nil
		}
		if state, ok := current[id]; ok {
			kind := domain.KindPage
			if h, exists := heads[id]; exists {
				kind = h.Kind
			}
			return &state, kind, nil
		}
		// Only an initial parent outside the requested current paths lands here.
		n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, id)
		if err != nil {
			return nil, "", err
		}
		state := n.State()
		current[id] = state
		return &state, n.Kind, nil
	}
	out := make([]IngestNodeBefore, len(ids))
	for i, id := range ids {
		n, memo := u.ingest[id]
		h := n.head
		if !memo {
			v := heads[id]
			h = IngestNodeBefore{ID: id, Exists: v.Exists && v.Live, Kind: v.Kind,
				Revision: v.Revision, PageBytes: v.PageBytes,
				RevisionKnown: u.write.Tree && v.HasContent, PageBytesKnown: u.write.Tree && v.HasContent}
		}
		if !h.Exists {
			out[i] = h
			continue
		}
		state, kind, err := initial(id)
		if err != nil {
			return nil, err
		}
		if state == nil || kind != h.Kind {
			return nil, errors.New("ingest before: missing initial node")
		}
		h.IsSourceRoot = id == u.write.Notebook.SourceRootID
		h.IsRootAgents = kind == domain.KindPage && state.ParentID == nil && shared.TitleKey(state.Name) == shared.TitleKey(domain.AgentsTitle)
		seen := map[uuid.UUID]bool{id: true}
		limit := domain.MaxDepth
		if kind == domain.KindAsset {
			limit++
		}
		for depth, parent := 1, state.ParentID; parent != nil; depth++ {
			if depth >= limit || seen[*parent] {
				return nil, errors.New("ingest before: invalid initial ancestry")
			}
			seen[*parent] = true
			if *parent == u.write.Notebook.SourceRootID {
				h.InSource = true
			}
			ancestor, ancestorKind, err := initial(*parent)
			if err != nil {
				return nil, err
			}
			if ancestor == nil || ancestorKind != domain.KindPage {
				return nil, errors.New("ingest before: missing initial ancestor")
			}
			parent = ancestor.ParentID
		}
		out[i] = h
	}
	return out, nil
}
