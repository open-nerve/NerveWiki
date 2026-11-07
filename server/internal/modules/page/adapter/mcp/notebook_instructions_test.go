package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The sentences, written out here again: a test that built them from the
// code's constants would agree with any change.
const (
	paragraphLead = "This notebook is an LLM Wiki: pages under `"
	paragraphRest = "/` are sources you add once and never change; index.md and log.md are generated; "
)

func notebookParagraph(root string) string {
	return paragraphLead + root + paragraphRest + "run the prompts ingest, query and lint."
}

func accountParagraph(root string) string {
	return paragraphLead + root + paragraphRest + "its prompts ingest, query and lint are served at `/mcp?notebook=" + nb.String() + "`."
}

// wikiOf is an LLM Wiki's material: its source root at path, titled title.
func wikiOf(path, title string, agents *domain.Agents) app.NotebookInstructions {
	i := app.NotebookInstructions{NotebookID: nb, Mode: domain.ModeLLMWiki, SourceRootPath: path, SourceRootTitle: title}
	if agents != nil {
		i.Agents, i.HasAgents = *agents, true
	}
	return i
}

// everything is a fits that holds for all.
func everything(string) bool { return true }

// sdkTextBytes is text's bytes as the real SDK writes a text block,
// without its quotes: the measure TextBytes must agree with, which a test
// that asked TextBytes itself could not tell.
func sdkTextBytes(t *testing.T, text string) int {
	t.Helper()
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: instructionAuth{}})
	server := mcpserver.New(mcpserver.Options{Name: "instruction-probe", Version: "test",
		Credentials: func(context.Context) (mcpserver.Credential, bool) {
			return mcpserver.Credential{UserID: uuid.UUID{1}, TokenID: uuid.UUID{2}}, true
		},
		Notebooks: instructionNotebooks{}, Errors: api.Errors, RequestTimeout: 4 * time.Second, MaxBodyBytes: 1 << 20, Logger: slog.New(slog.DiscardHandler),
		Tools: []mcpserver.Tool{mcpserver.NewTool(mcpserver.Spec{Name: "probe", Entry: mcpserver.BothEntries}, func(context.Context, mcpserver.Call, struct{}) (mcpserver.Result, error) {
			return mcpserver.Result{Blocks: []mcpserver.Block{{Text: text}}}, nil
		})},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"probe","arguments":{}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	var got struct {
		Result struct {
			Content []struct {
				Text json.RawMessage `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || len(got.Result.Content) != 1 {
		t.Fatalf("the SDK's probe = %d %.300s, %v", rec.Code, rec.Body.String(), err)
	}
	return len(got.Result.Content[0].Text) - 2
}

// Both entries write a notebook's instructions the one way, and share the
// answers' bounds (M10/P5 design 3.8): the system paragraph first, whole,
// what follows cut to what is left; a standard notebook's text is what it
// always was.
func TestNotebookInstructionsShareTheirBudget(t *testing.T) {
	agents := domain.Agents{Body: "# Rules <&>", Title: "AGENTS"}
	text := func(t *testing.T, i app.NotebookInstructions, entry mcpserver.Entry) string {
		t.Helper()
		got, err := NotebookText(i, entry, everything)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("standard", func(t *testing.T) {
		for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
			if got := text(t, app.NotebookInstructions{Mode: domain.ModeStandard, Agents: agents, HasAgents: true}, entry); got != agents.Body {
				t.Errorf("with AGENTS = %q", got)
			}
			if got := text(t, app.NotebookInstructions{Mode: domain.ModeStandard}, entry); got != NotebookInstructions {
				t.Errorf("without AGENTS = %.100q", got)
			}
		}
	})
	t.Run("notebook", func(t *testing.T) {
		if got, want := text(t, wikiOf("raw", "raw", &agents), mcpserver.NotebookEntry), notebookParagraph("raw")+"\n\n"+agents.Body; got != want {
			t.Errorf("= %q, want %q", got, want)
		}
	})
	t.Run("account", func(t *testing.T) {
		a, _, err := openOf(t, opened{answer: app.Opened{Instructions: wikiOf("a/raw", "raw", &agents)}})
		if want := accountParagraph("a/raw") + "\n\n" + agents.Body; err != nil || a.Instructions != want {
			t.Errorf("open_notebook's instructions = %q, %v; want %q", a.Instructions, err, want)
		}
	})
	t.Run("built_in", func(t *testing.T) {
		if got, want := text(t, wikiOf("raw", "raw", nil), mcpserver.NotebookEntry), notebookParagraph("raw")+"\n\n"+NotebookInstructions; got != want {
			t.Errorf("no AGENTS = %.200q", got)
		}
		a, _, err := openOf(t, opened{answer: app.Opened{Instructions: wikiOf("raw", "raw", nil)}})
		if want := accountParagraph("raw") + "\n\n" + NotebookInstructions; err != nil || a.Instructions != want || a.InstructionsFrom != nil {
			t.Errorf("open_notebook without AGENTS = %.200q from %v, %v", a.Instructions, a.InstructionsFrom, err)
		}
	})

	// The paragraph's room: its fixed words, the path, the title.
	t.Run("path_at_bound", func(t *testing.T) {
		for _, tt := range []struct {
			entry     mcpserver.Entry
			room      int
			paragraph func(string) string
		}{
			{mcpserver.NotebookEntry, 349, notebookParagraph},
			{mcpserver.AccountEntry, 286, accountParagraph},
		} {
			// The bound is of the bytes as written: & counts one, not its escape.
			for _, char := range []string{"p", "&"} {
				fits := strings.Repeat(char, tt.room)
				if got := text(t, wikiOf(fits, "raw", &agents), tt.entry); !strings.HasPrefix(got, tt.paragraph(fits)+"\n\n") {
					t.Errorf("%s: %d bytes of path = %.120q, want the path", tt.entry, tt.room, got)
				}
				if n := len(tt.paragraph(fits)); n != maxSystemBytes {
					t.Errorf("%s: the longest paragraph is %d bytes, want %d", tt.entry, n, maxSystemBytes)
				}
				if got := text(t, wikiOf(fits+char, "raw", &agents), tt.entry); !strings.HasPrefix(got, tt.paragraph("raw")+"\n\n") {
					t.Errorf("%s: %d bytes of path = %.120q, want the title", tt.entry, tt.room+1, got)
				}
			}
		}
	})
	t.Run("title_cut", func(t *testing.T) {
		long := strings.Repeat("漢", 200) // no real title is this long: 255 bytes fit either way
		got := systemParagraph(wikiOf(long, long, nil), mcpserver.NotebookEntry)
		if len(got) > maxSystemBytes || !utf8.ValidString(got) || !strings.HasSuffix(got, "…"+paragraphRest+"run the prompts ingest, query and lint.") ||
			!strings.HasPrefix(got, paragraphLead+"漢") {
			t.Errorf("a title past the room = %d bytes, %q", len(got), got)
		}
	})

	// One byte of the handshake's bound: the same bytes in the real SDK.
	t.Run("at_bound", func(t *testing.T) {
		lead := notebookParagraph("raw") + "\n\n"
		room := domain.MaxInstructionsBytes - mcpserver.TextBytes(lead)
		lines := room/3 - 1 // "x\n" is three bytes of a text block
		whole := domain.Agents{Body: strings.Repeat("x\n", lines) + strings.Repeat("x", room-3*lines), Title: "AGENTS"}
		if mcpserver.TextBytes(whole.Body) != room {
			t.Fatalf("the body is %d bytes, want %d", mcpserver.TextBytes(whole.Body), room)
		}
		got := text(t, wikiOf("raw", "raw", &whole), mcpserver.NotebookEntry)
		if got != lead+whole.Body || sdkTextBytes(t, got) != domain.MaxInstructionsBytes {
			t.Errorf("at the bound: %d bytes in the SDK, want the body whole in %d", sdkTextBytes(t, got), domain.MaxInstructionsBytes)
		}
		over := domain.Agents{Body: whole.Body + "x", Title: "AGENTS"}
		got = text(t, wikiOf("raw", "raw", &over), mcpserver.NotebookEntry)
		note := `(The rest of AGENTS.md is cut here: read it whole with read, path "AGENTS.md".)`
		if !strings.HasPrefix(got, lead+"x\n") || !strings.HasSuffix(got, "\n\n"+note) || sdkTextBytes(t, got) > domain.MaxInstructionsBytes {
			t.Errorf("a byte past the bound: %d bytes in the SDK, %.80q … %q", sdkTextBytes(t, got), got, got[len(got)-90:])
		}
	})

	// open_notebook's budget is a member's, measured with the paragraph,
	// and leaves the root's reserve.
	t.Run("member_escaping", func(t *testing.T) {
		body, cut := domain.AgentsBody(strings.Repeat("\"\n", 20000), 0, mcpserver.TextBytes)
		if !cut {
			t.Fatal("the body was not cut")
		}
		cutBody := domain.Agents{Body: body, Cut: true, Title: strings.Repeat("&", 255)}
		a, raw, err := openOf(t, opened{answer: app.Opened{Instructions: wikiOf("raw", "raw", &cutBody), Pages: 100, Root: longRoot(100)}})
		if err != nil {
			t.Fatal(err)
		}
		skeleton := a
		skeleton.Instructions, skeleton.Overview.Root, skeleton.Truncated = "", []rootItem{}, false
		s, _ := line(skeleton)
		budget := mcpserver.MaxAnswerBytes - mcpserver.TextBytes(s) - openNotebookRootReserve
		if !strings.HasPrefix(a.Instructions, accountParagraph("raw")+"\n\n") || mcpserver.MemberBytes(a.Instructions) > budget ||
			sdkTextBytes(t, raw) > mcpserver.MaxAnswerBytes || len(a.Overview.Root) < 28 {
			t.Errorf("the instructions are %d bytes as a member of %d, the answer %d, %d root nodes", mcpserver.MemberBytes(a.Instructions), budget,
				sdkTextBytes(t, raw), len(a.Overview.Root))
		}
	})

	// No answer leaves so little: only the function shows the guard.
	t.Run("no_room", func(t *testing.T) {
		long := domain.Agents{Body: strings.Repeat("x\n", 500), Cut: true, Title: "AGENTS"}
		note := `(The rest of AGENTS.md is cut here: read it whole with read, path "AGENTS.md".)`
		for name, tt := range map[string]struct {
			material app.NotebookInstructions
			least    string
		}{
			"AGENTS":   {wikiOf("raw", "raw", &long), notebookParagraph("raw") + "\n\n" + note},
			"built-in": {wikiOf("raw", "raw", nil), notebookParagraph("raw") + "\n\n" + NotebookInstructions},
		} {
			up := func(n int) func(string) bool { return func(s string) bool { return len(s) <= n } }
			if got, err := NotebookText(tt.material, mcpserver.NotebookEntry, up(len(tt.least)-1)); !errors.Is(err, errNoRoomForInstructions) {
				t.Errorf("%s: a byte too little = %.80q, %v; want the error", name, got, err)
			}
			if got, err := NotebookText(tt.material, mcpserver.NotebookEntry, up(len(tt.least))); err != nil || got != tt.least {
				t.Errorf("%s: just enough = %.80q, %v; want the paragraph and what must follow it", name, got, err)
			}
		}
	})
}
