package bootstrap

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
)

// fakeInvitations admits with admit or fails with err, and records what it
// was asked.
type fakeInvitations struct {
	admit bool
	err   error
	asked []string
}

func (f *fakeInvitations) Admits(_ context.Context, id uuid.UUID, token, email string) (bool, error) {
	f.asked = append(f.asked, id.String()+" "+token+" "+email)
	return f.admit, f.err
}

// Open sign-up does not look at the invitation; closed sign-up opens only
// to an invitation the workspace module admits, for the address the
// attempt shows, and its failure is the policy's.
func TestSignupPolicy(t *testing.T) {
	id := uuid.NewV7()
	invited := identity.SignupAttempt{Email: "dana@corp.com", Invitation: &identity.SignupInvitation{ID: id, Token: "nwk_inv_x"}}
	plain := identity.SignupAttempt{Email: "dana@corp.com"}
	boom := errors.New("database down")
	for _, tt := range []struct {
		name    string
		open    bool
		check   fakeInvitations
		attempt identity.SignupAttempt
		allowed bool
		err     error
		asked   bool
	}{
		{"open, without an invitation", true, fakeInvitations{}, plain, true, nil, false},
		{"open, with one it would not admit", true, fakeInvitations{}, invited, true, nil, false},
		{"closed, without an invitation", false, fakeInvitations{admit: true}, plain, false, nil, false},
		{"closed, with one it admits", false, fakeInvitations{admit: true}, invited, true, nil, true},
		{"closed, with one it does not admit", false, fakeInvitations{}, invited, false, nil, true},
		{"closed, and the check fails", false, fakeInvitations{err: boom}, invited, false, boom, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			check := tt.check
			allowed, err := signupPolicy{open: tt.open, invitations: &check}.AllowSignup(context.Background(), tt.attempt)

			if allowed != tt.allowed || !errors.Is(err, tt.err) {
				t.Errorf("AllowSignup() = %v, %v; want %v, %v", allowed, err, tt.allowed, tt.err)
			}
			want := []string(nil)
			if tt.asked {
				want = []string{id.String() + " nwk_inv_x dana@corp.com"}
			}
			if len(check.asked) != len(want) || (len(want) == 1 && check.asked[0] != want[0]) {
				t.Errorf("the check was asked %q, want %q", check.asked, want)
			}
		})
	}
}
