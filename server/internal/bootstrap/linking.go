package bootstrap

import (
	"context"
	"encoding/json"
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
	changes := make([]linking.Change, len(e.Changes))
	for i, c := range e.Changes {
		changes[i] = linking.Change{NodeID: c.NodeID, Revision: c.Revision}
		if c.Before != nil {
			changes[i].Before = &linking.Place{ParentID: c.Before.ParentID, Name: c.Before.Name}
		}
		if c.After != nil {
			changes[i].After = &linking.Place{ParentID: c.After.ParentID, Name: c.After.Name}
		}
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

// linkTargets is what linking reads of the pages, page's LinkTargets.
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

func linkNodes(nodes []page.LinkNode) []linking.Node {
	out := make([]linking.Node, len(nodes))
	for i, n := range nodes {
		out[i] = linking.Node{ID: n.ID, Path: linkSteps(n.Path)}
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
