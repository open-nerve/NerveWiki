package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// A sign-in's fifth event stream supersedes its oldest, through the whole
// program (M12 design 4.6.5; P3 design 3.9.3): alice signs in again, a
// session of its own, and opens five streams with its access token, each
// past its hello; the first ends with reset superseded, the other four,
// and a stream of her personal access token opened before them, each get
// the next write. The streams of settle and of the team's own sign-in
// are another credential's: one still closing does not count against
// this one.
func TestASignInsFifthStreamSupersedesItsOldest(t *testing.T) {
	tm, nb, marker, _ := eventsTeam(t)
	pat := openStream(t, tm.base, "alice's token", createToken(t, tm.contract, tm.base, tm.tokens["alice"]))
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/auth/login", "", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`)
	var signedIn authTokens
	if err := json.Unmarshal([]byte(body), &signedIn); status != http.StatusOK || err != nil || signedIn.AccessToken == "" {
		t.Fatalf("alice's sign-in = %d %s", status, body)
	}
	var streams []*eventStream
	for i := range 5 {
		streams = append(streams, openStream(t, tm.base, fmt.Sprintf("alice's stream %d", i+1), signedIn.AccessToken))
	}
	streams[0].reset(t, "superseded")
	tm.quiet(t, nb, marker, append(streams[1:], pat)...)
}
