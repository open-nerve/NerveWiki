package app_test

import (
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// fakeHistory is the history in memory beside the store: the changesets a
// test made, with their items and versions; the versions' contents by page
// and revision; the nodes and contents now are the store's. Each read is a
// call, its count of ids or refs with it where it takes them; the
// recorder's before runs another transaction's commit before a read.
type fakeHistory struct {
	*recorder
	store    *fakeStore
	notebook uuid.UUID
	sets     []*fakeChangeset
	versions map[domain.VersionRef]string
}

// fakeChangeset is a changeset the history holds: gone when its notebook
// was deleted.
type fakeChangeset struct {
	domain.Changeset
	items    []domain.ChangesetItem
	versions []domain.ChangesetVersion
	gone     bool
}

// changeset adds a changeset of eng made by by at at with its items and
// versions, as the units would have written them, without recording a
// call: each version's base's size is its content's, when the history
// holds it.
func (h *fakeHistory) changeset(kind domain.ChangesetKind, by uuid.UUID, at time.Time, items []domain.ChangesetItem,
	versions ...domain.ChangesetVersion) *fakeChangeset {
	c := &fakeChangeset{Changeset: domain.Changeset{ID: uuid.NewV7(), NotebookID: h.notebook, Kind: kind, Client: domain.ClientWeb,
		By: by, CreatedAt: at, UpdatedAt: at}, items: items}
	for _, v := range versions {
		if v.Base != nil {
			v.BaseBytes = len(h.versions[domain.VersionRef{NodeID: v.NodeID, Revision: *v.Base}])
		}
		c.versions = append(c.versions, v)
	}
	h.sets = append(h.sets, c)
	return c
}

// version holds content as the page id's version of revision, and is the
// changeset's version of it on base (0: none).
func (h *fakeHistory) version(id uuid.UUID, base, revision int, content string) domain.ChangesetVersion {
	h.versions[domain.VersionRef{NodeID: id, Revision: revision}] = content
	v := domain.ChangesetVersion{NodeID: id, Revision: revision, Hash: hashOf(content)}
	if base > 0 {
		v.Base = &base
	}
	return v
}

func hashOf(content string) []byte {
	sum := sha256.Sum256([]byte(content))
	return sum[:]
}

func (h *fakeHistory) find(id uuid.UUID) *fakeChangeset {
	for _, c := range h.sets {
		if c.ID == id && !c.gone {
			return c
		}
	}
	return nil
}

// logged is c as the log reads it: the brief of the one it reverts while
// the history holds it.
func (h *fakeHistory) logged(c *fakeChangeset) domain.Changeset {
	out := c.Changeset
	out.Reverts = nil
	if v := h.find(c.RevertsID); c.RevertsID != (uuid.UUID{}) && v != nil {
		out.Reverts = &domain.ChangesetBrief{ID: v.ID, Kind: v.Kind, By: v.By, CreatedAt: v.CreatedAt}
	}
	return out
}

func (h *fakeHistory) FindChangeset(ctx context.Context, id uuid.UUID) (domain.Changeset, error) {
	h.record(ctx, "FindChangeset")
	c := h.find(id)
	if c == nil {
		return domain.Changeset{}, app.ErrNotFound
	}
	return h.logged(c), nil
}

// touches reports whether c changed the node id: an item or a version.
func (c *fakeChangeset) touches(id uuid.UUID) bool {
	return slices.ContainsFunc(c.items, func(i domain.ChangesetItem) bool { return i.NodeID == id }) ||
		slices.ContainsFunc(c.versions, func(v domain.ChangesetVersion) bool { return v.NodeID == id })
}

// ListChangesets lists as the statements do; its call names the filter.
func (h *fakeHistory) ListChangesets(ctx context.Context, notebookID uuid.UUID, f domain.LogFilter, below domain.ChangesetCursor,
	limit int) ([]domain.Changeset, error) {
	call := []string{"ListChangesets"}
	if f.Actor != nil {
		call = append(call, "actor")
	}
	if f.Client != "" {
		call = append(call, "client "+string(f.Client))
	}
	if f.AllMCP {
		call = append(call, "mcp")
	}
	if f.PageID != nil {
		call = append(call, "page")
	}
	h.record(ctx, strings.Join(call, " "))
	var out []domain.Changeset
	for _, c := range h.sets {
		if c.gone || c.NotebookID != notebookID || cmp.Or(c.CreatedAt.Compare(below.At), c.ID.Compare(below.ID)) >= 0 ||
			f.Actor != nil && c.By != *f.Actor || f.Client != "" && c.Client != f.Client ||
			f.AllMCP && !strings.HasPrefix(string(c.Client), "mcp:") || f.PageID != nil && !c.touches(*f.PageID) {
			continue
		}
		out = append(out, h.logged(c))
	}
	slices.SortFunc(out, func(a, b domain.Changeset) int { return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), b.ID.Compare(a.ID)) })
	return out[:min(limit, len(out))], nil
}

func (h *fakeHistory) ChangesetFacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error) {
	h.record(ctx, fmt.Sprintf("ChangesetFacts %d", len(ids)))
	out := map[uuid.UUID]domain.ChangesetFacts{}
	for _, id := range ids {
		c := h.find(id)
		if c == nil {
			continue
		}
		var reverts []*fakeChangeset
		for _, r := range h.sets {
			if r.RevertsID == id && !r.gone {
				reverts = append(reverts, r)
			}
		}
		slices.SortFunc(reverts, func(a, b *fakeChangeset) int { return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), b.ID.Compare(a.ID)) })
		facts := domain.ChangesetFacts{Nodes: min(len(c.items), domain.MaxLogCount), Contents: min(len(c.versions), domain.MaxLogCount)}
		for _, r := range reverts[:min(len(reverts), domain.MaxRevertedBy)] {
			facts.RevertedBy = append(facts.RevertedBy, r.ID)
		}
		out[id] = facts
	}
	return out, nil
}

// ChangesetMoves are what the changeset's units marked: whether any of
// its items renames or moves a node.
func (h *fakeHistory) ChangesetMoves(ctx context.Context, id uuid.UUID) (domain.ChangesetMoves, error) {
	h.record(ctx, "ChangesetMoves")
	var out domain.ChangesetMoves
	if c := h.find(id); c != nil {
		for _, i := range c.items {
			if i.Before != nil && i.After != nil {
				out.Renames = out.Renames || i.Before.Name != i.After.Name
				out.Moves = out.Moves || !domain.SameParent(i.Before.ParentID, i.After.ParentID)
			}
		}
	}
	return out, nil
}

// changes are c's changes by node id after after, at most limit, as the
// statements read them: each node as it is now, its parents' names.
func (h *fakeHistory) changes(c *fakeChangeset, after uuid.UUID, limit int) []domain.ChangeRow {
	ids := map[uuid.UUID]bool{}
	for _, i := range c.items {
		ids[i.NodeID] = true
	}
	for _, v := range c.versions {
		ids[v.NodeID] = true
	}
	sorted := slices.SortedFunc(func(yield func(uuid.UUID) bool) {
		for id := range ids {
			if id.Compare(after) > 0 && !yield(id) {
				return
			}
		}
	}, uuid.UUID.Compare)
	place := func(s *domain.TreeState) *domain.ChangePlace {
		if s == nil {
			return nil
		}
		p := &domain.ChangePlace{ParentID: s.ParentID, Name: s.Name, SortOrder: s.SortOrder}
		if s.ParentID != nil {
			if parent := h.store.anyNode(*s.ParentID); parent.ID != (uuid.UUID{}) {
				p.ParentName = &parent.Name
			}
		}
		return p
	}
	var out []domain.ChangeRow
	for _, id := range sorted[:min(limit, len(sorted))] {
		n := h.store.anyNode(id)
		row := domain.ChangeRow{NodeID: id, NodeKind: n.Kind, Name: n.Name, Alive: n.DeletedAt == nil && n.ID != (uuid.UUID{})}
		if i := slices.IndexFunc(c.items, func(i domain.ChangesetItem) bool { return i.NodeID == id }); i >= 0 {
			row.Before, row.After, row.Restored = place(c.items[i].Before), place(c.items[i].After), c.items[i].Restored
		}
		if i := slices.IndexFunc(c.versions, func(v domain.ChangesetVersion) bool { return v.NodeID == id }); i >= 0 {
			row.Content = &domain.ChangeVersion{Base: c.versions[i].Base, Revision: c.versions[i].Revision}
		}
		out = append(out, row)
	}
	return out
}

func (h *fakeHistory) ListChanges(ctx context.Context, id, after uuid.UUID, limit int) ([]domain.ChangeRow, error) {
	h.record(ctx, "ListChanges")
	if c := h.find(id); c != nil {
		return h.changes(c, after, limit), nil
	}
	return nil, nil
}

func (h *fakeHistory) FirstChanges(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]domain.ChangeRow, error) {
	h.record(ctx, fmt.Sprintf("FirstChanges %d %d", len(ids), n))
	out := map[uuid.UUID][]domain.ChangeRow{}
	for _, id := range ids {
		if c := h.find(id); c != nil {
			out[id] = h.changes(c, uuid.UUID{}, n)
		}
	}
	return out, nil
}

func (h *fakeHistory) CountItems(ctx context.Context, id uuid.UUID, limit int) (int, error) {
	h.record(ctx, "CountItems")
	if c := h.find(id); c != nil {
		return min(len(c.items), limit), nil
	}
	return 0, nil
}

func (h *fakeHistory) ChangesetItems(ctx context.Context, id uuid.UUID) ([]domain.ChangesetItem, error) {
	h.record(ctx, "ChangesetItems")
	if c := h.find(id); c != nil {
		out := slices.Clone(c.items)
		slices.SortFunc(out, func(a, b domain.ChangesetItem) int { return a.NodeID.Compare(b.NodeID) })
		return out, nil
	}
	return nil, nil
}

func (h *fakeHistory) ChangesetItemsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetItem, error) {
	h.record(ctx, fmt.Sprintf("ChangesetItemsOf %d", len(ids)))
	var out []domain.ChangesetItem
	if c := h.find(id); c != nil {
		for _, it := range c.items {
			if slices.Contains(ids, it.NodeID) {
				out = append(out, it)
			}
		}
		slices.SortFunc(out, func(a, b domain.ChangesetItem) int { return a.NodeID.Compare(b.NodeID) })
	}
	return out, nil
}

func (h *fakeHistory) ChangesetVersionsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetVersion, error) {
	h.record(ctx, fmt.Sprintf("ChangesetVersionsOf %d", len(ids)))
	var out []domain.ChangesetVersion
	if c := h.find(id); c != nil {
		for _, v := range c.versions {
			if slices.Contains(ids, v.NodeID) {
				out = append(out, v)
			}
		}
		slices.SortFunc(out, func(a, b domain.ChangesetVersion) int { return a.NodeID.Compare(b.NodeID) })
	}
	return out, nil
}

func (h *fakeHistory) ChangesetVersions(ctx context.Context, id uuid.UUID) ([]domain.ChangesetVersion, error) {
	h.record(ctx, "ChangesetVersions")
	if c := h.find(id); c != nil {
		out := slices.Clone(c.versions)
		slices.SortFunc(out, func(a, b domain.ChangesetVersion) int { return a.NodeID.Compare(b.NodeID) })
		return out, nil
	}
	return nil, nil
}

// nodeItem is a changeset's item of a node, the history's or one the store
// recorded: a deletion, a restore, or neither; when it was first written
// and last, its changeset's time for the history's.
type nodeItem struct {
	changeset          uuid.UUID
	first, at          time.Time
	deletion, restored bool
}

// itemsOf are the node id's items.
func (h *fakeHistory) itemsOf(id uuid.UUID) []nodeItem {
	var out []nodeItem
	for _, c := range h.sets {
		for _, it := range c.items {
			if it.NodeID == id {
				out = append(out, nodeItem{changeset: c.ID, first: c.CreatedAt, at: c.CreatedAt, deletion: it.After == nil,
					restored: it.Restored})
			}
		}
	}
	for _, it := range h.store.recorded {
		if it.Change.NodeID == id {
			out = append(out, nodeItem{changeset: it.ChangesetID, first: h.store.firsts[[2]uuid.UUID{it.ChangesetID, id}], at: it.At,
				deletion: it.Change.After == nil, restored: it.Change.Restored != nil})
		}
	}
	return out
}

// deletedByRevert reports whether the node id's deletion at at undid the
// restore of the trash item the changeset reverted made, as NodesNow reads
// it, by the items' own times (M8/P4 design 3.7): an item of a revert the
// store made deletes the node at at, and that revert reverted a changeset
// whose item restored the node, first written after the deletion of the
// changeset reverted's item, with no deletion of the node between them.
func (h *fakeHistory) deletedByRevert(id uuid.UUID, at time.Time, reverted uuid.UUID) bool {
	items := h.itemsOf(id)
	d := slices.IndexFunc(items, func(i nodeItem) bool { return i.changeset == reverted && i.deletion })
	if d < 0 {
		return false
	}
	from := items[d].at
	undid := func(c, r uuid.UUID) bool {
		deletes := slices.ContainsFunc(items, func(i nodeItem) bool { return i.changeset == c && i.deletion && i.at.Equal(at) })
		ri := slices.IndexFunc(items, func(i nodeItem) bool { return i.changeset == r && i.restored })
		if !deletes || ri < 0 || !from.Before(items[ri].first) {
			return false
		}
		to := items[ri].first
		return !slices.ContainsFunc(items, func(i nodeItem) bool { return i.deletion && i.at.After(from) && i.at.Before(to) })
	}
	return slices.ContainsFunc(h.store.changesets, func(c app.Changeset) bool {
		return c.Kind == domain.ChangesetRevert && undid(c.ID, c.RevertsID)
	})
}

// NodesNow reads the store's nodes, alive or in the trash, a root with its
// counts and whether its deletion undid the restore of the trash item the
// changeset reverted made.
func (h *fakeHistory) NodesNow(ctx context.Context, ids []uuid.UUID, reverted uuid.UUID) (map[uuid.UUID]domain.NodeNow, error) {
	h.record(ctx, fmt.Sprintf("NodesNow %d", len(ids)))
	out := map[uuid.UUID]domain.NodeNow{}
	for _, id := range ids {
		n := h.store.anyNode(id)
		if n.ID == (uuid.UUID{}) {
			continue
		}
		now := domain.NodeNow{Node: n.Node, DeletedAt: n.DeletedAt, TrashRoot: n.TrashRoot}
		if n.TrashRoot {
			now.TrashPages, now.TrashAssets = h.store.counts[id][0], h.store.counts[id][1]
			now.DeletedByRevert = h.deletedByRevert(id, *n.DeletedAt, reverted)
		}
		out[id] = now
	}
	return out, nil
}

// ContentsNow reads the store's contents of the pages alive.
func (h *fakeHistory) ContentsNow(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ContentNow, error) {
	h.record(ctx, fmt.Sprintf("ContentsNow %d", len(ids)))
	out := map[uuid.UUID]domain.ContentNow{}
	for _, id := range ids {
		if c, ok := h.store.contents[id]; ok {
			out[id] = domain.ContentNow{Revision: c.Revision, Hash: c.Hash, Bytes: c.ByteSize}
		}
	}
	return out, nil
}

func (h *fakeHistory) VersionHashes(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef][]byte, error) {
	h.record(ctx, fmt.Sprintf("VersionHashes %d", len(refs)))
	out := map[domain.VersionRef][]byte{}
	for _, r := range refs {
		if content, ok := h.versions[r]; ok {
			out[r] = hashOf(content)
		}
	}
	return out, nil
}

func (h *fakeHistory) VersionContents(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef]app.PageContent, error) {
	h.record(ctx, fmt.Sprintf("VersionContents %d", len(refs)))
	out := map[domain.VersionRef]app.PageContent{}
	for _, r := range refs {
		if content, ok := h.versions[r]; ok {
			out[r] = app.PageContent{Content: content, Revision: r.Revision, Hash: hashOf(content)}
		}
	}
	return out, nil
}

// historyReads are the log's reads over the fixture.
func (f *fixture) historyReads() app.HistoryReads {
	return app.HistoryReads{Notebooks: f.notebooks, History: f.log, Names: f.names, Auth: f.auth}
}
