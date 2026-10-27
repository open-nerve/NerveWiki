package bootstrap

import (
	"bytes"
	"crypto/sha256"
	"maps"
	"net/http"
	"os"
	"testing"
)

// Upload's step has fresh facts, whereas an existing attachment entering or
// returning to the source area must use the complete cross-module SHA bridge.
func TestSourceAssetHashesFollowUploadMoveAndRestore(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.llmWiki(t, "Attachment sources")
	folder := tm.createPage(t, "alice", wiki.id, "", "Folder")
	child := tm.createPage(t, "alice", wiki.id, folder, "Child")
	var assets []struct{ id, body string }
	for _, row := range []struct{ parent, name, body string }{
		{wiki.raw, "fresh.txt", "fresh source"}, {wiki.raw, "fresh-empty.bin", ""},
		{child, "existing.txt", "existing source"}, {child, "existing-empty.bin", ""},
	} {
		a := tm.upload(t, "alice", wiki.id, row.parent, row.name, row.body)
		assets = append(assets, struct{ id, body string }{a.ID, row.body})
	}
	check := func(existingInside bool) {
		t.Helper()
		for i, a := range assets {
			var got []byte
			if err := tm.pool.QueryRow(t.Context(), "SELECT source_hash FROM nodes WHERE id = $1", a.id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			var want []byte
			if i < 2 || existingInside {
				sum := sha256.Sum256([]byte(a.body))
				want = sum[:]
			}
			if !bytes.Equal(got, want) {
				t.Errorf("attachment %s source hash %x, want %x", a.id, got, want)
			}
		}
		assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
	}
	check(false)
	tm.send(t, nodeMove("alice", folder, wiki.raw), http.StatusOK)
	check(true)
	tm.send(t, nodeDeletion("alice", folder), http.StatusNoContent)
	check(false)
	tm.send(t, nodeRestore("alice", folder), http.StatusOK)
	check(true)
	tm.send(t, nodeMove("alice", folder, ""), http.StatusOK)
	check(false)
}

func TestSourceHashWriteFailureRollsBackPageAndAsset(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.llmWiki(t, "Rollback sources")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Kept", "persisted source")
	tm.upload(t, "alice", wiki.id, wiki.raw, "kept.bin", "")
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	digests := func() map[string]string {
		t.Helper()
		got := map[string]string{}
		for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
			var digest string
			if err := tm.pool.QueryRow(t.Context(), "SELECT md5(coalesce(string_agg(row_to_json(x)::text, '' ORDER BY row_to_json(x)::text), '')) FROM "+table+" x").Scan(&digest); err != nil {
				t.Fatal(err)
			}
			got[table] = digest
		}
		return got
	}
	before, files := digests(), storedBlobs(t, tm.storage)
	for _, statement := range []string{
		"CREATE SEQUENCE source_hash_attempts",
		`CREATE FUNCTION refuse_source_hash() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
			IF NEW.source_hash IS NOT NULL AND NEW.source_hash IS DISTINCT FROM OLD.source_hash THEN
				PERFORM nextval('source_hash_attempts'); RAISE EXCEPTION 'source hash write refused';
			END IF; RETURN NEW; END $$`,
		"CREATE TRIGGER refuse_source_hash BEFORE UPDATE OF source_hash ON nodes FOR EACH ROW EXECUTE FUNCTION refuse_source_hash()",
	} {
		if _, err := tm.pool.Exec(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	for i, operation := range []step{
		request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages", `{"parent_id":"`+wiki.raw+`","title":"Refused","content":"body"}`),
		assetUpload(t, "alice", wiki.id, wiki.raw, "refused.bin", "bytes"),
	} {
		tm.send(t, operation, http.StatusInternalServerError)
		if after := digests(); !maps.Equal(after, before) {
			t.Errorf("failed source write changed persisted rows: before %v after %v", before, after)
		}
		// Upload retains a file after an infrastructure error because a COMMIT
		// outcome could be unknown. This test knows the transaction rolled back;
		// observe precisely this attempt's orphan, then remove only its file.
		after := storedBlobs(t, tm.storage)
		orphans := 0
		for id, path := range after {
			if kept, ok := files[id]; ok {
				if path != kept {
					t.Errorf("existing blob %s moved", id)
				}
				continue
			}
			orphans++
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		if orphans != i || !maps.Equal(storedBlobs(t, tm.storage), files) {
			t.Errorf("failed operation %d left %d owned orphans, want %d and existing blobs unchanged", i, orphans, i)
		}
		assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	}
	if got := count(t, tm.pool, "SELECT last_value FROM source_hash_attempts"); got != 2 {
		t.Errorf("source hash write attempts = %d, want page and asset", got)
	}
	if _, err := tm.pool.Exec(t.Context(), "DROP TRIGGER refuse_source_hash ON nodes"); err != nil {
		t.Fatal(err)
	}
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Retried", "body")
	tm.upload(t, "alice", wiki.id, wiki.raw, "retried.bin", "bytes")
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}
