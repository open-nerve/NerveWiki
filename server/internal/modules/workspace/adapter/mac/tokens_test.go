package macadapter_test

import (
	"bytes"
	"strings"
	"testing"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/mac"
)

func TestTokens(t *testing.T) {
	tokens := macadapter.New(bytes.Repeat([]byte{7}, 32))
	id := uuid.MustParse("0199a2b4-0000-7000-8000-000000000001")
	token := tokens.Token(id)

	// nwk_inv_ and 16 bytes in base64url: 22 characters.
	if !strings.HasPrefix(token, "nwk_inv_") || len(token) != len("nwk_inv_")+22 || token != tokens.Token(id) {
		t.Errorf("Token() = %q, want nwk_inv_ and 22 characters, the same each time", token)
	}
	if !tokens.Valid(id, token) {
		t.Errorf("Valid(id, its token) = false")
	}
	other := uuid.MustParse("0199a2b4-0000-7000-8000-000000000002")
	tampered := token[:len(token)-1] + map[bool]string{true: "B", false: "A"}[strings.HasSuffix(token, "A")]
	for name, tt := range map[string]struct {
		tokens macadapter.Tokens
		id     uuid.UUID
		token  string
	}{
		"another invitation's": {tokens, other, token},
		"another key's":        {macadapter.New(bytes.Repeat([]byte{8}, 32)), id, token},
		"a character changed":  {tokens, id, tampered},
		"without its prefix":   {tokens, id, strings.TrimPrefix(token, "nwk_inv_")},
		"a PAT's prefix":       {tokens, id, "nwk_pat_" + strings.TrimPrefix(token, "nwk_inv_")},
		"not base64url":        {tokens, id, "nwk_inv_" + strings.Repeat("!", 22)},
		"cut short":            {tokens, id, token[:len(token)-4]},
		"empty":                {tokens, id, ""},
	} {
		if tt.tokens.Valid(tt.id, tt.token) {
			t.Errorf("Valid(%s) = true", name)
		}
	}
}

// The token is pinned: a change of the derivation would void every link
// sent. The answer was computed apart, with Python's hmac.
func TestTokenKnownAnswer(t *testing.T) {
	const want = "nwk_inv_wnrj8QdJTrbWb9LOlimByw"
	if got := macadapter.New(bytes.Repeat([]byte{7}, 32)).Token(uuid.MustParse("0199a2b4-0000-7000-8000-000000000001")); got != want {
		t.Errorf("Token() = %q, want %q", got, want)
	}
}
