package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// appMarkdown is the Markdown serve builds, with the extensions it
// registers.
func appMarkdown(t *testing.T) (*markdown.Markdown, []markdown.Extension) {
	t.Helper()
	exts := markdownExtensions()
	md, err := markdown.New(exts)
	if err != nil {
		t.Fatal(err)
	}
	return md, exts
}

// The application's Markdown renders every fixture and every generated
// input to HTML that passes the check with its extensions' markup (M4/P3
// design 3.10).
func TestTheAppsMarkdownRendersCheckedHTML(t *testing.T) {
	md, exts := appMarkdown(t)
	inputs := map[string]string{"ordinary": markdowntest.Normal(64 << 10)}
	for _, f := range markdowntest.Fixtures(t) {
		inputs[f.Name] = string(f.Content)
	}
	for _, in := range markdowntest.Pathological() {
		inputs[in.Name] = in.Make(16 << 10)
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for name, content := range inputs {
		html, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := markdowntest.CheckHTML(html, exts...); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The facts of the application's parse keep none of its tree (M6 design
// 4.7): a write keeps them through its unit, its extensions' results
// among them.
func TestTheAppsFactsOutliveTheTree(t *testing.T) {
	markdowntest.CheckFacts(t, markdownExtensions())
}

// The server's parse budget is the configuration's (M6 design 4.7): all but
// 8 KiB of it held, 8 KiB are free, and a byte more is busy once the
// configured wait has passed.
func TestTheServersParseBudgetIsTheConfigurations(t *testing.T) {
	cfg := config.Config{Page: config.PageConfig{ParseBudgetBytes: 64 << 10, ParseMaxWait: 50 * time.Millisecond}}
	_, budget, err := parsing(cfg, slog.New(slog.DiscardHandler))
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
// its extensions' parsers: each must be linear too (M4/P3 design 3.4).
func TestTheAppsMarkdownCostsAboutItsSize(t *testing.T) {
	md, _ := appMarkdown(t)
	markdowntest.CheckCosts(t, md)
}
