package bootstrap

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// pageEvents publishes the page module's write units and edit sessions on
// the event stream (M5 design 8): a pages event of each unit, a lock event
// of each opening and end. The two modules do not import each other, so
// their values meet here.
type pageEvents struct {
	publisher *events.Publisher
}

func (p pageEvents) PagesChanged(ctx context.Context, e page.Event) error {
	changes := make([]events.PageChange, len(e.Changes))
	for i, c := range e.Changes {
		changes[i] = events.PageChange{PageID: c.NodeID, Tree: c.Moves(), Revision: c.Revision}
	}
	return p.publisher.PagesWritten(ctx, events.PagesWritten{WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, Changes: changes})
}

func (p pageEvents) EditSessionOpened(ctx context.Context, o page.SessionOpened) error {
	return p.publisher.LockChanged(ctx, events.LockChanged{WorkspaceID: o.WorkspaceID, NotebookID: o.NotebookID, PageID: o.PageID, SessionID: o.SessionID})
}

func (p pageEvents) EditSessionEnded(ctx context.Context, e page.SessionEnded) error {
	return p.publisher.LockChanged(ctx, events.LockChanged{WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, PageID: e.PageID, SessionID: e.SessionID})
}

// notebookEvents publishes the notebook module's visibility changes and
// deletions on the event stream: each resets the streams it concerns,
// which reconnect and read what they see again (M5 design 4.10).
type notebookEvents struct {
	publisher *events.Publisher
}

func (n notebookEvents) VisibilityChanged(ctx context.Context, v notebook.VisibilityChange) error {
	return n.publisher.AccessChanged(ctx, events.AccessChanged{WorkspaceID: v.WorkspaceID, UserIDs: v.UserIDs, Reached: v.Reached})
}

func (n notebookEvents) NotebookDeleted(ctx context.Context, d notebook.NotebookDeletion) error {
	return n.publisher.NotebooksDeleted(ctx, events.NotebooksDeleted{WorkspaceID: d.WorkspaceID, NotebookIDs: d.NotebookIDs})
}
