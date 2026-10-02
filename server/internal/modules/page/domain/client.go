package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Client is where a write came from (v0.1 design 3.8): the adapter knows,
// the use case records it (M4 design 4).
type Client string

// The clients of M4; M9 adds mcp:<name>.
const (
	ClientWeb Client = "web" // a sign-in session's access token
	ClientAPI Client = "api" // a personal access token
	ClientCLI Client = "cli"
)

// Valid reports whether the changesets table takes c: one of the three, or
// mcp: and a name of 1–128 characters with no control character.
func (c Client) Valid() bool {
	switch c {
	case ClientWeb, ClientAPI, ClientCLI:
		return true
	}
	name, ok := strings.CutPrefix(string(c), "mcp:")
	n := utf8.RuneCountInString(name)
	return ok && utf8.ValidString(name) && n >= 1 && n <= 128 && !strings.ContainsFunc(name, unicode.IsControl)
}
