package app

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type ingestBeforeRead struct {
	before   IngestBefore
	heads    IngestIndexHeads
	notebook uuid.UUID
	nodes    map[uuid.UUID]IngestNodeBefore
	index    map[uuid.UUID]IngestIndexedHead
}

func sortedIngestIDs(ids []uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(ids)
}
func orderedIngestIDs(ids []uuid.UUID) bool {
	for i := 1; i < len(ids); i++ {
		if ids[i-1].Compare(ids[i]) >= 0 {
			return false
		}
	}
	return true
}
func currentIngestHead(h IngestIndexedHead, revision int) bool {
	return h.Present && h.PageRevision == revision && h.IndexedRevision == revision && h.Extractor == domain.Extractor
}

func (r *ingestBeforeRead) load(ctx context.Context, ids []uuid.UUID) error {
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
		nodes, err := r.before.NodesBefore(ctx, batch)
		if err != nil {
			return err
		}
		if len(nodes) != len(batch) {
			return errors.New("ingest: incomplete before metadata")
		}
		var revisions []IngestRevision
		for i, n := range nodes {
			if n.ID != batch[i] || n.Exists && n.Kind != "page" && n.Kind != "asset" || n.PageBytes < 0 || n.Revision < 0 {
				return errors.New("ingest: malformed before metadata")
			}
			r.nodes[n.ID] = n
			if n.Exists && n.Kind == "page" {
				revisions = append(revisions, IngestRevision{ID: n.ID, Revision: n.Revision})
			}
		}
		heads, err := r.heads.IndexedHeads(ctx, r.notebook, revisions)
		if err != nil {
			return err
		}
		if len(heads) != len(revisions) {
			return errors.New("ingest: incomplete before index heads")
		}
		for i, h := range heads {
			if h.ID != revisions[i].ID || h.PageRevision != revisions[i].Revision {
				return errors.New("ingest: malformed before index head")
			}
			r.index[h.ID] = h
		}
	}
	return nil
}

// qualify returns both the certainly eligible references and the complete
// finite upper bound. A late target size/revision never becomes a known fact.
func (r *ingestBeforeRead) qualify(ctx context.Context, refs []QualifiedReference) ([]QualifiedReference, []uuid.UUID, bool, error) {
	ids := make([]uuid.UUID, len(refs))
	for i, ref := range refs {
		ids[i] = ref.TargetID
	}
	if err := r.load(ctx, ids); err != nil {
		return nil, nil, false, err
	}
	var known []QualifiedReference
	var possible []uuid.UUID
	uncertain := false
	for _, ref := range refs {
		n := r.nodes[ref.TargetID]
		if !n.Exists || !n.InSource || n.IsSourceRoot {
			continue
		}
		if ref.TargetIsAsset {
			if n.Kind == "asset" {
				known = append(known, ref)
				possible = append(possible, ref.TargetID)
			}
			continue
		}
		if n.Kind != "page" || n.PageBytesKnown && n.PageBytes == 0 {
			continue
		}
		possible = append(possible, ref.TargetID)
		if !n.PageBytesKnown || !n.RevisionKnown || !currentIngestHead(r.index[n.ID], n.Revision) {
			uncertain = true
		} else {
			known = append(known, ref)
		}
	}
	return known, sortedIngestIDs(possible), uncertain, nil
}

// captureForIngest is called only after Index.PagesChanged has acquired the
// notebook index lock and before its first Replace/Delete. The only callbacks
// here collect this unit's facts; public Capture never selects the private reader.
func captureForIngest(ctx context.Context, u *ingestUnit) error {
	r := QualifiedBefore{Heads: u.deps.Heads, Reads: u.deps.Reads}
	if u.deps.OldBodies == nil {
		return r.Capture(ctx, u.event.NotebookID, u.event.BeforeIngest, u.writtenOrDeleted, u.direct, u.followed)
	}
	return r.capture(ctx, u.event.NotebookID, u.event.BeforeIngest, u.writtenOrDeleted, u.direct, u.followed, u.deps.OldBodies, u.prepared)
}

// Capture finishes every old D before reading F, while every old indexed head
// and resolved edge still exists. The caller may only then Replace or Delete.
func (r QualifiedBefore) Capture(ctx context.Context, notebook uuid.UUID, before IngestBefore, writtenOrDeleted []uuid.UUID,
	direct func(OldQualifiedFacts) error, followed func(OldSourceFacts) error,
) error {
	return r.capture(ctx, notebook, before, writtenOrDeleted, direct, followed, nil, nil)
}

func (r QualifiedBefore) capture(ctx context.Context, notebook uuid.UUID, before IngestBefore, writtenOrDeleted []uuid.UUID,
	direct func(OldQualifiedFacts) error, followed func(OldSourceFacts) error,
	oldBodies IngestOldBodyReads, prepared *sourcePreparationSet,
) error {
	if before == nil || r.Heads == nil || r.Reads == nil || !orderedIngestIDs(writtenOrDeleted) {
		return errors.New("ingest: incomplete capture dependencies or unordered W")
	}
	reader := ingestBeforeRead{before: before, heads: r.Heads, notebook: notebook,
		nodes: map[uuid.UUID]IngestNodeBefore{}, index: map[uuid.UUID]IngestIndexedHead{}}
	if err := reader.load(ctx, writtenOrDeleted); err != nil {
		return err
	}
	var wiki, sources []uuid.UUID
	for _, id := range writtenOrDeleted {
		n := reader.nodes[id]
		f := OldQualifiedFacts{PageID: id}
		switch {
		case !n.Exists:
			f.State = OldFactsKnownAbsent
		case n.Kind != "page" || n.InSource || n.IsSourceRoot || n.IsRootAgents:
			f.State = OldFactsKnownIneligible
		case n.RevisionKnown && currentIngestHead(reader.index[id], n.Revision):
			wiki = append(wiki, id)
			continue
		}
		if err := direct(f); err != nil {
			return err
		}
	}
	if err := referencePages(ctx, r.Reads, notebook, wiki, true, func(id uuid.UUID, refs []QualifiedReference) error {
		known, possible, uncertain, err := reader.qualify(ctx, refs)
		if err != nil {
			return err
		}
		f := OldQualifiedFacts{PageID: id, State: OldFactsKnown, References: known}
		if uncertain {
			f.State = OldFactsUnknown
			f.Unknown = OldUncertainty{Extent: OldUnknownPossibleTargets, PossibleTargets: possible}
		}
		return direct(f)
	}); err != nil {
		return err
	}
	// The source role is independent: its wiki D was ineligible above.
	for _, id := range writtenOrDeleted {
		n := reader.nodes[id]
		f := OldSourceFacts{PageID: id}
		switch {
		case !n.Exists:
			f.State = OldFactsKnownAbsent
		case n.Kind != "page" || !n.InSource || n.IsSourceRoot || n.PageBytesKnown && n.PageBytes == 0:
			f.State = OldFactsKnownIneligible
		case n.RevisionKnown && currentIngestHead(reader.index[id], n.Revision):
			sources = append(sources, id)
			continue
		}
		if err := followed(f); err != nil {
			return err
		}
	}
	var assetCache *ingestAssetCache
	if oldBodies != nil {
		assetCache = &ingestAssetCache{} // One private capture, never public Capture.
	}
	qualifyAssets := func(ids []uuid.UUID) ([]uuid.UUID, error) {
		return reader.assets(ctx, ids)
	}
	emitAssets := func(id uuid.UUID, ids []uuid.UUID) error {
		ids, err := assetCache.qualify(ids, qualifyAssets)
		if err != nil {
			return err
		}
		f := OldSourceFacts{PageID: id, State: OldFactsKnown, AssetIDs: ids}
		if !reader.nodes[id].PageBytesKnown {
			f.State = OldFactsUnknown
			f.Unknown = OldUncertainty{Extent: OldUnknownPossibleTargets, PossibleTargets: ids}
		}
		return followed(f)
	}
	if oldBodies != nil {
		var fallback, planned []uuid.UUID
		for _, id := range sources {
			if prepared.plan(ctx, id) {
				planned = append(planned, id)
			} else {
				fallback = append(fallback, id)
			}
		}
		// New explicit order: all D, then the original fallback F stream,
		// then each prepared source's complete native read and metadata.
		if err := referenceIngestBodyAssets(ctx, notebook, fallback, oldBodies, emitAssets); err != nil {
			return err
		}
		for _, id := range planned {
			if err := ctx.Err(); err != nil {
				return err
			}
			assets, err := prepared.plans[id].ReadBefore(ctx, prepared.unit)
			if err != nil {
				return err
			}
			if err := emitAssets(id, assets); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	return referencePages(ctx, r.Reads, notebook, sources, false, func(id uuid.UUID, refs []QualifiedReference) error {
		// Public Capture retains the original current-head Bodies path.
		assets := refs[:0]
		for _, ref := range refs {
			if ref.TargetIsAsset && ref.Origin == ReferenceBody {
				assets = append(assets, ref)
			}
		}
		ids := make([]uuid.UUID, len(assets))
		for i, ref := range assets {
			ids[i] = ref.TargetID
		}
		return emitAssets(id, ids)
	})
}
