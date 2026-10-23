package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// The link index's parts (M6/P3 design 3.5): the page and linking modules
// do not import each other, so their values meet here.

// linkIndex is linking's observer of the page writes, as the page module
// calls it: each change's facts are read as the index keeps them.
type linkIndex struct {
	index linking.Index
}

func (l linkIndex) PagesChanged(ctx context.Context, e page.Event) error {
	changes := linkChanges(e.Changes)
	for i, c := range e.Changes {
		if c.Revision == 0 {
			continue
		}
		facts, err := linking.PageFacts(c.Facts)
		if err != nil {
			return fmt.Errorf("the facts of page %s: %w", c.NodeID, err)
		}
		changes[i].Facts = facts
	}
	return l.index.PagesChanged(ctx, linking.PagesChanged{WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, Changes: changes})
}

// linkChanges are the page module's changes as linking reads them, without
// their facts.
func linkChanges(changes []page.Change) []linking.Change {
	out := make([]linking.Change, len(changes))
	for i, c := range changes {
		out[i] = linking.Change{NodeID: c.NodeID, Revision: c.Revision}
		if c.Before != nil {
			out[i].Before = &linking.Place{ParentID: c.Before.ParentID, Name: c.Before.Name}
		}
		if c.After != nil {
			out[i].After = &linking.Place{ParentID: c.After.ParentID, Name: c.After.Name}
		}
	}
	return out
}

// linkRewrite is linking's participant of the page write units, as the
// page module calls it (M6/P4 design 4.1): the step's changes as linking
// reads them, the unit's appender adapted.
type linkRewrite struct {
	rewrite linking.Rewrite
}

func (l linkRewrite) Participate(ctx context.Context, s page.Step, u page.Appender) error {
	m := linking.Moved{NotebookID: s.NotebookID, At: s.At, UpdateLinks: s.Options.UpdateLinks, Changes: linkChanges(s.Changes)}
	if s.Batch != nil {
		if err := batchMoved(&m, *s.Batch); err != nil {
			return err
		}
	}
	err := l.rewrite.Participate(ctx, m, linkAppender{u})
	if tooMany := (*linking.TooManyRewrites)(nil); errors.As(err, &tooMany) {
		const hint = "move with update_links false, or in separate calls"
		if tooMany.Links > 0 {
			return page.BatchTooLarge("links the batch's renames and moves would rewrite", tooMany.Links, tooMany.Max, hint)
		}
		if tooMany.Bytes > 0 {
			return page.BatchTooLarge("bytes of pages the batch's renames and moves would rewrite", tooMany.Bytes, tooMany.Max, hint)
		}
		if tooMany.Kept > 0 {
			return page.BatchTooLarge("bytes of the parse budget the pages the batch's renames and moves rewrite would hold",
				tooMany.Kept, tooMany.Max, hint)
		}
		what := "pages whose links the batch's renames and moves would rewrite"
		if tooMany.Weight > 0 {
			what += fmt.Sprintf(", each page the batch wrote counting %d", tooMany.Weight)
		}
		return page.BatchTooLarge(what, tooMany.Pages, tooMany.Max, hint)
	}
	return err
}

// batchMoved has m, a batch's deferred step, follow what the batch tells
// besides its changes (M9/P3 design 3.7.1): the pages it wrote, each with
// its links before, its own changes, its final content's facts and the nodes
// it leaves the links to; the former parents' paths, never nil, which the
// rewrite reads them from alone; the nodes it deleted; its bounds, of pages,
// and what a page it wrote counts in it (M9/P3 review B-M3), of links
// (review B1-M3) and of their bytes (fix round 5, B2-M1); the nodes whose
// links it leaves (M9/P3 review B-M1), with their moves, for the paths
// before alone (review B1-N1); and those it created under each node after
// its last rename or move, out of its subtree (M9/P3 review B1-M1).
func batchMoved(m *linking.Moved, b page.BatchRewrite) error {
	m.Written = make(map[uuid.UUID]linking.WrittenPage, len(b.Written))
	for id, w := range b.Written {
		facts, err := linking.PageFacts(w.Facts)
		if err != nil {
			return fmt.Errorf("the facts of page %s: %w", id, err)
		}
		m.Written[id] = linking.WrittenPage{Before: linkLinksBefore(w.Before), Changes: linkChanges(w.Changes), Facts: facts,
			Frozen: w.Frozen, CreatedAfter: w.CreatedAfter}
	}
	m.Parents = linkParents(b.Parents)
	m.Gone, m.MaxPages, m.WrittenWeight, m.MaxLinks, m.MaxBytes = b.Gone, b.MaxPages, b.WrittenWeight, b.MaxLinks, b.MaxBytes
	m.Frozen, m.FrozenMoves, m.CreatedAfter = b.Frozen, linkChanges(b.FrozenMoves), b.CreatedAfter
	return nil
}

// linkParents are the paths of the former parents as linking reads them,
// never nil.
func linkParents(parents map[uuid.UUID][]page.PathStep) map[uuid.UUID][]linking.Step {
	out := make(map[uuid.UUID][]linking.Step, len(parents))
	for id, path := range parents {
		out[id] = planSteps(path)
	}
	return out
}

// linkLinksBefore is where a page's links led at a moment of a batch, as
// linking reads it: each lead field by field, a link ambiguous then
// ambiguous still (M9/P3 review A-M3), and the targets' paths.
func linkLinksBefore(b page.LinksBefore) linking.LinksBefore {
	out := linking.LinksBefore{Leads: make(map[int]linking.Resolution, len(b.Leads)),
		Targets: make(map[uuid.UUID][]linking.Step, len(b.Targets))}
	for start, l := range b.Leads {
		out.Leads[start] = linking.Resolution{ID: l.ID, Ambiguous: l.Ambiguous, Asset: l.Asset}
	}
	for id, path := range b.Targets {
		out.Targets[id] = planSteps(path)
	}
	return out
}

// pageLinksBefore is linking's LinksBefore as the page module reads it.
func pageLinksBefore(b linking.LinksBefore) page.LinksBefore {
	out := page.LinksBefore{Leads: make(map[int]page.Lead, len(b.Leads)), Targets: make(map[uuid.UUID][]page.PathStep, len(b.Targets))}
	for start, r := range b.Leads {
		out.Leads[start] = page.Lead{ID: r.ID, Ambiguous: r.Ambiguous, Asset: r.Asset}
	}
	for id, path := range b.Targets {
		steps := make([]page.PathStep, len(path))
		for i, s := range path {
			steps[i] = page.PathStep{ID: s.ID, Key: s.Key, Name: s.Name}
		}
		out.Targets[id] = steps
	}
	return out
}

// batchLinks is linking's links of a batch's plan, as the page module
// calls them (M9/P3 design 3.7.4): the plan's tree as linking reads one,
// the plan's parse answering linking's facts, the leads and paths field by
// field.
type batchLinks struct {
	links linking.BatchLinks
}

func (b batchLinks) Indexed(ctx context.Context, notebookID, id uuid.UUID, revision int, tree page.PlanTree) (
	page.LinksBefore, bool, error,
) {
	before, ok, err := b.links.Indexed(ctx, notebookID, id, revision, planPages{tree})
	return pageLinksBefore(before), ok, err
}

func (b batchLinks) Reached(ctx context.Context, notebookID, id uuid.UUID, revision int, changes []page.Change,
	frozen []uuid.UUID, createdAfter map[uuid.UUID][]uuid.UUID, tree page.PlanTree, reads *page.TreeReads,
) (bool, error) {
	return b.links.Reached(ctx, notebookID, id, revision, linkChanges(changes), frozen, createdAfter, planPages{tree}, tree.Gone(),
		treeReads(reads))
}

func (b batchLinks) Resolve(ctx context.Context, q page.LinksOn) (page.LinksBefore, error) {
	before, err := b.links.Resolve(ctx, linksOn(q))
	return pageLinksBefore(before), err
}

func (b batchLinks) Rewrite(ctx context.Context, q page.LinksMoved) (string, error) {
	return b.links.Rewrite(ctx, linking.LinksMoved{LinksOn: linksOn(q.LinksOn), Before: linkLinksBefore(q.Before),
		Changes: linkChanges(q.Changes), Frozen: q.Frozen, CreatedAfter: q.CreatedAfter, Reads: treeReads(q.Reads)})
}

// treeReads is where linking keeps what it reads of a tree of the plan,
// nil for none.
func treeReads(r *page.TreeReads) *any {
	if r == nil {
		return nil
	}
	return &r.Held
}

// linksOn is a content of a batch's plan as linking resolves it: on the
// plan's tree, the nodes the batch deleted by then gone, parsed by the
// plan's parse.
func linksOn(q page.LinksOn) linking.LinksOn {
	return linking.LinksOn{NotebookID: q.NotebookID, PageID: q.PageID, Content: q.Content, Tree: planPages{q.Tree},
		Gone: q.Tree.Gone(), Parse: func(ctx context.Context, content string) (linking.Facts, error) {
			facts, err := q.Parse(ctx, content)
			if err != nil {
				return linking.Facts{}, err
			}
			return linking.PageFacts(facts)
		}}
}

// planPages is the tree of a batch's plan as linking reads a notebook's
// pages: of the plan's notebook alone, none in the trash.
type planPages struct {
	tree page.PlanTree
}

func (p planPages) ByKeys(_ context.Context, _ uuid.UUID, keys []string) ([]linking.Node, error) {
	return planNodes(p.tree.ByKeys(keys)), nil
}

func (p planPages) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]linking.Node, error) {
	return planNodes(p.tree.Paths(ids)), nil
}

func (p planPages) Subtree(_ context.Context, _, id uuid.UUID) ([]linking.Step, error) {
	return planSteps(p.tree.Subtree(id)), nil
}

func (p planPages) SourcePath(context.Context, uuid.UUID, uuid.UUID) (linking.Node, bool, error) {
	return linking.Node{}, false, errors.New("a batch's plan reads no page in the trash")
}

func planNodes(nodes []page.NodePath) []linking.Node {
	out := make([]linking.Node, len(nodes))
	for i, n := range nodes {
		out[i] = linking.Node{ID: n.ID, Path: planSteps(n.Path), Asset: n.Asset}
	}
	return out
}

func planSteps(steps []page.PathStep) []linking.Step {
	out := make([]linking.Step, len(steps))
	for i, s := range steps {
		out[i] = linking.Step{ID: s.ID, Key: s.Key, Name: s.Name}
	}
	return out
}

// linkAppender is the page module's appender as a rewrite adds to it: the
// edit lock's page.locked, which the guard answers for a page the
// precheck found free, is linking.ErrGuardLocked too.
type linkAppender struct {
	unit page.Appender
}

func (a linkAppender) WriteContent(ctx context.Context, w linking.Rewritten) error {
	_, err := a.unit.WriteContent(ctx, page.ContentWrite{NodeID: w.PageID, Base: w.Base, Content: w.Content, Facts: w.Facts})
	if errors.Is(err, page.ErrLocked) {
		return fmt.Errorf("%w: %w", linking.ErrGuardLocked, err)
	}
	return err
}

func (a linkAppender) Defer(f func()) {
	a.unit.Defer(f)
}

// linkTargets is what linking reads of the pages and attachments, page's
// LinkTargets: the observer's reads, the rebuild's, and the attachments'
// links'.
type linkTargets struct {
	page page.LinkTargets
}

func (l linkTargets) ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]linking.Node, error) {
	nodes, err := l.page.ByKeys(ctx, notebookID, keys)
	return linkNodes(nodes), err
}

func (l linkTargets) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]linking.Node, error) {
	nodes, err := l.page.Paths(ctx, notebookID, ids)
	return linkNodes(nodes), err
}

func (l linkTargets) SourcePath(ctx context.Context, notebookID, id uuid.UUID) (linking.Node, bool, error) {
	n, ok, err := l.page.SourcePath(ctx, notebookID, id)
	return linkNodes([]page.LinkNode{n})[0], ok, err
}

func (l linkTargets) Attachments(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]linking.Attachment, error) {
	attachments, err := l.page.Attachments(ctx, notebookID, ids)
	out := make([]linking.Attachment, len(attachments))
	for i, a := range attachments {
		out[i] = linking.Attachment{Node: linkNodes([]page.LinkNode{a.LinkNode})[0], Alike: a.Alike}
	}
	return out, err
}

func (l linkTargets) Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]linking.Step, error) {
	steps, err := l.page.Subtree(ctx, notebookID, id)
	return linkSteps(steps), err
}

func (l linkTargets) PageIDs(ctx context.Context, notebookID uuid.UUID) ([]uuid.UUID, error) {
	return l.page.PageIDs(ctx, notebookID)
}

func (l linkTargets) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	return l.page.Content(ctx, id)
}

func (l linkTargets) All(ctx context.Context, notebookID uuid.UUID) ([]linking.Node, error) {
	nodes, err := l.page.All(ctx, notebookID)
	return linkNodes(nodes), err
}

func (l linkTargets) NotebookOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return l.page.NotebookOf(ctx, id)
}

func (l linkTargets) Rekey(ctx context.Context, notebookID uuid.UUID) ([]linking.Clash, error) {
	clashes, err := l.page.Rekey(ctx, notebookID)
	out := make([]linking.Clash, len(clashes))
	for i, c := range clashes {
		out[i] = c
	}
	return out, err
}

func linkNodes(nodes []page.LinkNode) []linking.Node {
	out := make([]linking.Node, len(nodes))
	for i, n := range nodes {
		out[i] = linking.Node{ID: n.ID, Path: linkSteps(n.Path), Asset: n.Asset}
	}
	return out
}

func linkSteps(steps []page.LinkStep) []linking.Step {
	out := make([]linking.Step, len(steps))
	for i, s := range steps {
		out[i] = linking.Step(s)
	}
	return out
}

// linkingNodePaths is page's NodePaths as linking reads them (M9/P4
// design 3.9.1): MCP's backlinks and index, page's words in its errors.
type linkingNodePaths struct {
	page.NodePaths
}

func (l linkingNodePaths) Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]linking.PagePath, error) {
	pages, err := l.NodePaths.Pages(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]linking.PagePath, len(pages))
	for id, p := range pages {
		out[id] = linking.PagePath(p)
	}
	return out, nil
}

// linkEvents publishes linking's links events on the event stream (M6
// design 4.8): of the notebook, its data the two sets of pages, each null
// for too many, and whether the aliases changed (M8 Codex review R3).
type linkEvents struct {
	publisher *events.Publisher
}

// linksData is the data of a links event.
type linksData struct {
	Pages   []uuid.UUID `json:"pages"`
	Targets []uuid.UUID `json:"targets"`
	Aliases bool        `json:"aliases"`
}

func (l linkEvents) LinksChanged(ctx context.Context, c linking.LinksChanged) error {
	data, err := json.Marshal(linksData{Pages: c.Pages, Targets: c.Targets, Aliases: c.Aliases})
	if err != nil {
		return err
	}
	return l.publisher.Publish(ctx, events.Event{Type: "links", WorkspaceID: c.WorkspaceID, NotebookID: c.NotebookID, Data: data})
}

// linkNotebookDeletion is linking's part in a notebook's deletion, as the
// notebook module calls it.
type linkNotebookDeletion struct {
	linking linking.NotebookDeletion
}

func (d linkNotebookDeletion) NotebookDeleted(ctx context.Context, x notebook.NotebookDeletion) error {
	return d.linking.NotebookDeleted(ctx, linking.NotebooksDeleted{NotebookIDs: x.NotebookIDs})
}
