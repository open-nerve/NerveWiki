package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"
)

// locksOf is a problem naming n locks, page.locked of a deletion as page's
// will be.
type locksOf struct {
	problem
	n int
}

func (l locksOf) ProblemLocks(each func(pageID, userID uuid.UUID, displayName string)) {
	for i := range l.n {
		each(uuid.MustParse(fmt.Sprintf("01920000-0000-7000-8000-%012d", i)), alice, "Alice")
	}
}

// decodedAnswer is toolError's answer of err, decoded as a client reads it.
func decodedAnswer(t *testing.T, s *Server, err error) map[string]any {
	t.Helper()
	res, _ := s.toolError(context.Background(), "write", err)
	b, _ := json.Marshal(res)
	var r map[string]any
	if jerr := json.Unmarshal(b, &r); jerr != nil {
		t.Fatal(jerr)
	}
	return r
}

// Extra adds the members a problem has not, and may set lock, locks and
// locks_count over the problem's, which only the module can give paths;
// code, message, fields and retry_after_seconds stay mcpserver's, as the
// text says them (M9/P2 design 3.10.1).
func TestExtraOverridesTheProblemsMembers(t *testing.T) {
	locked := lockProblem{problem{status: http.StatusConflict, code: "page.locked", detail: "Alice is editing it.",
		fields: []error{fieldProblem{"path", "invalid_format", "is odd"}}, retry: time.Second}}
	withPath := map[string]any{"page_id": visible.String(), "path": "Guide.md", "user_id": alice.String(), "display_name": "Alice", "you": false}
	s := New(bare(discard()))
	r := decodedAnswer(t, s, &ToolError{Err: locked, Extra: map[string]any{
		"lock": withPath, "extra": 1, "code": "page.other", "message": "Something else.", "fields": []string{"x"}, "retry_after_seconds": 99,
	}})
	member := errorOf(t, r)
	text := "page.locked: Alice is editing it. (path: invalid_format: is odd) Try again in 1 s."
	want := map[string]any{"code": "page.locked", "message": text, "lock": withPath, "extra": float64(1), "retry_after_seconds": float64(1),
		"fields": []any{map[string]any{"field": "path", "code": "invalid_format", "message": "is odd"}}}
	if toolText(t, r) != text || !jsonEqual(member, want) {
		t.Errorf("the member = %v, the text %q; want %v", member, toolText(t, r), want)
	}
	// The members a problem has not are still mcpserver's alone.
	r = decodedAnswer(t, s, &ToolError{Err: problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page."},
		Extra: map[string]any{"fields": []string{"x"}, "retry_after_seconds": 99, "hint": "Guide.md"}})
	if member := errorOf(t, r); !jsonEqual(member, map[string]any{"code": "page.not_found", "message": "page.not_found: No such page.", "hint": "Guide.md"}) {
		t.Errorf("the member = %v, want the problem's, and the hint", member)
	}
}

// An error's locks are at most MaxErrorLocks, locks_count their number.
func TestTheLocksOfAnErrorAreBounded(t *testing.T) {
	s := New(bare(discard()))
	for n, want := range map[int]int{25: MaxErrorLocks, 3: 3} {
		member := errorOf(t, decodedAnswer(t, s, locksOf{problem{status: http.StatusConflict, code: "page.locked", detail: "Pages are being edited."}, n}))
		locks, _ := member["locks"].([]any)
		if len(locks) != want || member["locks_count"] != float64(n) {
			t.Errorf("%d locks: %d listed, locks_count %v; want %d and %d", n, len(locks), member["locks_count"], want, n)
		}
	}
}

// itemsError is a ToolError of n items of size bytes each, built with
// the first k: the text names k, items holds them.
func itemsError(n, size int, nothing bool) *ToolError {
	return &ToolError{Err: problem{status: http.StatusConflict, code: "page.locked", detail: "x"}, Fit: &ErrorFit{N: n,
		Build: func(k int) (string, map[string]any) {
			items := make([]string, k)
			for i := range items {
				items[i] = fmt.Sprintf("%03d", i) + strings.Repeat("i", size-3)
			}
			text := fmt.Sprintf("page.locked: %d of the items.", k)
			if nothing {
				text += strings.Repeat("a", MaxAnswerBytes)
			}
			return text, map[string]any{"items": items}
		}}}
}

// An error of items is answered with the most that fit, as Fit finds
// them: k items within MaxAnswerBytes, and k+1 over it unless k is all;
// one whose answer of no item is over the bound is internal_error, logged
// at error; a Build that panics is internal_error, as any tool's panic.
func TestAnErrorFitsItsItems(t *testing.T) {
	s := serveMCP(t, setup{options: func(o *Options) {
		o.Tools = append(o.Tools,
			toolOf("items", func(context.Context, Call) (Result, error) { return Result{}, itemsError(30, 4000, false) }),
			toolOf("few", func(context.Context, Call) (Result, error) { return Result{}, itemsError(3, 4000, false) }),
			toolOf("nothing", func(context.Context, Call) (Result, error) { return Result{}, itemsError(30, 4000, true) }),
			toolOf("panics", func(context.Context, Call) (Result, error) {
				return Result{}, &ToolError{Err: problem{status: http.StatusConflict, code: "page.locked", detail: "x"},
					Fit: &ErrorFit{N: 3, Build: func(int) (string, map[string]any) { panic("a builder's fault") }}}
			}))
	}})
	for name, n := range map[string]int{"items": 30, "few": 3} {
		r := result(t, s.call(t, Pattern, name, nil))
		member := errorOf(t, r)
		items, _ := member["items"].([]any)
		k := len(items)
		structured, _ := encode(r["structuredContent"])
		if bytes := TextBytes(toolText(t, r)) + TextBytes(string(structured)); member["code"] != "page.locked" || bytes > MaxAnswerBytes ||
			toolText(t, r) != fmt.Sprintf("page.locked: %d of the items.", k) {
			t.Errorf("%s = %d items in %d bytes, %q; want page.locked within the bound, its text of them", name, k, bytes, toolText(t, r))
		}
		if k == n {
			continue
		}
		text, extra := itemsError(n, 4000, false).Fit.Build(k + 1)
		if code, bytes := ErrorAnswerBytes(&ToolError{Err: problem{status: http.StatusConflict, code: "page.locked", detail: "x"},
			Text: text, Extra: extra}); code != "internal_error" {
			t.Errorf("%s: %d items fit, and %d do too (%d bytes): Fit's k is not the most", name, k, k+1, bytes)
		}
		if k == 0 {
			t.Errorf("%s fitted no item, want some of 4 KB", name)
		}
	}
	for _, name := range []string{"nothing", "panics"} {
		if member := errorOf(t, result(t, s.call(t, Pattern, name, nil))); member["code"] != "internal_error" {
			t.Errorf("%s = %v, want internal_error", name, member)
		}
	}
	if line := s.logs.find(t, "MCP answer over its bound"); line == nil || line["tool"] != "nothing" || line["level"] != "ERROR" {
		t.Errorf("the over-bound line = %v, want nothing's at error", line)
	}
	if line := s.logs.find(t, "MCP method panicked"); line == nil || line["tool"] != "panics" {
		t.Errorf("the panic line = %v, want panics's", line)
	}
}

// ErrorAnswerBytes is the answer's: its code and bytes are those of a
// tool's call that answers the error, for an error with Fit and one with
// Extra; of one over the bound, internal_error, and the bytes the
// over-bound line logs (the answer itself has the call's request id).
func TestErrorAnswerBytesIsTheAnswer(t *testing.T) {
	notFound := problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page."}
	errs := map[string]error{
		"a_problem": notFound,
		"an_extra": &ToolError{Err: lockProblem{problem{status: http.StatusConflict, code: "page.locked", detail: "Alice."}},
			Text: `page.locked: "<&>" is being edited.`, Extra: map[string]any{"lock": map[string]any{"path": "<&>.md", "you": true}}},
		"a_fit":      itemsError(30, 4000, false),
		"some_locks": locksOf{problem{status: http.StatusConflict, code: "linking.pages_locked", detail: "Pages."}, 25},
		"a_retry":    problem{status: http.StatusServiceUnavailable, code: "server_busy", detail: "Busy.", retry: 1500 * time.Millisecond},
		"over":       itemsError(30, 4000, true),
	}
	s := serveMCP(t, setup{options: func(o *Options) {
		for name, err := range errs {
			o.Tools = append(o.Tools, toolOf(name, func(context.Context, Call) (Result, error) { return Result{}, err }))
		}
	}})
	for name, err := range errs {
		r := result(t, s.call(t, Pattern, name, nil))
		code, bytes := ErrorAnswerBytes(err)
		member := errorOf(t, r)
		if name == "over" {
			line := s.logs.find(t, "MCP answer over its bound")
			if code != "internal_error" || member["code"] != code || line == nil || line["bytes"] != float64(bytes) {
				t.Errorf("over: ErrorAnswerBytes = %s, %d; the call %v, its line %v", code, bytes, member["code"], line)
			}
			continue
		}
		structured, _ := encode(r["structuredContent"])
		if wire := TextBytes(toolText(t, r)) + TextBytes(string(structured)); code != member["code"] || bytes != wire {
			t.Errorf("%s: ErrorAnswerBytes = %s, %d; the call answered %v in %d bytes", name, code, bytes, member["code"], wire)
		}
	}
}

// The longest path, 2,818 bytes of &, each six bytes in the answer: the
// errors that name it fit through Fit, each answering its own code within
// MaxAnswerBytes (M9/P2 design 3.10.3): page.not_found and its hint of
// another path, the hint left out; page.locked with lock.path; a deletion's
// 20 locks, each with its path. Without Fit, the first is over the bound.
func TestAnErrorOfTheLongestPathFits(t *testing.T) {
	segment := strings.Repeat("&", 255)
	path := strings.Repeat(segment+"/", 10) + segment + ".md"
	name := strings.Repeat("&", 100)
	if len(path) != 2818 {
		t.Fatalf("the path is %d bytes, want 2,818", len(path))
	}
	quoted := `"` + path + `"`
	notFound := problem{status: http.StatusNotFound, code: "page.not_found", detail: "Nothing."}
	hinted := &ToolError{Err: notFound, Fit: &ErrorFit{N: 1, Build: func(k int) (string, map[string]any) {
		text := "page.not_found: Nothing is at " + quoted + "."
		if k == 1 {
			text += " A page is at " + quoted + ": its path ends with .md."
		}
		return text, nil
	}}}
	lockOf := func(k int) map[string]any {
		lock := map[string]any{"page_id": visible.String(), "user_id": alice.String(), "display_name": name, "you": false}
		if k == 1 {
			lock["path"] = path
		}
		return lock
	}
	locked := &ToolError{Err: lockProblem{problem{status: http.StatusConflict, code: "page.locked", detail: "Locked."}},
		Fit: &ErrorFit{N: 1, Build: func(k int) (string, map[string]any) {
			return "page.locked: " + quoted + " is being edited by " + name + " in the web editor.", map[string]any{"lock": lockOf(k)}
		}}}
	deleted := &ToolError{Err: locksOf{problem{status: http.StatusConflict, code: "page.locked", detail: "Locked."}, MaxErrorLocks},
		Fit: &ErrorFit{N: MaxErrorLocks, Build: func(k int) (string, map[string]any) {
			locks := make([]map[string]any, k)
			for i := range locks {
				locks[i] = lockOf(1)
			}
			return "page.locked: " + quoted + " has 20 pages being edited in the web editor, by " + name + ", " + name + " and " + name + ".",
				map[string]any{"lock": lockOf(0), "locks": locks, "locks_count": MaxErrorLocks}
		}}}
	for what, tt := range map[string]struct {
		err  error
		code string
	}{
		"the hint":     {hinted, "page.not_found"},
		"lock.path":    {locked, "page.locked"},
		"twenty locks": {deleted, "page.locked"},
	} {
		if code, bytes := ErrorAnswerBytes(tt.err); code != tt.code || bytes > MaxAnswerBytes {
			t.Errorf("%s: %s in %d bytes, want %s within %d", what, code, bytes, tt.code, MaxAnswerBytes)
		}
	}
	text, _ := hinted.Fit.Build(1)
	if code, bytes := ErrorAnswerBytes(&ToolError{Err: notFound, Text: text}); code != "internal_error" {
		t.Errorf("the hint without Fit = %s in %d bytes, want internal_error: it is over the bound", code, bytes)
	}
}

// An error's place, At, follows its code in whatever words answer it (M9/P3
// design 3.9.2): the problem's own with its fields, the module's Text,
// each k of Fit's; the member's message is the same words. Words that
// start without their code get the code and At before them, and an error
// line says so.
func TestAnErrorsPlaceFollowsItsCode(t *testing.T) {
	notFound := problem{status: http.StatusNotFound, code: "page.not_found", detail: "No such page.",
		fields: []error{fieldProblem{"ops[3].from", "invalid_format", "is odd"}}}
	s := New(bare(discard()))
	for _, tt := range []struct {
		name string
		err  *ToolError
		want string
	}{
		{"the problem's own words", &ToolError{Err: notFound, At: "ops[3] (move)"},
			"page.not_found: ops[3] (move): No such page. (ops[3].from: invalid_format: is odd)"},
		{"the module's", &ToolError{Err: notFound, Text: `page.not_found: Nothing is at "A.md".`, At: "ops[3] (move)"},
			`page.not_found: ops[3] (move): Nothing is at "A.md".`},
		{"none", &ToolError{Err: notFound, Text: "page.not_found: Nothing."}, "page.not_found: Nothing."},
	} {
		r := decodedAnswer(t, s, tt.err)
		if toolText(t, r) != tt.want || errorOf(t, r)["message"] != tt.want {
			t.Errorf("%s: the text %q, the message %q; want %q", tt.name, toolText(t, r), errorOf(t, r)["message"], tt.want)
		}
	}
	for _, n := range []int{30, 3} {
		fit := itemsError(n, 4000, false)
		fit.At = "ops[0] (delete)"
		r := decodedAnswer(t, s, fit)
		items, _ := errorOf(t, r)["items"].([]any)
		text, extra := itemsError(n, 4000, false).Fit.Build(len(items))
		want := strings.Replace(text, "page.locked: ", "page.locked: ops[0] (delete): ", 1)
		if toolText(t, r) != want || len(items) == 0 {
			t.Errorf("%d items: %q, want %q", n, toolText(t, r), want)
		}
		if _, bytes := ErrorAnswerBytes(fit); bytes > MaxAnswerBytes {
			t.Errorf("%d items with their place: %d bytes, over the bound", n, bytes)
		}
		if k := len(items); k < n {
			if code, _ := ErrorAnswerBytes(&ToolError{Err: fit.Err, Text: want, Extra: extra}); code == "internal_error" {
				t.Errorf("%d items: the %d answered do not fit with their place", n, k)
			}
		}
	}
	served := serveMCP(t, setup{options: func(o *Options) {
		o.Tools = append(o.Tools, toolOf("stray", func(context.Context, Call) (Result, error) {
			return Result{}, &ToolError{Err: notFound, Text: "Nothing is here.", At: "ops[1] (edit)"}
		}))
	}})
	r := result(t, served.call(t, Pattern, "stray", nil))
	line := served.logs.find(t, "MCP error's words start without their code")
	if toolText(t, r) != "page.not_found: ops[1] (edit): Nothing is here." || line == nil || line["tool"] != "stray" ||
		line["level"] != "ERROR" {
		t.Errorf("words without their code = %q, the line %v; want the code and the place before them, an error line",
			toolText(t, r), line)
	}
}

// A module cannot inject or replace a duplicate target through ToolError.Extra.
type duplicateProblem struct {
	problem
	target *uuid.UUID
}

func (e duplicateProblem) ProblemDuplicateOf() *uuid.UUID { return e.target }

func TestDuplicateTargetIsAReservedProblemMember(t *testing.T) {
	for _, target := range []*uuid.UUID{nil, &visible} {
		e := duplicateProblem{problem: problem{status: http.StatusConflict, code: "page.source_duplicate", detail: "Source exists."}, target: target}
		member := errorOf(t, decodedAnswer(t, New(bare(discard())), &ToolError{Err: fmt.Errorf("move: %w", e), Extra: map[string]any{"duplicate_of": alice.String(), "op": 2}}))
		got, exists := member["duplicate_of"]
		if target == nil && exists || target != nil && got != target.String() {
			t.Errorf("duplicate_of = %v (present %v), want %v", got, exists, target)
		}
		if member["code"] != "page.source_duplicate" || member["op"] != float64(2) {
			t.Errorf("lost structured members: %v", member)
		}
	}
}
