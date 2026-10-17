package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
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
// and notebook (the account entry's from its argument), what came of it,
// ok or the code of its error, a panic's too; never the arguments, never
// the answer.
func TestTheCallsAndHandshakesLogOneLineEach(t *testing.T) {
	missing := toolOf("missing", func(context.Context, Call) (Result, error) {
		return Result{}, problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page."}
	})
	boom := toolOf("boom", func(context.Context, Call) (Result, error) { nilMap()["x"]++; return Result{}, nil })
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, missing, boom) }})
	nb := Pattern + "?notebook=" + visible.String()
	result(t, s.post(t, context.Background(), nb, patToken, initialize, nil))
	result(t, s.call(t, nb, "echo", map[string]any{"s": "SECRET-ARGUMENT"}))
	result(t, s.call(t, Pattern, "echo", map[string]any{"s": 1}))
	result(t, s.call(t, Pattern, "echo", map[string]any{"notebook": visible.String()}))
	result(t, s.call(t, Pattern, "missing", nil))
	result(t, s.call(t, Pattern, "boom", nil))
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
	if len(calls) != 5 || calls[0]["tool"] != "echo" || calls[0]["outcome"] != "ok" || calls[0]["notebook_id"] != visible.String() ||
		calls[0]["client"] != UnknownClient || calls[0]["duration_ms"] == nil || calls[0]["request_id"] == "" ||
		calls[1]["outcome"] != "bad_request" || calls[1]["notebook_id"] != nil {
		t.Fatalf("the calls' lines = %v", calls)
	}
	for i, want := range []map[string]any{
		{"tool": "echo", "notebook_id": visible.String(), "outcome": "ok"},
		{"tool": "missing", "notebook_id": nil, "outcome": "page.not_found"},
		{"tool": "boom", "notebook_id": nil, "outcome": "internal_error"},
	} {
		for k, v := range want {
			if calls[2+i][k] != v {
				t.Errorf("%s's call line = %v, want %s %v", want["tool"], calls[2+i], k, v)
			}
		}
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
