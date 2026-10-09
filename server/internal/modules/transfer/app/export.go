package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Export runs an export's job (M7/P5 design 3.8): River's worker calls it
// with the job's id.
type Export struct {
	d ExportDeps
}

// ExportDeps are what Export needs.
type ExportDeps struct {
	Tx           shared.TxManager
	Snapshots    shared.Snapshots
	Authorizer   shared.Authorizer
	Notebooks    Notebooks
	Nodes        Nodes
	Linked       Linked
	Blobs        Blobs
	Archives     Archives
	Rows         Rows
	Contributors []Contributor
	Clock        Clock
	Logger       *slog.Logger
	// Beat is how often a running job writes its heartbeat and progress,
	// and reads whether it was cancelled: a second.
	Beat time.Duration
}

// NewExport returns the use case.
func NewExport(d ExportDeps) *Export {
	return &Export{d: d}
}

// contentsBatch is how many pages' contents one read of the snapshot asks
// for.
const contentsBatch = 200

// The finish's bounds: a job ending as the server stops has until River's
// grace after the shutdown's timeout; any other, long enough.
const (
	finishStopping = 900 * time.Millisecond
	finishWait     = 30 * time.Second
)

// The causes a running job's heartbeat stops it with.
var (
	errCancelRequested = errors.New("transfer: the job's cancel was asked")
	errGone            = errors.New("transfer: the job was deleted, or no longer runs")
)

// failure is an export's failure as its report tells it, and the error
// behind it, which is logged.
type failure struct {
	code domain.Failure
	err  error
}

func (f *failure) Error() string { return string(f.code) + ": " + f.err.Error() }
func (f *failure) Unwrap() error { return f.err }

func failed(code domain.Failure, err error) error {
	return &failure{code: code, err: err}
}

// Run runs the export id. A job no longer queued, cancelled or deleted
// meanwhile, is left as it is. A running one writes its heartbeat every
// Beat, which stops it when its cancel was asked or it was deleted; it
// ends succeeded, cancelled or failed with its report, its archive kept
// or dropped. Run fails only when the job's end cannot be written: the
// rescue fails it later.
func (e *Export) Run(ctx context.Context, id uuid.UUID) error {
	job, err := e.d.Rows.StartJob(ctx, id, e.d.Clock.Now())
	if errors.Is(err, ErrNoRow) {
		return nil
	}
	if err != nil {
		return err
	}
	attrs := []any{slog.String("job_id", job.ID.String()), slog.String("notebook_id", job.NotebookID.String()),
		slog.String("user_id", job.CreatedBy.String()), slog.String("client", string(job.Client))}
	e.d.Logger.InfoContext(ctx, "export started", attrs...)
	r := &run{e: e, job: job, report: domain.Report{}, name: job.Name}
	running, stop := context.WithCancelCause(ctx)
	beating := r.beat(running, stop)
	archive, err := r.write(running)
	stop(nil)
	<-beating
	return r.finish(ctx, running, archive, err, attrs)
}

// run is an export as it runs.
type run struct {
	e    *Export
	job  domain.Job
	done atomic.Int64
	all  atomic.Int64
	// report and name are the export's as it goes: its counts and problems,
	// and the archive's root folder's name once the snapshot is read.
	report domain.Report
	name   string
}

// progress is the run's progress now.
func (r *run) progress() domain.Progress {
	return domain.Progress{Done: r.done.Load(), Total: r.all.Load()}
}

// beat writes the job's heartbeat and progress every Beat until ctx ends,
// and stops the run when the job's cancel was asked, or it was deleted or
// no longer runs. A write that fails is logged, and tried again at the
// next beat: a job whose heartbeat stays old is failed by the rescue. The
// channel closes as it returns.
func (r *run) beat(ctx context.Context, stop context.CancelCauseFunc) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(r.e.d.Beat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			b, err := r.e.d.Rows.BeatJob(ctx, r.job.ID, r.e.d.Clock.Now(), r.progress())
			switch {
			case errors.Is(err, ErrNoRow) || err == nil && b.Deleted:
				stop(errGone)
			case err == nil && b.CancelRequested:
				stop(errCancelRequested)
			case err != nil && ctx.Err() == nil:
				r.e.d.Logger.WarnContext(ctx, "export heartbeat not written", slog.String("job_id", r.job.ID.String()), slog.Any("error", err))
			}
		}
	}()
	return done
}

// write writes the archive: the snapshot's pages and the contributors'
// files, then the attachments' files, then meta.json. It answers the
// archive, uncommitted, or the error that stopped it, the archive then
// dropped.
func (r *run) write(ctx context.Context) (Archive, error) {
	if err := r.authorize(ctx); err != nil {
		return nil, err
	}
	archive, err := r.e.d.Archives.Create(ctx, r.job.ID)
	if errors.Is(err, domain.ErrStorageFull) {
		return nil, failed(domain.FailureStorageFull, err)
	}
	if err != nil {
		return nil, err
	}
	err = r.fill(ctx, archive)
	if err != nil {
		if abortErr := archive.Abort(); abortErr != nil {
			r.e.d.Logger.WarnContext(ctx, "export archive not dropped", slog.String("job_id", r.job.ID.String()), slog.Any("error", abortErr))
		}
		return nil, err
	}
	return archive, nil
}

// authorize decides transfer.export again, as the job runs, for its
// starter acting through it: an account that can no longer read the
// notebook fails it. A notebook deleted meanwhile deleted the job too.
func (r *run) authorize(ctx context.Context) error {
	workspaceID, ok, err := r.e.d.Notebooks.WorkspaceOf(ctx, r.job.NotebookID)
	switch {
	case err != nil:
		return err
	case !ok:
		return errGone
	}
	actor := shared.Actor{UserID: r.job.CreatedBy, JobID: r.job.ID}
	_, err = r.e.d.Authorizer.Authorize(ctx, actor, domain.ActionExport, shared.Target{WorkspaceID: workspaceID, NotebookID: r.job.NotebookID})
	var denied *shared.Error
	if errors.Is(err, shared.ErrNotVisible) || errors.As(err, &denied) && denied.Kind == shared.KindForbidden {
		return failed(domain.FailureForbidden, err)
	}
	return err
}

// fill writes the archive's files.
func (r *run) fill(ctx context.Context, archive Archive) error {
	var plan *domain.Plan
	var notebook domain.Named
	var root *domain.Named
	var blobs map[uuid.UUID]uuid.UUID
	exportedAt := r.e.d.Clock.Now()
	err := r.e.d.Snapshots.WithinSnapshot(ctx, func(ctx context.Context) error {
		var err error
		if notebook, root, err = r.names(ctx); err != nil {
			return err
		}
		if plan, err = r.plan(ctx, notebook, root); err != nil {
			return err
		}
		r.name = plan.Root
		r.all.Store(int64(len(plan.Entries)))
		var assets []uuid.UUID
		for _, entry := range plan.Entries {
			if entry.Node.Asset {
				assets = append(assets, entry.Node.ID)
			}
		}
		if blobs, err = r.e.d.Blobs.Of(ctx, r.job.NotebookID, assets); err != nil {
			return err
		}
		if err := r.contribute(ctx, plan, archive, exportedAt); err != nil {
			return err
		}
		return r.pages(ctx, plan, archive)
	})
	if err != nil {
		return err
	}
	missing := map[uuid.UUID]bool{}
	for _, entry := range plan.Entries {
		if !entry.Node.Asset {
			continue
		}
		if err := r.attachment(ctx, plan, archive, entry, blobs); errors.Is(err, ErrFileMissing) {
			missing[entry.Node.ID] = true
		} else if err != nil {
			return err
		}
		r.done.Add(1)
	}
	meta, err := json.MarshalIndent(plan.Meta(exportedAt, notebook, root, func(e domain.Entry) bool { return !missing[e.Node.ID] }), "", "  ")
	if err != nil {
		return err
	}
	return add(archive, plan.Root+"/"+domain.MetaPath, exportedAt, false, bytes.NewReader(meta))
}

// names are the notebook's and the page's exported with its subtree, as
// the snapshot reads them: a page gone fails the export.
func (r *run) names(ctx context.Context) (domain.Named, *domain.Named, error) {
	name, ok, err := r.e.d.Notebooks.NameOf(ctx, r.job.NotebookID)
	switch {
	case err != nil:
		return domain.Named{}, nil, err
	case !ok:
		return domain.Named{}, nil, errGone
	}
	notebook := domain.Named{ID: r.job.NotebookID, Name: name}
	if r.job.RootID == nil {
		return notebook, nil, nil
	}
	page, ok, err := r.e.d.Nodes.Page(ctx, r.job.NotebookID, *r.job.RootID)
	switch {
	case err != nil:
		return domain.Named{}, nil, err
	case !ok:
		return domain.Named{}, nil, failed(domain.FailureRootNotFound, errors.New("the page exported is gone"))
	}
	return notebook, &domain.Named{ID: *r.job.RootID, Name: page}, nil
}

// plan maps the snapshot's scope: the pages without content and with
// children that a link of the scope leads to are files.
func (r *run) plan(ctx context.Context, notebook domain.Named, root *domain.Named) (*domain.Plan, error) {
	nodes, err := r.e.d.Nodes.Scope(ctx, r.job.NotebookID, r.job.RootID)
	if err != nil {
		return nil, err
	}
	if root != nil && len(nodes) == 0 {
		return nil, failed(domain.FailureRootNotFound, errors.New("the page exported is gone"))
	}
	parents := map[uuid.UUID]bool{}
	for _, n := range nodes {
		if n.ParentID != nil {
			parents[*n.ParentID] = true
		}
	}
	var sources, targets []uuid.UUID
	for _, n := range nodes {
		if n.Asset {
			continue
		}
		sources = append(sources, n.ID)
		if n.Empty && parents[n.ID] {
			targets = append(targets, n.ID)
		}
	}
	linked := map[uuid.UUID]bool{}
	if len(targets) > 0 {
		ids, err := r.e.d.Linked.Linked(ctx, r.job.NotebookID, sources, targets)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			linked[id] = true
		}
	}
	plan, err := domain.NewPlan(notebook, root, nodes, func(id uuid.UUID) bool { return linked[id] })
	if err != nil {
		return nil, err
	}
	for _, p := range plan.Renamed {
		r.report.Add(p)
	}
	r.report.Counts.Renamed = int64(len(plan.Renamed))
	return plan, nil
}

// contribute runs the contributors in their order, the first error
// stopping the export, and writes their files.
func (r *run) contribute(ctx context.Context, plan *domain.Plan, archive Archive, at time.Time) error {
	if len(r.e.d.Contributors) == 0 {
		return nil
	}
	scope := Scope{NotebookID: r.job.NotebookID, RootID: r.job.RootID, Nodes: make([]ScopeNode, len(plan.Entries))}
	for i, e := range plan.Entries {
		scope.Nodes[i] = ScopeNode{ID: e.Node.ID, Asset: e.Node.Asset, Path: e.Path}
	}
	sink := sink{plan: plan, archive: archive, at: at}
	for _, c := range r.e.d.Contributors {
		if err := c.Contribute(ctx, scope, sink); err != nil {
			if errors.Is(err, domain.ErrContributorConflict) {
				return failed(domain.FailureContributorConflict, err)
			}
			return fmt.Errorf("an export's contributor: %w", err)
		}
	}
	return nil
}

// sink is the contributors' Sink of a plan's archive.
type sink struct {
	plan    *domain.Plan
	archive Archive
	at      time.Time
}

func (s sink) Add(path string, content []byte) error {
	if err := s.plan.Contribute(path); err != nil {
		return err
	}
	return add(s.archive, s.plan.Root+"/"+path, s.at, domain.Stored(path), bytes.NewReader(content))
}

// pages writes the pages' files in the plan's order, contentsBatch pages
// a read of their contents; a page that is only its folder is a folder's
// entry.
func (r *run) pages(ctx context.Context, plan *domain.Plan, archive Archive) error {
	var batch []domain.Entry
	flush := func() error {
		var ids []uuid.UUID
		for _, e := range batch {
			if e.File != "" {
				ids = append(ids, e.Node.ID)
			}
		}
		contents, err := r.e.d.Nodes.Contents(ctx, ids)
		if err != nil {
			return err
		}
		for _, e := range batch {
			if e.File == "" {
				err = add(archive, plan.Root+"/"+e.Path, e.Node.Modified, true, nil)
			} else {
				err = add(archive, plan.Root+"/"+e.File, e.Node.Modified, false, strings.NewReader(contents[e.Node.ID]))
			}
			if err != nil {
				return err
			}
			r.done.Add(1)
		}
		batch = batch[:0]
		return nil
	}
	for _, e := range plan.Entries {
		if e.Node.Asset {
			continue
		}
		r.report.Counts.Pages++
		if batch = append(batch, e); len(batch) == contentsBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if len(batch) > 0 {
		return flush()
	}
	return nil
}

// attachment writes an attachment's file, its reads stopped with ctx; one
// without its file in the store is reported, ErrFileMissing.
func (r *run) attachment(ctx context.Context, plan *domain.Plan, archive Archive, e domain.Entry, blobs map[uuid.UUID]uuid.UUID) error {
	blob, ok := blobs[e.Node.ID]
	var file io.ReadCloser
	err := ErrFileMissing
	if ok {
		file, err = r.e.d.Blobs.Open(ctx, blob)
	}
	if errors.Is(err, ErrFileMissing) {
		r.report.Counts.Missing++
		r.report.Add(domain.Problem{Path: e.Path, Code: domain.ProblemFileMissing})
		return err
	}
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := add(archive, plan.Root+"/"+e.File, e.Node.Modified, domain.Stored(e.File), contextReader{ctx: ctx, r: file}); err != nil {
		return err
	}
	r.report.Counts.Attachments++
	return nil
}

// add adds a file to archive, a folder when r is nil: a store that runs out
// of room fails the export so.
func add(archive Archive, path string, modified time.Time, stored bool, r io.Reader) error {
	err := archive.Add(path, modified, stored, r)
	if errors.Is(err, domain.ErrStorageFull) {
		return failed(domain.FailureStorageFull, err)
	}
	return err
}

// contextReader reads r until ctx ends: a large file's copy stops with its
// job.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, context.Cause(c.ctx)
	}
	return c.r.Read(p)
}

// finish writes the run's end: archive committed and the job succeeded, the
// starter's earlier exports of the notebook expired and their archives
// deleted; or the job cancelled or failed, with what it did; nothing for a
// job deleted, or no longer running. River's context ends the job as a
// timeout or, as the server stops, an interruption.
func (r *run) finish(ctx, running context.Context, archive Archive, err error, attrs []any) error {
	wait := finishWait
	if ctx.Err() != nil {
		wait = finishStopping
	}
	end, cancel := context.WithTimeout(context.WithoutCancel(ctx), wait)
	defer cancel()
	e := Ended{State: domain.StateSucceeded, At: r.e.d.Clock.Now(), Name: r.name, Progress: r.progress()}
	if err == nil {
		bytes, commitErr := archive.Commit()
		if commitErr == nil {
			e.ResultBytes = &bytes
			return r.succeed(end, e, attrs)
		}
		err = commitErr
		if errors.Is(err, domain.ErrStorageFull) {
			err = failed(domain.FailureStorageFull, err)
		}
		r.drop(end)
	}
	cause := context.Cause(running)
	switch {
	case errors.Is(cause, errGone) || errors.Is(err, errGone):
		r.e.d.Logger.InfoContext(end, "export stopped: the job was deleted", attrs...)
		return nil
	case errors.Is(cause, errCancelRequested):
		e.State = domain.StateCancelled
	default:
		e.State, e.Report.Failure = domain.StateFailed, r.failureOf(ctx, err)
	}
	report := r.report
	report.Failure = e.Report.Failure
	e.Report = report
	ok, writeErr := r.e.d.Rows.FinishJob(end, r.job.ID, e)
	if writeErr != nil {
		return fmt.Errorf("write the end of export %s: %w", r.job.ID, writeErr)
	}
	if ok {
		level := slog.LevelInfo
		if e.Report.Failure == domain.FailureInternal {
			level = slog.LevelError
		}
		r.e.d.Logger.Log(end, level, "export ended", append(attrs, slog.String("state", string(e.State)),
			slog.String("failure", string(e.Report.Failure)), slog.Int64("nodes", e.Progress.Done), slog.Any("error", err))...)
	}
	return nil
}

// failureOf is why the run failed: River's context ended by its timeout or
// by the server's stop, or the run's own failure; any other is internal.
func (r *run) failureOf(ctx context.Context, err error) domain.Failure {
	var f *failure
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return domain.FailureTimeout
	case ctx.Err() != nil:
		return domain.FailureInterrupted
	case errors.As(err, &f):
		return f.code
	}
	return domain.FailureInternal
}

// succeed writes the job's success, and expires the starter's earlier
// exports of the notebook, whose archives it deletes once committed. A job
// deleted meanwhile, or no longer running, drops its archive.
func (r *run) succeed(ctx context.Context, e Ended, attrs []any) error {
	e.Report = r.report
	var expired []uuid.UUID
	ok := false
	err := r.e.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if ok, err = r.e.d.Rows.FinishJob(ctx, r.job.ID, e); err != nil || !ok {
			return err
		}
		expired, err = r.e.d.Rows.ExpireOthers(ctx, r.job.NotebookID, r.job.CreatedBy, r.job.ID)
		return err
	})
	if err != nil {
		return fmt.Errorf("write the success of export %s: %w", r.job.ID, err)
	}
	if !ok {
		r.drop(ctx)
		r.e.d.Logger.InfoContext(ctx, "export stopped: the job was deleted", attrs...)
		return nil
	}
	for _, id := range expired {
		if err := r.e.d.Archives.Delete(ctx, domain.KindExport, id); err != nil {
			r.e.d.Logger.WarnContext(ctx, "expired export's archive not deleted", slog.String("job_id", id.String()), slog.Any("error", err))
		}
	}
	r.e.d.Logger.InfoContext(ctx, "export ended", append(attrs, slog.String("state", string(domain.StateSucceeded)),
		slog.Int64("nodes", e.Progress.Done), slog.Int64("bytes", *e.ResultBytes))...)
	return nil
}

// drop deletes the run's archive, committed or not: the sweep deletes one
// it cannot.
func (r *run) drop(ctx context.Context) {
	if err := r.e.d.Archives.Delete(ctx, domain.KindExport, r.job.ID); err != nil {
		r.e.d.Logger.WarnContext(ctx, "export archive not deleted", slog.String("job_id", r.job.ID.String()), slog.Any("error", err))
	}
}
