package app_test

import (
	"bytes"
	"cmp"
	"context"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is the clock's first instant.
func now() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }

// tickingClock reads now first, then a microsecond later each time: a unit
// that read it twice for what must be one time would write two.
type tickingClock struct{ reads int }

func (c *tickingClock) Now() time.Time {
	c.reads++
	return now().Add(time.Duration(c.reads-1) * time.Microsecond)
}

type txKey struct{}

// fakeTx runs fn with a context that says it is in a transaction, and
// records whether fn failed (a rollback).
type fakeTx struct{ rolledBack bool }

func (f *fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if f.InTx(ctx) {
		return fn(ctx)
	}
	err := fn(context.WithValue(ctx, txKey{}, true))
	f.rolledBack = err != nil
	return err
}

func (f *fakeTx) InTx(ctx context.Context) bool { return ctx.Value(txKey{}) != nil }

// recorder is the calls of every fake, in order, each with whether it ran
// in the transaction: the unit's order of locks, decision and writes.
type recorder struct{ calls []string }

func (r *recorder) record(ctx context.Context, call string) {
	if ctx.Value(txKey{}) != nil {
		call += " in tx"
	}
	r.calls = append(r.calls, call)
}

// fakeWorkspaces shares a workspace unless it is in gone.
type fakeWorkspaces struct {
	*recorder
	gone map[uuid.UUID]bool
}

func (f fakeWorkspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "ShareWorkspace")
	return !f.gone[id], nil
}

// fakeNotebooks knows each notebook's workspace; one in gone is deleted by
// the time it is locked.
type fakeNotebooks struct {
	*recorder
	workspaces map[uuid.UUID]uuid.UUID
	gone       map[uuid.UUID]bool
}

func (f fakeNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	f.record(ctx, "WorkspaceOf")
	ws, ok := f.workspaces[id]
	return ws, ok, nil
}

func (f fakeNotebooks) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "ShareNotebook")
	_, ok := f.workspaces[id]
	return ok && !f.gone[id], nil
}

func (f fakeNotebooks) LockByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "LockNotebook")
	_, ok := f.workspaces[id]
	return ok && !f.gone[id], nil
}

// fakeStore is the repository in memory: the nodes not deleted, their
// contents, and what the changesets recorded.
type fakeStore struct {
	*recorder
	nodes      map[uuid.UUID]domain.Node
	contents   map[uuid.UUID]app.Content
	changesets []app.Changeset
	// items are the changesets' items, merged by node as the table's
	// upsert merges them.
	items     map[uuid.UUID]app.Item
	revisions []app.Revision
}

func (f *fakeStore) FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error) {
	f.record(ctx, "FindNode")
	n, ok := f.nodes[id]
	if !ok {
		return domain.Node{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error) {
	f.record(ctx, "FindNodeIn")
	n, ok := f.nodes[id]
	if !ok || n.NotebookID != notebookID {
		return domain.Node{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error) {
	f.record(ctx, "ListNodes")
	var out []domain.Node
	for _, n := range f.nodes {
		if n.NotebookID == notebookID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeStore) Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error) {
	f.record(ctx, "Children")
	var out []domain.Node
	for _, n := range f.nodes {
		if n.NotebookID == notebookID && sameParent(n.ParentID, parentID) {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b domain.Node) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), a.ID.Compare(b.ID))
	})
	return out, nil
}

func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (f *fakeStore) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error) {
	f.record(ctx, "Ancestors")
	var out []domain.Ancestor
	for p := f.nodes[id].ParentID; p != nil; p = f.nodes[*p].ParentID {
		out = append(out, domain.Ancestor{ID: *p, Name: f.nodes[*p].Name})
	}
	slices.Reverse(out)
	return out, nil
}

func (f *fakeStore) Subtree(ctx context.Context, notebookID, id uuid.UUID) (domain.Subtree, error) {
	f.record(ctx, "Subtree")
	root, ok := f.nodes[id]
	if !ok || root.NotebookID != notebookID {
		return nil, app.ErrNotFound
	}
	out := domain.Subtree{{Node: root, Level: 1}}
	for i := 0; i < len(out); i++ {
		var children []domain.Node
		for _, n := range f.nodes {
			if n.ParentID != nil && *n.ParentID == out[i].Node.ID {
				children = append(children, n)
			}
		}
		for _, c := range children {
			out = append(out, domain.SubtreeNode{Node: c, Level: out[i].Level + 1})
		}
	}
	slices.SortStableFunc(out, func(a, b domain.SubtreeNode) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.Node.SortOrder, b.Node.SortOrder), a.Node.ID.Compare(b.Node.ID))
	})
	return out, nil
}

func (f *fakeStore) ContentMeta(ctx context.Context, id uuid.UUID) (app.ContentMeta, error) {
	f.record(ctx, "ContentMeta")
	c, ok := f.contents[id]
	if !ok {
		return app.ContentMeta{}, app.ErrNotFound
	}
	return app.ContentMeta{Revision: c.Revision, ByteSize: c.ByteSize, UpdatedBy: c.By, UpdatedAt: c.At}, nil
}

func (f *fakeStore) CreateNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "CreateNode")
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) CreateContent(ctx context.Context, c app.Content) error {
	f.record(ctx, "CreateContent")
	f.contents[c.NodeID] = c
	return nil
}

func (f *fakeStore) RenameNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "RenameNode")
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) MoveNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "MoveNode")
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) DeleteNodes(ctx context.Context, ids []uuid.UUID, _ uuid.UUID, _ time.Time) error {
	f.record(ctx, "DeleteNodes")
	for _, id := range ids {
		delete(f.nodes, id)
		delete(f.contents, id)
	}
	return nil
}

func (f *fakeStore) SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error {
	f.record(ctx, "SetSortOrder")
	n := f.nodes[id]
	n.SortOrder = order
	f.nodes[id] = n
	return nil
}

func (f *fakeStore) CreateChangeset(ctx context.Context, c app.Changeset) error {
	f.record(ctx, "CreateChangeset")
	f.changesets = append(f.changesets, c)
	return nil
}

func (f *fakeStore) RecordItem(ctx context.Context, it app.Item) error {
	f.record(ctx, "RecordItem")
	if old, ok := f.items[it.Change.NodeID]; ok && old.ChangesetID == it.ChangesetID {
		it.Change.Before = old.Change.Before
	}
	f.items[it.Change.NodeID] = it
	return nil
}

func (f *fakeStore) RecordRevision(ctx context.Context, r app.Revision) error {
	f.record(ctx, "RecordRevision")
	f.revisions = append(f.revisions, r)
	return nil
}

// fakeAuthorizer grants the actions of grants, answers forbidden for those
// of forbidden, and the rest not visible.
type fakeAuthorizer struct {
	*recorder
	grants    map[shared.Action]bool
	forbidden map[shared.Action]bool
	targets   []shared.Target
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	f.record(ctx, "Authorize "+string(action))
	f.targets = append(f.targets, t)
	switch {
	case f.grants[action]:
		return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookEditor}, nil
	case f.forbidden[action]:
		return shared.Grant{}, shared.Forbidden()
	}
	return shared.Grant{}, shared.ErrNotVisible
}

// named is a registrant's call as the recorder keeps it: with its name,
// when it has one, so a test of several sees their order.
func named(call, name string) string {
	if name == "" {
		return call
	}
	return call + " " + name
}

// guard records the steps it sees and refuses them with err when set.
type guard struct {
	*recorder
	name  string
	err   error
	steps []app.Step
}

func (g *guard) GuardWrite(ctx context.Context, s app.Step) error {
	g.record(ctx, named("GuardWrite "+string(s.Operation), g.name))
	g.steps = append(g.steps, s)
	return g.err
}

// observer records the events it follows and answers err.
type observer struct {
	*recorder
	name   string
	err    error
	events []app.Event
}

func (o *observer) PagesChanged(ctx context.Context, e app.Event) error {
	o.record(ctx, named("PagesChanged", o.name))
	o.events = append(o.events, e)
	return o.err
}

// participant records the steps it follows. When rename is set, it
// renames that node to name through the unit; when retitle is, it renames
// the node the step changed, adding " (retitled)" to its name.
type participant struct {
	*recorder
	label   string
	steps   []app.Step
	rename  *uuid.UUID
	name    string
	retitle bool
}

func (p *participant) Participate(ctx context.Context, s app.Step, u app.Appender) error {
	p.record(ctx, named("Participate "+string(s.Operation), p.label))
	p.steps = append(p.steps, s)
	switch {
	case p.rename != nil:
		_, err := u.Rename(ctx, *p.rename, p.name)
		return err
	case p.retitle:
		c := s.Changes[0]
		_, err := u.Rename(ctx, c.NodeID, c.After.Name+" (retitled)")
		return err
	}
	return nil
}

// fixture is the fakes over one recorder: alice in the workspace acme with
// its notebook eng.
type fixture struct {
	rec        *recorder
	tx         *fakeTx
	clock      *tickingClock
	workspaces fakeWorkspaces
	notebooks  fakeNotebooks
	store      *fakeStore
	auth       *fakeAuthorizer
	logs       *bytes.Buffer
	guards     []app.WriteGuard
	partakers  []app.Participant
	observers  []app.PageObserver
	alice      uuid.UUID
	acme, eng  uuid.UUID
}

func newFixture() *fixture {
	rec := &recorder{}
	f := &fixture{
		rec: rec, tx: &fakeTx{}, clock: &tickingClock{},
		workspaces: fakeWorkspaces{recorder: rec, gone: map[uuid.UUID]bool{}},
		store: &fakeStore{recorder: rec, nodes: map[uuid.UUID]domain.Node{}, contents: map[uuid.UUID]app.Content{},
			items: map[uuid.UUID]app.Item{}},
		auth: &fakeAuthorizer{recorder: rec, grants: map[shared.Action]bool{}, forbidden: map[shared.Action]bool{}},
		logs: &bytes.Buffer{}, alice: uuid.NewV7(), acme: uuid.NewV7(), eng: uuid.NewV7(),
	}
	f.notebooks = fakeNotebooks{recorder: rec, workspaces: map[uuid.UUID]uuid.UUID{f.eng: f.acme}, gone: map[uuid.UUID]bool{}}
	return f
}

func (f *fixture) logger() *slog.Logger { return slog.New(slog.NewTextHandler(f.logs, nil)) }

func (f *fixture) writer() *app.Writer {
	return app.NewWriter(app.WriterDeps{
		Tx: f.tx, Clock: f.clock, Auth: f.auth, Workspaces: f.workspaces, Notebooks: f.notebooks, Nodes: f.store,
		NodeWriter: f.store, Changesets: f.store, Guards: f.guards, Participants: f.partakers, Observers: f.observers,
	})
}

// grant has the authorizer allow actions.
func (f *fixture) grant(actions ...shared.Action) {
	for _, a := range actions {
		f.auth.grants[a] = true
	}
}

// page adds a page of eng to the store, under parent, without recording a
// call: it was there before the test.
func (f *fixture) page(name string, parent *uuid.UUID, order float64) domain.Node {
	title, err := domain.CheckTitle("title", name)
	if err != nil {
		panic(err)
	}
	at := now().Add(-time.Hour)
	n := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, ParentID: parent, Kind: domain.KindPage, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: at, UpdatedAt: at}
	f.store.nodes[n.ID] = n
	f.store.contents[n.ID] = app.Content{NodeID: n.ID, Revision: 1, By: f.alice, At: at}
	return n
}

// asAlice is a context with alice as the caller by a sign-in session.
func (f *fixture) asAlice() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
}

// called reports whether a call was recorded.
func (f *fixture) called(call string) bool {
	return slices.Contains(f.rec.calls, call)
}
