package app

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type oldFollowed struct {
	source   uuid.UUID
	assets   []uuid.UUID
	possible bool
}
type ingestBounds struct {
	knownDirect, possibleDirect     map[uuid.UUID]bool
	allDirect                       bool
	knownFollowed, possibleFollowed map[uuid.UUID]bool
	allInheritedAssets              bool
	uncertain                       bool
	retained                        []oldFollowed
	unknownFParents                 []uuid.UUID
}
type ingestUnit struct {
	prepared         *sourcePreparationSet
	deps             *Ingest
	event            PagesChanged
	scope            ingestScope
	writtenOrDeleted []uuid.UUID
	bounds           ingestBounds
	oldPairs         int
	started          time.Time
}

func ingestMapIDs(m map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return sortedIngestIDs(out)
}

func (x Index) beginIngest(ctx context.Context, e PagesChanged) (*ingestUnit, error) {
	if e.Kind != "edit" || e.Notebook.Mode != "llm_wiki" {
		return nil, nil
	}
	var w []uuid.UUID
	for _, c := range e.Changes {
		if c.After != nil && c.Revision != 0 || c.Before != nil && c.After == nil {
			w = append(w, c.NodeID)
		}
	}
	if len(w) == 0 {
		return nil, nil
	}
	if e.BeforeIngest == nil || e.ChangesetID == uuid.Nil() || e.Notebook.SourceRootID == nil || x.Ingest == nil ||
		x.Ingest.Metadata == nil || x.Ingest.Heads == nil || x.Ingest.Reads == nil || x.Ingest.Candidates == nil || x.Ingest.Marks == nil {
		return nil, errors.New("ingest: incomplete edit observer dependencies")
	}
	u := &ingestUnit{deps: x.Ingest, event: e, writtenOrDeleted: sortedIngestIDs(w), started: time.Now(),
		bounds: ingestBounds{knownDirect: map[uuid.UUID]bool{}, possibleDirect: map[uuid.UUID]bool{},
			knownFollowed: map[uuid.UUID]bool{}, possibleFollowed: map[uuid.UUID]bool{}}}
	u.prepared = newSourcePreparationSet(e, x.Preparations)
	u.scope = ingestScope{identity: u, notebookID: e.NotebookID, rootID: *e.Notebook.SourceRootID, extractor: domain.Extractor}
	if err := captureForIngest(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *ingestUnit) direct(f OldQualifiedFacts) error {
	u.bounds.uncertain = u.bounds.uncertain || f.State == OldFactsUnknown
	for _, ref := range f.References {
		u.bounds.knownDirect[ref.TargetID] = true
		u.bounds.possibleDirect[ref.TargetID] = true
	}
	if f.State == OldFactsUnknown {
		if f.Unknown.Extent == OldUnknownAll {
			u.bounds.allDirect = true
		} else {
			for _, id := range f.Unknown.PossibleTargets {
				u.bounds.possibleDirect[id] = true
			}
		}
	}
	return nil
}
func (u *ingestUnit) followed(f OldSourceFacts) error {
	u.bounds.uncertain = u.bounds.uncertain || f.State == OldFactsUnknown
	if f.State == OldFactsKnownAbsent || f.State == OldFactsKnownIneligible {
		return nil
	}
	if f.State == OldFactsUnknown && f.Unknown.Extent == OldUnknownAll {
		if u.bounds.possibleDirect[f.PageID] || u.bounds.allDirect {
			u.bounds.allInheritedAssets = true
		} else {
			u.bounds.unknownFParents = append(u.bounds.unknownFParents, f.PageID)
		}
		return nil
	}
	assets := f.AssetIDs
	if f.State == OldFactsUnknown {
		assets = f.Unknown.PossibleTargets
	}
	if u.bounds.possibleDirect[f.PageID] || u.bounds.allDirect {
		for _, id := range assets {
			u.bounds.possibleFollowed[id] = true
			if f.State == OldFactsKnown && u.bounds.knownDirect[f.PageID] {
				u.bounds.knownFollowed[id] = true
			}
		}
	} else {
		// Transfer the one sorted asset slice, not an R-sized map or SQL pair array.
		u.oldPairs += len(assets)
		u.bounds.retained = append(u.bounds.retained, oldFollowed{source: f.PageID, assets: assets, possible: f.State == OldFactsUnknown})
	}
	return nil
}

type ingestNowRead struct {
	metadata       IngestMetadata
	heads          IngestIndexHeads
	notebook, root uuid.UUID
	nodes          map[uuid.UUID]IngestHead
	index          map[uuid.UUID]IngestIndexedHead
}

func (r *ingestNowRead) load(ctx context.Context, ids []uuid.UUID) error {
	var missing []uuid.UUID
	for _, id := range ids {
		if _, ok := r.nodes[id]; !ok {
			missing = append(missing, id)
		}
	}
	missing = sortedIngestIDs(missing)
	for len(missing) > 0 {
		take := min(LintReadRows, len(missing))
		batch := missing[:take]
		missing = missing[take:]
		nodes, err := r.metadata.Now(ctx, r.notebook, r.root, batch)
		if err != nil {
			return err
		}
		if len(nodes) != len(batch) {
			return errors.New("ingest: incomplete current metadata")
		}
		var revisions []IngestRevision
		for i, n := range nodes {
			if n.ID != batch[i] || !validIngestNode(n) {
				return errors.New("ingest: malformed current metadata")
			}
			r.nodes[n.ID] = n
			if n.Live && n.Kind == "page" {
				revisions = append(revisions, IngestRevision{ID: n.ID, Revision: n.Revision})
			}
		}
		heads, err := r.heads.IndexedHeads(ctx, r.notebook, revisions)
		if err != nil {
			return err
		}
		if len(heads) != len(revisions) {
			return errors.New("ingest: incomplete current index heads")
		}
		for i, h := range heads {
			if h.ID != revisions[i].ID || h.PageRevision != revisions[i].Revision {
				return errors.New("ingest: malformed current index head")
			}
			r.index[h.ID] = h
		}
	}
	return nil
}

func (u *ingestUnit) candidates(ctx context.Context, written []uuid.UUID) ([]IngestSource, error) {
	r := ingestNowRead{metadata: u.deps.Metadata, heads: u.deps.Heads, notebook: u.scope.notebookID, root: u.scope.rootID,
		nodes: map[uuid.UUID]IngestHead{}, index: map[uuid.UUID]IngestIndexedHead{}}
	ids := sortedIngestIDs(append(slices.Clone(written), u.scope.rootID))
	if err := r.load(ctx, ids); err != nil {
		return nil, err
	}
	root := r.nodes[u.scope.rootID]
	if !root.Exists || !root.Live || root.Kind != "page" || !root.IsSourceRoot || root.InSource {
		return nil, errors.New("ingest: invalid current source root")
	}
	var emitters []uuid.UUID
	for _, id := range sortedIngestIDs(slices.Clone(written)) {
		n := r.nodes[id]
		if n.Live && n.Kind == "page" && !n.InSource && !n.IsSourceRoot && !n.IsRootAgents {
			if !currentIngestHead(r.index[id], n.Revision) {
				return nil, errors.New("ingest: written wiki index is not current")
			}
			emitters = append(emitters, id)
		}
	}
	candidates := map[uuid.UUID]IngestSource{}
	err := referencePages(ctx, u.deps.Reads, u.scope.notebookID, emitters, true, func(_ uuid.UUID, refs []QualifiedReference) error {
		targets := make([]uuid.UUID, len(refs))
		for i, ref := range refs {
			targets[i] = ref.TargetID
		}
		if err := r.load(ctx, targets); err != nil {
			return err
		}
		for _, ref := range refs {
			n := r.nodes[ref.TargetID]
			if !n.Live || !n.InSource || n.IsSourceRoot {
				continue
			}
			switch {
			case ref.TargetIsAsset && n.Kind == "asset":
				candidates[n.ID] = IngestSource{ID: n.ID, Asset: true}
			case !ref.TargetIsAsset && n.Kind == "page" && n.PageBytes > 0 && currentIngestHead(r.index[n.ID], n.Revision):
				candidates[n.ID] = IngestSource{ID: n.ID, Revision: n.Revision}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]IngestSource, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b IngestSource) int { return a.ID.Compare(b.ID) })
	return out, nil
}

func (u *ingestUnit) finish(ctx context.Context, written []uuid.UUID) error {
	if u == nil {
		return nil
	}
	candidates, err := u.candidates(ctx, written)
	if err != nil {
		return err
	}
	// What W may have cited before can never be novel: those candidates leave
	// here, so saving a page with the sources it already cites stays on the
	// ordinary non-no-op path, which ends without any whole-notebook read.
	candidates = slices.DeleteFunc(candidates, func(c IngestSource) bool {
		return u.bounds.allDirect || u.bounds.possibleDirect[c.ID]
	})
	if len(candidates) == 0 {
		return nil
	}
	coverage, err := (IngestCoverage{Metadata: u.deps.Metadata, Heads: u.deps.Heads}).check(ctx, u.scope, u.writtenOrDeleted)
	if err != nil {
		return err
	}
	if coverage.state != CoverageComplete {
		if u.deps.Logger != nil {
			u.deps.Logger.DebugContext(ctx, "ingest coverage incomplete", "notebook_id", u.scope.notebookID,
				"changeset_id", u.event.ChangesetID, "reason", coverage.reason, "raw_nodes", coverage.visited,
				"candidates", len(candidates), "duration", time.Since(u.started))
		}
		return nil
	}
	if coverage.scope.identity != u || coverage.scope.notebookID != u.scope.notebookID || coverage.scope.rootID != u.scope.rootID ||
		coverage.scope.extractor != domain.Extractor || !slices.Equal(coverage.excludedW, u.writtenOrDeleted) {
		return errors.New("ingest: coverage proof belongs to another unit")
	}
	parents := slices.Clone(u.bounds.unknownFParents)
	parents = append(parents, coverage.unknownFParents...)
	for _, relation := range u.bounds.retained {
		parents = append(parents, relation.source)
	}
	parents = sortedIngestIDs(parents)
	evidence, err := u.deps.Candidates.NovelSources(ctx, IngestCandidateQuery{NotebookID: u.scope.notebookID,
		Candidates: candidates, EligibleEmitters: coverage.eligibleEmitters, CurrentSources: coverage.currentSources,
		UntouchedSources: coverage.untouchedSources, PossibleOldDirect: ingestMapIDs(u.bounds.possibleDirect), ParentIDs: parents})
	if err != nil {
		return err
	}
	if len(evidence.Candidates) != len(candidates) || len(evidence.Parents) != len(parents) {
		return errors.New("ingest: incomplete candidate evidence")
	}
	directParents := map[uuid.UUID]bool{}
	for i, p := range evidence.Parents {
		if p.ID != parents[i] {
			return errors.New("ingest: malformed parent evidence")
		}
		directParents[p.ID] = p.Direct || u.bounds.possibleDirect[p.ID]
	}
	for _, p := range append(u.bounds.unknownFParents, coverage.unknownFParents...) {
		if directParents[p] {
			u.bounds.allInheritedAssets = true
		}
	}
	for _, relation := range u.bounds.retained {
		if directParents[relation.source] {
			for _, id := range relation.assets {
				u.bounds.possibleFollowed[id] = true
			}
		}
	}
	u.bounds.retained, u.bounds.unknownFParents = nil, nil
	var novel []uuid.UUID
	// All SQL rows, flags and terminal errors have already been consumed.
	for i, fact := range evidence.Candidates {
		c := candidates[i]
		if fact.ID != c.ID || fact.Asset != c.Asset {
			return errors.New("ingest: malformed candidate evidence")
		}
		if fact.Direct {
			continue
		}
		if c.Asset && (u.bounds.allInheritedAssets || u.bounds.possibleFollowed[c.ID] || fact.FollowedUntouched) {
			continue
		}
		novel = append(novel, c.ID)
	}
	result := IngestMarkResult{}
	if len(novel) > 0 {
		result, err = u.deps.Marks.Add(ctx, u.scope.notebookID, u.event.ChangesetID, novel)
		if err != nil {
			return err
		}
	}
	if u.deps.Logger != nil && u.bounds.uncertain {
		u.deps.Logger.DebugContext(ctx, "ingest incomplete old evidence", "notebook_id", u.scope.notebookID,
			"changeset_id", u.event.ChangesetID, "raw_nodes", coverage.visited, "candidates", len(candidates),
			"old_f_pairs", u.oldPairs, "parent_ids", len(parents), "added", result.Added, "duration", time.Since(u.started))
	}
	return nil
}
