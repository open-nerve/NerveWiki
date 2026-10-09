package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// JobView is a job as its readers see it: who started it, by name, and an
// export's archive's address, signed, once it succeeded.
type JobView struct {
	Job           domain.Job
	CreatedByName string
	Download      *Signed
}

// views makes the jobs' views and decides who sees a job.
type views struct {
	auth      shared.Authorizer
	notebooks Notebooks
	names     Names
	signer    Signer
	clock     Clock
	// ttl is transfer.export_ttl.
	ttl time.Duration
}

// of are jobs' views, their addresses signed as of one time. An address
// expires with its signature, or with its export, ttl after it ended,
// whichever comes first; an export past it is expired already, its row
// expired by the next expiry.
func (v views) of(ctx context.Context, jobs []domain.Job) ([]JobView, error) {
	ids := make([]uuid.UUID, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, j.CreatedBy)
	}
	names, err := v.names.DisplayNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := v.clock.Now()
	out := make([]JobView, len(jobs))
	for i, j := range jobs {
		out[i] = JobView{Job: j, CreatedByName: names[j.CreatedBy]}
		if j.Kind != domain.KindExport || j.State != domain.StateSucceeded || j.Finished == nil {
			continue
		}
		until := j.Finished.Add(v.ttl)
		if !now.Before(until) {
			out[i].Job.State = domain.StateExpired
			continue
		}
		signed := v.signer.Sign(now, j.ID)
		if until.Before(signed.Expires) {
			signed.Expires = until
		}
		out[i].Download = &signed
	}
	return out, nil
}

// decide decides action on the notebook notebookID for actor, and tells
// whether the actor is its admin: notFound for a notebook that does not
// exist, is deleted, or that the caller cannot see.
func (v views) decide(ctx context.Context, actor shared.Actor, action shared.Action, notebookID uuid.UUID, notFound error) (bool, error) {
	workspaceID, ok, err := v.notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return false, err
	case !ok:
		return false, notFound
	}
	grant, err := v.auth.Authorize(ctx, actor, action, shared.Target{WorkspaceID: workspaceID, NotebookID: notebookID})
	if errors.Is(err, shared.ErrNotVisible) {
		return false, notFound
	}
	if err != nil {
		return false, err
	}
	return grant.NotebookRole == shared.NotebookAdmin, nil
}

// sees decides action on job j for actor: transfer.not_found for a job
// of a notebook the caller cannot see, or another's when the caller is not
// the notebook's admin (M7 design 4.14).
func (v views) sees(ctx context.Context, actor shared.Actor, action shared.Action, j domain.Job) error {
	admin, err := v.decide(ctx, actor, action, j.NotebookID, domain.ErrNotFound)
	if err != nil {
		return err
	}
	if j.CreatedBy != actor.UserID && !admin {
		return domain.ErrNotFound
	}
	return nil
}
