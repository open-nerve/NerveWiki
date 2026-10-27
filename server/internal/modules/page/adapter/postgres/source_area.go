package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const sourceLockSpace = 0x73726365 // "srce", separate from the linking index

// SourceHeads reads the requested source metadata in the caller's transaction.
func (s *Store) SourceHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	if len(ids) > domain.MaxUnitNodes {
		return nil, errors.New("source heads exceed the node bound")
	}
	out := make(map[uuid.UUID]app.SourceValue, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.planned(ctx).SourceHeads(ctx, gen.SourceHeadsParams{NotebookID: notebook, NodeIds: ids})
	if err != nil {
		return nil, fmt.Errorf("read source heads: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = app.SourceValue{ID: row.ID, Kind: domain.Kind(row.Kind), Hash: row.ContentHash,
			BeforeSourceHash: row.BeforeSourceHash, Bytes: row.ByteSize}
	}
	return out, nil
}

// ProbeSources finds bounded matches by source key and ID cursor.
func (s *Store) ProbeSources(ctx context.Context, notebook uuid.UUID, probes []app.SourceProbe) ([]app.SourceMatch, error) {
	if len(probes) > domain.MaxUnitNodes {
		return nil, errors.New("source probes exceed the node bound")
	}
	if len(probes) == 0 {
		return nil, nil
	}
	args := gen.ProbeSourcesParams{NotebookID: notebook}
	total := 0
	for _, probe := range probes {
		if probe.Take <= 0 || probe.Take > domain.MaxUnitNodes-total {
			return nil, errors.New("source probes require positive takes with a bounded total")
		}
		total += probe.Take
		args.Kinds = append(args.Kinds, string(probe.Key.Kind))
		args.Hashes = append(args.Hashes, probe.Key.Hash[:])
		var after uuid.UUID
		if probe.After != nil {
			after = *probe.After
		}
		args.AfterIds = append(args.AfterIds, after)
		args.HasAfters = append(args.HasAfters, probe.After != nil)
		args.Takes = append(args.Takes, int32(probe.Take))
	}
	rows, err := s.planned(ctx).ProbeSources(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("probe sources: %w", err)
	}
	out := make([]app.SourceMatch, len(rows))
	for i, row := range rows {
		out[i] = app.SourceMatch{Probe: int(row.Probe) - 1, ID: row.ID}
	}
	return out, nil
}

// SetSourceHashes writes changed live nodes in the selected notebook.
func (s *Store) SetSourceHashes(ctx context.Context, notebook uuid.UUID, values []app.SourceValue) (map[uuid.UUID]app.SourceValue, error) {
	if len(values) > domain.MaxUnitNodes {
		return nil, errors.New("source hash writes exceed the node bound")
	}
	out := map[uuid.UUID]app.SourceValue{}
	if len(values) == 0 {
		return out, nil
	}
	args := gen.SetSourceHashesParams{NotebookID: notebook}
	for _, value := range values {
		if value.Hash != nil && len(value.Hash) != 32 {
			return nil, errors.New("a source hash must be NULL or 32 bytes")
		}
		args.NodeIds = append(args.NodeIds, value.ID)
		args.Hashes = append(args.Hashes, value.Hash)
	}
	rows, err := s.planned(ctx).SetSourceHashes(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("set source hashes: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = app.SourceValue{ID: row.ID, Kind: domain.Kind(row.Kind), BeforeSourceHash: row.BeforeSourceHash}
	}
	return out, nil
}

// SourceBaselines reads only derived node keys, without page or asset metadata.
func (s *Store) SourceBaselines(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	if len(ids) > domain.MaxUnitNodes {
		return nil, errors.New("source baselines exceed the node bound")
	}
	out := map[uuid.UUID]app.SourceValue{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.planned(ctx).SourceBaselines(ctx, gen.SourceBaselinesParams{NotebookID: notebook, NodeIds: ids})
	if err != nil {
		return nil, fmt.Errorf("read source baselines: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = app.SourceValue{ID: row.ID, Kind: domain.Kind(row.Kind), BeforeSourceHash: row.BeforeSourceHash}
	}
	return out, nil
}

// LockSources serializes source first writes within a transaction.
func (s *Store) LockSources(ctx context.Context, notebook uuid.UUID) error {
	if !postgres.InTx(ctx) {
		return errors.New("the source lock is taken outside a transaction")
	}
	if err := s.queries(ctx).LockSources(ctx, gen.LockSourcesParams{Space: sourceLockSpace, Key: SourceLockKey(notebook)}); err != nil {
		return fmt.Errorf("lock sources: %w", err)
	}
	return nil
}

// SourceLockKey is the notebook's second advisory key. Hash collisions only
// serialize unrelated notebooks; they cannot let two writes miss each other.
func SourceLockKey(notebook uuid.UUID) int32 {
	h := fnv.New32a()
	_, _ = h.Write(notebook[:])
	return int32(h.Sum32())
}

// ForgetSources clears the notebook's live derived source hashes.
func (s *Store) ForgetSources(ctx context.Context, notebook uuid.UUID) error {
	if err := s.queries(ctx).ForgetSources(ctx, notebook); err != nil {
		return fmt.Errorf("forget source hashes: %w", err)
	}
	return nil
}
