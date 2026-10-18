package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
	"uuid"
)

// fakeNames is ClientNames in memory: Clean trims and drops "!", Recorded
// reads the names recorded, Record writes one that differs; each counts
// its calls. skip has Record write nothing, readErr and writeErr fail them;
// afterRead, once, runs between a read and its answer.
type fakeNames struct {
	mu                sync.Mutex
	recorded          map[uuid.UUID]string
	reads, writes     int
	skip              bool
	readErr, writeErr error
	afterRead         func()
}

func newFakeNames() *fakeNames { return &fakeNames{recorded: map[uuid.UUID]string{}} }

func (f *fakeNames) Clean(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "!", ""))
}

func (f *fakeNames) Recorded(_ context.Context, tokenID uuid.UUID) (string, error) {
	f.mu.Lock()
	f.reads++
	name, err, after := f.recorded[tokenID], f.readErr, f.afterRead
	f.afterRead = nil
	f.mu.Unlock()
	if after != nil {
		after()
	}
	if err != nil {
		return "", err
	}
	return name, nil
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

// A told name is the token's next calls' at once, those that tell none
// included, never the name its row held before, while its record writes
// nothing (another transaction holds the row) or fails: each call, telling
// it or none, records it again until a record writes it; then none records
// it again (M9/P1 design 3.16.4, S3 review B-M1).
func TestTheNamesKeepATellingUnrecorded(t *testing.T) {
	for _, fault := range []string{"skipped", "failed"} {
		port := newFakeNames()
		port.recorded[aliceTok] = "old-client"
		port.skip = fault == "skipped"
		if fault == "failed" {
			port.writeErr = errors.New("the database is gone")
		}
		s := withNames(t, port)
		result(t, s.post(t, context.Background(), Pattern, patToken, handshakeOf("new-client"), nil))
		for _, told := range []string{"", "new-client"} {
			if got := s.clientOfEcho(t, told); got != "new-client" {
				t.Errorf("%s: a call after the handshake telling %q: %q, want the new-client told", fault, told, got)
			}
		}
		if _, writes := port.counts(); writes != 3 || port.of(aliceTok) != "old-client" {
			t.Errorf("%s: %d records in a handshake and two calls, the row %q; want 3, old-client", fault, writes, port.of(aliceTok))
		}
		port.set(func(f *fakeNames) { f.skip, f.writeErr = false, nil })
		if got := s.clientOfEcho(t, ""); got != "new-client" || port.of(aliceTok) != "new-client" {
			t.Errorf("%s: once the record can write: the client %q, the row %q; want new-client", fault, got, port.of(aliceTok))
		}
		s.clientOfEcho(t, "")
		s.clientOfEcho(t, "new-client")
		if _, writes := port.counts(); writes != 4 {
			t.Errorf("%s: once written, %d records in all, want 4", fault, writes)
		}
	}
}

// A told name the row already holds, as after a restart, writes nothing:
// one read finds it there, and then neither a call that tells it nor one
// that tells none reads or writes (M9/P1 design 3.16.4, S3 review C-M2).
func TestTheNamesFindATellingOnTheRow(t *testing.T) {
	port := newFakeNames()
	port.recorded[aliceTok] = "agent-a"
	s := withNames(t, port)
	result(t, s.post(t, context.Background(), Pattern, patToken, handshakeOf("agent-a"), nil))
	if got := s.clientOfEcho(t, ""); got != "agent-a" {
		t.Errorf("a call after the handshake of the recorded name, telling none: %q, want agent-a", got)
	}
	s.clientOfEcho(t, "agent-a")
	s.clientOfEcho(t, "")
	if reads, writes := port.counts(); reads != 1 || writes != 1 {
		t.Errorf("%d reads and %d records, want one each: the handshake's record and the read that found it", reads, writes)
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
	port.set(func(f *fakeNames) { f.writeErr = nil })
	if got := s.clientOfEcho(t, "agent-b"); got != "agent-b" || port.of(aliceTok) != "agent-b" {
		t.Errorf("the next call telling agent-b: %q, the row %q; want agent-b recorded again (S3 review B-M2)", got, port.of(aliceTok))
	}
	if _, writes := port.counts(); writes != 2 {
		t.Errorf("%d records, want the failed one and its retry", writes)
	}
	time.Sleep(50 * time.Millisecond)
	for _, msg := range []string{"MCP client name not read", "MCP client name not recorded"} {
		if line := s.logs.find(t, msg); line == nil || line["level"] != "WARN" || line["token_id"] != aliceTok.String() {
			t.Errorf("%s: %v", msg, line)
		}
	}
}

// racing is fakeNames whose record of the name first writes, or skips,
// and then answers only when the test lets it: another told name's call
// runs in between.
type racing struct {
	*fakeNames
	first           string
	written, answer chan struct{}
}

func (r *racing) Record(ctx context.Context, tokenID uuid.UUID, name string) (bool, error) {
	w, err := r.fakeNames.Record(ctx, tokenID, name)
	if name == r.first {
		close(r.written)
		<-r.answer
	}
	return w, err
}

// Two clients telling names on one token at once: the name told last is
// the token's calls', the row's once recorded. One record answering after
// the other's never marks the other's name as on the row: when the later
// told name's record wrote nothing, it is recorded again (M9/P1 design
// 3.16.4, S3 review B-M1).
func TestTheNamesOfTwoClientsAtOnce(t *testing.T) {
	for _, skipped := range []bool{false, true} {
		port := &racing{fakeNames: newFakeNames(), first: "A", written: make(chan struct{}), answer: make(chan struct{})}
		n := newNames(port, discard())
		ctx := context.Background()
		var wg sync.WaitGroup
		wg.Go(func() { n.of(ctx, aliceTok, "A") })
		<-port.written
		port.set(func(f *fakeNames) { f.skip = skipped })
		if got := n.of(ctx, aliceTok, "B"); got != "B" {
			t.Errorf("skipped %v: B told: %q", skipped, got)
		}
		close(port.answer)
		wg.Wait()
		port.set(func(f *fakeNames) { f.skip = false })
		if got := n.of(ctx, aliceTok, ""); got != "B" || port.of(aliceTok) != "B" {
			t.Errorf("skipped %v: then a call telling none: %q, the row %q; want B, the name told last", skipped, got, port.of(aliceTok))
		}
	}
}

// A name told while a call that tells none reads the row is the token's:
// the older name the read found is not remembered over it.
func TestTheNamesKeepATellingOverAReadBeforeIt(t *testing.T) {
	port := newFakeNames()
	port.recorded[aliceTok] = "old-client"
	n := newNames(port, discard())
	ctx := context.Background()
	port.afterRead = func() { n.of(ctx, aliceTok, "new-client") }
	for i := range 2 {
		if got := n.of(ctx, aliceTok, ""); got != "new-client" {
			t.Errorf("call %d telling none, new-client told during the first's read: %q", i+1, got)
		}
	}
}

// The names remember 1,024 tokens: the 1,025th puts out the least
// recently used, not the first remembered, and its next call reads it
// again (S3 review B-N1).
func TestTheNamesForgetTheLeastRecentlyUsed(t *testing.T) {
	port := newFakeNames()
	n := newNames(port, discard())
	ctx := context.Background()
	tokens := make([]uuid.UUID, maxNamedTokens+1)
	for i := range tokens[:maxNamedTokens] {
		tokens[i] = uuid.NewV7()
		n.of(ctx, tokens[i], "")
	}
	n.of(ctx, tokens[0], "")
	tokens[maxNamedTokens] = uuid.NewV7()
	n.of(ctx, tokens[maxNamedTokens], "")
	if reads, _ := port.counts(); reads != len(tokens) {
		t.Fatalf("%d reads, want one a token", reads)
	}
	n.of(ctx, tokens[0], "")
	for _, tok := range tokens[2:] {
		n.of(ctx, tok, "")
	}
	if reads, _ := port.counts(); reads != len(tokens) {
		t.Errorf("the 1,024 last used read again: %d reads", reads)
	}
	n.of(ctx, tokens[1], "")
	if reads, _ := port.counts(); reads != len(tokens)+1 {
		t.Errorf("the least recently used token, put out, was not read again: %d reads", reads)
	}
}

// A told name is remembered as a copy of its own: never a part of the
// request's string, which can be the body's megabyte.
func TestTheNamesHoldTheirOwnBytes(t *testing.T) {
	n := newNames(newFakeNames(), discard())
	told := "agent-a" + strings.Repeat(" ", 1<<20)
	n.of(context.Background(), aliceTok, told)
	known, ok := n.remembered(aliceTok)
	if !ok || known.name != "agent-a" {
		t.Fatalf("remembered %q, %v", known.name, ok)
	}
	start := uintptr(unsafe.Pointer(unsafe.StringData(told)))
	if at := uintptr(unsafe.Pointer(unsafe.StringData(known.name))); at >= start && at < start+uintptr(len(told)) {
		t.Error("the name remembered is a part of the told one")
	}
}
