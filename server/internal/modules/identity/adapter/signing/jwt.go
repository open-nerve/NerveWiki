package signing

import (
	"errors"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// The reasons Verify gives for a token it rejects, besides
// app.ErrAccessTokenExpired. They are fixed texts because Authenticate logs
// the reason (M1/P1 design 3.5), and the errors of jwt/v5 and encoding/json can
// quote what the sender put in a forged token.
var (
	// errMalformed: not three strict base64url segments of JSON whose
	// claims have their types.
	errMalformed = errors.New("access token is malformed")
	// errSignatureInvalid: not signed with EdDSA under this instance's key,
	// including another or an unknown algorithm.
	errSignatureInvalid = errors.New("access token signature is invalid")
	// errClaimsInvalid: signed with this instance's key, but exp is missing,
	// nbf is in the future, or sub or sid is not a uuid.
	errClaimsInvalid = errors.New("access token claims are invalid")
)

// claims are the access token's only claims: sub, sid and exp (M1/P1 design 3.4).
type claims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
}

// AccessTokens implements app.AccessTokens: JWTs signed with EdDSA
// (Ed25519), header {"alg":"EdDSA","typ":"JWT"}.
type AccessTokens struct {
	keys *Keys
}

// NewAccessTokens returns access tokens signed with keys.
func NewAccessTokens(keys *Keys) *AccessTokens {
	return &AccessTokens{keys: keys}
}

// Issue signs c.
func (a *AccessTokens) Issue(c app.AccessClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: c.UserID.String(), ExpiresAt: jwt.NewNumericDate(c.ExpiresAt)},
		SessionID:        c.SessionID.String(),
	})
	return token.SignedString(a.keys.private)
}

// Verify checks token at now: only EdDSA, strict base64url, exp required.
// A valid signature whose exp has passed is app.ErrAccessTokenExpired: the
// signature is checked before the claims (jwt/v5 parser.go). Every other
// failure is one of the fixed reasons above, never the library's text.
func (a *AccessTokens) Verify(token string, now time.Time) (app.AccessClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithStrictDecoding(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	var c claims
	if _, err := parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return a.keys.public, nil }); err != nil {
		return app.AccessClaims{}, rejection(err)
	}
	userID, errSub := uuid.Parse(c.Subject)
	sessionID, errSid := uuid.Parse(c.SessionID)
	if errSub != nil || errSid != nil {
		return app.AccessClaims{}, errClaimsInvalid
	}
	return app.AccessClaims{UserID: userID, SessionID: sessionID, ExpiresAt: c.ExpiresAt.Time}, nil
}

// rejection turns an error of the jwt/v5 parser into its fixed reason. With
// Verify's options the parser fails in four ways: malformed, unverifiable
// (the alg is unknown or unspecified), signature invalid (another algorithm
// or key) and invalid claims, which wrap jwt.ErrTokenExpired when exp has
// passed.
func rejection(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return app.ErrAccessTokenExpired
	case errors.Is(err, jwt.ErrTokenInvalidClaims):
		return errClaimsInvalid
	case errors.Is(err, jwt.ErrTokenMalformed):
		return errMalformed
	default: // jwt.ErrTokenSignatureInvalid, jwt.ErrTokenUnverifiable
		return errSignatureInvalid
	}
}
