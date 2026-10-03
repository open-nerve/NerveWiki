package events

import (
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/events/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

// The publisher and what it publishes: the other modules' writes reach the
// streams this way, in their transactions (M5 design 8).
type (
	// Publisher publishes events in the caller's transaction.
	Publisher = app.Publisher
	// Event is another M's event: its type, workspace, notebook and data.
	Event = domain.Event
	// PagesWritten is a write unit of a notebook's pages.
	PagesWritten = app.PagesWritten
	// PageChange is a page a write unit changed.
	PageChange = app.PageChange
	// LockChanged is an edit session of a page opened or ended.
	LockChanged = app.LockChanged
	// AccessChanged is a change of who sees which notebooks of a
	// workspace.
	AccessChanged = app.AccessChanged
	// NotebooksDeleted is the deletion of notebooks of a workspace.
	NotebooksDeleted = app.NotebooksDeleted
)

// NewPublisher returns the publisher. It sends by NOTIFY in the caller's
// transaction and holds nothing else: the command-line tools build it
// too, without the stream.
func NewPublisher() *Publisher {
	return app.NewPublisher(postgresadapter.Notifier{})
}
