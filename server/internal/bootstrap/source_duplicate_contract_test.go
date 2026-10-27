package bootstrap

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
)

// Every public refusal names the committed source that remains after rollback.
// These requests use the actual HTTP modules, guards, persistence and storage;
// neither a forced error nor a replacement guard supplies the duplicate.
func TestSourceDuplicatesCarryTheirTargetEverywhere(t *testing.T) {
	t.Run("REST", sourceDuplicateRESTTargets)
	t.Run("unit-baseline", sourceDuplicateUnitBaseline)
	t.Run("new-source-baseline", sourceDuplicateNewSourceBaseline)
	t.Run("reports", sourceDuplicateReportTargets)
}

// Node age does not establish a valid rollback target: a preexisting empty
// source or an ordinary page has no matching source key at the unit's start.
func sourceDuplicateNewSourceBaseline(t *testing.T) {
	for _, scope := range []string{"notebook", "account"} {
		for _, kind := range []string{"created", "empty-first", "moved-in"} {
			t.Run(scope+"/"+kind, func(t *testing.T) {
				tm := newAcmeTeam(t, "member", "")
				wiki := tm.llmWiki(t, "New source baseline")
				const content = "source bytes only introduced by this unit"
				first := opWrite("raw/P.md", content)
				switch kind {
				case "empty-first":
					id := tm.createPage(t, "alice", wiki.id, wiki.raw, "P")
					first = opWith(first, "base_version", id+":1")
				case "moved-in":
					tm.createPageWith(t, "alice", wiki.id, "", "P", content)
					first = opMove("P.md", "raw/P.md")
				}
				before := sourceEntryRows(t, tm.pool)
				answer := sourceMCPCall(t, tm.agent(t, "alice", wiki.id), scope, "batch", batched(first, opWrite("raw/Q.md", content)))
				result := answer.result(t)
				structured, _ := result["structuredContent"].(map[string]any)
				problem, _ := structured["error"].(map[string]any)
				_, hasTarget := problem["duplicate_of"]
				message, _ := problem["message"].(string)
				if result["isError"] != true || problem["code"] != "page.source_duplicate" || problem["op"] != float64(1) ||
					hasTarget || strings.Contains(message, "raw/P.md") || !strings.Contains(message, "ops[1] (write)") {
					t.Errorf("new source duplicate = %s, want ops[1] duplicate without rolled-back target", answer.body)
				}
				if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
					t.Error("refused batch persisted newly introduced source identity, content or history")
				}
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)
			})
		}
	}
}

// A existed before the unit in both cases. Only a source at the unit's start
// remains a valid duplicate target when the entire batch rolls back (B-M1).
func sourceDuplicateUnitBaseline(t *testing.T) {
	for _, scope := range []string{"notebook", "account"} {
		for _, wasSource := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/source_%t", scope, wasSource), func(t *testing.T) {
				tm := newAcmeTeam(t, "member", "")
				wiki := tm.llmWiki(t, "Unit baseline")
				parent, from := "", "F.md"
				if wasSource {
					parent, from = wiki.raw, "raw/F.md"
				}
				folder := tm.createPage(t, "alice", wiki.id, parent, "F")
				const content = "baseline source bytes"
				a := tm.createPageWith(t, "alice", wiki.id, folder, "A", content)
				caller := tm.agent(t, "alice", wiki.id)
				before := sourceEntryRows(t, tm.pool)
				answer := sourceMCPCall(t, caller, scope, "batch", batched(
					opMove(from, "F.md"), opWrite("F/B.md", content), opMove("F.md", "raw/F.md")))
				result := answer.result(t)
				structured, _ := result["structuredContent"].(map[string]any)
				problem, _ := structured["error"].(map[string]any)
				if result["isError"] != true || problem["code"] != "page.source_duplicate" || problem["op"] != float64(2) {
					t.Errorf("batch duplicate = %s, want source_duplicate at ops[2]", answer.body)
				}
				id, present := problem["duplicate_of"]
				if wasSource && id != a || !wasSource && present {
					t.Errorf("duplicate_of = %v (present=%t), source at unit start=%t, A=%s", id, present, wasSource, a)
				}
				message, _ := problem["message"].(string)
				if !strings.Contains(message, "ops[2] (move)") || strings.Contains(message, "F/A.md") != wasSource {
					t.Errorf("batch duplicate message %q does not preserve op and baseline target visibility", message)
				}
				if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
					t.Error("refused batch persisted its move-out, new B, move-in or rewritten metadata")
				}
				if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name = 'B'", wiki.id) != 0 {
					t.Error("refused batch retained its newly created B")
				}
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)
			})
		}
	}
}

func sourceDuplicateRESTTargets(t *testing.T) {
	for _, operation := range []string{
		"createPage", "putPageContent", "moveNode", "restoreNode",
		"revertChangeset", "uploadAsset", "startRevert", "startTrashRestore",
	} {
		t.Run(operation, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Duplicate "+operation)
			const content = "same exact source bytes\n"
			var target string
			var c step
			switch operation {
			case "createPage", "putPageContent", "moveNode":
				target = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
				switch operation {
				case "createPage":
					body, err := json.Marshal(map[string]any{"parent_id": wiki.raw, "title": "Different title", "content": content})
					if err != nil {
						t.Fatal(err)
					}
					c = request("bob", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages", string(body))
				case "putPageContent":
					empty := tm.createPage(t, "bob", wiki.id, wiki.raw, "Empty")
					c = contentWrite("bob", empty, content, 1, "")
				case "moveNode":
					outside := tm.createPageWith(t, "alice", wiki.id, "", "Outside", content)
					c = nodeMove("alice", outside, wiki.raw)
				}
			case "restoreNode", "startTrashRestore":
				deleted := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Deleted", content)
				tm.send(t, nodeDeletion("alice", deleted), http.StatusNoContent)
				target = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
				c = nodeRestore("alice", deleted)
				if operation == "startTrashRestore" {
					c = trashRestoreStart("alice", wiki.id, deleted)
				}
			case "revertChangeset", "startRevert":
				moved := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Moved", content)
				tm.send(t, nodeMove("alice", moved, ""), http.StatusOK)
				changeset := tm.newest(t, wiki.id).ID
				target = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
				c = changesetRevert("alice", changeset)
				if operation == "startRevert" {
					c = revertStart("alice", wiki.id, changeset, "")
				}
			case "uploadAsset":
				// Zero-byte attachments participate even though empty pages do not.
				target = tm.upload(t, "alice", wiki.id, wiki.raw, "known.bin", "").ID
				c = assetUpload(t, "bob", wiki.id, wiki.raw, "different.bin", "")
			}
			before, files := sourceEntryRows(t, tm.pool), storedBlobs(t, tm.storage)
			status, body := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
			var problem struct {
				Code        string  `json:"code"`
				DuplicateOf *string `json:"duplicate_of"`
			}
			decodeAnswer(t, body, &problem)
			if status != http.StatusConflict || problem.Code != "page.source_duplicate" ||
				problem.DuplicateOf == nil || *problem.DuplicateOf != target {
				t.Errorf("duplicate response = %d %s, want 409 page.source_duplicate targeting %s", status, body, target)
			}
			if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
				t.Error("duplicate refusal changed persisted tree, content, hashes, history, sessions or notebook")
			}
			if after := storedBlobs(t, tm.storage); !maps.Equal(after, files) {
				t.Error("duplicate refusal retained an upload file or changed an existing file")
			}
			if count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id) != 0 ||
				count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind IN ('transfer.revert', 'transfer.trash_restore')") != 0 {
				t.Error("duplicate precheck created a background job")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}
