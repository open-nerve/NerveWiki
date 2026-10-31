package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

var (
	// ErrNotebookNotFound also hides a notebook the caller cannot see.
	ErrNotebookNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
	// ErrNotLLMWiki rejects a mode-specific rule under the standard contract.
	ErrNotLLMWiki = shared.NewError(shared.KindConflict, "notebook.not_llm_wiki", "The notebook is not an LLM Wiki.")
	// ErrLintRuleNotFound carries no unrecognized input. Its public schema
	// and translations arrive with the first HTTP consumer in S1-4.
	ErrLintRuleNotFound = shared.NewError(shared.KindNotFound, "wiki.lint_rule_not_found", "This health check rule does not exist.")
)
