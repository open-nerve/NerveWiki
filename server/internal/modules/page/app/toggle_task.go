package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ToggleTask ticks or clears a task item of a page's content: POST
// /api/v0/pages/{page_id}/toggle-task (M5/P6 design 3.3).
type ToggleTask struct {
	writer   *Writer
	nodes    Nodes
	parser   *ContentParser
	markdown Markdown
	logger   *slog.Logger
}

// NewToggleTask returns the use case.
func NewToggleTask(writer *Writer, nodes Nodes, parser *ContentParser, markdown Markdown, logger *slog.Logger) *ToggleTask {
	return &ToggleTask{writer: writer, nodes: nodes, parser: parser, markdown: markdown, logger: logger}
}

// TaskToggle ticks (Checked) or clears the task item whose character is
// at Offset in the content at revision Base.
type TaskToggle struct {
	Base    int
	Offset  int
	Checked bool
}

// Execute toggles p in the page id's content, from client, in a unit that
// keeps the tree; it answers the page as the unit leaves it. The page is
// found unlocked and the write decided first: page.not_found, forbidden.
// Then its content is read, outside a transaction: a base that is not its
// revision is page.revision_mismatch (an offset means something in its
// revision only); then, parsed within the budget, an offset that is no
// task item's is validation_failed (out_of_range). An item in that state
// already writes nothing: the page as it is, page.revision_mismatch if a
// write came since. The new content is parsed (the write decided
// already), and one that has no item in that state at the offset is
// validation_failed (not_allowed): ticking "- [ ]: /u" makes a link
// reference definition. Then the unit
// writes it as PutPageContent writes a content in no edit session: the
// page's lock, its base, its observers.
func (t *ToggleTask) Execute(ctx context.Context, id uuid.UUID, p TaskToggle, client domain.Client) (PageView, error) {
	n, err := t.nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return PageView{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return PageView{}, domain.ErrNotFound
	}
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionToggleTask, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	if err := t.writer.Allowed(ctx, spec); err != nil {
		return PageView{}, err
	}
	current, err := t.nodes.PageContent(ctx, id)
	switch {
	case err != nil:
		return PageView{}, found(err, domain.ErrNotFound)
	case current.Revision != p.Base:
		return PageView{}, domain.ErrRevisionMismatch
	}
	task, err := t.taskAt(ctx, current.Content, p.Offset)
	switch {
	case err != nil:
		return PageView{}, err
	case task.Checked == p.Checked:
		return t.unchanged(ctx, id, p.Base)
	}
	content := domain.Flip(current.Content, p.Offset, p.Checked)
	parse, release, err := t.parser.Decided(ctx, content)
	if err != nil {
		return PageView{}, err
	}
	defer release()
	if task, ok := find(t.markdown.Tasks(parse), p.Offset); !ok || task.Checked != p.Checked {
		return PageView{}, domain.TaskWouldGo()
	}
	var out PageView
	outcome, err := t.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		if _, err := u.WriteContent(ctx, ContentWrite{NodeID: id, Content: content, Parsed: parse, Base: p.Base}); err != nil {
			return err
		}
		out, err = readPage(ctx, t.nodes, id)
		return err
	})
	if err != nil {
		return PageView{}, err
	}
	if outcome.ChangesetID != (uuid.UUID{}) {
		t.logger.InfoContext(ctx, "page task toggled", append(logged(outcome, out.Node, client),
			slog.Int("revision", out.Content.Revision), slog.Int("offset", p.Offset), slog.Bool("checked", p.Checked))...)
	}
	return out, nil
}

// unchanged answers the page id, its item in the state asked for at base
// already: the page as it is, unless a write came since, which is
// page.revision_mismatch, as a write on base would be.
func (t *ToggleTask) unchanged(ctx context.Context, id uuid.UUID, base int) (PageView, error) {
	v, err := readPage(ctx, t.nodes, id)
	switch {
	case err != nil:
		return PageView{}, err
	case v.Content.Revision != base:
		return PageView{}, domain.ErrRevisionMismatch
	}
	return v, nil
}

// taskAt is content's task item at offset, from its parse within the
// budget, which it gives back at once; 422 for none.
func (t *ToggleTask) taskAt(ctx context.Context, content string, offset int) (Task, error) {
	parse, release, err := t.parser.Decided(ctx, content)
	if err != nil {
		return Task{}, err
	}
	defer release()
	task, ok := find(t.markdown.Tasks(parse), offset)
	if !ok {
		return Task{}, domain.NotATask()
	}
	return task, nil
}

// find is the task item of tasks at offset.
func find(tasks []Task, offset int) (Task, bool) {
	for _, task := range tasks {
		if task.Offset == offset {
			return task, true
		}
	}
	return Task{}, false
}
