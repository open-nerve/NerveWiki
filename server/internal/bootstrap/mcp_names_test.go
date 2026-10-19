package bootstrap

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The clients' names through serve (M9/P1 design 3.16.4): what a client
// tells, cleaned, is recorded on its personal access token and is each
// call's client, in the app that heard it and in another on its database;
// a write's changeset is of the client mcp:<name> (M9/P2 design 3.13.3,
// the last hop).

// handshakeAs is a handshake at the notebook entry of the client name.
func (m *mcpApp) handshakeAs(t *testing.T, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": mcpVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": name, "version": "1"}}})
	m.post(t, context.Background(), atNotebook(m.notebook), m.pat, string(body), nil).result(t)
}

// callAs is the structured result of a call of tool with args at the
// notebook entry, its _meta telling the name told unless it is "".
func (m *mcpApp) callAs(t *testing.T, tool string, args map[string]any, told string) map[string]any {
	t.Helper()
	params := map[string]any{"name": tool, "arguments": args}
	if told != "" {
		params["_meta"] = map[string]any{"io.modelcontextprotocol/clientInfo": map[string]any{"name": told, "version": "1"}}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": params})
	res := m.post(t, context.Background(), atNotebook(m.notebook), m.pat, string(body), afterHandshake()).result(t)
	if res["isError"] == true {
		t.Fatalf("%s told %q answered an error: %v", tool, told, res)
	}
	sc, _ := res["structuredContent"].(map[string]any)
	return sc
}

// clientOfEcho is the client echo's call is told of.
func (m *mcpApp) clientOfEcho(t *testing.T, told string) string {
	t.Helper()
	client, _ := m.callAs(t, "echo", map[string]any{}, told)["client"].(string)
	return client
}

// clientOfWrite is the client of the changeset of a write of the new page
// title, its call telling told, as clientOfEcho's.
func (m *mcpApp) clientOfWrite(t *testing.T, pool *pgxpool.Pool, title, told string) string {
	t.Helper()
	id, _ := m.callAs(t, "write", map[string]any{"path": title + ".md", "content": ""}, told)["changeset_id"].(string)
	return queryStrings(t, pool, "SELECT client FROM changesets WHERE id = $1", id)[0]
}

// A token's client is unknown until its client tells a name; the name
// told, cleaned, is then on the token and each call's client, a call that
// tells none included; an app started anew on the database reads it, and
// keeps it when its client tells it again, a record that writes nothing
// (M9/P1 S3 review C-M2); a call's _meta telling another changes it; a
// name of a bidirectional override and 200 characters is recorded
// cleaned, 128 of them. At each step a write's changeset is of mcp: and
// the client then, as the page's adapter hands it to the use case.
func TestTheClientNameIsTheTokens(t *testing.T) {
	db := pgtest.NewDatabase(t)
	logs := &syncBuffer{}
	m := runMCPApp(t, wireMCPApp(t, testConfig(t, db, false), logs, mcpTestTools(newMCPSeen())), db, logs)
	pool := connect(t, db)
	recorded := func() string {
		t.Helper()
		var name *string
		if err := pool.QueryRow(context.Background(), `SELECT client_name FROM api_tokens`).Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name == nil {
			return ""
		}
		return *name
	}
	wrote := func(step int, m *mcpApp, told, want string) {
		t.Helper()
		if got := m.clientOfWrite(t, pool, "Step "+strconv.Itoa(step)+told, told); got != want {
			t.Errorf("step %d: the write's changeset is of %q, want %q", step, got, want)
		}
	}
	if got := m.clientOfEcho(t, ""); got != mcpserver.UnknownClient || recorded() != "" {
		t.Errorf("before a name is told: the client %q, recorded %q; want unknown, none", got, recorded())
	}
	wrote(1, m, "", "mcp:unknown")
	m.handshakeAs(t, "  Claude Code\t")
	if got := m.clientOfEcho(t, ""); got != "Claude Code" || recorded() != "Claude Code" {
		t.Errorf("after the handshake: the client %q, recorded %q; want Claude Code", got, recorded())
	}
	wrote(2, m, "", "mcp:Claude Code")
	again := &mcpApp{base: runApp(t, wireMCPApp(t, testConfig(t, db, false), &syncBuffer{}, mcpTestTools(newMCPSeen()))), pat: m.pat,
		notebook: m.notebook}
	if got := again.clientOfEcho(t, ""); got != "Claude Code" {
		t.Errorf("an app anew on the database: the client %q, want the recorded Claude Code", got)
	}
	wrote(3, again, "", "mcp:Claude Code")
	restarted := &mcpApp{base: runApp(t, wireMCPApp(t, testConfig(t, db, false), &syncBuffer{}, mcpTestTools(newMCPSeen()))), pat: m.pat,
		notebook: m.notebook}
	restarted.handshakeAs(t, "Claude Code") // the name on the token: its record writes nothing
	if got := restarted.clientOfEcho(t, ""); got != "Claude Code" {
		t.Errorf("an app anew whose client tells the recorded name, then a call telling none: %q, want Claude Code", got)
	}
	wrote(4, restarted, "", "mcp:Claude Code")
	if got := again.clientOfEcho(t, "Codex"); got != "Codex" || recorded() != "Codex" || again.clientOfEcho(t, "") != "Codex" {
		t.Errorf("a call's _meta telling Codex: the client %q, recorded %q; want Codex", got, recorded())
	}
	wrote(5, again, "Codex", "mcp:Codex")
	wrote(5, again, "", "mcp:Codex")
	m.handshakeAs(t, "\u202e"+strings.Repeat("n", 200))
	if want := strings.Repeat("n", 128); recorded() != want || m.clientOfEcho(t, "") != want {
		t.Errorf("a name of an override and 200 characters: recorded %q, want 128 of them", recorded())
	}
	wrote(6, m, "", "mcp:"+strings.Repeat("n", 128))
}
