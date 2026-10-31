//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// assertLintPayloadRules consumes the page already written by the real parser
// fixture. Only the module-boundary page/root metadata is synthetic. wanted is
// the handwritten a + repeated U+2028 + z/x witness: its canonical key equals
// the full raw target. invalidText is a complete L target or one stable J item,
// never the whole array containing two identical J items. nil wanted means no
// wanted group. Expected identities do not use the domain's constructors.
func assertLintPayloadRules(t *testing.T, f fixture, page uuid.UUID, sourceBytes int, wanted *string, invalidText string, notLink bool) {
	t.Helper()
	const maxContentBytes = 5 * 1024 * 1024
	if sourceBytes <= 0 || sourceBytes >= maxContentBytes || invalidText == "" || !utf8.ValidString(invalidText) {
		t.Fatal("payload consumer needs a legal source and a complete handwritten invalid-source oracle")
	}
	if wanted != nil && (len(*wanted) <= maxContentBytes || invalidText != *wanted || notLink) {
		t.Fatal("escaped-target consumer needs the complete >5MiB target as both wanted and invalid L")
	}
	root := uuid.UUID{15: 1}
	if root == page {
		root[15] = 2
	}
	in := linking.ReadInput{NotebookID: f.eng, EffectiveMode: linking.ModeLLM, Explicit: true, AreaRootID: &root,
		Nodes: []linking.CompactNode{
			{ID: root, Kind: linking.NodePage, NameKey: "raw"},
			{ID: page, Kind: linking.NodePage, NameKey: "payload-wiki", PageBytes: int64(sourceBytes)},
		}}
	binding := linking.CursorBinding{NotebookID: f.eng, Explicit: true, Mode: linking.ModeLLM, AreaRootID: &root}
	text, cut := lintPayloadDisplay(invalidText)
	var header [9]byte
	header[0] = 'L'
	if notLink {
		header[0] = 'J'
	}
	binary.BigEndian.PutUint64(header[1:], uint64(len(invalidText)))
	digest := sha256.New()
	_, _ = digest.Write(header[:])
	_, _ = io.WriteString(digest, invalidText)
	invalidHash := fmt.Sprintf("%x", digest.Sum(nil))
	invalid := linking.Item{Rule: linking.InvalidSources, ItemKey: "s:" + page.String() + ":" + invalidHash,
		PageID: &page, Target: text, TargetCut: cut, NotLink: notLink}
	invalidPosition := linking.Position{ID: &page, Hash: invalidHash}
	var wantedItems []linking.Item
	var wantedPosition linking.Position
	if wanted != nil {
		digest.Reset()
		_, _ = io.WriteString(digest, *wanted)
		hash := fmt.Sprintf("%x", digest.Sum(nil))
		target, targetCut := lintPayloadDisplay(*wanted)
		wantedItems = []linking.Item{{Rule: linking.Wanted, ItemKey: "w:" + hash, Target: target, TargetCut: targetCut, Pages: 1}}
		wantedPosition = linking.Position{Pages: 1, Prefix: []byte((*wanted)[:256]), Hash: hash}
	}
	pageItem := func(rule linking.Rule) linking.Item {
		return linking.Item{Rule: rule, ItemKey: "p:" + page.String(), PageID: &page}
	}
	// The only indexed live page is current, valid, has no inbound link and
	// has sources but neither type nor summary. The empty area has no sources.
	expected := []struct {
		rule  linking.Rule
		items []linking.Item
		pos   linking.Position
	}{
		{linking.Wanted, wantedItems, wantedPosition},
		{linking.Orphans, []linking.Item{pageItem(linking.Orphans)}, linking.Position{ID: &page}},
		{linking.Ambiguous, nil, linking.Position{}},
		{linking.InvalidFrontmatter, nil, linking.Position{}},
		{linking.MissingType, []linking.Item{pageItem(linking.MissingType)}, linking.Position{ID: &page}},
		{linking.MissingSummary, []linking.Item{pageItem(linking.MissingSummary)}, linking.Position{ID: &page}},
		{linking.UndigestedSources, nil, linking.Position{}},
		{linking.InvalidSources, []linking.Item{invalid}, invalidPosition},
	}
	rules := linking.NewLintRules(f.pool, maxContentBytes, nil)
	err := postgres.NewTxManager(f.pool, time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
		summary, err := rules.Prepare(ctx, in, linking.RuleQuery{})
		if err != nil {
			return fmt.Errorf("payload summary consumer: %w", err)
		}
		if summary.StalePages() != 0 {
			t.Fatalf("payload current fixture became stale: %d", summary.StalePages())
		}
		for _, want := range expected {
			lintPayloadProjection(t, ctx, summary, binding, want.rule, want.items, want.pos, int64(len(want.items)))
			if want.rule != linking.Wanted && want.rule != linking.InvalidSources {
				continue
			}
			list, err := rules.Prepare(ctx, in, linking.RuleQuery{Rule: &want.rule, Limit: 1})
			if err != nil {
				return fmt.Errorf("payload %s list consumer: %w", want.rule, err)
			}
			cursor := lintPayloadProjection(t, ctx, list, binding, want.rule, want.items, want.pos, int64(len(want.items)))
			if len(want.items) == 0 {
				continue
			}
			decoded, err := linking.DecodeLintCursor(cursor)
			if err != nil {
				return err
			}
			after, err := decoded.PositionFor(binding, want.rule)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(after, want.pos) {
				t.Fatalf("payload %s decoded identity=%+v, want %+v", want.rule, after, want.pos)
			}
			end, err := rules.Prepare(ctx, in, linking.RuleQuery{Rule: &want.rule, Limit: 1, After: &after})
			if err != nil {
				return fmt.Errorf("payload %s continuation consumer: %w", want.rule, err)
			}
			// The cursor changes only the item page; count still covers the full
			// qualifying set, even after the single real item has been returned.
			lintPayloadProjection(t, ctx, end, binding, want.rule, nil, linking.Position{}, 1)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real payload rules: source_bytes=%d invalid_full_bytes=%d wanted_groups=%d invalid_sources=1 not_link=%t; summary/list identities and continuation exhausted",
		sourceBytes, len(invalidText), len(wantedItems), notLink)
}

func lintPayloadDisplay(full string) (string, bool) {
	end := 0
	for offset, r := range full {
		next := offset + utf8.RuneLen(r)
		if next > 256 {
			break
		}
		end = next
	}
	return full[:end], end < len(full)
}

func lintPayloadProjection(t *testing.T, ctx context.Context, prepared *linking.PreparedRules, binding linking.CursorBinding,
	rule linking.Rule, expected []linking.Item, position linking.Position, count int64) string {
	t.Helper()
	gotCount, err := prepared.RuleCount(ctx, rule)
	if err != nil || gotCount != (linking.Count{Value: count}) {
		t.Fatalf("payload %s count=%+v error=%v, want uncapped %d", rule, gotCount, err, count)
	}
	items, more, err := prepared.RuleItems(ctx, rule)
	if err != nil || more || len(items) != len(expected) {
		t.Fatalf("payload %s items=%d more=%t error=%v, want %d and terminal page", rule, len(items), more, err, len(expected))
	}
	if len(items) == 0 {
		return ""
	}
	item := items[0]
	cursor := item.Cursor
	item.Cursor = ""
	if !reflect.DeepEqual(item, expected[0]) {
		t.Fatalf("payload %s full identity/projection=%+v, want %+v", rule, item, expected[0])
	}
	// Check the wire identity independently of the production encoder/decoder,
	// including byte-prefix ordering data and the explicit notebook/root scope.
	wireBytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || cursor == "" || len(cursor) > 2048 {
		t.Fatalf("payload %s cursor envelope: %v", rule, err)
	}
	var wire map[string]any
	if err := json.Unmarshal(wireBytes, &wire); err != nil {
		t.Fatal(err)
	}
	wantPayload := map[string]any{"v": float64(1), "r": string(rule), "n": binding.NotebookID.String(),
		"s": "E:llm_wiki:" + binding.AreaRootID.String()}
	if rule == linking.Wanted {
		wantPayload["c"], wantPayload["k"] = float64(1), base64.RawURLEncoding.EncodeToString(position.Prefix)
	} else {
		wantPayload["i"] = position.ID.String()
	}
	if position.Hash != "" {
		wantPayload["h"] = position.Hash
	}
	wantWire := map[string]any{"v": float64(1), "p": wantPayload}
	if !reflect.DeepEqual(wire, wantWire) {
		t.Fatalf("payload %s cursor identity=%v, want %v", rule, wire, wantWire)
	}
	return cursor
}
