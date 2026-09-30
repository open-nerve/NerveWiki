package app

import (
	"crypto/rand"
	"net/netip"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// Tokens are what registration (and, from P2, login and refresh) return.
type Tokens struct {
	AccessToken      string
	AccessExpiresIn  time.Duration
	RefreshToken     string
	RefreshExpiresAt time.Time // the session's absolute end
}

// Issuance is how sessions and their tokens are made (M1/P1 design 3.4).
type Issuance struct {
	Tokens     AccessTokens
	MAC        RefreshTokenMAC
	AccessTTL  time.Duration // auth.access_token_ttl
	SessionTTL time.Duration // auth.session_ttl
}

// refreshToken returns generation of session id, with a new secret and
// its tag.
func (i Issuance) refreshToken(sessionID uuid.UUID, generation uint32) domain.RefreshToken {
	t := domain.RefreshToken{SessionID: sessionID, Generation: generation}
	_, _ = rand.Read(t.Secret[:]) // never fails since Go 1.24
	t.Tag = i.MAC.Tag(t.MACMessage())
	return t
}

// tokens returns refresh with a new access token of userID; sessionEnd is
// the session's absolute end.
func (i Issuance) tokens(userID uuid.UUID, refresh domain.RefreshToken, now, sessionEnd time.Time) (Tokens, error) {
	access, err := i.Tokens.Issue(AccessClaims{UserID: userID, SessionID: refresh.SessionID, ExpiresAt: now.Add(i.AccessTTL)})
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, AccessExpiresIn: i.AccessTTL, RefreshToken: refresh.String(), RefreshExpiresAt: sessionEnd}, nil
}

// newSession returns a new session of userID at generation 0, which the
// caller inserts, and its tokens: a sign-in from the client with userAgent
// at ip. The session ends SessionTTL from now, for good.
func (i Issuance) newSession(userID uuid.UUID, userAgent string, ip netip.Addr, now time.Time) (NewSession, Tokens, error) {
	refresh := i.refreshToken(uuid.NewV7(), 0)
	s := NewSession{
		ID:        refresh.SessionID,
		UserID:    userID,
		TokenHash: refresh.SecretHash(),
		UserAgent: domain.SanitizeUserAgent(userAgent),
		IP:        ip,
		ExpiresAt: now.Add(i.SessionTTL),
		Now:       now,
	}
	tokens, err := i.tokens(userID, refresh, now, s.ExpiresAt)
	return s, tokens, err
}
