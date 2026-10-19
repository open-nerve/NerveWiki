package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PathMove is move's arguments (M9 design 4.10): UpdateLinks is
// update_links, true when the agent left it out.
type PathMove struct {
	From, To    string
	UpdateLinks bool
}

// Relocated is a move's outcome: the node at its path after. Outcome's
// ChangesetID is zero when nothing changed.
type Relocated struct {
	Path    string
	Node    domain.Node
	Outcome Outcome
}

// RelocatePath moves, renames or both a page or an attachment from its
// path to its full new path in one step (Unit.Relocate; M9/P2 design
// 3.8.3), its links rewritten unless UpdateLinks is false.
type RelocatePath struct {
	writer    *Writer
	paths     NodePaths
	notebooks Notebooks
	auth      shared.Authorizer
	logger    *slog.Logger
}

// NewRelocatePath returns the use case.
func NewRelocatePath(writer *Writer, paths PathStore, notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger) *RelocatePath {
	return &RelocatePath{writer: writer, paths: NewNodePaths(paths), notebooks: notebooks, auth: auth, logger: logger}
}

// Execute moves the node at p's from to p's to in the notebook notebookID
// from client: to is the node's whole new path, never a page to move it
// into. Its order: the actor; the two paths and their kinds, 422; the
// notebook, notebook.not_found; the node, page.not_found; node.rename when
// to's parent is from's, node.move otherwise, 403; then, in a unit that
// changes the tree, the node found again under the lock, to's parent
// walked, and Unit.Relocate's 422 and 409s, a name taken answered at to. A
// move to where the node is, so named, writes nothing. A refusal of an
// edit lock carries its pages' paths.
func (m *RelocatePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathMove, client domain.Client) (Relocated, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Relocated{}, err
	}
	from, err := domain.ParsePath("from", p.From, false)
	if err != nil {
		return Relocated{}, err
	}
	to, err := domain.ParsePath("to", p.To, false)
	if err != nil {
		return Relocated{}, err
	}
	if err := domain.MoveKinds(from, to); err != nil {
		return Relocated{}, err
	}
	if err := decideNotebook(ctx, m.notebooks, m.auth, actor, domain.ActionRead, notebookID); err != nil {
		return Relocated{}, err
	}
	if _, err := m.paths.Find(ctx, notebookID, from); err != nil {
		return Relocated{}, err
	}
	// One chain of keys is one parent: under the lock, the node they find.
	action := domain.ActionMove
	if slices.Equal(from.Keys[:len(from.Keys)-1], to.Keys[:len(to.Keys)-1]) {
		action = domain.ActionRename
	}
	if err := decideNotebook(ctx, m.notebooks, m.auth, actor, action, notebookID); err != nil {
		return Relocated{}, err
	}
	var out Relocated
	spec := UnitSpec{NotebookID: notebookID, Action: action, Tree: true, Client: client,
		Options: Options{UpdateLinks: p.UpdateLinks}, NotFound: domain.ErrNotebookNotFound}
	outcome, err := m.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		at, err := m.paths.Find(ctx, notebookID, from)
		if err != nil {
			return err
		}
		chain, err := m.parentOf(ctx, notebookID, to)
		if err != nil {
			return err
		}
		var parent *uuid.UUID
		if len(chain) > 0 {
			parent = new(chain[len(chain)-1].ID)
		}
		moved, err := u.Relocate(ctx, at.Node.ID, Placement{ParentID: parent, Name: to.Titles[len(to.Titles)-1], Field: "to"})
		if taken := (*domain.PathTaken)(nil); errors.As(err, &taken) {
			taken.Path = to.Written // to is the whole new path, which the answer says
		}
		if err != nil {
			return err
		}
		out.Path, out.Node = domain.PathOf(chain, moved), moved
		return nil
	})
	if err != nil {
		return Relocated{}, withLockPaths(ctx, m.paths, notebookID, m.logger, err)
	}
	out.Outcome = outcome
	if outcome.ChangesetID != (uuid.UUID{}) {
		msg := "node moved"
		if action == domain.ActionRename {
			msg = "node renamed"
		}
		m.logger.InfoContext(ctx, msg, append(logged(outcome, out.Node, client), slog.Bool("update_links", p.UpdateLinks))...)
	}
	return out, nil
}

// parentOf is the pages from the root to the parent to asks for, none for
// the root, walked in the unit's transaction: the first segment that fails
// decides. One that names an attachment is 422 on to, an attachment holds
// no nodes; one that names nothing is page.not_found at to's parent's
// path, a move creating no parent pages, unless a segment that names
// nothing ends with .md: then write's 422 on to, the parent written as a
// page's path (domain.MissingAncestors).
func (m *RelocatePath) parentOf(ctx context.Context, notebookID uuid.UUID, to domain.Path) ([]domain.Node, error) {
	keys := to.Keys[:len(to.Keys)-1]
	chain, err := m.paths.Walk(ctx, notebookID, keys)
	var on *domain.AttachmentOnPath
	switch {
	case errors.As(err, &on):
		return nil, holdsNoNodes(on.Segment)
	case err != nil:
		return nil, err
	case len(chain) < len(keys):
		if err := domain.MissingAncestors("to", to, len(chain)); err != nil {
			return nil, err
		}
		return nil, &domain.PathNotFound{Path: domain.PathOfNames(to.Titles[:len(keys)], domain.KindPage),
			Hint: domain.MoveParentHint, ParentOf: "to"}
	case len(chain) > 0 && chain[len(chain)-1].Kind != domain.KindPage:
		return nil, holdsNoNodes(len(chain))
	}
	return chain, nil
}

// holdsNoNodes is 422 on to: its segment segment names an attachment,
// where to goes on under it.
func holdsNoNodes(segment int) error {
	return domain.NotAllowed("to", fmt.Sprintf("segment %d is an attachment, which holds no nodes", segment))
}
