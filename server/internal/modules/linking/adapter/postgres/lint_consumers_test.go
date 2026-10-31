package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Only compact page metadata and the final asset-size port are supplied here;
// every head, edge, property, count and pending-F answer is real PostgreSQL.
// Bootstrap separately verifies the real page/asset composition boundary.
type lintConsumer struct {
	f     fixture
	in    linking.ReadInput
	sizes *lintConsumerSizes
	rules *linking.LintRules
	trace *lintTrace
	stale int64
}
type lintConsumerSizes struct {
	notebook uuid.UUID
	values   map[uuid.UUID]int64
	calls    [][]uuid.UUID
}

func (s *lintConsumerSizes) BytesOf(_ context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if notebook != s.notebook {
		return nil, fmt.Errorf("foreign asset notebook")
	}
	s.calls = append(s.calls, slices.Clone(ids))
	out := make(map[uuid.UUID]int64, len(ids))
	for _, id := range ids {
		n, ok := s.values[id]
		if !ok {
			return nil, fmt.Errorf("missing actual fixture asset %s", id)
		}
		out[id] = n
	}
	return out, nil
}
func newLintConsumer(t *testing.T) *lintConsumer {
	t.Helper()
	f := newFixture(t)
	area := lintID(900000)
	c := &lintConsumer{f: f, in: linking.ReadInput{NotebookID: f.eng, EffectiveMode: linking.ModeLLM, Explicit: true, AreaRootID: &area}, sizes: &lintConsumerSizes{notebook: f.eng, values: map[uuid.UUID]int64{}}, trace: &lintTrace{}}
	c.in.Nodes = []linking.CompactNode{{ID: area, Kind: linking.NodePage, NameKey: "raw", CreatedAt: time.Unix(1700000000, 0).UTC()}}
	pool := lintPool(t, f, c.trace, 2)
	c.rules = linking.NewLintRules(pool, 5*1024*1024, c.sizes)
	c.f.tx = postgres.NewTxManager(pool, time.Second)
	return c
}
func (c *lintConsumer) page(t *testing.T, n uint64, key, content string, source bool) domain.Facts {
	t.Helper()
	id := lintID(n)
	facts := originFacts(t, content)
	c.f.replace(t, app.Page{ID: id, NotebookID: c.f.eng, Revision: 1}, facts)
	node := linking.CompactNode{ID: id, Kind: linking.NodePage, NameKey: key, PageBytes: int64(len(content)), CreatedAt: time.Unix(1700000000+int64(n), 0).UTC()}
	if source {
		node.ParentID = c.in.AreaRootID
		c.in.SourceIDs = append(c.in.SourceIDs, id)
	}
	c.in.Nodes = append(c.in.Nodes, node)
	return facts
}
func (c *lintConsumer) asset(n uint64, key string) {
	id := lintID(n)
	c.in.Nodes = append(c.in.Nodes, linking.CompactNode{ID: id, ParentID: c.in.AreaRootID, Kind: linking.NodeAsset, NameKey: key, Linkable: true, CreatedAt: time.Unix(1700000000+int64(n), 0).UTC()})
	c.in.SourceIDs = append(c.in.SourceIDs, id)
	c.sizes.values[id] = int64(700 + n)
}
func (c *lintConsumer) resolve(t *testing.T, n uint64, facts domain.Facts, targets map[string]domain.Resolution) {
	t.Helper()
	seen := map[string]bool{}
	for _, l := range facts.Links {
		if target, ok := targets[l.Target]; ok {
			c.f.resolve(t, lintID(n), target, l.Start)
			seen[l.Target] = true
		}
	}
	for name := range targets {
		if !seen[name] {
			t.Fatalf("production parser did not persist target %q", name)
		}
	}
}
func (c *lintConsumer) legacy(t *testing.T, n uint64) {
	t.Helper()
	lintExec(t, c.f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, lintID(n))
	lintExec(t, c.f, `UPDATE page_links SET property_key=NULL WHERE source_id=$1 AND property_key=''`, lintID(n))
	c.stale++
}

type lintAnswer struct {
	count linking.Count
	items []linking.Item
	more  bool
	stale int64
}

func (c *lintConsumer) answer(t *testing.T, query linking.RuleQuery) map[linking.Rule]lintAnswer {
	t.Helper()
	out := map[linking.Rule]lintAnswer{}
	err := c.f.tx.WithinSnapshot(t.Context(), func(ctx context.Context) error {
		prepared, err := c.rules.Prepare(ctx, c.in, query)
		if err != nil {
			return err
		}
		rules := []linking.Rule{linking.Wanted, linking.Orphans, linking.Ambiguous, linking.InvalidFrontmatter, linking.MissingType, linking.MissingSummary, linking.UndigestedSources, linking.InvalidSources}
		if query.Rule != nil {
			rules = []linking.Rule{*query.Rule}
		}
		for _, rule := range rules {
			count, err := prepared.RuleCount(ctx, rule)
			if err != nil {
				return err
			}
			items, more, err := prepared.RuleItems(ctx, rule)
			if err != nil {
				return err
			}
			out[rule] = lintAnswer{count, items, more, prepared.StalePages()}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("actual module Prepare: %v", err)
	}
	return out
}
func (c *lintConsumer) binding() linking.CursorBinding {
	return linking.CursorBinding{NotebookID: c.in.NotebookID, Explicit: c.in.Explicit, Mode: c.in.EffectiveMode, AreaRootID: c.in.AreaRootID}
}
func lintCursorPosition(t *testing.T, c *lintConsumer, rule linking.Rule, item linking.Item) *linking.Position {
	t.Helper()
	cursor, err := linking.DecodeLintCursor(item.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := cursor.PositionFor(c.binding(), rule)
	if err != nil {
		t.Fatal(err)
	}
	return &pos
}
func lintPendingIDs(t *testing.T, items []linking.Item) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, len(items))
	for i, v := range items {
		if v.NodeID == nil || v.ItemKey != "n:"+v.NodeID.String() {
			t.Fatalf("pending identity: %+v", v)
		}
		ids[i] = *v.NodeID
	}
	return ids
}
func (c *lintConsumer) assertPending(t *testing.T, want []uuid.UUID) {
	t.Helper()
	rule := linking.UndigestedSources
	preview := c.answer(t, linking.RuleQuery{})[rule]
	if preview.count != (linking.Count{Value: int64(len(want))}) || preview.stale != c.stale || !slices.Equal(lintPendingIDs(t, preview.items), want[:min(len(want), 5)]) || preview.more != (len(want) > 5) {
		t.Fatalf("pending summary count/preview/stale: %+v; want %v stale=%d", preview, want, c.stale)
	}
	var got []uuid.UUID
	var after *linking.Position
	for {
		result := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 2})[rule]
		if result.count != preview.count || result.stale != c.stale {
			t.Fatalf("pending count changed across page: %+v", result)
		}
		for _, item := range result.items {
			if item.Kind == linking.NodeAsset && item.Bytes != c.sizes.values[*item.NodeID] {
				t.Fatalf("actual final asset bytes: %+v", item)
			}
		}
		got = append(got, lintPendingIDs(t, result.items)...)
		if !result.more {
			break
		}
		if len(result.items) == 0 || len(got) > len(want) {
			t.Fatal("pending cursor made no progress")
		}
		after = lintCursorPosition(t, c, rule, result.items[len(result.items)-1])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pending exact items: %v; want %v", got, want)
	}
}
func lintHashText(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func lintSourceIdentity(id uuid.UUID, text string, notLink bool) string {
	var header [9]byte
	header[0] = 'L'
	if notLink {
		header[0] = 'J'
	}
	binary.BigEndian.PutUint64(header[1:], uint64(len(text)))
	h := sha256.New()
	_, _ = h.Write(header[:])
	_, _ = h.Write([]byte(text))
	return "s:" + id.String() + ":" + hex.EncodeToString(h.Sum(nil))
}

func testLintEightRules(t *testing.T) {
	c := newLintConsumer(t)
	facts := c.page(t, 10, "wiki", "---\ntype: ' '\nsummary: ' '\nsources: ['[[need-source]]', {x: 1}]\n---\n[[Need]] [[ambig]]", false)
	c.page(t, 20, "target", "---\ntype: note\nsummary: ok\n---\ntarget", false)
	bad := c.page(t, 30, "bad", "---\na: [\n---\n", false)
	if bad.FrontmatterValid {
		t.Fatal("expected a real invalid frontmatter witness")
	}
	c.page(t, 40, "orphan", "---\ntype: note\nsummary: ok\n---\norphan", false)
	c.page(t, 50, "a", "source a", true)
	c.page(t, 51, "b", "source b", true)
	c.asset(52, "file.pdf")
	c.resolve(t, 10, facts, map[string]domain.Resolution{"ambig": {ID: lintID(20), Ambiguous: true}})
	summary := c.answer(t, linking.RuleQuery{})
	expected := map[linking.Rule][]string{
		linking.Wanted:             {"w:" + lintHashText("need"), "w:" + lintHashText("need-source")},
		linking.Orphans:            {"p:" + lintID(10).String(), "p:" + lintID(30).String(), "p:" + lintID(40).String()},
		linking.Ambiguous:          {"a:" + lintID(10).String() + ":" + lintHashText("ambig")},
		linking.InvalidFrontmatter: {"p:" + lintID(30).String()},
		linking.MissingType:        {"p:" + lintID(10).String()}, linking.MissingSummary: {"p:" + lintID(10).String()},
		linking.UndigestedSources: {"n:" + lintID(50).String(), "n:" + lintID(51).String(), "n:" + lintID(52).String()},
		linking.InvalidSources:    {lintSourceIdentity(lintID(10), "need-source", false), lintSourceIdentity(lintID(10), `{"x":1}`, true)},
	}
	slices.Sort(expected[linking.InvalidSources])
	for _, rule := range []linking.Rule{linking.Wanted, linking.Orphans, linking.Ambiguous, linking.InvalidFrontmatter, linking.MissingType, linking.MissingSummary, linking.UndigestedSources, linking.InvalidSources} {
		t.Run(string(rule), func(t *testing.T) {
			want := expected[rule]
			result := summary[rule]
			if result.count != (linking.Count{Value: int64(len(want))}) || result.more || result.stale != 0 {
				t.Fatalf("nonempty %s summary: %+v", rule, result)
			}
			var got []string
			for _, item := range result.items {
				got = append(got, item.ItemKey)
				if item.Rule != rule {
					t.Fatalf("wrong rule item: %+v", item)
				}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("actual %s summary keys=%v want=%v", rule, got, want)
			}
			got = nil
			var after *linking.Position
			for i := 0; i < len(want); i++ {
				page := c.answer(t, linking.RuleQuery{Rule: &rule, After: after, Limit: 1})[rule]
				if page.count != result.count || len(page.items) != 1 || page.more != (i+1 < len(want)) {
					t.Fatalf("nonempty %s page %d: %+v", rule, i, page)
				}
				got = append(got, page.items[0].ItemKey)
				after = lintCursorPosition(t, c, rule, page.items[0])
			}
			if !slices.Equal(got, want) {
				t.Fatalf("actual %s page keys=%v want=%v", rule, got, want)
			}
		})
	}
}

func testLintOriginD(t *testing.T, after bool) {
	c := newLintConsumer(t)
	for i, name := range []string{"digested-a", "pending-b", "digested-c"} {
		c.page(t, uint64(100+i), name, "source", true)
	}
	targets := map[string]domain.Resolution{"digested-a": {ID: lintID(100)}, "pending-b": {ID: lintID(101)}, "digested-c": {ID: lintID(102)}}
	if !after {
		facts := c.page(t, 0, "wiki", "---\n'': '[[pending-b]]'\n---\n[[digested-a]] [[digested-c]]", false)
		c.resolve(t, 0, facts, targets)
	} else {
		facts := c.page(t, 0, "first", strings.Repeat("[[digested-a]] ", 512), false)
		c.resolve(t, 0, facts, map[string]domain.Resolution{"digested-a": targets["digested-a"]})
		facts = c.page(t, 1, "second", "---\n'': '[[pending-b]]'\n---\n[[digested-c]]", false)
		c.resolve(t, 1, facts, map[string]domain.Resolution{"pending-b": targets["pending-b"], "digested-c": targets["digested-c"]})
	}
	var propertyPaths int
	if err := c.f.pool.QueryRow(t.Context(), `SELECT count(*) FROM page_links WHERE property_key=''`).Scan(&propertyPaths); err != nil || propertyPaths != 1 {
		t.Fatalf("real non-NULL empty-property witness: %d %v", propertyPaths, err)
	}
	c.assertPending(t, []uuid.UUID{lintID(101)})
	var probes int
	for _, call := range c.trace.names() {
		if call == "LintHasSourceProperties" {
			probes++
		}
		if call == "LintFirstSourceHeads" || call == "LintFirstSourceValues" || call == "LintSourceSlotHeadsFirst" {
			t.Fatalf("source absence queried property/slot rows: %v", c.trace.names())
		}
	}
	// assertPending executes one summary and one list request for this item.
	if probes != 2 {
		t.Fatalf("body-only consumer needs one absence probe per request: calls=%v", c.trace.names())
	}
	name := "LintBodyReferencesFirst"
	if after {
		name = "LintBodyReferencesAfter"
	}
	if !slices.Contains(c.trace.names(), name) {
		t.Fatalf("actual consumer omitted %s: %v", name, c.trace.names())
	}
}
func testLintOriginF(t *testing.T, kind string) {
	c := newLintConsumer(t)
	body := map[string]string{"wikilink": "[[follow.pdf]]", "link": "[file](follow.pdf)", "embed": "![[follow.pdf]]", "image": "![file](follow.pdf)"}[kind]
	facts := c.page(t, 100, "parent", body, true)
	if len(facts.Links) != 1 || facts.Links[0].Kind != kind {
		t.Fatalf("actual body %s: %+v", kind, facts.Links)
	}
	c.resolve(t, 100, facts, map[string]domain.Resolution{"follow.pdf": {ID: lintID(2000), Asset: true}})
	for _, p := range []struct {
		id                    uint64
		name, content, target string
		asset                 uint64
	}{
		{101, "digested", "[x](digested.pdf)", "digested.pdf", 2001},
		{102, "empty-property", "---\n'': '[[empty.pdf]]'\n---\n", "empty.pdf", 2002},
		{103, "named-property", "---\nnamed: '[[named.pdf]]'\n---\n", "named.pdf", 2003},
		{104, "legacy", "[[legacy.pdf]]", "legacy.pdf", 2004},
	} {
		facts := c.page(t, p.id, p.name, p.content, true)
		c.resolve(t, p.id, facts, map[string]domain.Resolution{p.target: {ID: lintID(p.asset), Asset: true}})
	}
	c.legacy(t, 104)
	for i, key := range []string{"follow.pdf", "digested.pdf", "empty.pdf", "named.pdf", "legacy.pdf", "alone.pdf", "direct.pdf"} {
		c.asset(uint64(2000+i), key)
	}
	facts = c.page(t, 400, "wiki", "[[digested]] [[direct.pdf]]", false)
	c.resolve(t, 400, facts, map[string]domain.Resolution{"digested": {ID: lintID(101)}, "direct.pdf": {ID: lintID(2006), Asset: true}})
	c.assertPending(t, []uuid.UUID{lintID(100), lintID(102), lintID(103), lintID(2002), lintID(2003), lintID(2004), lintID(2005)})
	if !slices.Contains(c.trace.names(), "LintPendingAssets") {
		t.Fatal("pending result did not execute real F")
	}
}

// D's deliberate overbroad emitter relation is a statement-defense test,
// not a legal ReadScope. The version parameter is literally 2, independent
// of a future constant edit; literal v1 includes an actually collapsed path.
func testLintOriginStaleD(t *testing.T, after bool) {
	f := newFixture(t)
	first, old, last := lintID(0), lintID(1), lintID(2)
	cur := originFacts(t, "[[current]]")
	legacy := originFacts(t, "---\n'': '[[empty]]'\n---\n[[old-body]]")
	for _, id := range []uuid.UUID{first, last} {
		f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, cur)
		f.resolve(t, id, domain.Resolution{ID: lintID(100)}, cur.Links[0].Start)
	}
	f.replace(t, app.Page{ID: old, NotebookID: f.eng, Revision: 1}, legacy)
	for i, l := range legacy.Links {
		f.resolve(t, old, domain.Resolution{ID: lintID(uint64(102 + i))}, l.Start)
	}
	lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, old)
	lintExec(t, f, `UPDATE page_links SET property_key=NULL WHERE source_id=$1 AND property_key=''`, old)
	if got := f.rows(t, `SELECT extractor::text,(SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key IS NULL)::text FROM indexed_pages WHERE node_id=$1`, old); !reflect.DeepEqual(got, [][]string{{"1", "2"}}) {
		t.Fatalf("literal-v1 collapsed origin: %v", got)
	}
	type observed struct {
		source uuid.UUID
		start  int32
		target uuid.UUID
		asset  bool
	}
	var got []observed
	add := func(source uuid.UUID, start int32, target *uuid.UUID, asset bool) {
		if target == nil {
			t.Fatal("unexpected NULL target")
		}
		got = append(got, observed{source, start, *target, asset})
	}
	q := gen.New(postgres.Planned(f.pool))
	emitters := []uuid.UUID{first, old, last}
	if !after {
		rows, err := q.LintBodyReferencesFirst(t.Context(), gen.LintBodyReferencesFirstParams{NotebookID: f.eng, Extractor: 2, WikiPageIds: emitters, Take: 512})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			add(r.SourceID, r.RangeStart, r.ResolvedID, r.ResolvedAsset)
		}
	} else {
		rows, err := q.LintBodyReferencesAfter(t.Context(), gen.LintBodyReferencesAfterParams{NotebookID: f.eng, Extractor: 2, WikiPageIds: emitters, AfterSource: first, AfterStart: int32(cur.Links[0].Start), Take: 512})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			add(r.SourceID, r.RangeStart, r.ResolvedID, r.ResolvedAsset)
		}
	}
	want := []observed{{first, int32(cur.Links[0].Start), lintID(100), false}, {last, int32(cur.Links[0].Start), lintID(100), false}}
	if after {
		want = want[1:]
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("D literal-v1 statement defense: %+v; want current-only %+v", got, want)
	}
}
