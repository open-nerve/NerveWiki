package bootstrap

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// readPromptLists is api/mcp/prompts.json, beside tools.json, decoded.
func readPromptLists(t testing.TB) toolLists {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(toolListsFile()), "prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lists toolLists
	if err := json.Unmarshal(b, &lists); err != nil {
		t.Fatalf("api/mcp/prompts.json: %v", err)
	}
	return lists
}

// listedPrompts are the prompts each entry lists, the notebook entry's of
// notebook, every member as the wire has it. A list that is no array, null
// included, fails here.
func (m *mcpApp) listedPrompts(t testing.TB, notebook string) toolLists {
	t.Helper()
	lists := toolLists{}
	for _, name := range slices.Sorted(maps.Keys(mcpEntries())) {
		path := mcpserver.Pattern
		if mcpEntries()[name] == mcpserver.NotebookEntry {
			path = atNotebook(notebook)
		}
		r := m.post(t, context.Background(), path, m.pat, `{"jsonrpc":"2.0","id":2,"method":"prompts/list"}`, afterHandshake()).result(t)
		prompts, ok := r["prompts"].([]any)
		if !ok {
			t.Fatalf("prompts/list at %s = %v, want its prompts", path, r)
		}
		lists[name] = prompts
	}
	return lists
}

// capabilitiesAt are the capabilities a handshake at path says, as JSON.
func (m *mcpApp) capabilitiesAt(t testing.TB, path string) string {
	t.Helper()
	b, err := json.Marshal(m.post(t, context.Background(), path, m.pat, initializeBody(mcpVersion), nil).result(t)["capabilities"])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// createLLMWiki is a new LLM Wiki of acme, through the API.
func (m *mcpApp) createLLMWiki(t *testing.T) string {
	t.Helper()
	status, created := ask(t, m.contract, http.MethodPost, m.base+"/api/v0/workspaces/acme/notebooks", m.access, `{"name":"Field notes","mode":"llm_wiki"}`)
	if status != http.StatusCreated {
		t.Fatalf("create an LLM Wiki = %d %s", status, created)
	}
	return idOf(t, answer{body: created})
}

// getPrompt is a raw prompts/get at path, with token.
func (m *mcpApp) getPrompt(t testing.TB, path, token, name string, args map[string]string) mcpAnswer {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "prompts/get", "params": map[string]any{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	return m.post(t, context.Background(), path, token, string(body), afterHandshake())
}

// promptRefusal is a JSON-RPC error's code, message and data.code.
func promptRefusal(t testing.TB, a mcpAnswer) (code float64, message, dataCode string) {
	t.Helper()
	e, ok := a.decoded(t)["error"].(map[string]any)
	if a.status != http.StatusOK || !ok {
		t.Fatalf("answer %d %.300s, want a JSON-RPC error", a.status, a.body)
	}
	code, _ = e["code"].(float64)
	message, _ = e["message"].(string)
	if data, ok := e["data"].(map[string]any); ok {
		dataCode, _ = data["code"].(string)
	}
	return code, message, dataCode
}

// The prompts each entry lists are the contract's, api/mcp/prompts.json,
// member by member, in both modes (M10 design 4.15): the notebook entry
// lists the four, the LLM Wiki's three and the upgrade's (M11/P3 design
// 3.18), in a standard notebook too, the account entry none, and only the
// notebook entry says prompts.
func TestThePromptListsAreTheContract(t *testing.T) {
	m := startMCPApp(t, nil)
	wiki := m.createLLMWiki(t)
	want := readPromptLists(t)
	if len(want["notebook"]) != 4 || len(want["account"]) != 0 {
		t.Fatalf("the file has %d and %d prompts, want 4 and 0", len(want["notebook"]), len(want["account"]))
	}
	for name, notebook := range map[string]string{"an LLM Wiki": wiki, "a standard notebook": m.notebook} {
		got := m.listedPrompts(t, notebook)
		for _, d := range toolListDiff(got, want) {
			t.Errorf("%s: %s", name, d)
		}
		if at := m.capabilitiesAt(t, atNotebook(notebook)); at != `{"prompts":{},"tools":{}}` {
			t.Errorf("%s: the notebook entry's capabilities = %s", name, at)
		}
	}
	if at := m.capabilitiesAt(t, mcpserver.Pattern); at != `{"tools":{}}` {
		t.Errorf("the account entry's capabilities = %s", at)
	}

	// The comparison itself: one less, one more and one reworded are told.
	got := m.listedPrompts(t, wiki)
	if len(got["notebook"]) == 0 {
		t.Fatal("the notebook entry lists no prompt to compare")
	}
	reworded := map[string]any{}
	maps.Copy(reworded, got["notebook"][0].(map[string]any))
	reworded["description"] = "Something else."
	for name, list := range map[string][]any{
		"one less": got["notebook"][1:],
		"one more": append(slices.Clone(got["notebook"]), map[string]any{"name": "more"}),
		"reworded": append([]any{reworded}, got["notebook"][1:]...),
	} {
		if d := toolListDiff(toolLists{"notebook": list, "account": got["account"]}, want); len(d) == 0 {
			t.Errorf("%s: the comparison found no difference", name)
		}
	}
}

// The three prompts of the composition root reach their handlers through
// the notebook entry (M10 design 4.15): each its text, an argument once;
// a standard notebook's refusal; the arguments' bound before it; the
// account entry's unknown prompt; and a notebook no one lets the caller
// see stops at the HTTP edge.
func TestTheWikiPromptsAreReached(t *testing.T) {
	m := startMCPApp(t, nil)
	wiki := m.createLLMWiki(t)
	at := atNotebook(wiki)
	text := func(t *testing.T, a mcpAnswer) string {
		t.Helper()
		messages, _ := a.result(t)["messages"].([]any)
		if len(messages) != 1 {
			t.Fatalf("the messages = %v, want one", messages)
		}
		message := messages[0].(map[string]any)
		content := message["content"].(map[string]any)
		if message["role"] != "user" || content["type"] != "text" {
			t.Fatalf("the message = %v, want the user's text", message)
		}
		return content["text"].(string)
	}
	descriptions := map[string]string{}
	for _, p := range readPromptLists(t)["notebook"] {
		object := p.(map[string]any)
		descriptions[object["name"].(string)] = object["description"].(string)
	}
	for _, tt := range []struct {
		name, first, argument, value string
	}{
		{"ingest", "Ingest a source into this LLM Wiki.", "source", "raw/the-unique-source.md"},
		{"query", "Answer a question from this LLM Wiki.", "question", "the unique question?"},
		{"lint", "Check and maintain this LLM Wiki.", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]string{}
			if tt.argument != "" {
				args[tt.argument] = tt.value
			}
			a := m.getPrompt(t, at, m.pat, tt.name, args)
			got := text(t, a)
			if first, _, _ := strings.Cut(got, "\n"); first != tt.first {
				t.Errorf("the first line = %q", first)
			}
			if d := a.result(t)["description"]; d != descriptions[tt.name] {
				t.Errorf("the description = %v, want the list's", d)
			}
			if tt.value != "" && (strings.Count(string(a.body), tt.value) != 1 || !strings.HasSuffix(got, ":\n"+tt.value)) {
				t.Errorf("the argument is not in the answer once, at the message's end: %.300s", got[len(got)-80:])
			}
		})
	}
	t.Run("no source", func(t *testing.T) {
		if got := text(t, m.getPrompt(t, at, m.pat, "ingest", nil)); strings.Contains(got, "supplied by the caller:") {
			t.Errorf("a get without a source ends %q", got[len(got)-80:])
		}
	})
	// An argument goes as it was sent: an empty source is a source, and
	// the white space around one is its own; a key of no prompt is nobody's.
	for name, source := range map[string]string{"an empty source": "", "a spaced source": "  a b.md\t"} {
		t.Run(name, func(t *testing.T) {
			got := text(t, m.getPrompt(t, at, m.pat, "ingest", map[string]string{"source": source}))
			if !strings.HasSuffix(got, "Source path supplied by the caller:\n"+source) {
				t.Errorf("the message ends %q", got[len(got)-60:])
			}
		})
	}
	t.Run("a key of no prompt", func(t *testing.T) {
		if got := text(t, m.getPrompt(t, at, m.pat, "lint", map[string]string{"": "INJECTED"})); strings.Contains(got, "INJECTED") {
			t.Errorf("the message ends %q", got[len(got)-60:])
		}
	})
	t.Run("no question", func(t *testing.T) {
		if code, _, data := promptRefusal(t, m.getPrompt(t, at, m.pat, "query", nil)); code != -32602 || data != "validation_failed" {
			t.Errorf("a get without its question = %v %q", code, data)
		}
	})
	t.Run("standard", func(t *testing.T) {
		code, message, data := promptRefusal(t, m.getPrompt(t, atNotebook(m.notebook), m.pat, "lint", nil))
		if code != -32602 || message != "notebook.not_llm_wiki: The notebook is not an LLM Wiki." || data != "notebook.not_llm_wiki" {
			t.Errorf("a standard notebook's get = %v %q %q", code, message, data)
		}
	})
	t.Run("standard over the bound", func(t *testing.T) {
		long := map[string]string{"question": strings.Repeat("x", mcpserver.MaxPromptArgumentBytes+1)}
		if code, _, data := promptRefusal(t, m.getPrompt(t, atNotebook(m.notebook), m.pat, "query", long)); code != -32602 || data != "validation_failed" {
			t.Errorf("an argument over its bound in a standard notebook = %v %q, want it refused first", code, data)
		}
	})
	t.Run("account", func(t *testing.T) {
		if code, message, data := promptRefusal(t, m.getPrompt(t, mcpserver.Pattern, m.pat, "lint", nil)); code != -32602 ||
			!strings.Contains(message, "unknown prompt") || data != "" {
			t.Errorf("the account entry's get = %v %q %q", code, message, data)
		}
	})
	t.Run("hidden", func(t *testing.T) {
		if code := m.getPrompt(t, atNotebook(m.hidden), m.pat, "lint", nil).problem(t, http.StatusNotFound); code != "notebook.not_found" {
			t.Errorf("a notebook the caller cannot see = %q", code)
		}
	})
	t.Run("logged", func(t *testing.T) {
		lines := func() []map[string]any {
			return slices.DeleteFunc(m.logLines(t), func(l map[string]any) bool { return l["msg"] != "MCP prompt call" })
		}
		before := len(lines())
		m.getPrompt(t, at, m.pat, "ingest", map[string]string{"source": "raw/never-logged.md"})
		after := lines()
		if len(after) != before+1 {
			t.Fatalf("%d call lines after %d: want this get's one", len(after), before)
		}
		if line := after[len(after)-1]; line["prompt"] != "ingest" || line["entry"] != "notebook" || line["notebook_id"] != wiki || line["outcome"] != "ok" {
			t.Errorf("the call line = %v", line)
		}
		for _, l := range m.logLines(t) {
			if b, err := json.Marshal(l); err != nil || strings.Contains(string(b), "never-logged") {
				t.Errorf("a line holds the argument: %s, %v", b, err)
			}
		}
	})
}
