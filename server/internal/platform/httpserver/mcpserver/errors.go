package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// ToolError is a tool's error whose words its module writes (M9 design
// 4.7): Err is classified as any error is; Text, when set, is the
// answer's message, and Extra adds members to structuredContent.error:
// those the problem has not, and lock, locks and locks_count over the
// problem's, which only the module can give paths; code, message, fields
// and retry_after_seconds are always mcpserver's, as the text says them
// (M9/P2 design 3.10.1).
type ToolError struct {
	Err   error
	Text  string
	Extra map[string]any
	// Fit, when set, builds the error with k of its N items: mcpserver
	// answers the largest k it finds whose whole answer is within
	// MaxAnswerBytes; Text and Extra are not read then.
	Fit *ErrorFit
	// At, when set, is where in the call the error is, as a batch's op
	// "ops[3] (move)" (M9/P3 design 3.9.2): mcpserver writes it after the
	// code in whatever words answer, "<code>: <At>: <the rest>", the
	// module's Text, each of Fit's and the problem's own, so that every k
	// of Fit carries it. A module's words that do not start with "<code>: "
	// are its fault: At and the code go before them, an error line says so.
	At string
}

// ErrorFit is an error of a list, or of parts that may not all fit (M9/P2
// design 3.10): Build(k) gives the text and the extra members with k of
// the N items. mcpserver searches k as Fit does; Build(0) over the bound is
// internal_error.
type ErrorFit struct {
	N     int
	Build func(k int) (text string, extra map[string]any)
}

// MaxErrorLocks is how many locks an error's structuredContent lists at
// most (M9/P2 design 3.10): the rest are counted in locks_count.
const MaxErrorLocks = 20

// reservedMembers are the members of structuredContent.error that Extra
// never sets: what the text says, the answer's own.
func reservedMembers() map[string]bool {
	return map[string]bool{"code": true, "message": true, "fields": true, "retry_after_seconds": true, "duplicate_of": true}
}

func (e *ToolError) Error() string {
	if e.Err == nil {
		return "mcpserver: a ToolError without its Err"
	}
	return e.Err.Error()
}
func (e *ToolError) Unwrap() error { return e.Err }

// outcomeOK is a call's outcome in its log line when it answered a
// result; any other outcome is the code of its error.
const outcomeOK = "ok"

// The messages of a fault's internal_error: the request id goes with
// them, for the logs.
const (
	internalGone     = "the client went away"
	internalDeadline = "the call ran out of time"
	internalFailed   = "the server failed"
)

// errorMember is structuredContent.error: the code and the message the
// text block holds, and the problem's members.
type errorMember map[string]any

// toolError is the answer of err, a tool's error, met in ctx, the call's
// (M9/P1 design 3.3.6): never a protocol error, always a result with
// isError, its text and structuredContent.error the one classification of
// REST's; and the code it answers.
//
// A ToolError without its Err has nothing to classify: a fault of the
// tool's, its Text lost, its log's error saying so.
func (s *Server) toolError(ctx context.Context, name string, err error) (*mcp.CallToolResult, string) {
	a := httpserver.ProblemOf(ctx, err)
	attrs := []slog.Attr{slog.String("tool", name), slog.String("request_id", httpserver.RequestID(ctx))}
	switch a.Fault {
	case httpserver.FaultGone:
		s.logger.LogAttrs(ctx, slog.LevelDebug, "MCP client went away", attrs...)
		return s.internalError(ctx, internalGone), httpserver.CodeInternal
	case httpserver.FaultDeadline:
		s.logger.LogAttrs(ctx, slog.LevelWarn, "MCP call deadline exceeded", append(attrs, slog.Any("error", err))...)
		return s.internalError(ctx, internalDeadline), httpserver.CodeInternal
	case httpserver.FaultUnknown:
		s.logger.LogAttrs(ctx, slog.LevelError, "MCP tool failed", append(attrs, slog.Any("error", err))...)
		return s.internalError(ctx, internalFailed), httpserver.CodeInternal
	}
	res, n, stray, merr := problemAnswer(a, err)
	if stray {
		s.logger.LogAttrs(ctx, slog.LevelError, "MCP error's words start without their code", attrs...)
	}
	switch {
	case merr != nil:
		s.logger.LogAttrs(ctx, slog.LevelError, "MCP tool failed", append(attrs,
			slog.Any("error", fmt.Errorf("encoding its error's member: %w", merr)))...)
		return s.internalError(ctx, internalFailed), httpserver.CodeInternal
	case n > MaxAnswerBytes:
		return s.overBound(ctx, name, n)
	}
	return res, a.Problem.Code
}

// problemAnswer is the answer of err, a tool's error that a classified as
// a problem, and its bytes as MaxAnswerBytes counts them: the text, the
// module's or the problem's code, detail, fields and retry, a ToolError's
// At after its code (placed); the member, the problem's and then Extra's
// (membersOf); for a ToolError with Fit, the answer of the most items it
// finds within the bound, or of none when even that is over it. stray
// tells that a module's words with At did not start with their code. The
// error is a member that does not encode.
func problemAnswer(a httpserver.Answer, err error) (res *mcp.CallToolResult, n int, stray bool, merr error) {
	text := a.Problem.Code + ": " + a.Problem.Detail + fieldsText(a.Problem.Errors)
	if a.Retry > 0 {
		text += fmt.Sprintf(" Try again in %d s.", retrySeconds(a))
	}
	var te *ToolError
	at := ""
	if errors.As(err, &te) {
		at = te.At
	}
	answerOf := func(words string, extra map[string]any) (*mcp.CallToolResult, int, error) {
		if words == "" {
			words = text
		}
		words, wrong := placed(a.Problem.Code, at, words)
		stray = stray || wrong
		res, err := errorAnswer(words, membersOf(a, words, extra))
		if err != nil {
			return nil, 0, err
		}
		return res, answerBytes(res), nil
	}
	if te == nil {
		res, n, merr = answerOf("", nil)
		return res, n, stray, merr
	}
	if te.Fit == nil {
		res, n, merr = answerOf(te.Text, te.Extra)
		return res, n, stray, merr
	}
	type measured struct {
		res *mcp.CallToolResult
		n   int
	}
	m, _, _, ferr := fitItems(te.Fit.N, func(k int) (measured, bool, error) {
		res, n, err := answerOf(te.Fit.Build(k))
		return measured{res, n}, n <= MaxAnswerBytes, err
	})
	return m.res, m.n, stray, ferr
}

// placed is words with at after their code: "<code>: <at>: <the rest>";
// words as they are for no at. Words that do not start with "<code>: " get
// both before them, and wrong tells it.
func placed(code, at, words string) (out string, wrong bool) {
	if at == "" {
		return words, false
	}
	lead := code + ": "
	if rest, ok := strings.CutPrefix(words, lead); ok {
		return lead + at + ": " + rest, false
	}
	return lead + at + ": " + words, true
}

// membersOf is structuredContent.error of a problem answered with text:
// its code, the message and the problem's members, its locks at most
// MaxErrorLocks with locks_count their number; then extra's, but for the
// members the answer itself owns (M9/P2 design 3.10.1).
func membersOf(a httpserver.Answer, text string, extra map[string]any) errorMember {
	p := a.Problem
	member := errorMember{"code": p.Code, "message": text}
	if len(p.Errors) > 0 {
		member["fields"] = p.Errors
	}
	if p.DuplicateOf != nil {
		member["duplicate_of"] = p.DuplicateOf
	}
	if p.Lock != nil {
		member["lock"] = p.Lock
	}
	if len(p.Locks) > 0 {
		member["locks"] = p.Locks[:min(len(p.Locks), MaxErrorLocks)]
		member["locks_count"] = len(p.Locks)
	}
	if a.Retry > 0 {
		member["retry_after_seconds"] = retrySeconds(a)
	}
	reserved := reservedMembers()
	for k, v := range extra {
		if !reserved[k] {
			member[k] = v
		}
	}
	return member
}

// ErrorAnswerBytes is the code mcpserver answers err with, a tool's
// error, and the size of that answer (M9/P2 design 3.10.5): err takes
// the path of a tool's error, Fit's search and the members' writing
// included, so a module's test measures its errors as they are answered.
// When even k = 0 is over the bound, code is internal_error and bytes is
// the size of the answer that went over, which the over-bound line logs:
// the internal_error answer itself carries the call's request id, which
// err does not have.
func ErrorAnswerBytes(err error) (code string, bytes int) {
	a := httpserver.ProblemOf(context.Background(), err)
	if a.Fault == httpserver.FaultNone {
		_, n, _, merr := problemAnswer(a, err)
		switch {
		case merr == nil && n <= MaxAnswerBytes:
			return a.Problem.Code, n
		case merr == nil:
			return httpserver.CodeInternal, n
		}
	}
	return httpserver.CodeInternal, answerBytes(internalAnswer("", internalFailed))
}

// retrySeconds is a's retry as Retry-After writes it: whole seconds,
// rounded up.
func retrySeconds(a httpserver.Answer) int {
	return int(math.Ceil(a.Retry.Seconds()))
}

// argumentsError is the answer of arguments the tool's schema does not
// take (M9/P1 design 2.1 item 3): bad_request, the problems at REST's
// paths, which never repeat a value; the arguments themselves, when they
// are no object, at "arguments". Sixteen paths of 256 bytes, each byte of
// the longest escape, are more than MaxAnswerBytes in the text and the
// member together: such an answer goes without them, the code and the
// message still saying what is wrong, and through errorResult's bound as
// every error's answer does.
func (s *Server) argumentsError(ctx context.Context, name string, e *bodyshape.Error) (*mcp.CallToolResult, string) {
	fields := make([]httpserver.FieldError, 0, len(e.Fields))
	for _, f := range e.Fields {
		field := f.Field
		if field == "" {
			field = "arguments"
		}
		fields = append(fields, httpserver.FieldError{Field: field, Code: f.Code, Message: f.Error()})
	}
	const lead = httpserver.CodeBadRequest + ": The arguments do not match the tool's input schema."
	text := lead + fieldsText(fields)
	if res, err := errorAnswer(text, errorMember{"code": httpserver.CodeBadRequest, "message": text, "fields": fields}); err == nil &&
		answerBytes(res) <= MaxAnswerBytes {
		return res, httpserver.CodeBadRequest
	}
	return s.errorResult(ctx, name, httpserver.CodeBadRequest, lead, errorMember{"code": httpserver.CodeBadRequest, "message": lead})
}

// fieldsText is fields as an error's text adds them: " (<field>: <code>:
// <message>; …)", nothing for none.
func fieldsText(fields []httpserver.FieldError) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.Field + ": " + f.Code + ": " + f.Message
	}
	return " (" + strings.Join(parts, "; ") + ")"
}

// internalError is internal_error of a fault: message, and the request
// id the logs carry.
func (s *Server) internalError(ctx context.Context, message string) *mcp.CallToolResult {
	return internalAnswer(httpserver.RequestID(ctx), message)
}

// internalAnswer is internal_error's answer of message for the request id.
func internalAnswer(id, message string) *mcp.CallToolResult {
	text := httpserver.CodeInternal + ": " + message
	if message != internalGone {
		text += "; request id " + id
	}
	// A member of strings alone always encodes: errorAnswer's error is
	// never set.
	res, _ := errorAnswer(text, errorMember{"code": httpserver.CodeInternal, "message": text, "request_id": id})
	return res
}

// errorResult is the error answer of text and member, within
// MaxAnswerBytes, and its code: a member that does not encode (a tool's
// Extra) or an answer over the bound is a fault of the tool's,
// internal_error at once, never a result the SDK could not write.
func (s *Server) errorResult(ctx context.Context, name, code, text string, member errorMember) (*mcp.CallToolResult, string) {
	res, err := errorAnswer(text, member)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "MCP tool failed", slog.String("tool", name),
			slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", fmt.Errorf("encoding its error's member: %w", err)))
		return s.internalError(ctx, internalFailed), httpserver.CodeInternal
	}
	if n := answerBytes(res); n > MaxAnswerBytes {
		return s.overBound(ctx, name, n)
	}
	return res, code
}

// errorAnswer is the result of an error: isError, text, and member as
// structuredContent.error, encoded once here, so that what the SDK sends
// is what was measured, and a member that cannot be written is found
// before the SDK has it.
func errorAnswer(text string, member errorMember) (*mcp.CallToolResult, error) {
	structured, err := encode(map[string]any{"error": member})
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: structured,
	}, nil
}

// overBound is the answer of a tool's answer of n bytes over
// MaxAnswerBytes: a fault, logged, its answer never sent.
func (s *Server) overBound(ctx context.Context, name string, n int) (*mcp.CallToolResult, string) {
	s.logger.LogAttrs(ctx, slog.LevelError, "MCP answer over its bound",
		slog.String("tool", name), slog.Int("bytes", n), slog.String("request_id", httpserver.RequestID(ctx)))
	return s.internalError(ctx, internalFailed), httpserver.CodeInternal
}

// methodError is the JSON-RPC error of err, met by a method that is no
// tool's call (the handshake's instructions, M9/P1 design 3.15.2):
// classified and logged as a tool's error is, an internal error that
// names the request; a problem, which no such method answers today, in
// its own words.
func (s *Server) methodError(ctx context.Context, method string, err error) error {
	a := httpserver.ProblemOf(ctx, err)
	id := httpserver.RequestID(ctx)
	attrs := []slog.Attr{slog.String("method", method), slog.String("request_id", id)}
	message := internalFailed
	switch a.Fault {
	case httpserver.FaultNone:
		return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: a.Problem.Code + ": " + a.Problem.Detail}
	case httpserver.FaultGone:
		s.logger.LogAttrs(ctx, slog.LevelDebug, "MCP client went away", attrs...)
		return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: httpserver.CodeInternal + ": " + internalGone}
	case httpserver.FaultDeadline:
		s.logger.LogAttrs(ctx, slog.LevelWarn, "MCP call deadline exceeded", append(attrs, slog.Any("error", err))...)
		message = internalDeadline
	default:
		s.logger.LogAttrs(ctx, slog.LevelError, "MCP method failed", append(attrs, slog.Any("error", err))...)
	}
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: httpserver.CodeInternal + ": " + message + "; request id " + id}
}
