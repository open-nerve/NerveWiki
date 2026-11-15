package domain

import (
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotReady is a stream opened while the server does not listen for
// events, starting or reconnecting: it could miss what is sent before the
// server listens. 503 not_ready, retry in a second (M5 design 4.10).
var ErrNotReady = shared.NotReady(time.Second)

// AccountStreamsRetry is the Retry-After of a stream refused at its
// account's bound: the web's longest backoff.
const AccountStreamsRetry = 30 * time.Second

// ErrTooManyStreams is a stream refused at its account's bound, the app's
// MaxAccountStreams: 429 rate_limited, retry in 30 seconds (M12 design
// 4.6.5). The web waits Retry-After, as it does for any refused opening.
var ErrTooManyStreams = &shared.Error{Kind: shared.KindRateLimited, Code: shared.CodeRateLimited,
	Detail: "This account has 64 event streams open: close one, or retry later.", RetryDelay: AccountStreamsRetry}
