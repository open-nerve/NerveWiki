package postgresadapter_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The arguments reach real PostgreSQL inside the consumer's actual snapshot.
// Reintroducing already-known valid pages must fail even when SQL removes them
// and the public answer remains correct.
type lintFrontmatterCall struct {
	name string
	ids  []uuid.UUID
}
type lintFrontmatterTrace struct {
	base  *lintTrace
	mu    sync.Mutex
	calls []lintFrontmatterCall
}

func (p *lintFrontmatterTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = p.base.TraceQueryStart(ctx, conn, data)
	fields := strings.Fields(data.SQL)
	if len(fields) < 3 || fields[0] != "--" || fields[1] != "name:" || !strings.HasPrefix(fields[2], "LintFrontmatter") {
		return ctx
	}
	var ids []uuid.UUID
	for _, arg := range data.Args {
		if values, ok := arg.([]uuid.UUID); ok {
			ids = slices.Clone(values)
		}
	}
	p.mu.Lock()
	p.calls = append(p.calls, lintFrontmatterCall{name: fields[2], ids: ids})
	p.mu.Unlock()
	return ctx
}
func (p *lintFrontmatterTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	p.base.TraceQueryEnd(ctx, conn, data)
}

func lintFrontmatterConsumer(t *testing.T) (*lintConsumer, *lintFrontmatterTrace) {
	t.Helper()
	c := newLintConsumer(t)
	trace := &lintFrontmatterTrace{base: c.trace}
	pool := lintPool(t, c.f, trace, 2)
	c.rules = linking.NewLintRules(pool, 5*1024*1024, c.sizes)
	c.f.tx = postgres.NewTxManager(pool, time.Second)
	return c, trace
}
func (p *lintFrontmatterTrace) assert(t *testing.T, want []uuid.UUID, count, items int) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	var counts, itemCalls int
	for _, call := range p.calls {
		if !slices.Equal(call.ids, want) {
			t.Errorf("frontmatter SQL received known-valid or excluded pages: %s candidate count=%d, want exactly %d qualified IDs", call.name, len(call.ids), len(want))
		}
		if call.name == "LintFrontmatterCount" {
			counts++
		} else {
			itemCalls++
		}
	}
	if counts != count || itemCalls != items {
		t.Errorf("frontmatter SQL call counts=%d/%d, want %d/%d", counts, itemCalls, count, items)
	}
	p.calls = nil
}
func lintFrontmatterIDs(t *testing.T, got lintAnswer, want []uuid.UUID, count int64, capped, more bool, stale int64) {
	t.Helper()
	if got.count != (linking.Count{Value: count, Capped: capped}) || got.more != more || got.stale != stale || len(got.items) != len(want) {
		t.Fatalf("frontmatter count/items/more/stale=%+v; want %d/%d/%t/%d", got, count, len(want), more, stale)
	}
	for i, item := range got.items {
		if item.PageID == nil || *item.PageID != want[i] || item.ItemKey != "p:"+want[i].String() || item.Rule != linking.InvalidFrontmatter || item.Cursor == "" {
			t.Fatalf("frontmatter item %d=%+v, want exact page %s", i, item, want[i])
		}
	}
}

func TestLintFrontmatterEligibility(t *testing.T) {
	const bad = "---\na: [\n---\n"
	const valid = "---\ntype: note\nsummary: complete\n---\nbody"
	rule := linking.InvalidFrontmatter
	for _, modeCase := range []struct {
		name string
		mode linking.LintMode
	}{{"standard", linking.ModeStandard}, {"llm", linking.ModeLLM}} {
		mode := modeCase.mode
		t.Run("mixed-"+modeCase.name, func(t *testing.T) {
			c, trace := lintFrontmatterConsumer(t)
			root := *c.in.AreaRootID
			facts := originFacts(t, bad)
			if facts.FrontmatterValid {
				t.Fatal("parser did not make the invalid root witness")
			}
			c.f.replace(t, app.Page{ID: root, NotebookID: c.f.eng, Revision: 1}, facts)
			c.in.Nodes[0].PageBytes = int64(len(bad))
			c.page(t, 0, "agents", bad, false)
			c.page(t, 10, "valid", valid, false)
			c.page(t, 20, "invalid", bad, false)
			c.page(t, 30, "old-invalid", bad, false)
			c.legacy(t, 30)
			c.page(t, 40, "source-invalid", bad, true)
			c.page(t, 41, "source-valid", valid, true)
			c.page(t, 42, "source-old", bad, true)
			c.legacy(t, 42)
			want := []uuid.UUID{lintID(0), lintID(20), root}
			if mode == linking.ModeStandard {
				c.in.EffectiveMode, c.in.AreaRootID, c.in.SourceIDs = mode, nil, nil
				want = []uuid.UUID{lintID(0), lintID(20), lintID(40), root}
			}
			first := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 2})[rule]
			lintFrontmatterIDs(t, first, want[:2], int64(len(want)), false, true, 2)
			trace.assert(t, want, 1, 1)
			after := lintCursorPosition(t, c, rule, first.items[1])
			last := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 2})[rule]
			lintFrontmatterIDs(t, last, want[2:], int64(len(want)), false, false, 2)
			trace.assert(t, want, 1, 1)
		})
		t.Run("all-valid-"+modeCase.name, func(t *testing.T) {
			c, trace := lintFrontmatterConsumer(t)
			c.f.replace(t, app.Page{ID: *c.in.AreaRootID, NotebookID: c.f.eng, Revision: 1}, originFacts(t, valid))
			c.in.Nodes[0].PageBytes = int64(len(valid))
			c.page(t, 0, "agents", valid, false)
			c.page(t, 10, "valid", valid, false)
			c.page(t, 30, "old-invalid", bad, false)
			c.legacy(t, 30)
			if mode == linking.ModeLLM {
				c.page(t, 40, "source-invalid", bad, true)
			} else {
				c.in.EffectiveMode, c.in.AreaRootID, c.in.SourceIDs = mode, nil, nil
			}
			got := c.answer(t, linking.RuleQuery{Rule: &rule})[rule]
			lintFrontmatterIDs(t, got, nil, 0, false, false, 1)
			trace.assert(t, nil, 0, 0)
			if names := c.trace.names(); !slices.Equal(names, []string{"LintHeads"}) {
				t.Fatalf("known-valid frontier still read payload rules: %v, want only actual Heads", names)
			}
		})
	}
	for _, n := range []int{7, 10007} {
		t.Run(fmt.Sprintf("complete-before-cap-%d", n), func(t *testing.T) {
			c, trace := lintFrontmatterConsumer(t)
			// Valid, source, and literal-v1 prefixes each exceed the count cap.
			// Current invalid answers come last; neither scope nor cap may truncate them.
			for group := 0; group < 3; group++ {
				ids := make([]uuid.UUID, 10001)
				for i := range ids {
					ids[i] = lintID(uint64(10 + group*20000 + i))
					node := linking.CompactNode{ID: ids[i], Kind: linking.NodePage, NameKey: "prefix", PageBytes: 1}
					if group == 1 {
						node.ParentID = c.in.AreaRootID
						c.in.SourceIDs = append(c.in.SourceIDs, ids[i])
					}
					c.in.Nodes = append(c.in.Nodes, node)
				}
				extractor := 2
				if group == 2 {
					extractor = 1
					c.stale += int64(len(ids))
				}
				lintExec(t, c.f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,$3,$4`, ids, c.f.eng, extractor, group == 0)
			}
			want := make([]uuid.UUID, n)
			for i := range want {
				want[i] = lintID(uint64(60000 + i))
				c.in.Nodes = append(c.in.Nodes, linking.CompactNode{ID: want[i], Kind: linking.NodePage, NameKey: "invalid", PageBytes: 1})
			}
			lintExec(t, c.f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,2,false`, want, c.f.eng)
			first := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 5})[rule]
			lintFrontmatterIDs(t, first, want[:5], int64(min(n, 10000)), n >= 10000, true, 10001)
			trace.assert(t, want, 1, 1)
			lastBefore := 4
			if n > 10000 {
				lastBefore = 9998
			}
			last := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 100, After: &linking.Position{ID: &want[lastBefore]}})[rule]
			lintFrontmatterIDs(t, last, want[lastBefore+1:], int64(min(n, 10000)), n >= 10000, false, 10001)
			trace.assert(t, want, 1, 1)
		})
	}
}
