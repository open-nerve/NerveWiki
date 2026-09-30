package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// confirmRun is what one Confirm did.
type confirmRun struct {
	err      error
	prepared int
	written  int
}

// confirm runs Confirm for the test account with password, counting the
// prepare and write calls; write runs inside the transaction or fails the
// run.
func confirm(account *fakeAccount, hasher *fakeHasher, tx *fakeTx, password string, prepareErr error) confirmRun {
	var run confirmRun
	ctx := shared.WithActor(context.Background(), sessionActor())
	c := currentPassword(account, hasher, tx)
	snapshot, err := c.Account(ctx, sessionActor())
	if err != nil {
		return confirmRun{err: err}
	}
	run.err = c.Confirm(ctx, sessionActor(), password, snapshot.PasswordHash, testNow(),
		func() error { run.prepared++; return prepareErr },
		func(ctx context.Context) error {
			if !inTx(ctx) {
				return errors.New("written outside the transaction")
			}
			run.written++
			return nil
		})
	return run
}

// The right password: the preparation once, then the write under the lock,
// in one transaction.
func TestConfirmWritesUnderTheLock(t *testing.T) {
	account, hasher, tx := newFakeAccount("hashed:Tr0ub4dor&3"), &fakeHasher{}, &fakeTx{}

	run := confirm(account, hasher, tx, "Tr0ub4dor&3", nil)

	if run.err != nil || run.prepared != 1 || run.written != 1 || tx.calls != 1 || account.locks != 1 || len(account.outsideTx) != 0 {
		t.Errorf("Confirm() = %+v, %d transactions, %d locks (outside a transaction: %v); want one of each", run, tx.calls, account.locks, account.outsideTx)
	}
}

// A wrong password is 422 identity.current_password_incorrect: nothing is
// prepared, no transaction begins.
func TestConfirmRefusesAWrongPassword(t *testing.T) {
	account, hasher, tx := newFakeAccount("hashed:Tr0ub4dor&3"), &fakeHasher{}, &fakeTx{}

	run := confirm(account, hasher, tx, "Wr0ng-password", nil)

	if !errors.Is(run.err, domain.ErrCurrentPasswordIncorrect) || run.prepared != 0 || tx.calls != 0 {
		t.Errorf("Confirm() = %+v, %d transactions; want current_password_incorrect before anything else", run, tx.calls)
	}
}

// The hash changes between the check and the lock (M1/P3 design 3.4): the
// password is verified again against the hash found under the lock, and
// the transaction runs once more; the preparation is not repeated. A second
// change, like a password that no longer matches, is refused.
func TestConfirmWhenTheHashChangesMeanwhile(t *testing.T) {
	tests := []struct {
		name     string
		changes  []string // the row's hash after each Verify
		err      error
		verified []string
		written  int
	}{
		{"rehashed by a login", []string{"hashed:Tr0ub4dor&3"}, nil, []string{"old:Tr0ub4dor&3", "hashed:Tr0ub4dor&3"}, 1},
		{"changed to another password", []string{"hashed:N3w-Passw0rd!"}, domain.ErrCurrentPasswordIncorrect,
			[]string{"old:Tr0ub4dor&3", "hashed:N3w-Passw0rd!"}, 0},
		{"changed twice", []string{"hashed:Tr0ub4dor&3", "old2:Tr0ub4dor&3"}, domain.ErrCurrentPasswordIncorrect,
			[]string{"old:Tr0ub4dor&3", "hashed:Tr0ub4dor&3"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, tx := newFakeAccount("old:Tr0ub4dor&3"), &fakeTx{}
			hasher := &fakeHasher{}
			changes := slices.Clone(tt.changes)
			hasher.onVerify = func() {
				if len(changes) > 0 {
					account.hash, changes = changes[0], changes[1:]
				}
			}

			run := confirm(account, hasher, tx, "Tr0ub4dor&3", nil)

			if !errors.Is(run.err, tt.err) || (tt.err == nil) != (run.err == nil) || run.written != tt.written ||
				run.prepared != 1 || !slices.Equal(hasher.verified, tt.verified) {
				t.Errorf("Confirm() = %+v, verified %q; want error %v, %d writes, one preparation, verified %q",
					run, hasher.verified, tt.err, tt.written, tt.verified)
			}
		})
	}
}

// The preparation's error stops Confirm before any transaction; the lock's
// 401 stops it before the write; an account gone since authentication is
// 401.
func TestConfirmStops(t *testing.T) {
	boom := errors.New("hasher failed")
	account, tx := newFakeAccount("hashed:Tr0ub4dor&3"), &fakeTx{}
	if run := confirm(account, &fakeHasher{}, tx, "Tr0ub4dor&3", boom); !errors.Is(run.err, boom) || tx.calls != 0 {
		t.Errorf("with a failing preparation: %+v, %d transactions; want its error and none", run, tx.calls)
	}

	deactivated := newFakeAccount("hashed:Tr0ub4dor&3")
	deactivated.active = false
	run := confirm(deactivated, &fakeHasher{}, &fakeTx{}, "Tr0ub4dor&3", nil)
	var se *shared.Error
	if !errors.As(run.err, &se) || se.ProblemStatus() != 401 || run.written != 0 {
		t.Errorf("with the account deactivated under the lock: %+v, want 401 and no write", run)
	}

	gone := newFakeAccount("hashed:Tr0ub4dor&3")
	gone.readErr = app.ErrNotFound
	if run := confirm(gone, &fakeHasher{}, &fakeTx{}, "Tr0ub4dor&3", nil); !errors.As(run.err, &se) || se.ProblemStatus() != 401 {
		t.Errorf("with the account gone: %+v, want 401", run)
	}
}
