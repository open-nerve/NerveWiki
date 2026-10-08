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
	return l.rewrite.Participate(ctx, m, linkAppender{u})
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

// linkTargets is what linking reads of the pages, page's LinkTargets: the
// observer's reads, and the rebuild's.
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

// linkEvents publishes linking's links events on the event stream (M6
// design 4.8): of the notebook, its data the two sets of pages, each null
// for too many.
type linkEvents struct {
	publisher *events.Publisher
}

// linksData is the data of a links event.
type linksData struct {
	Pages   []uuid.UUID `json:"pages"`
	Targets []uuid.UUID `json:"targets"`
}

func (l linkEvents) LinksChanged(ctx context.Context, c linking.LinksChanged) error {
	data, err := json.Marshal(linksData{Pages: c.Pages, Targets: c.Targets})
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
