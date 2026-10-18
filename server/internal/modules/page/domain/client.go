package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Client is where a write came from (v0.1 design 3.8): the adapter knows,
// the use case records it (M4 design 4).
type Client string

// The clients of M4; M9 adds mcp:<name> (MCPClient).
const (
	ClientWeb Client = "web" // a sign-in session's access token
	ClientAPI Client = "api" // a personal access token
	ClientCLI Client = "cli"
)

// mcpPrefix starts an agent's client; UnknownClientName is the name of one
// that told none (mcpserver.UnknownClient, which page's adapter/mcp holds
// it to).
const (
	mcpPrefix         = "mcp:"
	UnknownClientName = "unknown"
)

// Valid reports whether the changesets and edit_sessions tables take c
// (M9 design 4.2): one of the three, or mcp: and a name of 1 to
// shared.MaxClientNameChars characters of UTF-8, none of them unshowable
// (shared.Unshowable, the set the tables' checks spell out).
func (c Client) Valid() bool {
	switch c {
	case ClientWeb, ClientAPI, ClientCLI:
		return true
	}
	name, ok := strings.CutPrefix(string(c), mcpPrefix)
	n := utf8.RuneCountInString(name)
	return ok && utf8.ValidString(name) && n >= 1 && n <= shared.MaxClientNameChars && !strings.ContainsFunc(name, shared.Unshowable)
}

// IsMCP reports whether c is an agent's: mcp: and its name.
func (c Client) IsMCP() bool { return strings.HasPrefix(string(c), mcpPrefix) }

// MCPClient is the client of the agent named name (M9 design 4.5): mcp:
// and the name cleaned again, unknown when nothing is left. A name a token
// recorded, cleaned before, stays itself: CleanClientName is idempotent.
func MCPClient(name string) Client {
	if name = shared.CleanClientName(name); name == "" {
		name = UnknownClientName
	}
	return Client(mcpPrefix + name)
}
