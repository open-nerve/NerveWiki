package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CheckUpgrade checks a notebook's upgrade, or reads one rule's items in
// its scope (M11 design 4.7): GET …/upgrade/check, GET …/upgrade/check/
// {rule} and MCP's upgrade_verify.
type CheckUpgrade struct {
	d    UpgradeDeps
	lint *Lint
	runs Runner
}

// UpgradeChecked is a check (query without a rule) or a page of a rule's
// items: one of the two.
type UpgradeChecked struct {
	Check *domain.UpgradeCheck
	Items *domain.LintResult
}

// NewCheckUpgrade returns the use case, which computes through lint's
// runs, a slot of them each.
func NewCheckUpgrade(d UpgradeDeps, lint *Lint, runs Runner) *CheckUpgrade {
	return &CheckUpgrade{d: d, lint: lint, runs: runs}
}

// errUpgradeDefect is an upgrade's check that found what only a defect
// leaves (M11/P2 design 3.11): a 500.
var errUpgradeDefect = errors.New("wiki: an upgrade's check found a defect")

// Execute checks notebook id's upgrade, or reads query's rule's items in
// its scope, in the order of M11/P2 design 3.11:
//
//  1. a cursor that does not decode is bad_request;
//  2. the notebook (404) and notebook.change_mode (404, 403);
//  3. no rule with a limit or a cursor is validation_failed, an unknown
//     rule wiki.lint_rule_not_found;
//  4. the upgrade, read unlocked: none, or neither upgrading nor
//     reviewing, wiki.upgrade_state; its scope is an LLM Wiki's explicit
//     contract with the source root it proposes;
//  5. the items: a cursor bound to another scope, rule or root
//     bad_request, a limit outside 1-100 validation_failed;
//  6. lint's runs, keyed by the upgrade: server_busy; in the snapshot,
//     the upgrade read again;
//  7. notebook.change_mode again before the answer.
func (c *CheckUpgrade) Execute(ctx context.Context, id uuid.UUID, query LintQuery) (UpgradeChecked, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeChecked{}, err
	}
	query.Rule, query.Limit, query.Cursor = lintCopy(query.Rule), lintCopy(query.Limit), lintCopy(query.Cursor)
	var decoded DecodedCursor
	if query.Cursor != nil {
		if decoded, err = c.d.Cursors.Decode(*query.Cursor); err != nil {
			return UpgradeChecked{}, err
		}
		if decoded == nil {
			return UpgradeChecked{}, errLintInvariant
		}
	}
	if err := c.d.authorize(ctx, actor, id); err != nil {
		return UpgradeChecked{}, err
	}
	var rule domain.LintRule
	if query.Rule == nil {
		var fields []shared.FieldError
		if query.Limit != nil {
			fields = append(fields, shared.FieldError{Field: "limit", Code: shared.FieldNotAllowed, Message: "requires a rule"})
		}
		if query.Cursor != nil {
			fields = append(fields, shared.FieldError{Field: "cursor", Code: shared.FieldNotAllowed, Message: "requires a rule"})
		}
		if len(fields) != 0 {
			return UpgradeChecked{}, shared.Invalid(fields...)
		}
	} else {
		var known bool
		if rule, known = domain.ParseLintRule(*query.Rule); !known {
			return UpgradeChecked{}, domain.ErrLintRuleNotFound
		}
	}
	up, ok, err := c.d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return UpgradeChecked{}, err
	case !ok || up.State != domain.UpgradeUpgrading && up.State != domain.UpgradeReviewing:
		return UpgradeChecked{}, domain.ErrUpgradeState
	}
	scope := upgradeScope(up)
	key := upgradeKey(up, domain.LintUpgradeCheck, up.ID)
	read := RuleQuery{WantedByPage: true}
	if query.Rule != nil {
		var after *domain.LintPosition
		if decoded != nil {
			pos, err := decoded.PositionFor(scope, rule)
			if err != nil {
				return UpgradeChecked{}, err
			}
			pos.ID, pos.Prefix = lintCopy(pos.ID), slices.Clone(pos.Prefix)
			after = &pos
		}
		limit, err := shared.PageSize(query.Limit)
		if err != nil {
			return UpgradeChecked{}, err
		}
		key.Kind, key.Rule, key.Limit = domain.LintUpgradeItems, rule, limit
		read = RuleQuery{Rule: &rule, After: after, Limit: limit}
		if after != nil {
			if key.CursorCanonical, err = c.d.Cursors.Encode(scope, rule, *after); err != nil {
				return UpgradeChecked{}, err
			}
		}
	}
	result, err := c.runs.Do(ctx, key, func(worker context.Context) (RunOutput, error) {
		return c.compute(worker, up, func(up domain.Upgrade, now domain.Upgrade) bool {
			return now.ID == up.ID && (now.State == domain.UpgradeUpgrading || now.State == domain.UpgradeReviewing)
		}, read, key.Kind)
	})
	if err != nil {
		return UpgradeChecked{}, err
	}
	if err := c.d.authorize(ctx, actor, id); err != nil {
		return UpgradeChecked{}, err
	}
	if err := ctx.Err(); err != nil {
		return UpgradeChecked{}, err
	}
	if query.Rule != nil {
		items := domain.CloneLintResult(result.Lint)
		return UpgradeChecked{Items: &items}, nil
	}
	if result.Check == nil {
		return UpgradeChecked{}, errLintInvariant
	}
	return UpgradeChecked{Check: result.Check}, nil
}

// upgradeScope is up's check's scope: an LLM Wiki's explicit contract with
// the source root it proposes (M11 design 4.7).
func upgradeScope(up domain.Upgrade) LintScope {
	llm, root := domain.ModeLLMWiki, up.SourceRootID
	return LintScope{NotebookID: up.NotebookID, Contract: &llm, AreaRootID: &root}
}

// upgradeKey is a run's key of up's check of kind, in its scope, scoped by
// scope: the upgrade's id, or a submission's.
func upgradeKey(up domain.Upgrade, kind domain.LintQueryKind, scope uuid.UUID) RunKey {
	return RunKey{NotebookID: up.NotebookID, Explicit: true, Contract: domain.ModeLLMWiki, HasRoot: true, RootID: up.SourceRootID,
		Kind: kind, Scope: scope}
}

// authorize is notebook.change_mode of actor in notebook id: none, and one
// the actor cannot see, are notebook.not_found; one they are no admin of
// forbidden.
func (d UpgradeDeps) authorize(ctx context.Context, actor shared.Actor, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	workspace, ok, err := d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		return domain.ErrNotebookNotFound
	}
	_, err = d.Authorizer.Authorize(ctx, actor, d.Action, shared.Target{WorkspaceID: workspace, NotebookID: id})
	if errors.Is(err, shared.ErrNotVisible) {
		return domain.ErrNotebookNotFound
	}
	return err
}

// compute is a run's worker: in one snapshot, the clock, the upgrade read
// again, which still must be up's as still judges it (wiki.upgrade_state
// otherwise: one approved or rolled back while the run queued, B-M3), and
// the check (a summary query) or the rule's items.
func (c *CheckUpgrade) compute(ctx context.Context, up domain.Upgrade, still func(up, now domain.Upgrade) bool, query RuleQuery,
	kind domain.LintQueryKind,
) (RunOutput, error) {
	var out RunOutput
	err := c.d.Snapshots.WithinSnapshot(ctx, func(snapshot context.Context) error {
		at := c.d.Clock.Now().UTC()
		now, ok, err := c.d.Rows.Current(snapshot, up.NotebookID)
		switch {
		case err != nil:
			return err
		case !ok || !still(up, now):
			return domain.ErrUpgradeState
		}
		if kind == domain.LintUpgradeItems {
			out, err = c.lint.computeIn(snapshot, upgradeScope(now), query, domain.LintItems, at)
			return err
		}
		out, err = c.checkIn(snapshot, now, at)
		return err
	})
	if err != nil {
		out.Result, out.Check = domain.LintResult{}, nil
		return out, err
	}
	if err := ctx.Err(); err != nil {
		out.Result, out.Check = domain.LintResult{}, nil
		return out, err
	}
	return out, nil
}

// checkIn is up's check as of at, through the snapshot ctx carries (M11/P2
// design 3.11): the roots (a proposed root that is no live page is a
// defect: an error line, a 500); the summary of the eight rules with the
// general emitters' wanted counts; the baseline, what of it was kept and
// the changesets; the judgement; the paths of the gates' nodes alive. The
// submission's too.
func (c *CheckUpgrade) checkIn(ctx context.Context, up domain.Upgrade, at time.Time) (RunOutput, error) {
	ids := []any{slog.String("notebook_id", up.NotebookID.String()), slog.String("upgrade_id", up.ID.String())}
	roots, err := c.d.Gates.Roots(ctx, up.NotebookID, up.SourceRootID)
	if err != nil {
		return RunOutput{}, err
	}
	if !roots.RootPage {
		c.d.Logger.ErrorContext(ctx, "upgrade source root is no live page", ids...)
		return RunOutput{}, fmt.Errorf("%w: the source root %s is no live page", errUpgradeDefect, up.SourceRootID)
	}
	scope, query := upgradeScope(up), RuleQuery{WantedByPage: true}
	prepared, stats, input, err := c.lint.prepareIn(ctx, scope, query)
	out := RunOutput{Stats: stats}
	if err != nil {
		return out, err
	}
	summary := domain.LintResult{Kind: domain.LintSummary, Mode: stats.Mode, ComputedAt: at, StalePages: *stats.StalePages}
	if err := c.lint.project(ctx, scope, query, prepared, &summary, &out.Stats); err != nil {
		return out, err
	}
	read := domain.UpgradeRead{Upgrade: up, Roots: roots, Area: input.SourceIDs, Lint: summary, WantedByPage: prepared.WantedByPage(),
		Reserved: c.d.ReservedKeys, Nodes: make([]domain.GateNode, len(input.Nodes))}
	for i, n := range input.Nodes {
		read.Nodes[i] = domain.GateNode{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Key: n.NameKey}
	}
	if read.Baseline, err = c.d.Gates.Baseline(ctx, up.ID); err != nil {
		return out, err
	}
	if read.Preserved, err = c.d.Gates.Preserved(ctx, up.ID); err != nil {
		return out, err
	}
	if read.Changesets, err = c.d.Gates.Changesets(ctx, up.ID); err != nil {
		return out, err
	}
	check := domain.JudgeUpgrade(read)
	check.ComputedAt = at
	if p := read.Preserved; p.Nodes != up.BaselineNodes || p.Pages != up.BaselinePages {
		c.d.Logger.ErrorContext(ctx, "upgrade baseline rows not its counts", append(ids, slog.Int("nodes", p.Nodes),
			slog.Int("pages", p.Pages), slog.Int("baseline_nodes", up.BaselineNodes), slog.Int("baseline_pages", up.BaselinePages))...)
	}
	g := check.Gates
	named := slices.Concat(g.ReservedTitles.NodeIDs, g.Links.NodeIDs, g.Preserved.NodeIDs)
	check.Paths = map[uuid.UUID]string{}
	if len(named) > 0 {
		if check.Paths, err = c.d.Paths.Paths(ctx, up.NotebookID, named); err != nil {
			return out, err
		}
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Check = &check
	return out, nil
}
