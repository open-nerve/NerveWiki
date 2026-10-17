package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// wireText is s's bytes in a text block as the SDK writes it, without
// the surrounding quotes: the block's JSON less an empty block's.
func wireText(t *testing.T, s string) int {
	t.Helper()
	block, err := json.Marshal(&mcp.TextContent{Text: s})
	if err != nil {
		t.Fatal(err)
	}
	empty, _ := json.Marshal(&mcp.TextContent{})
	return len(block) - len(empty)
}

// wireMember is s's bytes as a string member of a structured answer on
// the wire: the text block of {"k": s} less that of {"k": ""}.
func wireMember(t *testing.T, s string) int {
	t.Helper()
	with, err := encode(map[string]string{"k": s})
	if err != nil {
		t.Fatal(err)
	}
	without, _ := encode(map[string]string{"k": ""})
	return wireText(t, string(with)) - wireText(t, string(without))
}

// The characters of the measures' table (M9/P1 design 3.4.4), each row's
// and the others.
func measuredRunes() []string {
	return []string{`"`, `\`, "\n", "\r", "\t", "\b", "\f", "\x00", "\x01", "\x1f", "<", ">", "&", "\u2028", "\u2029",
		"\x7f", "\u0080", "\u0085", "\u009f", "a", " ", "é", "漢", "😀", "'", "/", "~"}
}

// TextBytes and MemberBytes are the SDK's encoding of a text block and of
// a structured answer's member: each row of the table, a fixed example,
// and random strings of all of them.
func TestTheMeasuresAreTheWires(t *testing.T) {
	fixed := []struct {
		s            string
		text, member int
	}{
		{`"`, 2, 4}, {`\`, 2, 4}, {"\n", 2, 3}, {"\r", 2, 3}, {"\t", 2, 3}, {"\b", 2, 3}, {"\f", 2, 3},
		{"\x01", 6, 7}, {"\x00", 6, 7}, {"\u2028", 6, 7}, {"\u2029", 6, 7}, {"<", 6, 6}, {">", 6, 6}, {"&", 6, 6},
		{"\x7f", 1, 1}, {"\u0085", 2, 2}, {"\u009f", 2, 2}, {"a", 1, 1}, {"é", 2, 2}, {"漢", 3, 3},
		{"\xff", 3, 3}, {"a\r\nb", 6, 8},
	}
	for _, f := range fixed {
		if got, wire := TextBytes(f.s), wireText(t, f.s); got != f.text || got != wire {
			t.Errorf("TextBytes(%q) = %d, the wire's %d, want %d", f.s, got, wire, f.text)
		}
		if got, wire := MemberBytes(f.s), wireMember(t, f.s); got != f.member || got != wire {
			t.Errorf("MemberBytes(%q) = %d, the wire's %d, want %d", f.s, got, wire, f.member)
		}
	}
	runes := measuredRunes()
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		var b strings.Builder
		for range r.IntN(40) {
			b.WriteString(runes[r.IntN(len(runes))])
		}
		s := b.String()
		if got, wire := TextBytes(s), wireText(t, s); got != wire {
			t.Fatalf("TextBytes(%q) = %d, the wire's %d", s, got, wire)
		}
		if got, wire := MemberBytes(s), wireMember(t, s); got != wire {
			t.Fatalf("MemberBytes(%q) = %d, the wire's %d", s, got, wire)
		}
	}
}

// textTool answers blocks of text.
func textTool(name string, blocks ...Block) Tool {
	return toolOf(name, func(context.Context, Call) (Result, error) { return Result{Blocks: blocks}, nil })
}

// An answer of exactly MaxAnswerBytes is answered, one byte over is
// internal_error, logged at error, the answer never sent; a structured
// answer is measured by its JSON text.
func TestTheAnswersBoundIsExact(t *testing.T) {
	member, _ := encode(map[string]string{"k": ""})
	quotes := strings.Repeat(`"`, (MaxAnswerBytes-TextBytes(string(member)))/MemberBytes(`"`))
	pad := strings.Repeat("a", MaxAnswerBytes-TextBytes(string(member))-MemberBytes(quotes))
	at := toolOf("structured_at", func(context.Context, Call) (Result, error) {
		return Result{Structured: map[string]string{"k": quotes + pad}}, nil
	})
	over := toolOf("structured_over", func(context.Context, Call) (Result, error) {
		return Result{Structured: map[string]string{"k": quotes + pad + "a"}}, nil
	})
	s := serveMCP(t, setup{options: func(o *Options) {
		o.Tools = append(o.Tools, textTool("text_at", Block{Text: strings.Repeat("<", MaxAnswerBytes/6)}),
			textTool("text_over", Block{Text: strings.Repeat("a", MaxAnswerBytes/2)}, Block{Text: strings.Repeat("a", MaxAnswerBytes/2+1)}),
			at, over)
	}})
	for _, name := range []string{"text_at", "structured_at"} {
		if r := result(t, s.call(t, Pattern, name, nil)); r["isError"] == true {
			t.Errorf("%s at the bound = %v, want its answer", name, r)
		}
	}
	for _, name := range []string{"text_over", "structured_over"} {
		r := result(t, s.call(t, Pattern, name, nil))
		if member := errorOf(t, r); member["code"] != "internal_error" || member["request_id"] == "" {
			t.Errorf("%s over the bound = %v, want internal_error with the request id", name, r)
		}
	}
	if n := countLogs(t, s.logs, "MCP answer over its bound", "ERROR"); n != 2 {
		t.Errorf("%d answers over the bound logged at error, want 2", n)
	}
	if e := s.logs.find(t, "MCP answer over its bound"); e["bytes"] != float64(MaxAnswerBytes+1) || e["tool"] != "text_over" {
		t.Errorf("the log has %v, want text_over's %d bytes", e, MaxAnswerBytes+1)
	}
}

// countLogs is how many lines of msg the log has at level.
func countLogs(t *testing.T, logs *logBuffer, msg, level string) int {
	t.Helper()
	n := 0
	for _, e := range logs.entries(t) {
		if e["msg"] == msg && e["level"] == level {
			n++
		}
	}
	return n
}

// fitAnswer is a list's answer as S2's list builds one: items, and a
// cursor after the last, whose length follows that item's.
type fitAnswer struct {
	Items      []string `json:"items"`
	NextCursor *string  `json:"next_cursor"`
}

// fitOf is the build of Fit over items, recording each k asked.
func fitOf(items []string, asked *[]int) func(k int) any {
	return func(k int) any {
		*asked = append(*asked, k)
		a := fitAnswer{Items: items[:k]}
		if k < len(items) {
			cursor := base64.RawURLEncoding.EncodeToString([]byte(items[k]))
			a.NextCursor = &cursor
		}
		return a
	}
}

// Fit answers n items when they fit, or k that fit with k+1 that do not,
// measured as the answer is: members of quotes count twice escaped. Its
// candidates gallop then bisect, none past the first over, at most
// 2 log₂ n + 2 of them.
func TestFitHoldsTheAnswersMeasure(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	shapes := []struct {
		name string
		item func(i int) string
		n    int
	}{
		{"short items, all fit", func(i int) string { return fmt.Sprintf("Page %d.md", i) }, 300},
		{"quotes and newlines", func(int) string { return strings.Repeat("\"\n", 100) }, 1000},
		{"HTML's characters", func(int) string { return strings.Repeat("<&>", 50) }, 1000},
		{"lengths at random", func(int) string { return strings.Repeat("é\"", 1+r.IntN(1400)) }, 1000},
		{"one item", func(int) string { return "only" }, 1},
		{"none", func(int) string { return "" }, 0},
		{"an item over the bound", func(int) string { return strings.Repeat("a", MaxAnswerBytes) }, 5},
	}
	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			items := make([]string, sh.n)
			for i := range items {
				items[i] = sh.item(i)
			}
			var asked []int
			build := fitOf(items, &asked)
			res, k, err := Fit(sh.n, build)
			if err != nil {
				t.Fatal(err)
			}
			text := string(res.Structured.(json.RawMessage))
			if k < 0 || k > sh.n || (k > 0 && TextBytes(text) > MaxAnswerBytes) {
				t.Fatalf("Fit = %d items of %d bytes, want at most %d bytes", k, TextBytes(text), MaxAnswerBytes)
			}
			measured := asked
			want, _ := encode(build(k))
			if string(want) != text {
				t.Errorf("Fit's answer is not build(%d)'s", k)
			}
			if k < sh.n {
				next, _ := encode(build(k + 1))
				if TextBytes(string(next)) <= MaxAnswerBytes {
					t.Errorf("Fit = %d items, but %d fit", k, k+1)
				}
			}
			if bound := 2*int(math.Log2(float64(max(sh.n, 1)))) + 2; len(measured) > bound {
				t.Errorf("Fit measured %d candidates %v, want at most %d", len(measured), measured, bound)
			}
			firstOver := sh.n
			for _, c := range measured {
				if text, _ := encode(build(c)); TextBytes(string(text)) > MaxAnswerBytes {
					firstOver = c
					break
				}
			}
			for _, c := range measured {
				if c > firstOver {
					t.Errorf("Fit measured %d items, past the first over, %d (%v)", c, firstOver, measured)
				}
			}
		})
	}
}

// A tool that fills its answer through Fit is never over the bound, even
// of members that the JSON's length would count at half (M9/P1 design
// 3.4.4, B-I1).
func TestATooLongListIsFitNotRefused(t *testing.T) {
	items := make([]string, 1000)
	for i := range items {
		items[i] = strings.Repeat(`"`, 200)
	}
	list := toolOf("list", func(context.Context, Call) (Result, error) {
		var asked []int
		res, _, err := Fit(len(items), fitOf(items, &asked))
		return res, err
	})
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, list) }})
	r := result(t, s.call(t, Pattern, "list", nil))
	if r["isError"] == true {
		t.Fatalf("a list of quotes = %v, want its items", r)
	}
	if n := TextBytes(toolText(t, r)); n > MaxAnswerBytes {
		t.Errorf("the answer holds %d bytes, over the bound", n)
	}
}

// A structured answer is one encoding: its text block and its
// structuredContent are the same JSON, <, > and & as they are in the text
// the model reads; a text tool's blocks have no structuredContent; an
// image and a link are the SDK's blocks.
func TestTheStructuredResultKeepsHTMLCharacters(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G'}
	s := serveMCP(t, setup{options: func(o *Options) {
		o.Tools = append(o.Tools,
			toolOf("html", func(context.Context, Call) (Result, error) {
				return Result{Structured: map[string]string{"path": "a<b>&c.md", "line": "x\ny\"z"}}, nil
			}),
			textTool("read", Block{Text: `{"total_lines":1}`}, Block{Text: "a <b> & c"},
				Block{Image: &Image{Data: png, MIME: "image/png"}},
				Block{Link: &Link{URI: "http://wiki.example/api/v0/assets/x/content?b=1&e=2", Name: "a.pdf", MIME: "application/pdf", Size: 10}}))
	}})
	r := result(t, s.call(t, Pattern, "html", nil))
	text := toolText(t, r)
	if !strings.Contains(text, `"a<b>&c.md"`) || strings.Contains(text, `\u003c`) {
		t.Errorf("the text block = %s, want <, > and & as they are", text)
	}
	var fromText, structured map[string]any
	_ = json.Unmarshal([]byte(text), &fromText)
	structured, _ = r["structuredContent"].(map[string]any)
	if fromText["path"] != "a<b>&c.md" || fromText["line"] != "x\ny\"z" || structured["path"] != fromText["path"] || structured["line"] != fromText["line"] {
		t.Errorf("text %v and structuredContent %v, want the same JSON", fromText, structured)
	}
	r = result(t, s.call(t, Pattern, "read", nil))
	if _, ok := r["structuredContent"]; ok {
		t.Errorf("a text tool's answer has structuredContent: %v", r)
	}
	content := r["content"].([]any)
	image, link := content[2].(map[string]any), content[3].(map[string]any)
	if toolText(t, r) != `{"total_lines":1}` || content[1].(map[string]any)["text"] != "a <b> & c" ||
		image["type"] != "image" || image["mimeType"] != "image/png" || image["data"] != base64.StdEncoding.EncodeToString(png) ||
		link["type"] != "resource_link" || link["uri"] != "http://wiki.example/api/v0/assets/x/content?b=1&e=2" ||
		link["name"] != "a.pdf" || link["mimeType"] != "application/pdf" || link["size"] != float64(10) {
		t.Errorf("the blocks = %v, want the text, the image and the link", content)
	}
}

// A result neither structured nor of blocks, both, or a block of two
// kinds is the tool's fault: internal_error, logged.
func TestAMalformedResultIsInternal(t *testing.T) {
	results := map[string]Result{
		"neither":   {},
		"both":      {Structured: map[string]any{}, Blocks: []Block{{Text: "a"}}},
		"two_kinds": {Blocks: []Block{{Text: "a", Image: &Image{MIME: "image/png"}}}},
		"no_kind":   {Blocks: []Block{{}}},
		"unencoded": {Structured: map[string]any{"f": func() {}}},
	}
	s := serveMCP(t, setup{options: func(o *Options) {
		for name, res := range results {
			o.Tools = append(o.Tools, toolOf(name, func(context.Context, Call) (Result, error) { return res, nil }))
		}
	}})
	for name := range results {
		r := result(t, s.call(t, Pattern, name, nil))
		if member := errorOf(t, r); member["code"] != "internal_error" {
			t.Errorf("%s = %v, want internal_error", name, r)
		}
	}
	if n := countLogs(t, s.logs, "MCP tool failed", "ERROR"); n != len(results) {
		t.Errorf("%d malformed results logged at error, want %d", n, len(results))
	}
}

// lockProblem is a problem that names a lock, as page.locked will.
type lockProblem struct{ problem }

func (lockProblem) ProblemLock() (pageID, userID uuid.UUID, displayName string, ok bool) {
	return visible, alice, "Alice", true
}

// Each row of the error table (M9/P1 design 3.3.6): a problem's code and
// detail, its fields and retry; a ToolError's words and members; a lock;
// the client gone, the deadline, any other fault; never a protocol error,
// the text and structuredContent.error's message the same.
func TestTheToolErrorsAreTheTables(t *testing.T) {
	notFound := problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page."}
	invalid := problem{status: http.StatusUnprocessableEntity, code: "validation_failed", detail: "The request has invalid values.",
		fields: []error{fieldProblem{"path", "invalid_format", "segment 2 must not start or end with white space"}}}
	busy := problem{status: http.StatusServiceUnavailable, code: "server_busy", detail: "The server is busy.", retry: 1500 * time.Millisecond}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	tests := []struct {
		name   string
		ctx    context.Context
		err    error
		text   string
		member map[string]any
		log    string // msg:level
	}{
		{"a problem", context.Background(), notFound, "page.not_found: No such page.",
			map[string]any{"code": "page.not_found", "message": "page.not_found: No such page."}, ""},
		{"its fields", context.Background(), fmt.Errorf("read: %w", invalid),
			"validation_failed: The request has invalid values. (path: invalid_format: segment 2 must not start or end with white space)",
			map[string]any{"code": "validation_failed", "message": "validation_failed: The request has invalid values. (path: invalid_format: segment 2 must not start or end with white space)",
				"fields": []any{map[string]any{"field": "path", "code": "invalid_format", "message": "segment 2 must not start or end with white space"}}}, ""},
		{"its retry", context.Background(), busy, "server_busy: The server is busy. Try again in 2 s.",
			map[string]any{"code": "server_busy", "message": "server_busy: The server is busy. Try again in 2 s.", "retry_after_seconds": float64(2)}, ""},
		{"a tool's words", context.Background(), &ToolError{Err: notFound, Text: "page.not_found: no page at Guide; Guide.md is a page.",
			Extra: map[string]any{"hint": "Guide.md", "code": "overwritten"}}, "page.not_found: no page at Guide; Guide.md is a page.",
			map[string]any{"code": "page.not_found", "message": "page.not_found: no page at Guide; Guide.md is a page.", "hint": "Guide.md"}, ""},
		{"a lock", context.Background(), lockProblem{problem{status: http.StatusConflict, code: "page.locked", detail: "Alice is editing it."}},
			"page.locked: Alice is editing it.", map[string]any{"code": "page.locked", "message": "page.locked: Alice is editing it.",
				"lock": map[string]any{"page_id": visible.String(), "user_id": alice.String(), "display_name": "Alice"}}, ""},
		{"the client gone", cancelled, fmt.Errorf("read: %w", context.Canceled), "internal_error: the client went away",
			map[string]any{"code": "internal_error", "message": "internal_error: the client went away", "request_id": ""}, "MCP client went away:DEBUG"},
		{"the deadline", expired, fmt.Errorf("read: %w", context.DeadlineExceeded), "internal_error: the call ran out of time; request id ",
			map[string]any{"code": "internal_error", "message": "internal_error: the call ran out of time; request id ", "request_id": ""},
			"MCP call deadline exceeded:WARN"},
		{"another fault", context.Background(), errors.New("database is down"), "internal_error: the server failed; request id ",
			map[string]any{"code": "internal_error", "message": "internal_error: the server failed; request id ", "request_id": ""},
			"MCP tool failed:ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := &logBuffer{}
			s := New(Options{Credentials: credentialsOf, Notebooks: &fakeNotebooks{}, RequestTimeout: time.Second, MaxBodyBytes: 1,
				Logger: jsonLogger(logs)})
			res, code := s.toolError(tt.ctx, "read", tt.err)
			b, _ := json.Marshal(res)
			var r map[string]any
			_ = json.Unmarshal(b, &r)
			if member := errorOf(t, r); toolText(t, r) != tt.text || !jsonEqual(member, tt.member) || code != tt.member["code"] {
				t.Errorf("toolError() = %s (%s), want %q and %v", b, code, tt.text, tt.member)
			}
			msg, level, _ := strings.Cut(tt.log, ":")
			if tt.log == "" && len(logs.entries(t)) != 0 || tt.log != "" && countLogs(t, logs, msg, level) != 1 {
				t.Errorf("the log has %v, want %q", logs.entries(t), tt.log)
			}
		})
	}
}

// jsonLogger logs JSON lines at debug to logs.
func jsonLogger(logs *logBuffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// jsonEqual compares two decoded JSON values.
func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// A handler that answers a result and an error is answered its error,
// and the SDK never sees both (it would log the result); a tool's error
// answer is within the bound too.
func TestAToolNeverAnswersAResultAndAnError(t *testing.T) {
	both := toolOf("both", func(context.Context, Call) (Result, error) {
		return Result{Structured: map[string]string{"secret": "the page's content"}}, problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page."}
	})
	long := toolOf("long", func(context.Context, Call) (Result, error) {
		return Result{}, &ToolError{Err: problem{status: http.StatusNotFound, code: "page.not_found", detail: "x"}, Text: strings.Repeat("<", MaxAnswerBytes/6)}
	})
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, both, long) }})
	r := result(t, s.call(t, Pattern, "both", nil))
	if member := errorOf(t, r); member["code"] != "page.not_found" {
		t.Errorf("both = %v, want its error", r)
	}
	r = result(t, s.call(t, Pattern, "long", nil))
	if member := errorOf(t, r); member["code"] != "internal_error" {
		t.Errorf("an error answer over the bound = %v, want internal_error", r)
	}
	time.Sleep(50 * time.Millisecond)
	for _, e := range s.logs.entries(t) {
		if strings.Contains(fmt.Sprint(e), "the page's content") || e["msg"] == "jsonrpc2 internal error" {
			t.Errorf("the log has %v", e)
		}
	}
}

// The arguments are checked against the schema before they are decoded:
// each kind of problem at REST's path, never its value; null for an
// optional argument and no arguments at all taken as absent; the account
// entry's notebook taken out, its id as REST parses one (M9/P1 design
// 3.3.3).
func TestTheArgumentsAreCheckedAsTheSchemaSays(t *testing.T) {
	type argsIn struct {
		Path  string `json:"path"`
		Limit *int   `json:"limit,omitempty"`
	}
	args := NewTool(Spec{Name: "args", Entry: BothEntries, Notebook: true}, func(_ context.Context, c Call, in argsIn) (Result, error) {
		out := map[string]any{"path": in.Path, "notebook": c.Notebook.String(), "limit": nil}
		if in.Limit != nil {
			out["limit"] = *in.Limit
		}
		return Result{Structured: out}, nil
	})
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, args) }})
	nb := Pattern + "?notebook=" + visible.String()
	const lead = "bad_request: The arguments do not match the tool's input schema."
	long := strings.Repeat("é", 200)
	many := map[string]any{"path": "a"}
	for i := range 17 {
		many[fmt.Sprintf("x%02d", i)] = 1
	}
	tests := []struct {
		name, path string
		body       string // the params' arguments member, "" for none
		text       string // the bad_request's, "" for a result
		result     map[string]any
	}{
		{"a required argument left out", nb, `"arguments":{}`, lead + " (path: required: is required)", nil},
		{"an argument it has not", nb, `"arguments":{"path":"a","secret":"TOP"}`, lead + " (secret: not_allowed: is not a property of this request)", nil},
		{"a name in another case", nb, `"arguments":{"Path":"a"}`, lead + " (Path: not_allowed: is not a property of this request; path: required: is required)", nil},
		{"the wrong type", nb, `"arguments":{"path":1,"limit":"TOP"}`, lead + " (limit: invalid_format: has the wrong type or format; path: invalid_format: has the wrong type or format)", nil},
		{"null for a required one", nb, `"arguments":{"path":null}`, lead + " (path: invalid_format: has the wrong type or format)", nil},
		{"null for an optional one", nb, `"arguments":{"path":"a","limit":null}`, "", map[string]any{"path": "a", "limit": nil, "notebook": visible.String()}},
		{"no arguments", nb, ``, lead + " (path: required: is required)", nil},
		{"null arguments", nb, `"arguments":null`, lead + " (path: required: is required)", nil},
		{"arguments that are no object", nb, `"arguments":"TOP"`, lead + " (arguments: invalid_format: has the wrong type or format)", nil},
		{"the notebook entry's notebook", nb, `"arguments":{"path":"a","notebook":"` + visible.String() + `"}`,
			lead + " (notebook: not_allowed: is not a property of this request)", nil},
		{"the account entry without its notebook", Pattern, `"arguments":{"path":"a"}`, lead + " (notebook: required: is required)", nil},
		{"the account entry's notebook", Pattern, `"arguments":{"notebook":"` + visible.String() + `","path":"a","limit":3}`, "",
			map[string]any{"path": "a", "limit": float64(3), "notebook": visible.String()}},
		{"its capitals", Pattern, `"arguments":{"notebook":"` + strings.ToUpper(visible.String()) + `","path":"a"}`, "",
			map[string]any{"path": "a", "limit": nil, "notebook": visible.String()}},
		{"no uuid", Pattern, `"arguments":{"notebook":"nope","path":"a"}`, "",
			map[string]any{"path": "a", "limit": nil, "notebook": uuid.UUID{}.String()}},
		{"a long name", nb, `"arguments":{"path":"a","` + long + `":1}`,
			lead + " (" + long[:252] + "…: not_allowed: is not a property of this request)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := `{"name":"args"}`
			if tt.body != "" {
				params = `{"name":"args",` + tt.body + `}`
			}
			r := result(t, s.post(t, context.Background(), tt.path, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+params+`}`, v2025()))
			if tt.text == "" {
				if r["isError"] == true || !jsonEqual(r["structuredContent"], tt.result) {
					t.Errorf("args = %v, want %v", r, tt.result)
				}
				return
			}
			member := errorOf(t, r)
			if text := toolText(t, r); text != tt.text || member["code"] != "bad_request" || member["message"] != text {
				t.Errorf("args = %q %v, want %q", text, member, tt.text)
			}
			if strings.Contains(toolText(t, r), "TOP") {
				t.Errorf("the answer repeats a value: %s", toolText(t, r))
			}
		})
	}
	// Seventeen problems: sixteen are told.
	r := result(t, s.call(t, nb, "args", many))
	if fields := errorOf(t, r)["fields"].([]any); len(fields) != 16 {
		t.Errorf("17 problems told as %d, want 16", len(fields))
	}
}
