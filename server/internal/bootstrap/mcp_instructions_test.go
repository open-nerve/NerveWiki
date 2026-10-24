package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// A notebook's instructions through serve (M9/P1 design 3.15, 3.17.4):
// its root page AGENTS changed through REST, read by MCP's handshake and
// open_notebook.

// instructionsAt are the instructions a handshake at path answers.
func (m *mcpApp) instructionsAt(t *testing.T, path string) string {
	t.Helper()
	r := m.post(t, context.Background(), path, m.pat, initializeBody(mcpVersion), nil).result(t)
	instructions, _ := r["instructions"].(string)
	return instructions
}

// opened is open_notebook's answer of notebook through the account entry.
func (m *mcpApp) opened(t *testing.T, notebook string) map[string]any {
	t.Helper()
	res := m.call(t, mcpserver.Pattern, "open_notebook", map[string]any{"notebook": notebook}).result(t)
	sc, _ := res["structuredContent"].(map[string]any)
	if res["isError"] == true || sc == nil {
		t.Fatalf("open_notebook %s = %v, want its answer", notebook, res)
	}
	return sc
}

// rest sends c's request with alice's session and answers its body,
// failing unless its status is want.
func (r readsApp) rest(t *testing.T, c step, want int) string {
	t.Helper()
	status, body := askTyped(t, r.contract, c.method, r.base+c.path, r.access, c.contentType, c.body)
	if status != want {
		t.Fatalf("%s = %d %s, want %d", c.name(), status, body, want)
	}
	return body
}

// revisionOf is the page id's revision now.
func (r readsApp) revisionOf(t *testing.T, id string) int {
	t.Helper()
	var c pageContent
	decodeAnswer(t, r.rest(t, request("alice", http.MethodGet, "/api/v0/pages/"+id+"/content", ""), http.StatusOK), &c)
	return c.Revision
}

// write writes content over the page id's, on its revision now.
func (r readsApp) write(t *testing.T, id, content string) {
	t.Helper()
	r.rest(t, contentWrite("alice", id, content, r.revisionOf(t, id), ""), http.StatusOK)
}

// cutNote is the line that ends an AGENTS page cut short, of its title.
func cutNote(title string) string {
	return "\n\n(The rest of " + title + ".md is cut here: read it whole with read, path \"" + title + ".md\".)"
}

// The instructions follow the AGENTS page through each of G16's changes,
// each through REST (M9/P1 design 3.15.1, 3.17.4): a replacement, a
// rename into the key and out of it, a deletion, a restore from the trash
// (the text before the deletion), a version restored and its undoing (new
// revisions, the texts of their time); and a write's next handshake, a
// rename in case alone, an attachment of the key at the root and a page of
// it below the root (neither is it), the frontmatter dropped, a content
// empty without it (the built-in ones), a cut at 32 KiB with its note. At
// each, open_notebook gives the handshake's instructions, and where they
// come from; of a page of quotes, which it cuts further, as many lines as
// fit with the root's reserve. The pages', the links' and the search's
// invariants hold after (checkPages), as at the end of each of 4.15's
// interleavings, this one's an AGENTS page replaced between two
// handshakes (M9 closeout A-D6).
func TestTheInstructionsFollowTheAGENTSPage(t *testing.T) {
	r := startReadsApp(t, "")
	builtIn := r.instructionsAt(t, atNotebook(r.notebook))
	if !strings.HasPrefix(builtIn, "This is a notebook of Nerve Wiki") || !strings.Contains(builtIn, "\n- read: ") {
		t.Fatalf("the built-in instructions = %q", builtIn)
	}
	expect := func(step, want, from string) {
		t.Helper()
		if got := r.instructionsAt(t, atNotebook(r.notebook)); got != want {
			t.Errorf("%s: the handshake's instructions = %.300q, want %.300q", step, got, want)
		}
		o := r.opened(t, r.notebook)
		gotFrom, _ := o["instructions_from"].(string)
		if o["instructions"] != want || gotFrom != from || (from == "") != (o["instructions_from"] == nil) {
			t.Errorf("%s: open_notebook's instructions = %.300q from %v, want %.300q from %q", step, o["instructions"], o["instructions_from"],
				want, from)
		}
	}
	rename := func(id, name string) { r.rest(t, nodeRename("alice", id, name), http.StatusOK) }
	expect("no AGENTS page", builtIn, "")

	attachment := r.upload(t, "", "AGENTS", imageOf(t, "png"))
	expect("an attachment of the key at the root", builtIn, "")
	r.rest(t, nodeDeletion("alice", attachment.ID), http.StatusNoContent)
	docs := r.createPage(t, "", "Docs", "")
	r.createPage(t, docs, "AGENTS", "# Nested\n")
	expect("a page of the key below the root", builtIn, "")

	agents := r.createPage(t, "", "AGENTS", "\ufeff---\r\ntags: [rules]\r\n---\r\n\r\n# Rules\r\nOne.\r\n")
	expect("the frontmatter dropped", "# Rules\r\nOne.", "AGENTS.md")
	r.write(t, agents, "# Rules\nTwo.\n")
	expect("a write's next handshake", "# Rules\nTwo.", "AGENTS.md")
	rename(agents, "agents")
	expect("a rename in case alone", "# Rules\nTwo.", "agents.md")

	rename(agents, "Old rules")
	expect("renamed out", builtIn, "")
	newer := r.createPage(t, "", "AGENTS", "# New\n")
	expect("replaced", "# New", "AGENTS.md")

	rename(newer, "New rules")
	notes := r.createPage(t, "", "Notes", "# Notes\n")
	expect("none of the key", builtIn, "")
	rename(notes, "agents")
	expect("renamed into the key", "# Notes", "agents.md")
	rename(notes, "Notes")
	expect("renamed out of it", builtIn, "")

	rename(newer, "AGENTS")
	expect("renamed back", "# New", "AGENTS.md")
	r.rest(t, nodeDeletion("alice", newer), http.StatusNoContent)
	expect("deleted", builtIn, "")
	r.rest(t, nodeRestore("alice", newer), http.StatusOK)
	expect("restored from the trash", "# New", "AGENTS.md")

	r.write(t, newer, "# New\nchanged\n")
	expect("written", "# New\nchanged", "AGENTS.md")
	r.rest(t, versionRestore("alice", newer, 1, r.revisionOf(t, newer)), http.StatusOK)
	expect("a version restored", "# New", "AGENTS.md")
	var restore string
	if err := r.pool.QueryRow(context.Background(), `SELECT id FROM changesets WHERE notebook_id = $1 AND kind = 'version_restore'`,
		r.notebook).Scan(&restore); err != nil {
		t.Fatal(err)
	}
	var reverted revertAnswer
	if err := json.Unmarshal([]byte(r.rest(t, changesetRevert("alice", restore), http.StatusOK)), &reverted); err != nil || !reverted.Changed {
		t.Fatalf("the restore's revert = %+v, %v; want it changed", reverted, err)
	}
	expect("the restore undone", "# New\nchanged", "AGENTS.md")

	r.write(t, newer, "---\ntags: [none]\n---\n \n")
	expect("empty without its frontmatter", builtIn, "")

	long := strings.Repeat(strings.Repeat("x", 99)+"\n", 400)
	r.write(t, newer, long)
	cut := r.instructionsAt(t, atNotebook(r.notebook))
	body, ok := strings.CutSuffix(cut, cutNote("AGENTS"))
	if size := mcpserver.TextBytes(cut); !ok || !strings.HasPrefix(long, body+"\n") || size > 32<<10 || size < 32<<10-100-len(cutNote("AGENTS")) {
		t.Errorf("cut at 32 KiB: %d bytes ending %q, want the content to a line's end and the note within 32 KiB", size, cut[max(0, len(cut)-120):])
	}
	expect("cut at 32 KiB", cut, "AGENTS.md")
	rename(newer, "Agents")
	expect("the cut's note in the title's case now", strings.TrimSuffix(cut, cutNote("AGENTS"))+cutNote("Agents"), "Agents.md")

	// Quotes and line breaks, which a member writes escaped twice:
	// open_notebook cuts them further than the handshake, to the most of
	// their lines that leaves the root 8,192 bytes of the answer (3.17.3,
	// S3 review C-M1).
	quotes := strings.Repeat("\"\n", 20000)
	r.write(t, newer, quotes)
	o := r.opened(t, r.notebook)
	got, _ := o["instructions"].(string)
	kept, ok := strings.CutSuffix(got, cutNote("Agents"))
	budget := mcpserver.MaxAnswerBytes - mcpserver.TextBytes(skeletonOf(t, o)) - 8192
	longer := kept + "\n\"" + cutNote("Agents")
	if !ok || !strings.HasPrefix(quotes, kept+"\n") || mcpserver.MemberBytes(got) > budget ||
		(mcpserver.MemberBytes(longer) <= budget && mcpserver.TextBytes(longer) <= 32<<10) {
		t.Errorf("a page of quotes: open_notebook's instructions are %d bytes as a member, a line more %d, their budget %d",
			mcpserver.MemberBytes(got), mcpserver.MemberBytes(longer), budget)
	}
	checkPages(t, r.pool)
}

// skeletonOf is open_notebook's answer o without its instructions and its
// root's nodes, as its text block writes it.
func skeletonOf(t *testing.T, o map[string]any) string {
	t.Helper()
	overview, _ := o["overview"].(map[string]any)
	skeleton := map[string]any{"notebook": o["notebook"], "mode": o["mode"], "instructions": "", "instructions_from": o["instructions_from"],
		"overview": map[string]any{"pages": overview["pages"], "attachments": overview["attachments"],
			"last_changed_at": overview["last_changed_at"], "root": []any{}}, "truncated": false}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(skeleton); err != nil || len(o) != len(skeleton) || len(overview) != 4 {
		t.Fatalf("the skeleton of %v: %v", o, err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
