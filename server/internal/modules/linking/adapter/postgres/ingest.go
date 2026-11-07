package postgresadapter

import (
	"context"
	"errors"
	"math"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// IngestReads holds only the linking owner's evidence queries.
type IngestReads struct {
	store      *Store
	maxPayload int64
}

func NewIngestReads(pool *pgxpool.Pool, maxContentBytes int) *IngestReads {
	return &IngestReads{store: New(pool), maxPayload: app.MaxLintPayloadBytes(maxContentBytes)}
}
func orderedIngestIDs(ids []uuid.UUID) bool {
	for i := 1; i < len(ids); i++ {
		if ids[i-1].Compare(ids[i]) >= 0 {
			return false
		}
	}
	return true
}
func ingestRevisions(revisions []app.IngestRevision) ([]uuid.UUID, []int32, error) {
	ids := make([]uuid.UUID, len(revisions))
	values := make([]int32, len(revisions))
	for i, r := range revisions {
		if r.Revision < 0 || r.Revision > math.MaxInt32 || i > 0 && revisions[i-1].ID.Compare(r.ID) >= 0 {
			return nil, nil, errors.New("ingest evidence: invalid revisions")
		}
		ids[i], values[i] = r.ID, int32(r.Revision)
	}
	return ids, values, nil
}

func (r *IngestReads) IndexedHeads(ctx context.Context, notebook uuid.UUID, revisions []app.IngestRevision) ([]app.IngestIndexedHead, error) {
	if len(revisions) > app.LintReadRows {
		return nil, errors.New("ingest evidence: more than 512 heads")
	}
	ids, values, err := ingestRevisions(revisions)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.store.planned(ctx).IngestIndexedHeads(ctx, gen.IngestIndexedHeadsParams{NotebookID: notebook, PageIds: ids, PageRevisions: values})
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, errors.New("ingest evidence: incomplete heads")
	}
	out := make([]app.IngestIndexedHead, len(rows))
	for i, row := range rows {
		if row.ID != ids[i] || row.PageRevision != values[i] {
			return nil, errors.New("ingest evidence: mismatched heads")
		}
		h := app.IngestIndexedHead{ID: row.ID, PageRevision: int(row.PageRevision), Present: row.Present}
		if row.Present {
			h.IndexedRevision, h.Extractor, h.FrontmatterValid = int(row.IndexedRevision), int(row.Extractor), row.FrontmatterValid
		} else if row.IndexedRevision != 0 || row.Extractor != 0 || row.FrontmatterValid {
			return nil, errors.New("ingest evidence: nonzero missing head")
		}
		out[i] = h
	}
	return out, nil
}

// NovelSources returns complete candidate AND parent evidence. The generated
// query consumes every Scan and rows.Err before these flags are interpreted.
func (r *IngestReads) NovelSources(ctx context.Context, q app.IngestCandidateQuery) (app.IngestCandidateEvidence, error) {
	out := app.IngestCandidateEvidence{}
	if !postgres.InTx(ctx) {
		return out, errors.New("ingest evidence: no write transaction")
	}
	for _, ids := range [][]uuid.UUID{q.EligibleEmitters, q.UntouchedSources, q.PossibleOldDirect, q.ParentIDs} {
		if !orderedIngestIDs(ids) {
			return out, errors.New("ingest evidence: sets must be sorted and unique")
		}
	}
	sourceIDs, revisions, err := ingestRevisions(q.CurrentSources)
	if err != nil {
		return out, err
	}
	current := make(map[uuid.UUID]int, len(sourceIDs))
	for _, source := range q.CurrentSources {
		if source.Revision < 1 {
			return out, errors.New("ingest evidence: invalid source revision")
		}
		current[source.ID] = source.Revision
	}
	for _, id := range q.UntouchedSources {
		if current[id] == 0 {
			return out, errors.New("ingest evidence: untouched source outside proof")
		}
	}
	for _, id := range q.EligibleEmitters {
		if current[id] != 0 {
			return out, errors.New("ingest evidence: wiki emitter in source proof")
		}
	}
	args := gen.IngestNovelSourcesParams{NotebookID: q.NotebookID, Extractor: domain.Extractor,
		CurrentSourceIds: sourceIDs, CurrentSourceRevisions: revisions, EligibleEmitterIds: q.EligibleEmitters,
		UntouchedSourceIds: q.UntouchedSources, PossibleOldDirectIds: q.PossibleOldDirect, ParentIds: q.ParentIDs,
		MaxSlotItems: 10000, MaxJsonBytes: r.maxPayload}
	for i, c := range q.Candidates {
		if i > 0 && q.Candidates[i-1].ID.Compare(c.ID) >= 0 || c.Asset && c.Revision != 0 ||
			!c.Asset && (c.Revision < 1 || current[c.ID] != c.Revision) {
			return out, errors.New("ingest evidence: invalid candidate proof")
		}
		args.CandidateIds = append(args.CandidateIds, c.ID)
		args.CandidateAssets = append(args.CandidateAssets, c.Asset)
		args.CandidateRevisions = append(args.CandidateRevisions, int32(c.Revision))
	}
	if len(q.Candidates) == 0 {
		return out, errors.New("ingest evidence: no candidates")
	}
	rows, err := r.store.planned(ctx).IngestNovelSources(ctx, args)
	if err != nil {
		return out, err
	}
	var invalid bool
	for _, row := range rows {
		invalid = invalid || row.InvalidSourceValue == nil || row.InvalidProof == nil
		if row.InvalidSourceValue != nil {
			invalid = invalid || *row.InvalidSourceValue
		}
		if row.InvalidProof != nil {
			invalid = invalid || *row.InvalidProof
		}
		switch row.RowKind {
		case "candidate":
			i := len(out.Candidates)
			if i >= len(q.Candidates) || row.ID != q.Candidates[i].ID || row.Asset != q.Candidates[i].Asset {
				invalid = true
			}
			out.Candidates = append(out.Candidates, app.IngestCandidateFact{ID: row.ID, Asset: row.Asset, Direct: row.DirectlyDigested, FollowedUntouched: row.FollowedByUntouched})
		case "parent":
			i := len(out.Parents)
			if i >= len(q.ParentIDs) || row.ID != q.ParentIDs[i] || row.Asset || row.FollowedByUntouched {
				invalid = true
			}
			out.Parents = append(out.Parents, app.IngestParentFact{ID: row.ID, Direct: row.DirectlyDigested})
		default:
			invalid = true
		}
	}
	if invalid || len(out.Candidates) != len(q.Candidates) || len(out.Parents) != len(q.ParentIDs) {
		return app.IngestCandidateEvidence{}, errors.New("ingest evidence: malformed or incomplete candidate/parent evidence")
	}
	return out, ctx.Err()
}
