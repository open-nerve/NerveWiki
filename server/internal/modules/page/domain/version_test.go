package domain_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A version holds the revisions from after its base to its own; the one
// that created its page, those from 1; none holds one below 1.
func TestAVersionHoldsTheRevisionsAfterItsBase(t *testing.T) {
	base := 2
	session := domain.Version{Base: &base, Revision: 5}
	creation := domain.Version{Revision: 3}
	for _, tt := range []struct {
		v        domain.Version
		revision int
		want     bool
	}{
		{session, 2, false}, {session, 3, true}, {session, 5, true}, {session, 6, false},
		{creation, 0, false}, {creation, -1, false}, {creation, 1, true}, {creation, 3, true}, {creation, 4, false},
	} {
		if got := tt.v.Holds(tt.revision); got != tt.want {
			t.Errorf("version (%v, %d] holds %d = %v, want %v", tt.v.Base, tt.v.Revision, tt.revision, got, tt.want)
		}
	}
}
