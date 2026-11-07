package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Playbooks answers the LLM Wiki's prompts (M10 design 4.15). It reads no
// source, starts no lint and writes nothing: the tools the text names
// check and authorize what the agent does next.
type Playbooks struct {
	notebooks Notebooks
	auth      shared.Authorizer
}

// NewPlaybooks returns the use case.
func NewPlaybooks(notebooks Notebooks, auth shared.Authorizer) *Playbooks {
	return &Playbooks{notebooks: notebooks, auth: auth}
}

// Execute is kind's text for the notebook, argument with it when the
// caller gave one, in lint's order: the actor; lint.read, whose refusal of
// one who cannot see the notebook is notebook.not_found; the notebook's
// mode, notebook.not_llm_wiki for a standard one.
func (p *Playbooks) Execute(ctx context.Context, notebook uuid.UUID, kind domain.Playbook, argument *string) (string, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return "", err
	}
	if err := authorizeLintRead(ctx, p.notebooks, p.auth, actor, notebook); err != nil {
		return "", err
	}
	state, found, err := p.notebooks.StateOf(ctx, notebook)
	switch {
	case err != nil:
		return "", err
	case !found:
		return "", domain.ErrNotebookNotFound
	case state.Mode != domain.ModeLLMWiki:
		return "", domain.ErrNotLLMWiki
	}
	return domain.PlaybookText(kind, argument), nil
}
