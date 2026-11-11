package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SourceRebuildStore is what a source area's rebuild reads and writes, in
// its caller's transaction: the source area's statements (sources.sql,
// LintNodeMetadata) and the rebuild's own heads (M11/P3 decision 292).
type SourceRebuildStore interface {
	ForgetSources(ctx context.Context, notebook uuid.UUID) error
	SourceNodes(ctx context.Context, notebook, root uuid.UUID) ([]SourceNode, error)
	SourceRebuildHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]SourceValue, error)
	SetSourceHashes(ctx context.Context, notebook uuid.UUID, values []SourceValue) (map[uuid.UUID]SourceValue, error)
}

// RebuildSources sets notebook's source hashes as an LLM Wiki whose source
// root is root wants them (M10 design 4.8, M11 design 4.9 step 4), in the
// caller's transaction, which holds the notebook's row: every live node's
// cleared, then each strict live descendant's of root, a page with content
// its content's hash, an attachment its file's SHA-256, a zero-byte one's
// too; an empty page none, as the units keep them (unit_sources.go). It
// answers how many it set. The pages' heads, then the attachments' files,
// then the writes, MaxUnitNodes a chunk, by id. A head or an attachment's
// file missing, or a chunk's write that changed another number of rows
// than it holds (each was cleared, so each changes), is an error, which
// rolls the caller back (M11 design 4.9: not skipped, review B-N6); the
// store's errors are wrapped with %w, so that a deadline stays one
// (M11/P3 design 3.4).
func RebuildSources(ctx context.Context, s SourceRebuildStore, attachments AttachmentInfos, notebook, root uuid.UUID) (int, error) {
	if err := s.ForgetSources(ctx, notebook); err != nil {
		return 0, fmt.Errorf("rebuild the source hashes: %w", err)
	}
	nodes, err := s.SourceNodes(ctx, notebook, root)
	if err != nil {
		return 0, fmt.Errorf("rebuild the source hashes: %w", err)
	}
	var pages, assets []uuid.UUID
	for _, n := range nodes {
		switch {
		case n.Kind == domain.KindPage && n.PageBytes > 0:
			pages = append(pages, n.ID)
		case n.Kind == domain.KindAsset:
			assets = append(assets, n.ID)
		}
	}
	slices.SortFunc(pages, uuid.UUID.Compare)
	slices.SortFunc(assets, uuid.UUID.Compare)
	values := make([]SourceValue, 0, len(pages)+len(assets))
	for ids := range slices.Chunk(pages, domain.MaxUnitNodes) {
		heads, err := s.SourceRebuildHeads(ctx, notebook, ids)
		if err != nil {
			return 0, fmt.Errorf("rebuild the source hashes: %w", err)
		}
		for _, id := range ids {
			v, ok := heads[id]
			if !ok || v.Kind != domain.KindPage {
				return 0, fmt.Errorf("rebuild the source hashes: source metadata missing for node %s", id)
			}
			if len(v.Hash) != sha256.Size || v.Bytes <= 0 {
				return 0, fmt.Errorf("rebuild the source hashes: invalid source metadata for node %s", id)
			}
			values = append(values, SourceValue{ID: id, Kind: domain.KindPage, Hash: v.Hash, Bytes: v.Bytes})
		}
	}
	for ids := range slices.Chunk(assets, domain.MaxUnitNodes) {
		infos, err := attachments.Of(ctx, ids)
		if err != nil {
			return 0, fmt.Errorf("rebuild the source hashes: %w", err)
		}
		for _, id := range ids {
			info, ok := infos[id]
			if !ok {
				return 0, fmt.Errorf("rebuild the source hashes: source attachment metadata missing for node %s", id)
			}
			if len(info.SHA256) != sha256.Size || info.Bytes < 0 {
				return 0, fmt.Errorf("rebuild the source hashes: invalid source metadata for node %s", id)
			}
			values = append(values, SourceValue{ID: id, Kind: domain.KindAsset, Hash: info.SHA256, Bytes: info.Bytes})
		}
	}
	for chunk := range slices.Chunk(values, domain.MaxUnitNodes) {
		changed, err := s.SetSourceHashes(ctx, notebook, chunk)
		if err != nil {
			return 0, fmt.Errorf("rebuild the source hashes: %w", err)
		}
		if len(changed) != len(chunk) {
			return 0, fmt.Errorf("rebuild the source hashes: a chunk of %d changed %d rows of notebook %s", len(chunk), len(changed), notebook)
		}
	}
	return len(values), nil
}
