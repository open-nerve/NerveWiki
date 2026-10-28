// Package postgresadapter is the page module's repository adapter: sqlc
// queries (queries/, generated into gen/) over the transaction that the
// context carries, or the pool.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The unique indexes on siblings' name keys (00042): the roots' and the
// children's, which a path's segments are read by.
const (
	rootNameKey  = "nodes_notebook_id_name_key_root_key"
	childNameKey = "nodes_parent_id_name_key_key"
)

// Store implements the app's repository ports.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// queries runs in the context's transaction when there is one.
func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// planned uses DescribeExec, avoiding pgx's named statement cache. The
// server's plan_cache_mode still decides whether its plan is generic.
func (s *Store) planned(ctx context.Context) *gen.Queries {
	return gen.New(postgres.Planned(postgres.DB(ctx, s.pool)))
}

// uniqueViolation reports whether err broke one of the unique constraints
// or indexes names.
func uniqueViolation(err error, names ...string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && slices.Contains(names, pgErr.ConstraintName)
}

// nameTaken reports whether err broke a unique index on siblings' name
// keys: the roots' for a root, the children's for a child, as no row is in
// both.
func nameTaken(err error) bool {
	return uniqueViolation(err, rootNameKey, childNameKey)
}

// notFound turns no row into app.ErrNotFound.
func notFound(what string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

// nodeOf is a row of the queries that read a node's columns: they read the
// same, so their rows convert to this one.
func nodeOf(r gen.FindNodeRow) domain.Node {
	return domain.Node{
		ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: domain.Kind(r.Kind), Name: r.Name,
		NameKey: r.NameKey, SortOrder: r.SortOrder, CreatedBy: r.CreatedByID, UpdatedBy: r.UpdatedByID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// FindNode implements app.Nodes.
func (s *Store) FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error) {
	row, err := s.queries(ctx).FindNode(ctx, id)
	if err != nil {
		return domain.Node{}, notFound("find node", err)
	}
	return nodeOf(row), nil
}

// FindNodeIn implements app.Nodes.
func (s *Store) FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error) {
	row, err := s.queries(ctx).FindNodeIn(ctx, gen.FindNodeInParams{ID: id, NotebookID: notebookID})
	if err != nil {
		return domain.Node{}, notFound("find node in notebook", err)
	}
	return nodeOf(gen.FindNodeRow(row)), nil
}

// ListNodes implements app.Nodes.
func (s *Store) ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error) {
	rows, err := s.queries(ctx).ListNodes(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// Children implements app.Nodes.
func (s *Store) Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error) {
	rows, err := s.queries(ctx).Children(ctx, gen.ChildrenParams{NotebookID: notebookID, ParentID: parentID})
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// RootByKey implements app.PathStore.
func (s *Store) RootByKey(ctx context.Context, notebookID uuid.UUID, key string) (domain.Node, bool, error) {
	row, err := s.queries(ctx).RootByKey(ctx, gen.RootByKeyParams{NotebookID: notebookID, NameKey: key})
	return foundNode(gen.FindNodeRow(row), err, "find a root by key")
}

// ChildByKey implements app.PathStore.
func (s *Store) ChildByKey(ctx context.Context, notebookID, parentID uuid.UUID, key string) (domain.Node, bool, error) {
	row, err := s.queries(ctx).ChildByKey(ctx, gen.ChildByKeyParams{NotebookID: notebookID, ParentID: &parentID, NameKey: key})
	return foundNode(gen.FindNodeRow(row), err, "find a child by key")
}

// PathsOf implements app.PathStore: LinkTargetsByIDs' paths, each
// written by domain.PathOfNames.
func (s *Store) PathsOf(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	paths, err := s.LinkTargetsByIDs(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(paths))
	for _, p := range paths {
		names := make([]string, len(p.Steps))
		for i, step := range p.Steps {
			names[i] = step.Name
		}
		kind := domain.KindPage
		if p.Asset {
			kind = domain.KindAsset
		}
		out[p.ID] = domain.PathOfNames(names, kind)
	}
	return out, nil
}

// LinePaths implements app.PathStore: LinkTargetsByIDs' paths, as PathsOf
// reads them, step by step.
func (s *Store) LinePaths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]domain.PathStep, error) {
	paths, err := s.LinkTargetsByIDs(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]domain.PathStep, len(paths))
	for _, p := range paths {
		steps := make([]domain.PathStep, len(p.Steps))
		for i, step := range p.Steps {
			steps[i] = domain.PathStep{ID: step.ID, Key: step.Key, Name: step.Name}
		}
		out[p.ID] = steps
	}
	return out, nil
}

// ChildPageIDs is the pages not deleted of notebookID whose parent is
// parent, the root's for uuid.UUID{}, in no order (M9/P4 design 3.9.1):
// RootPageIDs or ChildPageIDs, each by an index of the parent's children,
// 00012's or 00042's.
func (s *Store) ChildPageIDs(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error) {
	var (
		ids []uuid.UUID
		err error
	)
	if parent == (uuid.UUID{}) {
		ids, err = s.queries(ctx).RootPageIDs(ctx, notebookID)
	} else {
		ids, err = s.queries(ctx).ChildPageIDs(ctx, gen.ChildPageIDsParams{NotebookID: notebookID, ParentID: &parent})
	}
	if err != nil {
		return nil, fmt.Errorf("list the child pages of %s: %w", parent, err)
	}
	return ids, nil
}

// AgentsPage implements app.AgentsStore: an attachment holding the key
// has no revision.
func (s *Store) AgentsPage(ctx context.Context, notebookID uuid.UUID, key string) (uuid.UUID, domain.Kind, string, int, bool, error) {
	row, err := s.queries(ctx).AgentsPage(ctx, gen.AgentsPageParams{NotebookID: notebookID, NameKey: key})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.UUID{}, "", "", 0, false, nil
	case err != nil:
		return uuid.UUID{}, "", "", 0, false, fmt.Errorf("find the AGENTS page: %w", err)
	}
	return row.ID, domain.Kind(row.Kind), row.Name, int(row.Revision), true, nil
}

// foundNode is the node of a lookup's row and error: false for no row.
func foundNode(row gen.FindNodeRow, err error, what string) (domain.Node, bool, error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Node{}, false, nil
	case err != nil:
		return domain.Node{}, false, fmt.Errorf("%s: %w", what, err)
	}
	return nodeOf(row), true, nil
}

// PageStates implements app.PageStates.
func (s *Store) PageStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageState, error) {
	rows, err := s.planned(ctx).PageStates(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("read the pages' states: %w", err)
	}
	out := make(map[uuid.UUID]app.PageState, len(rows))
	for _, r := range rows {
		out[r.NodeID] = app.PageState{Revision: int(r.Revision), Bytes: int(r.ByteSize), UpdatedAt: r.UpdatedAt, Hash: r.ContentHash}
	}
	return out, nil
}

// AliveNodes implements app.PageStates.
func (s *Store) AliveNodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	alive, err := s.planned(ctx).AliveNodes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("read the nodes alive: %w", err)
	}
	out := make(map[uuid.UUID]bool, len(alive))
	for _, id := range alive {
		out[id] = true
	}
	return out, nil
}

// NameCursor is where a list by title key and id goes on: after the node
// of this key and id.
type NameCursor struct {
	Key string
	ID  uuid.UUID
}

// AssetsUnder is the attachments not deleted under parentID (nil: the
// root) of notebookID, by title key and id, after after when it is set, at
// most limit.
func (s *Store) AssetsUnder(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *NameCursor, limit int) (
	[]domain.Node, error,
) {
	params := gen.AssetsUnderParams{NotebookID: notebookID, ParentID: parentID, MaxRows: int32(limit)}
	if after != nil {
		params.AfterKey, params.AfterID = &after.Key, &after.ID
	}
	rows, err := s.queries(ctx).AssetsUnder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// Ancestors implements app.Nodes. A chain that does not reach a root is an
// error: only a defect makes one.
func (s *Store) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error) {
	rows, err := s.queries(ctx).Ancestors(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ancestors: %w", err)
	}
	if len(rows) > 0 && rows[len(rows)-1].ParentID != nil {
		return nil, fmt.Errorf("ancestors of node %s: the chain does not reach a root", id)
	}
	out := make([]domain.Ancestor, len(rows))
	for i, r := range rows {
		out[i] = domain.Ancestor{ID: r.ID, Name: r.Name}
	}
	slices.Reverse(out)
	return out, nil
}

// subtreeLevels bounds Subtree's levels: only a defect could make a chain
// that loops, and the tree is far shallower (domain.MaxDepth).
const subtreeLevels = 64

// Subtree implements app.Nodes: the node, then its descendants level by
// level, each level in order and read in one statement by its parents.
// DescribeExec is used; ChildrenOfAll also supports a forced generic plan.
func (s *Store) Subtree(ctx context.Context, notebookID, id uuid.UUID) (domain.Subtree, error) {
	q, levels := s.queries(ctx), s.planned(ctx)
	root, err := q.FindNodeIn(ctx, gen.FindNodeInParams{ID: id, NotebookID: notebookID})
	if err != nil {
		return nil, notFound("subtree", err)
	}
	out := domain.Subtree{{Level: 1, Node: nodeOf(gen.FindNodeRow(root))}}
	parents := []uuid.UUID{root.ID}
	for level := 2; level <= subtreeLevels && len(parents) > 0; level++ {
		rows, err := levels.ChildrenOfAll(ctx, gen.ChildrenOfAllParams{NotebookID: notebookID, Parents: parents})
		if err != nil {
			return nil, fmt.Errorf("subtree: %w", err)
		}
		parents = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			out = append(out, domain.SubtreeNode{Level: level, Node: nodeOf(gen.FindNodeRow(r))})
			parents[i] = r.ID
		}
	}
	return out, nil
}

// ContentMeta implements app.Nodes.
func (s *Store) ContentMeta(ctx context.Context, id uuid.UUID) (app.ContentMeta, error) {
	row, err := s.queries(ctx).ContentMeta(ctx, id)
	if err != nil {
		return app.ContentMeta{}, notFound("content meta", err)
	}
	return app.ContentMeta{Revision: int(row.Revision), ByteSize: int(row.ByteSize), UpdatedBy: row.UpdatedByID,
		UpdatedAt: row.UpdatedAt}, nil
}

// PageContent implements app.Nodes.
func (s *Store) PageContent(ctx context.Context, id uuid.UUID) (app.PageContent, error) {
	row, err := s.queries(ctx).PageContent(ctx, id)
	if err != nil {
		return app.PageContent{}, notFound("page content", err)
	}
	return app.PageContent{Content: row.Content, Revision: int(row.Revision), Hash: row.ContentHash}, nil
}

// PageContentWithin implements app.Nodes.
func (s *Store) PageContentWithin(ctx context.Context, id uuid.UUID, max int) (app.PageContent, bool, error) {
	row, err := s.queries(ctx).PageContentWithin(ctx, gen.PageContentWithinParams{NodeID: id, MaxBytes: int32(min(max, math.MaxInt32))})
	switch {
	case err != nil:
		return app.PageContent{}, false, notFound("page content within", err)
	case !row.Within:
		return app.PageContent{Revision: int(row.Revision), Hash: row.ContentHash}, false, nil
	}
	return app.PageContent{Content: row.Content, Revision: int(row.Revision), Hash: row.ContentHash}, true, nil
}

// LockContent implements app.Nodes.
func (s *Store) LockContent(ctx context.Context, notebookID, id uuid.UUID) (app.ContentLock, error) {
	row, err := s.queries(ctx).LockContent(ctx, gen.LockContentParams{NodeID: id, NotebookID: notebookID})
	if err != nil {
		return app.ContentLock{}, notFound("lock content", err)
	}
	return app.ContentLock{Revision: int(row.Revision), Hash: row.ContentHash, ByteSize: int(row.ByteSize)}, nil
}

// CreateNode implements app.NodeWriter.
func (s *Store) CreateNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).CreateNode(ctx, gen.CreateNodeParams{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: n.ParentID, Kind: string(n.Kind), Name: n.Name, NameKey: n.NameKey,
		SortOrder: n.SortOrder, By: n.CreatedBy, Now: n.CreatedAt,
	})
	switch {
	case nameTaken(err):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

// CreateContent implements app.NodeWriter.
func (s *Store) CreateContent(ctx context.Context, c app.Content) error {
	if err := s.queries(ctx).CreateContent(ctx, gen.CreateContentParams{
		NodeID: c.NodeID, Content: c.Content, Revision: int32(c.Revision), ContentHash: c.Hash,
		ByteSize: int32(c.ByteSize), By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("create content: %w", err)
	}
	return nil
}

// WriteContent implements app.NodeWriter.
func (s *Store) WriteContent(ctx context.Context, c app.Content) error {
	if err := s.queries(ctx).WriteContent(ctx, gen.WriteContentParams{
		NodeID: c.NodeID, Content: c.Content, Revision: int32(c.Revision), ContentHash: c.Hash,
		ByteSize: int32(c.ByteSize), By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("write content: %w", err)
	}
	return nil
}

// RenameNode implements app.NodeWriter.
func (s *Store) RenameNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).RenameNode(ctx, gen.RenameNodeParams{
		ID: n.ID, Name: n.Name, NameKey: n.NameKey, By: n.UpdatedBy, Now: n.UpdatedAt,
	})
	switch {
	case nameTaken(err):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("rename node: %w", err)
	}
	return nil
}

// MoveNode implements app.NodeWriter.
func (s *Store) MoveNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).MoveNode(ctx, gen.MoveNodeParams{
		ID: n.ID, ParentID: n.ParentID, SortOrder: n.SortOrder, By: n.UpdatedBy, Now: n.UpdatedAt,
	})
	switch {
	case nameTaken(err):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("move node: %w", err)
	}
	return nil
}

// RelocateNode implements app.NodeWriter.
func (s *Store) RelocateNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).RelocateNode(ctx, gen.RelocateNodeParams{
		ID: n.ID, ParentID: n.ParentID, SortOrder: n.SortOrder, Name: n.Name, NameKey: n.NameKey, By: n.UpdatedBy,
		Now: n.UpdatedAt,
	})
	switch {
	case nameTaken(err):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("relocate node: %w", err)
	}
	return nil
}

// DeleteNodes implements app.NodeWriter.
func (s *Store) DeleteNodes(ctx context.Context, d app.Deletion) error {
	stray, err := s.queries(ctx).DeleteNodes(ctx, gen.DeleteNodesParams{
		Ids: d.IDs, By: d.By, Now: d.At, Root: d.Root, Pages: int32(d.Pages), Assets: int32(d.Assets), Height: int32(d.Height),
	})
	switch {
	case err != nil:
		return fmt.Errorf("delete nodes: %w", err)
	case stray != 0:
		return fmt.Errorf("delete nodes: %d nodes deleted at this time already are under them: a unit deletes from the top down", stray)
	}
	return nil
}

// RestoreNodes implements app.NodeWriter. A name a sibling holds already
// is an error of its own, no code of the restore's: the unit numbered
// them under the notebook's lock, so only a defect makes one.
func (s *Store) RestoreNodes(ctx context.Context, r app.Restoration) (app.RestoredRows, error) {
	// Build both arrays in the same order as IDs. The explicit presence bit
	// distinguishes an absent value from a requested NULL without changing
	// ordinary restorations or the existing row/history checks.
	hashes, hasHashes := make([][]byte, len(r.IDs)), make([]bool, len(r.IDs))
	for i, id := range r.IDs {
		hashes[i], hasHashes[i] = r.SourceHashes[id]
		if hashes[i] != nil && len(hashes[i]) != 32 {
			return app.RestoredRows{}, errors.New("a restored source hash must be NULL or 32 bytes")
		}
	}
	row, err := s.queries(ctx).RestoreNodes(ctx, gen.RestoreNodesParams{
		Ids: r.IDs, Names: r.Names, NameKeys: r.NameKeys, By: r.By, Now: r.At, Root: r.Root, ParentID: r.ParentID,
		SortOrder: r.SortOrder, DeletedAt: r.DeletedAt, SourceHashes: hashes, HasSourceHashes: hasHashes,
	})
	switch {
	case nameTaken(err):
		return app.RestoredRows{}, fmt.Errorf("restore nodes: a name a sibling holds, though the unit numbered them: %w", err)
	case err != nil:
		return app.RestoredRows{}, fmt.Errorf("restore nodes: %w", err)
	}
	return app.RestoredRows{Nodes: int(row.Nodes), Contents: int(row.Contents), CurrentVersions: int(row.CurrentVersions),
		Creations: int(row.Creations), Gaps: int(row.Gaps)}, nil
}

// MarkTrashRoots implements app.NodeWriter, planned with its arguments.
func (s *Store) MarkTrashRoots(ctx context.Context, roots []app.TrashMark, at time.Time) (int, error) {
	p := gen.MarkTrashRootsParams{Ids: make([]uuid.UUID, len(roots)), Pages: make([]int32, len(roots)),
		Assets: make([]int32, len(roots)), Heights: make([]int32, len(roots)), DeletedAt: at}
	for i, r := range roots {
		p.Ids[i], p.Pages[i], p.Assets[i], p.Heights[i] = r.ID, int32(r.Pages), int32(r.Assets), int32(r.Height)
	}
	n, err := s.planned(ctx).MarkTrashRoots(ctx, p)
	if err != nil {
		return 0, fmt.Errorf("mark trash roots: %w", err)
	}
	return int(n), nil
}

// SetSortOrder implements app.NodeWriter.
func (s *Store) SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error {
	if err := s.queries(ctx).SetSortOrder(ctx, gen.SetSortOrderParams{ID: id, SortOrder: order}); err != nil {
		return fmt.Errorf("set sort order: %w", err)
	}
	return nil
}

// DeleteNotebooksPages implements app.NotebookPages.
func (s *Store) DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteNotebooksPages(ctx, gen.DeleteNotebooksPagesParams{NotebookIds: ids, By: by, Now: at}); err != nil {
		return fmt.Errorf("delete the notebooks' pages: %w", err)
	}
	return nil
}

// Analyze updates the statistics of the module's tables an import writes
// (M7/P6 design 3.6).
func (s *Store) Analyze(ctx context.Context) error {
	if err := s.queries(ctx).Analyze(ctx); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}
