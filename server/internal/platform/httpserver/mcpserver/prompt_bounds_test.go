package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// promptGet is a prompt's handler.
type promptGet = func(ctx context.Context, c Call, args map[string]string) (string, []string, error)

// answersText is a handler that answers description and messages.
func answersText(description string, messages ...string) promptGet {
	return func(context.Context, Call, map[string]string) (string, []string, error) {
		return description, messages, nil
	}
}

// promptServer serves a prompt "p" of both entries with arguments, its
// handler get: gets counts the handler's calls.
func promptServer(t *testing.T, get promptGet, arguments ...PromptArgument) (s *served, gets *atomic.Int32) {
	t.Helper()
	gets = &atomic.Int32{}
	counted := func(ctx context.Context, c Call, args map[string]string) (string, []string, error) {
		gets.Add(1)
		return get(ctx, c, args)
	}
	s = serveMCP(t, setup{options: func(o *Options) {
		o.Prompts = []Prompt{{Name: "p", Entry: BothEntries, Arguments: arguments, Get: counted}}
	}})
	return s, gets
}

// notebookPath is the notebook entry of the notebook alice sees.
func notebookPath() string { return Pattern + "?notebook=" + visible.String() }

// getPromptBody is the body of a prompts/get of name with args.
func getPromptBody(t testing.TB, name string, args map[string]string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 3, "method": methodGetPrompt,
		"params": map[string]any{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// getPrompt is a raw prompts/get in the notebook entry, after a handshake
// of 2025-11-25.
func (s *served) getPrompt(ctx context.Context, t testing.TB, name string, args map[string]string) answer {
	t.Helper()
	return s.post(t, ctx, notebookPath(), patToken, getPromptBody(t, name, args), v2025())
}

// rpcError is the answer's JSON-RPC error, its data decoded.
func rpcError(t testing.TB, a answer) (code int, message string, data map[string]any) {
	t.Helper()
	code = rpcErrorCode(t, a)
	e := a.decoded(t)["error"].(map[string]any)
	message, _ = e["message"].(string)
	data, _ = e["data"].(map[string]any)
	return code, message, data
}

// promptLines are the call lines of the prompts so far.
func promptLines(t testing.TB, l *logBuffer) []map[string]any {
	t.Helper()
	var found []map[string]any
	for _, e := range l.entries(t) {
		if e["msg"] == "MCP prompt call" {
			found = append(found, e)
		}
	}
	return found
}

// awaitLog waits for the line of msg: a fault's line follows the client's
// answer, or its leaving, and the handler's return.
func awaitLog(t testing.TB, l *logBuffer, msg string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e := l.find(t, msg); e != nil {
			return e
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no log line %q in %v", msg, l.entries(t))
	return nil
}

// A prompt is served where its Entry says: a wiring fault panics, each
// entry lists its own and says prompts only when it has one, and the
// account entry without any lists none and answers a get as the SDK does.
func TestPromptEntries(t *testing.T) {
	get := answersText("")
	for _, tt := range []struct {
		name    string
		prompts []Prompt
		want    string
	}{
		{"no entry", []Prompt{{Name: "p", Get: get}}, "listed in no entry"},
		{"an entry that is none", []Prompt{{Name: "p", Entry: 4, Get: get}}, "listed in no entry"},
		{"a bit past the entries", []Prompt{{Name: "p", Entry: BothEntries | 4, Get: get}}, "listed in no entry"},
		{"a name that is none", []Prompt{{Name: "P", Entry: BothEntries, Get: get}}, "a name is 1 to 32"},
		{"twice", []Prompt{{Name: "p", Entry: NotebookEntry, Get: get}, {Name: "p", Entry: AccountEntry, Get: get}}, "registered twice"},
		{"no handler", []Prompt{{Name: "p", Entry: BothEntries}}, "has no handler"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if v := recover(); v == nil || !strings.Contains(v.(string), tt.want) {
					t.Errorf("New() panicked with %v, want %q", v, tt.want)
				}
			}()
			o := bare(discard())
			o.Prompts = tt.prompts
			New(o)
		})
	}

	capabilities := func(t *testing.T, s *served, path string) string {
		t.Helper()
		b, err := json.Marshal(result(t, s.post(t, context.Background(), path, patToken, initialize, nil))["capabilities"])
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	names := func(t *testing.T, s *served, path string) []string {
		t.Helper()
		listed, ok := result(t, s.post(t, context.Background(), path, patToken, `{"jsonrpc":"2.0","id":2,"method":"prompts/list"}`, v2025()))["prompts"].([]any)
		if !ok {
			t.Fatalf("%s: prompts/list has no array of prompts", path)
		}
		out := make([]string, len(listed))
		for i, p := range listed {
			out[i] = p.(map[string]any)["name"].(string)
		}
		return out
	}

	t.Run("each entry lists its own", func(t *testing.T) {
		s := serveMCP(t, setup{options: func(o *Options) {
			o.Prompts = []Prompt{{Name: "both", Entry: BothEntries, Get: get}, {Name: "ours", Entry: NotebookEntry, Get: get}, {Name: "theirs", Entry: AccountEntry, Get: get}}
		}})
		if got := names(t, s, notebookPath()); !slices.Equal(got, []string{"both", "ours"}) {
			t.Errorf("the notebook entry lists %v", got)
		}
		if got := names(t, s, Pattern); !slices.Equal(got, []string{"both", "theirs"}) {
			t.Errorf("the account entry lists %v", got)
		}
		for _, path := range []string{notebookPath(), Pattern} {
			if got := capabilities(t, s, path); got != `{"prompts":{},"tools":{}}` {
				t.Errorf("%s: capabilities %s", path, got)
			}
		}
	})

	t.Run("the notebook entry's alone", func(t *testing.T) {
		var gets atomic.Int32
		s := serveMCP(t, setup{options: func(o *Options) {
			o.Prompts = []Prompt{{Name: "ours", Entry: NotebookEntry, Get: func(context.Context, Call, map[string]string) (string, []string, error) {
				gets.Add(1)
				return "", nil, nil
			}}}
		}})
		if got := capabilities(t, s, notebookPath()); got != `{"prompts":{},"tools":{}}` {
			t.Errorf("the notebook entry's capabilities %s", got)
		}
		if got := capabilities(t, s, Pattern); got != `{"tools":{}}` {
			t.Errorf("the account entry's capabilities %s", got)
		}
		if got := names(t, s, Pattern); got == nil || len(got) != 0 {
			t.Errorf("the account entry lists %#v, want an empty list", got)
		}
		a := s.post(t, context.Background(), Pattern, patToken, getPromptBody(t, "ours", nil), v2025())
		if code, message, data := rpcError(t, a); code != -32602 || !strings.Contains(message, "unknown prompt") || data != nil {
			t.Errorf("the account entry's get = %d %q %v, want the SDK's unknown prompt", code, message, data)
		}
		b := s.getPrompt(context.Background(), t, "nothing", nil)
		if code, _, data := rpcError(t, b); code != -32602 || data != nil {
			t.Errorf("an unknown get = %d %v, want the SDK's -32602", code, data)
		}
		if n := gets.Load(); n != 0 {
			t.Errorf("the handler ran %d times, want never", n)
		}
		if lines := promptLines(t, s.logs); len(lines) != 0 {
			t.Errorf("a name the SDK knows not logged %v, want no call line (M10 decision 162)", lines)
		}
	})
}

// The declared arguments are checked before the handler: a required key
// left out, a value over MaxPromptArgumentBytes, whose words never repeat
// it; an empty value is a value, and an argument no prompt declares passes
// through.
func TestPromptArgumentsBeforeGet(t *testing.T) {
	declared := []PromptArgument{{Name: "question", Required: true}, {Name: "source"}}
	var seen map[string]string
	get := func(_ context.Context, _ Call, args map[string]string) (string, []string, error) {
		seen = maps.Clone(args)
		return "", []string{"ok"}, nil
	}
	refused := func(t *testing.T, args map[string]string, name, words string) {
		t.Helper()
		s, gets := promptServer(t, get, declared...)
		code, message, data := rpcError(t, s.getPrompt(context.Background(), t, "p", args))
		if code != -32602 || !strings.HasPrefix(message, "validation_failed: ") || !strings.Contains(message, `"`+name+`"`) ||
			!strings.Contains(message, words) || data["code"] != "validation_failed" {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		if strings.Contains(message, "xxxx") || strings.Contains(message, "éééé") {
			t.Errorf("the message repeats the value: %.200q", message)
		}
		if n := gets.Load(); n != 0 {
			t.Errorf("the handler ran %d times, want never", n)
		}
		if lines := promptLines(t, s.logs); len(lines) != 1 || lines[0]["outcome"] != "validation_failed" {
			t.Errorf("the call lines are %v, want one of validation_failed", lines)
		}
	}
	reached := func(t *testing.T, args map[string]string) {
		t.Helper()
		s, gets := promptServer(t, get, declared...)
		seen = nil
		result(t, s.getPrompt(context.Background(), t, "p", args))
		if n := gets.Load(); n != 1 || !maps.Equal(seen, args) {
			t.Errorf("the handler ran %d times with %.200q, want once with what was sent", n, seen)
		}
	}
	t.Run("missing", func(t *testing.T) { refused(t, map[string]string{"source": "a"}, "question", "missing") })
	t.Run("no_arguments", func(t *testing.T) { refused(t, nil, "question", "missing") })
	t.Run("empty_required", func(t *testing.T) { reached(t, map[string]string{"question": ""}) })
	t.Run("optional_left_out", func(t *testing.T) { reached(t, map[string]string{"question": "q"}) })
	t.Run("optional_empty", func(t *testing.T) { reached(t, map[string]string{"question": "q", "source": ""}) })
	t.Run("at4096", func(t *testing.T) { reached(t, map[string]string{"question": strings.Repeat("x", 4096)}) })
	t.Run("at4096_characters", func(t *testing.T) { reached(t, map[string]string{"question": strings.Repeat("é", 2048)}) })
	t.Run("at4096_optional", func(t *testing.T) {
		reached(t, map[string]string{"question": "q", "source": strings.Repeat("x", 4096)})
	})
	t.Run("over4096", func(t *testing.T) {
		refused(t, map[string]string{"question": strings.Repeat("x", 4097)}, "question", "over 4096 bytes")
	})
	t.Run("over4096_characters", func(t *testing.T) {
		refused(t, map[string]string{"question": strings.Repeat("é", 2048) + "x"}, "question", "over 4096 bytes")
	})
	t.Run("over4096_optional", func(t *testing.T) {
		refused(t, map[string]string{"question": "q", "source": strings.Repeat("x", 4097)}, "source", "over 4096 bytes")
	})
	t.Run("within_the_request", func(t *testing.T) {
		refused(t, map[string]string{"question": strings.Repeat("x", 60<<10)}, "question", "over 4096 bytes")
	})
	t.Run("undeclared", func(t *testing.T) { reached(t, map[string]string{"question": "q", "other": strings.Repeat("x", 5000)}) })
}

// answerTexts are the raw JSON strings of the answer's description and of
// its messages' texts, as the SDK wrote them, their quotes taken off.
func answerTexts(t testing.TB, a answer) []string {
	t.Helper()
	var v struct {
		Result struct {
			Description json.RawMessage `json:"description"`
			Messages    []struct {
				Content struct {
					Text json.RawMessage `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(a.body, &v); err != nil {
		t.Fatalf("answer %d %.200s: %v", a.status, a.body, err)
	}
	var out []string
	if len(v.Result.Description) > 2 {
		out = append(out, string(v.Result.Description[1:len(v.Result.Description)-1]))
	}
	for _, m := range v.Result.Messages {
		out = append(out, string(m.Content.Text[1:len(m.Content.Text)-1]))
	}
	return out
}

// The answer's bound is TextBytes of its description and its messages,
// the bytes the SDK writes: exactly MaxAnswerBytes goes out, whatever
// escapes it holds, and a byte more is a fault, logged, no part of it
// sent.
func TestPromptAnswersUseSDKTextBytes(t *testing.T) {
	sent := func(t *testing.T, description string, messages ...string) {
		t.Helper()
		s, _ := promptServer(t, answersText(description, messages...))
		a := s.getPrompt(context.Background(), t, "p", nil)
		result(t, a)
		n := 0
		for _, text := range answerTexts(t, a) {
			n += len(text)
		}
		if n != MaxAnswerBytes {
			t.Errorf("the SDK wrote %d bytes of text, want %d", n, MaxAnswerBytes)
		}
		if lines := promptLines(t, s.logs); len(lines) != 1 || lines[0]["outcome"] != "ok" {
			t.Errorf("the call lines are %v, want one of ok", lines)
		}
	}
	refused := func(t *testing.T, description string, messages ...string) {
		t.Helper()
		s, _ := promptServer(t, answersText(description, messages...))
		a := s.getPrompt(context.Background(), t, "p", nil)
		code, message, data := rpcError(t, a)
		if code != -32603 || !strings.HasPrefix(message, "internal_error: the server failed; request id ") || data != nil {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		if a.decoded(t)["result"] != nil {
			t.Errorf("an answer over its bound sent a result: %.200s", a.body)
		}
		want := TextBytes(description)
		for _, m := range messages {
			want += TextBytes(m)
		}
		line := s.logs.find(t, "MCP answer over its bound")
		if line == nil || line["level"] != "ERROR" || line["prompt"] != "p" || line["bytes"] != float64(want) {
			t.Errorf("the log line = %v, want the prompt and its %d bytes at error", line, want)
		}
		if lines := promptLines(t, s.logs); len(lines) != 1 || lines[0]["outcome"] != "internal_error" {
			t.Errorf("the call lines are %v, want one of internal_error", lines)
		}
	}
	a := func(n int) string { return strings.Repeat("a", n) }
	t.Run("at_bound", func(t *testing.T) { sent(t, "d", a(MaxAnswerBytes-1)) })
	t.Run("over", func(t *testing.T) { refused(t, "d", a(MaxAnswerBytes)) })
	t.Run("messages_at_bound", func(t *testing.T) { sent(t, "", a(16<<10), a(16<<10), a(16<<10)) })
	t.Run("messages_over", func(t *testing.T) { refused(t, "", a(16<<10), a(16<<10), a(16<<10+1)) })
	t.Run("description_at_bound", func(t *testing.T) { sent(t, a(MaxAnswerBytes)) })
	t.Run("description_over", func(t *testing.T) { refused(t, a(MaxAnswerBytes+1)) })
	// 4,096 of the longest escape on each side are the bound exactly; raw,
	// they are 8 KiB, so a bound that counted raw bytes passes one more.
	lt := func(n int) string { return strings.Repeat("<", n) }
	t.Run("escaped_at_bound", func(t *testing.T) { sent(t, lt(4096), lt(4096)) })
	t.Run("escaped_over", func(t *testing.T) { refused(t, lt(4096), lt(4097)) })
	for _, c := range []struct {
		name, char string
		bytes      int
	}{
		{"gt", ">", 6}, {"amp", "&", 6}, {"nul", "\x00", 6}, {"u2028", " ", 6}, {"u2029", " ", 6},
		{"newline", "\n", 2}, {"return", "\r", 2}, {"tab", "\t", 2}, {"backspace", "\b", 2}, {"formfeed", "\f", 2},
		{"quote", `"`, 2}, {"backslash", `\`, 2},
	} {
		n := MaxAnswerBytes / c.bytes
		t.Run(c.name+"_at_bound", func(t *testing.T) { sent(t, "", strings.Repeat(c.char, n)) })
		t.Run(c.name+"_over", func(t *testing.T) { refused(t, "", strings.Repeat(c.char, n+1)) })
	}
}

// A get has no exception to MaxOtherBytes: the padding goes in an argument
// the prompt does not declare, or the arguments' own bound would answer
// first.
func TestPromptRequestBodyHasNoException(t *testing.T) {
	bodyOf := func(t *testing.T, size int) string {
		t.Helper()
		empty := getPromptBody(t, "p", map[string]string{"padding": ""})
		body := getPromptBody(t, "p", map[string]string{"padding": strings.Repeat("x", size-len(empty))})
		if len(body) != size {
			t.Fatalf("the body is %d bytes, want %d", len(body), size)
		}
		return body
	}
	t.Run("at65536", func(t *testing.T) {
		s, gets := promptServer(t, answersText("", "ok"))
		a := s.post(t, context.Background(), notebookPath(), patToken, bodyOf(t, MaxOtherBytes), v2025())
		if a.status != http.StatusOK || gets.Load() != 1 {
			t.Errorf("answer %d %.200s, the handler ran %d times; want 200, once", a.status, a.body, gets.Load())
		}
	})
	t.Run("over65536", func(t *testing.T) {
		s, gets := promptServer(t, answersText("", "ok"))
		a := s.post(t, context.Background(), notebookPath(), patToken, bodyOf(t, MaxOtherBytes+1), v2025())
		if a.status != http.StatusRequestEntityTooLarge || a.decoded(t)["code"] != "payload_too_large" || gets.Load() != 0 {
			t.Errorf("answer %d %.200s, the handler ran %d times; want 413, never", a.status, a.body, gets.Load())
		}
	})
}

// A handler's error is classified as a tool's is: a problem in its own
// words, -32602 for a 4xx and -32603 for a 5xx, its code in data; a fault
// as any method's, without data, at its level. Every get has one line,
// which never holds what the caller sent.
func TestPromptErrorsAndLogs(t *testing.T) {
	failing := func(err error) promptGet {
		return func(context.Context, Call, map[string]string) (string, []string, error) {
			return "", []string{"lost"}, err
		}
	}
	outcome := func(t *testing.T, s *served, want string) {
		t.Helper()
		lines := promptLines(t, s.logs)
		if len(lines) != 1 {
			t.Fatalf("the call lines are %v, want one", lines)
		}
		l := lines[0]
		if l["outcome"] != want || l["prompt"] != "p" || l["entry"] != "notebook" || l["notebook_id"] != visible.String() ||
			l["user_id"] != alice.String() || l["token_id"] != aliceTok.String() || l["request_id"] == "" || l["level"] != "INFO" {
			t.Errorf("the call line = %v, want outcome %s", l, want)
		}
		if _, ok := l["duration_ms"]; !ok {
			t.Errorf("the call line = %v, without its duration", l)
		}
	}

	t.Run("ok", func(t *testing.T) {
		s, _ := promptServer(t, answersText("", "fine"))
		result(t, s.getPrompt(context.Background(), t, "p", nil))
		outcome(t, s, "ok")
	})
	t.Run("problem4xx", func(t *testing.T) {
		s, _ := promptServer(t, failing(problem{status: http.StatusConflict, code: "notebook.not_llm_wiki", detail: "The notebook is not an LLM Wiki."}))
		a := s.getPrompt(context.Background(), t, "p", nil)
		code, message, data := rpcError(t, a)
		if code != -32602 || message != "notebook.not_llm_wiki: The notebook is not an LLM Wiki." || data["code"] != "notebook.not_llm_wiki" {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		if strings.Contains(string(a.body), "lost") {
			t.Errorf("the answer holds the messages of a handler that failed: %s", a.body)
		}
		outcome(t, s, "notebook.not_llm_wiki")
	})
	t.Run("problem5xx", func(t *testing.T) {
		s, _ := promptServer(t, failing(problem{status: http.StatusServiceUnavailable, code: "wiki.busy", detail: "Busy."}))
		code, message, data := rpcError(t, s.getPrompt(context.Background(), t, "p", nil))
		if code != -32603 || message != "wiki.busy: Busy." || data["code"] != "wiki.busy" {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		outcome(t, s, "wiki.busy")
	})

	unknown := func(t *testing.T, err error, cause string) {
		t.Helper()
		s, _ := promptServer(t, failing(err))
		code, message, data := rpcError(t, s.getPrompt(context.Background(), t, "p", nil))
		if code != -32603 || !strings.HasPrefix(message, "internal_error: the server failed; request id ") || data != nil {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		if line := awaitLog(t, s.logs, "MCP method failed"); line["level"] != "ERROR" || line["method"] != methodGetPrompt ||
			!strings.Contains(line["error"].(string), cause) {
			t.Errorf("the log line = %v", line)
		}
		outcome(t, s, "internal_error")
	}
	t.Run("unknown", func(t *testing.T) { unknown(t, errors.New("the disk is gone"), "the disk is gone") })
	// On a context that lives they are the server's own, not the client's.
	t.Run("bare_canceled", func(t *testing.T) { unknown(t, fmt.Errorf("reading: %w", context.Canceled), "reading") })
	t.Run("bare_deadline", func(t *testing.T) { unknown(t, fmt.Errorf("reading: %w", context.DeadlineExceeded), "reading") })

	t.Run("client_cancel", func(t *testing.T) {
		entered := make(chan struct{})
		s, _ := promptServer(t, func(ctx context.Context, _ Call, _ map[string]string) (string, []string, error) {
			close(entered)
			<-ctx.Done()
			return "", nil, ctx.Err()
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-entered
			cancel()
		}()
		s.getPrompt(ctx, t, "p", nil)
		if line := awaitLog(t, s.logs, "MCP client went away"); line["level"] != "DEBUG" || line["method"] != methodGetPrompt {
			t.Errorf("the log line = %v, want the client's leaving at debug", line)
		}
		awaitLog(t, s.logs, "MCP prompt call")
		outcome(t, s, "internal_error")
		if lines := s.logs.at(t, "ERROR"); len(lines) != 0 {
			t.Errorf("a client that left logged %v at error", lines)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		s := serveMCP(t, setup{options: func(o *Options) {
			o.RequestTimeout = 200 * time.Millisecond
			o.Prompts = []Prompt{{Name: "p", Entry: BothEntries, Get: func(ctx context.Context, _ Call, _ map[string]string) (string, []string, error) {
				<-ctx.Done()
				return "", nil, ctx.Err()
			}}}
		}})
		code, message, data := rpcError(t, s.getPrompt(context.Background(), t, "p", nil))
		if code != -32603 || !strings.HasPrefix(message, "internal_error: the call ran out of time; request id ") || data != nil {
			t.Errorf("answer = %d %q %v", code, message, data)
		}
		if line := awaitLog(t, s.logs, "MCP call deadline exceeded"); line["level"] != "WARN" || line["method"] != methodGetPrompt {
			t.Errorf("the log line = %v", line)
		}
		outcome(t, s, "internal_error")
	})
	t.Run("panic", func(t *testing.T) {
		s, _ := promptServer(t, func(context.Context, Call, map[string]string) (string, []string, error) { panic("prompt: getting") })
		a := s.getPrompt(context.Background(), t, "p", nil)
		e, _ := a.decoded(t)["error"].(map[string]any)
		if len(e) != 2 || e["code"] != float64(-32603) || e["message"] != "internal error" {
			t.Errorf("answer = %d %s, want only the code and the message", a.status, a.body)
		}
		if line := awaitLog(t, s.logs, "MCP method panicked"); line["level"] != "ERROR" || line["method"] != methodGetPrompt {
			t.Errorf("the log line = %v", line)
		}
		outcome(t, s, "internal_error")
	})
	t.Run("no_arguments", func(t *testing.T) {
		const source, question, text = "secret-source-path", "secret-question-words", "secret-answer-text"
		s, _ := promptServer(t, answersText("secret-answer-description", text), PromptArgument{Name: "source"}, PromptArgument{Name: "question"})
		result(t, s.getPrompt(context.Background(), t, "p", map[string]string{"source": source, "question": question}))
		over := s.getPrompt(context.Background(), t, "p", map[string]string{"source": source, "question": "secret-" + strings.Repeat("x", 5000)})
		if code, _, _ := rpcError(t, over); code != -32602 {
			t.Errorf("an argument over its bound answered %d", code)
		}
		if len(promptLines(t, s.logs)) != 2 {
			t.Fatalf("the call lines are %v, want two", promptLines(t, s.logs))
		}
		for _, e := range s.logs.entries(t) {
			if line := fmt.Sprint(e); strings.Contains(line, "secret-") {
				t.Errorf("a log line holds what was sent or answered: %.300s", line)
			}
		}
	})
}

// A get over the Go client reaches the handler with its call's identity
// (M10 design 4.15): the client's name, the notebook and the token.
func TestPromptCallKnowsItsCaller(t *testing.T) {
	var got Call
	s, _ := promptServer(t, func(_ context.Context, c Call, _ map[string]string) (string, []string, error) {
		got = c
		return "", []string{"ok"}, nil
	})
	cs := s.connect(t, notebookPath(), nil)
	if _, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "p"}); err != nil {
		t.Fatal(err)
	}
	if got.Entry != NotebookEntry || got.Notebook != visible || got.UserID != alice || got.TokenID != aliceTok {
		t.Errorf("the handler had %+v", got)
	}
}
