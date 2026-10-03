package domain

import (
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotReady is a stream opened while the server does not listen for
// events, starting or reconnecting: it could miss what is sent before the
// server listens. 503 not_ready, retry in a second (M5 design 4.10).
var ErrNotReady = shared.NotReady(time.Second)
