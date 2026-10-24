package wiki_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// recordedSeeds records the seeds it gets.
type recordedSeeds struct{ seeds []wiki.Seed }

func (r *recordedSeeds) Seed(_ context.Context, s wiki.Seed) error {
	r.seeds = append(r.seeds, s)
	return nil
}

func (r *recordedSeeds) Parse(content string) (wiki.Parsed, error) {
	return wiki.Parsed{Facts: len(content)}, nil
}

// The module root's seeder reaches its seeds (v0.1 design 13.1 item 21):
// an LLM Wiki's creation seeds the template as TemplatePages has it, and
// answers its source root; a standard one seeds nothing.
func TestTheTemplateReachesItsSeeds(t *testing.T) {
	seeds := &recordedSeeds{}
	template, err := wiki.NewTemplate(seeds)
	if err != nil {
		t.Fatal(err)
	}
	root := uuid.NewV7()
	got, err := template.NotebookCreated(context.Background(), wiki.Creation{NotebookID: uuid.NewV7(), Mode: wiki.ModeLLMWiki,
		SourceRootID: root, Locale: wiki.LocaleZhCN, Client: "web", At: time.Now()})
	pages := wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleZhCN)
	if err != nil || got != root || len(seeds.seeds) != 1 || len(seeds.seeds[0].Pages) != len(pages) {
		t.Fatalf("NotebookCreated() = %s, %v, seeds %+v; want %s, one seed of %d pages", got, err, seeds.seeds, root, len(pages))
	}
	for i, p := range seeds.seeds[0].Pages {
		if p.Title != pages[i].Title || p.Content != pages[i].Content || (p.ID == root) != pages[i].SourceRoot {
			t.Errorf("page %d seeded %+v, want %+v", i, p, pages[i])
		}
	}
	if got, err := template.NotebookCreated(context.Background(), wiki.Creation{NotebookID: uuid.NewV7(), Mode: "standard",
		Locale: wiki.LocaleEn, Client: "web", At: time.Now()}); err != nil || got != (uuid.UUID{}) || len(seeds.seeds) != 1 {
		t.Errorf("a standard notebook: %s, %v, %d seeds; want nothing", got, err, len(seeds.seeds))
	}
	// TemplatePages answers a copy: the caller cannot change the contract.
	pages[0].Content = "changed"
	if again := wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleZhCN); again[0].Content == "changed" ||
		len(wiki.TemplatePages("standard", wiki.LocaleEn)) != 0 {
		t.Error("TemplatePages answered the contract's own pages, or a standard notebook's")
	}
}
