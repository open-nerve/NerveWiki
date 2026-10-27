package httpadapter

import (
	"context"
	"strconv"
	"time"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apigen"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// server implements gen.StrictServerInterface: it only translates between
// the generated types and the use cases.
type server struct {
	uc UseCases
}

// StartExport serves POST /api/v0/notebooks/{notebook_id}/exports.
func (s server) StartExport(ctx context.Context, req gen.StartExportRequestObject) (gen.StartExportResponseObject, error) {
	var root *uuid.UUID
	if req.Body.RootID.IsSpecified() && !req.Body.RootID.IsNull() {
		id := req.Body.RootID.MustGet()
		root = &id
	}
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return nil, err
	}
	j, err := s.uc.Start.Run(ctx, req.NotebookID, root, domain.Client(client))
	if err != nil {
		return nil, err
	}
	return gen.StartExport202JSONResponse(jobOf(j)), nil
}

// StartTrashRestore serves POST /api/v0/notebooks/{notebook_id}/trash-restores.
func (s server) StartTrashRestore(ctx context.Context, req gen.StartTrashRestoreRequestObject) (gen.StartTrashRestoreResponseObject, error) {
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return nil, err
	}
	j, err := s.uc.Restore.Execute(ctx, app.RestoreRequest{NotebookID: req.NotebookID, NodeID: req.Body.NodeID, Client: domain.Client(client)})
	if err != nil {
		return nil, err
	}
	return gen.StartTrashRestore202JSONResponse(jobOf(j)), nil
}

// StartRevert serves POST /api/v0/notebooks/{notebook_id}/reverts.
func (s server) StartRevert(ctx context.Context, req gen.StartRevertRequestObject) (gen.StartRevertResponseObject, error) {
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return nil, err
	}
	j, err := s.uc.Revert.Execute(ctx, app.RevertRequest{NotebookID: req.NotebookID, ChangesetID: req.Body.ChangesetID,
		Message: req.Body.Message, Client: domain.Client(client)})
	if err != nil {
		return nil, err
	}
	return gen.StartRevert202JSONResponse(jobOf(j)), nil
}

// ListTransferJobs serves GET /api/v0/notebooks/{notebook_id}/transfer-jobs.
func (s server) ListTransferJobs(ctx context.Context, req gen.ListTransferJobsRequestObject) (gen.ListTransferJobsResponseObject, error) {
	page, err := s.uc.Reads.List(ctx, req.NotebookID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListTransferJobs200JSONResponse{Data: make([]gen.TransferJob, len(page.Jobs)), NextCursor: nullable.NewNullNullable[string]()}
	if page.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(page.NextCursor)
	}
	for i, j := range page.Jobs {
		out.Data[i] = jobOf(j)
	}
	return out, nil
}

// GetTransferJob serves GET /api/v0/transfer-jobs/{job_id}.
func (s server) GetTransferJob(ctx context.Context, req gen.GetTransferJobRequestObject) (gen.GetTransferJobResponseObject, error) {
	j, err := s.uc.Reads.Get(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	return gen.GetTransferJob200JSONResponse(detailOf(j)), nil
}

// CancelTransferJob serves POST /api/v0/transfer-jobs/{job_id}/cancel.
func (s server) CancelTransferJob(ctx context.Context, req gen.CancelTransferJobRequestObject) (gen.CancelTransferJobResponseObject, error) {
	j, err := s.uc.Cancel.Run(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	return gen.CancelTransferJob200JSONResponse(jobOf(j)), nil
}

// DownloadURL is the address of the job id's archive signed as s, which
// the download reads back.
func DownloadURL(id uuid.UUID, s app.Signed) string {
	return "/api/v0/transfer-jobs/" + id.String() + "/download?e=" + strconv.FormatInt(s.Expires.Unix(), 10) + "&s=" + s.Signature
}

// jobOf is a job's view as the API answers it.
func jobOf(v app.JobView) gen.TransferJob {
	j := v.Job
	return gen.TransferJob{
		ID: j.ID, NotebookID: j.NotebookID, RootID: maybe(j.RootID), Name: j.Name, ChangesetID: maybe(j.ChangesetID),
		TargetChangesetID: maybe(j.TargetChangesetID), Kind: gen.TransferKind(j.Kind), State: gen.TransferState(j.State),
		Client: gen.TransferClient(j.Client), CreatedBy: apigen.UserRef{UserID: j.CreatedBy, DisplayName: v.CreatedByName},
		CreatedAt: j.CreatedAt, StartedAt: maybe(j.Started), CancelRequestedAt: maybe(j.CancelRequested), FinishedAt: maybe(j.Finished),
		Progress: gen.TransferProgress{Done: j.Progress.Done, Total: j.Progress.Total}, ResultBytes: maybe(j.ResultBytes),
		Report: reportOf(j.Report, v.LockedByName), Download: downloadOf(j.ID, v.Download),
	}
}

// detailOf is a job's view with its report's problems and conflicts.
func detailOf(v app.JobView) gen.TransferJobDetail {
	j := jobOf(v)
	out := gen.TransferJobDetail{
		ID: j.ID, NotebookID: j.NotebookID, RootID: j.RootID, Name: j.Name, ChangesetID: j.ChangesetID, TargetChangesetID: j.TargetChangesetID,
		Kind: j.Kind, State: j.State, Client: j.Client, CreatedBy: j.CreatedBy, CreatedAt: j.CreatedAt, StartedAt: j.StartedAt,
		CancelRequestedAt: j.CancelRequestedAt, FinishedAt: j.FinishedAt, Progress: j.Progress, ResultBytes: j.ResultBytes, Report: j.Report,
		Download: j.Download, Problems: []gen.TransferProblem{}, Conflicts: []apigen.RevertConflict{},
	}
	if r := v.Job.Report; r != nil {
		out.ProblemsTruncated, out.ConflictCount = r.Truncated, r.ConflictCount
		for _, c := range r.Conflicts {
			out.Conflicts = append(out.Conflicts, apigen.RevertConflict{NodeID: c.NodeID, Name: c.Name,
				Reason: apigen.RevertConflictReason(c.Reason), HolderID: c.HolderID})
		}
		for _, p := range r.Problems {
			to := nullable.NewNullNullable[string]()
			if p.To != "" {
				to = nullable.NewNullableWithValue(p.To)
			}
			out.Problems = append(out.Problems, gen.TransferProblem{Path: p.Path, Code: gen.TransferProblemCode(p.Code), To: to})
		}
	}
	return out
}

// reportOf is a job's report as the API answers it: the holder of the
// lock it met by lockedBy, their name.
func reportOf(r *domain.Report, lockedBy string) nullable.Nullable[gen.TransferReport] {
	if r == nil {
		return nullable.NewNullNullable[gen.TransferReport]()
	}
	c := r.Counts
	out := gen.TransferReport{
		Counts: gen.TransferCounts{Pages: c.Pages, Attachments: c.Attachments, Renamed: c.Renamed, Missing: c.Missing, Skipped: c.Skipped,
			Restored: c.Restored, Deleted: c.Deleted, PutBack: c.PutBack, Written: c.Written},
		Failure: nullable.NewNullNullable[gen.TransferFailure](),
		Placed:  nullable.NewNullNullable[apigen.TrashPlacement](),
		Lock:    nullable.NewNullNullable[apigen.PageLock](),
		Refusal: nullable.NewNullNullable[gen.TransferRefusal](),
	}
	if r.Failure != "" {
		out.Failure = nullable.NewNullableWithValue(gen.TransferFailure(r.Failure))
	}
	if p := r.Placed; p != nil {
		out.Placed = nullable.NewNullableWithValue(apigen.TrashPlacement{ParentID: maybe(p.ParentID), Reason: apigen.TrashPlacementReason(p.Reason)})
	}
	if l := r.Lock; l != nil {
		out.Lock = nullable.NewNullableWithValue(apigen.PageLock{PageID: l.PageID, UserID: l.UserID, DisplayName: lockedBy})
	}
	if f := r.Refusal; f != nil {
		out.Refusal = nullable.NewNullableWithValue(gen.TransferRefusal{Code: f.Code, NodeID: f.NodeID, DuplicateOf: maybe(f.DuplicateOf)})
	}
	return nullable.NewNullableWithValue(out)
}

func downloadOf(id uuid.UUID, s *app.Signed) nullable.Nullable[gen.TransferDownload] {
	if s == nil {
		return nullable.NewNullNullable[gen.TransferDownload]()
	}
	return nullable.NewNullableWithValue(gen.TransferDownload{URL: DownloadURL(id, *s), ExpiresAt: s.Expires})
}

// maybe is v, or null for nil.
func maybe[T uuid.UUID | time.Time | int64](v *T) nullable.Nullable[T] {
	if v == nil {
		return nullable.NewNullNullable[T]()
	}
	return nullable.NewNullableWithValue(*v)
}
