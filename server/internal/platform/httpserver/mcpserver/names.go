package mcpserver

import (
	"container/list"
	"context"
	"log/slog"
	"strings"
	"sync"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// maxNamedTokens is how many tokens' client names names remembers.
const maxNamedTokens = 1024

// names is the client of each call (M9 design 4.5): the name the request
// tells; else the name its token last told this process; else, for a
// token the process has not heard, the name its row holds, read once (""
// too); else UnknownClient. It remembers 1,024 tokens (an LRU).
//
// A told name is remembered at once, so that the token's next calls are
// its, and recorded, best effort: it is known to be on the row once a
// record writes it, or once a read after a record that wrote nothing (the
// name already there, a row another transaction holds, the token gone)
// finds it there. Until then each call of the token records it again, a
// failure a warning with the token's id. A told name the row is known to
// hold is no write and no read. Without a port every call's client is
// UnknownClient.
type names struct {
	port   ClientNames
	logger *slog.Logger
	mu     sync.Mutex
	lru    *tokenLRU
}

func newNames(port ClientNames, logger *slog.Logger) *names {
	return &names{port: port, logger: logger, lru: newTokenLRU(maxNamedTokens)}
}

// of is the client of a request of tokenID that tells told, "" for none.
// A record or a read that failed leaves the name remembered as it was:
// the told name, still to record; nothing, to read again, so that the
// token is never unknown for the process's life for one failure.
func (n *names) of(ctx context.Context, tokenID uuid.UUID, told string) string {
	if n.port == nil {
		return UnknownClient
	}
	if name := n.port.Clean(told); name != "" {
		if known, ok := n.remembered(tokenID); ok && known.name == name && known.onRow {
			return name
		}
		n.remember(tokenID, name)
		n.record(ctx, tokenID, name)
		return name
	}
	known, ok := n.remembered(tokenID)
	switch {
	case !ok:
		recorded, err := n.port.Recorded(ctx, tokenID)
		if err != nil {
			n.warn(ctx, "MCP client name not read", tokenID, err)
			return UnknownClient
		}
		known = n.learn(tokenID, recorded)
	case !known.onRow:
		n.record(ctx, tokenID, known.name)
	}
	if known.name == "" {
		return UnknownClient
	}
	return known.name
}

// record records name, told for tokenID, and remembers it on the row when
// the record wrote it, or when a read after a record that wrote nothing
// finds it there.
func (n *names) record(ctx context.Context, tokenID uuid.UUID, name string) {
	written, err := n.port.Record(ctx, tokenID, name)
	if err != nil {
		n.warn(ctx, "MCP client name not recorded", tokenID, err)
		return
	}
	if !written {
		recorded, err := n.port.Recorded(ctx, tokenID)
		switch {
		case err != nil:
			n.warn(ctx, "MCP client name not read", tokenID, err)
			return
		case recorded != name:
			return
		}
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lru.onRow(tokenID, name)
}

func (n *names) warn(ctx context.Context, msg string, tokenID uuid.UUID, err error) {
	n.logger.LogAttrs(ctx, slog.LevelWarn, msg, slog.String("token_id", tokenID.String()),
		slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", err))
}

func (n *names) remembered(tokenID uuid.UUID) (namedToken, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lru.get(tokenID)
}

// remember remembers name, told for tokenID, as not yet known to be on
// the row. A copy: the cache holds the name's bytes, not the request's.
func (n *names) remember(tokenID uuid.UUID, name string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lru.put(namedToken{token: tokenID, name: strings.Clone(name)})
}

// learn remembers recorded, the name tokenID's row holds, unless a told
// name came first: what is remembered then.
func (n *names) learn(tokenID uuid.UUID, recorded string) namedToken {
	n.mu.Lock()
	defer n.mu.Unlock()
	if known, ok := n.lru.get(tokenID); ok {
		return known
	}
	known := namedToken{token: tokenID, name: recorded, onRow: true}
	n.lru.put(known)
	return known
}

// toldName is the client name a request tells: an initialize's
// clientInfo.name, or any request's _meta clientInfo (ClientInfo()); ""
// for none, as a call after a stateless initialize tells none. Only the
// name, never the title (M9 design 11). An initialize reaches the server
// as a ServerRequest of InitializeParams: mcp.InitializeRequest is the
// client's.
func toldName(req mcp.Request) string {
	if req == nil {
		return ""
	}
	if p, ok := req.GetParams().(*mcp.InitializeParams); ok && p != nil && p.ClientInfo != nil && p.ClientInfo.Name != "" {
		return p.ClientInfo.Name
	}
	if r, ok := req.(interface{ ClientInfo() *mcp.Implementation }); ok {
		if info := r.ClientInfo(); info != nil {
			return info.Name
		}
	}
	return ""
}

type clientKey struct{}

// withClient is ctx with the call's client, which the tools' wrapper and
// the logs read.
func withClient(ctx context.Context, client string) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// clientOf is the client the handshake put in ctx: UnknownClient without
// one.
func clientOf(ctx context.Context) string {
	if client, ok := ctx.Value(clientKey{}).(string); ok && client != "" {
		return client
	}
	return UnknownClient
}

// tokenLRU holds the client names of the tokens last used, at most max:
// the least recently used goes first. Its user holds the lock.
type tokenLRU struct {
	max     int
	order   *list.List // of *namedToken, the most recent first
	entries map[uuid.UUID]*list.Element
}

// namedToken is a token's client name, and whether its row is known to
// hold it: a told name is not until it is recorded.
type namedToken struct {
	token uuid.UUID
	name  string
	onRow bool
}

func newTokenLRU(maxTokens int) *tokenLRU {
	return &tokenLRU{max: maxTokens, order: list.New(), entries: map[uuid.UUID]*list.Element{}}
}

func (l *tokenLRU) get(token uuid.UUID) (namedToken, bool) {
	e, ok := l.entries[token]
	if !ok {
		return namedToken{}, false
	}
	l.order.MoveToFront(e)
	return *e.Value.(*namedToken), true
}

func (l *tokenLRU) put(named namedToken) {
	if e, ok := l.entries[named.token]; ok {
		*e.Value.(*namedToken) = named
		l.order.MoveToFront(e)
		return
	}
	l.entries[named.token] = l.order.PushFront(&named)
	if l.order.Len() > l.max {
		last := l.order.Back()
		l.order.Remove(last)
		delete(l.entries, last.Value.(*namedToken).token)
	}
}

// onRow marks token's name on the row, when it is still name: a name told
// since is not.
func (l *tokenLRU) onRow(token uuid.UUID, name string) {
	if e, ok := l.entries[token]; ok && e.Value.(*namedToken).name == name {
		e.Value.(*namedToken).onRow = true
	}
}
