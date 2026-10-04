package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

// Publisher publishes events in the caller's transaction: they reach the
// streams when it commits, and not when it rolls back (M5 design 4.10).
// It holds no stream; the command-line tools build it too.
type Publisher struct {
	notifier Notifier
}

// NewPublisher returns a publisher through n.
func NewPublisher(n Notifier) *Publisher {
	return &Publisher{notifier: n}
}

// Publish publishes e: another M's events come this way (M5 design 8). Its
// type must be one an event can have (domain.CheckType), and its data fit a
// payload: it holds ids only. Either error is the caller's, whose
// transaction rolls back.
func (p *Publisher) Publish(ctx context.Context, e domain.Event) error {
	payload, err := domain.Encode(e)
	if err != nil {
		return err
	}
	return p.notifier.Notify(ctx, payload)
}

// PageChange is a page a write unit changed: whether in the tree, and the
// revision of the content it wrote, zero for none.
type PageChange struct {
	PageID   uuid.UUID
	Tree     bool
	Revision int
}

// PagesWritten is a write unit of a notebook's pages.
type PagesWritten struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	Changes     []PageChange
}

// PagesWritten publishes a pages event of the unit: whether it changed the
// tree, and the pages whose content it wrote, none listed past MaxPages.
func (p *Publisher) PagesWritten(ctx context.Context, w PagesWritten) error {
	data := domain.Pages{Pages: []domain.PageRevision{}}
	for _, c := range w.Changes {
		data.Tree = data.Tree || c.Tree
		if c.Revision != 0 {
			data.Pages = append(data.Pages, domain.PageRevision{ID: c.PageID, Revision: c.Revision})
		}
	}
	if len(data.Pages) > domain.MaxPages {
		data.Pages = nil
	}
	return p.shedding(ctx, domain.TypePages, w.WorkspaceID, w.NotebookID, data, domain.Pages{Tree: data.Tree})
}

// LockChanged is an edit session of a page opened or ended.
type LockChanged struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	PageID      uuid.UUID
	SessionID   uuid.UUID
}

// LockChanged publishes a lock event of the page.
func (p *Publisher) LockChanged(ctx context.Context, l LockChanged) error {
	return p.shedding(ctx, domain.TypeLock, l.WorkspaceID, l.NotebookID, domain.Lock{PageID: l.PageID, SessionID: l.SessionID}, nil)
}

// AccessChanged is a change of who may see which notebooks of a workspace:
// the accounts it concerns, and whether it may concern every member.
type AccessChanged struct {
	WorkspaceID uuid.UUID
	UserIDs     []uuid.UUID
	Reached     bool
}

// AccessChanged publishes an access event of the workspace, which resets
// the streams it concerns; it reaches every member when the accounts do
// not fit a payload. A change that concerns no one publishes nothing.
func (p *Publisher) AccessChanged(ctx context.Context, a AccessChanged) error {
	if !a.Reached && len(a.UserIDs) == 0 {
		return nil
	}
	data := domain.Access{UserIDs: append([]uuid.UUID{}, a.UserIDs...), Reached: a.Reached}
	return p.shedding(ctx, domain.TypeAccess, a.WorkspaceID, uuid.Nil(), data, domain.Access{Reached: true})
}

// NotebooksDeleted is the deletion of notebooks of a workspace.
type NotebooksDeleted struct {
	WorkspaceID uuid.UUID
	NotebookIDs []uuid.UUID
}

// NotebooksDeleted publishes a notebooks_deleted event of the workspace,
// which resets the streams that see one of the notebooks, or, when they do
// not fit a payload, the workspace. No notebook publishes nothing.
func (p *Publisher) NotebooksDeleted(ctx context.Context, d NotebooksDeleted) error {
	if len(d.NotebookIDs) == 0 {
		return nil
	}
	data := domain.NotebooksDeleted{NotebookIDs: d.NotebookIDs}
	return p.shedding(ctx, domain.TypeNotebooksDeleted, d.WorkspaceID, uuid.Nil(), data, domain.NotebooksDeleted{})
}

// shedding publishes the event of data, or, when its payload is too long,
// of shed, which lists less; nil sheds nothing.
func (p *Publisher) shedding(ctx context.Context, t domain.Type, workspaceID, notebookID uuid.UUID, data, shed any) error {
	e, err := domain.NewEvent(t, workspaceID, notebookID, data)
	if err != nil {
		return err
	}
	err = p.Publish(ctx, e)
	if !errors.Is(err, domain.ErrTooLong) || shed == nil {
		return err
	}
	if e, err = domain.NewEvent(t, workspaceID, notebookID, shed); err != nil {
		return err
	}
	return p.Publish(ctx, e)
}
