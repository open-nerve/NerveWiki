package bootstrap

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// An upgrade's start (M11/P1 design 3.15): the wiki module's use case
// calls the notebook module's upgrade's columns and the page module's
// baseline through these, the modules' values converted field by field.

// wikiUpgradeStates are notebook.UpgradeStates as the wiki module's start
// calls them.
type wikiUpgradeStates struct {
	states *notebook.UpgradeStates
}

func (s wikiUpgradeStates) Lock(ctx context.Context, id uuid.UUID) (wiki.UpgradeNotebook, bool, error) {
	state, ok, err := s.states.Lock(ctx, id)
	if err != nil || !ok {
		return wiki.UpgradeNotebook{}, ok, err
	}
	mode, err := wikiModeOf(state.Mode)
	if err != nil {
		return wiki.UpgradeNotebook{}, false, err
	}
	up := wiki.NotebookUpgrade{ID: state.Upgrade.ID, By: state.Upgrade.By, At: state.Upgrade.At}
	switch state.Upgrade.State {
	case "":
	case notebook.UpgradeUpgrading:
		up.State = wiki.UpgradeUpgrading
	case notebook.UpgradeReviewing:
		up.State = wiki.UpgradeReviewing
	case notebook.UpgradeRollingBack:
		up.State = wiki.UpgradeRollingBack
	default:
		return wiki.UpgradeNotebook{}, false, errWikiLintValue
	}
	return wiki.UpgradeNotebook{Mode: mode, SourceRootID: state.SourceRootID, Upgrade: up}, true, nil
}

func (s wikiUpgradeStates) Start(ctx context.Context, id uuid.UUID, up wiki.NotebookUpgrade) error {
	if up.State != wiki.UpgradeUpgrading {
		return errWikiLintValue
	}
	return s.states.Start(ctx, id, notebook.Upgrade{ID: up.ID, State: notebook.UpgradeUpgrading, By: up.By, At: up.At})
}

func (s wikiUpgradeStates) Publish(ctx context.Context, c wiki.ModeChange) error {
	from, err := notebookModeOf(c.From)
	if err != nil {
		return err
	}
	to, err := notebookModeOf(c.To)
	if err != nil {
		return err
	}
	return s.states.Publish(ctx, notebook.ModeChange{WorkspaceID: c.WorkspaceID, NotebookID: c.NotebookID, From: from, To: to,
		By: c.By, At: c.At})
}

func wikiModeOf(m notebook.Mode) (wiki.Mode, error) {
	switch m {
	case notebook.ModeStandard:
		return wiki.ModeStandard, nil
	case notebook.ModeLLMWiki:
		return wiki.ModeLLMWiki, nil
	}
	return "", errWikiLintValue
}

func notebookModeOf(m wiki.Mode) (notebook.Mode, error) {
	switch m {
	case wiki.ModeStandard:
		return notebook.ModeStandard, nil
	case wiki.ModeLLMWiki:
		return notebook.ModeLLMWiki, nil
	}
	return "", errWikiLintValue
}

// wikiUpgradePages are page.Upgrades as the wiki module's start calls
// them.
type wikiUpgradePages struct {
	upgrades *page.Upgrades
}

func (p wikiUpgradePages) Capture(ctx context.Context, upgradeID, notebookID uuid.UUID) (wiki.UpgradeBaseline, error) {
	base, err := p.upgrades.Capture(ctx, upgradeID, notebookID)
	return wiki.UpgradeBaseline{Nodes: base.Nodes, Pages: base.Pages}, err
}

func (p wikiUpgradePages) Protect(ctx context.Context, upgradeID, notebookID, rootID uuid.UUID) error {
	return p.upgrades.Protect(ctx, upgradeID, notebookID, rootID)
}

func (p wikiUpgradePages) SetWanted(ctx context.Context, upgradeID uuid.UUID, wanted map[uuid.UUID]int) error {
	return p.upgrades.SetWanted(ctx, upgradeID, wanted)
}

func (p wikiUpgradePages) Analyze(ctx context.Context) error { return p.upgrades.Analyze(ctx) }

func (p wikiUpgradePages) ClearExpiredSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) error {
	return p.upgrades.ClearExpiredSessions(ctx, notebookID, at)
}

func (p wikiUpgradePages) LiveSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) ([]wiki.SessionHolder, int, error) {
	holders, total, err := p.upgrades.LiveSessions(ctx, notebookID, at)
	if err != nil {
		return nil, 0, err
	}
	out := make([]wiki.SessionHolder, len(holders))
	for i, h := range holders {
		out[i] = wiki.SessionHolder{PageID: h.PageID, UserID: h.UserID}
	}
	return out, total, nil
}

func (p wikiUpgradePages) RootNodes(ctx context.Context, notebookID uuid.UUID, keys []string) ([]wiki.RootNode, error) {
	nodes, err := p.upgrades.RootNodes(ctx, notebookID, keys)
	if err != nil {
		return nil, err
	}
	out := make([]wiki.RootNode, len(nodes))
	for i, n := range nodes {
		kind, err := wikiPageKind(n.Kind)
		if err != nil {
			return nil, err
		}
		out[i] = wiki.RootNode{ID: n.ID, Kind: kind, Key: n.Key}
	}
	return out, nil
}
