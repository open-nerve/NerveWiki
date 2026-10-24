// Package wiki is the module of the LLM Wiki mode (M10 design 4.1): the
// modes' contracts and the LLM Wiki's template. Its root is what bootstrap
// sees: NewTemplate, the notebook module's seeder of a creation, over the
// page module's template writes; TemplatePages, the template as the
// contract holds it; Actions for the composition's checks. P1 has no
// tables, routes or tools of its own.
package wiki

import (
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Template is the creation's seeder of the LLM Wiki mode (M10 design 4.4).
type Template = app.Template

// Seeds, Seed, SeedPage, Parsed and Creation are its port and values,
// which the composition root fills from the page and notebook modules.
type (
	Seeds    = app.Seeds
	Seed     = app.Seed
	SeedPage = app.SeedPage
	Parsed   = app.Parsed
	Creation = app.Creation
	Mode     = domain.Mode
	Locale   = domain.Locale
	// TemplatePage is a page a mode's creation writes: its title, its
	// content, and whether it is the source root.
	TemplatePage = domain.TemplatePage
)

// The mode and the languages the composition root names: an alias carries
// no constants, and bootstrap may not import wiki/domain.
const (
	ModeLLMWiki = domain.ModeLLMWiki
	LocaleEn    = domain.LocaleEn
	LocaleZhCN  = domain.LocaleZhCN
)

// NewTemplate is the seeder over seeds, its templates parsed now.
func NewTemplate(seeds Seeds) (*Template, error) { return app.NewTemplate(seeds) }

// TemplatePages are the pages mode's creation writes in locale l, in
// order, as the contract holds them: none for a standard notebook, a mode
// or language there is none of. The composition root's tests compare a
// created notebook with them (M10/P1 design 3.16).
func TemplatePages(mode Mode, l Locale) []TemplatePage {
	c, _ := domain.ContractOf(mode)
	return slices.Clone(c.Templates[l])
}

// Actions are the wiki module's actions in the rule table: none in P1;
// P3 adds lint.read (M10 design 4.17).
func Actions() []shared.Action { return nil }
