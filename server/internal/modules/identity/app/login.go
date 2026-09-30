package app

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LoginDeps are Login's collaborators and settings.
type LoginDeps struct {
	Accounts  LoginAccountReader
	Locker    CredentialLocker
	Passwords PasswordHashWriter
	Sessions  SessionCreator
	Verifier  PasswordVerifier
	Hasher    PasswordHasher // for a rehash
	Tx        shared.TxManager
	Issuance  Issuance
	Clock     Clock
	Logger    *slog.Logger
	// DummyHash is a hash of a random password with the current parameters,
	// made at startup: an unknown address is verified against it, so that
	// it takes as long as a known one (M1/P2 design 3.4).
	DummyHash string
}

// Login signs an account in with its password: POST /api/v0/auth/login.
type Login struct {
	d LoginDeps
}

// NewLogin returns the use case.
func NewLogin(d LoginDeps) *Login {
	return &Login{d: d}
}

// LoginInput is a sign-in and where it comes from.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        netip.Addr
}

// Execute signs in.Email in (M1/P2 design 3.4):
//
//  1. the account of the normalized address is read, its hash kept as the
//     snapshot; an unknown address is verified against DummyHash and is
//     401 identity.invalid_credentials, like a wrong password;
//  2. the password is verified against the snapshot outside the
//     transaction, and hashed again there when the parameters changed;
//  3. one transaction locks the account row, checks that the hash still
//     equals the snapshot and that the account is active (403
//     identity.account_deactivated, only now that the password is known to
//     be right), writes the new hash if any, and inserts the session.
//
// When the hash changed in between, a concurrent login hashed the password
// again, or the password changed: the password is verified against the new
// hash, and step 3 is done once more. A second change fails with 401.
func (l *Login) Execute(ctx context.Context, in LoginInput) (Tokens, error) {
	account, err := l.find(ctx, shared.NormalizeEmail(in.Email))
	if errors.Is(err, ErrNotFound) {
		if _, _, err := l.d.Verifier.Verify(ctx, in.Password, l.d.DummyHash); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, l.failed(ctx, domain.ErrInvalidCredentials, "invalid_credentials", in.IP, uuid.Nil())
	}
	if err != nil {
		return Tokens{}, err
	}

	snapshot := account.PasswordHash
	for range 2 {
		ok, rehash, err := l.d.Verifier.Verify(ctx, in.Password, snapshot)
		if err != nil {
			return Tokens{}, err
		}
		if !ok {
			break
		}
		newHash := ""
		if rehash {
			if newHash, err = l.d.Hasher.Hash(ctx, in.Password); err != nil {
				return Tokens{}, err
			}
		}
		now := l.d.Clock.Now()
		session, tokens, err := l.d.Issuance.newSession(account.ID, in.UserAgent, in.IP, now)
		if err != nil {
			return Tokens{}, err
		}

		current := snapshot
		err = l.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			locked, err := l.d.Locker.LockForCredentials(ctx, account.ID)
			if err != nil {
				return err
			}
			if current = locked.PasswordHash; current != snapshot {
				return nil
			}
			if !locked.Active {
				return domain.ErrAccountDeactivated
			}
			if newHash != "" {
				if err := l.d.Passwords.UpdatePasswordHash(ctx, account.ID, newHash, now); err != nil {
					return err
				}
			}
			return l.d.Sessions.CreateSession(ctx, session)
		})
		switch {
		case errors.Is(err, domain.ErrAccountDeactivated):
			return Tokens{}, l.failed(ctx, err, "deactivated", in.IP, account.ID)
		case err != nil:
			return Tokens{}, err
		case current == snapshot:
			l.d.Logger.InfoContext(ctx, "signed in", slog.String("user_id", account.ID.String()),
				slog.String("session_id", session.ID.String()), slog.String("ip", in.IP.String()))
			return tokens, nil
		}
		snapshot = current
	}
	return Tokens{}, l.failed(ctx, domain.ErrInvalidCredentials, "invalid_credentials", in.IP, account.ID)
}

// find reads the account of email. An address that cannot be valid is not
// looked up, so the database never sees what it could not store (a NUL, a
// byte that is not UTF-8).
func (l *Login) find(ctx context.Context, email string) (LoginAccount, error) {
	if !shared.ValidEmail(email) {
		return LoginAccount{}, ErrNotFound
	}
	return l.d.Accounts.FindLoginAccount(ctx, email)
}

// failed logs a failed sign-in and returns err: the account only when there
// is one, never the address.
func (l *Login) failed(ctx context.Context, err error, reason string, ip netip.Addr, userID uuid.UUID) error {
	attrs := []slog.Attr{slog.String("reason", reason), slog.String("ip", ip.String())}
	if userID != uuid.Nil() {
		attrs = append(attrs, slog.String("user_id", userID.String()))
	}
	l.d.Logger.LogAttrs(ctx, slog.LevelInfo, "sign-in failed", attrs...)
	return err
}
