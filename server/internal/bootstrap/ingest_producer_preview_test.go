package bootstrap

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// H08 repeats the four SDK Fit legs with real producer writes and legal
// depth11 assets. The REST control and both MCP scopes each create real marks.
func testIngestProducerPreview(t *testing.T) {
	for _, tool := range []string{"detail", "log"} {
		t.Run(tool, func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Producer preview","mode":"llm_wiki"}`)
			char, n := "a", 20
			if tool == "log" {
				char, n = "&", 3
			}
			parent, path := *nb.SourceRootID, "raw"
			for range 9 {
				name := strings.Repeat(char, 255)
				parent = tm.createPage(t, "alice", nb.ID, parent, name)
				path += "/" + name
			}
			sources := make([]previewSource, n)
			var body strings.Builder
			for i := range sources {
				name := strings.Repeat(char, 249) + fmt.Sprintf("%02d.txt", i)
				a := tm.upload(t, "alice", nb.ID, parent, name, fmt.Sprintf("real %s source %d", tool, i))
				sources[i] = previewSource{id: uuid.MustParse(a.ID), path: path + "/" + name, name: name}
				if len(sources[i].path) != 2563 || len(sources[i].path) > 4096 {
					t.Fatalf("legal source path %dB", len(sources[i].path))
				}
				body.WriteString("[source](<" + sources[i].path + ">)\n")
			}
			slices.SortFunc(sources, func(a, b previewSource) int { return a.id.Compare(b.id) })
			want := make([]string, n)
			for i, s := range sources {
				want[i] = s.id.String()
			}
			output := tm.createPage(t, "alice", nb.ID, "", "Output")
			f := ingestFixture{tm: tm, nb: nb.ID, p: output}
			restCS := f.write(t, output, body.String())
			assertIngestMarks(t, tm, restCS, want...)
			if got := tm.resolvedFrom(t, output); !slices.Equal(got, want) {
				t.Fatalf("real deep parser/resolver targets=%v want%v", got, want)
			}
			f.write(t, output, "REST control complete")
			for _, scope := range []string{"notebook", "account"} {
				t.Run(scope, func(t *testing.T) {
					m := tm.agent(t, "alice", nb.ID)
					client := "ingest-producer-" + tool + "-" + scope
					m.handshakeAs(t, client)
					message := "Real source ingestion"
					if tool == "log" {
						message = strings.Repeat("\x01", 4096)
					}
					before := tm.content(t, "alice", output).Revision
					args := map[string]any{"message": message, "ops": []map[string]any{opWriteOn("Output.md", body.String(), versionOf(output, before))}}
					result := answerOf[mcpBatched](t, sourceMCPCall(t, m, scope, "batch", args))
					if result.ChangesetID == nil || !result.Changed || result.ResultsCount != 1 {
						t.Fatalf("real batch=%+v", result)
					}
					cs := *result.ChangesetID
					if got := ingestCS(t, tm, output, before+1); got != cs {
						t.Fatalf("returned cs%s != actual%s", cs, got)
					}
					assertIngestMarks(t, tm, cs, want...)
					seq := queryStrings(t, tm.pool, "SELECT seq::text||':'||node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq", cs)
					wantSeq := make([]string, n)
					for i, id := range want {
						wantSeq[i] = fmt.Sprintf("%d:%s", i+1, id)
					}
					if !slices.Equal(seq, wantSeq) {
						t.Fatalf("real marks sequence %v want%v", seq, wantSeq)
					}
					p := previewCase{id: uuid.MustParse(cs), notebook: uuid.MustParse(nb.ID), sources: sources, client: "mcp:" + client, message: &message}
					var updated time.Time
					if err := tm.pool.QueryRow(t.Context(), "SELECT created_by_id,created_at,updated_at FROM changesets WHERE id=$1", cs).Scan(&p.actor, &p.at, &updated); err != nil {
						t.Fatal(err)
					}
					change := map[string]any{"node": map[string]any{"id": output, "kind": "page", "name": "Output", "alive": true}, "kind": "content", "before": nil, "after": nil, "content": map[string]any{"base_revision": before, "revision": before + 1}}
					p.changes = []any{change}
					detail := maps.Clone(change)
					detail["node"] = map[string]any{"id": output, "kind": "page", "name": "Output", "alive": true, "path": "Output.md"}
					p.detailChanges = []any{detail}
					header := func(s int) map[string]any {
						h := p.header(s, s < n)
						h["content_count"] = 1
						h["node_count"] = 0 // Content-only writes do not change the tree.
						h["updated_at"] = updated.Format(time.RFC3339Nano)
						h["created_by"] = map[string]any{"user_id": p.actor.String(), "display_name": tm.displayName(t, "alice")}
						return h
					}
					var build func(int, int) any
					if tool == "detail" {
						start := previewCursor(t, domain.ChangeCursor{})
						build = func(s, j int) any {
							h := header(s)
							h["renames"], h["moves"] = false, false
							var next any
							if j == 0 {
								next = start
							}
							return map[string]any{"changeset": h, "changes": p.detailChanges[:j], "next_cursor": next}
						}
					} else {
						build = func(s, j int) any {
							h := header(s)
							h["changes"] = p.changes[:j]
							if j < 1 {
								h["changes_cut"] = true
							}
							return map[string]any{"changesets": []any{h}, "next_cursor": nil}
						}
					}
					best := previewMax(t, n, func(s int) any { return build(s, 0) })
					changes := previewMax(t, 1, func(j int) any { return build(best, j) })
					if best < 1 || best >= n {
						t.Fatalf("legal producer source fit best=%d of%d, expected actual nonempty cut", best, n)
					}
					callTool := "changeset"
					callArgs := map[string]any{"id": cs}
					if tool == "log" {
						callTool = "log"
						callArgs = map[string]any{"client": "mcp:" + client, "limit": 1}
					}
					got := previewWire(t, sourceMCPCall(t, m, scope, callTool, callArgs))
					sameJSON(t, "real producer complete SDK envelope", got, normalizePreview(t, build(best, changes)))
					f.write(t, output, "prepare next independent producer")
				})
			}
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
}
