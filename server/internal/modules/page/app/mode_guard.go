package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ModeGuard keeps a notebook's mode's contract in each of its writes
// (M10 design 4.6): a standard notebook's steps it lets by at once,
// reading nothing; an LLM Wiki's it judges by CheckContract on the step's
// facts, and asks page.manage_source, once a unit, of a step that needs
// it. Its refusals are marked (Refused) with the node they are about.
type ModeGuard struct {
	auth    shared.Authorizer
	sources SourceAncestors
	hashes  SourceStore
}

// SourceAncestors reads a page's already-locked notebook ancestry.
type SourceAncestors interface {
	Ancestors(context.Context, uuid.UUID) ([]domain.Ancestor, error)
}

// NewModeGuard is the guard over auth, the authorizer the units decide
// with. It uses auth only when a step needs an action asked.
func NewModeGuard(auth shared.Authorizer, sources SourceAncestors, hashes SourceStore) *ModeGuard {
	return &ModeGuard{auth: auth, sources: sources, hashes: hashes}
}

// VetoEditSession refuses a source page even when its body is empty. The
// notebook and content locks are held by the opening's unit; an earlier
// unlocked read would miss a move into the source area while waiting.
func (g *ModeGuard) VetoEditSession(ctx context.Context, o SessionOpening) error {
	if o.Notebook.Mode != domain.ModeLLMWiki || o.PageID == o.Notebook.SourceRootID {
		return nil
	}
	line, err := g.sources.Ancestors(ctx, o.PageID)
	if err != nil {
		return err
	}
	place := domain.NodePlace{}
	if len(line) > 0 {
		place.ParentID = &line[len(line)-1].ID
	}
	facts := domain.StepFacts{Operation: domain.OpContent, Kind: o.Kind, EditSession: true,
		Nodes: []domain.NodeFacts{{ID: o.PageID, Kind: domain.KindPage, Before: &place, After: &place}}}
	facts = domain.SourceFacts(facts, o.Notebook.SourceRootID, line, line, nil)
	if v := domain.CheckContract(o.Notebook, facts); v.Err != nil {
		return domain.Refuse(v.Err, v.NodeID)
	}
	return nil
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
	v := domain.CheckContract(s.Notebook, s.contractFacts())
	if v.Err != nil {
		return domain.Refuse(v.Err, v.NodeID)
	}
	if err := g.checkSourceDuplicates(ctx, s); err != nil {
		return err
	}
	switch {
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
