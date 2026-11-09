package app

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// maxUpgradeLocks bounds the edit sessions a refused start names; its
// detail says how many there are (M11 design 4.4 step 7).
const maxUpgradeLocks = 100

// UpgradeDeps are what an upgrade's start and read, its abandonment and
// its rollback's end need (M11/P1 design 3.14, 3.15, 3.20): Action is the
// notebook module's notebook.change_mode; Timeout bounds the start's
// transaction (decisions 227, 232).
type UpgradeDeps struct {
	Tx         shared.TxManager
	Snapshots  shared.Snapshots
	Notebooks  Notebooks
	Workspaces UpgradeWorkspaces
	States     UpgradeStates
	Pages      UpgradePages
	Rollback   RollbackPages
	Rows       UpgradeRows
	Writing    WritingJobs
	Rollbacks  Rollbacks
	Names      Names
	Authorizer shared.Authorizer
	Action     shared.Action
	Template   *Template
	Clock      Clock
	Logger     Logger
	Timeout    time.Duration
}

// locked locks notebook id's workspace's row FOR SHARE and its row FOR NO
// KEY UPDATE, decides the caller's action, and reads the clock once: the
// first steps of an admin's write of the upgrade, in its transaction
// (M11 design 4.4 step 3). notebook.not_found for no row or a notebook
// the caller cannot see, forbidden for one they are no admin of.
func (d UpgradeDeps) locked(ctx context.Context, actor shared.Actor, workspace, id uuid.UUID) (UpgradeNotebook, time.Time, error) {
	if ok, err := d.Workspaces.ShareByID(ctx, workspace); err != nil || !ok {
		return UpgradeNotebook{}, time.Time{}, orNotebookNotFound(err)
	}
	state, ok, err := d.States.Lock(ctx, id)
	if err != nil || !ok {
		return UpgradeNotebook{}, time.Time{}, orNotebookNotFound(err)
	}
	_, err = d.Authorizer.Authorize(ctx, actor, d.Action, shared.Target{WorkspaceID: workspace, NotebookID: id})
	if errors.Is(err, shared.ErrNotVisible) {
		return UpgradeNotebook{}, time.Time{}, domain.ErrNotebookNotFound
	}
	if err != nil {
		return UpgradeNotebook{}, time.Time{}, err
	}
	return state, d.Clock.Now().UTC(), nil
}

// StartUpgrade starts upgrading a standard notebook to an LLM Wiki:
// POST /api/v0/notebooks/{notebook_id}/upgrade (M11 design 4.4).
type StartUpgrade struct {
	d    UpgradeDeps
	lint *Lint
	runs *Runs
}

// NewStartUpgrade returns the use case, which prepares the wanted rule
// through lint and holds one of runs' slots while it does.
func NewStartUpgrade(d UpgradeDeps, lint *Lint, runs *Runs) *StartUpgrade {
	return &StartUpgrade{d: d, lint: lint, runs: runs}
}

// startOutcome is what the start's transaction leaves for its log line.
type startOutcome struct {
	workspace uuid.UUID
	view      UpgradeView
	agents    bool
	wanted    int64
}

// Execute starts the upgrade of notebook id from the template in locale,
// "" for en, in the order of M11 design 4.4:
//
//  1. locale zh-CN or en (422);
//  2. the notebook's workspace, unlocked (404);
//  3. one transaction, on a deadline of its own: the workspace's row FOR
//     SHARE, the notebook's FOR NO KEY UPDATE, the decision of
//     notebook.change_mode (404, 403), the clock once;
//  4. an LLM Wiki is notebook.already_llm_wiki;
//  5. an upgrade under way notebook.upgrading;
//  6. a job writing the tree transfer.busy;
//  7. the expired edit sessions cleared, a live one
//     wiki.upgrade_pages_locked, naming at most 100;
//  8. an attachment titled raw or AGENTS at the root
//     wiki.upgrade_title_taken; a page raw is the proposed source root;
//  9. a lint slot, now or never: server_busy;
//  10. the notebook's upgrade, a new id;
//  11. the baseline;
//  12. the template's pages the notebook lacks, a seeded raw protected;
//  13. the wanted rule prepared, a stale index
//     wiki.upgrade_stale_index, the baseline pages' wanted counts;
//  14. the module's row;
//  15. the mode's subscribers told;
//  16. after the commit, the slot released: a deadline of the start's own
//     is server_busy with a warn line; the baseline analyzed on the
//     request's ctx, a failure a warn line; an info line.
func (s *StartUpgrade) Execute(ctx context.Context, id uuid.UUID, locale string) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	l := domain.LocaleEn
	if locale != "" {
		l = domain.Locale(locale)
	}
	if l != domain.LocaleEn && l != domain.LocaleZhCN {
		return UpgradeView{}, shared.Invalid(shared.FieldError{Field: "template_locale", Code: shared.FieldInvalidFormat, Message: "must be zh-CN or en"})
	}
	workspace, ok, err := s.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	started := time.Now()
	ctxStart, cancel := context.WithTimeout(ctx, s.d.Timeout)
	defer cancel()
	var release func()
	defer func() {
		if release != nil {
			release()
		}
	}()
	var out startOutcome
	err = s.d.Tx.WithinTx(ctxStart, func(ctx context.Context) error {
		var err error
		out, err = s.start(ctx, actor, client, workspace, id, l, &release)
		return err
	})
	// Committed, or rolled back: the slot goes back now.
	if release != nil {
		release()
		release = nil
	}
	elapsed := time.Since(started)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctxStart.Err() != nil && ctx.Err() == nil {
			s.d.Logger.WarnContext(ctx, "upgrade start timed out", slog.String("workspace_id", workspace.String()),
				slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()),
				slog.Int64("elapsed_ms", elapsed.Milliseconds()), slog.Int64("timeout_ms", s.d.Timeout.Milliseconds()))
			return UpgradeView{}, shared.ServerBusy(time.Second)
		}
		return UpgradeView{}, err
	}
	up := out.view.Upgrade
	if err := s.d.Pages.Analyze(ctx); err != nil {
		s.d.Logger.WarnContext(ctx, "upgrade baseline not analyzed", slog.String("notebook_id", id.String()),
			slog.String("upgrade_id", up.ID.String()), slog.String("error", err.Error()))
	}
	s.d.Logger.InfoContext(ctx, "upgrade started", slog.String("workspace_id", workspace.String()),
		slog.String("notebook_id", id.String()), slog.String("upgrade_id", up.ID.String()),
		slog.String("user_id", actor.UserID.String()), slog.Int("baseline_nodes", up.BaselineNodes),
		slog.Int("baseline_pages", up.BaselinePages), slog.Bool("seeded_root", up.SeededRoot),
		slog.Bool("seeded_agents", out.agents), slog.Int64("wanted", out.wanted),
		slog.Int64("elapsed_ms", time.Since(started).Milliseconds()))
	return out.view, nil
}

// start is the start's transaction, steps 3 to 15; release receives the
// lint slot it holds.
func (s *StartUpgrade) start(ctx context.Context, actor shared.Actor, client string, workspace, id uuid.UUID, l domain.Locale,
	release *func()) (startOutcome, error) {
	state, at, err := s.d.locked(ctx, actor, workspace, id)
	if err != nil {
		return startOutcome{}, err
	}
	switch {
	case state.Mode == domain.ModeLLMWiki:
		return startOutcome{}, domain.ErrAlreadyLLMWiki
	case state.Upgrade.ID != uuid.UUID{}:
		return startOutcome{}, domain.ErrUpgrading
	}
	writing, err := s.d.Writing.Writing(ctx, id)
	if err != nil {
		return startOutcome{}, err
	}
	if writing {
		return startOutcome{}, domain.ErrWriting
	}
	if err := s.refuseSessions(ctx, id, at); err != nil {
		return startOutcome{}, err
	}
	roots, err := s.d.Pages.RootNodes(ctx, id, []string{shared.TitleKey(domain.SourceRootTitle), shared.TitleKey(domain.AgentsTitle)})
	if err != nil {
		return startOutcome{}, err
	}
	var root uuid.UUID
	missing := map[string]bool{shared.TitleKey(domain.SourceRootTitle): true, shared.TitleKey(domain.AgentsTitle): true}
	for _, n := range roots {
		if n.Kind != domain.NodePage {
			return startOutcome{}, domain.ErrUpgradeTitleTaken
		}
		delete(missing, n.Key)
		if n.Key == shared.TitleKey(domain.SourceRootTitle) {
			root = n.ID
		}
	}
	held, ok := s.runs.TryHold()
	if !ok {
		return startOutcome{}, shared.ServerBusy(time.Second)
	}
	*release = held
	upgradeID := uuid.NewV7()
	if err := s.d.States.Start(ctx, id, NotebookUpgrade{ID: upgradeID, State: domain.UpgradeUpgrading, By: actor.UserID, At: at}); err != nil {
		return startOutcome{}, err
	}
	base, err := s.d.Pages.Capture(ctx, upgradeID, id)
	if err != nil {
		return startOutcome{}, err
	}
	up := domain.Upgrade{ID: upgradeID, NotebookID: id, State: domain.UpgradeUpgrading, SourceRootID: root, Locale: l,
		BaselineNodes: base.Nodes, BaselinePages: base.Pages, CreatedBy: actor.UserID, CreatedAt: at}
	if len(missing) > 0 {
		var keys []string
		// The template's order: raw, then AGENTS.
		for _, title := range []string{domain.SourceRootTitle, domain.AgentsTitle} {
			if missing[shared.TitleKey(title)] {
				keys = append(keys, shared.TitleKey(title))
			}
		}
		seeded, err := s.d.Template.SeedUpgrade(ctx, id, upgradeID, l, client, at, keys)
		if err != nil {
			return startOutcome{}, err
		}
		if raw, ok := seeded[shared.TitleKey(domain.SourceRootTitle)]; ok {
			up.SourceRootID, up.SeededRoot = raw, true
			if err := s.d.Pages.Protect(ctx, upgradeID, id, raw); err != nil {
				return startOutcome{}, err
			}
		}
		if agents, ok := seeded[shared.TitleKey(domain.AgentsTitle)]; ok {
			up.SeededAgentsID = &agents
		}
	}
	llm, wanted := domain.ModeLLMWiki, domain.RuleWanted
	prepared, _, err := s.lint.prepareIn(ctx, LintScope{NotebookID: id, Contract: &llm, AreaRootID: &up.SourceRootID},
		RuleQuery{Rule: &wanted, Limit: 1, WantedByPage: true})
	if err != nil {
		return startOutcome{}, err
	}
	if prepared.StalePages() > 0 {
		return startOutcome{}, domain.ErrUpgradeStaleIndex
	}
	if err := s.d.Pages.SetWanted(ctx, upgradeID, prepared.WantedByPage()); err != nil {
		return startOutcome{}, err
	}
	count, err := prepared.RuleCount(ctx, wanted)
	if err != nil {
		return startOutcome{}, err
	}
	up.BaselineWanted, up.BaselineWantedCapped = int(count.Value), count.Capped
	if err := s.d.Rows.Insert(ctx, up, actor.UserID, at); err != nil {
		return startOutcome{}, err
	}
	if err := s.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return startOutcome{}, err
	}
	names, err := s.d.Names.DisplayNames(ctx, []uuid.UUID{actor.UserID})
	if err != nil {
		return startOutcome{}, err
	}
	return startOutcome{workspace: workspace, view: UpgradeView{Upgrade: up, StartedByName: names[actor.UserID]},
		agents: up.SeededAgentsID != nil, wanted: count.Value}, nil
}

// refuseSessions clears the notebook's expired edit sessions as of at and
// refuses a live one, naming the first 100 and how many there are.
func (s *StartUpgrade) refuseSessions(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := s.d.Pages.ClearExpiredSessions(ctx, id, at); err != nil {
		return err
	}
	holders, total, err := s.d.Pages.LiveSessions(ctx, id, at)
	if err != nil || total == 0 {
		return err
	}
	holders = holders[:min(len(holders), maxUpgradeLocks)]
	ids := make([]uuid.UUID, 0, len(holders))
	for _, h := range holders {
		ids = append(ids, h.UserID)
	}
	names, err := s.d.Names.DisplayNames(ctx, ids)
	if err != nil {
		return err
	}
	locks := make([]shared.LockHolder, len(holders))
	for i, h := range holders {
		locks[i] = shared.LockHolder{PageID: h.PageID, UserID: h.UserID, DisplayName: names[h.UserID]}
	}
	return domain.UpgradePagesLocked(locks, total)
}

// orNotebookNotFound is err, or notebook.not_found for a lock that found
// no row.
func orNotebookNotFound(err error) error {
	if err != nil {
		return err
	}
	return domain.ErrNotebookNotFound
}
