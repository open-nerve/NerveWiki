package app

import (
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

// An import's writes (M7/P6 design 3.13): the plan's nodes in batches,
// each written in one unit of the page module, which holds the notebook's
// row as it runs, so each is bounded; the pages parsed and the
// attachments' files written before it, outside it.

// A batch's bounds: batchNodes nodes, batchBytes of the pages' contents,
// batchLinks of their links; a page past them alone. The statistics are
// refreshed every analyzeEvery nodes written; a parse or a unit the server
// is too busy for waits at most maxBackoff before it is tried again.
const (
	batchNodes   = 100
	batchBytes   = 8 << 20
	batchLinks   = 20000
	analyzeEvery = 10000
	maxBackoff   = 30 * time.Second
)

// errBusy is a parse the server was too busy for as the run held a batch
// already parsed: the batch is written first, which frees its share of
// the parse budget.
var errBusy = errors.New("transfer: the parse budget is busy")

// item is a node of a batch: its index in the plan, a page's content and
// its parse, an attachment's file once written.
type item struct {
	index   int
	content string
	parsed  Parsed
	file    *File
}

// created is what a unit did of an item: the node it created, or the page
// too deep it skipped, and everything under it.
type created struct {
	index   int
	id      uuid.UUID
	name    string
	dropped bool
}

// write creates the plan's nodes a batch at a time: between two, the
// heartbeat's stop ends the run, the units written kept.
func (r *importRun) write(ctx, running context.Context) error {
	n := len(r.plan.Nodes)
	r.ids, r.names, r.dropped = make([]uuid.UUID, n), make([]string, n), make([]bool, n)
	for next := 0; next < n; {
		if running.Err() != nil {
			return context.Cause(running)
		}
		items, after, err := r.batch(running, next)
		if err == nil {
			err = r.unit(ctx, running, items)
		}
		release(items)
		if err != nil {
			return err
		}
		next = after
	}
	return nil
}

// batch reads and parses the batch from the plan's node start, and
// answers it and where the next starts. A node under a page dropped is
// skipped. A page whose content or links would pass the batch's bounds
// starts the next; so does one the parse budget is too busy for.
func (r *importRun) batch(ctx context.Context, start int) ([]item, int, error) {
	var items []item
	var size int64
	links := 0
	next := start
	for ; next < len(r.plan.Nodes) && len(items) < batchNodes; next++ {
		node := r.plan.Nodes[next]
		if node.Parent >= 0 && r.dropped[node.Parent] {
			r.drop(next)
			continue
		}
		if node.Asset {
			items = append(items, item{index: next})
			continue
		}
		n := r.sizes[node.Entry]
		if len(items) > 0 && size+n > batchBytes {
			break
		}
		content, err := r.content(ctx, node)
		if err != nil {
			return items, next, err
		}
		parsed, err := r.parse(ctx, content, len(items) > 0)
		if errors.Is(err, errBusy) {
			break
		}
		if err != nil {
			return items, next, err
		}
		if len(items) > 0 && links+parsed.Links() > batchLinks {
			parsed.Release()
			break
		}
		items = append(items, item{index: next, content: content, parsed: parsed})
		size, links = size+n, links+parsed.Links()
	}
	return items, next, nil
}

// drop skips the plan's node i, under a page too deep.
func (r *importRun) drop(i int) {
	r.dropped[i] = true
	r.skip(domain.Problem{Path: r.plan.Nodes[i].Path, Code: domain.ProblemTooDeep})
	r.done.Add(1)
}

// content reads a page's content again, which validate checked; a page
// that is only its folder has none.
func (r *importRun) content(ctx context.Context, node domain.ImportNode) (string, error) {
	if node.Entry < 0 {
		return "", nil
	}
	rc, err := r.archive.Open(node.Entry)
	if err != nil {
		return "", fmt.Errorf("read an import's page again: %w", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: rc}, r.i.d.MaxContent+1))
	if err != nil {
		return "", fmt.Errorf("read an import's page again: %w", err)
	}
	return string(data), nil
}

// parse parses a page's content for its unit. When the server is too busy
// for it, a batch already parsed, holding, is written first: errBusy;
// without one, the parse is tried again after a wait.
func (r *importRun) parse(ctx context.Context, content string, holding bool) (Parsed, error) {
	var parsed Parsed
	err := r.retry(ctx, func() error {
		var err error
		parsed, err = r.i.d.Tree.Parse(ctx, content)
		if busy(err) && holding {
			return errBusy
		}
		return err
	})
	return parsed, err
}

// retry runs do until it answers anything but 503: after a wait of
// Backoff, doubled each time up to maxBackoff, until ctx ends.
func (r *importRun) retry(ctx context.Context, do func() error) error {
	wait := r.i.d.Backoff
	for {
		err := do()
		if !busy(err) {
			return err
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return context.Cause(ctx)
		case <-t.C:
		}
		wait = min(2*wait, maxBackoff)
	}
}

// busy reports whether err is the server's 503: its parse budget, or a
// lock, too busy.
func busy(err error) bool {
	var e *shared.Error
	return errors.As(err, &e) && e.Kind == shared.KindUnavailable
}

// release gives back the parse budget the batch's pages hold.
func release(items []item) {
	for _, it := range items {
		if it.parsed != nil {
			it.parsed.Release()
		}
	}
}

// unit writes the batch: its attachments' files, then its nodes in one
// unit, run with River's context, which the heartbeat's stop does not
// end; the server too busy for it, it is tried again. A unit that rolled
// back drops the files; one whose commit failed leaves them to the
// attachments' sweep.
func (r *importRun) unit(ctx, running context.Context, items []item) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.put(running, items); err != nil {
		r.dropFiles(ctx, items)
		return err
	}
	var done []created
	var changeset uuid.UUID
	// uncertain is a unit that failed neither in do nor refused: its
	// commit may have failed after the database took it.
	uncertain := false
	err := r.retry(running, func() error {
		if running.Err() != nil {
			return context.Cause(running)
		}
		rolledBack := false
		var err error
		changeset, err = r.i.d.Tree.Import(ctx, ImportSpec{NotebookID: r.job.NotebookID, Action: domain.ActionImport, Client: r.job.Client,
			Changeset: r.changeset}, func(ctx context.Context, u ImportUnit) error {
			done = done[:0]
			err := r.create(ctx, u, items, &done)
			rolledBack = err != nil
			return err
		})
		var refused *shared.Error
		uncertain = err != nil && !rolledBack && !errors.As(err, &refused)
		return err
	})
	if err != nil {
		if !uncertain {
			r.dropFiles(ctx, items)
		}
		return unitFailure(err)
	}
	if r.changeset == (uuid.UUID{}) {
		r.changeset = changeset
	}
	r.tell(ctx, items, done)
	return nil
}

// unitFailure is why a unit failed: forbidden for a job whose account can
// no longer write in the notebook, or see it; the failure it found; any
// other is internal.
func unitFailure(err error) error {
	var e *shared.Error
	if errors.As(err, &e) && (e.Kind == shared.KindForbidden || e.Kind == shared.KindNotFound || e.Kind == shared.KindUnauthenticated) {
		return failed(domain.FailureForbidden, err)
	}
	return err
}

// put writes the batch's attachments' files from the archive, their reads
// stopped with ctx: storage_full when the store has no room.
func (r *importRun) put(ctx context.Context, items []item) error {
	for k := range items {
		node := r.plan.Nodes[items[k].index]
		if !node.Asset {
			continue
		}
		f, err := r.putFile(ctx, node)
		if err != nil {
			return err
		}
		items[k].file = &f
	}
	return nil
}

func (r *importRun) putFile(ctx context.Context, node domain.ImportNode) (File, error) {
	rc, err := r.archive.Open(node.Entry)
	if err != nil {
		return File{}, fmt.Errorf("read an import's attachment again: %w", err)
	}
	defer func() { _ = rc.Close() }()
	f, err := r.i.d.Attachments.Put(ctx, node.Name, contextReader{ctx: ctx, r: rc}, r.i.d.MaxAsset)
	if errors.Is(err, domain.ErrStorageFull) {
		return File{}, failed(domain.FailureStorageFull, err)
	}
	return f, err
}

// dropFiles deletes the files of items whose nodes were not created: the
// orphans' sweep deletes one it cannot.
func (r *importRun) dropFiles(ctx context.Context, items []item) {
	ctx = context.WithoutCancel(ctx)
	for _, it := range items {
		if it.file == nil {
			continue
		}
		if err := r.i.d.Attachments.Drop(ctx, *it.file); err != nil {
			r.logger.WarnContext(ctx, "import attachment's file not deleted", slog.String("job_id", r.job.ID.String()), slog.Any("error", err))
		}
	}
}

// create creates the batch's nodes in u, recording each into done: a
// page too deep is dropped, with everything under it; a parent gone fails
// the import, root_not_found for where it goes, tree_changed for a page
// it created.
func (r *importRun) create(ctx context.Context, u ImportUnit, items []item, done *[]created) error {
	ids := map[int]uuid.UUID{}
	dropped := map[int]bool{}
	for _, it := range items {
		node := r.plan.Nodes[it.index]
		if node.Parent >= 0 && (r.dropped[node.Parent] || dropped[node.Parent]) {
			dropped[it.index] = true
			*done = append(*done, created{index: it.index, dropped: true})
			continue
		}
		parent := r.job.RootID
		if node.Parent >= 0 {
			id, ok := ids[node.Parent]
			if !ok {
				id = r.ids[node.Parent]
			}
			parent = &id
		}
		var c CreatedNode
		var err error
		// The names of the later siblings are theirs: a number given this
		// one takes none of them.
		i := it.index
		reserved := func(key string) bool { return r.last[node.Parent][key] > i }
		if node.Asset {
			file := *it.file
			c, err = u.CreateAsset(ctx, ImportedAsset{ParentID: parent, Name: node.Name, File: file, Reserved: reserved},
				func(ctx context.Context, n CreatedNode) error {
					return r.i.d.Attachments.Attach(ctx, file, Owner{NodeID: n.ID, NotebookID: r.job.NotebookID, CreatedBy: r.job.CreatedBy,
						CreatedAt: n.CreatedAt})
				})
		} else {
			c, err = u.CreatePage(ctx, ImportedPage{ParentID: parent, Name: node.Name, Content: it.content, Parsed: it.parsed, Reserved: reserved})
		}
		switch {
		case errors.Is(err, ErrTooDeep):
			dropped[it.index] = true
			*done = append(*done, created{index: it.index, dropped: true})
			continue
		case errors.Is(err, ErrNoParent) && node.Parent < 0:
			return failed(domain.FailureRootNotFound, err)
		case errors.Is(err, ErrNoParent):
			return failed(domain.FailureTreeChanged, err)
		case err != nil:
			return err
		}
		ids[it.index] = c.ID
		*done = append(*done, created{index: it.index, id: c.ID, name: c.Name})
	}
	return nil
}

// tell records what a committed unit did: its nodes created, counted, and
// renamed when their names are not the archive's; those dropped skipped,
// their files deleted. The statistics are refreshed every analyzeEvery
// nodes.
func (r *importRun) tell(ctx context.Context, items []item, done []created) {
	files := map[int]*File{}
	for _, it := range items {
		files[it.index] = it.file
	}
	var unattached []item
	for _, c := range done {
		node := r.plan.Nodes[c.index]
		if c.dropped {
			r.drop(c.index)
			if files[c.index] != nil {
				unattached = append(unattached, item{file: files[c.index]})
			}
			continue
		}
		r.ids[c.index], r.names[c.index] = c.id, c.name
		if node.Asset {
			r.report.Counts.Attachments++
		} else {
			r.report.Counts.Pages++
		}
		if c.name != node.Original {
			r.report.Counts.Renamed++
			r.report.Add(domain.Problem{Path: node.Path, Code: domain.ProblemRenamed, To: r.pathOf(c.index)})
		}
		r.done.Add(1)
		r.unanalyzed++
	}
	r.publish()
	r.dropFiles(ctx, unattached)
	if r.unanalyzed >= analyzeEvery {
		r.analyze(ctx)
	}
}

// pathOf is the path of the plan's node i in the notebook, from where the
// import goes: its names and its parents' joined by "/".
func (r *importRun) pathOf(i int) string {
	node := r.plan.Nodes[i]
	if node.Parent < 0 {
		return r.names[i]
	}
	return r.pathOf(node.Parent) + "/" + r.names[i]
}
