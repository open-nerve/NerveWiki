package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ModeGuard keeps a notebook's mode's contract in each of its writes
// (M10 design 4.6): a standard notebook's steps it lets by at once,
// reading nothing; an LLM Wiki's it judges by CheckContract on the step's
// facts, and asks page.manage_source, once a unit, of a step that needs
// it. Its refusals are marked (Refused) with the node they are about.
type ModeGuard struct {
	auth shared.Authorizer
}

// NewModeGuard is the guard over auth, the authorizer the units decide
// with. It uses auth only when a step needs an action asked.
func NewModeGuard(auth shared.Authorizer) *ModeGuard {
	return &ModeGuard{auth: auth}
}

// GuardWrite judges s by its notebook's contract, the state its unit read
// under the notebook's lock: page.protected and page.virtual_file as
// CheckContract answers them, page.source_admin_only for a step whose need
// the unit's actor is not granted. A database's error is answered as it
// is, and rolls the unit back.
func (g *ModeGuard) GuardWrite(ctx context.Context, s Step) error {
	if s.Notebook.Mode != domain.ModeLLMWiki {
		return nil
	}
	v := domain.CheckContract(s.Notebook, domain.FactsOf(s.Operation, s.Kind, s.Changes))
	switch {
	case v.Err != nil:
		return domain.Refuse(v.Err, v.NodeID)
	case v.Needs != "":
		err := s.Grants.Ask(ctx, v.Needs, g.decide(s, v.Needs))
		if isForbidden(err) {
			return domain.Refuse(domain.ErrSourceAdminOnly, v.NodeID)
		}
		return err
	}
	return nil
}

// decide is the decision of action on s's notebook for the unit's actor,
// ctx's: its own decision just saw the notebook, so that one not visible
// is denied too (Grants.Ask), page.source_admin_only as forbidden is.
func (g *ModeGuard) decide(s Step, action shared.Action) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		actor, err := shared.RequireActor(ctx)
		if err != nil {
			return err
		}
		_, err = g.auth.Authorize(ctx, actor, action, shared.Target{WorkspaceID: s.WorkspaceID, NotebookID: s.NotebookID})
		return err
	}
}
