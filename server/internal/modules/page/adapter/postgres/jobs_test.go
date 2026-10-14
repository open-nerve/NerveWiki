package postgresadapter_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// jobsTrash is a notebook's trash of three items, as the background jobs
// read it: R (deleted at at) holds A and B, A holds photo.png, B holds C;
// S (at at plus a minute) is a lone page; T (at at) under another root
// holds U. Earlier, under B, is an item of its own of an earlier time.
type jobsTrash struct {
	at                                      time.Time
	r, a, b, c, photo, s, tRoot, u, earlier domain.Node
}

func newJobsTrash(t *testing.T, f fixture) jobsTrash {
	t.Helper()
	j := jobsTrash{at: now().Add(time.Minute)}
	j.r = f.page(t, f.eng, nil, "R", 0)
	j.b, j.a = f.page(t, f.eng, &j.r.ID, "B", 1), f.page(t, f.eng, &j.r.ID, "A", 0)
	j.photo = f.asset(t, f.eng, &j.a.ID, "photo.png", 0)
	j.c = f.page(t, f.eng, &j.b.ID, "C", 2)
	j.earlier = f.page(t, f.eng, &j.b.ID, "Earlier", 0)
	j.s = f.page(t, f.eng, nil, "S", 1)
	j.tRoot = f.page(t, f.eng, nil, "T", 2)
	j.u = f.page(t, f.eng, &j.tRoot.ID, "U", 0)
	f.exec(t, "UPDATE page_contents SET content = 'abc', byte_size = 3, content_hash = sha256('abc'), revision = 4 WHERE node_id = $1",
		j.a.ID)
	f.trashed(t, now(), 1, 0, 1, j.earlier.ID)
	f.trashed(t, j.at, 4, 1, 3, j.r.ID, j.b.ID, j.a.ID, j.photo.ID, j.c.ID)
	f.trashed(t, j.at.Add(time.Minute), 1, 0, 1, j.s.ID)
	f.trashed(t, j.at, 2, 0, 2, j.tRoot.ID, j.u.ID)
	return j
}

// A batch's members and many roots are read each at its own time: a root
// with its counts, a page with its content's revision and bytes; a node
// deleted at another time, of another notebook or alive is not.
func TestTrashNodesAt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	j := newJobsTrash(t, f)
	alive := f.page(t, f.eng, nil, "Alive", 9)
	at := map[uuid.UUID]time.Time{j.r.ID: j.at, j.a.ID: j.at, j.photo.ID: j.at, j.s.ID: j.at.Add(time.Minute),
		j.earlier.ID: j.at, alive.ID: j.at, uuid.NewV7(): j.at}
	nodes, err := f.s.TrashNodesAt(ctx, f.eng, at)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range nodes {
		got[n.Node.Name] = fmt.Sprintf("root %t %d/%d/%d r%d %db", n.Root, n.Pages, n.Assets, n.Height, n.Revision, n.Bytes)
	}
	want := map[string]string{"R": "root true 4/1/3 r1 0b", "A": "root false 0/0/0 r4 3b", "photo.png": "root false 0/0/0 r0 0b",
		"S": "root true 1/0/1 r1 0b"}
	if len(got) != len(want) {
		t.Fatalf("TrashNodesAt() = %v, want %v", got, want)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("TrashNodesAt(%s) = %s, want %s", name, got[name], w)
		}
	}
	if n := nodes[j.a.ID]; n.Node.ParentID == nil || *n.Node.ParentID != j.r.ID || n.Node.NotebookID != f.eng {
		t.Errorf("TrashNodesAt(A) = %+v, want A under R as it reads", n.Node)
	}
	if nodes, err := f.s.TrashNodesAt(ctx, f.ops, at); err != nil || len(nodes) != 0 {
		t.Errorf("TrashNodesAt(of another notebook) = %v, %v; want none", nodes, err)
	}
}

// Many items' groups read a level at a time are each the one TrashGroup
// reads: its root first, a level after another, each in its order; one
// not deleted at its time is not among them.
func TestTrashGroupsAreTheGroupsTrashGroupReads(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	j := newJobsTrash(t, f)
	roots := map[uuid.UUID]time.Time{j.r.ID: j.at, j.s.ID: j.at.Add(time.Minute), j.tRoot.ID: j.at, j.earlier.ID: j.at}
	groups, err := f.s.TrashGroups(ctx, f.eng, roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("TrashGroups() = %d groups, want R's, S's and T's, not Earlier's of another time", len(groups))
	}
	for root, at := range roots {
		if root == j.earlier.ID {
			continue
		}
		one, err := f.s.TrashGroup(ctx, f.eng, root, at)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(groups[root]) != fmt.Sprint(one) {
			t.Errorf("TrashGroups()[%s] = %+v\nwant TrashGroup's %+v", root, groups[root], one)
		}
	}
	var names []string
	for _, m := range groups[j.r.ID] {
		names = append(names, m.Node.Name)
	}
	// Each level in order across its parents: photo.png's 0 before C's 2.
	if want := []string{"R", "A", "B", "photo.png", "C"}; !slices.Equal(names, want) {
		t.Errorf("R's group %v, want %v", names, want)
	}
}

// A group's shape below its root: each level by parent, a parent's
// children in order, with their kinds and pages' bytes; nothing of
// another time.
func TestTrashShape(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	j := newJobsTrash(t, f)
	shape, err := f.s.TrashShape(ctx, f.eng, j.r.ID, j.at)
	if err != nil {
		t.Fatal(err)
	}
	names := map[uuid.UUID]string{j.a.ID: "A", j.b.ID: "B", j.c.ID: "C", j.photo.ID: "photo.png", j.r.ID: "R"}
	var got []string
	for _, n := range shape {
		got = append(got, fmt.Sprintf("%s<%s %t %db", names[n.ID], names[n.ParentID], n.Asset, n.Bytes))
	}
	// A and B are the root's, by order; then the next level, by parent.
	want := []string{"A<R false 3b", "B<R false 0b"}
	next := []string{"photo.png<A true 0b", "C<B false 0b"}
	if j.a.ID.Compare(j.b.ID) > 0 {
		slices.Reverse(next)
	}
	if want = append(want, next...); !slices.Equal(got, want) {
		t.Errorf("TrashShape(R) = %v, want %v", got, want)
	}
	if shape, err := f.s.TrashShape(ctx, f.eng, j.r.ID, now()); err != nil || len(shape) != 0 {
		t.Errorf("TrashShape(R at another time) = %v, %v; want none", shape, err)
	}
}

// The nodes a background restore's batch leaves in the trash become
// items with their counts: only those still deleted at the time and no
// root already, which it counts.
func TestMarkTrashRoots(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	j := newJobsTrash(t, f)
	n, err := f.s.MarkTrashRoots(ctx, []app.TrashMark{{ID: j.a.ID, Pages: 1, Assets: 1, Height: 1}, {ID: j.b.ID, Pages: 2, Height: 2},
		{ID: j.r.ID, Pages: 9}, {ID: j.earlier.ID, Pages: 9}, {ID: j.s.ID, Pages: 9}}, j.at)
	if err != nil || n != 2 {
		t.Fatalf("MarkTrashRoots() = %d, %v; want A and B marked alone", n, err)
	}
	for _, c := range []struct {
		id   uuid.UUID
		want string
	}{{j.a.ID, "1/1/1"}, {j.b.ID, "2/0/2"}, {j.r.ID, "4/1/3"}, {j.earlier.ID, "1/0/1"}, {j.s.ID, "1/0/1"}} {
		root, err := f.s.FindTrashRoot(ctx, c.id)
		if got := fmt.Sprintf("%d/%d/%d", root.Pages, root.Assets, root.Height); err != nil || got != c.want {
			t.Errorf("FindTrashRoot(%s) = %s, %v; want %s", c.id, got, err, c.want)
		}
	}
	if root, err := f.s.TrashRootOf(ctx, j.photo.ID); err != nil || root != j.a.ID {
		t.Errorf("TrashRootOf(photo.png) = %s, %v; want A, an item of its own", root, err)
	}
}

// A batch's items and versions are those of its nodes alone, by node.
func TestChangesetItemsAndVersionsOfNodes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	p, q, r := f.page(t, f.eng, nil, "P", 0), f.page(t, f.eng, nil, "Q", 1), f.page(t, f.eng, nil, "R", 2)
	for _, n := range []domain.Node{p, q, r} {
		f.item(t, cs.ID, now(), domain.Change{NodeID: n.ID, After: new(n.State())})
		f.version(t, cs.ID, n.ID, nil, 1, n.Name)
	}
	ids := []uuid.UUID{r.ID, p.ID, uuid.NewV7()}
	items, err := f.s.ChangesetItemsOf(ctx, cs.ID, ids)
	want := []uuid.UUID{p.ID, r.ID}
	slices.SortFunc(want, uuid.UUID.Compare)
	if err != nil || len(items) != 2 || items[0].NodeID != want[0] || items[1].NodeID != want[1] || items[0].After == nil {
		t.Errorf("ChangesetItemsOf(R, P) = %+v, %v; want P's and R's by node", items, err)
	}
	versions, err := f.s.ChangesetVersionsOf(ctx, cs.ID, ids)
	if err != nil || len(versions) != 2 || versions[0].NodeID != want[0] || versions[1].NodeID != want[1] || versions[0].Base != nil {
		t.Errorf("ChangesetVersionsOf(R, P) = %+v, %v; want P's and R's creations by node", versions, err)
	}
	if items, err := f.s.ChangesetItemsOf(ctx, uuid.NewV7(), ids); err != nil || len(items) != 0 {
		t.Errorf("ChangesetItemsOf(another changeset) = %+v, %v; want none", items, err)
	}
}

// The jobs' statements of many keys read by them (M8/P4 design 3.3; v0.1
// design 13.4, item 9), scans of whole tables off: nodes by their key, a
// level's children through the siblings' index, a changeset's items and
// versions of nodes by their (changeset_id, node_id) keys; and each is
// planned with its arguments however many times a connection runs it.
func TestTheJobsReadsUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.logOfSomeSize(t)
	// A large group, analyzed: a root, 100 folders under it and 99 pages
	// under each, deleted at one time; the statements read 100 of its
	// nodes, or the children of ten of them, a level's part.
	at := now().Add(time.Hour)
	root := f.page(t, f.eng, nil, "Root", 0)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT uuidv7(), $1, $2, 'page', 'F' || i, 'f' || i, i, $3, $3, $4, $4 FROM generate_series(1, 100) i`,
		f.eng, root.ID, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT uuidv7(), $1, n.id, 'page', 'P' || i, 'p' || i, i, $3, $3, $4, $4
		FROM nodes n, generate_series(1, 99) i WHERE n.parent_id = $2`, f.eng, root.ID, f.alice, now())
	f.exec(t, `UPDATE nodes SET deleted_at = $2, trash_root = (id = $1),
		trash_pages = CASE WHEN id = $1 THEN 10001 END, trash_assets = CASE WHEN id = $1 THEN 0 END,
		trash_height = CASE WHEN id = $1 THEN 3 END
		WHERE id = $1 OR parent_id = $1 OR parent_id IN (SELECT id FROM nodes WHERE parent_id = $1)`, root.ID, at)
	f.exec(t, "ANALYZE")
	rows, err := f.pool.Query(ctx, `SELECT id FROM nodes WHERE parent_id = $1 ORDER BY id`, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(ids) != 100 {
		t.Fatalf("the folders: %d, %v", len(ids), err)
	}
	times := make([]time.Time, len(ids))
	for i := range times {
		times[i] = at
	}
	for _, tt := range []struct {
		name    string
		s       statement
		indexes []string
	}{
		{"TrashNodesAt", statementOf(t, func(q *gen.Queries) error {
			_, err := q.TrashNodesAt(ctx, gen.TrashNodesAtParams{Ids: ids, DeletedAts: times, NotebookID: f.eng})
			return err
		}), []string{"nodes_pkey"}},
		{"TrashChildrenAt", statementOf(t, func(q *gen.Queries) error {
			_, err := q.TrashChildrenAt(ctx, gen.TrashChildrenAtParams{Parents: ids[:10], DeletedAts: times[:10], NotebookID: f.eng})
			return err
		}), []string{"nodes_notebook_id_parent_id_idx"}},
		{"TrashShapeOfAll", statementOf(t, func(q *gen.Queries) error {
			_, err := q.TrashShapeOfAll(ctx, gen.TrashShapeOfAllParams{NotebookID: f.eng, Parents: ids[:10], DeletedAt: at})
			return err
		}), []string{"nodes_notebook_id_parent_id_idx"}},
		{"MarkTrashRoots", statementOf(t, func(q *gen.Queries) error {
			_, err := q.MarkTrashRoots(ctx, gen.MarkTrashRootsParams{Ids: ids, Pages: make([]int32, len(ids)),
				Assets: make([]int32, len(ids)), Heights: make([]int32, len(ids)), DeletedAt: at})
			return err
		}), []string{"nodes_pkey"}},
		{"ChangesetItemsOf", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetItemsOf(ctx, gen.ChangesetItemsOfParams{ID: id, NodeIds: ids})
			return err
		}), []string{"changeset_items_changeset_id_node_id_key"}},
		{"ChangesetVersionsOf", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetVersionsOf(ctx, gen.ChangesetVersionsOfParams{ID: id, NodeIds: ids})
			return err
		}), []string{"page_revisions_changeset_id_node_id_key", "page_revisions_node_id_revision_key"}},
	} {
		plan := f.planOf(t, tt.s)
		text := strings.Join(plan, "\n")
		for _, index := range tt.indexes {
			if !slices.ContainsFunc(plan, func(line string) bool { return slices.Contains(strings.Fields(line), index) }) {
				t.Errorf("%s does not read %s:\n%s", tt.name, index, text)
			}
		}
		if strings.Contains(text, "Seq Scan") {
			t.Errorf("%s scans a table whole:\n%s", tt.name, text)
		}
	}

	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	one := map[uuid.UUID]time.Time{ids[0]: at}
	for range 8 {
		if _, err := s.TrashNodesAt(ctx, f.eng, one); err != nil {
			t.Fatal(err)
		}
		if _, err := s.TrashGroups(ctx, f.eng, one); err != nil {
			t.Fatal(err)
		}
		if _, err := s.TrashShape(ctx, f.eng, ids[0], at); err != nil {
			t.Fatal(err)
		}
		if _, err := s.MarkTrashRoots(ctx, []app.TrashMark{{ID: root.ID}}, at); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ChangesetItemsOf(ctx, id, ids); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ChangesetVersionsOf(ctx, id, ids); err != nil {
			t.Fatal(err)
		}
	}
	cached := pgtest.CachedStatements(t, pool)
	for _, name := range []string{"TrashNodesAt", "TrashChildrenAt", "TrashShapeOfAll", "MarkTrashRoots", "ChangesetItemsOf",
		"ChangesetVersionsOf"} {
		if slices.Contains(cached, name) {
			t.Errorf("%s is a cached statement: %v", name, cached)
		}
	}
}
