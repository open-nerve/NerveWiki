package app_test

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"slices"
	"strings"
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
// the time it is locked. waited, when set, runs while a write waits for the
// notebook's lock: what another write commits meanwhile.
type fakeNotebooks struct {
	*recorder
	workspaces map[uuid.UUID]uuid.UUID
	gone       map[uuid.UUID]bool
	waited     func()
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
	if f.waited != nil {
		f.waited()
	}
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
	// touched are the changesets written again, with when.
	touched map[uuid.UUID]time.Time
	// sessions are the edit sessions; held are those another transaction
	// holds.
	sessions map[uuid.UUID]app.EditSession
	held     map[uuid.UUID]bool
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

func (f *fakeStore) PageContent(ctx context.Context, id uuid.UUID) (app.PageContent, error) {
	f.record(ctx, "PageContent")
	c, ok := f.contents[id]
	if !ok {
		return app.PageContent{}, app.ErrNotFound
	}
	return app.PageContent{Content: c.Content, Revision: c.Revision, Hash: c.Hash}, nil
}

func (f *fakeStore) LockContent(ctx context.Context, notebookID, id uuid.UUID) (app.ContentLock, error) {
	f.record(ctx, "LockContent")
	n, ok := f.nodes[id]
	c, has := f.contents[id]
	if !ok || !has || n.NotebookID != notebookID || n.Kind != domain.KindPage {
		return app.ContentLock{}, app.ErrNotFound
	}
	return app.ContentLock{Revision: c.Revision, Hash: c.Hash, ByteSize: c.ByteSize}, nil
}

// fakeMarkdown takes a content's facts as the content itself and renders
// it in a <p>, with the page it rendered for; err, when set, is Render's.
type fakeMarkdown struct {
	*recorder
	pages []app.PageRef
	err   error
	// panics has Facts panic.
	panics bool
	// tasksRead, when set, runs once after Tasks: someone's write while
	// the tasks are read.
	tasksRead func()
}

func (f *fakeMarkdown) Facts(content string) app.Facts {
	f.record(context.Background(), "Facts")
	if f.panics {
		panic("the parse failed")
	}
	return content
}

// Tasks finds a task item at the start of a line, "- [ ]", "- [\t]",
// "- [x]" or "- [X]", but for a "- [x]: " line, which a link reference definition
// is: ticking "- [ ]: /u" makes one, as in the tasks extension.
func (f *fakeMarkdown) Tasks(facts app.Facts) []app.Task {
	f.record(context.Background(), "Tasks")
	var out []app.Task
	at := 0
	for line := range strings.SplitAfterSeq(facts.(string), "\n") {
		if strings.HasPrefix(line, "- [") && len(line) > 4 && strings.ContainsRune(" \txX", rune(line[3])) && line[4] == ']' &&
			!strings.HasPrefix(line, "- [x]: ") {
			out = append(out, app.Task{Offset: at + 3, Checked: line[3] == 'x' || line[3] == 'X'})
		}
		at += len(line)
	}
	if read := f.tasksRead; read != nil {
		f.tasksRead = nil
		read()
	}
	return out
}

// fakeBudget records each take of the parse budget with its bytes, each
// keeping of the facts' share and each release; err, when set, is Take's.
type fakeBudget struct {
	*recorder
	err      error
	held     int
	released int
}

func (b *fakeBudget) Take(ctx context.Context, n int) (app.BudgetHold, error) {
	b.record(ctx, fmt.Sprintf("Take %d", n))
	if b.err != nil {
		return nil, b.err
	}
	b.held += n
	return &fakeHold{budget: b, n: n}, nil
}

// fakeHold is a take of fakeBudget: it records the facts it keeps by their
// size, fakeMarkdown's facts being the content, and keeps nothing back, as
// the bytes they keep do not matter here; it gives all back once.
type fakeHold struct {
	budget *fakeBudget
	n      int
	done   bool
}

func (h *fakeHold) KeepFacts(f app.Facts) {
	content, _ := f.(string)
	h.budget.record(context.Background(), fmt.Sprintf("KeepFacts %d", len(content)))
}

func (h *fakeHold) Release() {
	if h.done {
		return
	}
	h.done = true
	h.budget.record(context.Background(), fmt.Sprintf("Release %d", h.n))
	h.budget.held -= h.n
	h.budget.released++
}

func (f *fakeMarkdown) Render(ctx context.Context, content string, page app.PageRef) (string, error) {
	f.record(ctx, "Render")
	f.pages = append(f.pages, page)
	return "<p>" + content + "</p>", f.err
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

func (f *fakeStore) WriteContent(ctx context.Context, c app.Content) error {
	f.record(ctx, "WriteContent")
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

func (f *fakeStore) TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error {
	f.record(ctx, "TouchChangeset")
	f.touched[id] = at
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

// RecordRevision inserts the version, or updates the page's in the
// changeset, keeping its base and id, as the table's upsert does.
func (f *fakeStore) RecordRevision(ctx context.Context, r app.Revision) error {
	f.record(ctx, "RecordRevision")
	for i, old := range f.revisions {
		if old.ChangesetID == r.ChangesetID && old.NodeID == r.NodeID {
			r.ID, r.Base = old.ID, old.Base
			f.revisions[i] = r
			return nil
		}
	}
	f.revisions = append(f.revisions, r)
	return nil
}

func (f *fakeStore) CreateSession(ctx context.Context, s app.EditSession) error {
	f.record(ctx, "CreateSession")
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeStore) LockSession(ctx context.Context, id uuid.UUID) (app.EditSession, error) {
	f.record(ctx, "LockSession")
	s, ok := f.sessions[id]
	if !ok {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) SetSessionWrite(ctx context.Context, id, changesetID uuid.UUID, revision int) error {
	f.record(ctx, "SetSessionWrite")
	s := f.sessions[id]
	s.ChangesetID, s.Revision = changesetID, revision
	f.sessions[id] = s
	return nil
}

func (f *fakeStore) FindLiveSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	f.record(ctx, "FindLiveSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) HeartbeatSession(ctx context.Context, id, userID uuid.UUID, now, until time.Time) (app.EditSession, error) {
	f.record(ctx, "HeartbeatSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	s.ExpiresAt = until
	f.sessions[id] = s
	return s, nil
}

// EndSession deletes the caller's session alive at now, or their
// tombstone.
func (f *fakeStore) EndSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	f.record(ctx, "EndSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || s.EndedReason == "" && !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	delete(f.sessions, id)
	return s, nil
}

func (f *fakeStore) FindEndedSession(ctx context.Context, id, userID uuid.UUID) (app.EditSession, error) {
	f.record(ctx, "FindEndedSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || s.EndedReason == "" {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

// AliveSessionsOf are the pages' sessions alive at now, by when they
// opened.
func (f *fakeStore) AliveSessionsOf(ctx context.Context, ids []uuid.UUID, now time.Time) ([]app.EditSession, error) {
	f.record(ctx, "AliveSessionsOf")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NodeID) && s.Alive(now) {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b app.EditSession) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	return out, nil
}

// DeleteExpiredSessionsOf deletes the page's sessions expired at now,
// tombstones among them.
func (f *fakeStore) DeleteExpiredSessionsOf(ctx context.Context, id uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteExpiredSessionsOf")
	for sid, s := range f.sessions {
		if s.NodeID == id && !s.ExpiresAt.After(now) {
			delete(f.sessions, sid)
		}
	}
	return nil
}

// EndAliveSessions makes tombstones of the sessions e ends, at e.At or
// their opening, the later; it sorts them by when they opened, for the
// tests' sake, as the SQL does not.
func (f *fakeStore) EndAliveSessions(ctx context.Context, e app.SessionsEnd) ([]app.EditSession, error) {
	f.record(ctx, "EndAliveSessions")
	var out []app.EditSession
	for id, s := range f.sessions {
		if s.NodeID != e.NodeID || !s.Alive(e.At) || e.UserID != nil && s.UserID != *e.UserID {
			continue
		}
		s.EndedReason, s.EndedByID, s.EndedAt = e.Reason, e.By, e.At
		if s.CreatedAt.After(e.At) {
			s.EndedAt = s.CreatedAt
		}
		if e.Until.After(s.ExpiresAt) {
			s.ExpiresAt = e.Until
		}
		f.sessions[id] = s
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b app.EditSession) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

// DeleteExpiredSessions deletes at most batch of the sessions expired at
// now, tombstones among them; held are skipped, as rows another
// transaction holds.
func (f *fakeStore) DeleteExpiredSessions(ctx context.Context, now time.Time, batch int) (int, error) {
	f.record(ctx, "DeleteExpiredSessions")
	n := 0
	for id, s := range f.sessions {
		if n < batch && !s.ExpiresAt.After(now) && !f.held[id] {
			delete(f.sessions, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, _ uuid.UUID, _ time.Time) error {
	f.record(ctx, "DeleteNotebooksPages")
	for id, n := range f.nodes {
		if slices.Contains(ids, n.NotebookID) {
			delete(f.nodes, id)
			delete(f.contents, id)
		}
	}
	return nil
}

func (f *fakeStore) DeleteNotebookSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	f.record(ctx, "DeleteNotebookSessions")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NotebookID) {
			out = append(out, s)
			delete(f.sessions, s.ID)
		}
	}
	slices.SortFunc(out, func(a, b app.EditSession) int { return a.ID.Compare(b.ID) })
	return out, nil
}

func (f *fakeStore) DeleteNodeSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	f.record(ctx, "DeleteNodeSessions")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NodeID) {
			out = append(out, s)
			delete(f.sessions, s.ID)
		}
	}
	return out, nil
}

// fakeNames gives each account's display name.
type fakeNames map[uuid.UUID]string

func (n fakeNames) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string)
	for _, id := range ids {
		if name, ok := n[id]; ok {
			out[id] = name
		}
	}
	return out, nil
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
// the node the step changed, adding " (retitled)" to its name; when write
// is, it writes that content through the unit, and keeps the revision.
// It defers deferred, when set, first.
type participant struct {
	*recorder
	label    string
	steps    []app.Step
	rename   *uuid.UUID
	name     string
	retitle  bool
	write    *app.ContentWrite
	revision int
	deferred func()
}

func (p *participant) Participate(ctx context.Context, s app.Step, u app.Appender) error {
	p.record(ctx, named("Participate "+string(s.Operation), p.label))
	p.steps = append(p.steps, s)
	if p.deferred != nil {
		u.Defer(p.deferred)
	}
	switch {
	case p.rename != nil:
		_, err := u.Rename(ctx, *p.rename, p.name)
		return err
	case p.retitle:
		c := s.Changes[0]
		_, err := u.Rename(ctx, c.NodeID, c.After.Name+" (retitled)")
		return err
	case p.write != nil:
		var err error
		p.revision, err = u.WriteContent(ctx, *p.write)
		return err
	}
	return nil
}

// vetoer records the openings it sees and refuses them with err when set.
type vetoer struct {
	*recorder
	name     string
	err      error
	openings []app.SessionOpening
}

func (v *vetoer) VetoEditSession(ctx context.Context, o app.SessionOpening) error {
	v.record(ctx, named("VetoEditSession", v.name))
	v.openings = append(v.openings, o)
	return v.err
}

// subscriber records the openings and the ends it follows and answers
// err.
type subscriber struct {
	*recorder
	name   string
	err    error
	opened []app.SessionOpened
	ended  []app.SessionEnded
}

func (s *subscriber) EditSessionOpened(ctx context.Context, o app.SessionOpened) error {
	s.record(ctx, named("EditSessionOpened", s.name))
	s.opened = append(s.opened, o)
	return s.err
}

func (s *subscriber) EditSessionEnded(ctx context.Context, e app.SessionEnded) error {
	s.record(ctx, named("EditSessionEnded", s.name))
	s.ended = append(s.ended, e)
	return s.err
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
	md         *fakeMarkdown
	budget     *fakeBudget
	auth       *fakeAuthorizer
	logs       *bytes.Buffer
	guards     []app.WriteGuard
	partakers  []app.Participant
	observers  []app.PageObserver
	vetoers    []app.EditSessionVetoer
	enders     []app.EditSessionSubscriber
	names      fakeNames
	alice      uuid.UUID
	acme, eng  uuid.UUID
}

func newFixture() *fixture {
	rec := &recorder{}
	f := &fixture{
		rec: rec, tx: &fakeTx{}, clock: &tickingClock{},
		workspaces: fakeWorkspaces{recorder: rec, gone: map[uuid.UUID]bool{}},
		store: &fakeStore{recorder: rec, nodes: map[uuid.UUID]domain.Node{}, contents: map[uuid.UUID]app.Content{},
			items: map[uuid.UUID]app.Item{}, touched: map[uuid.UUID]time.Time{}, sessions: map[uuid.UUID]app.EditSession{},
			held: map[uuid.UUID]bool{}},
		auth: &fakeAuthorizer{recorder: rec, grants: map[shared.Action]bool{}, forbidden: map[shared.Action]bool{}},
		logs: &bytes.Buffer{}, names: fakeNames{}, alice: uuid.NewV7(), acme: uuid.NewV7(), eng: uuid.NewV7(),
	}
	f.notebooks = fakeNotebooks{recorder: rec, workspaces: map[uuid.UUID]uuid.UUID{f.eng: f.acme}, gone: map[uuid.UUID]bool{}}
	f.md = &fakeMarkdown{recorder: rec}
	f.budget = &fakeBudget{recorder: rec}
	return f
}

func (f *fixture) logger() *slog.Logger { return slog.New(slog.NewTextHandler(f.logs, nil)) }

func (f *fixture) writer() *app.Writer {
	return app.NewWriter(app.WriterDeps{
		Tx: f.tx, Clock: f.clock, Auth: f.auth, Workspaces: f.workspaces, Notebooks: f.notebooks, Nodes: f.store,
		NodeWriter: f.store, Changesets: f.store, SessionWriter: f.store, Names: f.names, Guards: f.guards, Participants: f.partakers,
		Observers: f.observers, SessionVetoers: f.vetoers, SessionSubscribers: f.enders,
	})
}

func (f *fixture) parser() *app.ContentParser {
	return app.NewContentParser(f.writer(), f.md, f.budget)
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
	sum := sha256.Sum256(nil)
	f.store.contents[n.ID] = app.Content{NodeID: n.ID, Revision: 1, Hash: sum[:], By: f.alice, At: at}
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
