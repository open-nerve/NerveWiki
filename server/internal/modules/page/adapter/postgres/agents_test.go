package postgresadapter_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The AGENTS page is the notebook's root node not deleted of the title
// key, with its title now and its content's revision; an attachment of
// the key has no revision; a node of the key below the root, deleted, or
// in another notebook is none (M9/P1 design 3.15.1).
func TestTheAgentsPageIsTheRootOfItsKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	key := shared.TitleKey(domain.AgentsTitle)
	docs := f.page(t, f.eng, nil, "Docs", 0)
	f.page(t, f.eng, &docs.ID, "AGENTS", 0)
	f.page(t, f.ops, nil, "AGENTS", 0)
	agents := f.page(t, f.eng, nil, "Agents", 1)
	f.exec(t, "UPDATE page_contents SET revision = 4 WHERE node_id = $1", agents.ID)
	node, kind, title, revision, ok, err := f.s.AgentsPage(ctx, f.eng, key)
	if err != nil || !ok || node != agents.ID || kind != domain.KindPage || title != "Agents" || revision != 4 {
		t.Errorf("AgentsPage = %s %s %q %d %v %v, want Agents at revision 4", node, kind, title, revision, ok, err)
	}
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", agents.ID, now())
	if _, _, _, _, ok, err := f.s.AgentsPage(ctx, f.eng, key); ok || err != nil {
		t.Errorf("a deleted AGENTS page is found: %v, %v", ok, err)
	}
	asset := f.asset(t, f.eng, nil, "agents", 2)
	node, kind, _, revision, ok, err = f.s.AgentsPage(ctx, f.eng, key)
	if err != nil || !ok || node != asset.ID || kind != domain.KindAsset || revision != 0 {
		t.Errorf("an attachment of the key = %s %s %d %v %v, want it, of no revision", node, kind, revision, ok, err)
	}
	if _, _, _, _, ok, err := f.s.AgentsPage(ctx, uuid.NewV7(), key); ok || err != nil {
		t.Errorf("a notebook of none = %v, %v", ok, err)
	}
}

// The AGENTS page is one point lookup, as RootByKey (M9/P1 design
// 3.15.1): the notebook's roots of the key by the roots' unique index
// (00042), both its columns equalities of its condition, and the content
// by the contents' key; with statistics or without, planned for its
// arguments or generic. The data are of a key shared by most nodes and by
// roots across notebooks: 20,000 roots of a notebook, each with a child
// AGENTS, its own root AGENTS, and one in each of 2,000 notebooks; each
// page with its content. Analyzed, the roots of the key were estimated as
// the key's share of the table, and a join of the contents read them all
// by a hash: the revision is a subquery.
func TestTheAgentsPageIsAPointLookup(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	const insert = `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at) `
	f.exec(t, insert+`SELECT uuidv7(), $1, NULL, 'page', 'R' || i, 'r' || i, i, $2, $2, $3, $3 FROM generate_series(1, 20000) i`,
		f.eng, f.alice, now())
	f.exec(t, insert+`SELECT uuidv7(), $1, p.id, 'page', 'AGENTS', 'agents', 0, $2, $2, $3, $3 FROM nodes p WHERE p.notebook_id = $1`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT uuidv7(), workspace_id, 'N' || i, $2, $2, $3, $3 FROM notebooks, generate_series(1, 2000) i WHERE id = $1`,
		f.eng, f.alice, now())
	f.exec(t, insert+`SELECT uuidv7(), id, NULL, 'page', 'AGENTS', 'agents', 0, $1, $1, $2, $2 FROM notebooks`, f.alice, now())
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '', 1, sha256(''), 0, $1, $2 FROM nodes`, f.alice, now())
	s := statementOf(t, func(q *gen.Queries) error {
		_, err := q.AgentsPage(ctx, gen.AgentsPageParams{NotebookID: f.eng, NameKey: shared.TitleKey(domain.AgentsTitle)})
		return err
	})
	check := func(stage string, plan planNode, raw string) {
		t.Helper()
		for _, tt := range []struct {
			table, index string
			columns      []string
		}{
			{"nodes", "nodes_notebook_id_name_key_root_key", []string{"notebook_id", "name_key"}},
			{"page_contents", "page_contents_pkey", []string{"node_id"}},
		} {
			scans := plan.all(func(n *planNode) bool { return n.Relation == tt.table })
			if len(scans) != 1 || !reads(tt.table, tt.index, "")(scans[0]) {
				t.Errorf("AgentsPage, %s, reads %s by other than %s:\n%s", stage, tt.table, tt.index, raw)
				continue
			}
			cond := scans[0].IndexCond
			for _, bitmap := range scans[0].all(func(n *planNode) bool { return n.Index == tt.index }) {
				cond += " " + bitmap.IndexCond
			}
			for _, column := range tt.columns {
				if !strings.Contains(cond, "."+column+" = ") {
					t.Errorf("AgentsPage, %s: the condition on %s, %q, has no equality of %s", stage, tt.table, cond, column)
				}
			}
		}
	}
	for _, stage := range []string{"without statistics", "analyzed"} {
		if stage == "analyzed" {
			f.exec(t, "ANALYZE nodes, page_contents")
		}
		plan, raw := f.planTreeWith(t, s)
		check(stage+", for its arguments", plan, raw)
		plan, raw = f.genericPlanTree(t, s.sql)
		check(stage+", generic", plan, raw)
	}
}
