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
