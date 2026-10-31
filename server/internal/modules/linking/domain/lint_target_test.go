package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintPresence map[string]uint8

func (p lintPresence) Has(key string, asset bool) bool {
	bit := uint8(1)
	if asset {
		bit = 2
	}
	return p[key]&bit != 0
}

func TestLintWantedCanonical(t *testing.T) {
	for _, tt := range []struct {
		name, raw, kind string
		parents         []string
		llm             bool
		presence        lintPresence
		want            string
		qualified       bool
	}{
		{"paths-a-x-b-x/a", "a/x", "wikilink", nil, false, nil, "a/x", true},
		{"paths-a-x-b-x/b", "b/x", "wikilink", nil, false, nil, "b/x", true},
		{"unicode/Straße", "Ä/Straße.md", "link", nil, false, nil, "ä/strasse", true},
		{"unicode/STRASSE", "A\u0308/STRASSE.MD", "link", nil, false, nil, "ä/strasse", true},
		{"unicode/combining", "E\u0301/İ", "wikilink", nil, false, nil, "é/i\u0307", true},
		{"nested-folder", "../X.md", "wikilink", []string{"a", "b"}, false, nil, "a/x", true},
		{"nested-folder-current", "./X", "wikilink", []string{"a", "b"}, false, nil, "a/b/x", true},
		{"ordinary-no-folder", "a/x", "wikilink", []string{"b"}, false, nil, "a/x", true},
		{"above-root", "../../x", "wikilink", []string{"a"}, false, nil, "", false},
		{"rooted-pair", "/A/X.md", "wikilink", []string{"b"}, false, nil, "a/x", true},
		{"rooted-pair/plain", "A/x", "wikilink", nil, false, nil, "a/x", true},
		{"one-md", "X.md.md", "wikilink", nil, false, nil, "x.md", true},
		{"invalid-double-slash", "a//x", "wikilink", nil, false, nil, "", false},
		{"invalid-parent-inside", "a/../x", "wikilink", nil, false, nil, "", false},
		{"invalid-root-parent", "/../x", "wikilink", nil, false, nil, "", false},
		{"kinds-and-extensions/image", "Node.js.md", "image", nil, false, nil, "", false},
		{"kinds-and-extensions/embed", "a.b.c", "embed", nil, false, nil, "", false},
		{"kinds-and-extensions/dotted-page", "Node.js", "wikilink", nil, false, nil, "node.js", true},
		{"kinds-and-extensions/image-page", "Page.md", "image", nil, false, nil, "page", true},
		{"kinds-and-extensions/hidden", ".hidden", "embed", nil, false, nil, ".hidden", true},
		{"kinds-and-extensions/trailing", "trailing.", "image", nil, false, nil, "trailing.", true},
		{"kinds-and-extensions/plain", "plain", "embed", nil, false, nil, "plain", true},
		{"presence/asset-spelling", "Node.js", "wikilink", nil, false, lintPresence{"node.js": 2}, "", false},
		{"presence/page-spelling", "Node.js.md", "wikilink", nil, false, lintPresence{"node.js": 2}, "node.js", true},
		{"presence/page-wins-md", "Node.js.md", "link", nil, false, lintPresence{"node.js": 3}, "node.js", true},
		{"virtual-scope/standard", "index.md", "wikilink", nil, false, nil, "index", true},
		{"virtual-scope/LLM", "index.md", "wikilink", nil, true, nil, "", false},
		{"virtual-scope/double-md", "/LOG.md.md", "wikilink", nil, true, nil, "", false},
		{"virtual-scope/relative-root", "../index", "wikilink", []string{"a"}, true, nil, "", false},
		{"virtual-scope/relative-nested", "./index", "wikilink", []string{"a"}, true, nil, "a/index", true},
		{"virtual-scope/multisegment", "a/log", "wikilink", nil, true, nil, "a/log", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := domain.WantedTarget(t.Context(), tt.raw, tt.kind, tt.parents, tt.llm, tt.presence)
			if err != nil || ok != tt.qualified || got != tt.want {
				t.Fatalf("wanted %q = %q,%t,%v; want %q,%t", tt.raw, got, ok, err, tt.want, tt.qualified)
			}
		})
	}
	t.Run("presence-actual-node-differential", func(t *testing.T) {
		for _, name := range []string{"plain", ".hidden", "trailing.", "a.b.c", "Node.js", "Node.js.md", "Straße.PDF"} {
			for _, asset := range []bool{false, true} {
				node := domain.Node{Asset: asset, Path: []domain.Step{{Key: shared.TitleKey(name), Name: name}}}
				p := lintPresence{}
				if node.Linkable() {
					p[shared.TitleKey(name)] = 1
					if asset {
						p[shared.TitleKey(name)] = 2
					}
				}
				for _, raw := range []string{name, name + ".md", strings.ToUpper(name)} {
					target, ok := domain.ParseTarget(raw)
					if !ok {
						t.Fatal("valid fixture target rejected")
					}
					if target.ReadsAsAssetWith(p) != target.ReadsAsAsset([]domain.Node{node}) {
						t.Fatalf("presence mismatch name=%q raw=%q asset=%t", name, raw, asset)
					}
				}
			}
		}
	})
	t.Run("cancel-long-normalization", func(t *testing.T) {
		base, cancel := context.WithCancel(t.Context())
		ctx := &lintCancelContext{Context: base, cancel: cancel, left: 10}
		got, ok, err := domain.WantedTarget(ctx, strings.Repeat("Straße", 20000)+"/x", "wikilink", nil, false, lintPresence{})
		if got != "" || ok || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled wanted = length%d,%t,%v", len(got), ok, err)
		}
	})
	t.Run("long-normalization-progress", func(t *testing.T) {
		raw := "a" + strings.Repeat("\u2028", 6000) + "z/x"
		got, ok, err := domain.WantedTarget(t.Context(), raw, "wikilink", nil, false, lintPresence{})
		if err != nil || !ok || got != raw {
			t.Fatalf("legal expanded target = length%d,%t,%v", len(got), ok, err)
		}
	})
	t.Run("parse-grammar-differential", func(t *testing.T) {
		for _, prefix := range []string{"", "/", "./", "../", "./../", "../../"} {
			for _, tail := range []string{"", "A", "A.MD", "A.md.md", "A/B", "A//B", "A/./B", "A/../B", ".md", "../X"} {
				raw := prefix + tail
				parsed, valid := domain.ParseTarget(raw)
				got, ok, err := domain.WantedTarget(t.Context(), raw, "wikilink", []string{"p", "q", "r"}, false, lintPresence{})
				want := ""
				if valid {
					keys := parsed.Keys
					if parsed.Relative {
						keys = append([]string{"p", "q", "r"}[:3-parsed.Up:3-parsed.Up], keys...)
					}
					want = strings.Join(keys, "/")
				}
				if err != nil || ok != valid || got != want {
					t.Fatalf("parse difference %q got %q,%t,%v want %q,%t", raw, got, ok, err, want, valid)
				}
			}
		}
	})
}

type lintCancelContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *lintCancelContext) Err() error {
	if c.left > 0 {
		c.left--
		if c.left == 0 {
			c.cancel()
		}
	}
	return c.Context.Err()
}
