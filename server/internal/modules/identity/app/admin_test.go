package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// adminStore is the administrator's ports over one account, alice@corp.com,
// recording each step; taken is an address another account has.
type adminStore struct {
	log    *steps
	active bool
	taken  string
}

func (s *adminStore) LockAccountByEmail(ctx context.Context, email string) (app.LockedAccount, error) {
	s.log.add("lock "+email, ctx)
	if email != "alice@corp.com" {
		return app.LockedAccount{}, app.ErrNotFound
	}
	return app.LockedAccount{ID: testUserID(), Email: email, PasswordHash: "hashed:old", Active: s.active}, nil
}

func (s *adminStore) CreateUser(ctx context.Context, u app.NewUser) error {
	s.log.add("create "+u.Email+" as "+u.DisplayName+" with "+u.PasswordHash, ctx)
	if u.Email == s.taken {
		return domain.ErrEmailTaken
	}
	return nil
}

func (s *adminStore) UpdatePasswordHash(ctx context.Context, _ uuid.UUID, hash string, _ time.Time) error {
	s.log.add("password "+hash, ctx)
	return nil
}

func (s *adminStore) RevokeSessions(ctx context.Context, _, keep uuid.UUID, reason domain.RevokeReason, _ time.Time) (int, error) {
	s.log.add("revoke sessions for "+string(reason)+", keeping "+keep.String(), ctx)
	return 2, nil
}

func (s *adminStore) RevokeAllAPITokens(ctx context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	s.log.add("revoke tokens", ctx)
	return 1, nil
}

func (s *adminStore) ChangeEmail(ctx context.Context, _ uuid.UUID, email string, _ time.Time) error {
	s.log.add("email "+email, ctx)
	if email == s.taken {
		return domain.ErrEmailTaken
	}
	return nil
}

func (s *adminStore) ActivateUser(ctx context.Context, _ uuid.UUID, _ time.Time) error {
	s.log.add("activate", ctx)
	return nil
}

func (s *adminStore) CountUsableAPITokens(ctx context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	s.log.add("count tokens", ctx)
	return 3, nil
}

// fieldsOf are the field and code of each problem of err, a 422.
func fieldsOf(err error) []string {
	var se *shared.Error
	if !errors.As(err, &se) {
		return nil
	}
	var out []string
	for _, f := range se.Fields {
		out = append(out, f.Field+" "+f.Code)
	}
	return out
}

// The administrator's create checks and hashes as registration does, then
// inserts the account, named after its address, in one statement.
func TestCreateUser(t *testing.T) {
	log, logs, hasher := &steps{}, &bytes.Buffer{}, &fakeHasher{}
	store := &adminStore{log: log, taken: "bob@corp.com"}
	uc := app.NewCreateUser(app.CreateUserDeps{
		Rules: domain.NewPasswordRules(), Hasher: hasher, Users: store, Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(logs, nil)),
	})

	created, err := uc.Execute(context.Background(), " Carol@Corp.com ", "Tr0ub4dor&3")

	if err != nil || created.Email != "carol@corp.com" || created.ID == uuid.Nil() ||
		!slices.Equal(log.done, []string{"create carol@corp.com as carol with hashed:Tr0ub4dor&3 outside the transaction"}) {
		t.Errorf("Execute() = %+v, %v after %q; want carol created with the hash", created, err, log.done)
	}
	if out := logs.String(); !strings.Contains(out, `"msg":"account created"`) || !strings.Contains(out, `"by":"cli"`) || strings.Contains(out, "carol@") {
		t.Errorf("logs = %s, want the creation by the command line, without the address", out)
	}

	_, invalid := uc.Execute(context.Background(), "not an address", "short")
	_, taken := uc.Execute(context.Background(), "bob@corp.com", "Tr0ub4dor&3")
	if got := fieldsOf(invalid); !slices.Equal(got, []string{"email invalid_format", "password too_short"}) || hasher.calls != 2 {
		t.Errorf("an invalid account = %v (%v) after %d hashes; want both problems before any hash", invalid, got, hasher.calls)
	}
	if !errors.Is(taken, domain.ErrEmailTaken) {
		t.Errorf("an address in use = %v, want identity.email_taken", taken)
	}
}

func newResetPassword(store *adminStore, hasher *fakeHasher, tx *fakeTx) *app.ResetPassword {
	return app.NewResetPassword(app.ResetPasswordDeps{
		Accounts: store, Passwords: store, Sessions: store, APITokens: store, Hasher: hasher, Rules: domain.NewPasswordRules(),
		Tx: tx, Clock: fixedClock(testNow()), Logger: slog.New(slog.DiscardHandler),
	})
}

// A reset writes the hash, then revokes every session and every token, in
// one transaction after the account row lock.
func TestResetPassword(t *testing.T) {
	log, hasher, tx := &steps{}, &fakeHasher{}, &fakeTx{}
	uc := newResetPassword(&adminStore{log: log, active: true}, hasher, tx)

	result, err := uc.Execute(context.Background(), "Alice@Corp.com", "N3w-Passw0rd!")

	want := []string{"lock alice@corp.com", "password hashed:N3w-Passw0rd!", "revoke sessions for password_reset, keeping " + uuid.Nil().String(), "revoke tokens"}
	if err != nil || result != (app.ResetPasswordResult{Email: "alice@corp.com", Sessions: 2, APITokens: 1}) || !slices.Equal(log.done, want) || tx.calls != 1 {
		t.Errorf("Execute() = %+v, %v after %q in %d transactions; want %q in one", result, err, log.done, tx.calls, want)
	}
}

// The rules check the password with the address first (422, no hash, no
// transaction); an address no account has is identity.account_not_found,
// and one that cannot be valid is not looked up.
func TestResetPasswordFails(t *testing.T) {
	log, hasher, tx := &steps{}, &fakeHasher{}, &fakeTx{}
	uc := newResetPassword(&adminStore{log: log}, hasher, tx)

	_, weak := uc.Execute(context.Background(), "alice@corp.com", "alice2026!")
	if got := fieldsOf(weak); !slices.Equal(got, []string{"password common_password"}) || hasher.calls != 0 || tx.calls != 0 {
		t.Errorf("a password from the address = %v (%v) after %d hashes, %d transactions; want refused first", weak, got, hasher.calls, tx.calls)
	}
	_, unknown := uc.Execute(context.Background(), "bob@corp.com", "N3w-Passw0rd!")
	_, invalid := uc.Execute(context.Background(), "bob\x00@corp.com", "N3w-Passw0rd!")
	if !errors.Is(unknown, domain.ErrAccountNotFound) || !errors.Is(invalid, domain.ErrAccountNotFound) ||
		!slices.Equal(log.done, []string{"lock bob@corp.com"}) {
		t.Errorf("unknown = %v, invalid = %v after %q; want identity.account_not_found, the invalid address not looked up", unknown, invalid, log.done)
	}
}

// A new address changes under the lock and ends every session; the tokens
// stay. The address the account has, or one that cannot be valid, is
// refused before any transaction.
func TestSetEmail(t *testing.T) {
	log, tx := &steps{}, &fakeTx{}
	store := &adminStore{log: log, taken: "bob@corp.com"}
	uc := app.NewSetEmail(app.SetEmailDeps{Accounts: store, Emails: store, Sessions: store, Tx: tx, Clock: fixedClock(testNow()), Logger: slog.New(slog.DiscardHandler)})

	result, err := uc.Execute(context.Background(), "alice@corp.com", " Alice@Example.org")

	want := []string{"lock alice@corp.com", "email alice@example.org", "revoke sessions for email_changed, keeping " + uuid.Nil().String()}
	if err != nil || result != (app.SetEmailResult{Email: "alice@example.org", Sessions: 2}) || !slices.Equal(log.done, want) {
		t.Errorf("Execute() = %+v, %v after %q; want %q", result, err, log.done, want)
	}

	log.done, tx.calls = nil, 0
	_, invalid := uc.Execute(context.Background(), "alice@corp.com", "not an address")
	_, same := uc.Execute(context.Background(), "alice@corp.com", "ALICE@corp.com ")
	_, taken := uc.Execute(context.Background(), "alice@corp.com", "bob@corp.com")
	if got := fieldsOf(invalid); !slices.Equal(got, []string{"new_email invalid_format"}) || !errors.Is(same, domain.ErrEmailUnchanged) || tx.calls != 1 {
		t.Errorf("invalid = %v (%v), same = %v after %d transactions; want both refused before any", invalid, got, same, tx.calls)
	}
	if !errors.Is(taken, domain.ErrEmailTaken) {
		t.Errorf("an address in use = %v, want identity.email_taken", taken)
	}
}

func newDeactivateAccount(log *steps, logs *bytes.Buffer, active bool) *app.DeactivateAccount {
	return app.NewDeactivateAccount(app.DeactivateAccountDeps{
		Accounts: &adminStore{log: log, active: active},
		Steps: app.DeactivationSteps{
			Users: stepUsers{log}, Sessions: stepSessions{log},
			Vetoers: []app.DeactivationVetoer{stepVetoer{name: "a", log: log}}, Subscribers: []app.DeactivationSubscriber{stepSubscriber{name: "a", log: log}},
		},
		Tx: &fakeTx{}, Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(logs, nil)),
	})
}

// The administrator's deactivation runs the steps of the caller's own, the
// registrants too, under the lock of the account named by its address. An
// account inactive already is left as it is: the registrants see each
// deactivation once.
func TestDeactivateAccount(t *testing.T) {
	log, logs := &steps{}, &bytes.Buffer{}
	result, err := newDeactivateAccount(log, logs, true).Execute(context.Background(), "alice@corp.com")

	want := []string{"lock alice@corp.com", "veto a", "deactivate", "revoke all for deactivated, keeping " + uuid.Nil().String(), "follow a"}
	if err != nil || result != (app.DeactivateAccountResult{Email: "alice@corp.com", Sessions: 3}) || !slices.Equal(log.done, want) {
		t.Errorf("Execute() = %+v, %v after %q; want %q", result, err, log.done, want)
	}
	if out := logs.String(); !strings.Contains(out, `"msg":"account deactivated"`) || !strings.Contains(out, `"by":"cli"`) {
		t.Errorf("logs = %s, want the deactivation by the command line", out)
	}

	log, logs = &steps{}, &bytes.Buffer{}
	result, err = newDeactivateAccount(log, logs, false).Execute(context.Background(), "alice@corp.com")
	if err != nil || !result.Already || !slices.Equal(log.done, []string{"lock alice@corp.com"}) || logs.Len() != 0 {
		t.Errorf("an inactive account: %+v, %v after %q with logs %q; want already, nothing done or logged", result, err, log.done, logs.String())
	}
	if _, err := newDeactivateAccount(&steps{}, &bytes.Buffer{}, true).Execute(context.Background(), "bob@corp.com"); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("an unknown address = %v, want identity.account_not_found", err)
	}
}

func newActivate(log *steps, active bool) *app.Activate {
	store := &adminStore{log: log, active: active}
	return app.NewActivate(app.ActivateDeps{
		Accounts: store, Users: store, APITokens: store, Tx: &fakeTx{}, Clock: fixedClock(testNow()), Logger: slog.New(slog.DiscardHandler),
	})
}

// Activation makes an inactive account active and counts its tokens usable
// again; an active account is left as it is.
func TestActivate(t *testing.T) {
	log := &steps{}
	result, err := newActivate(log, false).Execute(context.Background(), "alice@corp.com")
	if err != nil || result != (app.ActivateResult{Email: "alice@corp.com", APITokens: 3}) ||
		!slices.Equal(log.done, []string{"lock alice@corp.com", "activate", "count tokens"}) {
		t.Errorf("Execute() = %+v, %v after %q; want activated with 3 tokens", result, err, log.done)
	}

	log = &steps{}
	result, err = newActivate(log, true).Execute(context.Background(), "alice@corp.com")
	if err != nil || !result.Already || !slices.Equal(log.done, []string{"lock alice@corp.com"}) {
		t.Errorf("an active account: %+v, %v after %q; want already, nothing done", result, err, log.done)
	}
}
