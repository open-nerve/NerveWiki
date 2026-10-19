package bootstrap

import (
	"log/slog"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
)

// The composition root reads and serves two large request bodies at once,
// one a credential, each waiting two seconds at most for its slot (M9/P2
// design 3.2.2; M9 design 11, item 16): the large bodies' whole-program
// test waits less, so the numbers are pinned here, of apiConfig itself,
// without an app or a database.
func TestTheAPIConfigBoundsTheLargeBodies(t *testing.T) {
	cfg := testConfig(t, "", false)
	got := apiConfig(cfg, slog.New(slog.DiscardHandler), ratelimit.New(time.Now), nil, nil, nil, nil)
	if got.LargeBodies != 2 || got.LargeBodyWait != 2*time.Second {
		t.Errorf("apiConfig bounds %d large bodies, waiting %v; want 2 and 2s", got.LargeBodies, got.LargeBodyWait)
	}
}
