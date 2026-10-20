package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PathDelete is delete's arguments (M9 design 4.10).
type PathDelete struct {
	Path      string
	Recursive bool
}

// Deleted is a deletion's outcome: the node deleted and how many pages and
// attachments went into the trash with it, itself included.
type Deleted struct {
	Path          string
	Node          domain.Node
	Pages, Assets int
	Outcome       Outcome
}

// DeletePath puts a page or an attachment in the trash by its path; a page
// with children only with Recursive (M9/P2 design 3.8.4).
type DeletePath struct {
	writer    *Writer
	paths     NodePaths
	notebooks Notebooks
	auth      shared.Authorizer
	logger    *slog.Logger
}

// NewDeletePath returns the use case.
func NewDeletePath(writer *Writer, paths PathStore, notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger) *DeletePath {
	return &DeletePath{writer: writer, paths: NewNodePaths(paths), notebooks: notebooks, auth: auth, logger: logger}
}

// Execute deletes the node at p's path in the notebook notebookID, with
// its subtree, from client. Its order: the actor; the path, 422; the
// notebook, notebook.not_found; the node, page.not_found; node.delete's
// 403; then, in a unit that changes the tree, the node found again under
// the lock and its subtree read there: one with nodes under its root is
// 422 on recursive without it, a count that no unlocked read could keep;
// then the edit locks of the subtree, which an agent's deletion meets in
// any session (M9/P2 design 3.6), their refusal carrying their pages'
// paths.
func (d *DeletePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathDelete, client domain.Client) (Deleted, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Deleted{}, err
	}
	path, err := domain.ParsePath("path", p.Path, false)
	if err != nil {
		return Deleted{}, err
	}
	if err := decideNotebook(ctx, d.notebooks, d.auth, actor, domain.ActionRead, notebookID); err != nil {
		return Deleted{}, err
	}
	if _, err := d.paths.Find(ctx, notebookID, path); err != nil {
		return Deleted{}, err
	}
	if err := decideNotebook(ctx, d.notebooks, d.auth, actor, domain.ActionDelete, notebookID); err != nil {
		return Deleted{}, err
	}
	var out Deleted
	spec := UnitSpec{NotebookID: notebookID, Action: domain.ActionDelete, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound}
	outcome, err := d.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		at, err := d.paths.Find(ctx, notebookID, path)
		if err != nil {
			return err
		}
		sub, err := u.Subtree(ctx, at.Node.ID)
		if err != nil {
			return err
		}
		if err := domain.DeletionOf(domain.Args{}, sub, p.Recursive); err != nil {
			return err
		}
		out.Path, out.Node = domain.PathOf(at.Ancestors, at.Node), at.Node
		out.Pages, out.Assets = sub.Counts()
		return u.DeleteSubtree(ctx, sub)
	})
	if err != nil {
		return Deleted{}, withLockPaths(ctx, d.paths, notebookID, d.logger, err)
	}
	out.Outcome = outcome
	d.logger.InfoContext(ctx, "node deleted", append(logged(outcome, out.Node, client), slog.Int("nodes", out.Pages+out.Assets),
		slog.Int("pages", out.Pages), slog.Int("assets", out.Assets))...)
	return out, nil
}
