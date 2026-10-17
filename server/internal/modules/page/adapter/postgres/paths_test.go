package postgresadapter_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A path's segment is a node not deleted by its title's key: a root at the
// root alone, a child under its parent alone, each in its notebook (M9/P1
// design 3.9.2).
func TestThePathsLookUpByKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	guide := f.page(t, f.eng, nil, "Guide", 0)
	setup := f.page(t, f.eng, &guide.ID, "Setup", 0)
	f.page(t, f.eng, &guide.ID, "Straße", 1)
	f.page(t, f.ops, nil, "Elsewhere", 0)
	gone := f.page(t, f.eng, nil, "Gone", 1)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		parent   *uuid.UUID
		key      string
		want     string
	}{
		{"a root", f.eng, nil, "guide", "Guide"},
		{"a root by its key, not its title", f.eng, nil, "Guide", ""},
		{"a child is no root", f.eng, nil, "setup", ""},
		{"a child", f.eng, &guide.ID, "setup", "Setup"},
		{"a child by its folded key", f.eng, &guide.ID, shared.TitleKey("STRASSE"), "Straße"},
		{"a root is no child", f.eng, &guide.ID, "guide", ""},
		{"a child of another parent", f.eng, &setup.ID, "setup", ""},
		{"another notebook's root", f.eng, nil, "elsewhere", ""},
		{"a root in another notebook", f.ops, nil, "guide", ""},
		{"a child in another notebook", f.ops, &guide.ID, "setup", ""},
		{"a deleted root", f.eng, nil, "gone", ""},
	} {
		var name string
		var ok bool
		var err error
		if tt.parent == nil {
			n, found, ferr := f.s.RootByKey(ctx, tt.notebook, tt.key)
			name, ok, err = n.Name, found, ferr
		} else {
			n, found, ferr := f.s.ChildByKey(ctx, tt.notebook, *tt.parent, tt.key)
			name, ok, err = n.Name, found, ferr
		}
		if err != nil || ok != (tt.want != "") || name != tt.want {
			t.Errorf("%s: %q, %v, %v; want %q", tt.name, name, ok, err, tt.want)
		}
	}
}

// PageStates reads each page's revision, size and last write: not of a
// deleted content, nor of an attachment, which has none.
func TestPageStates(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1)
	gone := f.page(t, f.eng, nil, "Gone", 2)
	photo := f.asset(t, f.eng, nil, "photo.png", 3)
	later := now().Add(time.Hour)
	f.exec(t, "UPDATE page_contents SET revision = 7, content = 'twelve bytes', byte_size = 12, updated_at = $2 WHERE node_id = $1", b.ID, later)
	f.exec(t, "UPDATE page_contents SET deleted_at = $2 WHERE node_id = $1", gone.ID, now())
	got, err := f.s.PageStates(ctx, []uuid.UUID{a.ID, b.ID, gone.ID, photo.ID, uuid.NewV7()})
	switch {
	case err != nil:
		t.Fatal(err)
	case len(got) != 2:
		t.Errorf("PageStates = %+v, want A's and B's", got)
	case got[a.ID] != (app.PageState{Revision: 1, Bytes: 0, UpdatedAt: now()}):
		t.Errorf("A's state is %+v", got[a.ID])
	case got[b.ID].Revision != 7 || got[b.ID].Bytes != 12 || !got[b.ID].UpdatedAt.Equal(later):
		t.Errorf("B's state is %+v", got[b.ID])
	}
}

// lookupPlan is a path lookup's statement and the index it reads by: each
// column given an equality in the index's condition, the others checked on
// the row; analyzed, by it once the table is analyzed, whatever it reads
// by before.
type lookupPlan struct {
	name     string
	s        statement
	index    string
	columns  []string
	filter   []string
	analyzed bool
}

// rootPlan and childPlan are RootByKey's and ChildByKey's lookups.
func rootPlan(t *testing.T, notebook uuid.UUID, key string) lookupPlan {
	return lookupPlan{"RootByKey(" + key + ")", statementOf(t, func(q *gen.Queries) error {
		_, err := q.RootByKey(context.Background(), gen.RootByKeyParams{NotebookID: notebook, NameKey: key})
		return err
	}), "nodes_notebook_id_name_key_root_key", []string{"notebook_id", "name_key"}, nil, false}
}

func childPlan(t *testing.T, notebook, parent uuid.UUID, key string) lookupPlan {
	return lookupPlan{"ChildByKey(" + key + ")", statementOf(t, func(q *gen.Queries) error {
		_, err := q.ChildByKey(context.Background(), gen.ChildByKeyParams{NotebookID: notebook, ParentID: &parent, NameKey: key})
		return err
	}), "nodes_parent_id_name_key_key", []string{"parent_id", "name_key"}, []string{"notebook_id"}, true}
}

// check fails t unless plan reads nodes once, by l's index, every column
// of the index an equality of its condition and l's others of the row's.
func (l lookupPlan) check(t *testing.T, stage string, plan planNode, raw string) {
	t.Helper()
	scans := plan.all(func(n *planNode) bool { return n.Relation == "nodes" })
	if len(scans) != 1 || !reads("nodes", l.index, "")(scans[0]) {
		t.Errorf("%s, %s, reads nodes by other than %s:\n%s", l.name, stage, l.index, raw)
		return
	}
	cond := scans[0].IndexCond
	for _, bitmap := range scans[0].all(func(n *planNode) bool { return n.Index == l.index }) {
		cond += " " + bitmap.IndexCond
	}
	for _, column := range l.columns {
		if !strings.Contains(cond, "(nodes."+column+" = ") {
			t.Errorf("%s, %s: the index's condition %q has no equality of %s", l.name, stage, cond, column)
		}
	}
	for _, column := range l.filter {
		if !strings.Contains(scans[0].Filter, "(nodes."+column+" = ") {
			t.Errorf("%s, %s: the filter %q has no equality of %s", l.name, stage, scans[0].Filter, column)
		}
	}
}

// The path's lookups are point lookups (M9/P1 design 3.9.2): a root read by
// the roots' unique index (notebook_id, name_key), without statistics and
// analyzed; a child by the children's (parent_id, name_key) once the
// table is analyzed (without statistics, the titles' index may be cheaper:
// 00042's header). Both are 00042's, each column of the index an equality
// of its condition, with the production settings. On the shapes of data
// the notebook's other indexes were taken on (M9/P1 S2 reviews, A-I1 and
// C-I2; fix check A, A2-M1), each plan for its arguments and generic, as
// pgx's cached statements get either:
//   - folders: 1,000 folders, each with C1 to C100 and a README, a root
//     README, and 50,000 roots in another notebook: keys shared by many
//     parents' children (the titles' index read every node of the key);
//   - folders alone: the 1,000 folders, each with C1 to C100, and nothing
//     else, where the titles' index takes a child without statistics;
//   - one key: 20,000 roots, each with a child "notes", a root "notes", and
//     a root README in each of 2,002 notebooks: a key shared by most nodes,
//     and by roots across notebooks (analyzed, a root was read by a scan of
//     the table);
//   - a wide folder: one of 20,000 children among 20,001 roots, each root
//     with a child of a key the folder has too (the parents' index read
//     every child of the parent, and every root; and a folder's share of
//     the rows times a key's is many rows, but for a unique index).
//
// IS NOT DISTINCT FROM, either index gone, or one not unique, fail here,
// analyzed.
func TestThePathLookupsArePointLookups(t *testing.T) {
	const insert = `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at) `
	for _, tt := range []struct {
		name    string
		seed    func(f fixture)
		lookups func(f fixture) []lookupPlan
	}{
		{"folders", func(f fixture) {
			f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'F' || i, 'f' || i, i, $2, $2, $3, $3 FROM generate_series(1, 1000) i`,
				f.eng, f.alice, now())
			f.exec(t, insert+`VALUES (uuidv7(), $1, NULL, 'page', 'README', 'readme', 0, $2, $2, $3, $3)`, f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'C' || i, 'c' || i, i, $2, $2, $3, $3
				FROM nodes p, generate_series(1, 100) i WHERE p.notebook_id = $1 AND p.name LIKE 'F%'`, f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'README', 'readme', 0, $2, $2, $3, $3
				FROM nodes p WHERE p.notebook_id = $1 AND p.parent_id IS NULL AND p.name LIKE 'F%'`, f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'O' || i, 'o' || i, i, $2, $2, $3, $3 FROM generate_series(1, 50000) i`,
				f.ops, f.alice, now())
		}, func(f fixture) []lookupPlan {
			f7 := f.idOf(t, f.eng, "F7")
			return []lookupPlan{rootPlan(t, f.eng, "readme"), rootPlan(t, f.eng, "c7"), childPlan(t, f.eng, f7, "readme"),
				childPlan(t, f.eng, f7, "c7")}
		}},
		{"folders alone", func(f fixture) {
			f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'F' || i, 'f' || i, i, $2, $2, $3, $3 FROM generate_series(1, 1000) i`,
				f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'C' || i, 'c' || i, i, $2, $2, $3, $3
				FROM nodes p, generate_series(1, 100) i WHERE p.notebook_id = $1 AND p.parent_id IS NULL`, f.eng, f.alice, now())
		}, func(f fixture) []lookupPlan {
			return []lookupPlan{rootPlan(t, f.eng, "f7"), childPlan(t, f.eng, f.idOf(t, f.eng, "F7"), "c7")}
		}},
		{"one key", func(f fixture) {
			f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'R' || i, 'r' || i, i, $2, $2, $3, $3 FROM generate_series(1, 20000) i`,
				f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'notes', 'notes', 0, $2, $2, $3, $3 FROM nodes p WHERE p.notebook_id = $1`,
				f.eng, f.alice, now())
			f.exec(t, insert+`VALUES (uuidv7(), $1, NULL, 'page', 'notes', 'notes', 0, $2, $2, $3, $3)`, f.eng, f.alice, now())
			f.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
				SELECT uuidv7(), workspace_id, 'N' || i, $2, $2, $3, $3 FROM notebooks, generate_series(1, 2000) i WHERE id = $1`,
				f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), id, NULL, 'page', 'README', 'readme', 0, $1, $1, $2, $2 FROM notebooks`, f.alice, now())
		}, func(f fixture) []lookupPlan {
			return []lookupPlan{rootPlan(t, f.eng, "notes"), rootPlan(t, f.eng, "readme"), childPlan(t, f.eng, f.idOf(t, f.eng, "R7"), "notes")}
		}},
		{"a wide folder", func(f fixture) {
			f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'R' || i, 'r' || i, i, $2, $2, $3, $3 FROM generate_series(1, 20000) i`,
				f.eng, f.alice, now())
			f.exec(t, insert+`VALUES (uuidv7(), $1, NULL, 'page', 'W', 'w', 0, $2, $2, $3, $3)`, f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'C' || i, 'c' || i, i, $2, $2, $3, $3
				FROM nodes p, generate_series(1, 20000) i WHERE p.notebook_id = $1 AND p.name = 'W'`, f.eng, f.alice, now())
			f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'C7', 'c7', 0, $2, $2, $3, $3
				FROM nodes p WHERE p.notebook_id = $1 AND p.name LIKE 'R%'`, f.eng, f.alice, now())
		}, func(f fixture) []lookupPlan {
			return []lookupPlan{rootPlan(t, f.eng, "r7"), childPlan(t, f.eng, f.idOf(t, f.eng, "W"), "c7")}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.seed(f)
			lookups := tt.lookups(f)
			for _, stage := range []string{"without statistics", "analyzed"} {
				if stage == "analyzed" {
					f.exec(t, "ANALYZE nodes")
				}
				for _, l := range lookups {
					if l.analyzed && stage != "analyzed" {
						continue
					}
					plan, raw := f.planTreeWith(t, l.s)
					l.check(t, stage+", for its arguments", plan, raw)
					plan, raw = f.genericPlanTree(t, l.s.sql)
					l.check(t, stage+", generic", plan, raw)
				}
			}
		})
	}
}

// idOf is the id of the node of notebook named name.
func (f fixture) idOf(t *testing.T, notebook uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM nodes WHERE notebook_id = $1 AND name = $2`, notebook, name).
		Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// list's pages' states are read by the contents' key, a page of 1,000
// (M9/P1 design 3.10.3). Scans of whole tables off, as the other plans'
// tests have them: the table here is small.
func TestPageStatesReadByTheContentsKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "P", 2000)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '', 1, sha256(''), 0, $2, $3 FROM unnest($1::uuid[]) id`, ids, f.alice, now())
	plan, raw := f.planTreeOf(t, statementOf(t, func(q *gen.Queries) error {
		_, err := q.PageStates(ctx, ids[:1000])
		return err
	}))
	scans := plan.all(reads("page_contents", "page_contents_pkey", ""))
	if len(scans) != 1 || len(plan.all(seqScan)) > 0 {
		t.Fatalf("PageStates does not read page_contents by its key:\n%s", raw)
	}
	cond := scans[0].IndexCond
	for _, bitmap := range scans[0].all(func(n *planNode) bool { return n.IndexCond != "" }) {
		cond += " " + bitmap.IndexCond
	}
	if !strings.Contains(cond, ".node_id ") {
		t.Errorf("PageStates' index condition %q lacks node_id", cond)
	}
}

// list's attachments alive are read by the nodes' key, a page of 1,000 of
// a notebook's 50,000 nodes, without statistics and analyzed, with the
// production settings, planned with its argument as the store plans it
// (M9/P1 design 3.10.3).
func TestAliveNodesReadByTheNodesKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "P", 50000)
	s := statementOf(t, func(q *gen.Queries) error {
		_, err := q.AliveNodes(ctx, ids[20000:21000])
		return err
	})
	for _, stage := range []string{"without statistics", "analyzed"} {
		if stage == "analyzed" {
			f.exec(t, "ANALYZE nodes")
		}
		plan, raw := f.planTreeWith(t, s)
		scans := plan.all(func(n *planNode) bool { return n.Relation == "nodes" })
		if len(scans) != 1 || !reads("nodes", "nodes_pkey", "")(scans[0]) {
			t.Errorf("AliveNodes, %s, does not read nodes by their key:\n%s", stage, raw)
		}
	}
}

// AliveNodes are the nodes not deleted among the ids.
func TestAliveNodes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b := f.page(t, f.eng, nil, "A", 0), f.asset(t, f.eng, nil, "b.png", 1)
	gone := f.asset(t, f.eng, nil, "gone.png", 2)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	got, err := f.s.AliveNodes(ctx, []uuid.UUID{a.ID, b.ID, gone.ID, uuid.NewV7()})
	if err != nil || len(got) != 2 || !got[a.ID] || !got[b.ID] {
		t.Errorf("AliveNodes = %v, %v; want A and b.png", got, err)
	}
}
