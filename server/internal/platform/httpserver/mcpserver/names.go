package mcpserver

import (
	"container/list"
	"context"
	"log/slog"
	"sync"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// maxNamedTokens is how many tokens' client names names remembers.
const maxNamedTokens = 1024

// names is the client of each call (M9 design 4.5): the name the request
// tells; else the name its token last told, from an LRU of 1,024 tokens
// (a miss reads it; a read that answers, "" too, is remembered, a failed
// one is not); else UnknownClient. A told name that differs from the
// remembered one is recorded, best effort, and remembered only when a row
// was written: a failure is a warning with the token's id. Without a port
// every call's client is UnknownClient.
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
// A record skipped (0 rows: the name already there, a row another
// transaction holds, the token gone) or failed is not remembered, so that
// the next request that tells the name records it again; a read that
// failed is not, so that the next one reads again: the token is never
// unknown for the process's life for one failure.
func (n *names) of(ctx context.Context, tokenID uuid.UUID, told string) string {
	if n.port == nil {
		return UnknownClient
	}
	if name := n.port.Clean(told); name != "" {
		if known, ok := n.remembered(tokenID); ok && known == name {
			return name
		}
		written, err := n.port.Record(ctx, tokenID, name)
		switch {
		case err != nil:
			n.logger.LogAttrs(ctx, slog.LevelWarn, "MCP client name not recorded", slog.String("token_id", tokenID.String()),
				slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", err))
		case written:
			n.remember(tokenID, name)
		}
		return name
	}
	known, ok := n.remembered(tokenID)
	if !ok {
		recorded, err := n.port.Recorded(ctx, tokenID)
		if err != nil {
			n.logger.LogAttrs(ctx, slog.LevelWarn, "MCP client name not read", slog.String("token_id", tokenID.String()),
				slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", err))
			return UnknownClient
		}
		n.remember(tokenID, recorded)
		known = recorded
	}
	if known == "" {
		return UnknownClient
	}
	return known
}

func (n *names) remembered(tokenID uuid.UUID) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lru.get(tokenID)
}

func (n *names) remember(tokenID uuid.UUID, name string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lru.put(tokenID, name)
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

type namedToken struct {
	token uuid.UUID
	name  string
}

func newTokenLRU(maxTokens int) *tokenLRU {
	return &tokenLRU{max: maxTokens, order: list.New(), entries: map[uuid.UUID]*list.Element{}}
}

func (l *tokenLRU) get(token uuid.UUID) (string, bool) {
	e, ok := l.entries[token]
	if !ok {
		return "", false
	}
	l.order.MoveToFront(e)
	return e.Value.(*namedToken).name, true
}

func (l *tokenLRU) put(token uuid.UUID, name string) {
	if e, ok := l.entries[token]; ok {
		e.Value.(*namedToken).name = name
		l.order.MoveToFront(e)
		return
	}
	l.entries[token] = l.order.PushFront(&namedToken{token: token, name: name})
	if l.order.Len() > l.max {
		last := l.order.Back()
		l.order.Remove(last)
		delete(l.entries, last.Value.(*namedToken).token)
	}
}
