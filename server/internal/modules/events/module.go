// Package events is the event stream module (M5 design 4.10): the stream
// GET /api/v0/events, the hub that hands each event to the streams that
// see it, and the publisher the other modules' writes reach it through.
// It has no table: events travel by NOTIFY.
package events

import (
	"log/slog"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/events/adapter/http"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/events/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// Channel is the NOTIFY channel the server's listener listens on.
const Channel = postgresadapter.Channel

// The module's port.
type (
	// Visibility is what an account sees; bootstrap wires the workspace
	// and notebook modules' ports to it.
	Visibility = app.Visibility
	// Membership is a workspace an account is a member of, with its role.
	Membership = app.Membership
)

// Clock is the module's time.
type Clock interface {
	Now() time.Time
}

// Deps are what the module needs.
type Deps struct {
	Visibility Visibility
	Clock      Clock
	Logger     *slog.Logger
	// HeartbeatInterval is events.heartbeat_interval.
	HeartbeatInterval time.Duration
	// After is a timer: time.After when nil, a test's own otherwise.
	After func(time.Duration) <-chan time.Time
}

// Module is the event stream.
type Module struct {
	hub     *app.Hub
	handler *httpadapter.Handler
}

// New returns the module, its hub not listening yet.
func New(d Deps) *Module {
	after := d.After
	if after == nil {
		after = time.After
	}
	hub := app.NewHub(d.Logger)
	return &Module{hub: hub, handler: httpadapter.New(app.NewOpenStream(hub, d.Visibility), httpadapter.Config{
		Errors: httpserver.NewAPIErrors(d.Logger), Logger: d.Logger, Heartbeat: d.HeartbeatInterval, Now: d.Clock.Now, After: after,
	})}
}

// Notified hands the event of a payload the listener heard to the streams
// that see it.
func (m *Module) Notified(payload string) {
	m.hub.Notified(payload)
}

// Listening records whether the listener listens: a stream opens only
// while it does (503 not_ready otherwise).
func (m *Module) Listening(on bool) {
	m.hub.Listening(on)
}

// Reconnected resets every stream: the listener lost its connection, and
// what was sent meanwhile is lost (reset reconnected).
func (m *Module) Reconnected() {
	m.hub.ResetAll(domain.ResetReconnected)
}

// Streams is how many streams are open.
func (m *Module) Streams() int {
	return m.hub.Streams()
}

// Register mounts the stream on router behind api's long-lived
// middlewares: authentication, the rate limit, no deadline.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	router.Handle(httpadapter.Route, api.LongLived(m.handler))
}
