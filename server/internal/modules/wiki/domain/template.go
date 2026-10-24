package domain

import _ "embed"

// The LLM Wiki's template (M10 design 4.4; M10/P1 design 3.9): its two
// pages in each language, the source root raw and the agreement AGENTS,
// which its users rewrite after. The files are not named AGENTS.md: the
// repository's coding agents read a file of that name, and a file system
// that folds case reads agents.md as it too.
var (
	//go:embed template-en-raw.md
	enRaw string
	//go:embed template-en-agents.md
	enAgents string
	//go:embed template-zh-CN-raw.md
	zhCNRaw string
	//go:embed template-zh-CN-agents.md
	zhCNAgents string
)

// MaxTemplateBytes bounds each page of a template: an AGENTS page holds
// at most 8 KiB (M10 design 4.18).
const MaxTemplateBytes = 8 << 10

// The template's two titles, alike in every language: AGENTS is the title
// the instructions are found by (page/domain's AgentsTitle), and the
// system's instructions and AGENTS's text both say raw/.
const (
	SourceRootTitle = "raw"
	AgentsTitle     = "AGENTS"
)

// template is the LLM Wiki's pages in language l, in order: raw, its
// source root, then AGENTS; nil for a language there is none of.
func template(l Locale) []TemplatePage {
	raw, agents := enRaw, enAgents
	switch l {
	case LocaleEn:
	case LocaleZhCN:
		raw, agents = zhCNRaw, zhCNAgents
	default:
		return nil
	}
	return []TemplatePage{{Title: SourceRootTitle, Content: raw, SourceRoot: true}, {Title: AgentsTitle, Content: agents}}
}

// TemplateMessage is the message of the changeset that writes the
// template in language l (M10 design 4.4), "" for a language there is
// none of.
func TemplateMessage(l Locale) string {
	switch l {
	case LocaleEn:
		return "LLM Wiki template"
	case LocaleZhCN:
		return "LLM Wiki 模板"
	}
	return ""
}
