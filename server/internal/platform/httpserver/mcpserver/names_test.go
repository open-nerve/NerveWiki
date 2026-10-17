package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"
)

// fakeNames is ClientNames in memory: Clean trims and drops "!", Recorded
// reads the names recorded, Record writes one that differs; each counts
// its calls. skip has Record write nothing, readErr and writeErr fail them.
type fakeNames struct {
	mu                sync.Mutex
	recorded          map[uuid.UUID]string
	reads, writes     int
	skip              bool
	readErr, writeErr error
}

func newFakeNames() *fakeNames { return &fakeNames{recorded: map[uuid.UUID]string{}} }

func (f *fakeNames) Clean(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "!", ""))
}

func (f *fakeNames) Recorded(_ context.Context, tokenID uuid.UUID) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.readErr != nil {
		return "", f.readErr
	}
	return f.recorded[tokenID], nil
}

func (f *fakeNames) Record(_ context.Context, tokenID uuid.UUID, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	switch {
	case f.writeErr != nil:
		return false, f.writeErr
	case f.skip || f.recorded[tokenID] == name:
		return false, nil
	}
	f.recorded[tokenID] = name
	return true, nil
}

// counts are the port's reads and writes so far.
func (f *fakeNames) counts() (reads, writes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads, f.writes
}

// of is the name recorded on token.
func (f *fakeNames) of(token uuid.UUID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recorded[token]
}

// set has the port answer as change makes it.
func (f *fakeNames) set(change func(f *fakeNames)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

// handshakeOf is an initialize told the client name name.
func handshakeOf(name string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":` + quoted(name) + `,"version":"1"}}}`
}

// echoTold is a tools/call of echo whose _meta tells the client name name,
// "" for none.
func echoTold(name string) string {
	meta := ""
	if name != "" {
		meta = `,"_meta":{"io.modelcontextprotocol/clientInfo":{"name":` + quoted(name) + `,"version":"1"}}`
	}
	return `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"echo","arguments":{"notebook":"` + visible.String() + `"}` +
		meta + `}}`
}

func quoted(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// withNames is a server whose names are port's.
func withNames(t *testing.T, port ClientNames) *served {
	t.Helper()
	return serveMCP(t, setup{options: func(o *Options) { o.Names = port }})
}

// clientOfEcho is the client echo's call was told, its _meta telling
// told.
func (s *served) clientOfEcho(t *testing.T, told string) string {
	t.Helper()
	r := result(t, s.post(t, context.Background(), Pattern, patToken, echoTold(told), v2025()))
	sc, _ := r["structuredContent"].(map[string]any)
	client, _ := sc["client"].(string)
	return client
}

// A call's client is the name the request tells, an initialize's
// clientInfo or any request's _meta, cleaned; else the name its token
// last told; else unknown. Without the names, every call's client is
// unknown (M9/P1 design 3.16.4).
func TestTheNamesTakeTheirSources(t *testing.T) {
	port := newFakeNames()
	s := withNames(t, port)
	if got := s.clientOfEcho(t, ""); got != UnknownClient {
		t.Errorf("a token that never told a name: %q, want unknown", got)
	}
	result(t, s.post(t, context.Background(), Pattern, patToken, handshakeOf(" agent-a! "), nil))
	if port.of(aliceTok) != "agent-a" {
		t.Errorf("the handshake recorded %q, want the cleaned agent-a", port.of(aliceTok))
	}
	if got := s.clientOfEcho(t, ""); got != "agent-a" {
		t.Errorf("a call after the handshake, telling none: %q, want agent-a", got)
	}
	if got := s.clientOfEcho(t, "agent-b"); got != "agent-b" || port.of(aliceTok) != "agent-b" {
		t.Errorf("a call telling agent-b in its _meta: %q, recorded %q", got, port.of(aliceTok))
	}
	if got := s.clientOfEcho(t, "!!"); got != "agent-b" {
		t.Errorf("a name empty once cleaned: %q, want the token's agent-b", got)
	}
	time.Sleep(50 * time.Millisecond)
	if line := s.logs.find(t, "MCP initialize"); line == nil || line["client"] != "agent-a" {
		t.Errorf("the handshake's line = %v, want its client agent-a", line)
	}
	if line := callLine(t, s.logs, "echo"); line == nil || line["client"] != UnknownClient {
		t.Errorf("the first call's line = %v, want its client unknown", line)
	}
	bare := serveMCP(t, setup{})
	result(t, bare.post(t, context.Background(), Pattern, patToken, handshakeOf("agent-a"), nil))
	if got := bare.clientOfEcho(t, "agent-b"); got != UnknownClient {
		t.Errorf("without the names: %q, want unknown", got)
	}
}

// A name is recorded only when it differs from the one remembered: one
// client's calls write it once; another's writes it again; a name empty
// once cleaned writes nothing.
func TestTheNamesRecordOnlyWhatDiffers(t *testing.T) {
	port := newFakeNames()
	s := withNames(t, port)
	for range 3 {
		s.clientOfEcho(t, "agent-a")
	}
	if _, writes := port.counts(); writes != 1 {
		t.Errorf("three calls of one name wrote %d times, want once", writes)
	}
	s.clientOfEcho(t, "agent-b")
	s.clientOfEcho(t, "!")
	if _, writes := port.counts(); writes != 2 {
		t.Errorf("another name and an empty one wrote %d times in all, want twice", writes)
	}
}

// A token that never told a name is read once: the empty answer is
// remembered.
func TestTheNamesRememberAnEmptyName(t *testing.T) {
	port := newFakeNames()
	s := withNames(t, port)
	s.clientOfEcho(t, "")
	s.clientOfEcho(t, "")
	if reads, _ := port.counts(); reads != 1 {
		t.Errorf("two calls of a token that told no name read it %d times, want once", reads)
	}
}

// A name whose record wrote nothing (a row skipped) is not remembered:
// the next call that tells it records it again; the call goes on with it.
func TestTheNamesRetryASkippedWrite(t *testing.T) {
	port := newFakeNames()
	port.skip = true
	s := withNames(t, port)
	if got := s.clientOfEcho(t, "agent-a"); got != "agent-a" {
		t.Errorf("a call whose record was skipped: %q, want agent-a", got)
	}
	s.clientOfEcho(t, "agent-a")
	if _, writes := port.counts(); writes != 2 {
		t.Errorf("a skipped record was tried %d times in two calls, want twice", writes)
	}
	port.set(func(f *fakeNames) { f.skip = false })
	s.clientOfEcho(t, "agent-a")
	s.clientOfEcho(t, "agent-a")
	if _, writes := port.counts(); writes != 3 {
		t.Errorf("once written, %d records in all, want 3", writes)
	}
}

// A read that failed is not remembered: the next call reads again; the
// call's client is unknown meanwhile, the failure a warning. A write that
// failed is a warning too, the call's client the name told.
func TestTheNamesRetryAFailedRead(t *testing.T) {
	port := newFakeNames()
	port.readErr = errors.New("the database is gone")
	s := withNames(t, port)
	if got := s.clientOfEcho(t, ""); got != UnknownClient {
		t.Errorf("a call whose read failed: %q, want unknown", got)
	}
	port.set(func(f *fakeNames) { f.readErr, f.recorded[aliceTok] = nil, "agent-a" })
	if got := s.clientOfEcho(t, ""); got != "agent-a" {
		t.Errorf("the next call: %q, want agent-a, read again", got)
	}
	if reads, _ := port.counts(); reads != 2 {
		t.Errorf("%d reads, want 2", reads)
	}
	port.set(func(f *fakeNames) { f.writeErr = errors.New("the database is gone") })
	if got := s.clientOfEcho(t, "agent-b"); got != "agent-b" {
		t.Errorf("a call whose record failed: %q, want agent-b", got)
	}
	time.Sleep(50 * time.Millisecond)
	for _, msg := range []string{"MCP client name not read", "MCP client name not recorded"} {
		if line := s.logs.find(t, msg); line == nil || line["level"] != "WARN" || line["token_id"] != aliceTok.String() {
			t.Errorf("%s: %v", msg, line)
		}
	}
}

// The names remember 1,024 tokens: the 1,025th puts out the least
// recently used, whose next call reads it again.
func TestTheNamesForgetTheLeastRecentlyUsed(t *testing.T) {
	port := newFakeNames()
	n := newNames(port, discard())
	ctx := context.Background()
	tokens := make([]uuid.UUID, maxNamedTokens+1)
	for i := range tokens {
		tokens[i] = uuid.NewV7()
		n.of(ctx, tokens[i], "")
	}
	if reads, _ := port.counts(); reads != len(tokens) {
		t.Fatalf("%d reads, want one a token", reads)
	}
	for _, tok := range tokens[1:] {
		n.of(ctx, tok, "")
	}
	if reads, _ := port.counts(); reads != len(tokens) {
		t.Errorf("the 1,024 last used read again: %d reads", reads)
	}
	n.of(ctx, tokens[0], "")
	if reads, _ := port.counts(); reads != len(tokens)+1 {
		t.Errorf("the first token, put out, was not read again: %d reads", reads)
	}
}
