// Package postgresadapter sends the events by NOTIFY, in the publisher's
// transaction (M5 design 4.10).
package postgresadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Channel is the events' NOTIFY channel; River's are river_*.
const Channel = "nwiki_events"

// Notifier is the app's Notifier by postgres.Notify on Channel.
type Notifier struct{}

// Notify sends payload on Channel when the transaction in ctx commits;
// outside one it is an error.
func (Notifier) Notify(ctx context.Context, payload string) error {
	return postgres.Notify(ctx, Channel, payload)
}
