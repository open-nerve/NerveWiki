package identity_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/signing"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The interleavings of the account row lock protocol (M1/P3 design 3.3,
// 3.4, 3.9) run the use cases on a real database. A gate stops one of them
// at a chosen point, outside its transaction or inside it holding the lock,
// while the test runs another; pgtest.WaitForLockWaits tells when a
// transaction waits for the lock. Every wait has a deadline, so a test fails
// rather than hangs.

// waitLimit bounds every wait of these tests.
const waitLimit = 10 * time.Second

// gate stops a use case until the test opens it.
type gate struct {
	reached, opened chan struct{}
	once            sync.Once
}

func newGate() *gate { return &gate{reached: make(chan struct{}), opened: make(chan struct{})} }

// stop tells the test that the use case reached the gate, once, and waits
// until it is opened.
func (g *gate) stop() error {
	g.once.Do(func() { close(g.reached) })
	select {
	case <-g.opened:
		return nil
	case <-time.After(waitLimit):
		return errors.New("the gate was never opened")
	}
}

// await waits until the use case stops at the gate.
func (g *gate) await(t *testing.T) {
	t.Helper()
	select {
	case <-g.reached:
	case <-time.After(waitLimit):
		t.Fatal("nothing reached the gate")
	}
}

func (g *gate) open() { close(g.opened) }

// gatedHasher stands in for argon2: Hash gives "hashed:<password>:<salt>",
// and Verify takes that for any salt, and "old:<password>" as a hash of
// other parameters, which it asks to rehash. With a gate, its first Verify
// stops there. It records the hashes it verified.
type gatedHasher struct {
	salt     string
	gate     *gate // nil: never stops
	mu       sync.Mutex
	verified []string
}

func (h *gatedHasher) Hash(_ context.Context, password string) (string, error) {
	return "hashed:" + password + ":" + h.salt, nil
}

func (h *gatedHasher) Verify(_ context.Context, password, hash string) (bool, bool, error) {
	h.mu.Lock()
	h.verified = append(h.verified, hash)
	first := len(h.verified) == 1
	h.mu.Unlock()
	if first && h.gate != nil {
		if err := h.gate.stop(); err != nil {
			return false, false, err
		}
	}
	if hash == "old:"+password {
		return true, true, nil
	}
	return strings.HasPrefix(hash, "hashed:"+password+":"), false, nil
}

// gatedSessionCreator stops a login inside its transaction, holding the
// account row lock, before it inserts its session.
type gatedSessionCreator struct {
	app.SessionCreator
	gate *gate
}

func (s gatedSessionCreator) CreateSession(ctx context.Context, n app.NewSession) error {
	if err := s.gate.stop(); err != nil {
		return err
	}
	return s.SessionCreator.CreateSession(ctx, n)
}

// gatedRevoker stops a password change inside its transaction, holding the
// account row lock, after it wrote the new hash and before it revokes the
// other sessions.
type gatedRevoker struct {
	app.SessionRevoker
	gate *gate
}

func (r gatedRevoker) RevokeSessions(ctx context.Context, userID, keep uuid.UUID, reason domain.RevokeReason, now time.Time) (int, error) {
	if err := r.gate.stop(); err != nil {
		return 0, err
	}
	return r.SessionRevoker.RevokeSessions(ctx, userID, keep, reason, now)
}

// gatedTokenCreator stops a token creation inside its transaction, holding
// the account row lock, once it inserted the token.
type gatedTokenCreator struct {
	app.APITokenCreator
	gate *gate
}

func (c gatedTokenCreator) CreateAPIToken(ctx context.Context, n app.NewAPIToken) error {
	if err := c.APITokenCreator.CreateAPIToken(ctx, n); err != nil {
		return err
	}
	return c.gate.stop()
}

// account is alice@corp.com, whose password is Tr0ub4dor&3 and whose
// stored hash is given, with live sessions, on a database of its own.
type account struct {
	store    *postgresadapter.Store
	pool     *pgxpool.Pool
	tx       shared.TxManager
	id       uuid.UUID
	sessions []uuid.UUID
}

func newAccount(t *testing.T, hash string, sessions int) *account {
	t.Helper()
	ctx := context.Background()
	pool := newPool(t)
	a := &account{store: postgresadapter.New(pool), pool: pool, tx: postgres.NewTxManager(pool, 2*time.Second), id: uuid.NewV7()}
	now := time.Now()
	if err := a.store.CreateUser(ctx, app.NewUser{ID: a.id, Email: "alice@corp.com", PasswordHash: hash, DisplayName: "alice", Now: now}); err != nil {
		t.Fatal(err)
	}
	for range sessions {
		id := uuid.NewV7()
		err := a.store.CreateSession(ctx, app.NewSession{ID: id, UserID: a.id, TokenHash: make([]byte, 32), ExpiresAt: now.Add(time.Hour), Now: now})
		if err != nil {
			t.Fatal(err)
		}
		a.sessions = append(a.sessions, id)
	}
	return a
}

// as is a context acting with the account's i-th session.
func (a *account) as(i int) context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: a.id, SessionID: a.sessions[i]})
}

func (a *account) currentPassword(h app.PasswordVerifier) app.CurrentPassword {
	return app.CurrentPassword{
		Accounts: a.store, Verifier: h, Tx: a.tx,
		Lock: app.CredentialLock{Locker: a.store, Sessions: a.store, APITokens: a.store},
	}
}

func (a *account) login(h *gatedHasher, sessions app.SessionCreator) *app.Login {
	keys := signing.EphemeralKeys()
	return app.NewLogin(app.LoginDeps{
		Accounts: a.store, Locker: a.store, Passwords: a.store, Sessions: sessions, Verifier: h, Hasher: h, Tx: a.tx,
		Issuance: app.Issuance{Tokens: signing.NewAccessTokens(keys), MAC: signing.NewRefreshTokenMAC(keys), AccessTTL: time.Minute, SessionTTL: time.Hour},
		Clock:    clock.System{}, Logger: slog.New(slog.DiscardHandler), DummyHash: "hashed:dummy:0",
	})
}

func (a *account) changePassword(h *gatedHasher, sessions app.SessionRevoker) *app.ChangePassword {
	return app.NewChangePassword(app.ChangePasswordDeps{
		Password: a.currentPassword(h), Rules: domain.NewPasswordRules(), Hasher: h, Passwords: a.store, Sessions: sessions,
		Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler),
	})
}

func (a *account) createToken(h *gatedHasher) *app.CreateAPIToken {
	return a.createTokenWith(h, a.store)
}

func (a *account) createTokenWith(h *gatedHasher, tokens app.APITokenCreator) *app.CreateAPIToken {
	return app.NewCreateAPIToken(app.CreateAPITokenDeps{
		Password: a.currentPassword(h), Tokens: tokens, Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler),
	})
}

// admin is the administrator's commands on the account's database.
func (a *account) admin() *identity.Admin {
	return identity.NewAdmin(identity.AdminDeps{
		Pool: a.pool, Tx: a.tx, Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler), Password: testPassword(),
	})
}

// createCI creates a token named CI as the account's i-th session.
func (a *account) createCI(h *gatedHasher, tokens app.APITokenCreator, i int) error {
	_, err := a.createTokenWith(h, tokens).Execute(a.as(i), app.CreateAPITokenInput{
		Spec: domain.APITokenSpec{Name: "CI"}, CurrentPassword: "Tr0ub4dor&3",
	})
	return err
}

// async runs fn and returns where its error arrives.
func async(fn func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	return done
}

func await(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(waitLimit):
		t.Fatal("the use case did not finish")
		return nil
	}
}

func (a *account) signIn(uc *app.Login) error {
	_, err := uc.Execute(context.Background(), app.LoginInput{Email: "alice@corp.com", Password: "Tr0ub4dor&3"})
	return err
}

func changeInput() app.ChangePasswordInput {
	return app.ChangePasswordInput{Current: "Tr0ub4dor&3", New: "N3w-Passw0rd!"}
}

// state is the account's hash and the revoke reason of each session, "" for
// a live one, oldest first.
func (a *account) state(t *testing.T) (string, []string) {
	t.Helper()
	var hash string
	if err := a.pool.QueryRow(context.Background(), `SELECT password FROM users`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash, sessionReasons(t, a.pool)
}

func (a *account) tokens(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.pool.QueryRow(context.Background(), `SELECT count(*) FROM api_tokens`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (a *account) liveTokens(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.pool.QueryRow(context.Background(), `SELECT count(*) FROM api_tokens WHERE revoked_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Interleaving 1: a password change waits for the transaction of a login
// that holds the account row lock, then revokes the session that login
// created: signing in with the old password leaves no live session behind.
// The login stops inside its transaction, before it inserts its session;
// the test lets it go on once the change waits for the lock. The change's
// UPDATE of the row would wait for the login as well: this guards what the
// change revokes, not the lock itself, which interleaving 4 guards.
func TestAPasswordChangeWaitsForALoginThatHoldsTheLock(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 1)
	g := newGate()

	login := async(func() error { return a.signIn(a.login(&gatedHasher{salt: "login"}, gatedSessionCreator{a.store, g})) })
	g.await(t)
	change := async(func() error {
		return a.changePassword(&gatedHasher{salt: "change"}, a.store).Execute(a.as(0), changeInput())
	})
	pgtest.WaitForLockWaits(t, a.pool, 1, waitLimit)
	g.open()
	loginErr, changeErr := await(t, login), await(t, change)

	hash, reasons := a.state(t)
	if loginErr != nil || changeErr != nil || hash != "hashed:N3w-Passw0rd!:change" || !slices.Equal(reasons, []string{"", "password_changed"}) {
		t.Errorf("login %v, change %v; hash %q, sessions %q; want both done, the caller's session live, the login's revoked", loginErr, changeErr, hash, reasons)
	}
}

// Interleaving 2: a token creation from a session that a password change
// revokes waits for the change, which holds the lock, and then, the lock
// taken, finds its session revoked: 401, no token. The change stops inside
// its transaction, after it wrote the new hash, before it revokes.
func TestATokenCreationWaitsForAChangeThatRevokesItsSession(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 2)
	g := newGate()

	change := async(func() error {
		return a.changePassword(&gatedHasher{salt: "change"}, gatedRevoker{a.store, g}).Execute(a.as(0), changeInput())
	})
	g.await(t)
	create := async(func() error {
		_, err := a.createToken(&gatedHasher{}).Execute(a.as(1), app.CreateAPITokenInput{
			Spec: domain.APITokenSpec{Name: "CI"}, CurrentPassword: "Tr0ub4dor&3",
		})
		return err
	})
	pgtest.WaitForLockWaits(t, a.pool, 1, waitLimit)
	g.open()
	changeErr, createErr := await(t, change), await(t, create)

	var se *shared.Error
	if changeErr != nil || !errors.As(createErr, &se) || se.ProblemStatus() != 401 || a.tokens(t) != 0 {
		t.Errorf("change %v, creation %v, %d tokens; want the change, then 401 and no token", changeErr, createErr, a.tokens(t))
	}
}

// A session that ends between authentication and the transaction (a logout,
// which takes no account lock) creates no token: the credential lock checks
// the session again under the lock. The hash did not change, so only that
// check stops it. The creation stops at its password check, outside its
// transaction, while the session ends.
func TestATokenCreationWhoseSessionEndsMeanwhile(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 1)
	g := newGate()

	create := async(func() error {
		_, err := a.createToken(&gatedHasher{gate: g}).Execute(a.as(0), app.CreateAPITokenInput{
			Spec: domain.APITokenSpec{Name: "CI"}, CurrentPassword: "Tr0ub4dor&3",
		})
		return err
	})
	g.await(t)
	if _, err := a.pool.Exec(context.Background(), `UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'logout'`); err != nil {
		t.Fatal(err)
	}
	g.open()
	err := await(t, create)

	var se *shared.Error
	if !errors.As(err, &se) || se.ProblemStatus() != 401 || a.tokens(t) != 0 {
		t.Errorf("creation = %v with %d tokens, want 401 and none", err, a.tokens(t))
	}
}

// Interleaving 3: a password change verified the current password against
// a hash of old parameters; before its transaction, a login hashes the
// password again and commits. Under the lock the change finds the new hash,
// verifies the current password against it once more, and changes the
// password; the login's session is one of the others it revokes.
func TestAPasswordChangeWhileALoginHashesThePasswordAgain(t *testing.T) {
	a := newAccount(t, "old:Tr0ub4dor&3", 1)
	g := newGate()
	changeHasher := &gatedHasher{salt: "change", gate: g}

	change := async(func() error { return a.changePassword(changeHasher, a.store).Execute(a.as(0), changeInput()) })
	g.await(t)
	loginErr := a.signIn(a.login(&gatedHasher{salt: "login"}, a.store))
	g.open()
	changeErr := await(t, change)

	hash, reasons := a.state(t)
	if loginErr != nil || changeErr != nil || hash != "hashed:N3w-Passw0rd!:change" || !slices.Equal(reasons, []string{"", "password_changed"}) {
		t.Errorf("login %v, change %v; hash %q, sessions %q; want both done, the new hash, the login's session revoked", loginErr, changeErr, hash, reasons)
	}
	if want := []string{"old:Tr0ub4dor&3", "hashed:Tr0ub4dor&3:login"}; !slices.Equal(changeHasher.verified, want) {
		t.Errorf("the change verified %q, want %q", changeHasher.verified, want)
	}
}

// Interleaving 4: two password changes, the second from a personal access
// token, which the first does not revoke. The second verified the current
// password against the old hash; it waits for the lock while the first holds
// it, then finds the hash changed, and the current password it was given no
// longer matches: 422, and the first's hash stays. Without the lock the
// second would wait only at its UPDATE, then write over the first's hash.
func TestAPasswordChangeWaitsForAnotherThatHoldsTheLock(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 1)
	_, tokenID := insertToken(t, a.pool, nil)
	g := newGate()

	first := async(func() error {
		return a.changePassword(&gatedHasher{salt: "first"}, gatedRevoker{a.store, g}).Execute(a.as(0), changeInput())
	})
	g.await(t)
	second := async(func() error {
		ctx := shared.WithActor(context.Background(), shared.Actor{UserID: a.id, APITokenID: tokenID})
		return a.changePassword(&gatedHasher{salt: "second"}, a.store).Execute(ctx, changeInput())
	})
	pgtest.WaitForLockWaits(t, a.pool, 1, waitLimit)
	g.open()
	firstErr, secondErr := await(t, first), await(t, second)

	hash, _ := a.state(t)
	if firstErr != nil || !errors.Is(secondErr, domain.ErrCurrentPasswordIncorrect) || hash != "hashed:N3w-Passw0rd!:first" {
		t.Errorf("first %v, second %v, hash %q; want the first done, the second %v, the first's hash", firstErr, secondErr, hash, domain.ErrCurrentPasswordIncorrect)
	}
}

// Interleaving 5: the administrator's password reset waits for a token
// creation that holds the account row lock, its token inserted, then
// revokes that token with the others: a reset leaves no token made with
// the old password (M1/P4 design 3.6).
func TestAResetWaitsForATokenCreationThatHoldsTheLock(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 1)
	g := newGate()

	create := async(func() error { return a.createCI(&gatedHasher{}, gatedTokenCreator{a.store, g}, 0) })
	g.await(t)
	reset := async(func() error {
		_, err := a.admin().ResetPassword(context.Background(), "alice@corp.com", "N3w-Passw0rd!")
		return err
	})
	pgtest.WaitForLockWaits(t, a.pool, 1, waitLimit)
	g.open()
	createErr, resetErr := await(t, create), await(t, reset)

	if createErr != nil || resetErr != nil || a.tokens(t) != 1 || a.liveTokens(t) != 0 {
		t.Errorf("creation %v, reset %v; %d tokens, %d live; want both done, the token revoked", createErr, resetErr, a.tokens(t), a.liveTokens(t))
	}
}

// Interleaving 6: a reset waits for a sign-in that holds the lock, then
// revokes the session it created.
func TestAResetWaitsForALoginThatHoldsTheLock(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 0)
	g := newGate()

	login := async(func() error { return a.signIn(a.login(&gatedHasher{salt: "login"}, gatedSessionCreator{a.store, g})) })
	g.await(t)
	reset := async(func() error {
		_, err := a.admin().ResetPassword(context.Background(), "alice@corp.com", "N3w-Passw0rd!")
		return err
	})
	pgtest.WaitForLockWaits(t, a.pool, 1, waitLimit)
	g.open()
	loginErr, resetErr := await(t, login), await(t, reset)

	if _, reasons := a.state(t); loginErr != nil || resetErr != nil || !slices.Equal(reasons, []string{"password_reset"}) {
		t.Errorf("login %v, reset %v, sessions %q; want both done, the login's session revoked", loginErr, resetErr, reasons)
	}
}

// A token creation that verified the old password before a reset finds, under
// the lock, its session revoked: 401, no token.
func TestATokenCreationThatVerifiedBeforeAReset(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 1)
	g := newGate()

	create := async(func() error { return a.createCI(&gatedHasher{gate: g}, a.store, 0) })
	g.await(t)
	if _, err := a.admin().ResetPassword(context.Background(), "alice@corp.com", "N3w-Passw0rd!"); err != nil {
		t.Fatal(err)
	}
	g.open()
	err := await(t, create)

	var se *shared.Error
	if !errors.As(err, &se) || se.ProblemStatus() != 401 || a.tokens(t) != 0 {
		t.Errorf("creation = %v with %d tokens, want 401 and none", err, a.tokens(t))
	}
}

// A password change whose account the administrator gives another address
// after the rules judged the new password against the old one: set-email
// keeps the tokens, so a change made with one still holds its credential
// under the lock. There the rules judge the password again, against the
// address the account has, and refuse one close to it; the hash stays. The
// change stops verifying the current password, outside its transaction,
// while the address changes (R3 of the M1 adversarial review).
func TestAPasswordChangeWhoseAddressChangesMeanwhile(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 0)
	_, tokenID := insertToken(t, a.pool, nil)
	g := newGate()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: a.id, APITokenID: tokenID})

	change := async(func() error {
		return a.changePassword(&gatedHasher{salt: "change", gate: g}, a.store).
			Execute(ctx, app.ChangePasswordInput{Current: "Tr0ub4dor&3", New: "Zqxj2026!"})
	})
	g.await(t)
	if _, err := a.admin().SetEmail(context.Background(), "alice@corp.com", "zqxj@corp.com"); err != nil {
		t.Fatal(err)
	}
	g.open()
	err := await(t, change)

	var se *shared.Error
	hash, _ := a.state(t)
	if !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Code != shared.FieldCommonPassword || hash != "hashed:Tr0ub4dor&3:0" {
		t.Errorf("change = %v with hash %q; want new_password %s and the hash unchanged", err, hash, shared.FieldCommonPassword)
	}
}

// A sign-in that found the account by an address the administrator changes
// before its transaction answers as for an unknown address, and starts no
// session: the old address no longer signs in (M1/P4 design 3.6).
func TestASignInWhoseAddressChangesMeanwhile(t *testing.T) {
	a := newAccount(t, "hashed:Tr0ub4dor&3:0", 0)
	g := newGate()

	login := async(func() error { return a.signIn(a.login(&gatedHasher{salt: "login", gate: g}, a.store)) })
	g.await(t)
	if _, err := a.admin().SetEmail(context.Background(), "alice@corp.com", "alice@example.org"); err != nil {
		t.Fatal(err)
	}
	g.open()
	err := await(t, login)

	if _, reasons := a.state(t); !errors.Is(err, domain.ErrInvalidCredentials) || len(reasons) != 0 {
		t.Errorf("login = %v with sessions %q, want identity.invalid_credentials and none", err, reasons)
	}
}
