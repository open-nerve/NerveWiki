package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListedAuditEvent is an audit event with the profiles of its former owner
// and its actor.
type ListedAuditEvent struct {
	Event       domain.AuditEvent
	FormerOwner Profile
	Actor       Profile
}

// AuditPage is a page of audit events and the cursor of the next, "" after
// the last page.
type AuditPage struct {
	Events     []ListedAuditEvent
	NextCursor string
}

// ListNotebookAuditEventsDeps are what ListNotebookAuditEvents needs.
type ListNotebookAuditEventsDeps struct {
	Workspaces Workspaces
	Audit      AuditFinder
	Profiles   MemberProfiles
	Auth       shared.Authorizer
}

// ListNotebookAuditEvents lists a workspace's audit events a page at a
// time: GET /api/v0/workspaces/{slug}/notebook-audit-events (M3/P3 design
// 3.4).
type ListNotebookAuditEvents struct {
	d ListNotebookAuditEventsDeps
}

// NewListNotebookAuditEvents returns the use case.
func NewListNotebookAuditEvents(d ListNotebookAuditEventsDeps) *ListNotebookAuditEvents {
	return &ListNotebookAuditEvents{d: d}
}

// Execute returns the page of the workspace of slug's audit events that
// limit and cursor ask for, newest first, from the start when cursor is
// nil. A cursor shared.DecodeCursor refuses is 400 bad_request, judged
// first: it is the request's structure. Then the workspace and the
// decision, its admins alone; then the limit, 422 outside 1–100. One row
// more than the page is read to tell whether another follows. A read
// takes no lock and opens no transaction.
func (l *ListNotebookAuditEvents) Execute(ctx context.Context, slug string, limit *int, cursor *string) (AuditPage, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return AuditPage{}, err
	}
	var after *domain.AuditCursor
	if cursor != nil {
		after = new(domain.AuditCursor)
		if err := shared.DecodeCursor(*cursor, after); err != nil {
			return AuditPage{}, err
		}
	}
	workspaceID, _, err := authorizeIn(ctx, l.d.Workspaces, l.d.Auth, actor, domain.ActionListAudit, slug)
	if err != nil {
		return AuditPage{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return AuditPage{}, err
	}
	events, err := l.d.Audit.ListAuditEvents(ctx, workspaceID, after, size+1)
	if err != nil {
		return AuditPage{}, err
	}
	var page AuditPage
	if len(events) > size {
		last := events[size-1]
		if page.NextCursor, err = shared.EncodeCursor(domain.AuditCursor{CreatedAt: last.At, ID: last.ID}); err != nil {
			return AuditPage{}, err
		}
		events = events[:size]
	}
	if len(events) == 0 {
		return page, nil
	}
	var ids []uuid.UUID
	for _, e := range events {
		ids = append(ids, e.FormerOwnerID, e.ActorID)
	}
	profiles, err := profilesOf(ctx, l.d.Profiles, ids)
	if err != nil {
		return AuditPage{}, err
	}
	page.Events = make([]ListedAuditEvent, len(events))
	for i, e := range events {
		page.Events[i] = ListedAuditEvent{Event: e, FormerOwner: profiles[e.FormerOwnerID], Actor: profiles[e.ActorID]}
	}
	return page, nil
}
