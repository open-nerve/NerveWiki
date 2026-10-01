package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// OwnerlessNotebook is an ownerless notebook as its workspace's admins see
// it: its former owner's profile, and its size and last activity across
// the activity's sources (M3 design 4).
type OwnerlessNotebook struct {
	Notebook       domain.Notebook
	MemberCount    int
	FormerOwner    Profile
	SizeBytes      int64
	LastActivityAt time.Time
}

// ListOwnerlessNotebooksDeps are what ListOwnerlessNotebooks needs.
type ListOwnerlessNotebooksDeps struct {
	Workspaces Workspaces
	Notebooks  OwnerlessFinder
	Profiles   MemberProfiles
	Activities []NotebookActivitySource
	Auth       shared.Authorizer
}

// ListOwnerlessNotebooks lists a workspace's ownerless notebooks:
// GET /api/v0/workspaces/{slug}/ownerless-notebooks (M3/P3 design 3.3).
type ListOwnerlessNotebooks struct {
	d ListOwnerlessNotebooksDeps
}

// NewListOwnerlessNotebooks returns the use case.
func NewListOwnerlessNotebooks(d ListOwnerlessNotebooksDeps) *ListOwnerlessNotebooks {
	return &ListOwnerlessNotebooks{d: d}
}

// Execute returns the ownerless notebooks of the workspace of slug, the
// earliest to become so first, private ones included: the workspace's
// admins alone may list them, its members and guests are forbidden. A
// read takes no lock and opens no transaction.
func (l *ListOwnerlessNotebooks) Execute(ctx context.Context, slug string) ([]OwnerlessNotebook, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	workspaceID, _, err := authorizeIn(ctx, l.d.Workspaces, l.d.Auth, actor, domain.ActionListOwnerless, slug)
	if err != nil {
		return nil, err
	}
	listed, err := l.d.Notebooks.ListOwnerless(ctx, workspaceID)
	if err != nil || len(listed) == 0 {
		return nil, err
	}
	updated := make(map[uuid.UUID]time.Time, len(listed))
	owners := make([]uuid.UUID, 0, len(listed))
	for _, o := range listed {
		updated[o.Notebook.ID] = o.Notebook.UpdatedAt
		owners = append(owners, o.Notebook.Ownerless.FormerOwner)
	}
	activity, err := activityOf(ctx, l.d.Activities, updated)
	if err != nil {
		return nil, err
	}
	profiles, err := profilesOf(ctx, l.d.Profiles, owners)
	if err != nil {
		return nil, err
	}
	out := make([]OwnerlessNotebook, len(listed))
	for i, o := range listed {
		a := activity[o.Notebook.ID]
		out[i] = OwnerlessNotebook{
			Notebook: o.Notebook, MemberCount: o.MemberCount, FormerOwner: profiles[o.Notebook.Ownerless.FormerOwner],
			SizeBytes: a.Bytes, LastActivityAt: *a.LastWriteAt,
		}
	}
	return out, nil
}
