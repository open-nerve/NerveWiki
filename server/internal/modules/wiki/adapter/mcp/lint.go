package mcpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

type lintIn struct {
	Rule   *string `json:"rule,omitempty" jsonschema:"An exact health check rule. Omit for counts and previews; limit and cursor require a rule."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many items, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to continue after its last item."`
}

type pendingSourcesIn struct {
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many items, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to continue after its last item."`
}

func (t tools) check(ctx context.Context, call mcpserver.Call, in lintIn) (mcpserver.Result, error) {
	result, err := t.lint.Execute(ctx, app.LintScope{NotebookID: call.Notebook}, app.LintQuery{Rule: in.Rule, Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Result{}, err
	}
	return fitLint(result)
}

func (t tools) pendingSources(ctx context.Context, call mcpserver.Call, in pendingSourcesIn) (mcpserver.Result, error) {
	rule := string(domain.RuleUndigestedSources)
	return t.check(ctx, call, lintIn{Rule: &rule, Limit: in.Limit, Cursor: in.Cursor})
}
