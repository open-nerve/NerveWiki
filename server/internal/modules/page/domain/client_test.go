package domain_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The tables take the three clients, and mcp: with a name of 1 to 128
// characters of UTF-8 none of which is unshowable: a control, a line or
// paragraph separator, a bidirectional control. Other format characters
// are a name's (M9/P2 design 3.3).
func TestAClientIsValidAsTheTablesTakeIt(t *testing.T) {
	for _, ok := range []string{"web", "api", "cli", "mcp:a", "mcp:Claude Code", "mcp:" + strings.Repeat("名", 128),
		"mcp:a\u200bb", "mcp:a\u00adb", "mcp:\ufeffa", "mcp:a\u200db", "mcp:\u3000a", "mcp:\u00a0"} {
		if !domain.Client(ok).Valid() {
			t.Errorf("Client(%q).Valid() = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "WEB", "mobile", "mcp:", "mcp:a\nb", "mcp:a\x00", "mcp:\x7f", "mcp:\u0085", "mcp:\u009f",
		"mcp:a\u2028b", "mcp:a\u2029", "mcp:\u202ea", "mcp:\u061c", "mcp:\u200e", "mcp:\u200f", "mcp:\u2066", "mcp:\u2069",
		"mcp:" + strings.Repeat("名", 129), "mcp:\xff", "MCP:a"} {
		if domain.Client(bad).Valid() {
			t.Errorf("Client(%q).Valid() = true, want false", bad)
		}
	}
}

// IsMCP is an agent's client, mcp: and its name.
func TestIsMCPIsAnAgentsClient(t *testing.T) {
	for c, want := range map[domain.Client]bool{"mcp:a": true, "mcp:": true, domain.ClientWeb: false, domain.ClientAPI: false,
		domain.ClientCLI: false, "MCP:a": false} {
		if c.IsMCP() != want {
			t.Errorf("Client(%q).IsMCP() = %v, want %v", c, !want, want)
		}
	}
}

// MCPClient is mcp: and the name cleaned again, unknown when nothing is
// left.
func TestMCPClientIsTheCleanedName(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Claude Code", "mcp:Claude Code"},
		{"  Claude Code\t", "mcp:Claude Code"},
		{"", "mcp:unknown"},
		{"unknown", "mcp:unknown"},
		{"\u202eabc", "mcp:abc"},
		{strings.Repeat("n", 200), "mcp:" + strings.Repeat("n", 128)},
		{"co\xffdex", "mcp:codex"},
		{"\u2028\x00", "mcp:unknown"},
	} {
		if got := domain.MCPClient(tt.in); string(got) != tt.want {
			t.Errorf("MCPClient(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if domain.UnknownClientName != "unknown" {
		t.Errorf("UnknownClientName = %q", domain.UnknownClientName)
	}
}

// Every name, cleaned through MCPClient, is a client the tables take: a
// unit never refuses an agent's write for its name (page/app/unit.go's
// Valid check). The names are shared's S3 property's (client_name_test.go).
func TestEveryMCPClientIsValid(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	ranges := [][2]rune{{0, 0x7f}, {0x80, 0x24f}, {0x300, 0x36f}, {0x200b, 0x206f}, {0x3000, 0x30ff}, {0x4e00, 0x4e3f},
		{0x1100, 0x11ff}, {0xac00, 0xac3f}, {0xfe00, 0xfeff}, {0x1f600, 0x1f64f}}
	for i := range 100000 {
		var b strings.Builder
		for range r.IntN(200) {
			rg := ranges[r.IntN(len(ranges))]
			b.WriteRune(rg[0] + rune(r.IntN(int(rg[1]-rg[0]+1))))
		}
		if r.IntN(10) == 0 {
			b.WriteByte(0xff)
		}
		if c := domain.MCPClient(b.String()); !c.Valid() || !c.IsMCP() {
			t.Fatalf("#%d: MCPClient(%q) = %q, which the tables refuse", i, b.String(), c)
		}
	}
}
