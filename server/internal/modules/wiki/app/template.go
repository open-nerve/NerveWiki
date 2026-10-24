package app

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// Creation is a notebook being created, as the composition root converts
// notebook.NotebookCreation.
type Creation struct {
	NotebookID   uuid.UUID
	Mode         domain.Mode
	SourceRootID uuid.UUID
	Locale       domain.Locale
	Client       string
	At           time.Time
}

// Template is the creation's seeder (M10 design 4.4): for an LLM Wiki it
// writes its mode's template in its language, the source root with the
// creation's id, and answers that id; for a standard notebook it writes
// nothing and answers the zero id.
type Template struct {
	seeds  Seeds
	parsed map[domain.Locale][]templatePage
}

// templatePage is a page of the template with its facts, parsed once.
type templatePage struct {
	domain.TemplatePage
	parsed Parsed
}

// NewTemplate parses the templates once, at start-up (v0.1 design 13.3
// item 3's exception): an error is the server's fault, a template the
// page module refuses.
func NewTemplate(seeds Seeds) (*Template, error) {
	contract, _ := domain.ContractOf(domain.ModeLLMWiki)
	parsed := make(map[domain.Locale][]templatePage, len(contract.Templates))
	for l, pages := range contract.Templates {
		for _, p := range pages {
			facts, err := seeds.Parse(p.Content)
			if err != nil {
				return nil, fmt.Errorf("wiki: the %s template's page %s: %w", l, p.Title, err)
			}
			parsed[l] = append(parsed[l], templatePage{TemplatePage: p, parsed: facts})
		}
	}
	return &Template{seeds: seeds, parsed: parsed}, nil
}

// NotebookCreated writes c's template: the zero id and nothing for a
// standard notebook; for an LLM Wiki its pages in one unit, the source
// root's id the one c decided, the changeset's message the template's in
// its language, "LLM Wiki template" or "LLM Wiki 模板". A mode or a
// language there is no contract of is a defect: the notebook module
// checked both, and only a conversion of the composition root's would
// answer one.
func (t *Template) NotebookCreated(ctx context.Context, c Creation) (uuid.UUID, error) {
	contract, ok := domain.ContractOf(c.Mode)
	switch {
	case !ok:
		return uuid.UUID{}, fmt.Errorf("wiki: notebook %s of mode %q, which has no contract", c.NotebookID, c.Mode)
	case len(contract.Templates) == 0:
		return uuid.UUID{}, nil
	}
	pages, ok := t.parsed[c.Locale]
	if !ok {
		return uuid.UUID{}, fmt.Errorf("wiki: notebook %s of language %q, which has no template", c.NotebookID, c.Locale)
	}
	seed := Seed{NotebookID: c.NotebookID, At: c.At, Client: c.Client, Message: domain.TemplateMessage(c.Locale),
		Pages: make([]SeedPage, len(pages))}
	for i, p := range pages {
		seed.Pages[i] = SeedPage{Title: p.Title, Content: p.Content, Parsed: p.parsed}
		if p.SourceRoot {
			seed.Pages[i].ID = c.SourceRootID
		}
	}
	if err := t.seeds.Seed(ctx, seed); err != nil {
		return uuid.UUID{}, err
	}
	return c.SourceRootID, nil
}
