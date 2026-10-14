package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apigen"
)

// ListChangesetsUseCase is app.ListChangesets.
type ListChangesetsUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, q app.LogQuery) (app.ChangesetList, error)
}

// GetChangesetUseCase is app.GetChangeset.
type GetChangesetUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.ChangesetDetail, error)
}

// ListChangesUseCase is app.ListChanges.
type ListChangesUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (app.ChangeList, error)
}

// RevertChangesetUseCase is app.RevertChangeset.
type RevertChangesetUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, message *string, client domain.Client) (app.ChangesetReverted, error)
}

// ListChangesets serves GET /api/v0/notebooks/{notebook_id}/changesets.
func (h handler) ListChangesets(ctx context.Context, req gen.ListChangesetsRequestObject) (gen.ListChangesetsResponseObject, error) {
	p := req.Params
	l, err := h.uc.ListChangesets.Execute(ctx, req.NotebookID, app.LogQuery{Actor: p.Actor, Client: p.Client, PageID: p.PageID,
		Limit: p.Limit, Cursor: p.Cursor})
	if err != nil {
		return nil, err
	}
	out := gen.ListChangesets200JSONResponse{Data: make([]gen.Changeset, len(l.Changesets)), NextCursor: nullable.NewNullNullable[string]()}
	for i, c := range l.Changesets {
		out.Data[i] = changesetOf(c)
	}
	if l.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(l.NextCursor)
	}
	return out, nil
}

// GetChangeset serves GET /api/v0/changesets/{changeset_id}.
func (h handler) GetChangeset(ctx context.Context, req gen.GetChangesetRequestObject) (gen.GetChangesetResponseObject, error) {
	d, err := h.uc.GetChangeset.Execute(ctx, req.ChangesetID)
	if err != nil {
		return nil, err
	}
	c := changesetOf(d.LoggedChangeset)
	return gen.GetChangeset200JSONResponse{
		ID: c.ID, NotebookID: c.NotebookID, Kind: c.Kind, Client: c.Client, Message: c.Message, CreatedBy: c.CreatedBy,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Reverts: c.Reverts, RevertedBy: c.RevertedBy, Trimmed: c.Trimmed,
		NodeCount: c.NodeCount, ContentCount: c.ContentCount, Changes: c.Changes, Renames: d.Moves.Renames, Moves: d.Moves.Moves,
	}, nil
}

// ListChangesetChanges serves GET /api/v0/changesets/{changeset_id}/changes.
func (h handler) ListChangesetChanges(ctx context.Context, req gen.ListChangesetChangesRequestObject) (
	gen.ListChangesetChangesResponseObject, error,
) {
	l, err := h.uc.ListChanges.Execute(ctx, req.ChangesetID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListChangesetChanges200JSONResponse{Data: make([]gen.ChangesetChange, len(l.Changes)),
		NextCursor: nullable.NewNullNullable[string]()}
	for i, c := range l.Changes {
		out.Data[i] = changeOf(c)
	}
	if l.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(l.NextCursor)
	}
	return out, nil
}

// RevertChangeset serves POST /api/v0/changesets/{changeset_id}/revert. An
// empty request has no message.
func (h handler) RevertChangeset(ctx context.Context, req gen.RevertChangesetRequestObject) (gen.RevertChangesetResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	var message *string
	if req.Body != nil {
		message = req.Body.Message
	}
	r, err := h.uc.RevertChangeset.Execute(ctx, req.ChangesetID, message, client)
	if err != nil {
		return nil, err
	}
	out := gen.RevertChangeset200JSONResponse{ChangesetID: nullable.NewNullNullable[uuid.UUID](), Changed: r.Changed}
	if r.ChangesetID != (uuid.UUID{}) {
		out.ChangesetID = nullable.NewNullableWithValue(r.ChangesetID)
	}
	return out, nil
}

func changesetOf(l app.LoggedChangeset) gen.Changeset {
	c := l.Changeset
	out := gen.Changeset{
		ID: c.ID, NotebookID: c.NotebookID, Kind: gen.ChangesetKind(c.Kind), Client: string(c.Client),
		Message: nullable.NewNullNullable[string](), CreatedBy: apigen.UserRef{UserID: c.By, DisplayName: c.ByName}, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt, Reverts: nullable.NewNullNullable[gen.ChangesetBrief](), RevertedBy: append([]uuid.UUID{}, l.Facts.RevertedBy...),
		Trimmed: c.Trimmed, NodeCount: l.Facts.Nodes, ContentCount: l.Facts.Contents, Changes: make([]gen.ChangesetChange, len(l.Changes)),
	}
	if c.Message != nil {
		out.Message = nullable.NewNullableWithValue(*c.Message)
	}
	if r := c.Reverts; r != nil {
		out.Reverts = nullable.NewNullableWithValue(gen.ChangesetBrief{ID: r.ID, Kind: gen.ChangesetKind(r.Kind),
			CreatedBy: apigen.UserRef{UserID: r.By, DisplayName: r.ByName}, CreatedAt: r.CreatedAt})
	}
	for i, ch := range l.Changes {
		out.Changes[i] = changeOf(ch)
	}
	return out
}

func changeOf(r domain.ChangeRow) gen.ChangesetChange {
	state := func(p *domain.ChangePlace) nullable.Nullable[gen.ChangeState] {
		if p == nil {
			return nullable.NewNullNullable[gen.ChangeState]()
		}
		s := gen.ChangeState{ParentID: nullable.NewNullNullable[uuid.UUID](), ParentName: nullable.NewNullNullable[string](), Name: p.Name}
		if p.ParentID != nil {
			s.ParentID = nullable.NewNullableWithValue(*p.ParentID)
		}
		if p.ParentName != nil {
			s.ParentName = nullable.NewNullableWithValue(*p.ParentName)
		}
		return nullable.NewNullableWithValue(s)
	}
	out := gen.ChangesetChange{
		Node: gen.ChangeNode{ID: r.NodeID, Kind: gen.NodeKind(r.NodeKind), Name: r.Name, Alive: r.Alive},
		Kind: gen.ChangesetChangeKind(r.Change()), Before: state(r.Before), After: state(r.After),
		Content: nullable.NewNullNullable[gen.ChangeContent](),
	}
	if v := r.Content; v != nil {
		base := nullable.NewNullNullable[int]()
		if v.Base != nil {
			base = nullable.NewNullableWithValue(*v.Base)
		}
		out.Content = nullable.NewNullableWithValue(gen.ChangeContent{BaseRevision: base, Revision: v.Revision})
	}
	return out
}
