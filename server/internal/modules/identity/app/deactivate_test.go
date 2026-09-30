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

// steps records what a deactivation did, in order.
type steps struct{ done []string }

func (s *steps) add(step string, ctx context.Context) {
	if !inTx(ctx) {
		step += " outside the transaction"
	}
	s.done = append(s.done, step)
}

type stepVetoer struct {
	name string
	log  *steps
	err  error
	got  *app.Deactivation
}

func (v stepVetoer) VetoDeactivation(ctx context.Context, d app.Deactivation) error {
	v.log.add("veto "+v.name, ctx)
	if v.got != nil {
		*v.got = d
	}
	return v.err
}

type stepSubscriber struct {
	name string
	log  *steps
	err  error
}

func (s stepSubscriber) AccountDeactivated(ctx context.Context, _ app.Deactivation) error {
	s.log.add("follow "+s.name, ctx)
	return s.err
}

type stepUsers struct{ log *steps }

func (u stepUsers) DeactivateUser(ctx context.Context, _ uuid.UUID, _ time.Time) error {
	u.log.add("deactivate", ctx)
	return nil
}

type stepSessions struct{ log *steps }

func (s stepSessions) RevokeSessions(ctx context.Context, _, keep uuid.UUID, reason domain.RevokeReason, _ time.Time) (int, error) {
	s.log.add("revoke all for "+string(reason)+", keeping "+keep.String(), ctx)
	return 3, nil
}

// stepLocker records the account row lock.
type stepLocker struct {
	log    *steps
	active bool
}

func (l stepLocker) LockForCredentials(ctx context.Context, _ uuid.UUID) (app.LockedAccount, error) {
	l.log.add("lock", ctx)
	return app.LockedAccount{Email: "alice@corp.com", Active: l.active}, nil
}

func newDeactivate(log *steps, logs *bytes.Buffer, vetoers []app.DeactivationVetoer, subscribers []app.DeactivationSubscriber) *app.Deactivate {
	return app.NewDeactivate(app.DeactivateDeps{
		Lock:  app.CredentialLock{Locker: stepLocker{log: log, active: true}, Sessions: &fakeStore{credential: validCredential()}, APITokens: newFakeAPITokens(validToken())},
		Steps: app.DeactivationSteps{Users: stepUsers{log}, Sessions: stepSessions{log}, Vetoers: vetoers, Subscribers: subscribers},
		Tx:    &fakeTx{}, Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(logs, nil)),
	})
}

// In one transaction: the lock, the vetoers in order, the writes, the
// subscribers in order. Every session ends, the caller's own too, whatever
// the credential. The vetoers see the address read under the lock and the
// deactivation's instant.
func TestDeactivateInOrder(t *testing.T) {
	for name, actor := range map[string]shared.Actor{"session": sessionActor(), "token": tokenActor()} {
		t.Run(name, func(t *testing.T) {
			log, logs := &steps{}, &bytes.Buffer{}
			var got app.Deactivation
			uc := newDeactivate(log, logs,
				[]app.DeactivationVetoer{stepVetoer{name: "a", log: log, got: &got}, stepVetoer{name: "b", log: log}},
				[]app.DeactivationSubscriber{stepSubscriber{name: "a", log: log}, stepSubscriber{name: "b", log: log}})

			err := uc.Execute(asCaller(actor))

			want := []string{"lock", "veto a", "veto b", "deactivate", "revoke all for deactivated, keeping " + uuid.Nil().String(), "follow a", "follow b"}
			if err != nil || !slices.Equal(log.done, want) {
				t.Errorf("Execute() = %v, did %q; want %q", err, log.done, want)
			}
			if got != (app.Deactivation{UserID: testUserID(), Email: "alice@corp.com", At: testNow()}) {
				t.Errorf("the vetoer got %+v, want the account's id and address at the clock's now", got)
			}
			if out := logs.String(); !strings.Contains(out, `"msg":"account deactivated"`) || !strings.Contains(out, `"revoked_sessions":3`) ||
				!strings.Contains(out, `"by":"self"`) {
				t.Errorf("logs = %s, want the deactivation with the count, by the account itself", out)
			}
		})
	}
}

// The first refusal ends the deactivation before any write and is the
// answer; a subscriber's error is the answer too, after the writes (the
// transaction rolls them back).
func TestDeactivateStops(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "test.sole_admin", "The account is the sole administrator.")
	log, logs := &steps{}, &bytes.Buffer{}
	uc := newDeactivate(log, logs,
		[]app.DeactivationVetoer{stepVetoer{name: "a", log: log, err: refusal}, stepVetoer{name: "b", log: log}},
		[]app.DeactivationSubscriber{stepSubscriber{name: "a", log: log}})

	err := uc.Execute(asCaller(sessionActor()))

	if !errors.Is(err, refusal) || !slices.Equal(log.done, []string{"lock", "veto a"}) {
		t.Errorf("Execute() = %v, did %q; want the refusal after the first vetoer", err, log.done)
	}
	if out := logs.String(); !strings.Contains(out, `"code":"test.sole_admin"`) || !strings.Contains(out, `"by":"self"`) {
		t.Errorf("logs = %s, want the refusal's code, by the account itself", out)
	}

	boom := errors.New("subscriber failed")
	log = &steps{}
	uc = newDeactivate(log, &bytes.Buffer{}, nil, []app.DeactivationSubscriber{stepSubscriber{name: "a", log: log, err: boom}, stepSubscriber{name: "b", log: log}})
	if err := uc.Execute(asCaller(sessionActor())); !errors.Is(err, boom) || log.done[len(log.done)-1] != "follow a" {
		t.Errorf("Execute() = %v, did %q; want the subscriber's error, the next one not called", err, log.done)
	}
}
