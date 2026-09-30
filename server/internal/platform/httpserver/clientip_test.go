package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func clientsTrusting(logger *slog.Logger, cidrs ...string) *clientIPs {
	c := &clientIPs{logger: logger}
	for _, s := range cidrs {
		c.trusted = append(c.trusted, netip.MustParsePrefix(s))
	}
	return c
}

func requestFrom(remote string, forwarded ...string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v0/instance", nil)
	r.RemoteAddr = remote
	for _, f := range forwarded {
		r.Header.Add("X-Forwarded-For", f)
	}
	return r
}

// The client is the peer, unless the peer is a trusted proxy: then it is the
// first address of X-Forwarded-For, from the right, that is not a trusted
// proxy (M1/P1 design 3.5).
func TestClientIP(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		forwarded []string
		want      string
	}{
		{"untrusted peer: its forwarding is ignored", "203.0.113.7:5555", []string{"198.51.100.1"}, "203.0.113.7"},
		{"trusted peer without forwarding", "10.0.0.1:5555", nil, "10.0.0.1"},
		{"trusted peer forwards the client", "10.0.0.1:5555", []string{"198.51.100.1"}, "198.51.100.1"},
		{"trusted hops are skipped", "10.0.0.1:5555", []string{"198.51.100.1, 10.0.0.2"}, "198.51.100.1"},
		{"what the client wrote on the left is not believed", "10.0.0.1:5555", []string{"6.6.6.6,198.51.100.1 , 10.0.0.2"}, "198.51.100.1"},
		{"several header lines are one list", "10.0.0.1:5555", []string{"6.6.6.6", "198.51.100.1, 10.0.0.2"}, "198.51.100.1"},
		{"every hop trusted: the leftmost", "10.0.0.1:5555", []string{"10.0.0.3, 10.0.0.2"}, "10.0.0.3"},
		{"a malformed entry ends at the hop that forwarded it", "10.0.0.1:5555", []string{"198.51.100.1, bogus, 10.0.0.2"}, "10.0.0.2"},
		{"an entry with a port ends the walk too", "10.0.0.1:5555", []string{"198.51.100.1:40000"}, "10.0.0.1"},
		{"an empty entry ends the walk too", "10.0.0.1:5555", []string{"198.51.100.1,"}, "10.0.0.1"},
		{"IPv6 proxy and client", "[fd00::1]:443", []string{"2001:db8::5"}, "2001:db8::5"},
		{"a mapped client is IPv4", "10.0.0.1:5555", []string{"::ffff:198.51.100.1"}, "198.51.100.1"},
		{"a mapped peer is IPv4, and trusted", "[::ffff:10.0.0.1]:80", []string{"198.51.100.1"}, "198.51.100.1"},
		{"the zone is dropped", "[fe80::1%en0]:80", nil, "fe80::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := clientsTrusting(slog.New(slog.DiscardHandler), "10.0.0.0/8", "fd00::/8")

			if got := c.of(requestFrom(tt.remote, tt.forwarded...)); got != netip.MustParseAddr(tt.want) {
				t.Errorf("client = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestClientIPOfAnUnparsablePeerIsZero(t *testing.T) {
	c := clientsTrusting(slog.New(slog.DiscardHandler))
	if got := c.of(requestFrom("@unix-socket")); got.IsValid() {
		t.Errorf("client = %v, want the zero Addr", got)
	}
}

// warnings returns the warnings whose message starts with prefix.
func warnings(entries []map[string]any, prefix string) []map[string]any {
	var out []map[string]any
	for _, e := range entries {
		if msg, _ := e["msg"].(string); e["level"] == "WARN" && strings.HasPrefix(msg, prefix) {
			out = append(out, e)
		}
	}
	return out
}

// X-Forwarded-For from a peer that is not trusted most likely means a proxy
// missing from server.trusted_proxies: warned once per process, never again.
// A direct request without the header is no such sign, and is not warned.
func TestUntrustedForwardingIsWarnedOnce(t *testing.T) {
	logger, logs := captureLogs(t)
	c := clientsTrusting(logger, "10.0.0.0/8")

	c.of(requestFrom("203.0.113.6:5555"))
	c.of(requestFrom("203.0.113.7:5555", "198.51.100.1"))
	c.of(requestFrom("203.0.113.8:5555", "198.51.100.2"))
	c.of(requestFrom("10.0.0.1:5555", "198.51.100.3"))

	if all := warnings(logs(), ""); len(all) != 1 || all[0]["peer"] != "203.0.113.7" {
		t.Errorf("warnings = %v, want one, naming the peer 203.0.113.7", all)
	}
}

// A trusted proxy that forwards no X-Forwarded-For makes every client behind
// it count as the proxy: warned once per process.
func TestTrustedProxyWithoutForwardingIsWarnedOnce(t *testing.T) {
	logger, logs := captureLogs(t)
	c := clientsTrusting(logger, "10.0.0.0/8")

	c.of(requestFrom("10.0.0.1:5555", "198.51.100.1"))
	c.of(requestFrom("10.0.0.2:5555"))
	c.of(requestFrom("10.0.0.3:5555"))

	if all := warnings(logs(), ""); len(all) != 1 || all[0]["peer"] != "10.0.0.2" ||
		!strings.HasPrefix(all[0]["msg"].(string), "a trusted proxy forwarded a request without X-Forwarded-For") {
		t.Errorf("warnings = %v, want one, naming the proxy 10.0.0.2", all)
	}
}

// A trusted proxy that forwards an entry that is not a bare address (with a
// port, a host name) ends the walk at itself, so its clients count as the
// proxy: warned once per process, naming only the proxy, never what the
// header held. A well-formed chain is no such sign, and is not warned.
func TestMalformedForwardingIsWarnedOnce(t *testing.T) {
	logger, logs := captureLogs(t)
	c := clientsTrusting(logger, "10.0.0.0/8")

	c.of(requestFrom("10.0.0.1:5555", "198.51.100.1, 10.0.0.2"))
	c.of(requestFrom("10.0.0.1:5555", "198.51.100.2:40000, 10.0.0.2"))
	c.of(requestFrom("10.0.0.3:5555", "client.example"))

	entries := logs()
	if all := warnings(entries, ""); len(all) != 1 || all[0]["peer"] != "10.0.0.2" {
		t.Errorf("warnings = %v, want one, naming the proxy 10.0.0.2", all)
	}
	for _, forwarded := range []string{"198.51.100", "40000", "client.example"} {
		if text := fmt.Sprint(entries); strings.Contains(text, forwarded) {
			t.Errorf("logs hold %q from the header: %s", forwarded, text)
		}
	}
}
