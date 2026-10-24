package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// fakeSeeds records the seeds it gets and the contents it parses, the
// facts of each its content; it refuses to parse refuse, and answers err
// to a seed.
type fakeSeeds struct {
	seeds  []app.Seed
	parsed []string
	refuse string
	err    error
}

func (f *fakeSeeds) Seed(_ context.Context, s app.Seed) error {
	f.seeds = append(f.seeds, s)
	return f.err
}

func (f *fakeSeeds) Parse(content string) (app.Parsed, error) {
	f.parsed = append(f.parsed, content)
	if content == f.refuse {
		return app.Parsed{}, errors.New("the page module refused it")
	}
	return app.Parsed{Facts: "facts of " + content}, nil
}

func newTemplate(t *testing.T, seeds *fakeSeeds) *app.Template {
	t.Helper()
	template, err := app.NewTemplate(seeds)
	if err != nil {
		t.Fatal(err)
	}
	return template
}

// The template parses each page once, at its making; a standard notebook's
// creation seeds nothing; an LLM Wiki's seeds its language's two pages in
// one unit, raw with the creation's source root, raw first, at the
// creation's time and client, the changeset's message in its language,
// and answers the source root.
func TestTheTemplateSeedsAnLLMWiki(t *testing.T) {
	seeds := &fakeSeeds{}
	template := newTemplate(t, seeds)
	if len(seeds.parsed) != 4 {
		t.Errorf("NewTemplate parsed %d pages, want the 4 of both languages", len(seeds.parsed))
	}
	at := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	nb := uuid.NewV7()
	if root, err := template.NotebookCreated(context.Background(), app.Creation{NotebookID: nb, Mode: domain.ModeStandard,
		Locale: domain.LocaleEn, Client: "web", At: at}); err != nil || root != (uuid.UUID{}) || len(seeds.seeds) != 0 {
		t.Errorf("a standard notebook: %s, %v, seeds %+v; want the zero id and nothing", root, err, seeds.seeds)
	}
	for _, tt := range []struct {
		locale        domain.Locale
		client        string
		message, mark string
	}{
		{domain.LocaleEn, "api", "LLM Wiki template", "# How this LLM Wiki is kept"},
		{domain.LocaleZhCN, "web", "LLM Wiki 模板", "# 这本 LLM Wiki 的约定"},
	} {
		seeds.seeds = nil
		sourceRoot := uuid.NewV7()
		root, err := template.NotebookCreated(context.Background(), app.Creation{NotebookID: nb, Mode: domain.ModeLLMWiki,
			SourceRootID: sourceRoot, Locale: tt.locale, Client: tt.client, At: at})
		if err != nil || root != sourceRoot || len(seeds.seeds) != 1 {
			t.Fatalf("%s: NotebookCreated() = %s, %v, seeds %+v; want %s in one seed", tt.locale, root, err, seeds.seeds, sourceRoot)
		}
		s := seeds.seeds[0]
		if s.NotebookID != nb || !s.At.Equal(at) || s.Client != tt.client || s.Message != tt.message || len(s.Pages) != 2 {
			t.Fatalf("%s: seed %+v; want notebook %s at %v from %s, message %q, two pages", tt.locale, s, nb, at, tt.client, tt.message)
		}
		raw, agents := s.Pages[0], s.Pages[1]
		if raw.Title != "raw" || raw.ID != sourceRoot || raw.Parsed.Facts != "facts of "+raw.Content {
			t.Errorf("%s: the first page %+v, want raw, the source root %s, its facts", tt.locale, raw, sourceRoot)
		}
		if agents.Title != "AGENTS" || agents.ID != (uuid.UUID{}) || !strings.HasPrefix(agents.Content, tt.mark) ||
			agents.Parsed.Facts != "facts of "+agents.Content {
			t.Errorf("%s: the second page %+v, want AGENTS of a new id, in its language, its facts", tt.locale, agents)
		}
	}
}

// A seed's error is the creation's, as it is; a page the page module
// refuses to parse, a mode or a language there is no contract of, are the
// server's fault.
func TestTheTemplateFails(t *testing.T) {
	failure := errors.New("the unit failed")
	seeds := &fakeSeeds{err: failure}
	template := newTemplate(t, seeds)
	c := app.Creation{NotebookID: uuid.NewV7(), Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7(), Locale: domain.LocaleEn,
		Client: "web", At: time.Now()}
	if root, err := template.NotebookCreated(context.Background(), c); !errors.Is(err, failure) || root != (uuid.UUID{}) {
		t.Errorf("a failing seed: %s, %v; want the zero id and its error", root, err)
	}
	for name, change := range map[string]func(c *app.Creation){
		"no such mode":     func(c *app.Creation) { c.Mode = "LLM_WIKI" },
		"no such language": func(c *app.Creation) { c.Locale = "fr" },
	} {
		seeds.seeds = nil
		bad := c
		change(&bad)
		if root, err := template.NotebookCreated(context.Background(), bad); err == nil || root != (uuid.UUID{}) || len(seeds.seeds) != 0 {
			t.Errorf("%s: %s, %v, seeds %+v; want an error, nothing seeded", name, root, err, seeds.seeds)
		}
	}
	c2, _ := domain.ContractOf(domain.ModeLLMWiki)
	if _, err := app.NewTemplate(&fakeSeeds{refuse: c2.Templates[domain.LocaleZhCN][1].Content}); err == nil {
		t.Error("NewTemplate with a page refused = nil error, want the server's fault")
	}
}
