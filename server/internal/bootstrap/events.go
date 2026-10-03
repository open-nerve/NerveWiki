package bootstrap

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// eventsModule is the event stream over pool (M5 design 4.10), and the
// listener that feeds its hub: each payload, whether it listens, its
// reconnections. The listener connects when run starts it.
func eventsModule(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*events.Module, *postgres.Listener) {
	ev := events.New(events.Deps{
		Visibility:        eventsVisibility{workspaces: workspace.NewMemberships(pool), notebooks: notebook.NewVisibleNotebooks(pool)},
		Clock:             clock.System{},
		Logger:            logger,
		HeartbeatInterval: cfg.Events.HeartbeatInterval,
	})
	return ev, postgres.NewListener(pool, events.Channel, logger, postgres.ListenerOptions{
		OnNotify:    ev.Notified,
		OnListening: ev.Listening,
		OnReconnect: ev.Reconnected,
	})
}

// eventsVisibility is the workspace and notebook modules' read ports as
// the event stream's Visibility.
type eventsVisibility struct {
	workspaces workspace.Memberships
	notebooks  notebook.VisibleNotebooks
}

func (v eventsVisibility) WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]events.Membership, error) {
	listed, err := v.workspaces.WorkspacesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]events.Membership, len(listed))
	for i, m := range listed {
		out[i] = events.Membership{WorkspaceID: m.WorkspaceID, Role: m.Role}
	}
	return out, nil
}

func (v eventsVisibility) NotebooksIn(ctx context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error) {
	return v.notebooks.VisibleIn(ctx, workspaceID, userID, role)
}
