package mcpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// PlaybooksUseCase is the text of a playbook.
type PlaybooksUseCase interface {
	Execute(ctx context.Context, notebook uuid.UUID, kind domain.Playbook, argument *string) (string, error)
}

// The prompts' descriptions, as api/mcp/prompts.json has them.
const (
	ingestPlaybookDescription = `Turn an LLM Wiki source into maintained pages, in one batch, then check the result. LLM Wiki only.`
	lintPlaybookDescription   = `Check an LLM Wiki, make justified mechanical repairs, inspect recent changes, and report. LLM Wiki only.`
	queryPlaybookDescription  = `Answer a question from this LLM Wiki with references to its pages and sources. LLM Wiki only.`
)

// Prompts are the LLM Wiki's three prompts, in the notebook entry alone
// (M10 design 4.15): the account entry has no notebook to read them for,
// and its open_notebook points at this one. A standard notebook lists them
// too: the list is static, and a get answers notebook.not_llm_wiki.
func Prompts(p PlaybooksUseCase) []mcpserver.Prompt {
	return []mcpserver.Prompt{
		{Name: "ingest", Title: "Ingest a source", Description: ingestPlaybookDescription, Entry: mcpserver.NotebookEntry,
			Arguments: []mcpserver.PromptArgument{{Name: "source", Description: "The source path in this notebook. Omit it to list pending sources first."}},
			Get:       playbook(p, domain.PlaybookIngest, ingestPlaybookDescription, "source")},
		{Name: "lint", Title: "Check and maintain the wiki", Description: lintPlaybookDescription, Entry: mcpserver.NotebookEntry,
			Get: playbook(p, domain.PlaybookLint, lintPlaybookDescription, "")},
		{Name: "query", Title: "Answer from the wiki", Description: queryPlaybookDescription, Entry: mcpserver.NotebookEntry,
			Arguments: []mcpserver.PromptArgument{{Name: "question", Description: "The question to answer from this notebook.", Required: true}},
			Get:       playbook(p, domain.PlaybookQuery, queryPlaybookDescription, "question")},
	}
}

// playbook is a prompt's get: its one user message is kind's text, with
// the argument named argument when the caller gave it; "" names none. An
// empty value is given: the tools that follow judge it.
func playbook(p PlaybooksUseCase, kind domain.Playbook, description, argument string) func(context.Context, mcpserver.Call, map[string]string) (string, []string, error) {
	return func(ctx context.Context, c mcpserver.Call, args map[string]string) (string, []string, error) {
		var given *string
		if v, ok := args[argument]; ok && argument != "" {
			given = &v
		}
		text, err := p.Execute(ctx, c.Notebook, kind, given)
		if err != nil {
			return "", nil, err
		}
		return description, []string{text}, nil
	}
}
