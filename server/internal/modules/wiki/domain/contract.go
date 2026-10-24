// Package domain holds the wiki module's rules: the modes' contracts and
// the LLM Wiki's template, in each language.
package domain

// Mode is a notebook's mode, as the wiki module knows it: the composition
// root converts the notebook module's, field by field (v0.1 design 13.1
// item 11).
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// Locale is a template's language: en, the default, or zh-CN.
type Locale string

const (
	LocaleEn   Locale = "en"
	LocaleZhCN Locale = "zh-CN"
)

// Locales are the languages a template is written in.
func Locales() []Locale {
	return []Locale{LocaleEn, LocaleZhCN}
}

// Contract is what a mode asks of a notebook (M10 design 4.1): the pages
// its creation writes, in each language. M10 has two; M11's upgrade reads
// the same table, and a later mode is a row more.
type Contract struct {
	Mode      Mode
	Templates map[Locale][]TemplatePage
}

// TemplatePage is a page a mode's creation writes at the notebook's root,
// in order: the source root first.
type TemplatePage struct {
	Title      string
	Content    string
	SourceRoot bool
}

// ContractOf is mode's contract: standard writes nothing; llm_wiki writes
// raw, its source root, then AGENTS. false for a mode there is none of.
func ContractOf(mode Mode) (Contract, bool) {
	switch mode {
	case ModeStandard:
		return Contract{Mode: mode}, true
	case ModeLLMWiki:
		templates := make(map[Locale][]TemplatePage, len(Locales()))
		for _, l := range Locales() {
			templates[l] = template(l)
		}
		return Contract{Mode: mode, Templates: templates}, true
	}
	return Contract{}, false
}
