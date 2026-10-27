package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PageEdit is edit's arguments (M9 design 4.10).
type PageEdit struct {
	Path     string
	Old, New string
	All      bool    // replace_all
	Base     *string // base_version as given, nil without one
}

// Edited is an edit's outcome: the page, its version, how many times
// old_string was replaced, and whether the edit was made again once
// (Redone) after the page changed between its read and its write.
// Outcome's ChangesetID is zero when nothing changed.
type Edited struct {
	Path         string
	Node         domain.Node
	Version      domain.VersionToken
	Replacements int
	Redone       bool
	Outcome      Outcome
}

// EditPage replaces a string in a page's content on the server, atomically
// (M9 design 4.10; M9/P2 design 3.8.2): read unlocked, replaced, parsed,
// then written with the revision it read as the unit's Base, the path
// found again under the lock; once more from the read when the page or the
// path moved on in between, without Base.
type EditPage struct {
	writer    *Writer
	parser    *ContentParser
	paths     NodePaths
	nodes     Nodes
	states    PageStates
	locks     *LockHolders
	notebooks Notebooks
	auth      shared.Authorizer
	clock     Clock
	logger    *slog.Logger
}

// NewEditPage returns the use case.
func NewEditPage(writer *Writer, parser *ContentParser, paths PathStore, nodes Nodes, states PageStates,
	locks *LockHolders, notebooks Notebooks, auth shared.Authorizer, clock Clock, logger *slog.Logger,
) *EditPage {
	return &EditPage{writer: writer, parser: parser, paths: NewNodePaths(paths), nodes: nodes, states: states, locks: locks,
		notebooks: notebooks, auth: auth, clock: clock, logger: logger}
}

// errRedo is an attempt's unit that found the page moved on since its
// read: the path names another page, or the content has another revision.
var errRedo = errors.New("page: the page moved on between the edit's read and its unit")

// Execute edits the page at p's path in the notebook notebookID from
// client. Its order: the actor; the path, a page's, the two strings and
// the token, 422; the notebook, notebook.not_found; then an attempt: the
// page, page.not_found; page.write's 403, on the first; its content read,
// a token that is not its version 409, old_string replaced, 422; an alive
// edit session of the page, page.locked, before the parse's 503; and the
// unit, which finds the page again and writes on the revision read. A unit
// that finds the page moved on is made again from the read once, without
// Base: 409 page.revision_mismatch with Base, and when it moved on twice.
// A refusal of an edit lock carries the page's path.
func (e *EditPage) Execute(ctx context.Context, notebookID uuid.UUID, p PageEdit, client domain.Client) (Edited, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Edited{}, err
	}
	path, err := domain.ParsePath("path", p.Path, false)
	if err != nil {
		return Edited{}, err
	}
	if err := domain.PagePath(domain.Args{}, path, "edit"); err != nil {
		return Edited{}, err
	}
	if err := domain.CheckEdit(domain.Args{}, p.Old, p.New); err != nil {
		return Edited{}, err
	}
	var base *domain.VersionToken
	if p.Base != nil {
		t, err := domain.ParseVersionToken("base_version", *p.Base)
		if err != nil {
			return Edited{}, err
		}
		base = &t
	}
	if err := decideNotebook(ctx, e.notebooks, e.auth, actor, domain.ActionRead, notebookID); err != nil {
		return Edited{}, err
	}
	out, err := e.attempt(ctx, actor, notebookID, path, p, base, client, true)
	switch {
	case !errors.Is(err, errRedo):
	case base != nil:
		return Edited{}, e.mismatch(ctx, notebookID, path, base.Node)
	default:
		if out, err = e.attempt(ctx, actor, notebookID, path, p, nil, client, false); errors.Is(err, errRedo) {
			return Edited{}, e.mismatch(ctx, notebookID, path, uuid.UUID{})
		}
		out.Redone = true
	}
	if err != nil {
		return Edited{}, withLockPaths(ctx, e.paths, notebookID, e.logger, err)
	}
	if out.Outcome.ChangesetID != (uuid.UUID{}) {
		e.logger.InfoContext(ctx, "page edited", append(logged(out.Outcome, out.Node, client), slog.Int("revision", out.Version.Revision),
			slog.Int("replacements", out.Replacements), slog.Bool("redone", out.Redone))...)
	}
	return out, nil
}

// attempt is one try of the edit, from the read of the page to its unit,
// the parse's budget held for the try alone: errRedo when the unit finds
// the page moved on since the read. The first decides page.write; base,
// when set, must be the page's version as read.
func (e *EditPage) attempt(ctx context.Context, actor shared.Actor, notebookID uuid.UUID, path domain.Path, p PageEdit,
	base *domain.VersionToken, client domain.Client, first bool,
) (Edited, error) {
	at, err := e.paths.Find(ctx, notebookID, path)
	if err != nil {
		return Edited{}, err
	}
	if first {
		if err := decideNotebook(ctx, e.notebooks, e.auth, actor, domain.ActionWrite, notebookID); err != nil {
			return Edited{}, err
		}
	}
	read, err := e.nodes.PageContent(ctx, at.Node.ID)
	if err != nil {
		return Edited{}, found(err, domain.ErrNotFound)
	}
	version := domain.VersionToken{Node: at.Node.ID, Revision: read.Revision}
	switch {
	case base != nil && base.Node != version.Node:
		return Edited{}, &domain.VersionMismatch{Path: path.Written, Current: version, Why: domain.OtherNode}
	case base != nil && base.Revision != version.Revision:
		return Edited{}, &domain.VersionMismatch{Path: path.Written, Current: version, Why: domain.Changed}
	}
	r, err := domain.Replace(domain.Args{}, read.Content, p.Old, p.New, p.All)
	if err != nil {
		return Edited{}, err
	}
	if r.Content != read.Content {
		notebook, ok, err := e.notebooks.StateOf(ctx, notebookID)
		switch {
		case err != nil:
			return Edited{}, err
		case !ok:
			return Edited{}, domain.ErrNotebookNotFound
		}
		if notebook.Mode == domain.ModeLLMWiki {
			state := at.Node.State()
			facts := domain.FactsOf(domain.OpContent, domain.ChangesetEdit, []domain.Change{{NodeID: at.Node.ID, Before: &state, After: &state}})
			facts.Nodes[0].Kind = domain.KindPage
			hash := sha256.Sum256([]byte(r.Content))
			facts.Content = &domain.ContentFacts{Hash: hash[:], Bytes: len(r.Content), BeforeBytes: len(read.Content), Merged: 1}
			line := make([]domain.Ancestor, len(at.Ancestors))
			for i, n := range at.Ancestors {
				line[i] = domain.Ancestor{ID: n.ID, Name: n.Name}
			}
			facts = domain.SourceFacts(facts, notebook.SourceRootID, line, line, nil)
			if v := domain.CheckContract(notebook, facts); v.Err != nil {
				return Edited{}, domain.Refuse(v.Err, v.NodeID)
			}
		}
	}
	// An editor open before the call is the likeliest refusal: answered
	// before the parse takes its budget (M9/P2 design review A-M4).
	holders, err := e.locks.Of(ctx, []uuid.UUID{at.Node.ID}, e.clock.Now())
	if err != nil {
		return Edited{}, err
	}
	if len(holders) > 0 {
		return Edited{}, domain.Locked(holders[0].PageID, holders[0].UserID, holders[0].DisplayName)
	}
	facts, release, err := e.parser.Decided(ctx, r.Content)
	if err != nil {
		return Edited{}, err
	}
	defer release()
	out := Edited{Replacements: r.Count}
	spec := UnitSpec{NotebookID: notebookID, Action: domain.ActionWrite, Client: client, Options: Options{UpdateLinks: true},
		NotFound: domain.ErrNotebookNotFound}
	out.Outcome, err = e.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		now, err := e.paths.Find(ctx, notebookID, path)
		switch {
		case err != nil:
			return err
		case now.Node.ID != at.Node.ID:
			return errRedo
		}
		revision, err := u.WriteContent(ctx, ContentWrite{NodeID: now.Node.ID, Content: r.Content, Facts: facts, Base: read.Revision})
		switch {
		case errors.Is(err, domain.ErrRevisionMismatch):
			return errRedo
		case err != nil:
			return err
		}
		out.Path, out.Node = domain.PathOf(now.Ancestors, now.Node), now.Node
		out.Version = domain.VersionToken{Node: now.Node.ID, Revision: revision}
		return nil
	})
	return out, err
}

// mismatch is page.revision_mismatch at path once a unit found the page
// moved on since its read, with the version there now, read unlocked
// outside the unit: another page than node's there, or node's changed,
// for a token of node; for none (node zero), the edit made again found it
// moved on too.
func (e *EditPage) mismatch(ctx context.Context, notebookID uuid.UUID, path domain.Path, node uuid.UUID) error {
	at, err := e.paths.Find(ctx, notebookID, path)
	if err != nil {
		return err
	}
	current, err := currentVersion(ctx, e.states, at.Node.ID)
	if err != nil {
		return err
	}
	why := domain.KeptChanging
	switch {
	case node != uuid.UUID{} && node != at.Node.ID:
		why = domain.OtherNode
	case node != uuid.UUID{}:
		why = domain.Changed
	}
	return &domain.VersionMismatch{Path: path.Written, Current: current, Why: why}
}
