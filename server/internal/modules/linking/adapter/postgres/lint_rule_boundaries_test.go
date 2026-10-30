package postgresadapter_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func (c *lintConsumer) assertKeys(t *testing.T, rule linking.Rule, want []string) {
	t.Helper()
	preview := c.answer(t, linking.RuleQuery{})[rule]
	if preview.count != (linking.Count{Value: int64(len(want))}) || preview.more != (len(want) > 5) || preview.stale != c.stale {
		t.Fatalf("%s summary: %+v want %d stale=%d", rule, preview, len(want), c.stale)
	}
	keys := func(items []linking.Item) []string {
		out := make([]string, len(items))
		for i, v := range items {
			out[i] = v.ItemKey
		}
		return out
	}
	if got := keys(preview.items); !slices.Equal(got, want[:min(5, len(want))]) {
		t.Fatalf("%s preview: %v want %v", rule, got, want)
	}
	var got []string
	var after *linking.Position
	for {
		page := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 2})[rule]
		if page.count != preview.count || page.stale != c.stale {
			t.Fatalf("%s page count/stale: %+v", rule, page)
		}
		got = append(got, keys(page.items)...)
		if !page.more {
			break
		}
		if len(page.items) == 0 || len(got) > len(want) {
			t.Fatalf("%s no cursor progress", rule)
		}
		after = lintCursorPosition(t, c, rule, page.items[len(page.items)-1])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s all items: %v want %v", rule, got, want)
	}
}

func TestLintWantedUnion(t *testing.T) {
	t.Run("raw-spellings-on-same-page", func(t *testing.T) {
		c := newLintConsumer(t)
		c.page(t, 0, "wiki", "[[Note]] [[note.md]] [[/NOTE]] [[Note]]", false)
		c.page(t, 1, "other", "[[note]]", false)
		c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText("note")})
		r := c.answer(t, linking.RuleQuery{Rule: lintRulePointer(linking.Wanted), Limit: 1})[linking.Wanted]
		if len(r.items) != 1 || r.items[0].Pages != 2 || r.items[0].Target != "Note" {
			t.Fatalf("actual source-local union/first raw: %+v", r)
		}
	})
	t.Run("page-and-asset-spellings", func(t *testing.T) {
		c := newLintConsumer(t)
		c.page(t, 0, "wiki", "[[Node.js]] [[node.js.md]] [[Node.js]]", false)
		c.page(t, 1, "node.js", "---\ntype: note\nsummary: ok\n---\npage", false)
		// A non-source asset contributes presence but never a page candidate.
		c.in.Nodes = append(c.in.Nodes, linking.CompactNode{ID: lintID(2), Kind: linking.NodeAsset, NameKey: "node.js", Linkable: true})
		c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText("node.js")})
		r := c.answer(t, linking.RuleQuery{Rule: lintRulePointer(linking.Wanted), Limit: 1})[linking.Wanted]
		if r.items[0].Pages != 1 || r.items[0].Target != "node.js.md" {
			t.Fatalf("asset spelling changed page union/representative: %+v", r.items)
		}
	})
	t.Run("representative-after-kind-filter", func(t *testing.T) {
		c := newLintConsumer(t)
		c.page(t, 0, "wiki", "![picture](Note.js.md) [[NOTE.JS]]", false)
		c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText("note.js")})
		r := c.answer(t, linking.RuleQuery{Rule: lintRulePointer(linking.Wanted), Limit: 1})[linking.Wanted]
		if r.items[0].Target != "NOTE.JS" || r.items[0].Pages != 1 {
			t.Fatalf("filtered image became representative: %+v", r.items)
		}
	})
}
func lintRulePointer(rule linking.Rule) *linking.Rule { return &rule }

func testLintEligibilityBeforeCap(t *testing.T) {
	c := newLintConsumer(t)
	ids := make([]uuid.UUID, 10000)
	for i := range ids {
		ids[i] = lintID(uint64(i))
		c.in.SourceIDs = append(c.in.SourceIDs, ids[i])
		c.in.Nodes = append(c.in.Nodes, linking.CompactNode{ID: ids[i], ParentID: c.in.AreaRootID, Kind: linking.NodePage, NameKey: fmt.Sprintf("source-%d", i), CreatedAt: time.Unix(1700000000, 0).UTC()})
	}
	lintExec(t, c.f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,2,true`, ids, c.f.eng)
	var want []string
	for n := uint64(10000); n < 10007; n++ {
		c.page(t, n, fmt.Sprintf("wiki-%d", n), "wiki", false)
		want = append(want, "p:"+lintID(n).String())
	}
	for _, rule := range []linking.Rule{linking.Orphans, linking.MissingType, linking.MissingSummary} {
		c.assertKeys(t, rule, want)
	}
}
func testLintLongIdentities(t *testing.T) {
	c := newLintConsumer(t)
	a := "dir/" + strings.Repeat("a", 280) + "/x"
	b := "dir/" + strings.Repeat("a", 280) + "/y"
	c.page(t, 0, "wiki", "[["+a+"]] [["+b+"]]", false)
	c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText(a), "w:" + lintHashText(b)})
	r := c.answer(t, linking.RuleQuery{Rule: lintRulePointer(linking.Wanted), Limit: 2})[linking.Wanted]
	if len(r.items) != 2 || r.items[0].Target != r.items[1].Target || !r.items[0].TargetCut || !r.items[1].TargetCut || r.items[0].ItemKey == r.items[1].ItemKey {
		t.Fatalf("same display must keep two full identities: %+v", r.items)
	}
}
func testLintConsumerCountBeyondCap(t *testing.T) {
	c := newLintConsumer(t)
	for part := 0; part < 3; part++ {
		var body strings.Builder
		for n := part * 4000; n < min((part+1)*4000, 10007); n++ {
			fmt.Fprintf(&body, "[[wanted-%05d]] ", n)
		}
		facts := c.page(t, uint64(part), fmt.Sprintf("wiki-%d", part), body.String(), false)
		if len(facts.Links) != min(4000, 10007-part*4000) {
			t.Fatalf("parser's actual persisted links: %d", len(facts.Links))
		}
	}
	rule := linking.Wanted
	count := linking.Count{Value: 10000, Capped: true}
	summary := c.answer(t, linking.RuleQuery{})[rule]
	if summary.count != count || len(summary.items) != 5 || !summary.more {
		t.Fatalf("actual capped summary: %+v", summary)
	}
	after := &linking.Position{Pages: 1, Prefix: []byte("wanted-09998"), Hash: lintHashText("wanted-09998")}
	r := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 20})[rule]
	if r.count != count || len(r.items) != 8 || r.more {
		t.Fatalf("count cap truncated real later items: %+v", r)
	}
	for i, item := range r.items {
		raw := fmt.Sprintf("wanted-%05d", 9999+i)
		if item.ItemKey != "w:"+lintHashText(raw) || item.Pages != 1 || item.Target != raw {
			t.Fatalf("cap-tail item %d: %+v", i, item)
		}
	}
}
func testLintWantedAcrossBatches(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 0, "wiki", "stream witness", false)
	big := "a/" + strings.Repeat("b", 3*1024*1024) + "/x"
	// This is a bounded-reader index fixture, independently of the legal parser
	// growth witness in LP. Both first and final row batches have unique outputs.
	lintExec(t, c.f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous)
 SELECT $1,k,k+1,$2,'wikilink',CASE WHEN k<3 THEN $3 WHEN k=1024 THEN 'tail-only' ELSE 'same' END,CASE WHEN k<3 THEN 'x' WHEN k=1024 THEN 'tail-only' ELSE 'same' END,false FROM generate_series(0,1024) k`, lintID(0), c.f.eng, big)
	rule := linking.Wanted
	r := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 10})[rule]
	if r.count != (linking.Count{Value: 3}) || len(r.items) != 3 || r.more {
		t.Fatalf("real row/byte union: %+v", r)
	}
	expected := []string{big, "same", "tail-only"}
	for i, item := range r.items {
		if item.Pages != 1 || item.ItemKey != "w:"+lintHashText(expected[i]) {
			t.Fatalf("source seen reset at a row/byte cut: %+v", r.items)
		}
	}
	names := c.trace.names()
	heads, exact := 0, 0
	for _, name := range names {
		if name == "LintWantedEdgeHeadsFirst" || name == "LintWantedEdgeHeadsAfter" {
			heads++
		}
		if name == "LintEdgesByPositions" {
			exact++
		}
	}
	if heads < 3 || exact < 4 {
		t.Fatalf("no real first/later row and byte batches: %v", names)
	}
}

func testLintRootAgentsScopes(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 100, "a", "source", true)
	rootContent := "---\nsources: ['[[raw/a]]','[[bad]]']\n---\n"
	rootFacts := originFacts(t, rootContent)
	c.f.replace(t, app.Page{ID: *c.in.AreaRootID, NotebookID: c.f.eng, Revision: 1}, rootFacts)
	c.in.Nodes[0].PageBytes = int64(len(rootContent))
	c.resolve(t, 900000, rootFacts, map[string]domain.Resolution{"raw/a": {ID: lintID(100)}})
	agentFacts := c.page(t, 0, "agents", "---\nsources: ['[[raw/a]]','[[bad]]']\n---\n", false)
	c.resolve(t, 0, agentFacts, map[string]domain.Resolution{"raw/a": {ID: lintID(100)}})
	c.page(t, 50, "folder", "", false)
	c.page(t, 1, "agents", "---\nsources: '[[bad]]'\n---\n", false)
	for i := range c.in.Nodes {
		if c.in.Nodes[i].ID == lintID(1) {
			id := lintID(50)
			c.in.Nodes[i].ParentID = &id
		}
	}
	fenced := c.page(t, 2, "fenced", "```yaml\nsources: '[[bad]]'\n```", false)
	if len(fenced.Properties) != 0 || len(fenced.Links) != 0 {
		t.Fatal("fenced example became actual metadata")
	}
	c.assertKeys(t, linking.InvalidSources, []string{lintSourceIdentity(lintID(0), "bad", false), lintSourceIdentity(lintID(1), "bad", false), lintSourceIdentity(*c.in.AreaRootID, "bad", false)})
	c.assertKeys(t, linking.Orphans, []string{"p:" + lintID(1).String(), "p:" + lintID(2).String()})
	c.assertKeys(t, linking.MissingType, []string{"p:" + lintID(1).String(), "p:" + lintID(2).String(), "p:" + lintID(50).String()})
	c.assertPending(t, []uuid.UUID{lintID(100)})
}
func testLintEmptyPageVersusFolder(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 0, "empty-leaf", "", false)
	c.page(t, 1, "folder", "", false)
	c.in.Nodes = append(c.in.Nodes, linking.CompactNode{ID: lintID(400), ParentID: lintRuleIDPointer(1), Kind: linking.NodeAsset, NameKey: "child.pdf", Linkable: true})
	c.page(t, 100, "empty-source-leaf", "", true)
	c.page(t, 101, "empty-source-parent", "", true)
	c.page(t, 102, "source", "source", true)
	c.asset(202, "child.pdf")
	for i := range c.in.Nodes {
		if c.in.Nodes[i].ID == lintID(202) {
			c.in.Nodes[i].ParentID = lintRuleIDPointer(101)
		}
	}
	c.assertKeys(t, linking.Orphans, []string{"p:" + lintID(0).String()})
	c.assertPending(t, []uuid.UUID{lintID(102), lintID(202)})
}
func lintRuleIDPointer(n uint64) *uuid.UUID { id := lintID(n); return &id }
