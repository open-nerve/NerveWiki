package bootstrap

import (
	"crypto/sha256"
	"net/http"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The audit trigger observes physical row updates, independently of the final
// source-hash rebuild. Restoring first and assigning the same nodes' hashes in
// a second UPDATE leaves correct final values but doubles most node writes.
// Both the hash and the live transition must belong to the first row update.
func TestSourceRestoresWriteEachNodeOnce(t *testing.T) {
	for _, area := range []string{"source", "outside", "standard"} {
		for _, route := range []string{"restore", "revert", "background_restore", "background_revert"} {
			t.Run(area+"/"+route, func(t *testing.T) {
				tm := newAcmeTeamWiring(t, "", "", wiring{batch: transfer.Batch{Nodes: 2}})
				wiki := tm.llmWiki(t, "Restoration writes")
				if area == "standard" {
					wiki = tm.standardTwin(t, "Standard restoration writes", wiki)
				}
				parent := wiki.raw
				if area == "outside" {
					parent = ""
				}
				root := tm.createPage(t, "alice", wiki.id, parent, "Folder")
				page := tm.createPageWith(t, "alice", wiki.id, root, "Page", "restored page bytes")
				empty := tm.createPage(t, "alice", wiki.id, root, "Empty")
				asset := tm.upload(t, "alice", wiki.id, root, "asset.bin", "restored attachment bytes").ID
				zero := tm.upload(t, "alice", wiki.id, root, "empty.bin", "").ID
				tm.send(t, nodeDeletion("alice", root), http.StatusNoContent)
				deleted := tm.newest(t, wiki.id).ID
				for _, sql := range []string{
					`CREATE TABLE restored_node_updates (id uuid NOT NULL, was_deleted boolean NOT NULL,
						before_hash bytea, after_hash bytea)`,
					`CREATE FUNCTION audit_restored_node_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
						INSERT INTO restored_node_updates VALUES
							(NEW.id, OLD.deleted_at IS NOT NULL, OLD.source_hash, NEW.source_hash);
						RETURN NEW;
					END $$`,
					// A background restore may mark its still-deleted frontier.
					// Count all writes leaving a live node, including a later hash write.
					`CREATE TRIGGER audit_restored_node_update AFTER UPDATE ON nodes FOR EACH ROW
						WHEN (NEW.deleted_at IS NULL) EXECUTE FUNCTION audit_restored_node_update()`,
				} {
					if _, err := tm.pool.Exec(t.Context(), sql); err != nil {
						t.Fatal(err)
					}
				}
				switch route {
				case "restore":
					tm.send(t, nodeRestore("alice", root), http.StatusOK)
				case "revert":
					tm.reverted(t, "alice", deleted)
				case "background_restore", "background_revert":
					request := trashRestoreStart("alice", wiki.id, root)
					if route == "background_revert" {
						request = revertStart("alice", wiki.id, deleted, "")
					}
					job := tm.background(t, request)
					if job.State != "succeeded" || job.Progress.Done != 5 {
						t.Fatalf("restore job = %s, want all five nodes restored", job)
					}
				}
				for _, row := range []struct {
					id, body string
					eligible bool
				}{{root, "", false}, {page, "restored page bytes", true}, {empty, "", false},
					{asset, "restored attachment bytes", true}, {zero, "", true}} {
					var hash []byte
					if area == "source" && row.eligible {
						sum := sha256.Sum256([]byte(row.body))
						hash = sum[:]
					}
					var writes, together int
					if err := tm.pool.QueryRow(t.Context(), `SELECT count(*), count(*) FILTER
						(WHERE was_deleted AND before_hash IS NULL AND after_hash IS NOT DISTINCT FROM $2::bytea)
						FROM restored_node_updates WHERE id = $1`, row.id, hash).Scan(&writes, &together); err != nil {
						t.Fatal(err)
					}
					if writes != 1 || together != 1 {
						t.Errorf("restored node %s: writes=%d with live transition and intended hash=%d, want 1/1", row.id, writes, together)
					}
				}
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				checkPages(t, tm.pool)
				checkAssets(t, tm.pool, tm.storage)
				checkLinks(t, tm.pool)
			})
		}
	}
}
