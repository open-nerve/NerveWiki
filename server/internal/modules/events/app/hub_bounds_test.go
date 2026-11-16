package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// session is an actor of userID acting with the sign-in session id.
func session(userID, id uuid.UUID) shared.Actor { return shared.Actor{UserID: userID, SessionID: id} }

// openWith opens the stream of the actor on h, closed when the test ends.
func openWith(t *testing.T, h *app.Hub, a shared.Actor) (*app.Stream, error) {
	t.Helper()
	s, err := app.NewOpenStream(h, fakeVisibility{}).Execute(shared.WithActor(context.Background(), a))
	if err == nil {
		t.Cleanup(s.Close)
	}
	return s, err
}

// mustOpen is openWith failing t on an error.
func mustOpen(t *testing.T, h *app.Hub, a shared.Actor) *app.Stream {
	t.Helper()
	s, err := openWith(t, h, a)
	if err != nil {
		t.Fatalf("open %+v: %v", a, err)
	}
	return s
}

// held is a credential's streams, oldest first, and the actor of them.
type held struct {
	actor   shared.Actor
	streams []*app.Stream
}

// fill opens n sessions' MaxCredentialStreams streams each of userID on h,
// and returns them by session.
func fill(t *testing.T, h *app.Hub, userID uuid.UUID, n int) []held {
	t.Helper()
	out := make([]held, n)
	for i := range out {
		out[i].actor = session(userID, uuid.NewV7())
		for range app.MaxCredentialStreams {
			out[i].streams = append(out[i].streams, mustOpen(t, h, out[i].actor))
		}
	}
	return out
}

// A credential's fifth stream supersedes its oldest, which ends with
// superseded after the events it held; the other four go on. Another
// credential of the account, opened before them, and another account are
// untouched: the bound is the credential's (M12 design 4.6.5).
func TestACredentialsFifthStreamSupersedesItsOldest(t *testing.T) {
	h := listeningHub()
	other := mustOpen(t, h, session(alice(), uuid.NewV7()))
	bobs := mustOpen(t, h, session(bob(), uuid.NewV7()))
	id := uuid.NewV7()
	var streams []*app.Stream
	for range app.MaxCredentialStreams {
		streams = append(streams, mustOpen(t, h, session(alice(), id)))
	}
	h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{Pages: []domain.PageRevision{}}))
	fifth := mustOpen(t, h, session(alice(), id))
	if got := resetFor(streams[0]); got != domain.ResetSuperseded {
		t.Fatalf("the oldest's reset = %q, want superseded", got)
	}
	if got := received(streams[0]); len(got) != 1 || got[0].Type != domain.TypePages {
		t.Errorf("the oldest holds %v, want the event it was offered before", got)
	}
	h.Dispatch(ev(t, "later", acme(), seen(), map[string]int{"n": 1}))
	for i, s := range append(streams[1:], fifth, other) {
		want := []domain.Type{domain.TypePages, "later"}
		if s == fifth {
			want = want[1:]
		}
		var got []domain.Type
		for _, e := range received(s) {
			got = append(got, e.Type)
		}
		if r := resetFor(s); r != "" || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("stream %d: reset %q, got %v; want it open with %v", i, r, got, want)
		}
	}
	if r := resetFor(bobs); r != "" {
		t.Errorf("bob's stream reset %q", r)
	}
	if n := h.Streams(); n != 7 {
		t.Errorf("%d streams open, want 7: the superseded one until its handler closes it", n)
	}
}

// Each personal access token is a credential of its own, as each
// sign-in's session is (P3 design 3.9.1): alice's first token's fifth
// stream supersedes that token's oldest alone; her second token's, bob's
// token's and her session's streams, opened before them, are untouched,
// and each account counts its own (P3/S1 code review C-M1).
func TestEachTokenIsACredentialOfItsOwn(t *testing.T) {
	h := listeningHub()
	token := func(userID uuid.UUID) shared.Actor { return shared.Actor{UserID: userID, APITokenID: uuid.NewV7()} }
	first, second, bobs := token(alice()), token(alice()), token(bob())
	others := []*app.Stream{mustOpen(t, h, second), mustOpen(t, h, bobs), mustOpen(t, h, session(alice(), uuid.NewV7()))}
	var streams []*app.Stream
	for range app.MaxCredentialStreams {
		streams = append(streams, mustOpen(t, h, first))
	}
	fifth := mustOpen(t, h, first)
	for i, s := range append(others, streams...) {
		want := domain.ResetReason("")
		if s == streams[0] {
			want = domain.ResetSuperseded
		}
		if got := resetFor(s); got != want {
			t.Errorf("stream %d: reset %q, want %q", i, got, want)
		}
	}
	if resetFor(fifth) != "" {
		t.Error("the fifth was reset")
	}
	if a, b := app.AccountStreams(h, alice()), app.AccountStreams(h, bob()); a != app.MaxCredentialStreams+2 || b != 1 {
		t.Errorf("alice counts %d, bob %d; want %d and 1", a, b, app.MaxCredentialStreams+2)
	}
}

// A superseded stream leaves its own account's count, as its close would:
// the counts stay right should a credential ever be shared by accounts,
// as a credentialOf that lost a kind's case would make it (P3/S1 code
// review C-M1). No sign-in nor token is shared: bob's session id given
// to alice stands for such a fault.
func TestASupersededStreamLeavesItsOwnAccount(t *testing.T) {
	h := listeningHub()
	id := uuid.NewV7()
	var bobs []*app.Stream
	for range app.MaxCredentialStreams {
		bobs = append(bobs, mustOpen(t, h, session(bob(), id)))
	}
	alices := []*app.Stream{mustOpen(t, h, session(alice(), id))}
	if resetFor(bobs[0]) != domain.ResetSuperseded {
		t.Fatal("bob's oldest was not superseded")
	}
	if a, b := app.AccountStreams(h, alice()), app.AccountStreams(h, bob()); a != 1 || b != app.MaxCredentialStreams-1 {
		t.Errorf("alice counts %d, bob %d; want 1 and %d", a, b, app.MaxCredentialStreams-1)
	}
	// Superseded down to none, an account is no entry, as one closed down to
	// none is not; closed, every stream is forgotten (P3/S1 code review's
	// check, C-C1-N2).
	for range app.MaxCredentialStreams - 1 {
		alices = append(alices, mustOpen(t, h, session(alice(), id)))
	}
	if credentials, accounts := app.HubCounts(h); credentials != 1 || accounts != 1 || app.AccountStreams(h, alice()) != app.MaxCredentialStreams {
		t.Errorf("bob's all superseded: %d credentials, %d accounts counted, alice %d; want 1, 1 and %d", credentials, accounts,
			app.AccountStreams(h, alice()), app.MaxCredentialStreams)
	}
	for _, s := range append(bobs, alices...) {
		s.Close()
	}
	if credentials, accounts := app.HubCounts(h); credentials != 0 || accounts != 0 {
		t.Errorf("all closed: %d credentials, %d accounts counted; want none", credentials, accounts)
	}
}

// An account holds MaxAccountStreams over its credentials: the first
// stream of a seventeenth credential, past them, is refused,
// ErrTooManyStreams; once one closes, it opens. Closed, every stream is
// forgotten: the counts hold no credential and no account (M12 design
// 4.6.5; P3 design review B-N10).
func TestAnAccountsSixtyFifthStreamIsRefused(t *testing.T) {
	h := listeningHub()
	full := fill(t, h, alice(), app.MaxAccountStreams/app.MaxCredentialStreams)
	late := session(alice(), uuid.NewV7())
	if _, err := openWith(t, h, late); !errors.Is(err, domain.ErrTooManyStreams) {
		t.Fatalf("the 65th = %v, want ErrTooManyStreams", err)
	}
	if s := mustOpen(t, h, session(bob(), uuid.NewV7())); resetFor(s) != "" {
		t.Error("another account's stream was refused or reset")
	}
	full[3].streams[2].Close()
	last := mustOpen(t, h, late)
	if _, err := openWith(t, h, session(alice(), uuid.NewV7())); !errors.Is(err, domain.ErrTooManyStreams) {
		t.Errorf("past 64 again = %v, want ErrTooManyStreams", err)
	}
	for _, c := range full {
		for _, s := range c.streams {
			s.Close()
		}
	}
	last.Close()
	if credentials, accounts := app.HubCounts(h); credentials != 1 || accounts != 1 {
		t.Errorf("with bob's stream alone: %d credentials, %d accounts counted, want 1 and 1", credentials, accounts)
	}
}

// Closed, the hub's counts hold nothing: neither a credential whose
// streams all closed nor an account at 0, a superseded stream's close
// counting nothing.
func TestTheHubForgetsWhatClosed(t *testing.T) {
	h := listeningHub()
	id := uuid.NewV7()
	var all []*app.Stream
	for range app.MaxCredentialStreams + 2 {
		all = append(all, mustOpen(t, h, session(alice(), id)))
	}
	all = append(all, mustOpen(t, h, shared.Actor{UserID: bob(), APITokenID: uuid.NewV7()}))
	for _, s := range all {
		s.Close()
		s.Close() // closing twice counts nothing twice
	}
	if credentials, accounts := app.HubCounts(h); credentials != 0 || accounts != 0 || h.Streams() != 0 {
		t.Errorf("all closed: %d credentials, %d accounts, %d streams; want none", credentials, accounts, h.Streams())
	}
}

// An account at its bound has a credential at its own take the place of
// its oldest: never refused, the account's count the same (M12 design
// 4.6.5): a seventeenth credential is refused after it as before.
func TestAFullAccountsCredentialSupersedesItsOwn(t *testing.T) {
	h := listeningHub()
	full := fill(t, h, alice(), app.MaxAccountStreams/app.MaxCredentialStreams)
	again, err := openWith(t, h, full[5].actor)
	if err != nil || resetFor(full[5].streams[0]) != domain.ResetSuperseded || resetFor(again) != "" {
		t.Fatalf("a full credential of a full account = %v, its oldest %q; want its oldest superseded", err, resetFor(full[5].streams[0]))
	}
	if got := app.AccountStreams(h, alice()); got != app.MaxAccountStreams {
		t.Errorf("the account counts %d, want %d", got, app.MaxAccountStreams)
	}
	if _, err := openWith(t, h, session(alice(), uuid.NewV7())); !errors.Is(err, domain.ErrTooManyStreams) {
		t.Errorf("a new credential = %v, want ErrTooManyStreams", err)
	}
}

// A superseded stream counts no longer at once, though its handler has
// not closed it: an account at 63, a credential at 4 opening its fifth,
// then a new credential's first opens, the 64th (P3 design review B-N11:
// at 64, either way of counting refuses).
func TestASupersededStreamLeavesTheCountsAtOnce(t *testing.T) {
	h := listeningHub()
	full := fill(t, h, alice(), app.MaxAccountStreams/app.MaxCredentialStreams-1)
	three := uuid.NewV7()
	for range app.MaxCredentialStreams - 1 {
		mustOpen(t, h, session(alice(), three))
	}
	if got := app.AccountStreams(h, alice()); got != app.MaxAccountStreams-1 {
		t.Fatalf("the account counts %d, want 63", got)
	}
	mustOpen(t, h, full[0].actor)
	if resetFor(full[0].streams[0]) != domain.ResetSuperseded {
		t.Fatal("the full credential's oldest was not superseded")
	}
	if _, err := openWith(t, h, session(alice(), uuid.NewV7())); err != nil {
		t.Errorf("the 64th, the superseded one not closed = %v, want it open", err)
	}
}

// Clients that open again at once when reset come to rest (M12 design
// 4.6.5; review A-I1 of the M12 design): seventeen credentials of one
// account, one client each, open seventeen streams and none is reset. Had
// the bound been the account's, they would supersede each other forever.
// Five clients of one credential do: each step resets exactly one, the
// account counts 4 throughout, and they turn round for as long as they
// run, a credential's own affair.
func TestClientsThatReconnectAtOnceComeToRest(t *testing.T) {
	// run steps the clients: a client whose stream was reset closes it and
	// opens another, one a step, for 1,000 steps at most; it answers the
	// opens and the resets each step saw.
	run := func(t *testing.T, h *app.Hub, actors []shared.Actor, steps int) (opens int, resets []int) {
		t.Helper()
		streams := make([]*app.Stream, len(actors))
		for i, a := range actors {
			streams[i] = mustOpen(t, h, a)
			opens++
		}
		for range steps {
			n, first := 0, -1
			for i, s := range streams {
				if resetFor(s) != "" {
					n++
					if first < 0 {
						first = i
					}
				}
			}
			resets = append(resets, n)
			if first < 0 {
				return opens, resets
			}
			streams[first].Close()
			streams[first] = mustOpen(t, h, actors[first])
			opens++
		}
		return opens, resets
	}
	h := listeningHub()
	var actors []shared.Actor
	for range 17 {
		actors = append(actors, session(alice(), uuid.NewV7()))
	}
	if opens, resets := run(t, h, actors, 1_000); opens != 17 || len(resets) != 1 || resets[0] != 0 {
		t.Errorf("17 credentials: %d opens, resets %v; want 17 and none", opens, resets[:min(len(resets), 5)])
	}
	h = listeningHub()
	id := uuid.NewV7()
	actors = []shared.Actor{session(bob(), id), session(bob(), id), session(bob(), id), session(bob(), id), session(bob(), id)}
	_, resets := run(t, h, actors, 100)
	if len(resets) != 100 || strings.Trim(fmt.Sprint(resets), "[1 ]") != "" {
		t.Errorf("5 clients of one credential: resets each step %v, want exactly one, for every step", resets)
	}
	if got := app.AccountStreams(h, bob()); got != app.MaxCredentialStreams {
		t.Errorf("the account counts %d, want %d", got, app.MaxCredentialStreams)
	}
}

// The refusal says the bound it is: its 64 is MaxAccountStreams, and it is
// 429 rate_limited after 30 seconds, the web's longest backoff.
func TestTheRefusalSaysItsBound(t *testing.T) {
	e := domain.ErrTooManyStreams
	if !strings.Contains(e.Detail, fmt.Sprintf(" %d event streams ", app.MaxAccountStreams)) || e.Kind != shared.KindRateLimited ||
		e.Code != shared.CodeRateLimited || e.RetryDelay != domain.AccountStreamsRetry || domain.AccountStreamsRetry.Seconds() != 30 {
		t.Errorf("ErrTooManyStreams = %+v, want rate_limited after 30 s, its %d in its words", e, app.MaxAccountStreams)
	}
}
