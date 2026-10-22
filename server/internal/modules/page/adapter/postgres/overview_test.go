package postgresadapter_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
)

// changesetAt inserts a changeset of notebook written last at updated,
// made an hour before, deleted at deleted when it is not nil.
func (f fixture) changesetAt(t *testing.T, notebook uuid.UUID, updated time.Time, deleted *time.Time) {
	t.Helper()
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, deleted_at)
		VALUES (uuidv7(), $1, 'edit', 'web', $2, $3, $4, $5)`, notebook, f.alice, updated.Add(-time.Hour), updated, deleted)
}

// The overview's reads: the pages and attachments not deleted of the
// notebook alone; the latest updated_at of its changesets not deleted, nil
// without one; the root's first children in the tree's order, at most n,
// none below the root (M9/P1 design 3.17.3).
func TestTheOverviewReadsANotebook(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if pages, attachments, err := f.s.NotebookCounts(ctx, f.eng); pages != 0 || attachments != 0 || err != nil {
		t.Errorf("NotebookCounts of an empty notebook = %d, %d, %v", pages, attachments, err)
	}
	if last, err := f.s.NotebookLastChange(ctx, f.eng); last != nil || err != nil {
		t.Errorf("NotebookLastChange of an empty notebook = %v, %v; want nil", last, err)
	}
	if root, err := f.s.FirstChildren(ctx, f.eng, 3); len(root) != 0 || err != nil {
		t.Errorf("FirstChildren of an empty notebook = %v, %v", names(root), err)
	}
	b := f.page(t, f.eng, nil, "B", 2)
	f.page(t, f.eng, nil, "A", 1)
	f.page(t, f.eng, &b.ID, "Below", 0)
	f.asset(t, f.eng, nil, "logo.png", 3)
	f.asset(t, f.eng, &b.ID, "diagram.png", 1)
	f.page(t, f.eng, nil, "D", 4)
	gone := f.page(t, f.eng, nil, "Gone", 0.5)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	f.page(t, f.ops, nil, "Elsewhere", 0)
	pages, attachments, err := f.s.NotebookCounts(ctx, f.eng)
	if pages != 4 || attachments != 2 || err != nil {
		t.Errorf("NotebookCounts = %d, %d, %v; want 4 pages, 2 attachments", pages, attachments, err)
	}
	later := now().Add(2 * time.Hour)
	f.changesetAt(t, f.eng, now(), nil)
	f.changesetAt(t, f.eng, later, nil)
	f.changesetAt(t, f.eng, later.Add(time.Hour), &later)
	f.changesetAt(t, f.ops, later.Add(2*time.Hour), nil)
	if last, err := f.s.NotebookLastChange(ctx, f.eng); last == nil || !last.Equal(later) || err != nil {
		t.Errorf("NotebookLastChange = %v, %v; want %v", last, err, later)
	}
	for n, want := range map[int][]string{3: {"A", "B", "logo.png"}, 10: {"A", "B", "logo.png", "D"}} {
		if root, err := f.s.FirstChildren(ctx, f.eng, n); err != nil || !slices.Equal(names(root), want) {
			t.Errorf("FirstChildren(%d) = %v, %v; want %v", n, names(root), err, want)
		}
	}
}

// predicateOf is index's predicate, "" for an index whole.
func (f fixture) predicateOf(t *testing.T, index string) string {
	t.Helper()
	var predicate string
	err := f.pool.QueryRow(context.Background(), `SELECT coalesce(pg_get_expr(indpred, indrelid), '') FROM pg_index
		WHERE indexrelid = $1::regclass`, index).Scan(&predicate)
	if err != nil {
		t.Fatal(err)
	}
	return predicate
}

// leadingOf is index's first column.
func (f fixture) leadingOf(t *testing.T, index string) string {
	t.Helper()
	var column string
	err := f.pool.QueryRow(context.Background(), `SELECT a.attname FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0] WHERE i.indexrelid = $1::regclass`, index).Scan(&column)
	if err != nil {
		t.Fatal(err)
	}
	return column
}

// The overview reads by indexes of the notebook (13.4 item 9), before the
// statistics and after: FirstChildren the root's nodes alone, parent_id IS
// NULL an index condition or the predicate of the index it reads (the
// roots' unique index, 00042), which IS NOT DISTINCT FROM would be
// neither; the counts the notebook's nodes, the last change its
// changesets, each by an index that starts with notebook_id, any that
// does: the planner takes one or another as the data go, the two of the
// log sized alike (M9/P1 design 3.17.3, review A-M16, B-M3, its check B
// N-5). Scans of whole tables off, as the other plans' tests have them:
// the tables here are small.
func TestTheOverviewReadsUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"nodes", "changesets"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	ids := f.nodes(t, f.eng, "P", 2000)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT uuidv7(), $1, $2, 'page', 'C' || i, 'c' || i, i, $3, $3, $4, $4 FROM generate_series(1, 2000) i`, f.eng, ids[0], f.alice, now())
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, 'edit', 'web', $2, $3, $3 FROM generate_series(1, 2000) k`, f.eng, f.alice, now())
	for _, stage := range []string{"without statistics", "with statistics"} {
		if stage == "with statistics" {
			f.exec(t, "ANALYZE nodes, changesets")
		}
		for _, tt := range []struct {
			name  string
			s     statement
			table string
			root  bool // the root's nodes alone
		}{
			{"FirstChildren", statementOf(t, func(q *gen.Queries) error {
				_, err := q.FirstChildren(ctx, gen.FirstChildrenParams{NotebookID: f.eng, N: 101})
				return err
			}), "nodes", true},
			{"NotebookCounts", statementOf(t, func(q *gen.Queries) error {
				_, err := q.NotebookCounts(ctx, f.eng)
				return err
			}), "nodes", false},
			{"NotebookLastChange", statementOf(t, func(q *gen.Queries) error {
				_, err := q.NotebookLastChange(ctx, f.eng)
				return err
			}), "changesets", false},
		} {
			plan, raw := f.planTreeOf(t, tt.s)
			var scans []*planNode
			for _, read := range plan.all(func(n *planNode) bool { return n.Relation == tt.table }) {
				scans = append(scans, read.all(func(n *planNode) bool { return n.Index != "" && strings.Contains(n.IndexCond, ".notebook_id ") })...)
			}
			if len(scans) != 1 || len(plan.all(seqScan)) > 0 {
				t.Errorf("%s, %s: not one read of %s by the notebook:\n%s", stage, tt.name, tt.table, raw)
				continue
			}
			index, predicate := scans[0].Index, f.predicateOf(t, scans[0].Index)
			switch {
			case tt.root && !strings.Contains(scans[0].IndexCond+" "+predicate, "parent_id IS NULL"):
				t.Errorf("%s, %s: %s reads more than the root, condition %q, predicate %q", stage, tt.name, index, scans[0].IndexCond,
					predicate)
			case !tt.root && f.leadingOf(t, index) != "notebook_id":
				t.Errorf("%s, %s: read by %s, which does not start with notebook_id", stage, tt.name, index)
			}
		}
	}
}

// index's child pages are read by an index of a parent's children (M9/P4
// design 3.9.1; S2 review A-N1), before the statistics and after, custom
// plans and generic: RootPageIDs with parent_id IS NULL an index condition
// or the predicate of the index it reads (00012's, or the roots' of
// 00042), never a read of the notebook's nodes whole; ChildPageIDs with
// parent_id an index condition, by 00012's (notebook_id, parent_id) or
// 00042's children's (parent_id, name_key), which the planner takes as the
// data go: a parent of 2,000 children and one of none. Scans of whole
// tables off, as the overview's.
func TestTheChildPagesReadsUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, "ALTER TABLE nodes SET (autovacuum_enabled = false)")
	ids := f.nodes(t, f.eng, "P", 2000)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT uuidv7(), $1, $2, 'page', 'C' || i, 'c' || i, i, $3, $3, $4, $4 FROM generate_series(1, 2000) i`, f.eng, ids[0], f.alice, now())
	for _, stage := range []string{"without statistics", "with statistics"} {
		if stage == "with statistics" {
			f.exec(t, "ANALYZE nodes")
		}
		for _, tt := range []struct {
			name string
			s    statement
			root bool
		}{
			{"RootPageIDs", statementOf(t, func(q *gen.Queries) error {
				_, err := q.RootPageIDs(ctx, f.eng)
				return err
			}), true},
			{"ChildPageIDs of 2,000", statementOf(t, func(q *gen.Queries) error {
				_, err := q.ChildPageIDs(ctx, gen.ChildPageIDsParams{NotebookID: f.eng, ParentID: &ids[0]})
				return err
			}), false},
			{"ChildPageIDs of none", statementOf(t, func(q *gen.Queries) error {
				_, err := q.ChildPageIDs(ctx, gen.ChildPageIDsParams{NotebookID: f.eng, ParentID: &ids[1]})
				return err
			}), false},
		} {
			for _, generic := range []bool{false, true} {
				plan, raw := f.planTreeOf(t, tt.s)
				if generic {
					plan, raw = f.genericPlanTree(t, tt.s.sql, "enable_seqscan=off")
				}
				var reads []*planNode
				for _, read := range plan.all(func(n *planNode) bool { return n.Relation == "nodes" }) {
					reads = append(reads, read.all(func(n *planNode) bool { return n.Index != "" })...)
				}
				ok := len(reads) > 0 && len(plan.all(seqScan)) == 0
				for _, r := range reads {
					switch {
					case tt.root:
						ok = ok && strings.Contains(r.IndexCond+" "+f.predicateOf(t, r.Index), "parent_id IS NULL")
					default:
						ok = ok && strings.Contains(r.IndexCond, "parent_id =")
					}
				}
				if !ok {
					t.Errorf("%s, %s, generic %v: not read by an index of the parent's children:\n%s", stage, tt.name, generic, raw)
				}
			}
		}
	}
}
