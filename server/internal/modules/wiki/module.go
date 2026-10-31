// Package wiki is the module of the LLM Wiki mode (M10 design 4.1): the
// modes' contracts, the LLM Wiki's template and its internal lint runtime.
// Bootstrap constructs the template seeder separately from the runtime.
// Register exposes the same runtime through the module's HTTP routes.
package wiki

import (
	"slices"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
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
	ModeStandard = domain.ModeStandard
	ModeLLMWiki  = domain.ModeLLMWiki
	LocaleEn     = domain.LocaleEn
	LocaleZhCN   = domain.LocaleZhCN
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

// Actions are the wiki module's actions in the rule table.
func Actions() []shared.Action { return domain.Actions() }

// Register mounts the HTTP routes using this module's shared lint runtime.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m)
}
