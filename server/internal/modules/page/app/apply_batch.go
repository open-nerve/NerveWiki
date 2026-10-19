package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PathBatch is batch's arguments (M9 design 4.11).
type PathBatch struct {
	Ops     []domain.BatchArgs
	Message string
}

// OpDone is an op's outcome: the node it acted on and its path after the
// batch, or at the op for a node a later op deleted; a content op's page
// version as the batch leaves it (the deferred rewrite's write too), or
// as the op's composed write left it for a page a later op deleted;
// whether its tool would have changed anything at its step; the nodes a
// write created, root first, at their paths after the batch; an edit's
// replacements; a delete's pages and attachments, none for one merged
// into the later delete MergedInto.
type OpDone struct {
	Kind          domain.OpKind
	Path          string
	Node          domain.Node
	Version       domain.VersionToken
	Changed       bool
	Created       []Placed
	Replacements  int
	Pages, Assets int
	MergedInto    int // -1 for none
}

// Batched is a batch's outcome: each op's, in their order, and the unit's.
// Outcome's ChangesetID is zero when the batch left nothing.
type Batched struct {
	Ops     []OpDone
	Outcome Outcome
}

// ApplyBatch applies a batch of MCP's writes as one changeset (M9 design
// 4.11; M9/P3 design 3.6): planned on the notebook's tree read unlocked,
// then done again op by op in one unit under the notebook's lock, which
// must find what the plan found.
type ApplyBatch struct {
	writer    *Writer
	parser    *ContentParser
	paths     NodePaths
	nodes     Nodes
	states    PageStates
	links     BatchLinks
	locks     *LockHolders
	notebooks Notebooks
	auth      shared.Authorizer
	// maxRewrites bounds the pages the deferred rewrite writes: 0 is the
	// domain's, domain.MaxBatchRewrites, which only a measurement raises
	// (M9/P3 design 3.8.2).
	maxRewrites int
	clock       Clock
	logger      *slog.Logger
}

// NewApplyBatch returns the use case, its deferred rewrite bounded at
// maxRewrites pages, 0 for the domain's.
func NewApplyBatch(writer *Writer, parser *ContentParser, paths PathStore, nodes Nodes, states PageStates, links BatchLinks,
	locks *LockHolders, notebooks Notebooks, auth shared.Authorizer, maxRewrites int, clock Clock, logger *slog.Logger,
) *ApplyBatch {
	return &ApplyBatch{writer: writer, parser: parser, paths: NewNodePaths(paths), nodes: nodes, states: states, links: links,
		locks: locks, notebooks: notebooks, auth: auth, maxRewrites: cmp.Or(maxRewrites, domain.MaxBatchRewrites), clock: clock,
		logger: logger}
}

// replanned is a batch's unit that found the notebook other than its plan
// had it at the op op (M9/P3 design 3.6.2): what differed, the node, the
// content's version, or an error of a kind the plan checks, err; and for
// a content op, the page at its path then, with its version, nil for
// none. The unit rolled back.
type replanned struct {
	op      int
	what    string
	err     error
	current *domain.VersionToken
}

func (r *replanned) Error() string {
	return fmt.Sprintf("page: the batch's unit found the %s of ops[%d] other than its plan had it", r.what, r.op)
}

// Execute applies p in the notebook notebookID from client. Its order: the
// actor; CheckBatch's 422s; the notebook, notebook.not_found; page.batch's
// 403, unlocked; the tree's pass (PlanBatch): the first op that fails, the
// items past MaxUnitNodes at the op that passes them; the sizes of the
// pages its edits read, past MaxBatchReadBytes at the op that passes them,
// before any content is read; the contents' pass (Contents): the first op
// that fails, its parses' bounds (422) and budget (503) at it; the edit
// locks of the pages it writes, page.locked at the first op that meets one;
// the unit's bounds on what it parses, page.batch_too_large; the final
// contents parsed, 503; their links, page.batch_too_large; then the unit,
// which does each op again under the notebook's lock and must find what
// the plan found: else the whole of it once more from the plan, in the
// same parse session, and then page.revision_mismatch at the first op that
// differs. A refusal of an edit lock carries its pages' paths and the op
// it met.
func (b *ApplyBatch) Execute(ctx context.Context, notebookID uuid.UUID, p PathBatch, client domain.Client) (Batched, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Batched{}, err
	}
	steps, err := domain.CheckBatch(p.Ops, p.Message)
	if err != nil {
		return Batched{}, err
	}
	if err := decideNotebook(ctx, b.notebooks, b.auth, actor, domain.ActionBatch, notebookID); err != nil {
		return Batched{}, err
	}
	session := b.parser.Planned()
	defer session.Release()
	a := &batchAttempt{b: b, notebookID: notebookID, steps: steps, message: p.Message, client: client, session: session}
	out, err := a.run(ctx)
	var again *replanned
	if errors.As(err, &again) {
		b.logger.InfoContext(ctx, "batch planned again", slog.String("notebook_id", notebookID.String()), slog.Int("op", again.op),
			slog.String("differs", again.what))
		session.NewPlan()
		a = &batchAttempt{b: b, notebookID: notebookID, steps: steps, message: p.Message, client: client, session: session}
		if out, err = a.run(ctx); errors.As(err, &again) {
			err = changedTwice(steps, again)
		}
	}
	session.Release()
	if err != nil {
		return Batched{}, b.lockPaths(ctx, notebookID, err)
	}
	// Ids, counts and the unit's time alone: no path, content or message
	// (v0.1 design 13.1, item 10).
	attrs := []any{slog.String("workspace_id", out.Outcome.WorkspaceID.String()), slog.String("notebook_id", notebookID.String())}
	if out.Outcome.ChangesetID != (uuid.UUID{}) {
		attrs = append(attrs, slog.String("changeset_id", out.Outcome.ChangesetID.String()))
	}
	b.logger.InfoContext(ctx, "batch applied", append(attrs, slog.String("user_id", out.Outcome.By.String()),
		slog.String("client", string(client)), slog.Int("ops", len(steps)), slog.Int("pages_written", a.written),
		slog.Int("created", a.created), slog.Int("deleted", a.deleted), slog.Int("moved", a.moved), slog.Int("rewritten", a.rewritten),
		slog.Int64("unit_ms", a.unit.Milliseconds()))...)
	return out, nil
}

// changedTwice is page.revision_mismatch at the op that differed again in
// the second unit (M9/P3 design 3.6.2): a content op's at its path, with
// the version of the page there, why as its edit's mismatch tells it; any
// other op's, or a content op's with no page there, the notebook changed
// while the batch was planned, twice.
func changedTwice(steps []domain.BatchStep, r *replanned) error {
	s := steps[r.op]
	err := domain.BatchChanged()
	if (s.Kind == domain.OpKindWrite || s.Kind == domain.OpKindEdit) && r.current != nil {
		why := domain.KeptChanging
		switch {
		case s.Base != nil && s.Base.Node != r.current.Node:
			why = domain.OtherNode
		case s.Base != nil:
			why = domain.Changed
		}
		err = &domain.VersionMismatch{Path: s.Path.Written, Current: *r.current, Why: why}
	}
	return &domain.OpFailed{Op: r.op, Tool: s.Kind, Err: err}
}

// lockPaths is err with the paths of the pages a refusal of edit locks
// names (withLockPaths), within the op it failed at.
func (b *ApplyBatch) lockPaths(ctx context.Context, notebookID uuid.UUID, err error) error {
	var f *domain.OpFailed
	if errors.As(err, &f) {
		return &domain.OpFailed{Op: f.Op, Tool: f.Tool, Err: withLockPaths(ctx, b.paths, notebookID, b.logger, f.Err)}
	}
	return withLockPaths(ctx, b.paths, notebookID, b.logger, err)
}

// batchAttempt is one try of a batch, from the read of the tree to its
// unit: its plan, the facts of its final contents and the links they had,
// and, once its unit ran, what the unit did and its time.
type batchAttempt struct {
	b          *ApplyBatch
	notebookID uuid.UUID
	steps      []domain.BatchStep
	message    string
	client     domain.Client
	session    *ParseSession
	plan       *domain.BatchPlan
	base       map[uuid.UUID]domain.Node
	pages      map[uuid.UUID]domain.PlannedPage
	facts      map[uuid.UUID]Facts
	empty      Facts
	befores    map[uuid.UUID]WrittenPage
	// The unit's: each op's node and path as it found them, the pages it
	// wrote and their revisions then, and its counts and time.
	done      []OpDone
	revisions map[uuid.UUID]int
	wrote     map[uuid.UUID]bool
	written   int
	created   int
	deleted   int
	moved     int
	rewritten int
	unit      time.Duration
}

// run is the try: the plan, its reads and its checks, then the unit.
func (a *batchAttempt) run(ctx context.Context) (Batched, error) {
	if err := a.planned(ctx); err != nil {
		return Batched{}, err
	}
	spec := UnitSpec{NotebookID: a.notebookID, Action: domain.ActionBatch, Tree: true, Client: a.client,
		Options: Options{UpdateLinks: true}, Message: &a.message, NotFound: domain.ErrNotebookNotFound, DeferParticipants: true}
	started := time.Now()
	outcome, err := a.b.writer.Run(ctx, spec, a.apply)
	a.unit = time.Since(started)
	if err != nil {
		return Batched{}, err
	}
	return Batched{Ops: a.done, Outcome: outcome}, nil
}

// planned plans the batch on the tree read unlocked (M9/P3 design 3.4):
// the tree's pass, the reads' bound, the contents' pass, the edit locks'
// precheck, the unit's bounds, the final contents parsed and the links of
// each at its last content op.
func (a *batchAttempt) planned(ctx context.Context) error {
	nodes, err := a.b.nodes.ListNodes(ctx, a.notebookID)
	if err != nil {
		return err
	}
	a.base = make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		a.base[n.ID] = n
	}
	ids := make([][]uuid.UUID, len(a.steps))
	for i, s := range a.steps {
		if s.Kind == domain.OpKindWrite && s.Base == nil {
			ids[i] = make([]uuid.UUID, len(s.Path.Keys))
			for d := range ids[i] {
				ids[i][d] = uuid.NewV7()
			}
		}
	}
	a.plan = domain.PlanBatch(nodes, a.steps, ids)
	texts, heads, err := a.read(ctx)
	if err != nil {
		return err
	}
	if err := a.plan.Contents(texts, heads, &planLinks{ctx: ctx, a: a}); err != nil {
		return err
	}
	if a.plan.Failed != nil {
		return a.plan.Failed
	}
	a.pages = map[uuid.UUID]domain.PlannedPage{}
	for _, pg := range a.plan.Pages() {
		a.pages[pg.ID] = pg
	}
	if err := a.locked(ctx); err != nil {
		return err
	}
	if err := a.plan.CheckUnit(); err != nil {
		return err
	}
	contents := make(map[uuid.UUID]string, len(a.pages))
	for id, pg := range a.pages {
		contents[id] = pg.Content
	}
	if a.facts, err = a.session.DecidedAll(ctx, contents, func(links int) error {
		return domain.BatchTooLarge("links in the contents the batch writes and its plan parses", links, domain.MaxUnitLinks,
			"split the batch")
	}); err != nil {
		return err
	}
	a.empty = a.b.parser.markdown.Facts("") // an empty parent's: no budget taken
	return a.linksBefore(ctx)
}

// read is what the contents' pass reads of the pages before the batch
// (M9/P3 design 3.4.4): the heads of the pages its content ops write, one
// statement, the bound on the contents read whole checked on their sizes,
// then those contents, each with its revision in one statement. A page
// gone since the tree's read is page.not_found at its first content op.
func (a *batchAttempt) read(ctx context.Context) (map[uuid.UUID]domain.PageText, map[uuid.UUID]domain.PageHead, error) {
	texts, heads := a.plan.Reads()
	if len(texts)+len(heads) == 0 {
		return nil, nil, nil
	}
	firsts := map[uuid.UUID]int{}
	for _, pg := range a.plan.Pages() {
		firsts[pg.ID] = pg.First
	}
	gone := func(id uuid.UUID) error {
		first := firsts[id]
		return &domain.OpFailed{Op: first, Tool: a.steps[first].Kind, Err: domain.ErrNotFound}
	}
	states, err := a.b.states.PageStates(ctx, append(slices.Clone(texts), heads...))
	if err != nil {
		return nil, nil, err
	}
	read := make(map[uuid.UUID]domain.PageHead, len(states))
	for _, id := range append(slices.Clone(texts), heads...) {
		s, ok := states[id]
		if !ok {
			return nil, nil, gone(id)
		}
		read[id] = domain.PageHead{Revision: s.Revision, Bytes: s.Bytes, Hash: s.Hash}
	}
	if failed := a.plan.CheckReads(read); failed != nil {
		return nil, nil, failed
	}
	out := make(map[uuid.UUID]domain.PageText, len(texts))
	for _, id := range texts {
		c, err := a.b.nodes.PageContent(ctx, id)
		if errors.Is(err, ErrNotFound) {
			return nil, nil, gone(id)
		}
		if err != nil {
			return nil, nil, err
		}
		out[id] = domain.PageText{Content: c.Content, Revision: c.Revision}
	}
	return out, read, nil
}

// locked is the edit locks' precheck (M9/P3 design 3.4.5): the pages
// before the batch whose content it writes, an alive session of one
// page.locked at the first content op that writes such a page, before the
// parse takes its budget. The unit's guards refuse again.
func (a *batchAttempt) locked(ctx context.Context) error {
	var ids []uuid.UUID
	for _, pg := range a.pages {
		if !pg.Created {
			ids = append(ids, pg.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	holders, err := a.b.locks.Of(ctx, ids, a.b.clock.Now())
	if err != nil || len(holders) == 0 {
		return err
	}
	first := -1
	var held shared.LockHolder
	for _, h := range holders {
		if pg := a.pages[h.PageID]; first < 0 || pg.First < first {
			first, held = pg.First, h
		}
	}
	return &domain.OpFailed{Op: first, Tool: a.steps[first].Kind, Err: domain.Locked(held.PageID, held.UserID, held.DisplayName)}
}

// linksBefore is, for each page whose content the batch writes and that a
// later rename or move with update_links reaches, where its links lead at
// its last content op, on the plan's tree after it, from its final
// content's facts (M9/P3 design 3.4.5, 3.7.1): its deferred rewrite's
// before; and its own changes, those renames and moves.
func (a *batchAttempt) linksBefore(ctx context.Context) error {
	last := len(a.steps) - 1
	gone := a.plan.TreeAt(last).Gone()
	a.befores = make(map[uuid.UUID]WrittenPage, len(a.pages))
	for id, pg := range a.pages {
		if slices.Contains(gone, id) {
			continue // a page a later op deleted: no rewrite follows it
		}
		w := WrittenPage{Changes: a.plan.MovesBetween(pg.Last, last), Facts: a.facts[id]}
		if len(w.Changes) > 0 {
			facts := a.facts[id]
			before, err := a.b.links.Resolve(ctx, LinksOn{NotebookID: a.notebookID, PageID: id, Content: pg.Content,
				Tree: a.plan.TreeAt(pg.Last), Parse: func(context.Context, string) (Facts, error) { return facts, nil }})
			if err != nil {
				return err
			}
			w.Before = before
		}
		a.befores[id] = w
	}
	return nil
}

// apply is the batch's unit (M9/P3 design 3.6.2): the former parents'
// paths, read before any op; each op done again, but those merged into a
// later delete, finding what the plan found; the deferred rewrite, once;
// what it changed back dropped; and the answer's reads.
func (a *batchAttempt) apply(ctx context.Context, u *Unit) error {
	last := len(a.steps) - 1
	moves := a.plan.MovesBetween(-1, last)
	var formers []uuid.UUID
	for _, c := range moves {
		if c.Before.ParentID != nil && !domain.SameParent(c.Before.ParentID, c.After.ParentID) {
			formers = append(formers, *c.Before.ParentID)
		}
	}
	slices.SortFunc(formers, uuid.UUID.Compare)
	parents, err := a.b.paths.Lines(ctx, a.notebookID, slices.Compact(formers))
	if err != nil {
		return err
	}
	a.done, a.revisions, a.wrote = make([]OpDone, len(a.steps)), map[uuid.UUID]int{}, map[uuid.UUID]bool{}
	a.written, a.created, a.deleted, a.moved, a.rewritten = 0, 0, 0, 0, 0
	movedNodes := map[uuid.UUID]bool{}
	for i, s := range a.steps {
		ps := a.plan.Steps[i]
		a.done[i] = OpDone{Kind: s.Kind, Changed: ps.Changed, MergedInto: ps.MergedInto, Replacements: ps.Replacements,
			Pages: ps.Pages, Assets: ps.Assets}
		if ps.MergedInto >= 0 {
			a.done[i].Node, a.done[i].Path = a.base[ps.Node], ps.Path
			continue
		}
		var err error
		switch {
		case s.Kind == domain.OpKindWrite && len(ps.Created) > 0:
			err = a.create(ctx, u, i)
		case s.Kind == domain.OpKindWrite || s.Kind == domain.OpKindEdit:
			err = a.content(ctx, u, i)
		case s.Kind == domain.OpKindMove:
			err = a.move(ctx, u, i)
			if ps.Changed {
				movedNodes[ps.Node] = true
			}
		default:
			err = a.delete(ctx, u, i)
			a.deleted += ps.Pages + ps.Assets
		}
		if err != nil {
			return err
		}
	}
	a.moved = len(movedNodes)
	if err := a.participate(ctx, u, parents); err != nil {
		return err
	}
	if err := u.DropUnchanged(ctx); err != nil {
		return err
	}
	return a.answer(ctx)
}

// differs is the unit's finding at the op i other than the plan's: what,
// with the error of a kind the plan checks that the unit's operation
// answered, if any; current is the page at a content op's path then.
func (a *batchAttempt) differs(i int, what string, err error, current *domain.VersionToken) error {
	return &replanned{op: i, what: what, err: err, current: current}
}

// failed is err of the op i: a problem at the op, the batch's change of
// the notebook since its plan for one of a kind the plan checks; any other
// error as it is.
func (a *batchAttempt) failed(i int, err error) error {
	var problem *shared.Error
	switch {
	case !errors.As(err, &problem):
		return err
	case replans(problem):
		return a.differs(i, "tree", err, nil)
	}
	return &domain.OpFailed{Op: i, Tool: a.steps[i].Kind, Err: err}
}

// replans reports whether e is of a kind the plan checks, which the unit
// answering means the tree changed since the plan read it (M9/P3 design
// 3.6.2): a node not found, a rule's 422, a name taken, a cycle, a depth.
// A guard's refusal is none: it is answered as it is.
func replans(e *shared.Error) bool {
	switch e.Code {
	case domain.ErrNotFound.Code, shared.CodeValidationFailed, domain.ErrTitleTaken.Code, domain.ErrCycle.Code, domain.ErrTooDeep.Code:
		return true
	}
	return false
}

// create is a write that creates its page: the path walked, which must
// find the nodes the plan found, then each page the plan creates, with the
// id it gave it, the last with the page's final content.
func (a *batchAttempt) create(ctx context.Context, u *Unit, i int) error {
	s, ps := a.steps[i], a.plan.Steps[i]
	found, err := a.b.paths.Walk(ctx, a.notebookID, s.Path.Keys)
	var on *domain.AttachmentOnPath
	switch {
	case errors.As(err, &on):
		return a.differs(i, "node", nil, nil)
	case err != nil:
		return err
	case !slices.Equal(nodeIDs(found), ps.Found):
		return a.differs(i, "node", nil, a.pageAt(ctx, s, found))
	}
	pg := a.pages[ps.Node]
	for d, c := range ps.Created {
		draft := PageDraft{ID: c.ID, ParentID: c.ParentID, Title: c.Title, Facts: a.empty}
		if d == len(ps.Created)-1 {
			draft.Content, draft.Facts = pg.Content, a.facts[pg.ID]
		}
		n, err := u.CreatePage(ctx, draft)
		if err != nil {
			return a.failed(i, err)
		}
		a.done[i].Created = append(a.done[i].Created, Placed{Node: n})
		a.done[i].Node = n
	}
	a.created += len(ps.Created)
	a.revisions[pg.ID], a.wrote[pg.ID] = 1, true
	a.done[i].Path = ps.Path
	return nil
}

// pageAt is the version of the page found at the whole of s's path, in
// the unit's transaction, nil for none.
func (a *batchAttempt) pageAt(ctx context.Context, s domain.BatchStep, found []domain.Node) *domain.VersionToken {
	if len(found) != len(s.Path.Keys) || found[len(found)-1].Kind != domain.KindPage {
		return nil
	}
	v, err := currentVersion(ctx, a.b.states, found[len(found)-1].ID)
	if err != nil {
		return nil
	}
	return &v
}

// content is a write on a page there, with base_version or without, or an
// edit: the page its path names, which must be the plan's; at the page's
// first content op, its content row locked, which must be at the revision
// the plan read, and its final content written on it, nothing when the
// page holds it already.
func (a *batchAttempt) content(ctx context.Context, u *Unit, i int) error {
	s, ps := a.steps[i], a.plan.Steps[i]
	at, err := a.b.paths.Find(ctx, a.notebookID, s.Path)
	if err != nil {
		return a.failed(i, err)
	}
	a.done[i].Node, a.done[i].Path = at.Node, domain.PathOf(at.Ancestors, at.Node)
	if at.Node.ID != ps.Node {
		current, err := currentVersion(ctx, a.b.states, at.Node.ID)
		if err != nil {
			return a.differs(i, "node", nil, nil)
		}
		return a.differs(i, "node", nil, &current)
	}
	pg := a.pages[ps.Node]
	if pg.First != i || pg.Created {
		return nil
	}
	lock, err := a.b.nodes.LockContent(ctx, a.notebookID, at.Node.ID)
	if err != nil {
		return a.failed(i, found(err, domain.ErrNotFound))
	}
	if lock.Revision != pg.Revision {
		return a.differs(i, "version", nil, &domain.VersionToken{Node: at.Node.ID, Revision: lock.Revision})
	}
	revision, err := u.WriteContent(ctx, ContentWrite{NodeID: at.Node.ID, Content: pg.Content, Facts: a.facts[pg.ID], Base: lock.Revision})
	switch {
	case errors.Is(err, domain.ErrRevisionMismatch):
		// The row is locked and its revision compared: a defect, no 409,
		// so the problem is not wrapped.
		return fmt.Errorf("page: the batch's write of page %s met another's under its content's lock: %s", at.Node.ID, err.Error())
	case err != nil:
		return a.failed(i, err)
	}
	a.revisions[pg.ID] = revision
	if revision != lock.Revision {
		a.wrote[pg.ID] = true
	}
	return nil
}

// move is a move: the node from names and to's parent's line, which must
// be the plan's, then the unit's Relocate, its links followed at the
// batch's end unless update_links is false.
func (a *batchAttempt) move(ctx context.Context, u *Unit, i int) error {
	s, ps := a.steps[i], a.plan.Steps[i]
	at, err := a.b.paths.Find(ctx, a.notebookID, s.From)
	if err != nil {
		return a.failed(i, err)
	}
	if at.Node.ID != ps.Node {
		return a.differs(i, "node", nil, nil)
	}
	chain, err := a.b.paths.Walk(ctx, a.notebookID, s.To.Keys[:len(s.To.Keys)-1])
	var on *domain.AttachmentOnPath
	switch {
	case errors.As(err, &on):
		return a.differs(i, "node", nil, nil)
	case err != nil:
		return err
	case !slices.Equal(nodeIDs(chain), ps.Chain):
		return a.differs(i, "node", nil, nil)
	}
	var parent *uuid.UUID
	if len(chain) > 0 {
		parent = new(chain[len(chain)-1].ID)
	}
	moved, err := u.Relocate(ctx, at.Node.ID, Placement{ParentID: parent, Name: s.To.Titles[len(s.To.Titles)-1],
		Field: s.Args.Field("to"), KeepLinks: !s.UpdateLinks})
	if taken := (*domain.PathTaken)(nil); errors.As(err, &taken) {
		taken.Path = s.To.Written // to is the whole new path, which the answer says
	}
	if err != nil {
		return a.failed(i, err)
	}
	a.done[i].Node, a.done[i].Path = moved, domain.PathOf(chain, moved)
	return nil
}

// delete is a delete: the node its path names, which must be the plan's,
// and its subtree, which must hold the nodes the plan's group does, the
// deletions merged into it included; the plan judged recursive on its
// tree, where those were gone already.
func (a *batchAttempt) delete(ctx context.Context, u *Unit, i int) error {
	s, ps := a.steps[i], a.plan.Steps[i]
	at, err := a.b.paths.Find(ctx, a.notebookID, s.Path)
	if err != nil {
		return a.failed(i, err)
	}
	if at.Node.ID != ps.Node {
		return a.differs(i, "node", nil, nil)
	}
	sub, err := u.Subtree(ctx, at.Node.ID)
	if err != nil {
		return a.failed(i, err)
	}
	ids := make([]uuid.UUID, len(sub))
	for k, n := range sub {
		ids[k] = n.Node.ID
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	group := slices.Clone(ps.Subtree)
	slices.SortFunc(group, uuid.UUID.Compare)
	if !slices.Equal(ids, group) {
		return a.differs(i, "node", nil, nil)
	}
	a.done[i].Node, a.done[i].Path = at.Node, domain.PathOf(at.Ancestors, at.Node)
	if err := u.DeleteSubtree(ctx, sub); err != nil {
		return a.failed(i, err)
	}
	return nil
}

// participate has the unit's participants follow its renames and moves
// once (M9/P3 design 3.7.1): the pages the unit wrote, alive after it,
// with their own changes and links before; the former parents' paths; the
// nodes it deleted; its bound. None follows a batch without a rename or a
// move with update_links. The facts' shares the batch holds are given back
// first, the facts kept: the rewrite's read backs must find the budget. Its
// errors are at the first such op.
func (a *batchAttempt) participate(ctx context.Context, u *Unit, parents map[uuid.UUID][]domain.PathStep) error {
	first := -1
	for i, s := range a.steps {
		if s.Kind == domain.OpKindMove && s.UpdateLinks && a.plan.Steps[i].Changed {
			first = i
			break
		}
	}
	a.written = len(a.wrote)
	if first < 0 {
		return nil
	}
	final := a.plan.TreeAt(len(a.steps) - 1)
	gone := final.Gone()
	written := map[uuid.UUID]WrittenPage{}
	for id := range a.wrote {
		if !slices.Contains(gone, id) {
			written[id] = a.befores[id]
		}
	}
	a.session.Release()
	n, err := u.Participate(ctx, BatchRewrite{Written: written, Parents: parents, Gone: gone, MaxPages: a.b.maxRewrites, FirstMove: first})
	var problem *shared.Error
	if errors.As(err, &problem) {
		return &domain.OpFailed{Op: first, Tool: a.steps[first].Kind, Err: err}
	}
	a.rewritten = n
	return err
}

// answer reads what the answer tells, in the unit's transaction after its
// last write (M9/P3 design 3.6.3): the versions of the pages the content
// ops wrote, alive, and the paths of the nodes the ops acted on and
// created, alive; a node gone keeps its path at its op, a page gone the
// version its composed write left.
func (a *batchAttempt) answer(ctx context.Context) error {
	var pages, ids []uuid.UUID
	for id := range a.pages {
		pages = append(pages, id)
	}
	for _, d := range a.done {
		ids = append(ids, d.Node.ID)
		for _, c := range d.Created {
			ids = append(ids, c.Node.ID)
		}
	}
	states, err := a.b.states.PageStates(ctx, pages)
	if err != nil {
		return err
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	paths, err := a.b.paths.Paths(ctx, a.notebookID, slices.Compact(ids))
	if err != nil {
		return err
	}
	for i, s := range a.steps {
		d := &a.done[i]
		if path, ok := paths[d.Node.ID]; ok {
			d.Path = path
		}
		for k := range d.Created {
			d.Created[k].Path = paths[d.Created[k].Node.ID]
		}
		if s.Kind != domain.OpKindWrite && s.Kind != domain.OpKindEdit {
			continue
		}
		revision := a.revisions[d.Node.ID]
		if state, ok := states[d.Node.ID]; ok {
			revision = state.Revision
		}
		d.Version = domain.VersionToken{Node: d.Node.ID, Revision: revision}
	}
	return nil
}

// nodeIDs is the ids of nodes, in their order.
func nodeIDs(nodes []domain.Node) []uuid.UUID {
	out := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// planLinks is a batch's plan's LinkRewriter (M9/P3 design 3.4.4): the
// module's BatchLinks on the plan's trees, each content parsed in the
// request's session once the plan admitted it. It holds the request's
// context: the domain's plan calls it without one.
type planLinks struct {
	ctx context.Context
	a   *batchAttempt
}

func (r *planLinks) Indexed(id uuid.UUID, revision int) (domain.LinksBefore, bool, error) {
	return r.a.b.links.Indexed(r.ctx, r.a.notebookID, id, revision, r.a.plan.TreeAt(-1))
}

func (r *planLinks) Resolve(id uuid.UUID, content string, at int) (domain.LinksBefore, error) {
	return r.a.b.links.Resolve(r.ctx, LinksOn{NotebookID: r.a.notebookID, PageID: id, Content: content, Tree: r.a.plan.TreeAt(at),
		Parse: r.parse(id)})
}

func (r *planLinks) Rewrite(id uuid.UUID, content string, before domain.LinksBefore, since, until int) (string, error) {
	changes := r.a.plan.MovesBetween(since, until)
	return r.a.b.links.Rewrite(r.ctx, LinksMoved{
		LinksOn: LinksOn{NotebookID: r.a.notebookID, PageID: id, Content: content, Tree: r.a.plan.TreeAt(until), Parse: r.parse(id)},
		Before:  before, Changes: changes, Parents: r.a.plan.ParentsAt(since, changes),
	})
}

// parse is the plan's parse of the page id's contents: admitted by the
// plan within its bounds, then the session's.
func (r *planLinks) parse(id uuid.UUID) func(ctx context.Context, content string) (Facts, error) {
	return func(ctx context.Context, content string) (Facts, error) {
		if err := r.a.plan.Admit(id, content); err != nil {
			return nil, err
		}
		return r.a.session.Parse(ctx, id, content)
	}
}
