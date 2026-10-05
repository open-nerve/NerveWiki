package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// LinkPath is a page and its path from the root, itself last (M6/P3 design
// 3.3).
type LinkPath struct {
	ID    uuid.UUID
	Steps []LinkStep
}

// LinkStep is a page on a path: its id and title key.
type LinkStep struct {
	ID  uuid.UUID
	Key string
}

// LinkTargetsByKeys is the pages not deleted of notebookID whose title key
// is one of keys, with their paths.
func (s *Store) LinkTargetsByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]LinkPath, error) {
	rows, err := s.queries(ctx).LinkTargetsByKeys(ctx, gen.LinkTargetsByKeysParams{NotebookID: notebookID, Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("link targets by keys: %w", err)
	}
	return linkPaths(rows)
}

// LinkTargetsByIDs is the pages not deleted of notebookID among ids, with
// their paths.
func (s *Store) LinkTargetsByIDs(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkPath, error) {
	rows, err := s.queries(ctx).LinkTargetsByIDs(ctx, gen.LinkTargetsByIDsParams{NotebookID: notebookID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("link targets by ids: %w", err)
	}
	same := make([]gen.LinkTargetsByKeysRow, len(rows))
	for i, r := range rows {
		same[i] = gen.LinkTargetsByKeysRow(r)
	}
	return linkPaths(same)
}

// linkPaths groups rows, each a step of a page's path, the root's first,
// into the pages' paths. A path that does not reach a root is a defect: the
// bound of the query cut a loop.
func linkPaths(rows []gen.LinkTargetsByKeysRow) ([]LinkPath, error) {
	var out []LinkPath
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].ID != r.PageID {
			if r.ParentID != nil {
				return nil, fmt.Errorf("the path of page %s does not reach a root", r.PageID)
			}
			out = append(out, LinkPath{ID: r.PageID})
		}
		last := &out[len(out)-1]
		last.Steps = append(last.Steps, LinkStep{ID: r.ID, Key: r.NameKey})
	}
	return out, nil
}

// SetNameKeys sets each of nodes' title key to its NameKey; a node it does
// not find, deleted or none, is an error.
func (s *Store) SetNameKeys(ctx context.Context, nodes []domain.Node) error {
	var p gen.SetNameKeysParams
	for _, n := range nodes {
		p.Ids = append(p.Ids, n.ID)
		p.NameKeys = append(p.NameKeys, n.NameKey)
	}
	n, err := s.queries(ctx).SetNameKeys(ctx, p)
	if err != nil {
		return fmt.Errorf("set name keys: %w", err)
	}
	if int(n) != len(nodes) {
		return fmt.Errorf("set the name keys of %d nodes, of %d", n, len(nodes))
	}
	return nil
}
