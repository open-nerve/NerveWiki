package postgresadapter_test

import (
	"context"
	"maps"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

func TestProfiles(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	alice, bob, carol := newUser("alice@corp.com"), newUser("bob@corp.com"), newUser("carol@corp.com")
	bob.DisplayName = "Bob 研发"
	mustCreate(t, s, alice)
	mustCreate(t, s, bob)
	mustCreate(t, s, carol)

	got, err := s.Profiles(ctx, []uuid.UUID{alice.ID, bob.ID, uuid.NewV7()})

	want := map[uuid.UUID]domain.Profile{
		alice.ID: {DisplayName: "alice", Email: "alice@corp.com"},
		bob.ID:   {DisplayName: "Bob 研发", Email: "bob@corp.com"},
	}
	if err != nil || !maps.Equal(got, want) {
		t.Errorf("Profiles() = %v, %v; want alice's and bob's, and none for the unknown id", got, err)
	}
	if got, err := s.Profiles(ctx, nil); err != nil || len(got) != 0 {
		t.Errorf("Profiles(none) = %v, %v; want none", got, err)
	}
}

// An address finds its account, deactivated or not; another finds none.
// The address is compared as it is: the caller normalizes it.
func TestAccountIDByEmail(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	alice, bob := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, alice)
	mustCreate(t, s, bob)
	if err := s.DeactivateUser(ctx, bob.ID, now()); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		email string
		id    uuid.UUID
		ok    bool
	}{
		{"alice@corp.com", alice.ID, true},
		{"bob@corp.com", bob.ID, true},
		{"carol@corp.com", uuid.UUID{}, false},
		{"Alice@corp.com", uuid.UUID{}, false},
	} {
		if id, ok, err := s.AccountIDByEmail(ctx, tt.email); err != nil || id != tt.id || ok != tt.ok {
			t.Errorf("AccountIDByEmail(%s) = %s, %v, %v; want %s, %v", tt.email, id, ok, err, tt.id, tt.ok)
		}
	}
}
