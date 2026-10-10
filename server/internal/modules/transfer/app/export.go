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

// The pages' contents one read of the snapshot asks for: contentsBatch
// pages, or contentsBytes of their contents, whichever comes first; a page
// larger than contentsBytes alone.
const (
	contentsBatch = 200
	contentsBytes = 16 << 20
)

// Run runs the export id. A job no longer queued, cancelled or deleted
// meanwhile, is left as it is. A running one writes its heartbeat every
// Beat until its archive is committed, which stops it when its cancel was
// asked or it was deleted; it ends succeeded, cancelled or failed with its
// report, its archive kept or dropped. Run fails only when the job's end
// cannot be written: the rescue fails it later.
func (e *Export) Run(ctx context.Context, id uuid.UUID) error {
	job, err := e.d.Rows.StartJob(ctx, id, e.d.Clock.Now())
	if errors.Is(err, ErrNoRow) {
		return nil
	}
	if err != nil {
		return err
	}
	attrs := jobAttrs(job)
	e.d.Logger.InfoContext(ctx, "export started", attrs...)
	r := &run{jobRun: newJobRun(job, e.d.Rows, e.d.Clock, e.d.Logger, e.d.Beat), e: e}
	running, stop := context.WithCancelCause(ctx)
	beating := r.beat(running, stop)
	size, err := r.write(running)
	stop(nil)
	<-beating
	return r.finish(ctx, running, err, attrs, func(end context.Context, e Ended) error {
		e.ResultBytes = &size
		return r.succeed(end, e, attrs)
	})
}

// run is an export as it runs: its name is the archive's root folder's
// once the snapshot is read.
type run struct {
	*jobRun
	e *Export
}

// write writes the archive and commits it: the snapshot's pages and the
// contributors' files, then the attachments' files, then meta.json. A stop
// that came as the last of them were written stops it before the commit.
// It answers the archive's bytes, or the error that stopped it, the
// archive then dropped: one that fails to commit may be at its key all the
// same, and is deleted. A file still being written that Abort cannot
// delete is left to the store's opening.
func (r *run) write(ctx context.Context) (int64, error) {
	if err := r.authorize(ctx); err != nil {
		return 0, err
	}
	archive, err := r.e.d.Archives.Create(ctx, r.job.ID)
	if err != nil {
		return 0, storageFull(err)
	}
	err = r.fill(ctx, archive)
	if err == nil && ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	if err != nil {
		if abortErr := archive.Abort(); abortErr != nil {
			r.e.d.Logger.WarnContext(ctx, "export archive not dropped", slog.String("job_id", r.job.ID.String()), slog.Any("error", abortErr))
		}
		return 0, err
	}
	size, err := archive.Commit()
	if err != nil {
		r.drop(context.WithoutCancel(ctx))
		return 0, storageFull(err)
	}
	return size, nil
}

// authorize decides transfer.export again, as the job runs.
func (r *run) authorize(ctx context.Context) error {
	return authorizeJob(ctx, r.e.d.Notebooks, r.e.d.Authorizer, r.job, domain.ActionExport)
}

// fill writes the archive's files.
func (r *run) fill(ctx context.Context, archive Archive) error {
	var plan *domain.Plan
	var notebook domain.Named
	var root *domain.Named
	var blobs map[uuid.UUID]Blob
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
		if n.Bytes == 0 && parents[n.ID] {
			targets = append(targets, n.ID)
		}
	}
	linked := map[uuid.UUID]bool{}
	if len(targets) > 0 {
		ids, err := r.e.d.Linked.Linked(ctx, sources, targets)
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
// or contentsBytes a read of their contents; a page that is only its
// folder is a folder's entry. A page counts once written.
func (r *run) pages(ctx context.Context, plan *domain.Plan, archive Archive) error {
	var batch []domain.Entry
	var size int64
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
			r.report.Counts.Pages++
			r.done.Add(1)
		}
		batch, size = batch[:0], 0
		return nil
	}
	for _, e := range plan.Entries {
		if e.Node.Asset {
			continue
		}
		if len(batch) == contentsBatch || len(batch) > 0 && size+e.Node.Bytes > contentsBytes {
			if err := flush(); err != nil {
				return err
			}
		}
		batch = append(batch, e)
		size += e.Node.Bytes
	}
	if len(batch) > 0 {
		return flush()
	}
	return nil
}

// attachment writes an attachment's file, modified as its blob was
// written, its reads stopped with ctx; one without its file in the store
// is reported, ErrFileMissing.
func (r *run) attachment(ctx context.Context, plan *domain.Plan, archive Archive, e domain.Entry, blobs map[uuid.UUID]Blob) error {
	blob, ok := blobs[e.Node.ID]
	var file io.ReadCloser
	err := ErrFileMissing
	if ok {
		file, err = r.e.d.Blobs.Open(ctx, blob.ID)
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
	if err := add(archive, plan.Root+"/"+e.File, blob.Created, domain.Stored(e.File), contextReader{ctx: ctx, r: file}); err != nil {
		return err
	}
	r.report.Counts.Attachments++
	return nil
}

// add adds a file to archive, a folder when r is nil: a store that runs out
// of room fails the export so.
func add(archive Archive, path string, modified time.Time, stored bool, r io.Reader) error {
	return storageFull(archive.Add(path, modified, stored, r))
}

// storageFull is err, the failure storage_full for a store out of room.
func storageFull(err error) error {
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

// succeed writes the job's success, and expires the starter's earlier
// exports of the notebook, whose archives it deletes once committed. It
// locks the notebook's row first, FOR SHARE, as a deletion of the notebook
// locks it before the jobs': neither waits for a job's row the other
// holds. A job deleted meanwhile, or no longer running, drops its archive.
func (r *run) succeed(ctx context.Context, e Ended, attrs []any) error {
	e.Report = r.report
	var expired []uuid.UUID
	ok := false
	err := r.e.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if ok, err = r.e.d.Notebooks.ShareByID(ctx, r.job.NotebookID); err != nil || !ok {
			return err
		}
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
		r.e.d.Logger.InfoContext(ctx, "export stopped: the job was deleted, or no longer runs", attrs...)
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

// drop deletes the run's committed archive: the sweep deletes one it
// cannot.
func (r *run) drop(ctx context.Context) {
	if err := r.e.d.Archives.Delete(ctx, domain.KindExport, r.job.ID); err != nil {
		r.e.d.Logger.WarnContext(ctx, "export archive not deleted", slog.String("job_id", r.job.ID.String()), slog.Any("error", err))
	}
}
