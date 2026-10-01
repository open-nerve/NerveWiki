package app_test

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is the fixed clock's instant.
func now() time.Time { return time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC) }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now() }

type txKey struct{}

// inTx reports whether ctx came through fakeTx.
func inTx(ctx context.Context) bool { return ctx.Value(txKey{}) != nil }

// fakeTx runs fn with a context that says it is in a transaction, and
// records whether fn failed (a rollback).
type fakeTx struct{ rolledBack bool }

func (f *fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	err := fn(context.WithValue(ctx, txKey{}, true))
	f.rolledBack = err != nil
	return err
}

// fakeStore records the writes, each with whether it ran in the
// transaction, and answers the reads from workspaces and memberships.
type fakeStore struct {
	workspaces  map[string]domain.Workspace
	memberships []app.Membership
	createErr   error
	calls       []string
	created     []domain.Workspace
	members     []domain.Member
}

func (f *fakeStore) record(ctx context.Context, call string) {
	if inTx(ctx) {
		call += " in tx"
	}
	f.calls = append(f.calls, call)
}

func (f *fakeStore) CreateWorkspace(ctx context.Context, w domain.Workspace, by uuid.UUID) error {
	f.record(ctx, "CreateWorkspace by "+by.String())
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, w)
	return nil
}

func (f *fakeStore) AddMember(ctx context.Context, m domain.Member, by uuid.UUID, at time.Time) error {
	f.record(ctx, "AddMember by "+by.String()+" at "+at.Format(time.RFC3339))
	f.members = append(f.members, m)
	return nil
}

func (f *fakeStore) FindWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	f.record(ctx, "FindWorkspaceBySlug "+slug)
	w, ok := f.workspaces[slug]
	if !ok {
		return domain.Workspace{}, app.ErrNotFound
	}
	return w, nil
}

func (f *fakeStore) ListWorkspacesOf(ctx context.Context, userID uuid.UUID) ([]app.Membership, error) {
	f.record(ctx, "ListWorkspacesOf "+userID.String())
	return f.memberships, nil
}

func (f *fakeStore) SlugTaken(ctx context.Context, slug string) (bool, error) {
	f.record(ctx, "SlugTaken "+slug)
	_, ok := f.workspaces[slug]
	return ok, nil
}

// fakeAccounts answers ShareActiveAccount with err, and records the calls.
type fakeAccounts struct {
	err   error
	calls []string
}

func (f *fakeAccounts) ShareActiveAccount(ctx context.Context, id uuid.UUID) error {
	call := "ShareActiveAccount " + id.String()
	if inTx(ctx) {
		call += " in tx"
	}
	f.calls = append(f.calls, call)
	return f.err
}

// fakeAuthorizer grants the role roles hold for the target's workspace,
// and records the decisions asked for.
type fakeAuthorizer struct {
	roles map[uuid.UUID]shared.WorkspaceRole
	err   error
	calls []shared.Action
}

func (f *fakeAuthorizer) Authorize(_ context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	f.calls = append(f.calls, action)
	if f.err != nil {
		return shared.Grant{}, f.err
	}
	role, ok := f.roles[t.WorkspaceID]
	if !ok {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: role}, nil
}

// as is ctx with userID as the caller.
func as(userID uuid.UUID) context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: userID, SessionID: uuid.NewV7()})
}
