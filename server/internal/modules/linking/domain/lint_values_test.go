package domain_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestLintItems(t *testing.T) {
	t.Run("rules-count-and-page-zero", func(t *testing.T) {
		want := []domain.Rule{domain.Wanted, domain.Orphans, domain.Ambiguous, domain.InvalidFrontmatter, domain.MissingType, domain.MissingSummary, domain.UndigestedSources, domain.InvalidSources}
		if !reflect.DeepEqual(domain.ApplicableRules(domain.ModeLLM), want) || !reflect.DeepEqual(domain.ApplicableRules(domain.ModeStandard), want[:4]) || len(domain.ApplicableRules(0)) != 0 {
			t.Fatal("rule applicability/order differs")
		}
		copy := domain.ApplicableRules(domain.ModeLLM)
		copy[0] = domain.Orphans
		if domain.ApplicableRules(domain.ModeLLM)[0] != domain.Wanted {
			t.Fatal("rules leaked mutable shared state")
		}
		for _, tt := range []struct {
			n, value int64
			cap      bool
		}{{0, 0, false}, {9999, 9999, false}, {10000, 10000, true}, {10001, 10000, true}} {
			if got := domain.CapCount(tt.n); got.Value != tt.value || got.Capped != tt.cap {
				t.Fatalf("CapCount(%d)=%+v", tt.n, got)
			}
		}
		item, pos, err := domain.PageItem(domain.Orphans, uuid.UUID{})
		if err != nil || item.ItemKey != "p:00000000-0000-0000-0000-000000000000" || item.PageID == nil || pos.ID == nil || *pos.ID != (uuid.UUID{}) || pos.Begin {
			t.Fatalf("legal zero page = %+v %+v %v", item, pos, err)
		}
		for _, rule := range []domain.Rule{domain.Wanted, domain.Ambiguous, domain.InvalidSources, domain.UndigestedSources, "bad"} {
			if _, _, err := domain.PageItem(rule, uuid.UUID{}); err == nil {
				t.Fatalf("PageItem accepted nonpage rule %q", rule)
			}
		}
	})
	t.Run("identity-tags-and-length-vectors", func(t *testing.T) {
		// These byte vectors were calculated independently: tag, uint64 BE
		// UTF-8 byte length, then text. The accented string is two bytes.
		// L:"x" = 4c0000000000000003227822; J:"x" differs at 4a.
		// L:é = 4c0000000000000002c3a9.
		for _, tt := range []struct {
			text    string
			notLink bool
			hash    string
		}{
			{`"x"`, false, "4595ed6542fd217be2169d15794e2befbf7bcfd5c0ab32a11e039014cf6daa4d"},
			{`"x"`, true, "086a15cc59a8e66c2f06c7773a187338950c9a6d17ab91a59cdc7f385d5f5cd6"},
			{"é", false, "aef57cd696eae99480006d26fbf98b0adc69b8787f1d41aa136202374a253b58"},
			{`{"a":1e+30,"b":[true,null]}`, true, "801334a113ab5f551351290d175249c13905eed35e819970198647f83ac23074"},
		} {
			item, pos, err := domain.InvalidSourceItem(uuid.UUID{}, tt.text, tt.notLink)
			if err != nil || item.ItemKey != "s:00000000-0000-0000-0000-000000000000:"+tt.hash || pos.Hash != tt.hash || item.NotLink != tt.notLink {
				t.Fatalf("source identity text=%q J=%t = %+v %+v %v", tt.text, tt.notLink, item, pos, err)
			}
		}
		w, wp, err := domain.WantedItem("a/x", "/A/X.md", 12001)
		if err != nil || w.ItemKey != "w:1653a06856ec14bc20b4e1dcc951d601eda9f17d7af17cfd4a433cfe93a29b04" || w.Pages != 12001 || wp.Pages != 12001 || string(wp.Prefix) != "a/x" {
			t.Fatalf("wanted identity/pages = %+v %+v %v", w, wp, err)
		}
		a, ap, err := domain.AmbiguousItem(uuid.UUID{}, "raw/x")
		if err != nil || a.ItemKey != "a:00000000-0000-0000-0000-000000000000:843290ec038681b39e78d931a28ba769601e9b5a1b409e8a78ef570dc7dad1a5" || ap.Hash != "843290ec038681b39e78d931a28ba769601e9b5a1b409e8a78ef570dc7dad1a5" {
			t.Fatalf("ambiguous identity = %+v %+v %v", a, ap, err)
		}
	})
	t.Run("long-prefix-distinct-groups", func(t *testing.T) {
		prefix := strings.Repeat("a", 255) + "中"
		a, pa, err := domain.WantedItem(prefix+"a", prefix+"a", 2)
		if err != nil {
			t.Fatal(err)
		}
		b, pb, err := domain.WantedItem(prefix+"b", prefix+"b", 2)
		if err != nil {
			t.Fatal(err)
		}
		if a.Target != strings.Repeat("a", 255) || a.Target != b.Target || !a.TargetCut || !b.TargetCut || !utf8.ValidString(a.Target) || len(pa.Prefix) != 256 || pa.Prefix[255] != 0xe4 || !bytes.Equal(pa.Prefix, pb.Prefix) || a.ItemKey == b.ItemKey || pa.Hash == pb.Hash {
			t.Fatalf("full wanted identity was collapsed/truncated: %+v %+v %+v %+v", a, b, pa, pb)
		}
		for _, constructor := range []func(uuid.UUID, string) (domain.Item, domain.Position, error){domain.AmbiguousItem, func(id uuid.UUID, s string) (domain.Item, domain.Position, error) {
			return domain.InvalidSourceItem(id, s, false)
		}, func(id uuid.UUID, s string) (domain.Item, domain.Position, error) {
			return domain.InvalidSourceItem(id, s, true)
		}} {
			a, pa, err = constructor(uuid.UUID{}, prefix+"a")
			if err != nil {
				t.Fatal(err)
			}
			b, pb, err = constructor(uuid.UUID{}, prefix+"b")
			if err != nil {
				t.Fatal(err)
			}
			if a.Target != b.Target || a.ItemKey == b.ItemKey || pa.Hash == pb.Hash || !a.TargetCut {
				t.Fatal("long item identity used its display")
			}
		}
	})
	t.Run("pending-UTC-and-order", func(t *testing.T) {
		at := time.Date(2026, 10, 30, 8, 9, 10, 123456789, time.FixedZone("east", 8*3600))
		item, pos, err := domain.PendingItem(uuid.UUID{}, domain.NodeAsset, 123, at)
		if err != nil || item.ItemKey != "n:00000000-0000-0000-0000-000000000000" || item.Bytes != 123 || item.Kind != domain.NodeAsset || item.CreatedAt.Format(time.RFC3339Nano) != "2026-10-30T00:09:10.123456789Z" || !pos.CreatedAt.Equal(at) {
			t.Fatalf("pending = %+v %+v %v", item, pos, err)
		}
		if _, _, err = domain.PendingItem(uuid.UUID{}, domain.NodePage, -1, at); err == nil {
			t.Fatal("negative bytes accepted")
		}
		if _, _, err = domain.PendingItem(uuid.UUID{}, "bad", 1, at); err == nil {
			t.Fatal("bad kind accepted")
		}
	})
}

func TestLintPositionOrder(t *testing.T) {
	zero, one := uuid.UUID{}, uuid.MustParse("00000000-0000-0000-0000-000000000001")
	for _, tt := range []struct {
		rule domain.Rule
		a, b domain.Position
	}{
		{domain.Orphans, domain.Position{Begin: true}, domain.Position{ID: &zero}},
		{domain.Orphans, domain.Position{ID: &zero}, domain.Position{ID: &one}},
		{domain.Wanted, domain.Position{Pages: 4, Prefix: []byte("z"), Hash: "z"}, domain.Position{Pages: 3, Prefix: []byte("a"), Hash: "a"}},
		// Raw ff sorts after 00 even though base64 '_w' sorts before 'AA'.
		{domain.Wanted, domain.Position{Pages: 3, Prefix: []byte{0}, Hash: "z"}, domain.Position{Pages: 3, Prefix: []byte{255}, Hash: "a"}},
		{domain.Wanted, domain.Position{Pages: 3, Prefix: []byte("a"), Hash: "a"}, domain.Position{Pages: 3, Prefix: []byte("a"), Hash: "b"}},
		{domain.InvalidSources, domain.Position{ID: &zero, Hash: "z"}, domain.Position{ID: &one, Hash: "a"}},
		{domain.Ambiguous, domain.Position{ID: &zero, Hash: "a"}, domain.Position{ID: &zero, Hash: "b"}},
		{domain.UndigestedSources, domain.Position{ID: &one, CreatedAt: time.Unix(1, 0)}, domain.Position{ID: &zero, CreatedAt: time.Unix(2, 0)}},
	} {
		if domain.ComparePosition(tt.rule, tt.a, tt.b) >= 0 || domain.ComparePosition(tt.rule, tt.b, tt.a) <= 0 || domain.ComparePosition(tt.rule, tt.a, tt.a) != 0 {
			t.Fatalf("wrong %s order %+v %+v", tt.rule, tt.a, tt.b)
		}
	}
}
