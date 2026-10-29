package bootstrap

import (
	"maps"
	"net/http"
	"testing"
)

// The final blob observer sees the restored nodes and their prepared hashes
// in its transaction. An error there must still roll the entire restore back.
func TestSourceRestoresRollBackWhenTheBlobObserverFails(t *testing.T) {
	for _, route := range []string{"restore", "revert"} {
		t.Run(route, func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			wiki := tm.llmWiki(t, "Restore rollback")
			root := tm.createPage(t, "alice", wiki.id, wiki.raw, "Folder")
			tm.createPageWith(t, "alice", wiki.id, root, "Page", "page bytes")
			tm.upload(t, "alice", wiki.id, root, "file.bin", "asset bytes")
			tm.send(t, nodeDeletion("alice", root), http.StatusNoContent)
			deleted := tm.newest(t, wiki.id).ID
			before := sourceRollbackRows(t, tm.pool)
			for _, statement := range []string{
				"CREATE SEQUENCE restore_observer_reached",
				`CREATE FUNCTION refuse_restore_observer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
					IF NOT EXISTS (SELECT 1 FROM nodes WHERE id = NEW.node_id AND deleted_at IS NULL
						AND source_hash = NEW.sha256) THEN
						RAISE EXCEPTION 'restore observer did not see the maintained source hash';
					END IF;
					PERFORM nextval('restore_observer_reached');
					RAISE EXCEPTION 'injected failure in restored blob observer';
				END $$`,
				`CREATE TRIGGER refuse_restore_observer AFTER UPDATE ON asset_blobs FOR EACH ROW
					WHEN (OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL)
					EXECUTE FUNCTION refuse_restore_observer()`,
			} {
				if _, err := tm.pool.Exec(t.Context(), statement); err != nil {
					t.Fatal(err)
				}
			}
			command := nodeRestore("alice", root)
			if route == "revert" {
				command = changesetRevert("alice", deleted)
			}
			tm.send(t, command, http.StatusInternalServerError)
			var reached bool
			var attempts int
			if err := tm.pool.QueryRow(t.Context(), "SELECT is_called, last_value FROM restore_observer_reached").Scan(&reached, &attempts); err != nil {
				t.Fatal(err)
			}
			if !reached || attempts != 1 {
				t.Errorf("observer saw maintained hash = %t, attempts=%d; want true/1", reached, attempts)
			}
			if !maps.Equal(sourceRollbackRows(t, tm.pool), before) {
				t.Error("failed observer committed some node, content, history or blob rows")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
			if _, err := tm.pool.Exec(t.Context(), "DROP TRIGGER refuse_restore_observer ON asset_blobs"); err != nil {
				t.Fatal(err)
			}
			tm.send(t, command, http.StatusOK)
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
			checkLinks(t, tm.pool)
		})
	}
}
