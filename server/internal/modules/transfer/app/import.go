package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Import runs an import's job (M7/P6 design 3.13): River's worker calls it
// with the job's id.
type Import struct {
	d ImportDeps
}

// ImportDeps are what Import needs.
type ImportDeps struct {
	Authorizer  shared.Authorizer
	Notebooks   Notebooks
	Nodes       Nodes
	Tree        Tree
	Attachments Attachments
	// Statistics are the page, linking and asset modules', in this order.
	Statistics []Analyzer
	Archives   Archives
	Rows       Rows
	Clock      Clock
	Logger     *slog.Logger
	// Beat is how often a running job writes its heartbeat and progress,
	// and reads whether it was cancelled: a second.
	Beat time.Duration
	// MaxEntries is transfer.import_max_entries; MaxUnpacked
	// transfer.import_max_unpacked_bytes; MaxContent the most bytes of a
	// page's content, the page module's; MaxAsset asset.max_bytes.
	MaxEntries  int
	MaxUnpacked int64
	MaxContent  int64
	MaxAsset    int64
	// Backoff is the first wait before a parse or a unit the server was
	// too busy for is tried again: a second, doubled each time up to
	// maxBackoff.
	Backoff time.Duration
}

// NewImport returns the use case.
func NewImport(d ImportDeps) *Import {
	return &Import{d: d}
}

// Run runs the import id. A job no longer queued, cancelled or deleted
// meanwhile, is left as it is. A running one writes its heartbeat every
// Beat, which stops it between two units when its cancel was asked or it
// was deleted; it ends succeeded, cancelled or failed with its report,
// what it wrote kept. Its archive is deleted at every end. Run fails only
// when the job's end cannot be written: the rescue fails it later.
func (i *Import) Run(ctx context.Context, id uuid.UUID) error {
	job, err := i.d.Rows.StartJob(ctx, id, i.d.Clock.Now())
	if errors.Is(err, ErrNoRow) {
		i.drop(ctx, id)
		return nil
	}
	if err != nil {
		return err
	}
	attrs := jobAttrs(job)
	i.d.Logger.InfoContext(ctx, "import started", attrs...)
	r := &importRun{jobRun: newJobRun(job, i.d.Rows, i.d.Clock, i.d.Logger, i.d.Beat), i: i}
	running, stop := context.WithCancelCause(ctx)
	beating := r.beat(running, stop)
	err = r.run(ctx, running)
	if r.unanalyzed > 0 && ctx.Err() == nil {
		r.analyze(ctx)
	}
	stop(nil)
	<-beating
	err = r.finish(ctx, running, err, attrs, nil)
	i.drop(context.WithoutCancel(ctx), id)
	return err
}

// drop deletes the import id's archive: the sweep deletes one it cannot.
func (i *Import) drop(ctx context.Context, id uuid.UUID) {
	if err := i.d.Archives.Delete(ctx, domain.KindImport, id); err != nil {
		i.d.Logger.WarnContext(ctx, "import archive not deleted", slog.String("job_id", id.String()), slog.Any("error", err))
	}
}

// importRun is an import as it runs.
type importRun struct {
	*jobRun
	i       *Import
	archive ImportArchive
	// sizes are the bytes each entry kept unpacks to, by its index.
	sizes map[int]int64
	plan  domain.ImportPlan
	// ids are the nodes created, names their names, by the plan's index;
	// dropped are the plan's pages too deep as their unit found them, and
	// everything under them.
	ids     []uuid.UUID
	names   []string
	dropped []bool
	// changeset is the first unit's, which the later ones merge into.
	changeset uuid.UUID
	// unanalyzed is how many nodes were written since the statistics
	// were last refreshed.
	unanalyzed int
	buf        []byte
}

// run reads the archive and writes its nodes: running is River's context
// as the heartbeat may stop it, which every step but a unit heeds.
func (r *importRun) run(ctx, running context.Context) error {
	if err := authorizeJob(running, r.i.d.Notebooks, r.i.d.Authorizer, r.job, domain.ActionImport); err != nil {
		return err
	}
	archive, err := r.i.d.Archives.OpenImport(running, r.job.ID, r.i.d.MaxEntries)
	switch {
	case errors.Is(err, ErrNotZip):
		return failed(domain.FailureNotZip, err)
	case errors.Is(err, ErrTooManyEntries):
		return failed(domain.FailureTooManyEntries, err)
	case err != nil:
		return err
	}
	defer func() { _ = archive.Close() }()
	r.archive = archive
	sorted := domain.Classify(archive.Entries())
	for _, p := range sorted.Skipped {
		r.skip(p)
	}
	meta := r.meta(running, sorted.Meta)
	entries, err := r.validate(running, sorted.Entries, meta)
	if err != nil {
		return err
	}
	depth, err := r.depth(running)
	if err != nil {
		return err
	}
	r.plan = domain.NewImportPlan(entries, meta, depth)
	for _, p := range r.plan.Skipped {
		r.skip(p)
	}
	r.all.Store(int64(len(r.plan.Nodes)))
	return r.write(ctx, running)
}

// skip reports a problem of an entry or a node skipped.
func (r *importRun) skip(p domain.Problem) {
	r.report.Counts.Skipped++
	r.report.Add(p)
}

// meta reads the vault's meta.json, the entry index, -1 for none: what it
// tells of the order and of the contributors' files; nothing when it is
// larger than domain.MaxMeta, or cannot be read.
func (r *importRun) meta(ctx context.Context, index int) domain.ImportMeta {
	if index < 0 {
		return domain.ImportMeta{}
	}
	rc, err := r.archive.Open(index)
	if err != nil {
		return domain.ImportMeta{}
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: rc}, domain.MaxMeta+1))
	if err != nil || len(data) > domain.MaxMeta {
		return domain.ImportMeta{}
	}
	return domain.ReadMeta(data)
}

// validate reads each file of entries to its end, in the archive's order,
// writing nothing (M7/P6 design 3.11): it answers the entries to import,
// the files that fail skipped with their problems, and keeps the bytes
// each unpacks to. The files meta tells contributors added are left out.
// The archive fails unpacked_too_large once its files unpack to more than
// MaxUnpacked.
func (r *importRun) validate(ctx context.Context, entries []domain.ImportEntry, meta domain.ImportMeta) ([]domain.ImportEntry, error) {
	r.sizes, r.buf = map[int]int64{}, make([]byte, 32<<10)
	var kept []domain.ImportEntry
	var total int64
	for _, e := range entries {
		switch {
		case e.Folder:
			kept = append(kept, e)
			continue
		case meta.Contributed[e.Joined()]:
			continue
		}
		problem, err := r.check(ctx, e, &total)
		if err != nil {
			return nil, err
		}
		if problem != "" {
			r.skip(domain.Problem{Path: e.Name, Code: problem})
			continue
		}
		kept = append(kept, e)
	}
	return kept, nil
}

// check reads the file e to its end: a page's content checked as its unit
// would, an attachment's bytes counted.
func (r *importRun) check(ctx context.Context, e domain.ImportEntry, total *int64) (domain.ProblemCode, error) {
	largest := r.i.d.MaxAsset
	var content *bytes.Buffer
	if e.Page() {
		largest, content = r.i.d.MaxContent, &bytes.Buffer{}
	}
	rc, err := r.archive.Open(e.Index)
	if err != nil {
		return domain.ProblemUnreadable, nil //nolint:nilerr // the entry is skipped, the import goes on
	}
	defer func() { _ = rc.Close() }()
	n, problem, err := r.unpack(ctx, rc, e.Index, largest, total, content)
	if err != nil || problem != "" {
		return problem, err
	}
	if content != nil && r.i.d.Tree.CheckContent(content.String()) != nil {
		return domain.ProblemInvalidContent, nil
	}
	r.sizes[e.Index] = n
	return "", nil
}

// unpack reads rc, the entry index's data, to its end, into content when
// it is set: too_large past largest bytes, too_compressed
// (domain.TooCompressed), unreadable when its data is broken, each
// stopping the read. Every byte read counts toward total, the archive's,
// which fails it past MaxUnpacked.
func (r *importRun) unpack(ctx context.Context, rc io.Reader, index int, largest int64, total *int64, content *bytes.Buffer) (int64, domain.ProblemCode, error) {
	packed := r.archive.Packed(index)
	var n int64
	for {
		if ctx.Err() != nil {
			return n, "", context.Cause(ctx)
		}
		k, err := rc.Read(r.buf)
		if k > 0 {
			n += int64(k)
			if *total += int64(k); *total > r.i.d.MaxUnpacked {
				return n, "", failed(domain.FailureUnpackedTooLarge, fmt.Errorf("the entries unpack to more than %d bytes", r.i.d.MaxUnpacked))
			}
			switch {
			case n > largest:
				return n, domain.ProblemTooLarge, nil
			case domain.TooCompressed(n, packed):
				return n, domain.ProblemTooCompressed, nil
			}
			if content != nil {
				content.Write(r.buf[:k])
			}
		}
		if errors.Is(err, io.EOF) {
			return n, "", nil
		}
		if err != nil {
			return n, domain.ProblemUnreadable, nil
		}
	}
}

// depth is the depth of the page the import goes under, 0 for the
// notebook's root: root_not_found when it is gone.
func (r *importRun) depth(ctx context.Context) (int, error) {
	if r.job.RootID == nil {
		return 0, nil
	}
	depth, ok, err := r.i.d.Nodes.Depth(ctx, r.job.NotebookID, *r.job.RootID)
	if err == nil && !ok {
		err = failed(domain.FailureRootNotFound, errors.New("the page imported into is gone"))
	}
	return depth, err
}

// analyze refreshes the statistics of the tables the import fills: a
// failure is logged, the import going on.
func (r *importRun) analyze(ctx context.Context) {
	for _, a := range r.i.d.Statistics {
		if err := a.Analyze(ctx); err != nil {
			r.logger.WarnContext(ctx, "import statistics not refreshed", slog.String("job_id", r.job.ID.String()), slog.Any("error", err))
		}
	}
	r.unanalyzed = 0
}
