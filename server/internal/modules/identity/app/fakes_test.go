package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// The instant and the ids the tests use.
const (
	userIDText    = "0199a2b4-0000-7000-8000-000000000001"
	sessionIDText = "0199a2b4-0000-7000-8000-000000000002"
)

func testNow() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 123456000, time.UTC) }

// accessExpiry is the exp of an access token issued at testNow: 15 minutes
// later, rounded up to the second.
func accessExpiry() time.Time { return time.Date(2026, 9, 25, 10, 15, 1, 0, time.UTC) }

// testIssuance is auth's default TTLs, 15 minutes and 30 days, with mac.
func testIssuance(tokens *fakeTokens, mac fakeMAC) app.Issuance {
	return app.Issuance{Tokens: tokens, MAC: mac, AccessTTL: 15 * time.Minute, SessionTTL: 720 * time.Hour}
}

func testUserID() uuid.UUID    { return uuid.MustParse(userIDText) }
func testSessionID() uuid.UUID { return uuid.MustParse(sessionIDText) }

// fixedClock is always at its instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// fakeTx runs fn in a context marked as inside the transaction; fakeStore
// records whether each write happened there.
type fakeTx struct{ calls int }

type inTxKey struct{}

func (f *fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	return fn(context.WithValue(ctx, inTxKey{}, true))
}

func inTx(ctx context.Context) bool { return ctx.Value(inTxKey{}) == true }

// fakeHasher "hashes" by prefixing "hashed:"; a hash "old:<password>" has
// other parameters, so it verifies and asks for a rehash. It counts its
// calls and records the hashes it verified against.
type fakeHasher struct {
	calls      int // Hash calls
	calledInTx int // Hash calls inside a transaction: a hash holds no lock
	err        error
	verified   []string // the hashes Verify was given
	verifyErr  error
	onVerify   func() // runs inside every Verify: a transaction that commits meanwhile
}

func (h *fakeHasher) Hash(ctx context.Context, password string) (string, error) {
	h.calls++
	if inTx(ctx) {
		h.calledInTx++
	}
	if h.err != nil {
		return "", h.err
	}
	return "hashed:" + password, nil
}

func (h *fakeHasher) Verify(_ context.Context, password, hash string) (bool, bool, error) {
	h.verified = append(h.verified, hash)
	if h.onVerify != nil {
		h.onVerify()
	}
	if h.verifyErr != nil {
		return false, false, h.verifyErr
	}
	switch hash {
	case "hashed:" + password:
		return true, false, nil
	case "old:" + password:
		return true, true, nil
	}
	return false, false, nil
}

// fakeTokens issues "access:<sid>" and verifies what it issued at the instant
// it is given: like a JWT's exp, a token is expired from its ExpiresAt on. It
// records those instants.
type fakeTokens struct {
	issued     []app.AccessClaims
	claims     map[string]app.AccessClaims
	verifiedAt []time.Time
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{claims: map[string]app.AccessClaims{}}
}

func (f *fakeTokens) Issue(c app.AccessClaims) (string, error) {
	f.issued = append(f.issued, c)
	token := "access:" + c.SessionID.String()
	f.claims[token] = c
	return token, nil
}

var errBadSignature = errors.New("signature is invalid")

func (f *fakeTokens) Verify(token string, now time.Time) (app.AccessClaims, error) {
	f.verifiedAt = append(f.verifiedAt, now)
	c, ok := f.claims[token]
	if !ok {
		return app.AccessClaims{}, errBadSignature
	}
	if !now.Before(c.ExpiresAt) {
		return app.AccessClaims{}, app.ErrAccessTokenExpired
	}
	return c, nil
}

// fakeMAC tags with the first 16 bytes of SHA-256 over its key and the
// message: deterministic, different for every message, and for every key
// (a change of the signing key).
type fakeMAC struct{ key string }

func (m fakeMAC) Tag(message []byte) [16]byte {
	sum := sha256.Sum256(append([]byte(m.key), message...))
	return [16]byte(sum[:16])
}

func (m fakeMAC) Verify(message []byte, tag [16]byte) bool { return m.Tag(message) == tag }

// fakeStore is every repository port the use cases need.
type fakeStore struct {
	users       []app.NewUser
	sessions    []app.NewSession
	outsideTx   []string // writes made outside a transaction
	createErr   error    // CreateUser's error
	getUser     domain.User
	getUserErr  error
	getUserIDs  []uuid.UUID // accounts looked up
	credential  app.SessionCredential
	credErr     error
	credentials []uuid.UUID // sessions looked up
}

func (s *fakeStore) CreateUser(ctx context.Context, u app.NewUser) error {
	if !inTx(ctx) {
		s.outsideTx = append(s.outsideTx, "user")
	}
	if s.createErr != nil {
		return s.createErr
	}
	s.users = append(s.users, u)
	return nil
}

func (s *fakeStore) CreateSession(ctx context.Context, n app.NewSession) error {
	if !inTx(ctx) {
		s.outsideTx = append(s.outsideTx, "session")
	}
	s.sessions = append(s.sessions, n)
	return nil
}

func (s *fakeStore) GetUser(_ context.Context, id uuid.UUID) (domain.User, error) {
	s.getUserIDs = append(s.getUserIDs, id)
	return s.getUser, s.getUserErr
}

func (s *fakeStore) SessionCredential(_ context.Context, id uuid.UUID) (app.SessionCredential, error) {
	s.credentials = append(s.credentials, id)
	return s.credential, s.credErr
}

// fixedPolicy answers allow and err, and records what it was shown.
type fixedPolicy struct {
	allow bool
	err   error
	seen  *[]app.SignupAttempt
}

func (p fixedPolicy) AllowSignup(_ context.Context, a app.SignupAttempt) (bool, error) {
	if p.seen != nil {
		*p.seen = append(*p.seen, a)
	}
	return p.allow, p.err
}
