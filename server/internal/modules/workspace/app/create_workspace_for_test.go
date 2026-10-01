package app_test

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The administrator's creation (M2/P4 design 3.3): the account named by its
// address, shared first in the transaction, becomes the admin.

type creationFor struct {
	uc       *app.CreateWorkspaceFor
	store    *fakeStore
	accounts *fakeAccounts
	logs     *bytes.Buffer
	alice    uuid.UUID
}

func newCreationFor() creationFor {
	c := creationFor{store: &fakeStore{}, logs: &bytes.Buffer{}, alice: uuid.NewV7()}
	c.accounts = &fakeAccounts{emails: map[uuid.UUID]string{c.alice: "alice@corp.com"}, store: c.store}
	c.uc = app.NewCreateWorkspaceFor(app.CreateWorkspaceForDeps{Workspaces: c.store, Accounts: c.accounts, Tx: &fakeTx{},
		Clock: fixedClock{}, Logger: slog.New(slog.NewTextHandler(c.logs, nil))})
	return c
}

func TestCreateWorkspaceForMakesTheAccountItsAdmin(t *testing.T) {
	c := newCreationFor()

	got, err := c.uc.Execute(as(uuid.NewV7()), " Acme ", "acme", "alice@corp.com")

	if err != nil || len(c.store.created) != 1 || len(c.store.members) != 1 || got != c.store.created[0] {
		t.Fatalf("Execute() = %+v, %v; created %+v, members %+v", got, err, c.store.created, c.store.members)
	}
	if m := c.store.members[0]; got.Name != "Acme" || m.WorkspaceID != got.ID || m.UserID != c.alice || m.Role != shared.WorkspaceAdmin ||
		m.ID == uuid.Nil() || !m.CreatedAt.Equal(now()) {
		t.Errorf("created %+v with %+v; want Acme, alice its admin", got, m)
	}
	by := " by " + c.alice.String()
	wantCalls := inTxCalls("ShareActiveAccountByEmail alice@corp.com", "CreateWorkspace"+by, "AddMember admin"+by+" at 2026-10-01T10:00:00Z")
	if !slices.Equal(c.store.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", c.store.calls, wantCalls)
	}
	logs := c.logs.String()
	if !strings.Contains(logs, "by=cli") || !strings.Contains(logs, "user_id="+c.alice.String()) || strings.Contains(logs, "alice@") {
		t.Errorf("logs = %s, want by=cli and alice's id, not her address", logs)
	}
}

// The values are checked before anything; the account, first in the
// transaction: refused, nothing is written.
func TestCreateWorkspaceForRefusals(t *testing.T) {
	deactivated := shared.NewError(shared.KindForbidden, "identity.account_deactivated", "This account is deactivated.")
	for _, tt := range []struct {
		name, workspace, slug, email string
		accountErr                   error
		want                         error
		calls                        []string
	}{
		{"invalid values", "", "Not A Slug", "alice@corp.com", nil, nil, nil},
		{"no such account", "Acme", "acme", "erin@corp.com", nil, errNoAccount(), inTxCalls("ShareActiveAccountByEmail erin@corp.com")},
		{"a deactivated account", "Acme", "acme", "alice@corp.com", deactivated, deactivated, inTxCalls("ShareActiveAccountByEmail alice@corp.com")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newCreationFor()
			c.accounts.err = tt.accountErr

			_, err := c.uc.Execute(as(uuid.NewV7()), tt.workspace, tt.slug, tt.email)

			var se *shared.Error
			wrong := !errors.Is(err, tt.want)
			if tt.want == nil {
				wrong = !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 2
			}
			if wrong || !slices.Equal(c.store.calls, tt.calls) || c.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v after %q, nothing written or logged", err, c.store.calls, tt.want, tt.calls)
			}
		})
	}
}
