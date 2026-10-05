package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// appMarkdown is the Markdown serve builds, with the extensions it
// registers, a reading view's links leading where resolve tells.
func appMarkdown(t *testing.T, resolve obsidian.Resolve) (*markdown.Markdown, []markdown.Extension) {
	t.Helper()
	exts := markdownExtensions(resolve)
	md, err := markdown.New(exts)
	if err != nil {
		t.Fatal(err)
	}
	return md, exts
}

// everyLink resolves every link, to one page: the largest markup of a
// reading view's links but for the targets of those that resolve to none.
func everyLink(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]uuid.UUID, error) {
	to := make(map[int]uuid.UUID, len(links))
	for _, l := range links {
		to[l.Range.Start] = uuid.Max()
	}
	return to, nil
}

// The application's Markdown renders every fixture and every generated
// input to HTML that passes the check with its extensions' markup, and
// is no larger than CheckSize allows, every link resolved or none (M4/P3
// design 3.10, M6/P3 design 6.8).
func TestTheAppsMarkdownRendersCheckedHTML(t *testing.T) {
	inputs := map[string]string{"ordinary": markdowntest.Normal(64 << 10)}
	for _, f := range markdowntest.Fixtures(t) {
		inputs[f.Name] = string(f.Content)
	}
	for _, in := range markdowntest.Pathological() {
		inputs[in.Name] = in.Make(16 << 10)
	}
	for _, in := range markdowntest.Amplifying() {
		inputs[in.Name] = in.Make(markdowntest.AmplifyingSize)
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for _, resolve := range []obsidian.Resolve{everyLink, nil} {
		md, exts := appMarkdown(t, resolve)
		for name, content := range inputs {
			html, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if err := markdowntest.CheckHTML(html, exts...); err != nil {
				t.Errorf("%s: %v", name, err)
			}
			if err := markdowntest.CheckSize([]byte(content), html); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

// The links to pages, every one resolved or none, are within CheckSize at a
// size where its bound, not its headroom, decides (M6/P3B review L3): the
// state each carries, and an image its address, each time it is written
// or a reference used.
func TestTheAppsLinksAreWithinTheirBound(t *testing.T) {
	const n = 512 << 10
	inputs := map[string]string{
		"images of a short address":           "[x]: p#&\n\n" + strings.Repeat("![x] ", n/5),
		"images of a short address, unspaced": "[x]: p#&\n\n" + strings.Repeat("![x]", n/4),
		"images of an address of '&'":         "[x]: &&&&\n\n" + strings.Repeat("![x] ", n/5),
		"links of a short address":            "[x]: p#b\n\n" + strings.Repeat("[x] ", n/4),
		"wikilinks with anchors":              strings.Repeat("[[a#b]]", n/7),
		"embeds":                              strings.Repeat("![[p]]", n/6),
		"wikilinks of a long anchor":          strings.Repeat("[[a#"+strings.Repeat("Ⱥ", 64)+"]]", n/134),
		// In the property table, below the YAML's limit of values (M6/P6 design 4).
		"property links of a display of '&'": "---\na: [" + strings.Repeat("'[[&|"+strings.Repeat("&", n/9000-12)+"]]',", 9000) + "]\n---\n",
		"property links":                     "---\na: [" + strings.Repeat("'[["+strings.Repeat("a", n/9000-8)+"]]',", 9000) + "]\n---\n",
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for _, resolve := range []obsidian.Resolve{everyLink, nil} {
		md, _ := appMarkdown(t, resolve)
		for name, content := range inputs {
			html, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			t.Logf("%s, resolved %t: %.1f times", name, resolve != nil, float64(len(html))/float64(len(content)))
			if err := markdowntest.CheckSize([]byte(content), html); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

// The facts of the application's parse keep none of its tree (M6 design
// 4.7): a write keeps them through its unit, its extensions' results
// among them.
func TestTheAppsFactsOutliveTheTree(t *testing.T) {
	markdowntest.CheckFacts(t, markdownExtensions(nil))
}

// The server's parse budget is the configuration's (M6 design 4.7): all but
// 8 KiB of it held, 8 KiB are free, and a byte more is busy once the
// configured wait has passed.
func TestTheServersParseBudgetIsTheConfigurations(t *testing.T) {
	cfg := config.Config{Page: config.PageConfig{ParseBudgetBytes: 64 << 10, ParseMaxWait: 50 * time.Millisecond}}
	_, budget, err := parsing(cfg, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Takes past the least a take holds: the budget's last 8 KiB fit, and a
	// byte more does not.
	most, err := budget.Take(ctx, 56<<10)
	if err != nil {
		t.Fatal(err)
	}
	defer most.Release()
	last, err := budget.Take(ctx, 8<<10)
	if err != nil {
		t.Fatalf("the budget's last 8 KiB: %v", err)
	}
	last.Release()
	start := time.Now()
	if _, err := budget.Take(ctx, 8<<10+1); !errors.Is(err, markdown.ErrBusy) || time.Since(start) < cfg.Page.ParseMaxWait ||
		time.Since(start) > time.Second {
		t.Errorf("a byte beyond the budget = %v after %s, want busy after %s", err, time.Since(start), cfg.Page.ParseMaxWait)
	}
}

// The application's Markdown costs about the size of what it parses, with
// its extensions' parsers, every link resolved: each must be linear too
// (M4/P3 design 3.4).
func TestTheAppsMarkdownCostsAboutItsSize(t *testing.T) {
	md, _ := appMarkdown(t, everyLink)
	markdowntest.CheckCosts(t, md)
}
