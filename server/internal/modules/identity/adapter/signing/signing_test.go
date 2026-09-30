package signing

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// testKeyPEM is what `openssl genpkey -algorithm ed25519` writes (OpenSSL
// 3.6.3). A key for tests only.
const testKeyPEM = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIPneKoGsY0rpwLc94vhBW73igdoPaBAvGyjWyYTwGsUX
-----END PRIVATE KEY-----
`

func testKeys(t *testing.T) *Keys {
	t.Helper()
	k, err := ParseKeys([]byte(testKeyPEM))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestParseKeysReadsAnOpenSSLKey(t *testing.T) {
	k := testKeys(t)
	if len(k.private) != 64 || len(k.public) != 32 || len(k.mac) != 32 {
		t.Errorf("key sizes = %d, %d, %d; want 64, 32, 32", len(k.private), len(k.public), len(k.mac))
	}
	if bytes.Equal(k.mac, k.private.Seed()) {
		t.Error("the MAC key equals the seed, want a derived key")
	}
}

func TestParseKeysRejects(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecPKCS8, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, pem, want string }{
		{"not PEM", "secret-looking-garbage", `not a PEM "PRIVATE KEY" block (PKCS#8)`},
		{"another block type", strings.ReplaceAll(testKeyPEM, "PRIVATE KEY", "EC PRIVATE KEY"), `not a PEM "PRIVATE KEY" block (PKCS#8)`},
		{"an EC key", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecPKCS8})), "the key is *ecdsa.PrivateKey, want an Ed25519 key"},
		{"not PKCS#8", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("secret-looking-garbage")})), "parse PKCS#8: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseKeys([]byte(tt.pem))
			if err == nil || !strings.HasPrefix(err.Error(), tt.want) || strings.Contains(err.Error(), "secret-looking-garbage") {
				t.Errorf("ParseKeys() = %v, want %q without the input", err, tt.want)
			}
		})
	}
}

func TestEphemeralKeysDiffer(t *testing.T) {
	a, b := EphemeralKeys(), EphemeralKeys()
	if bytes.Equal(a.private, b.private) || bytes.Equal(a.mac, b.mac) {
		t.Error("two ephemeral keys are equal")
	}
}

// The claims of the tokens the tests issue.
const (
	userIDText    = "0199a2b4-0000-7000-8000-000000000001"
	sessionIDText = "0199a2b4-0000-7000-8000-000000000002"
)

func testNow() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC) }

func issue(t *testing.T, a *AccessTokens, exp time.Time) string {
	t.Helper()
	token, err := a.Issue(app.AccessClaims{UserID: uuid.MustParse(userIDText), SessionID: uuid.MustParse(sessionIDText), ExpiresAt: exp})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func segment(t *testing.T, token string, i int) string {
	t.Helper()
	parts := strings.Split(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(parts[i])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	userID, sessionID := uuid.MustParse(userIDText), uuid.MustParse(sessionIDText)
	now := testNow()
	a := NewAccessTokens(testKeys(t))
	token := issue(t, a, now.Add(15*time.Minute))

	if h := segment(t, token, 0); h != `{"alg":"EdDSA","typ":"JWT"}` {
		t.Errorf("header = %s", h)
	}
	if p := segment(t, token, 1); p != `{"sub":"`+userID.String()+`","exp":`+strconv.FormatInt(now.Add(15*time.Minute).Unix(), 10)+`,"sid":"`+sessionID.String()+`"}` {
		t.Errorf("payload = %s, want only sub, exp and sid", p)
	}
	got, err := a.Verify(token, now)
	want := app.AccessClaims{UserID: userID, SessionID: sessionID, ExpiresAt: now.Add(15 * time.Minute)}
	if err != nil || got.UserID != want.UserID || got.SessionID != want.SessionID || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("Verify() = %+v, %v; want %+v", got, err, want)
	}
}

func TestAccessTokenExpiry(t *testing.T) {
	now := testNow()
	a := NewAccessTokens(testKeys(t))
	token := issue(t, a, now.Add(time.Minute))

	if _, err := a.Verify(token, now.Add(59*time.Second)); err != nil {
		t.Errorf("Verify() a second before exp = %v, want valid", err)
	}
	if _, err := a.Verify(token, now.Add(time.Minute)); !errors.Is(err, app.ErrAccessTokenExpired) {
		t.Errorf("Verify() at exp = %v, want ErrAccessTokenExpired", err)
	}
	// An expired token with a bad signature is invalid, not expired.
	forged := token[:len(token)-4] + "AAAA"
	if _, err := a.Verify(forged, now.Add(time.Hour)); err == nil || errors.Is(err, app.ErrAccessTokenExpired) {
		t.Errorf("Verify() of a forged expired token = %v, want invalid and not expired", err)
	}
}

func TestAccessTokenVerifyRejects(t *testing.T) {
	userID, sessionID := uuid.MustParse(userIDText), uuid.MustParse(sessionIDText)
	now := testNow()
	keys := testKeys(t)
	a := NewAccessTokens(keys)
	valid := issue(t, a, now.Add(time.Minute))
	parts := strings.Split(valid, ".")
	sign := func(c jwt.Claims) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(keys.private)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	hs256, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String(), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))},
		SessionID:        sessionID.String(),
	}).SignedString([]byte(keys.public)) // the public key as an HMAC secret: the classic confusion
	if err != nil {
		t.Fatal(err)
	}
	tampered := base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(segment(t, valid, 1), userID.String(), sessionID.String(), 1)))
	tests := []struct{ name, token string }{
		{"another key", issue(t, NewAccessTokens(EphemeralKeys()), now.Add(time.Minute))},
		{"HS256", hs256},
		{"alg none", base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + parts[1] + "."},
		{"tampered payload", parts[0] + "." + tampered + "." + parts[2]},
		{"padded base64", parts[0] + "." + parts[1] + "=." + parts[2]},
		{"no exp", sign(claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}, SessionID: sessionID.String()})},
		{"no sub", sign(claims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}, SessionID: sessionID.String()})},
		{"no sid", sign(claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String(), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}})},
		{"a refresh token", "nwk_rt_" + strings.Repeat("A", 91)},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := a.Verify(tt.token, now); err == nil || errors.Is(err, app.ErrAccessTokenExpired) {
				t.Errorf("Verify() = %v, want invalid", err)
			}
		})
	}
}

// Every rejection is exactly one of the fixed reasons, never the text of
// jwt/v5 or encoding/json: Authenticate logs the reason (M1/P1 design 3.5), and
// that text can quote what the sender put in a forged token.
func TestAccessTokenVerifyGivesOnlyFixedReasons(t *testing.T) {
	userID, sessionID := uuid.MustParse(userIDText), uuid.MustParse(sessionIDText)
	now := testNow()
	const marker = "MARKER-chosen-by-the-sender"
	keys := testKeys(t)
	a := NewAccessTokens(keys)
	parts := strings.Split(issue(t, a, now.Add(time.Minute)), ".")
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	// forge keeps a valid header and signature over another payload.
	forge := func(payload string) string { return parts[0] + "." + b64(payload) + "." + parts[2] }
	sign := func(method jwt.SigningMethod, key any, c jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(method, c).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	sub, sid, exp := userID.String(), sessionID.String(), now.Add(time.Minute).Unix()
	expJSON := `"exp":` + strconv.FormatInt(exp, 10)
	withTimes := func(times string) string { return `{"sub":"` + sub + `","sid":"` + sid + `",` + times + `}` }
	tests := []struct {
		name  string
		token string
		want  error
	}{
		{"exp is a string", forge(withTimes(`"exp":"` + marker + `"`)), errMalformed},
		{"nbf is a string", forge(withTimes(expJSON + `,"nbf":"` + marker + `"`)), errMalformed},
		{"iat is a string", forge(withTimes(expJSON + `,"iat":"` + marker + `"`)), errMalformed},
		{"payload is not JSON", forge(marker), errMalformed},
		{"header is not JSON", b64(marker) + "." + parts[1] + "." + parts[2], errMalformed},
		{"bad signature", forge(`{"sub":"` + marker + `",` + expJSON + `}`), errSignatureInvalid},
		{"signature is not a signature", parts[0] + "." + parts[1] + "." + b64(marker), errSignatureInvalid},
		{"alg none", b64(`{"alg":"none","typ":"JWT"}`) + "." + parts[1] + ".", errSignatureInvalid},
		{"alg HS256", sign(jwt.SigningMethodHS256, []byte(keys.public), jwt.MapClaims{"sub": sub, "sid": sid, "exp": exp}), errSignatureInvalid},
		{"alg unknown", b64(`{"alg":"`+marker+`","typ":"JWT"}`) + "." + parts[1] + "." + parts[2], errSignatureInvalid},
		{"signed, sub is not a uuid", sign(jwt.SigningMethodEdDSA, keys.private, jwt.MapClaims{"sub": marker, "sid": sid, "exp": exp}), errClaimsInvalid},
		{"signed, nbf in the future", sign(jwt.SigningMethodEdDSA, keys.private, jwt.MapClaims{"sub": sub, "sid": sid, "exp": exp, "nbf": exp}), errClaimsInvalid},
		{"signed, no exp", sign(jwt.SigningMethodEdDSA, keys.private, jwt.MapClaims{"sub": sub, "sid": sid}), errClaimsInvalid},
		{"expired", issue(t, a, now), app.ErrAccessTokenExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := a.Verify(tt.token, now)
			if err == nil || !errors.Is(err, tt.want) || err.Error() != tt.want.Error() {
				t.Fatalf("Verify() = %v, want exactly %q", err, tt.want)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("Verify() = %q quotes the sender's text", err)
			}
			for _, part := range strings.Split(tt.token, ".") {
				if part != "" && strings.Contains(err.Error(), part) {
					t.Errorf("Verify() = %q quotes the token's segment %q", err, part)
				}
			}
		})
	}
}

func TestRefreshTokenMAC(t *testing.T) {
	keys := testKeys(t)
	m := NewRefreshTokenMAC(keys)
	msg := bytes.Repeat([]byte{7}, 52)
	tag := m.Tag(msg)

	if m.Tag(bytes.Clone(msg)) != tag {
		t.Error("the tag of the same message differs")
	}
	for i := range msg {
		changed := bytes.Clone(msg)
		changed[i] ^= 1
		if m.Tag(changed) == tag {
			t.Errorf("changing byte %d keeps the tag", i)
		}
	}
	if NewRefreshTokenMAC(EphemeralKeys()).Tag(msg) == tag {
		t.Error("another key gives the same tag")
	}
}

func TestRefreshTokenMACVerify(t *testing.T) {
	m := NewRefreshTokenMAC(testKeys(t))
	msg := bytes.Repeat([]byte{7}, 52)
	tag := m.Tag(msg)
	var random [16]byte
	_, _ = rand.Read(random[:])
	lastBit := tag
	lastBit[15] ^= 1

	if !m.Verify(msg, tag) {
		t.Error("Verify() of the message's own tag = false")
	}
	for name, forged := range map[string][16]byte{
		"a random tag":                      random,
		"the tag with its last bit flipped": lastBit,
		"the zero tag":                      {},
	} {
		if m.Verify(msg, forged) {
			t.Errorf("Verify() of %s = true", name)
		}
	}
	// Session id, generation and secret: a change anywhere voids the tag.
	for i := range msg {
		changed := bytes.Clone(msg)
		changed[i] ^= 1
		if m.Verify(changed, tag) {
			t.Errorf("Verify() with byte %d of the message changed = true", i)
		}
	}
	if NewRefreshTokenMAC(EphemeralKeys()).Verify(msg, tag) {
		t.Error("Verify() under another key = true")
	}
}

// A known answer, computed apart from this package (RFC 5869 and RFC 2104 by
// hand in Python) from testKeyPEM's seed: the MAC key is HKDF-SHA256 with no
// salt and the info "nervewiki refresh-token mac v1". A change of the info,
// the hash or the salt changes every tag and fails here.
func TestRefreshTokenMACKnownAnswer(t *testing.T) {
	const want = "d07ba0b6322db3c851ee0ac33c158250"
	tag := NewRefreshTokenMAC(testKeys(t)).Tag(bytes.Repeat([]byte{7}, 52))

	if got := hex.EncodeToString(tag[:]); got != want {
		t.Errorf("Tag() = %s, want %s", got, want)
	}
}
