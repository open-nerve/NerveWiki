package mcpserver

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// The SDK logs at warn and above alone, as component mcp-sdk: its info
// lines of every stateless POST stay out, a later With keeps the filter
// (M9/P1 design 3.4.5).
func TestTheSDKLogsOnlyWarnings(t *testing.T) {
	s := serveMCP(t, setup{})
	result(t, s.post(t, context.Background(), Pattern, patToken, initialize, nil))
	result(t, s.call(t, Pattern, "echo", map[string]any{"notebook": visible.String()}))
	s.post(t, context.Background(), Pattern, patToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, v2025())
	time.Sleep(50 * time.Millisecond)
	for _, e := range s.logs.entries(t) {
		if e["component"] == "mcp-sdk" {
			t.Errorf("the SDK logged %v", e)
		}
	}
	logs := &logBuffer{}
	logger := sdkLogger(jsonLogger(logs)).With("more", 1)
	logger.Info("an info line")
	logger.Warn("a warning")
	logger.WithGroup("g").Error("an error")
	entries := logs.entries(t)
	if len(entries) != 2 || entries[0]["msg"] != "a warning" || entries[0]["component"] != "mcp-sdk" || entries[1]["msg"] != "an error" {
		t.Errorf("the SDK's logger wrote %v, want the warning and the error, as mcp-sdk", entries)
	}
}

// A handshake logs one line, a call one line: who, through which entry
// and notebook, what came of it; never the arguments, never the answer.
func TestTheCallsAndHandshakesLogOneLineEach(t *testing.T) {
	s := serveMCP(t, setup{})
	nb := Pattern + "?notebook=" + visible.String()
	result(t, s.post(t, context.Background(), nb, patToken, initialize, nil))
	result(t, s.call(t, nb, "echo", map[string]any{"s": "SECRET-ARGUMENT"}))
	result(t, s.call(t, Pattern, "echo", map[string]any{"s": 1}))
	time.Sleep(50 * time.Millisecond)
	hs := s.logs.find(t, "MCP initialize")
	if hs == nil || hs["level"] != "INFO" || hs["entry"] != "notebook" || hs["notebook_id"] != visible.String() ||
		hs["protocol_version"] != "2025-11-25" || hs["client"] != UnknownClient || hs["user_id"] != alice.String() ||
		hs["token_id"] != aliceTok.String() || hs["request_id"] == "" {
		t.Errorf("the handshake's line = %v", hs)
	}
	var calls []map[string]any
	for _, e := range s.logs.entries(t) {
		if e["msg"] == "MCP tool call" {
			calls = append(calls, e)
		}
		if line := strings.Join(strings.Fields(strings.ReplaceAll(fmtMap(e), "\n", " ")), " "); strings.Contains(line, "SECRET-ARGUMENT") {
			t.Errorf("a log line holds an argument: %v", e)
		}
	}
	if len(calls) != 2 || calls[0]["tool"] != "echo" || calls[0]["outcome"] != "ok" || calls[0]["notebook_id"] != visible.String() ||
		calls[0]["client"] != UnknownClient || calls[0]["duration_ms"] == nil || calls[0]["request_id"] == "" ||
		calls[1]["outcome"] != "bad_request" || calls[1]["notebook_id"] != nil {
		t.Errorf("the calls' lines = %v", calls)
	}
}

// fmtMap is a log line as text.
func fmtMap(e map[string]any) string {
	var b strings.Builder
	for k, v := range e {
		b.WriteString(k + "=" + slog.AnyValue(v).String() + " ")
	}
	return b.String()
}
