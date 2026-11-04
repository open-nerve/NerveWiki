package bootstrap

import (
	"net/http"
	"slices"
	"testing"
)

// The first producer regression (P4 matrix U10/II/bridge): losing the real
// write event's Kind, ChangesetID or Before must lose this nonempty mark.
// The source is created legally through REST, and only the wiki page is
// subsequently written. No mark, changeset or index is inserted by this test.
func TestIngestUnit(t *testing.T) {
	t.Run("stable_index_scope", testStableIndexScope)
	t.Run("new_direct_only", testIngestNewDirectOnly)
	t.Run("finite_follow_closure", testIngestFiniteFollowClosure)
	t.Run("complete_candidates", testIngestCompleteCandidates)
	t.Run("marks_cap", testIngestMarksCap)
	t.Run("marks_rollback_session", testIngestSessionMarksRollback)
	t.Run("source_first_bytes", testIngestSourceFirstBytes)
	t.Run("tree_coverage", testIngestTreeCoverage)
	t.Run("producer_preview", testIngestProducerPreview)
	t.Run("index_serial", testIngestIndexSerial)
	t.Run("content_wait", testIngestContentWait)
	t.Run("bridge", func(t *testing.T) {
		for _, entry := range []string{"rest", "notebook", "account"} {
			t.Run(entry, func(t *testing.T) {
				tm := newAcmeTeam(t, "", "")
				wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Producer bridge","mode":"llm_wiki"}`)
				if wiki.Mode != "llm_wiki" || wiki.SourceRootID == nil {
					t.Fatalf("fixture notebook = %+v, want an LLM Wiki with a source root", wiki)
				}
				const evidence = "Independent source evidence.\n"
				const body = "# Digest\n\n[[raw/Source]]\n"
				source := tm.createPageWith(t, "alice", wiki.ID, *wiki.SourceRootID, "Source", evidence)
				output := tm.createPage(t, "alice", wiki.ID, "", "Digest")

				// Independent preconditions: S is a live, nonempty current source,
				// and P starts at an empty revision with no prior reference to S.
				if n := count(t, tm.pool, `SELECT count(*) FROM notebooks b
					JOIN nodes n ON n.notebook_id = b.id
					JOIN page_contents c ON c.node_id = n.id
					JOIN indexed_pages i ON i.node_id = n.id AND i.notebook_id = b.id
					WHERE b.id = $1 AND b.mode = 'llm_wiki' AND b.source_root_id = $2 AND b.deleted_at IS NULL
					AND n.id = $3 AND n.parent_id = $2 AND n.kind = 'page' AND n.deleted_at IS NULL
					AND c.content = $4 AND c.revision = 1 AND c.byte_size > 0 AND c.deleted_at IS NULL
					AND i.revision = c.revision AND i.extractor = 2`,
					wiki.ID, *wiki.SourceRootID, source, evidence); n != 1 {
					t.Fatalf("fixture has %d current nonempty sources matching S, want 1", n)
				}
				if before := tm.content(t, "alice", output); before.Content != "" || before.Revision != 1 {
					t.Fatalf("fixture P = %+v, want empty content at revision 1", before)
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM page_links WHERE notebook_id = $1 AND resolved_id = $2", wiki.ID, source); n != 0 {
					t.Fatalf("fixture already has %d links to S, want 0", n)
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM changeset_sources s
					JOIN changesets c ON c.id = s.changeset_id WHERE c.notebook_id = $1`, wiki.ID); n != 0 {
					t.Fatalf("fixture already has %d marks, want 0", n)
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND ingest_count <> 0", wiki.ID); n != 0 {
					t.Fatalf("fixture already has %d nonzero ingest counts, want 0", n)
				}
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)
				beforeChangesets := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", wiki.ID)

				var returnedChangeset string
				wantClient := "web"
				if entry == "rest" {
					tm.send(t, contentWrite("alice", output, body, 1, ""), http.StatusOK)
				} else {
					m := tm.agent(t, "alice", wiki.ID)
					m.handshakeAs(t, "ingest-bridge")
					args := map[string]any{"message": "Digest source", "ops": []map[string]any{
						opWriteOn("Digest.md", body, versionOf(output, 1)),
					}}
					result := answerOf[mcpBatched](t, sourceMCPCall(t, m, entry, "batch", args))
					if result.ChangesetID == nil || result.At == nil || !result.Changed || result.ResultsCount != 1 || len(result.Results) != 1 {
						t.Fatalf("batch = %+v, want one changed op and its real changeset", result)
					}
					op := result.Results[0]
					if op.Op != "write" || op.Path != "Digest.md" || op.NodeID != output || !op.Changed ||
						op.Version == nil || *op.Version != versionOf(output, 2) {
						t.Fatalf("batch op = %+v, want P written at revision 2", op)
					}
					returnedChangeset = *result.ChangesetID
					wantClient = "mcp:ingest-bridge"
				}

				// REST PUT returns a Page, not a changeset. Identify its exact
				// persisted revision, and cross-check the MCP answer against it.
				if after := tm.content(t, "alice", output); after.Content != body || after.Revision != 2 {
					t.Fatalf("write left P = %+v, want the direct reference at revision 2", after)
				}
				versions := queryStrings(t, tm.pool, `SELECT r.changeset_id::text
					FROM page_revisions r JOIN changesets c ON c.id = r.changeset_id
					WHERE r.node_id = $1 AND r.revision = 2 AND r.base_revision = 1 AND r.content = $2
					AND c.notebook_id = $3 AND c.kind = 'edit' AND c.deleted_at IS NULL AND NOT c.trimmed`,
					output, body, wiki.ID)
				if len(versions) != 1 {
					t.Fatalf("write's exact revision has changesets %v, want one real edit", versions)
				}
				cs := versions[0]
				if entry != "rest" && returnedChangeset != cs {
					t.Fatalf("batch changeset = %s, persisted revision belongs to %s", returnedChangeset, cs)
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", wiki.ID); n != beforeChangesets+1 {
					t.Fatalf("write made %d changesets, want exactly 1", n-beforeChangesets)
				}
				if got := queryStrings(t, tm.pool, `SELECT c.client || ' ' || u.email
					FROM changesets c JOIN users u ON u.id = c.created_by_id WHERE c.id = $1`, cs); !slices.Equal(got, []string{wantClient + " alice@example.com"}) {
					t.Fatalf("write's client/actor = %v, want %s alice@example.com", got, wantClient)
				}
				if got := tm.resolvedFrom(t, output); !slices.Equal(got, []string{source}) {
					t.Fatalf("write's direct targets = %v, want only S=%s", got, source)
				}
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)

				// Business RED belongs here, after the real write and its index
				// are proven healthy. These expectations do not call Add or the
				// producer's qualification helpers to manufacture their oracle.
				if got := queryStrings(t, tm.pool, `SELECT seq::text || ':' || node_id::text
					FROM changeset_sources WHERE changeset_id = $1 ORDER BY seq`, cs); !slices.Equal(got, []string{"1:" + source}) {
					t.Errorf("producer marks = %v, want exactly [1:%s]", got, source)
				}
				if got := count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id = $1", cs); got != 1 {
					t.Errorf("producer ingest_count = %d, want 1", got)
				}
				detail := tm.restOf(t, "alice", "/api/v0/changesets/"+cs)
				if detail["id"] != cs || detail["kind"] != "edit" {
					t.Fatalf("REST detail = %v, want the same edit %s", detail, cs)
				}
				if detail["ingest_count"] != float64(1) {
					t.Errorf("producer REST ingest_count = %v, want 1", detail["ingest_count"])
				}
				sameJSON(t, "producer REST ingest_sources", detail["ingest_sources"], []any{source})
			})
		}
	})
	runIngestUnitCases(t)
}
