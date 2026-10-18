package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// largeBodies are the slots of the large request bodies (M9 design 4.14,
// 10; M9/P2 design 3.2): the server's, a channel of LargeBodies, and each
// credential's one, in a table that mu guards, an entry counted by the
// requests holding or waiting for it and gone with the last of them; the
// holders of the server's slots, for the saturated line alone; and how
// many requests wait now.
type largeBodies struct {
	server      chan struct{}
	wait        time.Duration
	above       map[string]int64
	waiting     atomic.Int64
	mu          sync.Mutex
	credentials map[string]*credentialSlot
	holders     map[uint64]largeHolder
	held        uint64 // the holders' last key
}

// credentialSlot is a credential's one slot, and how many requests hold or
// wait for it.
type credentialSlot struct {
	slot chan struct{}
	refs int
}

// largeHolder is a request holding a slot of the server's: its credential
// and its route, which the saturated line names.
type largeHolder struct {
	Credential string `json:"credential"`
	Route      string `json:"route"`
}

// largeBusyRetry is the Retry-After of a large body the slots turned away.
const largeBusyRetry = time.Second

// errSaturated is a wait for a large body's slot that ran out.
var errSaturated = errors.New("httpserver: the large bodies' slots stayed taken")

func newLargeBodies(n int, wait time.Duration, above map[string]int64) *largeBodies {
	if n == 0 {
		return nil
	}
	return &largeBodies{
		server:      make(chan struct{}, n),
		wait:        wait,
		above:       maps.Clone(above),
		credentials: map[string]*credentialSlot{},
		holders:     map[uint64]largeHolder{},
	}
}

// enter counts a request of credential in its entry, made when there is
// none.
func (l *largeBodies) enter(credential string) *credentialSlot {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.credentials[credential]
	if !ok {
		c = &credentialSlot{slot: make(chan struct{}, 1)}
		l.credentials[credential] = c
	}
	c.refs++
	return c
}

// leave uncounts a request of credential, its entry gone with the last.
func (l *largeBodies) leave(credential string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.credentials[credential]
	if c.refs--; c.refs == 0 {
		delete(l.credentials, credential)
	}
}

// hold records a holder of a slot of the server's, under the key it
// answers.
func (l *largeBodies) hold(h largeHolder) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held++
	l.holders[l.held] = h
	return l.held
}

func (l *largeBodies) unhold(key uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.holders, key)
}

// holding is the holders of the server's slots, in the order they took
// them.
func (l *largeBodies) holding() []largeHolder {
	l.mu.Lock()
	defer l.mu.Unlock()
	keys := slices.Sorted(maps.Keys(l.holders))
	hs := make([]largeHolder, len(keys))
	for i, k := range keys {
		hs[i] = l.holders[k]
	}
	return hs
}

// largeBody takes a slot of the large bodies for a request of a route of
// BodyLimits whose body is longer than its threshold, from before its
// body is read to when it has been served (M9/P2 design 3.2.1): a declared
// length past the route's limit is 413 at once, with no slot; one past the
// threshold takes its credential's slot and one of the server's; a body
// of no declared length is read up to the threshold first, and what was
// read goes back in front of the rest.
func (a *API) largeBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, ok := a.bodyLimits[r.Pattern]
		if a.large == nil || !ok {
			next.ServeHTTP(w, r)
			return
		}
		limit = max(a.maxBodyBytes, limit)
		threshold, ok := a.large.above[r.Pattern]
		if !ok {
			threshold = a.maxBodyBytes
		}
		switch {
		case r.ContentLength > limit:
			a.Errors.Write(w, r, &http.MaxBytesError{Limit: limit})
			return
		case r.ContentLength > threshold:
		case r.ContentLength >= 0:
			next.ServeHTTP(w, r)
			return
		default:
			prefix, err := io.ReadAll(io.LimitReader(r.Body, threshold+1))
			if err != nil {
				a.Errors.BodyError(w, r, err)
				return
			}
			r.Body = prefixedBody{io.MultiReader(bytes.NewReader(prefix), r.Body), r.Body}
			if int64(len(prefix)) <= threshold {
				next.ServeHTTP(w, r)
				return
			}
		}
		credential, ok := r.Context().Value(credentialKey{}).(string)
		if !ok {
			credential = RequestMetaFrom(r.Context()).IPKey
		}
		release, err := a.takeLarge(r.Context(), credential, r.Pattern)
		if err != nil {
			a.Errors.Write(w, r, err)
			return
		}
		defer release()
		next.ServeHTTP(w, r)
	})
}

// prefixedBody is a body whose first bytes were read and are read again
// in front of the rest; closing it closes the request's.
type prefixedBody struct {
	io.Reader
	io.Closer
}

// takeLarge takes the credential's slot, then one of the server's, for a
// request of route, waiting LargeBodyWait at most for both: serverBusy
// past it, the request's own error if it ends first, as the password
// hasher's acquire waits (identity/adapter/argon2/hasher.go:151-170). The
// func it answers gives both back, the server's first.
func (a *API) takeLarge(ctx context.Context, credential, route string) (func(), error) {
	l := a.large
	own := l.enter(credential)
	var timer *time.Timer // one for both waits, made by the first
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	var waiting int64 // the requests waiting, this one too, when the wait ran out
	take := func(slot chan struct{}) error {
		select {
		case slot <- struct{}{}:
			return nil
		default:
		}
		l.waiting.Add(1)
		defer l.waiting.Add(-1)
		if timer == nil {
			timer = time.NewTimer(l.wait)
		}
		select {
		case slot <- struct{}{}:
			return nil
		case <-timer.C:
			waiting = l.waiting.Load()
			return errSaturated
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := take(own.slot); err != nil {
		l.leave(credential)
		return nil, a.saturated(ctx, err, route, credential, true, waiting)
	}
	if err := take(l.server); err != nil {
		<-own.slot
		l.leave(credential)
		return nil, a.saturated(ctx, err, route, credential, false, waiting)
	}
	key := l.hold(largeHolder{Credential: credential, Route: route})
	return func() {
		l.unhold(key)
		<-l.server
		<-own.slot
		l.leave(credential)
	}, nil
}

// saturated is err of a wait for a slot: the request's own error, or
// serverBusy for a wait that ran out, logged at info as the parse budget's
// is (platform/markdown/budget.go): the route, the credential, own (its
// credential's slot was the one taken, by its own first), how many waited
// and who holds the server's slots, so that an administrator finds them.
// Credentials are ids (v0.1 design 13.1 rule 10).
func (a *API) saturated(ctx context.Context, err error, route, credential string, own bool, waiting int64) error {
	if !errors.Is(err, errSaturated) {
		return err
	}
	a.logger.LogAttrs(ctx, slog.LevelInfo, "large request bodies are saturated",
		slog.String("request_id", RequestID(ctx)), slog.String("route", route), slog.String("credential", credential),
		slog.Bool("own", own), slog.Int64("waiting", waiting), slog.Any("holders", a.large.holding()))
	return serverBusy(largeBusyRetry)
}

// serverBusy is 503 server_busy, the large bodies' slots all taken: the
// platform's own problem, as rateLimited is (limit.go:49-56).
type serverBusy time.Duration

func (serverBusy) Error() string               { return "The server is busy; retry shortly." }
func (serverBusy) ProblemStatus() int          { return http.StatusServiceUnavailable }
func (serverBusy) ProblemCode() string         { return CodeServerBusy }
func (b serverBusy) RetryAfter() time.Duration { return time.Duration(b) }
