package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The MCP history over the whole program (M9/P2 design 3.17.3): log,
// changeset and versions answer what REST's reads answer, member for
// member, but a changed node's kind, "attachment" where REST writes
// "asset", and a node's path; and the two fallbacks of the answers' bound
// go on over a real database, skipping nothing.

// structured is a success's structured content, failing on a tool's
// error.
func structured(t *testing.T, a mcpAnswer) map[string]any {
	t.Helper()
	r := a.result(t)
	content, ok := r["structuredContent"].(map[string]any)
	if r["isError"] == true || !ok {
		t.Fatalf("%s answered %.500s, want a structured result", a.tool, a.body)
	}
	return content
}

// textBytes is a's answer as mcpserver bounds it: its text blocks'
// TextBytes.
func textBytes(t *testing.T, a mcpAnswer) int {
	t.Helper()
	n := 0
	content, _ := a.result(t)["content"].([]any)
	for _, c := range content {
		if text, ok := c.(map[string]any)["text"].(string); ok {
			n += mcpserver.TextBytes(text)
		}
	}
	return n
}

// wouldBe is the text an answer of v would be, as mcpserver encodes and
// bounds one: what a fallback answers instead of.
func wouldBe(t *testing.T, v any) int {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return mcpserver.TextBytes(strings.TrimSuffix(b.String(), "\n"))
}

// within fails t when a's answer is over the bound, which mcpserver
// would have answered as internal_error: every history answer fits.
func within(t *testing.T, a mcpAnswer) int {
	t.Helper()
	n := textBytes(t, a)
	if n > mcpserver.MaxAnswerBytes {
		t.Errorf("%s's answer is %d bytes, over %d", a.tool, n, mcpserver.MaxAnswerBytes)
	}
	return n
}

// restOf is by's REST read at path, its JSON as decoded.
func (tm acmeTeam) restOf(t *testing.T, by, path string) map[string]any {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+path, tm.tokens[by], "")
	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); status != http.StatusOK || err != nil {
		t.Fatalf("GET %s as %s = %d %s", path, by, status, body)
	}
	return v
}

// restPages are the items of by's REST list at path, with query, read a
// page at a time to its end.
func (tm acmeTeam) restPages(t *testing.T, by, path, query string) []any {
	t.Helper()
	var items []any
	for cursor := ""; ; {
		v := tm.restOf(t, by, path+"?"+query+cursorParam(cursor))
		data, _ := v["data"].([]any)
		items = append(items, data...)
		next, _ := v["next_cursor"].(string)
		if next == "" {
			return items
		}
		cursor = next
	}
}

// mcpPages are the items of m's tool's list, under member, with args, read
// a page at a time to its end, each answer within the bound; and its
// pages' items.
func mcpPages(t *testing.T, m *mcpApp, tool, member string, args map[string]any) ([]any, [][]any) {
	t.Helper()
	var items []any
	var pages [][]any
	for cursor := ""; ; {
		sent := maps.Clone(args)
		if cursor != "" {
			sent["cursor"] = cursor
		}
		a := m.use(t, tool, sent)
		within(t, a)
		s := structured(t, a)
		page, _ := s[member].([]any)
		items, pages = append(items, page...), append(pages, page)
		next, _ := s["next_cursor"].(string)
		switch {
		case next == "":
			return items, pages
		case len(pages) > 200:
			t.Fatalf("%s %v never ends: %d pages", tool, args, len(pages))
		}
		cursor = next
	}
}

// asMCP is REST's answer v as MCP's history writes it: a changed node's
// kind "attachment" where REST writes "asset" (M9/P2 design 3.16.3).
func asMCP(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = asMCP(e)
		}
		if node, ok := out["node"].(map[string]any); ok && node["kind"] == "asset" {
			node["kind"] = "attachment"
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = asMCP(e)
		}
		return out
	}
	return v
}

// sameJSON fails t unless got is want, member for member.
func sameJSON(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		t.Errorf("%s:\n%s\nwant\n%s", what, g, w)
	}
}

// treePaths are the paths of the notebook nb's nodes alive, by id, from
// REST's tree: each page's ancestors' titles, then its title and ".md";
// an attachment's name as it is.
func (tm acmeTeam) treePaths(t *testing.T, by, nb string) map[string]string {
	t.Helper()
	data, _ := tm.restOf(t, by, "/api/v0/notebooks/"+nb+"/nodes")["data"].([]any)
	dirs, paths := map[string]string{}, map[string]string{}
	for _, n := range data { // each parent before its children
		node, _ := n.(map[string]any)
		id, _ := node["id"].(string)
		name, _ := node["name"].(string)
		dir := name
		if parent, ok := node["parent_id"].(string); ok {
			dir = dirs[parent] + "/" + name
		}
		dirs[id], paths[id] = dir, dir
		if node["kind"] == "page" {
			paths[id] = dir + ".md"
		}
	}
	return paths
}

// withPaths are REST's changes as changeset answers them: each node with
// its path now, null when it is not alive.
func withPaths(changes []any, paths map[string]string) []any {
	out := asMCP(changes).([]any)
	for _, c := range out {
		node, _ := c.(map[string]any)["node"].(map[string]any)
		node["path"] = nil
		if path, ok := paths[node["id"].(string)]; ok && node["alive"] == true {
			node["path"] = path
		}
	}
	return out
}

// TestTheMCPHistoryIsRESTs reads a notebook's history through log,
// changeset and versions, and through REST, where they are the same
// (M9/P2 design 3.17.3): alice writes, edits, moves and deletes over MCP,
// uploads an attachment through REST and moves it over MCP; bob writes
// and renames through REST, and reverts alice's edit with a message;
// alice moves bob's page into Guide over MCP, the one move across
// parents; an import; an edit session's two saves, one version. The log,
// whole and narrowed by a path, an actor ("me" alice), a client and a
// limit, is REST's listChangesets; each changeset is REST's getChangeset
// without its first changes, its changes listChangesetChanges' with their
// nodes' paths, a change at a time too; versions are listPageVersions',
// whole and two at a time to a null next_cursor, each version's lines
// getPageVersion's content. A page deleted has no path, its change none;
// another notebook's changeset is not found here.
func TestTheMCPHistoryIsRESTs(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	alice := tm.agent(t, "alice", nb)
	alice.handshakeAs(t, "whole-program")
	plan := answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": "Guide/Plan.md", "content": "one\ntwo\n"}))
	if len(plan.Created) != 2 {
		t.Fatalf("the write created %+v, want Guide and Plan", plan.Created)
	}
	guide, roadmap := plan.Created[0].NodeID, plan.NodeID
	answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": "Guide/Plan.md", "content": "one\ntwo\nthree\n",
		"base_version": plan.Version}))
	edited := answerOf[mcpEdited](t, alice.use(t, "edit", map[string]any{"path": "Guide/Plan.md", "old_string": "two", "new_string": "2"}))
	if status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/changesets/"+*edited.ChangesetID+"/revert",
		tm.tokens["bob"], `{"message":"Back to the write"}`); status != http.StatusOK {
		t.Fatalf("bob's revert of alice's edit = %d %s", status, body)
	}
	answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": "Scratch.md", "content": "x\n"}))
	answerOf[mcpMoved](t, alice.use(t, "move", map[string]any{"from": "Guide/Plan.md", "to": "Guide/Roadmap.md"}))
	answerOf[mcpDeleted](t, alice.use(t, "delete", map[string]any{"path": "Scratch.md"}))
	flow := tm.upload(t, "alice", nb, guide, "flow.png", string(imageOf(t, "png"))).ID
	answerOf[mcpMoved](t, alice.use(t, "move", map[string]any{"from": "Guide/flow.png", "to": "Guide/flow-1.png"}))
	notes := tm.createPageWith(t, "bob", nb, "", "Notes", "n\n")
	tm.send(t, nodeRename("bob", notes, "Minutes"), http.StatusOK)
	across := answerOf[mcpMoved](t, alice.use(t, "move", map[string]any{"from": "Minutes.md", "to": "Guide/Minutes.md"}))
	if across.ChangesetID == nil {
		t.Fatal("the move of Minutes into Guide wrote no changeset")
	}
	if j := tm.imported(t, "alice", nb, "", vaultZip(t, vaultFile{"Imported.md", "imported\n"})); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	session := tm.openSession(t, "alice", roadmap)
	merged := tm.content(t, "alice", roadmap).Revision
	tm.send(t, contentWrite("alice", roadmap, "one\n2\nthree\nfour\n", merged, session), http.StatusOK)
	tm.send(t, contentWrite("alice", roadmap, "one\n2\nthree\nfour\nfive\n", merged+1, session), http.StatusOK)
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)

	me, _ := tm.restOf(t, "alice", "/api/v0/me")["id"].(string)
	bob, _ := tm.restOf(t, "bob", "/api/v0/me")["id"].(string)
	logs := "/api/v0/notebooks/" + nb + "/changesets"
	all := tm.restPages(t, "alice", logs, "limit=100")
	t.Run("log", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			args  map[string]any
			query string
		}{
			{"whole", map[string]any{}, ""},
			{"of a page", map[string]any{"path": "Guide/Roadmap.md"}, "page_id=" + roadmap},
			{"of an attachment", map[string]any{"path": "guide/FLOW-1.png"}, "page_id=" + flow},
			{"of me", map[string]any{"actor": "me"}, "actor=" + me},
			{"of bob", map[string]any{"actor": bob}, "actor=" + bob},
			{"of every agent", map[string]any{"client": "mcp"}, "client=mcp"},
			{"of the agent", map[string]any{"client": "mcp:whole-program"}, "client=" + url.QueryEscape("mcp:whole-program")},
			{"by twos", map[string]any{"limit": 2}, "limit=2"},
			{"of my agents by twos", map[string]any{"actor": "me", "client": "mcp", "limit": 2}, "actor=" + me + "&client=mcp"},
		} {
			got, pages := mcpPages(t, alice, "log", "changesets", tt.args)
			want := asMCP(tm.restPages(t, "alice", logs, tt.query)).([]any)
			sameJSON(t, "the log "+tt.name, got, want)
			if limit, ok := tt.args["limit"].(int); ok && len(pages) != (len(want)+limit-1)/limit {
				t.Errorf("the log %s is %d pages of %d rows, want %d of %d", tt.name, len(pages), len(want), (len(want)+limit-1)/limit, limit)
			}
			if len(want) == 0 {
				t.Errorf("REST's log %s is empty: nothing compared", tt.name)
			}
		}
		kinds := map[any]bool{}
		for _, row := range all {
			kinds[row.(map[string]any)["kind"]] = true
		}
		if !kinds["import"] || !kinds["revert"] || len(all) != 14 {
			t.Errorf("the log has %d changesets of the kinds %v, want 14, the import and the revert among them", len(all), kinds)
		}
	})
	t.Run("changeset", func(t *testing.T) {
		paths, attachments, moves := tm.treePaths(t, "alice", nb), 0, map[string]bool{}
		for _, row := range all {
			id, _ := row.(map[string]any)["id"].(string)
			want := asMCP(tm.restOf(t, "alice", "/api/v0/changesets/"+id)).(map[string]any)
			delete(want, "changes")
			if want["moves"] == true {
				moves[id] = true
			}
			changes := withPaths(tm.restPages(t, "alice", "/api/v0/changesets/"+id+"/changes", "limit=100"), paths)
			a := alice.use(t, "changeset", map[string]any{"id": id})
			within(t, a)
			s := structured(t, a)
			sameJSON(t, "changeset "+id, s["changeset"], want)
			sameJSON(t, "the changes of "+id, s["changes"], changes)
			if s["next_cursor"] != nil {
				t.Errorf("changeset %s goes on at %v, want all its changes on its page", id, s["next_cursor"])
			}
			sameJSON(t, "the changes of "+id+" one at a time", alice.changesByOnes(t, id, len(changes)), changes)
			for _, c := range changes {
				if node := c.(map[string]any)["node"].(map[string]any); node["id"] == flow && node["kind"] == "attachment" {
					attachments++
				}
				if c.(map[string]any)["kind"] == "moved" && !moves[id] {
					t.Errorf("changeset %s moves a node across parents and says moves %v", id, want["moves"])
				}
			}
		}
		if attachments < 2 {
			t.Errorf("%d changes of flow.png are an attachment's, want its upload and its move", attachments)
		}
		if len(moves) != 1 || !moves[*across.ChangesetID] {
			t.Errorf("the changesets that move a node across parents are %v, want the move of Minutes alone, %s", moves,
				*across.ChangesetID)
		}
		other := tm.openNotebook(t, "alice", "Other")
		elsewhere := answerOf[mcpWritten](t, tm.agent(t, "alice", other).use(t, "write", map[string]any{"path": "Elsewhere.md",
			"content": "e"}))
		for _, id := range []string{*elsewhere.ChangesetID, "not-a-uuid"} {
			if r := refusalOf(t, alice.use(t, "changeset", map[string]any{"id": id})); r.Code != "page.changeset_not_found" {
				t.Errorf("changeset %s in Eng = %+v, want page.changeset_not_found", id, r)
			}
		}
	})
	t.Run("versions", func(t *testing.T) {
		read := alice.use(t, "read", map[string]any{"path": "Guide/Roadmap.md"})
		var meta struct{ Version string }
		if err := json.Unmarshal([]byte(blocksOf(t, read.result(t))[0]["text"].(string)), &meta); err != nil {
			t.Fatal(err)
		}
		a := alice.use(t, "versions", map[string]any{"path": "guide/roadmap.md"})
		within(t, a)
		s := structured(t, a)
		versions := tm.restPages(t, "alice", "/api/v0/pages/"+roadmap+"/versions", "limit=100")
		sameJSON(t, "the versions of Roadmap", s["versions"], versions)
		if s["path"] != "Guide/Roadmap.md" || s["node_id"] != roadmap || s["current_version"] != meta.Version || len(versions) != 5 ||
			s["next_cursor"] != nil {
			t.Errorf("versions = %v %v %v and %d versions, next_cursor %v; want Guide/Roadmap.md, %s, %s and 5, null", s["path"],
				s["node_id"], s["current_version"], len(versions), s["next_cursor"], roadmap, meta.Version)
		}
		// Two at a time: REST's, the last page's next_cursor null.
		twos, pages := mcpPages(t, alice, "versions", "versions", map[string]any{"path": "Guide/Roadmap.md", "limit": 2})
		sameJSON(t, "the versions of Roadmap two at a time", twos, versions)
		if len(pages) != (len(versions)+1)/2 {
			t.Errorf("the versions of Roadmap two at a time are %d pages, want %d", len(pages), (len(versions)+1)/2)
		}
		for _, v := range versions {
			revision := int(v.(map[string]any)["revision"].(float64))
			want, _ := tm.restOf(t, "alice", "/api/v0/pages/"+roadmap+"/versions/"+strconv.Itoa(revision))["content"].(string)
			if got := alice.versionLines(t, "Guide/Roadmap.md", revision, revision, meta.Version); got != want {
				t.Errorf("version %d's lines are %q, want REST's %q", revision, got, want)
			}
		}
		// The session's saves are one version: the revision between them
		// answers that version's, as REST's does.
		rest := tm.restOf(t, "alice", "/api/v0/pages/"+roadmap+"/versions/"+strconv.Itoa(merged+1))
		if rest["revision"] != float64(merged+2) {
			t.Fatalf("REST's version %d is %v, want the session's %d", merged+1, rest["revision"], merged+2)
		}
		if got := alice.versionLines(t, "Guide/Roadmap.md", merged+1, merged+2, meta.Version); got != rest["content"] {
			t.Errorf("version %d's lines are %q, want the session's %q", merged+1, got, rest["content"])
		}
	})
	t.Run("a page deleted", func(t *testing.T) {
		for _, tool := range []string{"versions", "log"} {
			if r := refusalOf(t, alice.use(t, tool, map[string]any{"path": "Scratch.md"})); r.Code != "page.not_found" {
				t.Errorf("%s of Scratch.md, deleted, = %+v, want page.not_found", tool, r)
			}
		}
	})
}

// changesByOnes are the changes of m's changeset id, n of them, read a
// change a page, each answer within the bound: the changeset on the first
// page alone, n pages.
func (m *mcpApp) changesByOnes(t *testing.T, id string, n int) []any {
	t.Helper()
	var changes []any
	for cursor, page := "", 1; ; page++ {
		args := map[string]any{"id": id, "limit": 1}
		if cursor != "" {
			args["cursor"] = cursor
		}
		a := m.use(t, "changeset", args)
		within(t, a)
		s := structured(t, a)
		if (s["changeset"] != nil) != (page == 1) {
			t.Errorf("changeset %s's page %d has the changeset %v, want it on the first page alone", id, page, s["changeset"] != nil)
		}
		c, _ := s["changes"].([]any)
		changes = append(changes, c...)
		next, _ := s["next_cursor"].(string)
		if next == "" {
			if page != n {
				t.Errorf("changeset %s by ones is %d pages, want %d", id, page, n)
			}
			return changes
		}
		if page > n {
			t.Fatalf("changeset %s by ones goes on past %d pages", id, n)
		}
		cursor = next
	}
}

// versionLines are the lines of m's version holding revision of the page
// at path, read a line at a time from next_offset to the end, each answer
// the version's own revision own and the page's version now current. Each
// line fits in one answer: a version that goes on past its total_lines
// reads fails.
func (m *mcpApp) versionLines(t *testing.T, path string, revision, own int, current string) string {
	t.Helper()
	var lines strings.Builder
	for offset, column, reads := 1, 0, 1; offset > 0; reads++ {
		args := map[string]any{"path": path, "revision": revision, "offset": offset, "limit": 1}
		if column > 0 {
			args["column"] = column
		}
		a := m.use(t, "versions", args)
		within(t, a)
		blocks := blocksOf(t, a.result(t))
		var meta struct {
			Path           string `json:"path"`
			Kind           string `json:"kind"`
			Revision       int    `json:"revision"`
			CurrentVersion string `json:"current_version"`
			TotalLines     int    `json:"total_lines"`
			NextOffset     *int   `json:"next_offset"`
			NextColumn     *int   `json:"next_column"`
		}
		if len(blocks) != 2 || json.Unmarshal([]byte(blocks[0]["text"].(string)), &meta) != nil {
			t.Fatalf("versions %v answered %.500s, want two blocks", args, a.body)
		}
		if meta.Revision != own || meta.CurrentVersion != current || meta.Kind != "page" || !strings.EqualFold(meta.Path, path) {
			t.Fatalf("versions %v answered %+v, want revision %d, current_version %s", args, meta, own, current)
		}
		lines.WriteString(blocks[1]["text"].(string))
		offset, column = 0, 0
		if meta.NextOffset != nil {
			offset = *meta.NextOffset
		}
		if meta.NextColumn != nil {
			column = *meta.NextColumn
		}
		if offset > 0 && reads >= meta.TotalLines {
			t.Fatalf("versions %v goes on at line %d after %d reads of its %d lines", args, offset, reads, meta.TotalLines)
		}
	}
	return lines.String()
}

// longHistory is the history seedLongHistory seeds: w1 and w2, the log's
// two rows over the bound, w1 the newest; and c1, bob's revert of a rename
// of a page at the longest path, leaf.
type longHistory struct {
	w1, w2, c1, leaf string
}

// seedLongHistory seeds the history of the two fallbacks in the notebook
// nb (M9/P2 design 3.17.3). alice's agent writes P and P/Q, titled 255 &
// each, and X1–X3 under Q; W1 and W2, two changesets of hers through SQL,
// newer than the rest, each with the longest message and three items
// moving X1–X3 from P, named 255 &, to Q, named 255 &: five long names a
// change, which the log reads by joins, whatever the tree is. She writes a
// chain of ten pages, each titled 254 & and a digit, and renames its leaf
// in place; bob reverts the rename through REST with the longest message:
// C1, one change, its node's path the chain's.
func seedLongHistory(t *testing.T, tm acmeTeam, alice *mcpApp, nb string) longHistory {
	t.Helper()
	long, short := strings.Repeat("&", 255), strings.Repeat("&", 254)
	message := strings.Repeat("\x01", domain.MaxMessageBytes)
	var xs []string
	var parents []mcpWritten
	for i := 1; i <= 3; i++ {
		w := answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": long + "/" + long + "/" + short + strconv.Itoa(i) + ".md",
			"content": ""}))
		xs, parents = append(xs, w.NodeID), append(parents, w)
	}
	if len(parents[0].Created) != 3 {
		t.Fatalf("the first write created %d nodes, want P, Q and X1", len(parents[0].Created))
	}
	p, q := parents[0].Created[0].NodeID, parents[0].Created[1].NodeID
	segments := make([]string, 10)
	for i := range segments {
		segments[i] = short + strconv.Itoa(i)
	}
	leaf := strings.Join(segments, "/") + ".md"
	answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": leaf, "content": ""}))
	moved := answerOf[mcpMoved](t, alice.use(t, "move", map[string]any{"from": leaf,
		"to": strings.Join(segments[:9], "/") + "/" + short + "r.md"}))
	body, err := json.Marshal(map[string]any{"message": message})
	if err != nil {
		t.Fatal(err)
	}
	status, got := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/changesets/"+*moved.ChangesetID+"/revert", tm.tokens["bob"],
		string(body))
	var r revertAnswer
	decodeAnswer(t, got, &r)
	if status != http.StatusOK || r.ChangesetID == nil {
		t.Fatalf("bob's revert of the rename = %d %s", status, got)
	}
	h := longHistory{c1: *r.ChangesetID, leaf: leaf}
	ctx := context.Background()
	for i, after := range []time.Duration{2 * time.Hour, time.Hour} {
		id, at := uuid.NewV7(), time.Now().Add(after)
		if _, err := tm.pool.Exec(ctx, `INSERT INTO changesets (id, notebook_id, kind, client, message, created_by_id, created_at, updated_at)
			SELECT $1, $2, 'edit', 'mcp:whole-program', $3, u.id, $4, $4 FROM users u WHERE u.email = 'alice@example.com'`,
			id, nb, message, at); err != nil {
			t.Fatal(err)
		}
		for _, x := range xs {
			if _, err := tm.pool.Exec(ctx, `INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name,
				before_sort_order, after_parent_id, after_name, after_sort_order, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 1, $6, $5, 1, $7, $7)`, uuid.NewV7(), id, x, p, long, q, at); err != nil {
				t.Fatal(err)
			}
		}
		if i == 0 {
			h.w1 = id.String()
		} else {
			h.w2 = id.String()
		}
	}
	return h
}

// TestTheMCPHistoryAnswersGoOn reads the two fallbacks of the answers'
// bound on a real database (M9/P2 design 3.17.3): the log by twos, from
// its start, answers W1 alone, then W2 alone, each with the first two of
// REST's three changes and changes_cut, then the rest as REST's, in its
// order: W2 is not skipped. changeset C1 by ones answers the changeset
// alone first, next_cursor its first change's page, then that change with
// its node's long path, as REST's. Each answer is within the bound.
func TestTheMCPHistoryAnswersGoOn(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	alice := tm.agent(t, "alice", nb)
	alice.handshakeAs(t, "whole-program")
	h := seedLongHistory(t, tm, alice, nb)
	logs := "/api/v0/notebooks/" + nb + "/changesets"
	rest := asMCP(tm.restPages(t, "alice", logs, "limit=100")).([]any)
	if len(rest) < 4 || rest[0].(map[string]any)["id"] != h.w1 || rest[1].(map[string]any)["id"] != h.w2 {
		t.Fatalf("REST's log is %d rows, want W1, W2 first", len(rest))
	}
	want := make([]any, len(rest))
	for i, row := range rest {
		want[i] = row
		if i < 2 {
			cut := maps.Clone(row.(map[string]any))
			cut["changes"], cut["changes_cut"] = cut["changes"].([]any)[:2], true
			want[i] = cut
		}
	}
	got, pages := mcpPages(t, alice, "log", "changesets", map[string]any{"limit": 2})
	sameJSON(t, "the log by twos", got, want)
	for i, id := range []string{h.w1, h.w2} {
		if len(pages[i]) != 1 || pages[i][0].(map[string]any)["id"] != id {
			t.Errorf("the log's page %d is %d rows, want %s alone", i+1, len(pages[i]), id)
		}
	}
	whole := wouldBe(t, map[string]any{"changesets": rest[:1], "next_cursor": strings.Repeat("e", 120)})
	first := alice.use(t, "log", map[string]any{"limit": 2})
	t.Logf("W1: %d bytes whole; the fallback's answer %d", whole, within(t, first))
	if whole <= mcpserver.MaxAnswerBytes {
		t.Errorf("W1 is %d bytes whole, within the bound: the log's fallback is not reached", whole)
	}

	changes := withPaths(tm.restPages(t, "alice", "/api/v0/changesets/"+h.c1+"/changes", "limit=100"), tm.treePaths(t, "alice", nb))
	header := asMCP(tm.restOf(t, "alice", "/api/v0/changesets/"+h.c1)).(map[string]any)
	delete(header, "changes")
	if len(changes) != 1 || changes[0].(map[string]any)["node"].(map[string]any)["path"] != h.leaf {
		t.Fatalf("C1's changes are %v, want one, of the leaf at its long path", changes)
	}
	a := alice.use(t, "changeset", map[string]any{"id": h.c1, "limit": 1})
	alone := within(t, a)
	s := structured(t, a)
	sameJSON(t, "C1's first page", s["changeset"], header)
	start, _ := s["next_cursor"].(string)
	if c, _ := s["changes"].([]any); len(c) != 0 || start == "" {
		t.Fatalf("C1's first page has %d changes and next_cursor %q, want none and the first change's page", len(c), start)
	}
	a = alice.use(t, "changeset", map[string]any{"id": h.c1, "limit": 1, "cursor": start})
	onward := within(t, a)
	s = structured(t, a)
	sameJSON(t, "C1's second page", s["changes"], changes)
	if s["changeset"] != nil || s["next_cursor"] != nil {
		t.Errorf("C1's second page has the changeset %v and next_cursor %v, want neither", s["changeset"], s["next_cursor"])
	}
	page := wouldBe(t, map[string]any{"changeset": header, "changes": changes, "next_cursor": strings.Repeat("e", 120)})
	t.Logf("C1: %d bytes with its change; the fallback's answer %d, the cursor's page %d", page, alone, onward)
	if page <= mcpserver.MaxAnswerBytes {
		t.Errorf("C1's first page would be %d bytes, within the bound: the changeset's fallback is not reached", page)
	}
}
