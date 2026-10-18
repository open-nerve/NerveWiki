package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PathWrite is write's arguments (M9 design 4.10): Base is base_version as
// given, nil without one.
type PathWrite struct {
	Path    string
	Content string
	Base    *string
}

// Written is a write's outcome: the page, its path by the names it has,
// its version, and the nodes the write created, from the root down, the
// page last (none when the page was there). Outcome's ChangesetID is zero
// when nothing changed.
type Written struct {
	Path    string
	Node    domain.Node
	Version domain.VersionToken
	Created []Placed
	Outcome Outcome
}

// Placed is a node and its path.
type Placed struct {
	Path string
	Node domain.Node
}

// WritePath writes a page's whole content by its path for MCP's write (M9
// design 4.10; M9/P2 design 3.8.1): without Base it creates the page with
// its missing parents, empty, in one unit; with Base it replaces the
// content of the page the path names, which Base's node must be.
type WritePath struct {
	writer    *Writer
	parser    *ContentParser
	paths     NodePaths
	nodes     Nodes
	states    PageStates
	notebooks Notebooks
	auth      shared.Authorizer
	logger    *slog.Logger
}

// NewWritePath returns the use case.
func NewWritePath(writer *Writer, parser *ContentParser, paths PathStore, nodes Nodes, states PageStates,
	notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger,
) *WritePath {
	return &WritePath{writer: writer, parser: parser, paths: NewNodePaths(paths), nodes: nodes, states: states,
		notebooks: notebooks, auth: auth, logger: logger}
}

// Execute writes p in the notebook notebookID from client. Its order: the
// actor; the path, a page's, the content and the token, 422; then, to
// create, the notebook and page.create, notebook.not_found and 403, the
// parse's 503, and in a unit that changes the tree the path walked under
// the lock, each step's rules (domain.CreationOf), a page there of the
// same content written as nothing, another 409; to replace, the notebook,
// the page, page.not_found, then page.write's 403, the parse's 503, and in
// a unit that keeps the tree the page found again under the lock, which
// must be the token's, the content written on the token's revision, 409
// otherwise. A refusal of an edit lock carries its pages' paths, read once
// the parse's budget is back.
func (w *WritePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathWrite, client domain.Client) (Written, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Written{}, err
	}
	path, err := domain.ParsePath("path", p.Path, false)
	if err != nil {
		return Written{}, err
	}
	if path.Kind != domain.KindPage {
		return Written{}, domain.NotAllowed("path",
			"is an attachment's path: write writes pages, whose paths end with .md; attachments are uploaded in the web")
	}
	if err := domain.CheckContent("content", p.Content); err != nil {
		return Written{}, err
	}
	var out Written
	if p.Base == nil {
		out, err = w.create(ctx, actor, notebookID, path, p.Content, client)
	} else {
		var base domain.VersionToken
		if base, err = domain.ParseVersionToken("base_version", *p.Base); err != nil {
			return Written{}, err
		}
		out, err = w.replace(ctx, actor, notebookID, path, p.Content, base, client)
	}
	if err != nil {
		return Written{}, withLockPaths(ctx, w.paths, notebookID, w.logger, err)
	}
	return out, nil
}

// create creates the page at path with its missing parents, empty, the
// page with content, in one unit and one changeset; a page there already
// of the same bytes is answered as it is, nothing written.
func (w *WritePath) create(ctx context.Context, actor shared.Actor, notebookID uuid.UUID, path domain.Path, content string,
	client domain.Client,
) (Written, error) {
	if err := decideNotebook(ctx, w.notebooks, w.auth, actor, domain.ActionCreate, notebookID); err != nil {
		return Written{}, err
	}
	facts, release, err := w.parser.Decided(ctx, content)
	if err != nil {
		return Written{}, err
	}
	defer release()
	empty, done, err := w.parser.Decided(ctx, "") // an empty parent's facts: no budget taken
	if err != nil {
		return Written{}, err
	}
	defer done()
	var out Written
	spec := UnitSpec{NotebookID: notebookID, Action: domain.ActionCreate, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound}
	outcome, err := w.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		found, err := w.paths.Walk(ctx, notebookID, path.Keys)
		if on := (*domain.AttachmentOnPath)(nil); err != nil && !errors.As(err, &on) {
			return err // an attachment on the way is CreationOf's to refuse
		}
		c, err := domain.CreationOf("path", path, found)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(path.Keys))
		for _, n := range found {
			names = append(names, n.Name)
		}
		if c.Leaf != nil {
			return w.leaf(ctx, notebookID, path, content, *c.Leaf, names, &out)
		}
		parent := c.Parent
		for i, title := range c.Titles {
			d := PageDraft{ParentID: parent, Title: title, Facts: empty}
			if i == len(c.Titles)-1 {
				d.Content, d.Facts = content, facts
			}
			n, err := u.CreatePage(ctx, d)
			if err != nil {
				return err
			}
			names = append(names, n.Name)
			out.Created = append(out.Created, Placed{Path: domain.PathOfNames(names, domain.KindPage), Node: n})
			parent = &n.ID
		}
		page := out.Created[len(out.Created)-1]
		out.Path, out.Node, out.Version = page.Path, page.Node, domain.VersionToken{Node: page.Node.ID, Revision: 1}
		return nil
	})
	if err != nil {
		return Written{}, err
	}
	out.Outcome = outcome
	if outcome.ChangesetID != (uuid.UUID{}) {
		w.logger.InfoContext(ctx, "page created", append(logged(outcome, out.Node, client), slog.Int("pages", len(out.Created)))...)
	}
	return out, nil
}

// leaf answers a creation at a path whose page is there: its version when
// it holds content's bytes already, nothing written; page.title_taken
// otherwise. Its content's row is locked, harmless in a unit that changes
// the tree.
func (w *WritePath) leaf(ctx context.Context, notebookID uuid.UUID, path domain.Path, content string, n domain.Node,
	names []string, out *Written,
) error {
	current, err := w.nodes.LockContent(ctx, notebookID, n.ID)
	if err != nil {
		return found(err, domain.ErrNotFound)
	}
	sum := sha256.Sum256([]byte(content))
	if current.ByteSize != len(content) || !bytes.Equal(current.Hash, sum[:]) {
		return &domain.PathTaken{Path: path.Written}
	}
	out.Path, out.Node = domain.PathOfNames(names, domain.KindPage), n
	out.Version = domain.VersionToken{Node: n.ID, Revision: current.Revision}
	return nil
}

// replace writes content as the page's at path, which must be base's node,
// on base's revision.
func (w *WritePath) replace(ctx context.Context, actor shared.Actor, notebookID uuid.UUID, path domain.Path, content string,
	base domain.VersionToken, client domain.Client,
) (Written, error) {
	if err := decideNotebook(ctx, w.notebooks, w.auth, actor, domain.ActionRead, notebookID); err != nil {
		return Written{}, err
	}
	if _, err := w.paths.Find(ctx, notebookID, path); err != nil {
		return Written{}, err
	}
	if err := decideNotebook(ctx, w.notebooks, w.auth, actor, domain.ActionWrite, notebookID); err != nil {
		return Written{}, err
	}
	facts, release, err := w.parser.Decided(ctx, content)
	if err != nil {
		return Written{}, err
	}
	defer release()
	var out Written
	spec := UnitSpec{NotebookID: notebookID, Action: domain.ActionWrite, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound}
	outcome, err := w.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		at, err := w.paths.Find(ctx, notebookID, path)
		if err != nil {
			return err
		}
		if at.Node.ID != base.Node {
			return w.mismatch(ctx, path, at.Node.ID, domain.OtherNode)
		}
		revision, err := u.WriteContent(ctx, ContentWrite{NodeID: at.Node.ID, Content: content, Facts: facts, Base: base.Revision})
		if errors.Is(err, domain.ErrRevisionMismatch) {
			return w.mismatch(ctx, path, at.Node.ID, domain.Changed)
		}
		if err != nil {
			return err
		}
		out.Path, out.Node = domain.PathOf(at.Ancestors, at.Node), at.Node
		out.Version = domain.VersionToken{Node: at.Node.ID, Revision: revision}
		return nil
	})
	if err != nil {
		return Written{}, err
	}
	out.Outcome = outcome
	if outcome.ChangesetID != (uuid.UUID{}) {
		w.logger.InfoContext(ctx, "page content written", append(logged(outcome, out.Node, client), slog.Int("revision", out.Version.Revision))...)
	}
	return out, nil
}

// mismatch is page.revision_mismatch at path for why, with the version of
// the page id there now, read in the transaction ctx carries.
func (w *WritePath) mismatch(ctx context.Context, path domain.Path, id uuid.UUID, why domain.Mismatch) error {
	current, err := currentVersion(ctx, w.states, id)
	if err != nil {
		return err
	}
	return &domain.VersionMismatch{Path: path.Written, Current: current, Why: why}
}
