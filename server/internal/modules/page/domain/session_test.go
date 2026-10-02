package domain_test

import (
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The lease and the heartbeat are the protocol's numbers (v0.1 design
// 3.9): the web editor holds the same two (M4/P6), and a session survives
// two lost heartbeats.
func TestTheEditSessionLeaseOutlastsTwoLostHeartbeats(t *testing.T) {
	if domain.EditSessionLease != 60*time.Second || domain.EditSessionHeartbeat != 20*time.Second {
		t.Errorf("lease %v, heartbeat %v; want 60s and 20s", domain.EditSessionLease, domain.EditSessionHeartbeat)
	}
	if domain.EditSessionLease < 3*domain.EditSessionHeartbeat {
		t.Errorf("a lease of %v outlasts fewer than three heartbeats of %v", domain.EditSessionLease, domain.EditSessionHeartbeat)
	}
}
