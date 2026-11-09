package httpadapter

import (
	"context"
	"time"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apigen"
)

func (h handler) GetUpgrade(ctx context.Context, req gen.GetUpgradeRequestObject) (gen.GetUpgradeResponseObject, error) {
	view, err := h.upgrades.GetUpgrade(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	return gen.GetUpgrade200JSONResponse(upgradeOf(view)), nil
}

func (h handler) StartUpgrade(ctx context.Context, req gen.StartUpgradeRequestObject) (gen.StartUpgradeResponseObject, error) {
	var locale string
	if req.Body != nil && req.Body.TemplateLocale != nil {
		locale = string(*req.Body.TemplateLocale)
	}
	view, err := h.upgrades.StartUpgrade(ctx, req.NotebookID, locale)
	if err != nil {
		return nil, err
	}
	return gen.StartUpgrade200JSONResponse(upgradeOf(view)), nil
}

// upgradeOf is view as the contract's Upgrade: the pages the start wrote,
// raw then AGENTS; each step's fields null until it.
func upgradeOf(view app.UpgradeView) gen.Upgrade {
	up := view.Upgrade
	out := gen.Upgrade{
		ID: up.ID, NotebookID: up.NotebookID, State: gen.UpgradeState(up.State),
		StartedBy: apigen.UserRef{UserID: up.CreatedBy, DisplayName: view.StartedByName}, StartedAt: up.CreatedAt,
		SourceRootID: up.SourceRootID, SeededPageIds: []uuid.UUID{},
		Baseline:         gen.UpgradeBaseline{Nodes: up.BaselineNodes, Pages: up.BaselinePages, Wanted: gen.LintCount(up.BaselineWanted)},
		SubmittedBy:      nullable.NewNullNullable[apigen.UserRef](),
		SubmittedAt:      nullable.NewNullNullable[time.Time](),
		RollbackJobID:    nullable.NewNullNullable[uuid.UUID](),
		Ending:           nullable.NewNullNullable[gen.UpgradeEnding](),
		RollbackComplete: nullable.NewNullNullable[bool](),
		RollbackLeft:     nullable.NewNullNullable[gen.UpgradeRollbackLeft](),
	}
	if up.SeededRoot {
		out.SeededPageIds = append(out.SeededPageIds, up.SourceRootID)
	}
	if up.SeededAgentsID != nil {
		out.SeededPageIds = append(out.SeededPageIds, *up.SeededAgentsID)
	}
	if up.SubmittedBy != nil {
		out.SubmittedBy = nullable.NewNullableWithValue(apigen.UserRef{UserID: *up.SubmittedBy, DisplayName: view.SubmittedByName})
	}
	if up.SubmittedAt != nil {
		out.SubmittedAt = nullable.NewNullableWithValue(*up.SubmittedAt)
	}
	if up.RollbackJobID != nil {
		out.RollbackJobID = nullable.NewNullableWithValue(*up.RollbackJobID)
	}
	if up.Ending != "" {
		out.Ending = nullable.NewNullableWithValue(gen.UpgradeEnding(up.Ending))
	}
	if up.RollbackComplete != nil {
		out.RollbackComplete = nullable.NewNullableWithValue(*up.RollbackComplete)
	}
	if left := up.RollbackLeft; left != nil {
		out.RollbackLeft = nullable.NewNullableWithValue(gen.UpgradeRollbackLeft{Nodes: left.Nodes, Pages: left.Pages, Created: left.Created})
	}
	return out
}
