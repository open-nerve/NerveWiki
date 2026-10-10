// Package postgresadapter keeps the transfer module's rows in PostgreSQL,
// through the queries sqlc generates from queries/ into gen.
package postgresadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Store is the transfer module's repository.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// queries runs in the context's transaction when there is one.
func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// The unique indexes of one export queued or running for a reader in a
// notebook, and of one import in a notebook.
const (
	exportingKey = "transfer_jobs_exporting_key"
	importingKey = "transfer_jobs_importing_key"
)

// CreateJob implements app.Rows: a second export queued or running for the
// account in the notebook is domain.ErrBusy, a second import in the
// notebook domain.ErrImportBusy, whatever the count before said.
func (s *Store) CreateJob(ctx context.Context, j domain.Job) error {
	err := s.queries(ctx).CreateJob(ctx, gen.CreateJobParams{
		ID: j.ID, NotebookID: j.NotebookID, RootID: j.RootID, Kind: string(j.Kind), Name: j.Name, CreatedByID: j.CreatedBy,
		Client: string(j.Client), CreatedAt: j.CreatedAt,
	})
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == exportingKey:
		return domain.ErrBusy
	case errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == importingKey:
		return domain.ErrImportBusy
	}
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

// LockQueue implements app.Rows. On the pool the lock would end with the
// statement, so it is refused there.
func (s *Store) LockQueue(ctx context.Context) error {
	if !postgres.InTx(ctx) {
		return errors.New("lock the jobs' queue: not in a transaction, the lock would end with the statement")
	}
	if err := s.queries(ctx).LockQueue(ctx); err != nil {
		return fmt.Errorf("lock the jobs' queue: %w", err)
	}
	return nil
}

// CountActive implements app.Rows.
func (s *Store) CountActive(ctx context.Context) (int, error) {
	n, err := s.queries(ctx).CountActive(ctx)
	if err != nil {
		return 0, fmt.Errorf("count the active jobs: %w", err)
	}
	return int(n), nil
}

// Exporting implements app.Rows.
func (s *Store) Exporting(ctx context.Context, notebookID, userID uuid.UUID) (bool, error) {
	ok, err := s.queries(ctx).Exporting(ctx, gen.ExportingParams{NotebookID: notebookID, CreatedByID: userID})
	if err != nil {
		return false, fmt.Errorf("exporting: %w", err)
	}
	return ok, nil
}

// Importing implements app.Rows.
func (s *Store) Importing(ctx context.Context, notebookID uuid.UUID) (bool, error) {
	ok, err := s.queries(ctx).Importing(ctx, notebookID)
	if err != nil {
		return false, fmt.Errorf("importing: %w", err)
	}
	return ok, nil
}

// FindJob implements app.Rows.
func (s *Store) FindJob(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	r, err := s.queries(ctx).FindJob(ctx, id)
	if err != nil {
		return domain.Job{}, noRow("find job", err)
	}
	return jobOf(r)
}

// LockJob implements app.Rows. On the pool the lock would end with the
// statement, so it is refused there.
func (s *Store) LockJob(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	if !postgres.InTx(ctx) {
		return domain.Job{}, errors.New("lock job: not in a transaction, the lock would end with the statement")
	}
	r, err := s.queries(ctx).LockJob(ctx, id)
	if err != nil {
		return domain.Job{}, noRow("lock job", err)
	}
	return jobOf(gen.FindJobRow(r))
}

// ListJobs implements app.Rows: the reports come without their problems.
func (s *Store) ListJobs(ctx context.Context, notebookID uuid.UUID, by *uuid.UUID, after *app.Cursor, limit int) ([]domain.Job, error) {
	p := gen.ListJobsParams{NotebookID: notebookID, CreatedByID: by, RowLimit: int32(limit)} //nolint:gosec // a page's size
	if after != nil {
		p.AfterCreatedAt, p.AfterID = &after.CreatedAt, &after.ID
	}
	rows, err := s.queries(ctx).ListJobs(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	out := make([]domain.Job, len(rows))
	for i, r := range rows {
		if out[i], err = jobOf(gen.FindJobRow(r)); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// StartJob implements app.Rows.
func (s *Store) StartJob(ctx context.Context, id uuid.UUID, at time.Time) (domain.Job, error) {
	r, err := s.queries(ctx).StartJob(ctx, gen.StartJobParams{ID: id, At: &at})
	if err != nil {
		return domain.Job{}, noRow("start job", err)
	}
	return jobOf(gen.FindJobRow(r))
}

// BeatJob implements app.Rows.
func (s *Store) BeatJob(ctx context.Context, id uuid.UUID, at time.Time, p domain.Progress, report *domain.Report) (app.Beat, error) {
	var b []byte
	if report != nil {
		var err error
		if b, err = encodeReport(*report); err != nil {
			return app.Beat{}, err
		}
	}
	r, err := s.queries(ctx).BeatJob(ctx, gen.BeatJobParams{ID: id, At: &at, Done: p.Done, Total: p.Total, Report: b})
	if err != nil {
		return app.Beat{}, noRow("beat job", err)
	}
	return app.Beat{CancelRequested: r.CancelRequested, Deleted: r.Deleted}, nil
}

// FinishJob implements app.Rows.
func (s *Store) FinishJob(ctx context.Context, id uuid.UUID, e app.Ended) (bool, error) {
	report, err := encodeReport(e.Report)
	if err != nil {
		return false, err
	}
	n, err := s.queries(ctx).FinishJob(ctx, gen.FinishJobParams{
		ID: id, State: string(e.State), At: &e.At, Report: report, ResultBytes: e.ResultBytes, Name: e.Name,
		Done: e.Progress.Done, Total: e.Progress.Total,
	})
	if err != nil {
		return false, fmt.Errorf("finish job: %w", err)
	}
	return n == 1, nil
}

// ExpireOthers implements app.Rows.
func (s *Store) ExpireOthers(ctx context.Context, notebookID, userID, id uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).ExpireOthers(ctx, gen.ExpireOthersParams{NotebookID: notebookID, CreatedByID: userID, ID: id})
	if err != nil {
		return nil, fmt.Errorf("expire the other exports: %w", err)
	}
	return ids, nil
}

// CancelQueued implements app.Rows.
func (s *Store) CancelQueued(ctx context.Context, id uuid.UUID, at time.Time, r domain.Report) (bool, error) {
	report, err := encodeReport(r)
	if err != nil {
		return false, err
	}
	n, err := s.queries(ctx).CancelQueued(ctx, gen.CancelQueuedParams{ID: id, At: &at, Report: report})
	if err != nil {
		return false, fmt.Errorf("cancel the queued job: %w", err)
	}
	return n == 1, nil
}

// RequestCancel implements app.Rows.
func (s *Store) RequestCancel(ctx context.Context, id uuid.UUID, at time.Time) (bool, error) {
	n, err := s.queries(ctx).RequestCancel(ctx, gen.RequestCancelParams{ID: id, At: &at})
	if err != nil {
		return false, fmt.Errorf("request the job's cancel: %w", err)
	}
	return n == 1, nil
}

// ExpireExports implements app.MaintainedRows. It locks the rows for the
// statement alone: each is expired by it.
func (s *Store) ExpireExports(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).ExpireExports(ctx, gen.ExpireExportsParams{Before: before, Batch: int32(batch)}) //nolint:gosec // a batch
	if err != nil {
		return nil, fmt.Errorf("expire exports: %w", err)
	}
	return ids, nil
}

// InterruptJobs implements app.MaintainedRows.
func (s *Store) InterruptJobs(ctx context.Context, beatBefore *time.Time, at time.Time, r domain.Report) ([]app.Interrupted, error) {
	report, err := encodeReport(r)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries(ctx).InterruptJobs(ctx, gen.InterruptJobsParams{At: &at, Report: report, BeatBefore: beatBefore})
	if err != nil {
		return nil, fmt.Errorf("interrupt jobs: %w", err)
	}
	out := make([]app.Interrupted, len(rows))
	for i, x := range rows {
		out[i] = app.Interrupted{ID: x.ID, NotebookID: x.NotebookID, CreatedBy: x.CreatedByID, Client: domain.Client(x.Client)}
	}
	return out, nil
}

// QueuedJobs implements app.MaintainedRows.
func (s *Store) QueuedJobs(ctx context.Context, kind domain.Kind) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).QueuedJobs(ctx, string(kind))
	if err != nil {
		return nil, fmt.Errorf("queued jobs: %w", err)
	}
	return ids, nil
}

// FailQueued implements app.MaintainedRows.
func (s *Store) FailQueued(ctx context.Context, ids []uuid.UUID, at time.Time, r domain.Report) ([]app.Interrupted, error) {
	report, err := encodeReport(r)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries(ctx).FailQueued(ctx, gen.FailQueuedParams{At: &at, Report: report, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("fail the queued jobs: %w", err)
	}
	out := make([]app.Interrupted, len(rows))
	for i, x := range rows {
		out[i] = app.Interrupted{ID: x.ID, NotebookID: x.NotebookID, CreatedBy: x.CreatedByID, Client: domain.Client(x.Client)}
	}
	return out, nil
}

// LiveArchives implements app.MaintainedRows.
func (s *Store) LiveArchives(ctx context.Context, kind domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error) {
	live, err := s.queries(ctx).LiveArchives(ctx, gen.LiveArchivesParams{Ids: ids, Kind: string(kind)})
	if err != nil {
		return nil, fmt.Errorf("live archives: %w", err)
	}
	return live, nil
}

// DeleteJobsOfNotebooks implements app.MaintainedRows.
func (s *Store) DeleteJobsOfNotebooks(ctx context.Context, notebookIDs []uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteJobsOfNotebooks(ctx, gen.DeleteJobsOfNotebooksParams{NotebookIds: notebookIDs, At: &at}); err != nil {
		return fmt.Errorf("delete the notebooks' jobs: %w", err)
	}
	return nil
}

// ExpiredJobs implements app.MaintainedRows. It locks the rows for the
// batch's transaction: on the pool the locks would end with the
// statement, and two purges could take the same rows, so it is refused
// there.
func (s *Store) ExpiredJobs(ctx context.Context, before time.Time, batch int) (map[uuid.UUID]domain.Kind, error) {
	if !postgres.InTx(ctx) {
		return nil, errors.New("expired jobs: not in a transaction, the rows' locks would end with the statement")
	}
	rows, err := s.queries(ctx).ExpiredJobs(ctx, gen.ExpiredJobsParams{Before: before, Batch: int32(batch)}) //nolint:gosec // a batch
	if err != nil {
		return nil, fmt.Errorf("expired jobs: %w", err)
	}
	out := make(map[uuid.UUID]domain.Kind, len(rows))
	for _, r := range rows {
		out[r.ID] = domain.Kind(r.Kind)
	}
	return out, nil
}

// DeleteJobs implements app.MaintainedRows.
func (s *Store) DeleteJobs(ctx context.Context, ids []uuid.UUID) (int, error) {
	n, err := s.queries(ctx).DeleteJobs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("delete jobs: %w", err)
	}
	return int(n), nil
}

// noRow is err of what, pgx's no row as app.ErrNoRow.
func noRow(what string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNoRow
	}
	return fmt.Errorf("%s: %w", what, err)
}

// jobOf is a row as the domain reads it.
func jobOf(r gen.FindJobRow) (domain.Job, error) {
	j := domain.Job{
		ID: r.ID, NotebookID: r.NotebookID, RootID: r.RootID, Kind: domain.Kind(r.Kind), State: domain.State(r.State), Name: r.Name,
		CreatedBy: r.CreatedByID, Client: domain.Client(r.Client), Progress: domain.Progress{Done: r.ProgressDone, Total: r.ProgressTotal},
		CancelRequested: r.CancelRequestedAt, Heartbeat: r.HeartbeatAt, Started: r.StartedAt, Finished: r.FinishedAt,
		ResultBytes: r.ResultBytes, CreatedAt: r.CreatedAt,
	}
	// A running job's report is the heartbeat's, which only its end shows.
	if r.Report != nil && j.State.Ended() {
		report, err := decodeReport(r.Report)
		if err != nil {
			return domain.Job{}, fmt.Errorf("the report of job %s: %w", r.ID, err)
		}
		j.Report = &report
	}
	return j, nil
}

// reportJSON is a report as transfer_jobs.report keeps it.
type reportJSON struct {
	Failure   *string       `json:"failure"`
	Counts    countsJSON    `json:"counts"`
	Problems  []problemJSON `json:"problems"`
	Truncated bool          `json:"problems_truncated"`
}

type countsJSON struct {
	Pages       int64 `json:"pages"`
	Attachments int64 `json:"attachments"`
	Renamed     int64 `json:"renamed"`
	Missing     int64 `json:"missing"`
	Skipped     int64 `json:"skipped"`
}

type problemJSON struct {
	Path string `json:"path"`
	Code string `json:"code"`
	To   string `json:"to,omitempty"`
}

func encodeReport(r domain.Report) ([]byte, error) {
	out := reportJSON{Counts: countsJSON(r.Counts), Problems: make([]problemJSON, len(r.Problems)), Truncated: r.Truncated}
	if r.Failure != "" {
		f := string(r.Failure)
		out.Failure = &f
	}
	for i, p := range r.Problems {
		out.Problems[i] = problemJSON{Path: p.Path, Code: string(p.Code), To: p.To}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode the report: %w", err)
	}
	return b, nil
}

func decodeReport(b []byte) (domain.Report, error) {
	var in reportJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return domain.Report{}, err
	}
	r := domain.Report{Counts: domain.Counts(in.Counts), Truncated: in.Truncated}
	if in.Failure != nil {
		r.Failure = domain.Failure(*in.Failure)
	}
	for _, p := range in.Problems {
		r.Problems = append(r.Problems, domain.Problem{Path: p.Path, Code: domain.ProblemCode(p.Code), To: p.To})
	}
	return r, nil
}
