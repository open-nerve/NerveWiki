package mcpadapter

import (
	"context"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// submitted is upgrade_submit's answer: the upgrade's state and when it
// was submitted.
type submitted struct {
	State       string     `json:"state"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

// verify is upgrade_verify: the check, or one rule's items in its scope,
// lint's arguments (M11/P2 design 3.14).
func (t tools) verify(ctx context.Context, call mcpserver.Call, in lintIn) (mcpserver.Result, error) {
	checked, err := t.upgrades.CheckUpgrade(ctx, call.Notebook, app.LintQuery{Rule: in.Rule, Limit: in.Limit, Cursor: in.Cursor})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case checked.Items != nil:
		return fitLint(*checked.Items)
	case checked.Check != nil:
		return fitUpgradeCheck(*checked.Check)
	}
	return mcpserver.Result{}, errLintProjection
}

// submit is upgrade_submit: the upgrade reviewing, with its report kept,
// as its state and when it was submitted.
func (t tools) submit(ctx context.Context, call mcpserver.Call, _ struct{}) (mcpserver.Result, error) {
	view, err := t.upgrades.SubmitUpgrade(ctx, call.Notebook)
	if err != nil {
		return mcpserver.Result{}, err
	}
	return mcpserver.Result{Structured: submitted{State: string(view.Upgrade.State), SubmittedAt: view.Upgrade.SubmittedAt}}, nil
}
