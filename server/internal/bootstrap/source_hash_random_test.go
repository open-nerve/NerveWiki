package bootstrap

// Successful public operations are interleaved within their real prerequisites.
// Every step uses the independent raw-tree/body oracle; production membership
// readers are used only by the application under test.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"math/rand/v2"
	"net/http"
	"os"
	"testing"
)

func TestTheSourceRebuildMatchesSuccessfulRandomSteps(t *testing.T) {
	for _, seed := range []uint64{7, 29, 101, 347, 991, 2027} {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			// One app/database per seed, not per action; no parallel heavy work.
			tm := newAcmeTeam(t, "", "")
			rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
			counts := map[string]int{}
			successes, refusals := 0, 0
			check := func() {
				t.Helper()
				// Wait for settled jobs, then run complete page/link/search,
				// asset/file, transfer and notebook invariants.
				checkBackground(t, tm)
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				if t.Failed() {
					t.FailNow()
				}
			}
			type action struct {
				name string
				tags []string
				run  func()
			}
			run := func(a action) {
				t.Helper()
				t.Logf("seed=%d attempt=%d action=%s", seed, successes+1, a.name)
				a.run()
				check()
				// Neither rejected requests nor setup-only notebook creation is
				// counted; the request and its postconditions must finish first.
				successes++
				for _, tag := range a.tags {
					counts[tag]++
				}
			}
			refuse := func(label string, c step, code string, target *string) {
				t.Helper()
				check()
				before, files := sourceRandomRows(t, tm), sourceRandomFiles(t, tm)
				status, body := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
				var p struct {
					Code        string  `json:"code"`
					DuplicateOf *string `json:"duplicate_of"`
				}
				decodeAnswer(t, body, &p)
				if status != http.StatusConflict || p.Code != code ||
					(target == nil && p.DuplicateOf != nil) ||
					(target != nil && (p.DuplicateOf == nil || *target != *p.DuplicateOf)) {
					t.Fatalf("%s = %d %s, want 409 %s target=%v", label, status, body, code, target)
				}
				if after := sourceRandomRows(t, tm); !maps.Equal(before, after) {
					t.Fatalf("%s changed persisted rows", label)
				}
				if after := sourceRandomFiles(t, tm); !maps.Equal(files, after) {
					t.Fatalf("%s changed committed file identities or bytes", label)
				}
				check()
				refusals++
				t.Logf("seed=%d refused=%d action=%s (not a successful step)", seed, refusals, label)
			}

			for epoch := range 3 {
				prefix := fmt.Sprintf("S%dE%d", seed, epoch)
				wiki := tm.llmWiki(t, prefix)
				check() // Template/setup is checked, but not counted as a step.
				targetName, movedTargetName := prefix+"Target", prefix+"LongerTarget"
				sourceName := prefix + "Source"
				lead := fmt.Sprintf("%s-%x 原文\r\n", prefix, rng.Uint64())
				body := lead + "[[" + targetName + "]]\n"
				rewritten := lead + "[[" + movedTargetName + "]]\n"
				firstBody := fmt.Sprintf("%s-first-%x\r\n", prefix, rng.Uint64())
				ordinaryBody := fmt.Sprintf("%s-entry-%x\n", prefix, rng.Uint64())
				assetBody := ""
				if epoch != 0 {
					assetBody = fmt.Sprintf("%s-asset-%x\x00", prefix, rng.Uint64())
				}
				var target, source, empty, asset, ordinary, anchor string
				var rewrittenChange, undoneChange, renamedChange string
				place := func(id, parent string) {
					t.Helper()
					tm.send(t, nodeMove("alice", id, parent), http.StatusOK)
					var wantParent any
					if parent != "" {
						wantParent = parent
					}
					if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 AND parent_id IS NOT DISTINCT FROM $2::uuid AND deleted_at IS NULL", id, wantParent) != 1 {
						t.Fatalf("move of %s did not reach parent %q", id, parent)
					}
				}
				deleteNode := func(id string) {
					t.Helper()
					tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
					if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 AND deleted_at IS NOT NULL AND source_hash IS NULL", id) != 1 {
						t.Fatalf("deletion did not clear source hash for %s", id)
					}
				}
				restore := func(id string) {
					t.Helper()
					r := tm.restored(t, "alice", id)
					if r.ID != id || r.Placed.ParentID == nil || *r.Placed.ParentID != wiki.raw {
						t.Fatalf("restore of %s = %+v, want original source parent", id, r)
					}
				}
				version := func(changeset, content string, base, revision int) {
					t.Helper()
					tm.wrote(t, source, content, revision)
					tm.sourceRewriteVersion(t, changeset, source, content, base, revision, true)
					tm.resolves(t, source, target)
				}
				// Each queue preserves its real prerequisites. Randomly select
				// among ready queues, rather than random requests mostly refused.
				queues := [][]action{
					{
						{"rewrite target create", nil, func() {
							target = tm.createPage(t, "alice", wiki.id, "", targetName)
						}},
						{"nonempty source create", []string{"create"}, func() {
							source = tm.createPageWith(t, "alice", wiki.id, wiki.raw, sourceName, body)
							tm.wrote(t, source, body, 1)
							tm.resolves(t, source, target)
						}},
						{"system rewrite", []string{"rewrite"}, func() {
							tm.send(t, nodeRename("alice", target, movedTargetName), http.StatusOK)
							rewrittenChange = tm.newest(t, wiki.id).ID
							version(rewrittenChange, rewritten, 1, 2)
						}},
						{"rewrite undo", []string{"rewrite_undo"}, func() {
							undoneChange = tm.reverted(t, "alice", rewrittenChange)
							version(undoneChange, body, 2, 3)
						}},
						{"rewrite undo of undo", []string{"undo_of_undo"}, func() {
							id := tm.reverted(t, "alice", undoneChange)
							version(id, rewritten, 3, 4)
						}},
						{"source rename", []string{"manage", "rename"}, func() {
							tm.send(t, nodeRename("alice", source, sourceName+"Renamed"), http.StatusOK)
							renamedChange = tm.newest(t, wiki.id).ID
							if tm.bgName(t, source) != sourceName+"Renamed" {
								t.Fatal("source rename did not change its title")
							}
						}},
						{"background source rename undo", []string{"background_revert"}, func() {
							j := tm.background(t, revertStart("alice", wiki.id, renamedChange, ""))
							if j.State != "succeeded" || j.failure() != "" || j.Progress.Done != 1 || j.counts()["put_back"] != 1 || tm.bgName(t, source) != sourceName {
								t.Fatalf("background rename undo = %s", j)
							}
							tm.wrote(t, source, rewritten, 4)
						}},
						{"source delete for synchronous restore", []string{"delete"}, func() { deleteNode(source) }},
						{"source synchronous restore", []string{"restore"}, func() { restore(source) }},
						{"source delete for background restore", []string{"delete"}, func() { deleteNode(source) }},
						{"source background restore", []string{"background_restore"}, func() {
							j := tm.background(t, trashRestoreStart("alice", wiki.id, source))
							// Trash restore counts pages/assets; "restored" counts
							// restored items in a changeset-revert job instead.
							if j.State != "succeeded" || j.failure() != "" || j.Progress.Done != 1 || j.Progress.Total != 1 ||
								j.counts()["pages"] != 1 || j.counts()["attachments"] != 0 || j.counts()["restored"] != 0 ||
								j.Report == nil || j.Report.Placed == nil || j.Report.Placed.ParentID == nil || *j.Report.Placed.ParentID != wiki.raw {
								t.Fatalf("background source restore = %s", j)
							}
							if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 AND parent_id=$2 AND deleted_at IS NULL", source, wiki.raw) != 1 {
								t.Fatal("background source restore did not retain its original source parent")
							}
							tm.wrote(t, source, rewritten, 4)
						}},
					},
					{
						{"empty source create", nil, func() {
							empty = tm.createPage(t, "alice", wiki.id, wiki.raw, prefix+"Empty")
							tm.wrote(t, empty, "", 1)
						}},
						{"empty source first write", []string{"first"}, func() {
							tm.send(t, contentWrite("alice", empty, firstBody, 1, ""), http.StatusOK)
							tm.wrote(t, empty, firstBody, 2)
						}},
					},
					{
						{"source attachment upload", []string{"asset"}, func() {
							asset = tm.upload(t, "alice", wiki.id, wiki.raw, prefix+"Asset.bin", assetBody).ID
							if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 AND source_hash IS NOT NULL", asset) != 1 {
								t.Fatal("uploaded source attachment, including zero bytes, lacks a hash")
							}
						}},
						{"attachment out", []string{"entry_out", "out"}, func() { place(asset, "") }},
						{"attachment in", []string{"entry_out", "entry"}, func() { place(asset, wiki.raw) }},
						{"attachment delete", []string{"delete"}, func() { deleteNode(asset) }},
						{"attachment restore", []string{"restore"}, func() { restore(asset) }},
					},
					{
						{"ordinary page create", nil, func() {
							ordinary = tm.createPageWith(t, "alice", wiki.id, "", prefix+"Ordinary", ordinaryBody)
						}},
						{"page entry", []string{"entry_out", "entry"}, func() { place(ordinary, wiki.raw) }},
						{"page exit", []string{"entry_out", "out"}, func() { place(ordinary, "") }},
						{"page reentry", []string{"entry_out", "entry"}, func() { place(ordinary, wiki.raw) }},
						{"reorder anchor create", nil, func() {
							anchor = tm.createPage(t, "alice", wiki.id, wiki.raw, prefix+"Anchor")
						}},
						{"source reorder", []string{"manage", "reorder"}, func() {
							before := queryStrings(t, tm.pool, "SELECT sort_order::text FROM nodes WHERE id=$1", ordinary)
							c := request("alice", http.MethodPost, "/api/v0/nodes/"+ordinary+"/move",
								fmt.Sprintf(`{"parent_id":%q,"after_id":%q}`, wiki.raw, anchor))
							tm.send(t, c, http.StatusOK)
							after := queryStrings(t, tm.pool, "SELECT sort_order::text FROM nodes WHERE id=$1", ordinary)
							if len(before) != 1 || len(after) != 1 || before[0] == after[0] ||
								count(t, tm.pool, "SELECT count(*) FROM nodes n JOIN nodes a ON a.id=$2 WHERE n.id=$1 AND n.parent_id=a.parent_id AND n.sort_order>a.sort_order", ordinary, anchor) != 1 {
								t.Fatal("source reorder was a no-op or did not follow its anchor")
							}
						}},
					},
					{
						{"source import", []string{"import"}, func() {
							content := fmt.Sprintf("%s-import-%x\n", prefix, rng.Uint64())
							j := tm.imported(t, "alice", wiki.id, wiki.raw, vaultZip(t, vaultFile{prefix + "Import.md", content}))
							if j.State != "succeeded" || j.Report == nil || j.Report.Failure != nil || j.Report.Counts["pages"] != 1 || j.Report.Counts["skipped"] != 0 || j.Progress.Done != 1 || len(j.Problems) != 0 {
								t.Fatalf("source import = %+v", j)
							}
							if count(t, tm.pool, "SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.notebook_id=$1 AND n.parent_id=$2 AND n.name=$3 AND c.content=$4 AND n.deleted_at IS NULL", wiki.id, wiki.raw, prefix+"Import", content) != 1 {
								t.Fatal("successful import lacks its actual source page/body")
							}
						}},
					},
				}
				for len(queues) != 0 {
					i := rng.IntN(len(queues))
					run(queues[i][0])
					queues[i] = queues[i][1:]
					if len(queues[i]) == 0 {
						queues = append(queues[:i], queues[i+1:]...)
					}
				}
				// Genuine refused requests: no counters above are incremented.
				payload, err := json.Marshal(map[string]any{"parent_id": wiki.raw, "title": prefix + "Duplicate", "content": firstBody})
				if err != nil {
					t.Fatal(err)
				}
				refuse("duplicate source create", request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages", string(payload)), "page.source_duplicate", &empty)
				refuse("immutable second write", contentWrite("alice", empty, firstBody+"changed", 2, ""), "page.source_immutable", nil)
				refuse("duplicate attachment upload", assetUpload(t, "alice", wiki.id, wiki.raw, prefix+"Copy.bin", assetBody), "page.source_duplicate", &asset)

				if epoch == 0 {
					// Downgrade a populated wiki, then keep using its old sources.
					// The next epoch creates a fresh LLM Wiki in this same team.
					run(action{"prepare ordinary restore", nil, func() { deleteNode(source) }})
					run(action{"downgrade", []string{"downgrade"}, func() {
						status, body := tm.downgrade(t, "alice", wiki.id)
						if status != http.StatusOK {
							t.Fatalf("downgrade = %d %s", status, body)
						}
						if count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id=$1 AND mode='standard' AND source_root_id IS NULL", wiki.id) != 1 ||
							count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", wiki.id) != 0 {
							t.Fatal("downgrade did not clear mode/root/all source hashes")
						}
					}})
					run(action{"ordinary write after downgrade", []string{"standard_write"}, func() {
						tm.send(t, contentWrite("alice", empty, firstBody+"ordinary", 2, ""), http.StatusOK)
						tm.wrote(t, empty, firstBody+"ordinary", 3)
					}})
					run(action{"ordinary restore after downgrade", []string{"standard_restore"}, func() { restore(source) }})
				}
			}
			// Thirteen primary columns, with both entry directions and both
			// management forms asserted separately rather than hidden in totals.
			for _, category := range []string{"create", "first", "asset", "entry_out", "manage", "delete", "restore", "rewrite", "rewrite_undo", "undo_of_undo", "import", "background_restore", "background_revert", "entry", "out", "rename", "reorder"} {
				if counts[category] < 3 {
					t.Errorf("seed %d has %d successful %s steps, want >=3", seed, counts[category], category)
				}
			}
			if successes < 60 || counts["downgrade"] < 1 || counts["standard_write"] < 1 || counts["standard_restore"] < 1 {
				t.Errorf("seed %d: successful=%d categories=%v; need >=60 and downgrade followed by ordinary write/restore", seed, successes, counts)
			}
			if refusals != 9 {
				t.Errorf("seed %d: checked refusals=%d, want 9 outside success totals", seed, refusals)
			}
			t.Logf("seed=%d successful=%d refused=%d categories=%v", seed, successes, refusals, counts)
		})
	}
}

// Complete persisted rows for refused requests, not counts or production
// source readers. Stable jobs have settled before the before-image is taken.
func sourceRandomRows(t *testing.T, tm acmeTeam) map[string]string {
	t.Helper()
	rows := sourceRollbackRows(t, tm.pool)
	for _, table := range []string{"notebooks", "edit_sessions", "page_links", "page_aliases", "page_tags", "page_properties", "page_search_queue", "indexed_pages", "transfer_jobs"} {
		var data string
		if err := tm.pool.QueryRow(t.Context(), "SELECT coalesce(string_agg(to_jsonb(x)::text, E'\\n' ORDER BY to_jsonb(x)::text), '') FROM "+table+" x").Scan(&data); err != nil {
			t.Fatal(err)
		}
		rows[table] = data
	}
	var jobs string
	if err := tm.pool.QueryRow(t.Context(), `SELECT coalesce(string_agg(to_jsonb(j)::text, E'\n' ORDER BY to_jsonb(j)::text), '')
		FROM river_job j WHERE kind IN ('transfer.import', 'transfer.trash_restore', 'transfer.revert')`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	rows["writing river jobs"] = jobs
	return rows
}

// Preserve both file identity and bytes. A same-path overwrite must not evade
// a refusal assertion by leaving storedBlobs' key/path map unchanged.
func sourceRandomFiles(t *testing.T, tm acmeTeam) map[string]string {
	t.Helper()
	files := map[string]string{}
	for id, path := range storedBlobs(t, tm.storage) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[id] = fmt.Sprintf("%s:%x", path, sha256.Sum256(body))
	}
	for _, path := range storedImports(t, tm.storage) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files["import:"+path] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	return files
}
