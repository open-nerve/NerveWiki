package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpgradePlaybook is the upgrade prompt's text (M11 design 4.13): for a
// notebook upgrading alone, which the module's row tells (M11/P3 decision
// 286). Like Playbooks it reads no page and writes nothing: the upgrade's
// tools check and authorize what the agent does next.
type UpgradePlaybook struct {
	notebooks Notebooks
	auth      shared.Authorizer
	rows      UpgradeRows
}

// NewUpgradePlaybook returns the use case.
func NewUpgradePlaybook(notebooks Notebooks, auth shared.Authorizer, rows UpgradeRows) *UpgradePlaybook {
	return &UpgradePlaybook{notebooks: notebooks, auth: auth, rows: rows}
}

// Execute answers the upgrade playbook for notebook: the actor; lint.read
// decided as the other prompts decide it (notebook.not_found for a
// notebook the caller cannot see, or none); the upgrade under way, read
// unlocked; wiki.upgrade_state unless it is upgrading. It reads no mode:
// an LLM Wiki has no upgrade under way, and answers wiki.upgrade_state as
// a standard notebook without one does.
func (p *UpgradePlaybook) Execute(ctx context.Context, notebook uuid.UUID) (string, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return "", err
	}
	if err := authorizeLintRead(ctx, p.notebooks, p.auth, actor, notebook); err != nil {
		return "", err
	}
	up, found, err := p.rows.Current(ctx, notebook)
	switch {
	case err != nil:
		return "", err
	case !found || up.State != domain.UpgradeUpgrading:
		return "", domain.ErrUpgradeState
	}
	return domain.UpgradePlaybookText(), nil
}
