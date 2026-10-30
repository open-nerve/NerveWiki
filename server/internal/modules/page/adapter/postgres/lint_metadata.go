package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// LintNodes reads compact live metadata through the caller's transaction.
func (s *Store) LintNodes(ctx context.Context, notebook uuid.UUID) ([]app.LintNode, error) {
	rows, err := s.planned(ctx).LintNodeMetadata(ctx, notebook)
	if err != nil {
		return nil, fmt.Errorf("lint metadata: %w", err)
	}
	out := make([]app.LintNode, len(rows))
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if domain.Kind(row.Kind) == domain.KindPage && !row.HasContent {
			return nil, fmt.Errorf("lint metadata: live page has no content (notebook_id=%s node_id=%s)", notebook, row.ID)
		}
		out[i] = app.LintNode{ID: row.ID, ParentID: row.ParentID, Kind: domain.Kind(row.Kind),
			NameKey: row.NameKey, Linkable: row.Linkable, CreatedAt: row.CreatedAt, PageBytes: row.PageBytes}
	}
	return out, nil
}

// SourceNodes reads the complete strict live descendants of the proposed root.
func (s *Store) SourceNodes(ctx context.Context, notebook, root uuid.UUID) ([]app.SourceNode, error) {
	nodes, err := s.LintNodes(ctx, notebook)
	if err != nil {
		return nil, err
	}
	return lintSourceNodes(ctx, notebook, root, nodes)
}

// LintSourceData retains the same read for both strict membership and metadata.
// No state survives this call or is shared by another transaction.
func (s *Store) LintSourceData(ctx context.Context, notebook, root uuid.UUID) (app.SourceData, error) {
	nodes, err := s.LintNodes(ctx, notebook)
	if err != nil {
		return app.SourceData{}, err
	}
	sources, err := lintSourceNodes(ctx, notebook, root, nodes)
	if err != nil {
		return app.SourceData{}, err
	}
	if err := ctx.Err(); err != nil {
		return app.SourceData{}, err
	}
	return app.SourceData{Sources: sources, Nodes: nodes}, nil
}

func lintSourceNodes(ctx context.Context, notebook, root uuid.UUID, nodes []app.LintNode) ([]app.SourceNode, error) {
	byID := make(map[uuid.UUID]int, len(nodes))
	for i, node := range nodes {
		byID[node.ID] = i
	}
	rootIndex, ok := byID[root]
	if !ok || nodes[rootIndex].Kind != domain.KindPage {
		return nil, fmt.Errorf("source area: root is not a live page (notebook_id=%s node_id=%s)", notebook, root)
	}

	// Validate the whole live forest before selecting members. Starting at
	// notebook roots makes the existing depth limit independent of root's
	// position; a disconnected cycle or orphan cannot yield a partial area.
	children := make(map[uuid.UUID][]int, len(nodes))
	depth := make([]int, len(nodes))
	queue := make([]int, 0, len(nodes))
	for i, node := range nodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if node.ParentID == nil {
			queue = append(queue, i)
			depth[i] = 1
			continue
		}
		parent, exists := byID[*node.ParentID]
		if !exists || nodes[parent].Kind != domain.KindPage {
			return nil, fmt.Errorf("source area: missing live page ancestor (notebook_id=%s node_id=%s)", notebook, node.ID)
		}
		children[*node.ParentID] = append(children[*node.ParentID], i)
	}
	for at := 0; at < len(queue); at++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := queue[at]
		maxDepth := domain.MaxDepth
		if nodes[i].Kind == domain.KindAsset {
			maxDepth++ // An attachment may sit below the deepest legal page.
		}
		if depth[i] > maxDepth {
			return nil, fmt.Errorf("source area: live tree exceeds its depth (notebook_id=%s node_id=%s)", notebook, nodes[i].ID)
		}
		for _, child := range children[nodes[i].ID] {
			depth[child] = depth[i] + 1
			queue = append(queue, child)
		}
	}
	if len(queue) != len(nodes) {
		return nil, fmt.Errorf("source area: live tree contains a cycle (notebook_id=%s node_id=%s)", notebook, root)
	}

	// A new traversal starts at root's children, so root itself is never a
	// source. There is no member cap: the write-unit 10k bound does not apply.
	queue = append(queue[:0], children[root]...)
	out := make([]app.SourceNode, 0, len(queue))
	for at := 0; at < len(queue); at++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := nodes[queue[at]]
		out = append(out, app.SourceNode{ID: node.ID, Kind: node.Kind, CreatedAt: node.CreatedAt, PageBytes: node.PageBytes})
		queue = append(queue, children[node.ID]...)
	}
	return out, nil
}
