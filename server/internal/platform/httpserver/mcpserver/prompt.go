package mcpserver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// Prompt is a prompt of the registry (M10 registers the first). Its
// arguments are strings, as MCP's PromptArgument are: no JSON schema.
type Prompt struct {
	Name, Title, Description string
	Arguments                []PromptArgument
	Get                      func(ctx context.Context, c Call, args map[string]string) (description string, messages []string, err error)
}

// PromptArgument is one named string a prompt takes.
type PromptArgument struct {
	Name, Description string
	Required          bool
}

// addPrompts registers prompts in both entries: M9 has none in its
// production wiring, and M10, which writes the first, adds what it needs
// to this registry (M9/P1 design 3.4.7).
func (s *Server) addPrompts(prompts []Prompt) {
	named := map[string]bool{}
	for _, p := range prompts {
		switch {
		case !toolName.MatchString(p.Name):
			panic(fmt.Sprintf("mcpserver: prompt %q: a name is 1 to 32 of a-z and _", p.Name))
		case named[p.Name]:
			panic(fmt.Sprintf("mcpserver: prompt %s is registered twice", p.Name))
		case p.Get == nil:
			panic(fmt.Sprintf("mcpserver: prompt %s has no handler", p.Name))
		}
		named[p.Name] = true
		args := make([]*mcp.PromptArgument, len(p.Arguments))
		for i, a := range p.Arguments {
			args[i] = &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required}
		}
		for _, e := range []Entry{NotebookEntry, AccountEntry} {
			s.serverOf(e).AddPrompt(&mcp.Prompt{Name: p.Name, Title: p.Title, Description: p.Description, Arguments: args}, s.promptHandler(p, e))
		}
	}
}

// promptHandler is p's prompts/get in entry e: a required argument left
// out is the SDK's -32602, as an unknown tool is; each message is the
// user's text; an error of the handler is logged and a JSON-RPC internal
// error, any error alike: a problem's classification (ProblemOf) and the
// answer's bound are M10's, which writes the first prompt (M9/P1 S1
// review B).
func (s *Server) promptHandler(p Prompt, e Entry) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		for _, a := range p.Arguments {
			if _, ok := req.Params.Arguments[a.Name]; a.Required && !ok {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf("prompt %s: missing the argument %q", p.Name, a.Name)}
			}
		}
		r := requestOf(ctx)
		call := Call{Entry: e, Notebook: r.notebook, Client: clientOf(ctx), UserID: r.cred.UserID, TokenID: r.cred.TokenID}
		description, messages, err := p.Get(ctx, call, req.Params.Arguments)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "MCP prompt failed", slog.String("prompt", p.Name),
				slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", err))
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
		}
		res := &mcp.GetPromptResult{Description: description, Messages: make([]*mcp.PromptMessage, len(messages))}
		for i, m := range messages {
			res.Messages[i] = &mcp.PromptMessage{Role: "user", Content: &mcp.TextContent{Text: m}}
		}
		return res, nil
	}
}
