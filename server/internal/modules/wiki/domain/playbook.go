package domain

import (
	_ "embed"
	"strings"
)

// Playbook is one of the LLM Wiki's three prompts (M10 design 4.15): how
// an agent ingests a source, answers from the wiki, and keeps it. Each is
// static English text: the caller's argument follows it, the tools it
// names decide everything else.
type Playbook string

const (
	PlaybookIngest Playbook = "ingest"
	PlaybookQuery  Playbook = "query"
	PlaybookLint   Playbook = "lint"
)

// MaxPlaybookBytes bounds each playbook's text (M10 design 4.18).
const MaxPlaybookBytes = 16 << 10

var (
	//go:embed playbook-ingest.md
	ingestPlaybook string
	//go:embed playbook-query.md
	queryPlaybook string
	//go:embed playbook-lint.md
	lintPlaybook string
)

// PlaybookText is kind's message: its text, and argument, when the caller
// gave one, once and as it is, after the words that say what it is.
func PlaybookText(kind Playbook, argument *string) string {
	text, lead := "", ""
	switch kind {
	case PlaybookIngest:
		text, lead = ingestPlaybook, "Source path supplied by the caller:\n"
	case PlaybookQuery:
		text, lead = queryPlaybook, "Question supplied by the caller:\n"
	case PlaybookLint:
		text = lintPlaybook
	}
	text = strings.TrimSpace(text)
	if argument == nil {
		return text
	}
	return text + "\n\n" + lead + *argument
}
