package postgresadapter

import (
	"bytes"
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

var _ app.LintRuleStore = (*Store)(nil)

// This is ECMAScript TrimString's exact WhiteSpace + LineTerminator set.
// In particular FEFF is whitespace and 0085 is not. PostgreSQL's default
// btrim and Go's Unicode whitespace definition have different contracts.
const lintTrimChars = "\u0009\u000a\u000b\u000c\u000d\u0020\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

// Complete eligibility relations have no 512-row read-batch limit.
func lintRelationIDs(ids []uuid.UUID) error {
	for i, id := range ids {
		if i > 0 && bytes.Compare(ids[i-1][:], id[:]) >= 0 {
			return errors.New("lint rules: relation IDs must be unique and sorted")
		}
	}
	return nil
}

func lintPageRuleInput(in app.PageRuleInput, take int) error {
	if take < 1 || take > 101 {
		return errors.New("lint rules: items take must be between 1 and 101")
	}
	if err := lintRelationIDs(in.CandidateIDs); err != nil {
		return err
	}
	return lintRelationIDs(in.ExcludedEmitters)
}

// LintOrphanCount counts at most 10,000 qualified pages in this snapshot.
func (s *Store) LintOrphanCount(ctx context.Context, in app.PageRuleInput) (domain.Count, error) {
	if err := lintPageRuleInput(in, 1); err != nil {
		return domain.Count{}, err
	}
	n, err := s.planned(ctx).LintOrphanCount(ctx, gen.LintOrphanCountParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, ExcludedEmitters: in.ExcludedEmitters})
	if err != nil {
		return domain.Count{}, err
	}
	return domain.Count{Value: n, Capped: n >= 10000}, nil
}

// LintOrphanItems keeps a real zero UUID distinct from the first position.
func (s *Store) LintOrphanItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	if err := lintPageRuleInput(in, take); err != nil {
		return nil, err
	}
	if after == nil {
		return s.planned(ctx).LintOrphanItemsFirst(ctx, gen.LintOrphanItemsFirstParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, ExcludedEmitters: in.ExcludedEmitters, Take: int32(take)})
	}
	return s.planned(ctx).LintOrphanItemsAfter(ctx, gen.LintOrphanItemsAfterParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, ExcludedEmitters: in.ExcludedEmitters, Take: int32(take), AfterID: *after})
}

// LintFrontmatterCount counts at most 10,000 qualified pages in this snapshot.
func (s *Store) LintFrontmatterCount(ctx context.Context, in app.PageRuleInput) (domain.Count, error) {
	if err := lintPageRuleInput(in, 1); err != nil {
		return domain.Count{}, err
	}
	n, err := s.planned(ctx).LintFrontmatterCount(ctx, gen.LintFrontmatterCountParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs})
	if err != nil {
		return domain.Count{}, err
	}
	return domain.Count{Value: n, Capped: n >= 10000}, nil
}

// LintFrontmatterItems keeps a real zero UUID distinct from the first position.
func (s *Store) LintFrontmatterItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	if err := lintPageRuleInput(in, take); err != nil {
		return nil, err
	}
	if after == nil {
		return s.planned(ctx).LintFrontmatterItemsFirst(ctx, gen.LintFrontmatterItemsFirstParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, Take: int32(take)})
	}
	return s.planned(ctx).LintFrontmatterItemsAfter(ctx, gen.LintFrontmatterItemsAfterParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, Take: int32(take), AfterID: *after})
}

// LintMissingTypeCount counts at most 10,000 qualified pages in this snapshot.
func (s *Store) LintMissingTypeCount(ctx context.Context, in app.PageRuleInput) (domain.Count, error) {
	if err := lintPageRuleInput(in, 1); err != nil {
		return domain.Count{}, err
	}
	n, err := s.planned(ctx).LintMissingTypeCount(ctx, gen.LintMissingTypeCountParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars})
	if err != nil {
		return domain.Count{}, err
	}
	return domain.Count{Value: n, Capped: n >= 10000}, nil
}

// LintMissingTypeItems keeps a real zero UUID distinct from the first position.
func (s *Store) LintMissingTypeItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	if err := lintPageRuleInput(in, take); err != nil {
		return nil, err
	}
	if after == nil {
		return s.planned(ctx).LintMissingTypeItemsFirst(ctx, gen.LintMissingTypeItemsFirstParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars, Take: int32(take)})
	}
	return s.planned(ctx).LintMissingTypeItemsAfter(ctx, gen.LintMissingTypeItemsAfterParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars, Take: int32(take), AfterID: *after})
}

// LintMissingSummaryCount counts at most 10,000 qualified pages in this snapshot.
func (s *Store) LintMissingSummaryCount(ctx context.Context, in app.PageRuleInput) (domain.Count, error) {
	if err := lintPageRuleInput(in, 1); err != nil {
		return domain.Count{}, err
	}
	n, err := s.planned(ctx).LintMissingSummaryCount(ctx, gen.LintMissingSummaryCountParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars})
	if err != nil {
		return domain.Count{}, err
	}
	return domain.Count{Value: n, Capped: n >= 10000}, nil
}

// LintMissingSummaryItems keeps a real zero UUID distinct from the first position.
func (s *Store) LintMissingSummaryItems(ctx context.Context, in app.PageRuleInput, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	if err := lintPageRuleInput(in, take); err != nil {
		return nil, err
	}
	if after == nil {
		return s.planned(ctx).LintMissingSummaryItemsFirst(ctx, gen.LintMissingSummaryItemsFirstParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars, Take: int32(take)})
	}
	return s.planned(ctx).LintMissingSummaryItemsAfter(ctx, gen.LintMissingSummaryItemsAfterParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, CandidateIds: in.CandidateIDs, TrimChars: lintTrimChars, Take: int32(take), AfterID: *after})
}

// LintPendingAssets finds all candidates without a current body parent in F.
func (s *Store) LintPendingAssets(ctx context.Context, in app.PendingAssetInput, ids []uuid.UUID) ([]uuid.UUID, error) {
	if err := lintIDs(ids); err != nil {
		return nil, err
	}
	if err := lintRelationIDs(in.ParentCandidates); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.planned(ctx).LintPendingAssets(ctx, gen.LintPendingAssetsParams{NotebookID: in.NotebookID, Extractor: domain.Extractor, ParentIds: in.ParentCandidates, AssetIds: ids})
}
