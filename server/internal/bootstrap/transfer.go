package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The exports' and the imports' parts (M7/P5 design 3.8, 3.14; M7/P6
// design 3.3, 3.5, 3.16), and the background restores' and reverts' (M8/P4
// design 3.6): the transfer module reads a notebook's tree through the
// page module's ExportNodes and the attachments' files through the asset
// module's Blobs, writes an import's nodes, a restore's and a revert's
// batches through the page module's TreeWrites and an import's files
// through the asset module's Blobs, and follows the notebook module's
// deletions. The modules do not import each other, so their values meet
// here.

// transferNodes is the page module's ExportNodes as the transfer module
// reads them.
type transferNodes struct {
	page.ExportNodes
}

// Scope is the nodes of an export.
func (n transferNodes) Scope(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) ([]transfer.Node, error) {
	got, err := n.ExportNodes.Scope(ctx, notebookID, root)
	if err != nil {
		return nil, err
	}
	out := make([]transfer.Node, len(got))
	for i, x := range got {
		out[i] = transfer.Node(x)
	}
	return out, nil
}

// transferBlobs is the asset module's Blobs as the transfer module reads
// them.
type transferBlobs struct {
	asset.Blobs
}

// Of is the blob of each attachment of nodeIDs, and when it was written.
func (b transferBlobs) Of(ctx context.Context, notebookID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]transfer.Blob, error) {
	got, err := b.Blobs.Of(ctx, notebookID, nodeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]transfer.Blob, len(got))
	for node, x := range got {
		out[node] = transfer.Blob(x)
	}
	return out, nil
}

// Open opens a blob's file; a file not in the store is transfer's
// ErrFileMissing, which the export reports and goes on.
func (b transferBlobs) Open(ctx context.Context, blob uuid.UUID) (io.ReadCloser, error) {
	r, err := b.Blobs.Open(ctx, blob)
	if errors.Is(err, asset.ErrNoFile) {
		return nil, transfer.ErrFileMissing
	}
	return r, err
}

// transferNotebookDeletion is the jobs' part in a notebook's deletion as
// the notebook module calls it.
type transferNotebookDeletion struct {
	transfer transfer.NotebookDeletion
}

func (d transferNotebookDeletion) NotebookDeleted(ctx context.Context, x notebook.NotebookDeletion) error {
	return d.transfer.NotebooksDeleted(ctx, x.NotebookIDs, x.At)
}

// transferTree is the page module's TreeWrites as the transfer module's
// imports, restores and reverts write through it.
type transferTree struct {
	tree page.TreeWrites
}

func (t transferTree) CheckContent(content string) error {
	return t.tree.CheckContent(content)
}

func (t transferTree) Parse(ctx context.Context, content string) (transfer.Parsed, error) {
	p, err := t.tree.Parse(ctx, content)
	if err != nil {
		return nil, err
	}
	return parsedPage{p}, nil
}

// Import runs the import's unit: the unit's own refusal, a revert of the
// changeset it merges into, is the import's (M8/P3 design 3.7), as the
// unit's nodes' are.
func (t transferTree) Import(ctx context.Context, spec transfer.ImportSpec, do func(ctx context.Context, u transfer.ImportUnit) error) (uuid.UUID, error) {
	id, err := t.tree.Import(ctx, page.ImportSpec{NotebookID: spec.NotebookID, Action: spec.Action, Client: page.Client(spec.Client),
		Changeset: spec.Changeset}, func(ctx context.Context, u page.ImportUnit) error {
		return do(ctx, importUnit{u})
	})
	return id, treeError(err)
}

// CheckRestore and CheckRevert are the starts' checks: their refusals are
// the page module's problems, which the start answers as they are.
func (t transferTree) CheckRestore(ctx context.Context, notebookID, nodeID uuid.UUID) (string, error) {
	return t.tree.CheckRestore(ctx, notebookID, nodeID)
}

func (t transferTree) CheckRevert(ctx context.Context, notebookID, changesetID uuid.UUID, message *string,
	admit func(context.Context) error, batches transfer.JobBatches,
) (string, error) {
	return t.tree.CheckRevert(ctx, notebookID, changesetID, message, admit, pageJobBatches(batches))
}

func (t transferTree) PlanRestore(ctx context.Context, spec transfer.JobSpec, nodeID uuid.UUID) (transfer.JobPlan, error) {
	p, err := t.tree.PlanRestore(ctx, jobSpec(spec), nodeID)
	if err != nil {
		return nil, treeError(err)
	}
	return planOf(p), nil
}

func (t transferTree) PlanRevert(ctx context.Context, spec transfer.JobSpec, changesetID uuid.UUID, batches transfer.JobBatches) (transfer.JobPlan, error) {
	p, err := t.tree.PlanRevert(ctx, jobSpec(spec), changesetID, pageJobBatches(batches))
	if err != nil {
		return nil, treeError(err)
	}
	return planOf(p), nil
}

// RunBatch writes a batch of a plan PlanRestore or PlanRevert made.
func (t transferTree) RunBatch(ctx context.Context, spec transfer.JobSpec, plan transfer.JobPlan, from, to int) (transfer.BatchDone, error) {
	p, ok := plan.(jobPlan)
	if !ok {
		return transfer.BatchDone{}, errors.New("a background job's plan not the page module's")
	}
	d, err := t.tree.RunBatch(ctx, jobSpec(spec), p.plan, from, to)
	if err != nil {
		return transfer.BatchDone{}, treeError(err)
	}
	out := transfer.BatchDone{Nodes: d.Nodes, Pages: d.Pages, Assets: d.Assets, Renamed: d.Renamed, Skipped: d.Skipped,
		Restored: d.Restored, Deleted: d.Deleted, PutBack: d.PutBack, Written: d.Written}
	if d.Placed != nil {
		out.Placed = &transfer.Placement{ParentID: d.Placed.ParentID, Reason: string(d.Placed.Reason)}
	}
	return out, nil
}

func jobSpec(spec transfer.JobSpec) page.JobSpec {
	return page.JobSpec{NotebookID: spec.NotebookID, Client: page.Client(spec.Client), Changeset: spec.Changeset, Message: spec.Message}
}

// jobPlan is the page module's plan, its steps as the transfer module
// cuts them into batches, converted once.
type jobPlan struct {
	plan  page.JobPlan
	steps []transfer.JobStep
}

func planOf(p page.JobPlan) jobPlan {
	steps := make([]transfer.JobStep, len(p.Steps()))
	for i, s := range p.Steps() {
		steps[i] = transfer.JobStep(s)
	}
	return jobPlan{plan: p, steps: steps}
}

func (p jobPlan) Steps() []transfer.JobStep { return p.steps }
func (p jobPlan) Total() int                { return p.plan.Total() }

// parsedPage is a page's content the page module parsed.
type parsedPage struct {
	p page.Parsed
}

func (p parsedPage) Links() int { return p.p.Links }
func (p parsedPage) Release()   { p.p.Release() }

// importUnit is the page module's import unit as the transfer module's
// import calls it.
type importUnit struct {
	u page.ImportUnit
}

func (i importUnit) CreatePage(ctx context.Context, p transfer.ImportedPage) (transfer.CreatedNode, error) {
	parsed, ok := p.Parsed.(parsedPage)
	if !ok {
		return transfer.CreatedNode{}, errors.New("an import's page not parsed by the page module")
	}
	n, err := i.u.CreatePage(ctx, page.ImportedPage{ParentID: p.ParentID, Name: p.Name, Content: p.Content, Parsed: parsed.p,
		Reserved: p.Reserved})
	return createdNode(n), treeError(err)
}

func (i importUnit) CreateAsset(ctx context.Context, a transfer.ImportedAsset,
	after func(ctx context.Context, n transfer.CreatedNode) error,
) (transfer.CreatedNode, error) {
	n, err := i.u.CreateAsset(ctx, page.ImportedAsset{ParentID: a.ParentID, Name: a.Name,
		Meta: page.AssetMeta{MIME: a.File.MIME, Bytes: a.File.Bytes, SHA256: a.File.SHA256}, Reserved: a.Reserved},
		func(ctx context.Context, n page.NodeInfo) error { return after(ctx, createdNode(n)) })
	return createdNode(n), treeError(err)
}

func createdNode(n page.NodeInfo) transfer.CreatedNode {
	return transfer.CreatedNode{ID: n.ID, Name: n.Name, CreatedAt: n.CreatedAt}
}

// treeError is the page module's error as the jobs read it: a page too
// deep, a parent gone, the job's changeset reverted; a batch to halve,
// which keeps the page module's, wrapped, for the log of a revert it stops
// to say which bound (M9/P3 design 3.23); a trash item changed or expired,
// a changeset trimmed; a revert's conflicts and the lock it met, which
// keep the page module's problem, wrapped, for the report to read its
// conflicts and lock from. Other marked contract refusals keep their code
// and node, with an existing transfer refusal returned unchanged.
func treeError(err error) error {
	switch {
	case errors.Is(err, page.ErrTooDeep):
		return transfer.ErrTooDeep
	case errors.Is(err, page.ErrNoParent):
		return transfer.ErrNoParent
	case errors.Is(err, page.ErrChangesetReverted):
		return transfer.ErrChangesetReverted
	case errors.Is(err, page.ErrJobBatchTooLarge):
		return fmt.Errorf("%w: %w", transfer.ErrBatchTooLarge, err)
	case errors.Is(err, page.ErrTrashChanged):
		return transfer.ErrTrashChanged
	case errors.Is(err, page.ErrRestoreExpired):
		return transfer.ErrRestoreExpired
	case errors.Is(err, page.ErrChangesetTrimmed):
		return transfer.ErrChangesetTrimmed
	case errors.Is(err, page.ErrRevertConflict):
		return fmt.Errorf("%w: %w", transfer.ErrRevertConflict, err)
	case errors.Is(err, page.ErrLocked):
		return fmt.Errorf("%w: %w", transfer.ErrLocked, err)
	}
	var refused *transfer.Refused
	if errors.As(err, &refused) {
		return err
	}
	if code, node, ok := page.Refusal(err); ok {
		return &transfer.Refused{Code: code, NodeID: node, Err: err}
	}
	return err
}

// transferAttachments is the asset module's Blobs as the transfer module's
// imports write their files through it.
type transferAttachments struct {
	blobs asset.Blobs
}

func (a transferAttachments) Put(ctx context.Context, name string, r io.Reader, maxBytes int64) (transfer.File, error) {
	f, err := a.blobs.Put(ctx, name, r, maxBytes)
	switch {
	// An import reaches no ErrTooLarge (M7 closeout A-N7): an entry past
	// asset.max_bytes is skipped as the archive is checked, too_large, and
	// the archive's reader refuses data past an entry's size. It stays the
	// port's word for it.
	case errors.Is(err, asset.ErrTooLarge):
		return transfer.File{}, transfer.ErrTooLarge
	case errors.Is(err, asset.ErrStorageFull):
		return transfer.File{}, transfer.ErrStorageFull
	case err != nil:
		return transfer.File{}, err
	}
	return transfer.File(f), nil
}

func (a transferAttachments) Attach(ctx context.Context, f transfer.File, o transfer.Owner) error {
	return a.blobs.Attach(ctx, asset.File(f), asset.Owner(o))
}

func (a transferAttachments) Drop(ctx context.Context, f transfer.File) error {
	return a.blobs.Drop(ctx, asset.File(f))
}

// pageJobBatches preserves nil and converts only the public scheduling weights.
func pageJobBatches(batches transfer.JobBatches) page.JobBatches {
	if batches == nil {
		return nil
	}
	return func(steps []page.JobStep) [][2]int {
		out := make([]transfer.JobStep, len(steps))
		for i, s := range steps {
			out[i] = transfer.JobStep(s)
		}
		return batches(out)
	}
}
