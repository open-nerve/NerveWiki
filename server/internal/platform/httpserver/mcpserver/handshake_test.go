package mcpserver

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"
)

// instructed is fakeNotebooks whose instructions are text, or err: it
// counts their reads (M9/P1 design 3.15.2, review A-M15).
type instructed struct {
	*fakeNotebooks
	reads atomic.Int32
	text  string
	err   error
}

func (i *instructed) Instructions(_ context.Context, id uuid.UUID) (string, error) {
	i.reads.Add(1)
	if id != visible {
		return "", errors.New("the instructions of a notebook the gate did not let in")
	}
	return i.text, i.err
}

// withInstructions is a server whose notebook's instructions are text or
// err, and whose account entry's are account.
func withInstructions(t *testing.T, text, account string, err error) (*served, *instructed) {
	t.Helper()
	nb := &instructed{fakeNotebooks: &fakeNotebooks{}, text: text, err: err}
	return serveMCP(t, setup{options: func(o *Options) { o.Notebooks, o.Account = nb, account }}), nb
}

// A handshake's result has its entry's instructions: the notebook's, which
// the gate let in, or the account entry's; none when they are "".
func TestTheHandshakeSendsItsEntrysInstructions(t *testing.T) {
	s, _ := withInstructions(t, "# Conventions\nWrite <em> & \"quote\".", "Account rules.", nil)
	nb := Pattern + "?notebook=" + visible.String()
	if got := result(t, s.post(t, context.Background(), nb, patToken, initialize, nil))["instructions"]; got != "# Conventions\nWrite <em> & \"quote\"." {
		t.Errorf("the notebook entry's instructions = %q", got)
	}
	if got := result(t, s.post(t, context.Background(), Pattern, patToken, initialize, nil))["instructions"]; got != "Account rules." {
		t.Errorf("the account entry's instructions = %q", got)
	}
	cs := s.connect(t, nb, nil)
	if got := cs.InitializeResult().Instructions; !strings.HasPrefix(got, "# Conventions") {
		t.Errorf("the Go client's instructions = %q", got)
	}
	none, _ := withInstructions(t, "", "", nil)
	for _, path := range []string{nb, Pattern} {
		if r := result(t, none.post(t, context.Background(), path, patToken, initialize, nil)); r["instructions"] != nil {
			t.Errorf("%s: instructions %q, want none", path, r["instructions"])
		}
	}
}

// Only an initialize reads the instructions, once; its log line follows.
// A ping, a tools/list or a tools/call reads none, so that the handshake
// reads them only for an initialize's result, never before the SDK
// answers (M9/P1 design 6.3).
func TestTheHandshakeReadsInstructionsOnlyForInitialize(t *testing.T) {
	s, nb := withInstructions(t, "Rules.", "", nil)
	path := Pattern + "?notebook=" + visible.String()
	result(t, s.post(t, context.Background(), path, patToken, initialize, nil))
	if n := nb.reads.Load(); n != 1 {
		t.Fatalf("a handshake read the instructions %d times, want once", n)
	}
	result(t, s.post(t, context.Background(), path, patToken, `{"jsonrpc":"2.0","id":2,"method":"ping"}`, v2025()))
	result(t, s.post(t, context.Background(), path, patToken, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`, v2025()))
	result(t, s.call(t, path, "echo", nil))
	s.post(t, context.Background(), path, patToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, v2025())
	result(t, s.post(t, context.Background(), Pattern, patToken, initialize, nil))
	if n := nb.reads.Load(); n != 1 {
		t.Errorf("the instructions were read %d times, want once: by the notebook entry's handshake alone", n)
	}
}

// Instructions that fail to read are the handshake's JSON-RPC internal
// error, naming the request, logged at error: never the built-in ones,
// nor a handshake without them.
func TestInstructionsThatFailAreTheHandshakesError(t *testing.T) {
	s, _ := withInstructions(t, "", "", errors.New("the database is gone"))
	a := s.post(t, context.Background(), Pattern+"?notebook="+visible.String(), patToken, initialize, nil)
	v := a.decoded(t)
	e, _ := v["error"].(map[string]any)
	message, _ := e["message"].(string)
	if v["result"] != nil || e["code"] != float64(-32603) || !strings.HasPrefix(message, "internal_error: the server failed; request id ") {
		t.Fatalf("answer = %d %s, want the JSON-RPC internal error with the request id", a.status, a.body)
	}
	time.Sleep(50 * time.Millisecond)
	line := s.logs.find(t, "MCP method failed")
	if line == nil || line["level"] != "ERROR" || line["method"] != "initialize" || !strings.Contains(message, line["request_id"].(string)) ||
		!strings.Contains(line["error"].(string), "the database is gone") {
		t.Errorf("the log line = %v", line)
	}
	if s.logs.find(t, "MCP initialize") != nil {
		t.Error("a handshake that failed logged its initialize")
	}
}
