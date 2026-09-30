package httpserver

import (
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
)

// clientIPs tells who the client of a request is (M1/P1 design 3.5).
type clientIPs struct {
	logger  *slog.Logger
	trusted []netip.Prefix // server.trusted_proxies
	// Each misconfiguration warning is logged once per process: bootstrap
	// builds one API. untrusted: X-Forwarded-For while no proxy is trusted;
	// missing: a trusted proxy that forwards no X-Forwarded-For; malformed:
	// a trusted proxy that forwards an entry that is not a bare address.
	warnedUntrusted sync.Once
	warnedMissing   sync.Once
	warnedMalformed sync.Once
}

// of returns the client of r: the connection's peer, unless the peer is a
// trusted proxy. Then it is the first address of X-Forwarded-For, from the
// right, that is not a trusted proxy: the proxies append the address they
// received from, so what lies left of the first untrusted one is the
// client's to write. When every address is a trusted proxy the leftmost is
// the client; an entry that is not a bare address (one with a port, a host
// name) ends the walk at the trusted hop that forwarded it. Addresses lose
// their zone, and an IPv4-mapped IPv6 address is its IPv4 address. The zero
// Addr when the peer address does not parse. Forwarded and X-Real-IP are
// not read.
func (c *clientIPs) of(r *http.Request) netip.Addr {
	client := normalize(peerAddr(r.RemoteAddr))
	forwarded := r.Header.Values("X-Forwarded-For")
	trusted := c.isTrusted(client)
	switch {
	case len(forwarded) == 0 && trusted:
		c.warnedMissing.Do(func() {
			c.logger.WarnContext(r.Context(), "a trusted proxy forwarded a request without X-Forwarded-For: "+
				"the clients behind it count as the proxy; configure it to set the header",
				slog.String("peer", client.String()))
		})
		return client
	case len(forwarded) == 0:
		return client
	case !trusted && len(c.trusted) > 0:
		// Proxies are configured and this peer is none of them: a client
		// that writes the header itself. Warning would let any client name
		// itself in the log, and use up the warning a real misconfiguration
		// needs.
		return client
	case !trusted:
		c.warnedUntrusted.Do(func() {
			c.logger.WarnContext(r.Context(), "ignored X-Forwarded-For from a peer that is not a trusted proxy: "+
				"behind a reverse proxy, add its address to server.trusted_proxies, or every client counts as the proxy",
				slog.String("peer", client.String()))
		})
		return client
	}
	hops := strings.Split(strings.Join(forwarded, ","), ",")
	for _, hop := range slices.Backward(hops) {
		addr, err := netip.ParseAddr(strings.TrimSpace(hop))
		if err != nil {
			c.warnedMalformed.Do(func() {
				c.logger.WarnContext(r.Context(), "a trusted proxy forwarded an X-Forwarded-For entry that is not a bare IP address: "+
					"the clients behind it count as the proxy; the proxies must write bare addresses",
					slog.String("peer", client.String()))
			})
			break
		}
		client = normalize(addr)
		if !c.isTrusted(client) {
			break
		}
	}
	return client
}

func (c *clientIPs) isTrusted(ip netip.Addr) bool {
	return ip.IsValid() && slices.ContainsFunc(c.trusted, func(p netip.Prefix) bool { return p.Contains(ip) })
}

// peerAddr is the address of RemoteAddr without its port; the zero Addr
// when it does not parse.
func peerAddr(remote string) netip.Addr {
	ap, err := netip.ParseAddrPort(remote)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr()
}

func normalize(ip netip.Addr) netip.Addr {
	return ip.Unmap().WithZone("")
}
