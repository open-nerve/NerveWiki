package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// ToolError is a tool's error whose words its module writes (M9 design
// 4.7): Err is classified as any error is; Text, when set, is the
// answer's message, and Extra adds members to structuredContent.error.
type ToolError struct {
	Err   error
	Text  string
	Extra map[string]any
}

func (e *ToolError) Error() string { return e.Err.Error() }
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
	p := a.Problem
	text := p.Code + ": " + p.Detail + fieldsText(p.Errors)
	if a.Retry > 0 {
		text += fmt.Sprintf(" Try again in %d s.", retrySeconds(a))
	}
	member := errorMember{}
	var te *ToolError
	if errors.As(err, &te) {
		if te.Text != "" {
			text = te.Text
		}
		for k, v := range te.Extra {
			member[k] = v
		}
	}
	member["code"], member["message"] = p.Code, text
	if len(p.Errors) > 0 {
		member["fields"] = p.Errors
	}
	if p.Lock != nil {
		member["lock"] = p.Lock
	}
	if len(p.Locks) > 0 {
		member["locks"] = p.Locks
	}
	if a.Retry > 0 {
		member["retry_after_seconds"] = retrySeconds(a)
	}
	return s.errorResult(ctx, name, p.Code, text, member)
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
// message still saying what is wrong.
func argumentsError(e *bodyshape.Error) (*mcp.CallToolResult, string) {
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
	res := errorAnswer(text, errorMember{"code": httpserver.CodeBadRequest, "message": text, "fields": fields})
	if answerBytes(res) > MaxAnswerBytes {
		res = errorAnswer(lead, errorMember{"code": httpserver.CodeBadRequest, "message": lead})
	}
	return res, httpserver.CodeBadRequest
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
	id := httpserver.RequestID(ctx)
	text := httpserver.CodeInternal + ": " + message
	if message != internalGone {
		text += "; request id " + id
	}
	return errorAnswer(text, errorMember{"code": httpserver.CodeInternal, "message": text, "request_id": id})
}

// errorResult is the error answer of text and member, within
// MaxAnswerBytes, and its code: over the bound, a fault of the tool's,
// internal_error.
func (s *Server) errorResult(ctx context.Context, name, code, text string, member errorMember) (*mcp.CallToolResult, string) {
	res := errorAnswer(text, member)
	if n := answerBytes(res); n > MaxAnswerBytes {
		return s.overBound(ctx, name, n)
	}
	return res, code
}

// errorAnswer is the result of an error: isError, text, and member as
// structuredContent.error.
func errorAnswer(text string, member errorMember) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: map[string]any{"error": member},
	}
}

// overBound is the answer of a tool's answer of n bytes over
// MaxAnswerBytes: a fault, logged, its answer never sent.
func (s *Server) overBound(ctx context.Context, name string, n int) (*mcp.CallToolResult, string) {
	s.logger.LogAttrs(ctx, slog.LevelError, "MCP answer over its bound",
		slog.String("tool", name), slog.Int("bytes", n), slog.String("request_id", httpserver.RequestID(ctx)))
	return s.internalError(ctx, internalFailed), httpserver.CodeInternal
}
