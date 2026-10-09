package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Reads reads the jobs (M7/P5 design 3.11): each read decides again
// whether the caller reads the notebook, as a report names its pages.
type Reads struct {
	views views
	rows  Rows
}

// ReadsDeps are what Reads needs.
type ReadsDeps struct {
	Authorizer shared.Authorizer
	Notebooks  Notebooks
	Names      Names
	Signer     Signer
	Clock      Clock
	Rows       Rows
}

// NewReads returns the reads.
func NewReads(d ReadsDeps) *Reads {
	return &Reads{views: views{auth: d.Authorizer, notebooks: d.Notebooks, names: d.Names, signer: d.Signer, clock: d.Clock}, rows: d.Rows}
}

// JobPage is a page of a notebook's jobs, and the cursor of the next when
// another follows.
type JobPage struct {
	Jobs       []JobView
	NextCursor string
}

// Get reads the job id: transfer.not_found for none, a deleted one, one of
// a notebook the caller cannot read in (transfer.read), or another's when
// the caller is not the notebook's admin.
func (r *Reads) Get(ctx context.Context, id uuid.UUID) (JobView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return JobView{}, err
	}
	j, err := r.rows.FindJob(ctx, id)
	if errors.Is(err, ErrNoRow) {
		return JobView{}, domain.ErrNotFound
	}
	if err != nil {
		return JobView{}, err
	}
	if err := r.views.sees(ctx, actor, domain.ActionRead, j); err != nil {
		return JobView{}, err
	}
	got, err := r.views.of(ctx, []domain.Job{j})
	if err != nil {
		return JobView{}, err
	}
	return got[0], nil
}

// List lists the notebook's jobs, the newest first: the caller's, or every
// one for the notebook's admin; the page that limit and cursor ask for,
// from the first when cursor is nil. A cursor shared.DecodeCursor refuses
// is 400 bad_request, judged first; then the decision (transfer.read,
// notebook.not_found); then the limit, 422 outside 1–100.
func (r *Reads) List(ctx context.Context, notebookID uuid.UUID, limit *int, cursor *string) (JobPage, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return JobPage{}, err
	}
	var after *Cursor
	if cursor != nil {
		after = &Cursor{}
		if err := shared.DecodeCursor(*cursor, after); err != nil {
			return JobPage{}, err
		}
	}
	admin, err := r.views.decide(ctx, actor, domain.ActionRead, notebookID, domain.ErrNotebookNotFound)
	if err != nil {
		return JobPage{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return JobPage{}, err
	}
	var by *uuid.UUID
	if !admin {
		by = &actor.UserID
	}
	jobs, err := r.rows.ListJobs(ctx, notebookID, by, after, size+1)
	if err != nil {
		return JobPage{}, err
	}
	var out JobPage
	if len(jobs) > size {
		jobs = jobs[:size]
		last := jobs[size-1]
		if out.NextCursor, err = shared.EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID}); err != nil {
			return JobPage{}, err
		}
	}
	if out.Jobs, err = r.views.of(ctx, jobs); err != nil {
		return JobPage{}, err
	}
	return out, nil
}
