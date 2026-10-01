package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type creation struct {
	uc       *app.CreateWorkspace
	store    *fakeStore
	accounts *fakeAccounts
	tx       *fakeTx
	logs     *bytes.Buffer
}

func newCreation(enabled bool) creation {
	c := creation{store: &fakeStore{}, accounts: &fakeAccounts{}, tx: &fakeTx{}, logs: &bytes.Buffer{}}
	c.uc = app.NewCreateWorkspace(app.CreateWorkspaceDeps{
		Workspaces: c.store, Accounts: c.accounts, Tx: c.tx, Clock: fixedClock{},
		Logger: slog.New(slog.NewTextHandler(c.logs, nil)), CreationEnabled: enabled,
	})
	return c
}

func TestCreateWorkspaceMakesTheCallerItsAdmin(t *testing.T) {
	c := newCreation(true)
	alice := uuid.NewV7()

	got, err := c.uc.Execute(as(alice), "  Acme 研发 ", "acme")

	if err != nil || len(c.store.created) != 1 || len(c.store.members) != 1 {
		t.Fatalf("Execute() = %+v, %v; created %+v, members %+v", got, err, c.store.created, c.store.members)
	}
	w, m := c.store.created[0], c.store.members[0]
	if w.Slug != "acme" || w.Name != "Acme 研发" || !w.CreatedAt.Equal(now()) || !w.UpdatedAt.Equal(now()) || w.ID == uuid.Nil() {
		t.Errorf("created %+v, want acme named Acme 研发 at %v", w, now())
	}
	if m.WorkspaceID != w.ID || m.UserID != alice || m.Role != shared.WorkspaceAdmin || m.ID == uuid.Nil() || !m.CreatedAt.Equal(now()) {
		t.Errorf("member %+v, want alice as the admin of %s", m, w.ID)
	}
	if got != (app.Membership{Workspace: w, Role: shared.WorkspaceAdmin}) {
		t.Errorf("Execute() = %+v, want the workspace with the admin's role", got)
	}
	// The account's share and the writes, each in the transaction. That the
	// share comes first, "the account is deactivated" shows: refused, it
	// leaves the store uncalled.
	wantCalls := []string{"ShareActiveAccount " + alice.String() + " in tx"}
	if !slices.Equal(c.accounts.calls, wantCalls) {
		t.Errorf("accounts calls = %q, want %q", c.accounts.calls, wantCalls)
	}
	wantWrites := []string{"CreateWorkspace by " + alice.String() + " in tx", "AddMember by " + alice.String() + " at 2026-10-01T10:00:00Z in tx"}
	if !slices.Equal(c.store.calls, wantWrites) {
		t.Errorf("store calls = %q, want %q", c.store.calls, wantWrites)
	}
	logs := c.logs.String()
	if !strings.Contains(logs, "workspace_id="+w.ID.String()) || !strings.Contains(logs, "user_id="+alice.String()) ||
		strings.Contains(logs, "acme") || strings.Contains(logs, "Acme") {
		t.Errorf("logs = %s, want the ids and neither the slug nor the name", logs)
	}
}

func TestCreateWorkspaceRefusedWhileCreationIsOff(t *testing.T) {
	c := newCreation(false)

	// Even values that break the rules: the switch answers first.
	_, err := c.uc.Execute(as(uuid.NewV7()), "", "Not A Slug")

	if !errors.Is(err, domain.ErrCreationDisabled) || len(c.accounts.calls) != 0 || len(c.store.calls) != 0 {
		t.Errorf("Execute() = %v, accounts %q, store %q; want creation_disabled, nothing asked", err, c.accounts.calls, c.store.calls)
	}
}

func TestCreateWorkspaceChecksTheValuesBeforeTheTransaction(t *testing.T) {
	c := newCreation(true)

	_, err := c.uc.Execute(as(uuid.NewV7()), "", "api")

	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 2 {
		t.Errorf("Execute() = %v, want validation_failed on the name and the slug", err)
	}
	if len(c.accounts.calls) != 0 || len(c.store.calls) != 0 {
		t.Errorf("accounts %q, store %q; want nothing asked", c.accounts.calls, c.store.calls)
	}
}

func TestCreateWorkspaceFailsWithTheTransaction(t *testing.T) {
	deactivated := shared.NewError(shared.KindForbidden, "identity.account_deactivated", "This account is deactivated.")
	t.Run("the account is deactivated", func(t *testing.T) {
		c := newCreation(true)
		c.accounts.err = deactivated

		_, err := c.uc.Execute(as(uuid.NewV7()), "Acme", "acme")

		if !errors.Is(err, deactivated) || len(c.store.calls) != 0 || !c.tx.rolledBack {
			t.Errorf("Execute() = %v, store %q; want the account's refusal before any write", err, c.store.calls)
		}
	})
	t.Run("the slug is taken", func(t *testing.T) {
		c := newCreation(true)
		c.store.createErr = domain.ErrSlugTaken

		_, err := c.uc.Execute(as(uuid.NewV7()), "Acme", "acme")

		if !errors.Is(err, domain.ErrSlugTaken) || len(c.store.members) != 0 || !c.tx.rolledBack || c.logs.Len() != 0 {
			t.Errorf("Execute() = %v, members %+v; want slug_taken, no member, nothing logged", err, c.store.members)
		}
	})
}

func TestCreateWorkspaceNeedsACaller(t *testing.T) {
	c := newCreation(true)
	_, err := c.uc.Execute(context.Background(), "Acme", "acme")
	if !errors.Is(err, shared.Unauthenticated()) {
		t.Errorf("Execute() without a caller = %v, want unauthorized", err)
	}
}
