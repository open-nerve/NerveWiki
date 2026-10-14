package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// changeset adds a changeset of notebook made by by from client at at,
// through the store, and answers it.
func (f fixture) changeset(t *testing.T, notebook, by uuid.UUID, client domain.Client, at time.Time) app.Changeset {
	t.Helper()
	c := app.Changeset{ID: uuid.NewV7(), NotebookID: notebook, Kind: domain.ChangesetEdit, Client: client, By: by, At: at}
	if err := f.s.CreateChangeset(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

// item records the change as cs's item at at.
func (f fixture) item(t *testing.T, cs uuid.UUID, at time.Time, c domain.Change) {
	t.Helper()
	if err := f.s.RecordItems(context.Background(), []app.Item{{ID: uuid.NewV7(), ChangesetID: cs, Change: c, At: at}}); err != nil {
		t.Fatal(err)
	}
}

// version records content as cs's version of the page id on base (nil:
// its creation) to revision.
func (f fixture) version(t *testing.T, cs, id uuid.UUID, base *int, revision int, content string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	if err := f.s.RecordRevision(context.Background(), app.Revision{ID: uuid.NewV7(), ChangesetID: cs, NodeID: id, Base: base,
		Revision: revision, Content: content, Hash: sum[:], ByteSize: len(content), At: now()}); err != nil {
		t.Fatal(err)
	}
	return sum[:]
}

func changesetIDs(cs []domain.Changeset) []uuid.UUID {
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

// The log lists a notebook's changesets of a live notebook newer first,
// by id at one time, below its cursor; each of its three statements keeps
// what its filter keeps: one actor's, one client's or every MCP client's,
// those that changed a node by an item or a version, deleted or not. A
// revert has the brief of what it reverts while that one is there.
func TestListChangesetsNewerFirst(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := f.user(t, "bob@corp.com")
	p := f.page(t, f.eng, nil, "P", 0)
	at := func(m int) time.Time { return now().Add(time.Duration(m) * time.Minute) }
	c1 := f.changeset(t, f.eng, f.alice, domain.ClientWeb, at(1))
	c2 := f.changeset(t, f.eng, bob, "mcp:a", at(2))
	c3 := f.changeset(t, f.eng, f.alice, domain.ClientAPI, at(3))
	c4 := f.changeset(t, f.eng, bob, "mcp:b", at(3))
	c5 := f.changeset(t, f.eng, bob, domain.ClientWeb, at(4))
	gone := f.changeset(t, f.eng, f.alice, domain.ClientWeb, at(5))
	f.exec(t, "UPDATE changesets SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	f.changeset(t, f.ops, f.alice, domain.ClientWeb, at(6))
	revert := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetRevert, RevertsID: c3.ID, Client: domain.ClientCLI,
		By: f.alice, At: at(7)}
	purged := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetRevert, RevertsID: uuid.NewV7(),
		Client: domain.ClientCLI, By: f.alice, At: at(8)}
	for _, c := range []app.Changeset{revert, purged} {
		if err := f.s.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// P: created by c1 (an item), written by c4 (a version, in the trash since).
	state := p.State()
	f.item(t, c1.ID, at(1), domain.Change{NodeID: p.ID, After: &state, Revision: 1})
	f.version(t, c4.ID, p.ID, new(1), 2, "two")
	f.exec(t, "UPDATE page_revisions SET deleted_at = $2 WHERE node_id = $1", p.ID, now())
	// c3 and c4 at one time: the higher id first.
	same := []uuid.UUID{c3.ID, c4.ID}
	slices.SortFunc(same, func(a, b uuid.UUID) int { return b.Compare(a) })

	list := func(fl domain.LogFilter, below domain.ChangesetCursor, limit int) []uuid.UUID {
		t.Helper()
		got, err := f.s.ListChangesets(ctx, f.eng, fl, below, limit)
		if err != nil {
			t.Fatal(err)
		}
		return changesetIDs(got)
	}
	all := append([]uuid.UUID{purged.ID, revert.ID, c5.ID}, append(same, c2.ID, c1.ID)...)
	if got := list(domain.LogFilter{}, domain.ChangesetFrom(), 100); !slices.Equal(got, all) {
		t.Errorf("the log = %v, want %v", got, all)
	}
	if got := list(domain.LogFilter{}, domain.ChangesetCursor{At: at(3), ID: same[0]}, 2); !slices.Equal(got, []uuid.UUID{same[1], c2.ID}) {
		t.Errorf("below the first of two at one time, 2 = %v, want the second and c2", got)
	}
	for _, tt := range []struct {
		name string
		f    domain.LogFilter
		want []uuid.UUID
	}{
		{"bob's", domain.LogFilter{Actor: &bob}, []uuid.UUID{c5.ID, c4.ID, c2.ID}},
		{"bob's from the web", domain.LogFilter{Actor: &bob, Client: domain.ClientWeb}, []uuid.UUID{c5.ID}},
		{"the web's", domain.LogFilter{Client: domain.ClientWeb}, []uuid.UUID{c5.ID, c1.ID}},
		{"an MCP client's", domain.LogFilter{Client: "mcp:a"}, []uuid.UUID{c2.ID}},
		{"the MCP clients'", domain.LogFilter{AllMCP: true}, []uuid.UUID{c4.ID, c2.ID}},
		{"bob's of the MCP clients", domain.LogFilter{Actor: &bob, AllMCP: true}, []uuid.UUID{c4.ID, c2.ID}},
		{"P's", domain.LogFilter{PageID: &p.ID}, []uuid.UUID{c4.ID, c1.ID}},
		{"P's by alice", domain.LogFilter{PageID: &p.ID, Actor: &f.alice}, []uuid.UUID{c1.ID}},
		{"P's from the web", domain.LogFilter{PageID: &p.ID, Client: domain.ClientWeb}, []uuid.UUID{c1.ID}},
		{"P's of the MCP clients", domain.LogFilter{PageID: &p.ID, AllMCP: true}, []uuid.UUID{c4.ID}},
	} {
		if got := list(tt.f, domain.ChangesetFrom(), 100); !slices.Equal(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
		}
	}
	if got := list(domain.LogFilter{PageID: &p.ID}, domain.ChangesetCursor{At: at(3), ID: c4.ID}, 100); !slices.Equal(got, []uuid.UUID{c1.ID}) {
		t.Errorf("P's below c4 = %v, want c1", got)
	}
	if got := list(domain.LogFilter{Actor: &bob}, domain.ChangesetCursor{At: at(3), ID: c4.ID}, 100); !slices.Equal(got, []uuid.UUID{c2.ID}) {
		t.Errorf("bob's below c4 = %v, want c2", got)
	}

	got, err := f.s.ListChangesets(ctx, f.eng, domain.LogFilter{}, domain.ChangesetFrom(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if r := got[1]; r.RevertsID != c3.ID || r.Reverts == nil || r.Reverts.ID != c3.ID || r.Reverts.Kind != domain.ChangesetEdit ||
		r.Reverts.By != f.alice || !r.Reverts.CreatedAt.Equal(at(3)) || r.Client != domain.ClientCLI {
		t.Errorf("the revert = %+v, reverting %+v; want c3's brief", r, r.Reverts)
	}
	if r := got[0]; r.RevertsID != purged.RevertsID || r.Reverts != nil {
		t.Errorf("the revert of one purged = %+v, want its id, no brief", r)
	}
	one, err := f.s.FindChangeset(ctx, revert.ID)
	if err != nil || one.ID != revert.ID || one.Reverts == nil || one.NotebookID != f.eng || !one.CreatedAt.Equal(at(7)) {
		t.Errorf("FindChangeset(the revert) = %+v, %v", one, err)
	}
	if _, err := f.s.FindChangeset(ctx, gone.ID); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindChangeset(of a deleted notebook) = %v, want ErrNotFound", err)
	}
	f.exec(t, "UPDATE changesets SET trimmed = true, message = 'why' WHERE id = $1", c1.ID)
	if one, err := f.s.FindChangeset(ctx, c1.ID); err != nil || !one.Trimmed || one.Message == nil || *one.Message != "why" ||
		!one.UpdatedAt.Equal(at(1)) {
		t.Errorf("FindChangeset(c1) = %+v, %v; want it trimmed, with its message", one, err)
	}
}

// nodes adds n pages of notebook at the root, named prefix and a number,
// through SQL, and answers their ids by id.
func (f fixture) nodes(t *testing.T, notebook uuid.UUID, prefix string, n int) []uuid.UUID {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
		created_by_id, updated_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, NULL, 'page', $2 || i, lower($2) || i, i, $3, $3, $4, $4 FROM generate_series(1, $5::integer) i
		RETURNING id`, notebook, prefix, f.alice, now(), n)
	if err != nil {
		t.Fatal(err)
	}
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// bulk gives cs an item creating each of items, and a version of each of
// versions at revision, through SQL.
func (f fixture) bulk(t *testing.T, cs uuid.UUID, items, versions []uuid.UUID, revision int) {
	t.Helper()
	f.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at)
		SELECT uuidv7(), $1, n.id, n.name, n.sort_order, $3, $3 FROM nodes n WHERE n.id = ANY($2)`, cs, items, now())
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
		SELECT uuidv7(), $1, n.id, $4, '', sha256(''), 0, $3, $3 FROM nodes n WHERE n.id = ANY($2)`, cs, versions, now(), revision)
}

// A changeset's items and versions are counted to 1,000; its reverts are
// listed newest first, 100 at most, a revert of a deleted notebook not.
func TestChangesetFactsCountTo1000(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	nodes := f.nodes(t, f.eng, "N", 1001)
	at1000, at1001 := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()), f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	f.bulk(t, at1000.ID, nodes[:1000], nodes[1:1001], 1)
	f.bulk(t, at1001.ID, nodes, nodes[:999], 2)
	var reverts []uuid.UUID
	for i := range 102 {
		r := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetRevert, RevertsID: at1001.ID, Client: domain.ClientWeb,
			By: f.alice, At: now().Add(time.Duration(i) * time.Second)}
		if i < 100 {
			r.RevertsID = at1000.ID
		}
		if err := f.s.CreateChangeset(ctx, r); err != nil {
			t.Fatal(err)
		}
		reverts = append(reverts, r.ID)
	}
	f.exec(t, "UPDATE changesets SET deleted_at = $2 WHERE id = $1", reverts[0], now())
	got, err := f.s.ChangesetFacts(ctx, []uuid.UUID{at1000.ID, at1001.ID, uuid.NewV7()})
	if err != nil {
		t.Fatal(err)
	}
	newest := func(ids []uuid.UUID) []uuid.UUID {
		out := slices.Clone(ids)
		slices.Reverse(out)
		return out
	}
	if g := got[at1000.ID]; g.Nodes != 1000 || g.Contents != 1000 || !slices.Equal(g.RevertedBy, newest(reverts[1:100])) {
		t.Errorf("at 1,000 = %d, %d, %d reverts; want 1000, 1000 and the 99 alive, newest first", g.Nodes, g.Contents, len(g.RevertedBy))
	}
	if g := got[at1001.ID]; g.Nodes != 1000 || g.Contents != 999 || !slices.Equal(g.RevertedBy, newest(reverts[100:])) {
		t.Errorf("at 1,001 = %d, %d, %v; want 1000, 999 and its two reverts", g.Nodes, g.Contents, g.RevertedBy)
	}
	f.exec(t, "UPDATE changesets SET reverts_id = $1 WHERE reverts_id = $2", at1001.ID, at1000.ID)
	got, err = f.s.ChangesetFacts(ctx, []uuid.UUID{at1001.ID})
	if g := got[at1001.ID]; err != nil || len(g.RevertedBy) != 100 || g.RevertedBy[0] != reverts[101] || g.RevertedBy[99] != reverts[2] {
		t.Errorf("101 reverts = %d, %v; want the newest 100", len(g.RevertedBy), err)
	}
}

// A changeset renames when an item's names differ, moves when its parents
// do, a reorder neither, a creation or a deletion neither; among its first
// 1,000 items by node id alone.
func TestChangesetMoves(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.page(t, f.eng, nil, "P", 0)
	n := f.page(t, f.eng, nil, "N", 1)
	from := n.State()
	for _, tt := range []struct {
		name string
		to   domain.TreeState
		want domain.ChangesetMoves
	}{
		{"a rename", domain.TreeState{Name: "M", SortOrder: 1}, domain.ChangesetMoves{Renames: true}},
		{"a move", domain.TreeState{ParentID: &p.ID, Name: "N", SortOrder: 1}, domain.ChangesetMoves{Moves: true}},
		{"a move and rename", domain.TreeState{ParentID: &p.ID, Name: "M", SortOrder: 1}, domain.ChangesetMoves{Renames: true, Moves: true}},
		{"a reorder", domain.TreeState{Name: "N", SortOrder: 5}, domain.ChangesetMoves{}},
	} {
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
		f.item(t, cs.ID, now(), domain.Change{NodeID: n.ID, Before: &from, After: &tt.to})
		f.item(t, cs.ID, now(), domain.Change{NodeID: p.ID, Before: new(p.State())})
		if got, err := f.s.ChangesetMoves(ctx, cs.ID); err != nil || got != tt.want {
			t.Errorf("%s = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}
	nodes := f.nodes(t, f.eng, "X", 1001)
	cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	f.bulk(t, cs.ID, nodes, nil, 1)
	f.exec(t, "UPDATE changeset_items SET before_name = 'Old', before_sort_order = 0 WHERE changeset_id = $1 AND node_id = $2", cs.ID, nodes[1000])
	if got, err := f.s.ChangesetMoves(ctx, cs.ID); err != nil || got != (domain.ChangesetMoves{}) {
		t.Errorf("a rename as the 1,001st item = %+v, %v; want none", got, err)
	}
	f.exec(t, "UPDATE changeset_items SET before_name = 'Old', before_sort_order = 0 WHERE changeset_id = $1 AND node_id = $2", cs.ID, nodes[999])
	if got, err := f.s.ChangesetMoves(ctx, cs.ID); err != nil || !got.Renames {
		t.Errorf("a rename as the 1,000th item = %+v, %v; want it", got, err)
	}
}

// A changeset's changes merge its items and versions by node id, a page
// after a node id at a time: each node as it is now, its item's states
// with their parents' names, alive or deleted, none once the purge took
// the parent; its version's revisions. FirstChanges are each changeset's
// first n.
func TestListChangesMergesItemsAndVersions(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	parent, deleted := f.page(t, f.eng, nil, "Parent", 0), f.page(t, f.eng, nil, "Deleted", 1)
	purged := uuid.NewV7()
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", deleted.ID, now())
	moved, written, both := f.page(t, f.eng, nil, "Moved", 2), f.page(t, f.eng, nil, "Written", 3), f.page(t, f.eng, nil, "Both", 4)
	cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	movedFrom, movedTo := domain.TreeState{ParentID: &purged, Name: "Was", SortOrder: 9}, domain.TreeState{ParentID: &parent.ID, Name: "Moved", SortOrder: 2}
	f.item(t, cs.ID, now(), domain.Change{NodeID: moved.ID, Before: &movedFrom, After: &movedTo})
	bothAt := domain.TreeState{ParentID: &deleted.ID, Name: "Both", SortOrder: 4}
	f.item(t, cs.ID, now(), domain.Change{NodeID: both.ID, After: &bothAt, Revision: 1, Restored: &domain.Restored{DeletedAt: now()}})
	f.version(t, cs.ID, both.ID, nil, 1, "")
	f.version(t, cs.ID, written.ID, new(1), 2, "two")
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", written.ID, now())
	want := map[uuid.UUID]string{
		moved.ID:   "Moved alive moved before " + purged.String() + " <purged> Was after " + parent.ID.String() + " Parent Moved",
		both.ID:    "Both alive restored after " + deleted.ID.String() + " Deleted Both content <nil>→1",
		written.ID: "Written deleted content content 1→2",
	}
	ids := []uuid.UUID{moved.ID, both.ID, written.ID}
	slices.SortFunc(ids, uuid.UUID.Compare)
	describe := func(r domain.ChangeRow) string {
		s := r.Name + map[bool]string{true: " alive", false: " deleted"}[r.Alive] + " " + string(r.Change())
		place := func(label string, p *domain.ChangePlace) {
			if p != nil {
				name := "<purged>"
				if p.ParentName != nil {
					name = *p.ParentName
				}
				s += fmt.Sprintf(" %s %s %s %s", label, p.ParentID, name, p.Name)
			}
		}
		place("before", r.Before)
		place("after", r.After)
		if r.Content != nil {
			base := "<nil>"
			if r.Content.Base != nil {
				base = fmt.Sprint(*r.Content.Base)
			}
			s += fmt.Sprintf(" content %s→%d", base, r.Content.Revision)
		}
		return s
	}
	var got []domain.ChangeRow
	after := uuid.UUID{}
	for range 4 {
		page, err := f.s.ListChanges(ctx, cs.ID, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, page...)
		if len(page) < 2 {
			break
		}
		after = page[len(page)-1].NodeID
	}
	if len(got) != 3 {
		t.Fatalf("changes = %d, want 3", len(got))
	}
	for i, r := range got {
		if r.NodeID != ids[i] || describe(r) != want[r.NodeID] {
			t.Errorf("change %d = %s %q, want %s %q", i, r.NodeID, describe(r), ids[i], want[ids[i]])
		}
	}
	other := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	f.item(t, other.ID, now(), domain.Change{NodeID: parent.ID, Before: new(parent.State()), After: &domain.TreeState{Name: "P2"}})
	first, err := f.s.FirstChanges(ctx, []uuid.UUID{cs.ID, other.ID, uuid.NewV7()}, 2)
	if err != nil || len(first) != 2 || len(first[cs.ID]) != 2 || first[cs.ID][0].NodeID != ids[0] || first[cs.ID][1].NodeID != ids[1] ||
		len(first[other.ID]) != 1 || first[other.ID][0].Change() != domain.ChangeRenamed {
		t.Errorf("FirstChanges(2) = %+v, %v; want cs's first two, other's rename", first, err)
	}
}

// A revert's reads: the items counted to a limit; the items and versions by
// node, each version with its base's size, none once the purge took the
// base; the nodes now, a root with its counts and whether the revert of a
// restore deleted it, a node the purge took not among them; the contents
// of the pages alive; the versions' hashes and contents, a version the
// purge took not among them.
func TestTheRevertsReads(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p, q, r := f.page(t, f.eng, nil, "P", 0), f.page(t, f.eng, nil, "Q", 1), f.page(t, f.eng, nil, "R", 2)
	x := f.asset(t, f.eng, &p.ID, "x.png", 0)
	first := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	one := f.version(t, first.ID, p.ID, nil, 1, "one")
	f.version(t, first.ID, q.ID, nil, 1, "q")
	cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	two := f.version(t, cs.ID, p.ID, new(1), 2, "twotwo")
	f.version(t, cs.ID, q.ID, new(1), 2, "")
	f.exec(t, "DELETE FROM page_revisions WHERE changeset_id = $1 AND node_id = $2", first.ID, q.ID)
	rState, renamed := r.State(), domain.TreeState{Name: "R2", SortOrder: 2}
	f.item(t, cs.ID, now(), domain.Change{NodeID: r.ID, Before: &rState, After: &renamed})
	f.trashed(t, now(), 1, 1, 1, p.ID, x.ID)
	pState, xState := p.State(), x.State()
	f.item(t, cs.ID, now(), domain.Change{NodeID: p.ID, Before: &pState})
	f.item(t, cs.ID, now(), domain.Change{NodeID: x.ID, Before: &xState})

	if n, err := f.s.CountItems(ctx, cs.ID, 2); err != nil || n != 2 {
		t.Errorf("CountItems(2) = %d, %v; want 2", n, err)
	}
	if n, err := f.s.CountItems(ctx, cs.ID, 10); err != nil || n != 3 {
		t.Errorf("CountItems(10) = %d, %v; want 3", n, err)
	}
	items, err := f.s.ChangesetItems(ctx, cs.ID)
	byNode := map[uuid.UUID]domain.ChangesetItem{}
	for i, it := range items {
		byNode[it.NodeID] = it
		if i > 0 && items[i-1].NodeID.Compare(it.NodeID) >= 0 {
			t.Errorf("items not by node id: %v", items)
		}
	}
	if err != nil || len(items) != 3 || byNode[p.ID].Before == nil || byNode[p.ID].After != nil || byNode[p.ID].Revision != 1 ||
		byNode[x.ID].Revision != 0 || byNode[r.ID].After == nil || byNode[r.ID].After.Name != "R2" || !byNode[r.ID].Before.Same(rState) {
		t.Errorf("ChangesetItems = %+v, %v", items, err)
	}
	versions, err := f.s.ChangesetVersions(ctx, cs.ID)
	byPage := map[uuid.UUID]domain.ChangesetVersion{}
	for _, v := range versions {
		byPage[v.NodeID] = v
	}
	if err != nil || len(versions) != 2 || *byPage[p.ID].Base != 1 || byPage[p.ID].BaseBytes != 3 || byPage[p.ID].Revision != 2 ||
		!bytes.Equal(byPage[p.ID].Hash, two) || byPage[q.ID].BaseBytes != 0 {
		t.Errorf("ChangesetVersions = %+v, %v; want P's on 1 of 3 bytes, Q's base purged", versions, err)
	}
	gone := uuid.NewV7()
	nodes, err := f.s.NodesNow(ctx, []uuid.UUID{p.ID, x.ID, r.ID, gone})
	if err != nil || len(nodes) != 3 || nodes[p.ID].DeletedAt == nil || !nodes[p.ID].TrashRoot || nodes[p.ID].TrashPages != 1 ||
		nodes[p.ID].TrashAssets != 1 || nodes[x.ID].TrashRoot || nodes[x.ID].DeletedAt == nil || nodes[r.ID].DeletedAt != nil ||
		nodes[r.ID].Name != "R" || nodes[x.ID].Kind != domain.KindAsset {
		t.Errorf("NodesNow = %+v, %v", nodes, err)
	}
	if nodes[p.ID].DeletedByRevert {
		t.Errorf("NodesNow(P) = %+v, deleted by an edit's changeset; want no revert's deletion", nodes[p.ID])
	}
	// The deletion by the revert of a restore: its changeset, created at
	// the root's time, a revert of a trash item's restore, holds the root's
	// deletion. One holding another item of the root, created at another
	// time, or a revert of an edit did not delete it (fix check 1); nor
	// did one before the root's current deletion, an edit's, the root
	// restored between them (fix check 1, nit 2).
	restore := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetTrashRestore, Client: domain.ClientWeb,
		By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, restore); err != nil {
		t.Fatal(err)
	}
	byRevert := func(name string, order float64, at, made time.Time, reverts uuid.UUID, deletion bool) domain.Node {
		t.Helper()
		n := f.page(t, f.eng, nil, name, order)
		f.trashed(t, at, 1, 0, 1, n.ID)
		rev := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetRevert, RevertsID: reverts, Client: domain.ClientWeb,
			By: f.alice, At: made}
		if err := f.s.CreateChangeset(ctx, rev); err != nil {
			t.Fatal(err)
		}
		before, after := n.State(), n.State()
		change := domain.Change{NodeID: n.ID, Before: &before}
		if !deletion {
			change.After = &after
		}
		f.item(t, rev.ID, at, change)
		return n
	}
	s1, s2, s3, s4, s5 := now().Add(time.Minute), now().Add(2*time.Minute), now().Add(3*time.Minute), now().Add(4*time.Minute),
		now().Add(5*time.Minute)
	deleted, other := byRevert("S", 3, s1, s1, restore.ID, true).ID, byRevert("U", 4, s2, s2, restore.ID, false).ID
	later, ofAnEdit := byRevert("V", 5, s3, s3.Add(time.Hour), restore.ID, true).ID, byRevert("W", 6, s4, s4, cs.ID, true).ID
	y := byRevert("Y", 7, s5, s5, restore.ID, true)
	if _, err := f.s.RestoreNodes(ctx, restoration(s5, nil, 7, f.alice, y)); err != nil {
		t.Fatal(err)
	}
	s6 := s5.Add(2 * time.Hour)
	f.trashed(t, s6, 1, 0, 1, y.ID)
	f.item(t, f.changeset(t, f.eng, f.alice, domain.ClientWeb, s6).ID, s6, domain.Change{NodeID: y.ID, Before: new(y.State())})
	nodes, err = f.s.NodesNow(ctx, []uuid.UUID{deleted, other, later, ofAnEdit, y.ID, r.ID})
	if err != nil || !nodes[deleted].DeletedByRevert || nodes[other].DeletedByRevert || nodes[later].DeletedByRevert ||
		nodes[ofAnEdit].DeletedByRevert || nodes[y.ID].DeletedByRevert || !nodes[y.ID].DeletedAt.Equal(s6) ||
		nodes[r.ID].DeletedByRevert {
		t.Errorf("NodesNow = %+v, %v; want S deleted by a restore's revert alone, Y at its later deletion", nodes, err)
	}
	contents, err := f.s.ContentsNow(ctx, []uuid.UUID{p.ID, q.ID, r.ID})
	if err != nil || len(contents) != 2 || contents[r.ID].Revision != 1 || len(contents[q.ID].Hash) != 32 {
		t.Errorf("ContentsNow = %+v, %v; want Q's and R's, P being in the trash", contents, err)
	}
	refs := []domain.VersionRef{{NodeID: p.ID, Revision: 1}, {NodeID: p.ID, Revision: 2}, {NodeID: q.ID, Revision: 1}}
	hashes, err := f.s.VersionHashes(ctx, refs)
	if err != nil || len(hashes) != 2 || !bytes.Equal(hashes[refs[0]], one) || !bytes.Equal(hashes[refs[1]], two) {
		t.Errorf("VersionHashes = %v, %v; want P's two, Q's 1 purged", hashes, err)
	}
	texts, err := f.s.VersionContents(ctx, refs)
	if err != nil || len(texts) != 2 || texts[refs[0]].Content != "one" || !bytes.Equal(texts[refs[1]].Hash, two) ||
		texts[refs[1]].Content != "twotwo" || texts[refs[1]].Revision != 2 {
		t.Errorf("VersionContents = %+v, %v", texts, err)
	}
}
