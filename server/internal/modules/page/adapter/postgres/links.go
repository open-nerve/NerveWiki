package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// LinkPath is a page or an attachment and its path from the root, itself
// last (M6/P3 design 3.3; M7/P3 design 4.3); Asset tells an attachment.
type LinkPath struct {
	ID    uuid.UUID
	Steps []LinkStep
	Asset bool
}

// LinkStep is a node on a path, a page but for an attachment's last: its
// id, title key and name.
type LinkStep struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// LinkTargetsByKeys is the pages and attachments not deleted of notebookID
// whose title key is one of keys, with their paths.
func (s *Store) LinkTargetsByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]LinkPath, error) {
	rows, err := s.queries(ctx).LinkTargetsByKeys(ctx, gen.LinkTargetsByKeysParams{NotebookID: notebookID, Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("link targets by keys: %w", err)
	}
	return linkPaths(rows)
}

// LinkTargetsByIDs is the pages and attachments not deleted of notebookID
// among ids, with their paths, read planned with ids: without statistics,
// a plan for any compared each node with them one by one, 10,000 of 55,500
// nodes 0.7 s where theirs took 40 ms (M6 closeout FA5-Q1).
func (s *Store) LinkTargetsByIDs(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkPath, error) {
	rows, err := s.planned(ctx).LinkTargetsByIDs(ctx, gen.LinkTargetsByIDsParams{NotebookID: notebookID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("link targets by ids: %w", err)
	}
	same := make([]gen.LinkTargetsByKeysRow, len(rows))
	for i, r := range rows {
		same[i] = gen.LinkTargetsByKeysRow(r)
	}
	return linkPaths(same)
}

// AttachmentPath is an attachment with its path, and the number of its
// notebook's attachments with its title key, itself among them, to 2.
type AttachmentPath struct {
	LinkPath
	Alike int
}

// AttachmentsByIDs is the attachments not deleted of notebookID among ids,
// with their paths and how many attachments have each's title key, in one
// statement (M7/P3 design 4.6), planned with ids as LinkTargetsByIDs is.
func (s *Store) AttachmentsByIDs(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]AttachmentPath, error) {
	rows, err := s.planned(ctx).AttachmentsByIDs(ctx, gen.AttachmentsByIDsParams{NotebookID: notebookID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("attachments by ids: %w", err)
	}
	same := make([]gen.LinkTargetsByKeysRow, len(rows))
	alike := make(map[uuid.UUID]int, len(ids))
	for i, r := range rows {
		same[i] = gen.LinkTargetsByKeysRow{PageID: r.PageID, ID: r.ID, ParentID: r.ParentID, Name: r.Name, NameKey: r.NameKey,
			Kind: r.Kind, Up: r.Up}
		if r.Up == 0 {
			alike[r.ID] = int(r.Alike)
		}
	}
	paths, err := linkPaths(same)
	if err != nil {
		return nil, err
	}
	out := make([]AttachmentPath, len(paths))
	for i, p := range paths {
		out[i] = AttachmentPath{LinkPath: p, Alike: alike[p.ID]}
	}
	return out, nil
}

// linkPaths groups rows, each a step of a node's path, the root's first,
// into the nodes' paths, each node's kind its own step's, the last. A path
// that does not reach a root is a defect: it meets a deleted node, or the
// bound of the query cut a loop.
func linkPaths(rows []gen.LinkTargetsByKeysRow) ([]LinkPath, error) {
	var out []LinkPath
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].ID != r.PageID {
			if r.ParentID != nil {
				return nil, fmt.Errorf("the path of node %s does not reach a root", r.PageID)
			}
			out = append(out, LinkPath{ID: r.PageID})
		}
		last := &out[len(out)-1]
		last.Steps = append(last.Steps, LinkStep{ID: r.ID, Key: r.NameKey, Name: r.Name})
		last.Asset = r.Kind == string(domain.KindAsset)
	}
	return out, nil
}

// SetNameKeys sets each of nodes' title key to its NameKey, which may be
// another's old one: first to a key of its own, then to its new one. A node
// it does not find, deleted or none, is an error.
func (s *Store) SetNameKeys(ctx context.Context, nodes []domain.Node) error {
	var p gen.SetNameKeysParams
	for _, n := range nodes {
		p.Ids = append(p.Ids, n.ID)
		p.NameKeys = append(p.NameKeys, n.NameKey)
	}
	q := s.queries(ctx)
	unset, err := q.UnsetNameKeys(ctx, p.Ids)
	if err != nil {
		return fmt.Errorf("unset name keys: %w", err)
	}
	set, err := q.SetNameKeys(ctx, p)
	if err != nil {
		return fmt.Errorf("set name keys: %w", err)
	}
	if int(unset) != len(nodes) || int(set) != len(nodes) {
		return fmt.Errorf("set the name keys of %d nodes, of %d", min(unset, set), len(nodes))
	}
	return nil
}
