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
// to a seed. A seed answers its pages' ids, a new one for the zero id, as
// the page module writes them; ids, when set, instead.
type fakeSeeds struct {
	seeds  []app.Seed
	parsed []string
	refuse string
	err    error
	ids    []uuid.UUID
}

func (f *fakeSeeds) Seed(_ context.Context, s app.Seed) ([]uuid.UUID, error) {
	f.seeds = append(f.seeds, s)
	if f.err != nil {
		return nil, f.err
	}
	if f.ids != nil {
		return f.ids, nil
	}
	ids := make([]uuid.UUID, len(s.Pages))
	for i, p := range s.Pages {
		ids[i] = p.ID
		if p.ID == (uuid.UUID{}) {
			ids[i] = uuid.NewV7()
		}
	}
	return ids, nil
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

// The template parses each page once, at its making, and never again at a
// creation; a standard notebook's creation seeds nothing; an LLM Wiki's
// seeds its language's two pages in one unit, raw with the creation's
// source root, raw first, at the creation's time and client, the
// changeset's message in its language, each page with the facts parsed at
// the making, and answers the source root.
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
	if len(seeds.parsed) != 4 {
		t.Errorf("after three creations the pages were parsed %d times, want the 4 of the making alone", len(seeds.parsed))
	}
}

// The template answers the id the seed wrote the source root with, not
// the one it asked for: a seed that wrote raw with another id answers it,
// which the notebook module then refuses as not its own; a seed that
// answers another count of ids than its pages is the server's fault.
func TestTheTemplateAnswersTheSourceRootWritten(t *testing.T) {
	seeds := &fakeSeeds{}
	template := newTemplate(t, seeds)
	c := app.Creation{NotebookID: uuid.NewV7(), Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7(), Locale: domain.LocaleEn,
		Client: "web", At: time.Now()}
	written := uuid.NewV7()
	seeds.ids = []uuid.UUID{written, uuid.NewV7()}
	if root, err := template.NotebookCreated(context.Background(), c); err != nil || root != written {
		t.Errorf("raw written as %s: NotebookCreated() = %s, %v; want %s", written, root, err, written)
	}
	seeds.ids = []uuid.UUID{c.SourceRootID}
	if root, err := template.NotebookCreated(context.Background(), c); err == nil || root != (uuid.UUID{}) {
		t.Errorf("one id for two pages: NotebookCreated() = %s, %v; want the zero id and an error", root, err)
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

// An upgrade's start writes the template's pages the notebook lacks, in
// the template's order, as the upgrade's own unit, its changeset's message
// the template's, their contents parsed at start-up; and answers their ids
// by title key (M11/P1 design 3.14 step 12). A language or a key there is
// no page of, or a seed answering another count, is a defect.
func TestTheTemplateSeedsAnUpgrade(t *testing.T) {
	seeds := &fakeSeeds{}
	template := newTemplate(t, seeds)
	parsed := len(seeds.parsed)
	nb, upgrade, at := uuid.NewV7(), uuid.NewV7(), time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC)
	for _, tt := range []struct {
		locale  domain.Locale
		keys    []string
		titles  []string
		message string
	}{
		{domain.LocaleEn, []string{"raw", "agents"}, []string{"raw", "AGENTS"}, "LLM Wiki template"},
		{domain.LocaleZhCN, []string{"agents"}, []string{"AGENTS"}, "LLM Wiki 模板"},
		{domain.LocaleEn, []string{"raw"}, []string{"raw"}, "LLM Wiki template"},
	} {
		seeds.seeds = nil
		got, err := template.SeedUpgrade(context.Background(), nb, upgrade, tt.locale, "api", at, tt.keys)
		if err != nil || len(seeds.seeds) != 1 {
			t.Fatalf("%s %v: %v, seeds %+v", tt.locale, tt.keys, err, seeds.seeds)
		}
		s := seeds.seeds[0]
		if s.NotebookID != nb || s.Upgrade != upgrade || s.At != at || s.Client != "api" || s.Message != tt.message || len(s.Pages) != len(tt.titles) {
			t.Fatalf("%s %v seeded %+v", tt.locale, tt.keys, s)
		}
		c, _ := domain.ContractOf(domain.ModeLLMWiki)
		for i, p := range s.Pages {
			var want domain.TemplatePage
			for _, page := range c.Templates[tt.locale] {
				if page.Title == tt.titles[i] {
					want = page
				}
			}
			if p.Title != tt.titles[i] || p.ID != (uuid.UUID{}) || p.Content != want.Content || p.Parsed.Facts != "facts of "+want.Content {
				t.Errorf("%s page %d = %+v", tt.locale, i, p)
			}
		}
		if len(got) != len(tt.keys) {
			t.Errorf("%s %v answered %v", tt.locale, tt.keys, got)
		}
		for _, key := range tt.keys {
			if got[key] == (uuid.UUID{}) {
				t.Errorf("%s answered no id for %s: %v", tt.locale, key, got)
			}
		}
	}
	if len(seeds.parsed) != parsed {
		t.Errorf("an upgrade parsed the template again: %d parses", len(seeds.parsed)-parsed)
	}
	seeds.seeds = nil
	for name, call := range map[string]func() error{
		"no such language": func() error {
			_, err := template.SeedUpgrade(context.Background(), nb, upgrade, "fr", "api", at, []string{"raw"})
			return err
		},
		"no such page": func() error {
			_, err := template.SeedUpgrade(context.Background(), nb, upgrade, domain.LocaleEn, "api", at, []string{"raw", "index"})
			return err
		},
	} {
		if call() == nil || len(seeds.seeds) != 0 {
			t.Errorf("%s: seeded %+v", name, seeds.seeds)
		}
	}
	seeds.ids = []uuid.UUID{uuid.NewV7()}
	if _, err := template.SeedUpgrade(context.Background(), nb, upgrade, domain.LocaleEn, "api", at, []string{"raw", "agents"}); err == nil {
		t.Error("one id for two pages answered no error")
	}
}
