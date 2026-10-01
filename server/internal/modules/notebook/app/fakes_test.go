package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is the fixed clock's instant.
func now() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }

// tickingClock reads now first, then a microsecond later each time: a use
// case that read it twice for what must be one time would write two.
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
	err := fn(context.WithValue(ctx, txKey{}, true))
	f.rolledBack = err != nil
	return err
}

// recorder is the calls of every fake, in order, each with whether it ran
// in the transaction: the use cases' order of locks, decision and writes.
type recorder struct{ calls []string }

func (r *recorder) record(ctx context.Context, call string) {
	if ctx.Value(txKey{}) != nil {
		call += " in tx"
	}
	r.calls = append(r.calls, call)
}

// fakeWorkspaces answers from ids, by slug; a workspace in gone is deleted
// by the time it is shared.
type fakeWorkspaces struct {
	*recorder
	ids  map[string]uuid.UUID
	gone map[uuid.UUID]bool
}

func (f fakeWorkspaces) FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error) {
	f.record(ctx, "FindBySlug "+slug)
	id, ok := f.ids[slug]
	return id, ok, nil
}

func (f fakeWorkspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "ShareByID")
	return !f.gone[id], nil
}

// fakeStore answers from notebooks, by id; one in deleted is gone by the
// time it is locked. It keeps what was written.
type fakeStore struct {
	*recorder
	notebooks map[uuid.UUID]domain.Notebook
	deleted   map[uuid.UUID]bool
	listed    []app.Listed
	counts    map[uuid.UUID]int
	created   []domain.Notebook
	members   []domain.Member
	updated   []domain.Notebook
	// ofWorkspace are the ids DeleteNotebooksOf deletes.
	ofWorkspace []uuid.UUID
}

func (f *fakeStore) CreateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error {
	f.record(ctx, "CreateNotebook by "+by.String())
	f.created = append(f.created, n)
	return nil
}

func (f *fakeStore) AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error {
	f.record(ctx, "AddMember "+string(m.Role)+" by "+by.String())
	f.members = append(f.members, m)
	return nil
}

func (f *fakeStore) FindNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error) {
	f.record(ctx, "FindNotebook")
	n, ok := f.notebooks[id]
	if !ok {
		return domain.Notebook{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) LockNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error) {
	f.record(ctx, "LockNotebook")
	n, ok := f.notebooks[id]
	if !ok || f.deleted[id] {
		return domain.Notebook{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) CountMembers(ctx context.Context, id uuid.UUID) (int, error) {
	f.record(ctx, "CountMembers")
	return f.counts[id], nil
}

func (f *fakeStore) ListNotebooks(ctx context.Context, workspaceID, userID uuid.UUID, reached bool) ([]app.Listed, error) {
	call := "ListNotebooks"
	if reached {
		call += " reached"
	}
	f.record(ctx, call)
	return f.listed, nil
}

func (f *fakeStore) UpdateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error {
	f.record(ctx, "UpdateNotebook by "+by.String())
	f.updated = append(f.updated, n)
	return nil
}

func (f *fakeStore) DeleteNotebook(ctx context.Context, id, by uuid.UUID, at time.Time) error {
	f.record(ctx, "DeleteNotebook by "+by.String()+" at "+at.Format(time.RFC3339))
	return nil
}

func (f *fakeStore) DeleteNotebooksOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) ([]uuid.UUID, error) {
	f.record(ctx, "DeleteNotebooksOf by "+by.String()+" at "+at.Format(time.RFC3339))
	return f.ofWorkspace, nil
}

func (f *fakeStore) DeleteAuditEventsOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) error {
	f.record(ctx, "DeleteAuditEventsOf by "+by.String()+" at "+at.Format(time.RFC3339))
	return nil
}

// fakeAuthorizer grants the grant of grants for the action asked, and
// answers the rest not visible, or err when set.
type fakeAuthorizer struct {
	*recorder
	grants  map[shared.Action]shared.Grant
	err     error
	targets []shared.Target
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	f.record(ctx, "Authorize "+string(action))
	f.targets = append(f.targets, t)
	if f.err != nil {
		return shared.Grant{}, f.err
	}
	g, ok := f.grants[action]
	if !ok {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return g, nil
}

// fakeSubscriber records the deletions it follows, and answers err.
type fakeSubscriber struct {
	*recorder
	err  error
	got  []app.NotebookDeletion
	name string
}

func (f *fakeSubscriber) NotebookDeleted(ctx context.Context, d app.NotebookDeletion) error {
	f.record(ctx, "NotebookDeleted "+f.name)
	f.got = append(f.got, d)
	return f.err
}

// fixture is the fakes over one recorder, a workspace acme and a notebook
// in it, with its members (fakes_members_test.go).
type fixture struct {
	rec        *recorder
	workspaces fakeWorkspaces
	store      *fakeStore
	auth       *fakeAuthorizer
	tx         *fakeTx
	logs       *bytes.Buffer
	acme       uuid.UUID
	notebook   domain.Notebook
	team
}

func newFixture() fixture {
	rec := &recorder{}
	acme := uuid.NewV7()
	n := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: acme, Name: "Engineering", Access: shared.AccessNone,
		CreatedAt: now().Add(-time.Hour), UpdatedAt: now().Add(-time.Hour)}
	return fixture{
		rec:        rec,
		workspaces: fakeWorkspaces{recorder: rec, ids: map[string]uuid.UUID{"acme": acme}, gone: map[uuid.UUID]bool{}},
		store: &fakeStore{recorder: rec, notebooks: map[uuid.UUID]domain.Notebook{n.ID: n}, deleted: map[uuid.UUID]bool{},
			counts: map[uuid.UUID]int{n.ID: 3}},
		auth: &fakeAuthorizer{recorder: rec, grants: map[shared.Action]shared.Grant{}},
		tx:   &fakeTx{},
		logs: &bytes.Buffer{},
		acme: acme, notebook: n,
		team: newTeam(rec, n),
	}
}

func (f fixture) logger() *slog.Logger { return slog.New(slog.NewTextHandler(f.logs, nil)) }

// grant has the authorizer grant action with roles.
func (f fixture) grant(action shared.Action, ws shared.WorkspaceRole, nb shared.NotebookRole) {
	f.auth.grants[action] = shared.Grant{WorkspaceRole: ws, NotebookRole: nb}
}

// as is ctx with userID as the caller.
func as(userID uuid.UUID) context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: userID, SessionID: uuid.NewV7()})
}
