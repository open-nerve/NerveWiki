package domain_test

import (
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The lease and the heartbeat are the protocol's numbers (v0.1 design
// 3.9; M5 design 4.6): the web editor beats as often (M4/P6), and a
// session survives a hidden tab's heartbeats throttled to one a minute,
// with a minute to spare.
func TestTheEditSessionLeaseOutlastsThrottledHeartbeats(t *testing.T) {
	if domain.EditSessionLease != 120*time.Second || domain.EditSessionHeartbeat != 20*time.Second {
		t.Errorf("lease %v, heartbeat %v; want 120s and 20s", domain.EditSessionLease, domain.EditSessionHeartbeat)
	}
	if throttled := time.Minute; domain.EditSessionLease < 2*throttled {
		t.Errorf("a lease of %v outlasts fewer than two throttled heartbeats of %v", domain.EditSessionLease, throttled)
	}
}
