package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A prompt double, the registry M10 builds on (M9/P1 design 3.4.7): its
// entries say prompts without listChanged, prompts/list lists it, and
// prompts/get reaches its handler with its arguments, a required one left
// out the SDK's -32602.
func TestThePromptDoubleIsListedAndReached(t *testing.T) {
	var got map[string]string
	var call Call
	ingest := Prompt{Name: "ingest", Title: "Ingest", Description: "Takes a source in.",
		Arguments: []PromptArgument{{Name: "source", Description: "The source.", Required: true}, {Name: "note"}},
		Get: func(_ context.Context, c Call, args map[string]string) (string, []string, error) {
			got, call = args, c
			return "Ingest " + args["source"], []string{"Read " + args["source"] + ".", "Then write."}, nil
		}}
	s := serveMCP(t, setup{options: func(o *Options) { o.Prompts = []Prompt{ingest} }})
	cs := s.connect(t, Pattern+"?notebook="+visible.String(), nil)
	if caps := cs.InitializeResult().Capabilities; caps.Prompts == nil || caps.Prompts.ListChanged {
		t.Errorf("capabilities %+v, want prompts without listChanged", caps)
	}
	list, err := cs.ListPrompts(context.Background(), nil)
	if err != nil || len(list.Prompts) != 1 || list.Prompts[0].Name != "ingest" || len(list.Prompts[0].Arguments) != 2 ||
		!list.Prompts[0].Arguments[0].Required {
		t.Fatalf("prompts/list = %+v, %v; want ingest with its arguments", list, err)
	}
	res, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "ingest", Arguments: map[string]string{"source": "a.pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Description != "Ingest a.pdf" || len(res.Messages) != 2 || res.Messages[0].Role != "user" ||
		res.Messages[0].Content.(*mcp.TextContent).Text != "Read a.pdf." || got["source"] != "a.pdf" ||
		call.Entry != NotebookEntry || call.Notebook != visible || call.TokenID != aliceTok {
		t.Errorf("prompts/get = %+v, the handler had %v and %+v", res, got, call)
	}
	a := s.post(t, context.Background(), Pattern, patToken, `{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"ingest"}}`, v2025())
	if code := rpcErrorCode(t, a); code != -32602 {
		t.Errorf("prompts/get without its source answered %d, want -32602", code)
	}
}
