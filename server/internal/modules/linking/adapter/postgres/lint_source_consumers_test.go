package postgresadapter_test

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type lintInvalidWant struct {
	page    uint64
	text    string
	notLink bool
}

func (c *lintConsumer) assertInvalid(t *testing.T, want []lintInvalidWant) {
	t.Helper()
	keys := make([]string, len(want))
	byKey := make(map[string]lintInvalidWant, len(want))
	for i, w := range want {
		keys[i] = lintSourceIdentity(lintID(w.page), w.text, w.notLink)
		byKey[keys[i]] = w
	}
	// UUID then hex digest order is also the lexical order of these identities.
	slices.Sort(keys)
	c.assertKeys(t, linking.InvalidSources, keys)
	rule := linking.InvalidSources
	r := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 100})[rule]
	if len(r.items) != len(want) {
		t.Fatalf("actual invalid-source items: %d want %d", len(r.items), len(want))
	}
	for _, item := range r.items {
		w, ok := byKey[item.ItemKey]
		if !ok || item.PageID == nil || *item.PageID != lintID(w.page) || item.NotLink != w.notLink || item.Target != w.text[:min(256, len(w.text))] || item.TargetCut != (len(w.text) > 256) {
			t.Fatalf("full L/J identity and display: %+v, expected %+v", item, w)
		}
	}
}

func testLintFirstSourcesConsumer(t *testing.T) {
	c := newLintConsumer(t)
	facts := c.page(t, 0, "wiki", "---\nſources: '[[wrong]]'\nSoUrCeS: ['[[missing]]', plain, {0: '[[object]]'}, ['[[nested]]'], 3, null, true]\nsources: '[[later]]'\n---\n", false)
	if len(facts.Properties) != 3 || facts.Properties[1].Key != "SoUrCeS" || len(facts.Links) != 5 {
		t.Fatalf("real first-ASCII/property/edge fixture: %+v", facts)
	}
	c.assertInvalid(t, []lintInvalidWant{{0, "missing", false}, {0, `"plain"`, true}, {0, `{"0":"[[object]]"}`, true}, {0, `["[[nested]]"]`, true}, {0, "3", true}, {0, "null", true}, {0, "true", true}})
	// The actual nested/property links still belong to the general edge rule;
	// plain JSON strings do not invent an edge.
	var keys []string
	for _, target := range []string{"later", "missing", "nested", "object", "wrong"} {
		keys = append(keys, "w:"+lintHashText(target))
	}
	c.assertKeys(t, linking.Wanted, keys)
}

func testLintSourceLinkBoundary(t *testing.T) {
	c := newLintConsumer(t)
	facts := c.page(t, 0, "wiki", "---\nsources: ['[[actual]]', plain, 3]\n---\n", false)
	if len(facts.Links) != 1 || facts.Links[0].Target != "actual" {
		t.Fatalf("actual edge versus JSON-only values: %+v", facts.Links)
	}
	c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText("actual")})
	c.assertInvalid(t, []lintInvalidWant{{0, "actual", false}, {0, `"plain"`, true}, {0, "3", true}})
}

func testLintSourceIdentities(t *testing.T) {
	c := newLintConsumer(t)
	a, b := strings.Repeat("x", 280)+"a", strings.Repeat("x", 280)+"b"
	c.page(t, 0, "wiki", "---\nsources: [0, '[[0]]', '"+a+"', '"+b+"', '"+a+"']\n---\n", false)
	c.assertInvalid(t, []lintInvalidWant{{0, "0", true}, {0, "0", false}, {0, `"` + a + `"`, true}, {0, `"` + b + `"`, true}})
}

func testLintSourceCollisions(t *testing.T) {
	for _, variant := range []string{"both-in-area", "unresolved", "outside", "asset"} {
		t.Run(variant, func(t *testing.T) {
			c := newLintConsumer(t)
			c.page(t, 100, "a", "source", true)
			c.page(t, 101, "b", "source", true)
			c.page(t, 200, "outside", "outside", false)
			c.asset(300, "attachment.pdf")
			facts := c.page(t, 0, "wiki", "---\nSources: ['[[raw/a]]']\nSources.0: '[[raw/b]]'\n---\n", false)
			if len(facts.Links) != 2 || facts.Links[0].Property != "Sources.0" || facts.Links[1].Property != "Sources.0" {
				t.Fatalf("real dotted-key collision: %+v", facts.Links)
			}
			targets := map[string]domain.Resolution{"raw/a": {ID: lintID(100)}}
			switch variant {
			case "both-in-area":
				targets["raw/b"] = domain.Resolution{ID: lintID(101)}
			case "outside":
				targets["raw/b"] = domain.Resolution{ID: lintID(200)}
			case "asset":
				targets["raw/b"] = domain.Resolution{ID: lintID(300), Asset: true}
			}
			c.resolve(t, 0, facts, targets)
			c.assertInvalid(t, []lintInvalidWant{{0, `"[[raw/a]]"`, true}})
			c.assertPending(t, []uuid.UUID{lintID(100), lintID(101), lintID(300)})
		})
	}
}

func testLintSourceOldObject(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 100, "a", "source", true)
	facts := c.page(t, 0, "wiki", "---\nSources: {0: '[[raw/a]]'}\n---\n", false)
	if len(facts.Links) != 1 || facts.Links[0].Property != "Sources.0" {
		t.Fatalf("real object path witness: %+v", facts.Links)
	}
	c.resolve(t, 0, facts, map[string]domain.Resolution{"raw/a": {ID: lintID(100)}})
	c.assertInvalid(t, []lintInvalidWant{{0, `{"0":"[[raw/a]]"}`, true}})
	c.assertPending(t, []uuid.UUID{lintID(100)})
}

func testLintSourceCrossBatches(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 0, "wiki", "---\nSources: ['[[same]]']\n---\n", false)
	c.page(t, 1, "later", "---\nSources: '[[tail-only]]'\n---\n", false)
	// Real metadata/JSON and the actual consumer surround a deliberately dense
	// stored-edge fixture. Parser growth itself is proved separately by S62.
	lintExec(t, c.f, `DELETE FROM page_links WHERE source_id=$1`, lintID(0))
	big := "a/" + strings.Repeat("b", 3*1024*1024) + "/x"
	lintExec(t, c.f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,ambiguous)
 SELECT $1,k,k+1,$2,'wikilink','Sources.0',CASE WHEN k<3 THEN $3 ELSE 'same' END,'x',false FROM generate_series(0,1024) k`, lintID(0), c.f.eng, big)
	rule := linking.InvalidSources
	r := c.answer(t, linking.RuleQuery{Rule: &rule, Limit: 10})[rule]
	want := []string{lintSourceIdentity(lintID(0), `"[[same]]"`, true), lintSourceIdentity(lintID(1), "tail-only", false)}
	if r.count != (linking.Count{Value: 2}) || len(r.items) != 2 || r.more || r.items[0].ItemKey != want[0] || !r.items[0].NotLink || r.items[1].ItemKey != want[1] || r.items[1].NotLink {
		t.Fatalf("slot cardinality was lost before Finish/next source: %+v", r)
	}
	names := c.trace.names()
	heads, exact := 0, 0
	for _, name := range names {
		if name == "LintSourceSlotHeadsFirst" || name == "LintSourceSlotHeadsAfter" {
			heads++
		}
		if name == "LintSourceSlotEdges" {
			exact++
		}
	}
	if heads < 3 || exact < 4 {
		t.Fatalf("missing actual row/byte source-slot cuts: %v", names)
	}
	// Continue from the real first item's cursor, then from the final item.
	after := lintCursorPosition(t, c, rule, r.items[0])
	next := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 1})[rule]
	if next.count != r.count || len(next.items) != 1 || next.items[0].ItemKey != want[1] || next.more {
		t.Fatalf("later nonempty source lost: %+v", next)
	}
	end := c.answer(t, linking.RuleQuery{Rule: &rule, After: lintCursorPosition(t, c, rule, next.items[0]), Limit: 1})[rule]
	if end.count != r.count || len(end.items) != 0 || end.more {
		t.Fatalf("source cursor termination: %+v", end)
	}
}

func testLintSourceMultiParent(t *testing.T) {
	c := newLintConsumer(t)
	a := c.page(t, 100, "a", "[[shared.pdf]] [[a.pdf]]", true)
	b := c.page(t, 101, "b", "[[shared.pdf]] [[b.pdf]]", true)
	empty := c.page(t, 102, "empty", "[[empty.pdf]]", true)
	old := c.page(t, 103, "old", "[[old.pdf]]", true)
	c.legacy(t, 103)
	// Page byte eligibility is authoritative even if an indexed edge remains.
	// This deliberately exercises that metadata/index boundary, not a claim
	// that parsing an empty body can emit a reference.
	for i := range c.in.Nodes {
		if c.in.Nodes[i].ID == lintID(102) {
			c.in.Nodes[i].PageBytes = 0
		}
	}
	for n, key := range []string{"shared.pdf", "a.pdf", "b.pdf", "direct.pdf", "empty.pdf", "old.pdf", "alone.pdf"} {
		c.asset(uint64(200+n), key)
	}
	c.resolve(t, 100, a, map[string]domain.Resolution{"shared.pdf": {ID: lintID(200), Asset: true}, "a.pdf": {ID: lintID(201), Asset: true}})
	c.resolve(t, 101, b, map[string]domain.Resolution{"shared.pdf": {ID: lintID(200), Asset: true}, "b.pdf": {ID: lintID(202), Asset: true}})
	c.resolve(t, 102, empty, map[string]domain.Resolution{"empty.pdf": {ID: lintID(204), Asset: true}})
	c.resolve(t, 103, old, map[string]domain.Resolution{"old.pdf": {ID: lintID(205), Asset: true}})
	wiki := c.page(t, 0, "wiki", "[[raw/a]] [[direct.pdf]]", false)
	c.resolve(t, 0, wiki, map[string]domain.Resolution{"raw/a": {ID: lintID(100)}, "direct.pdf": {ID: lintID(203), Asset: true}})
	// shared has both digested and undigested parents; b.pdf is represented by
	// the undigested parent. Direct wiki D overrides the absence of all parents.
	c.assertPending(t, []uuid.UUID{lintID(101), lintID(204), lintID(205), lintID(206)})
}

func testLintNormalMixedVersionScope(t *testing.T) {
	c := newLintConsumer(t)
	c.page(t, 100, "source", "source", true)
	old := c.page(t, 0, "old", "---\nsources: '[[old-only]]'\n---\n[[raw/source]]", false)
	c.resolve(t, 0, old, map[string]domain.Resolution{"raw/source": {ID: lintID(100)}})
	c.legacy(t, 0)
	c.page(t, 1, "current", "---\nsources: '[[new-only]]'\n---\n", false)
	c.assertKeys(t, linking.Wanted, []string{"w:" + lintHashText("new-only")})
	c.assertInvalid(t, []lintInvalidWant{{1, "new-only", false}})
	c.assertPending(t, []uuid.UUID{lintID(100)})
}
