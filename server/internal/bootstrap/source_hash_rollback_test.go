package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// An attachment's callback runs after the node's hash is written. A failure
// within that callback must roll back both modules' rows. An infrastructure
// error retains the uploaded file for the orphan sweep, as M7 specifies;
// it is distinct from a known domain refusal that discards the file at once.
func TestSourceHashRollsBackWhenAttachmentCallbackFails(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.llmWiki(t, "Callback rollback")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Kept", "kept page")
	tm.upload(t, "alice", wiki.id, wiki.raw, "kept.bin", "kept file")
	before, files := sourceRollbackRows(t, tm.pool), storedBlobs(t, tm.storage)
	for _, statement := range []string{
		"CREATE SEQUENCE source_callback_reached",
		`CREATE FUNCTION refuse_source_callback() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM nodes n JOIN asset_blobs b ON b.node_id = n.id
				WHERE n.id = NEW.node_id AND n.notebook_id = NEW.notebook_id AND b.id = NEW.id
				AND n.deleted_at IS NULL AND n.source_hash = NEW.sha256) THEN
				RAISE EXCEPTION 'callback did not see the maintained source hash and inserted blob';
			END IF;
			PERFORM nextval('source_callback_reached');
			RAISE EXCEPTION 'injected failure after attachment callback insert';
		END $$`,
		"CREATE TRIGGER refuse_source_callback AFTER INSERT ON asset_blobs FOR EACH ROW EXECUTE FUNCTION refuse_source_callback()",
	} {
		if _, err := tm.pool.Exec(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	const content = "the failed callback's file"
	tm.send(t, assetUpload(t, "alice", wiki.id, wiki.raw, "callback.bin", content), http.StatusInternalServerError)
	var called bool
	var attempts int
	if err := tm.pool.QueryRow(t.Context(), "SELECT is_called, last_value FROM source_callback_reached").Scan(&called, &attempts); err != nil {
		t.Fatal(err)
	}
	if !called || attempts != 1 {
		t.Errorf("callback saw maintained hash and inserted row = %t, attempts %d; want true, 1", called, attempts)
	}
	if after := sourceRollbackRows(t, tm.pool); !maps.Equal(after, before) {
		t.Error("attachment callback failure changed committed node, content, history or blob rows")
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)

	// Only this isolated app wrote a file since the snapshot. Verify that
	// single extra file's bytes before removing our injected failure's orphan;
	// existing blobs must be untouched. This is test cleanup, not an assertion
	// that production cleans infrastructure failures synchronously.
	afterFiles := storedBlobs(t, tm.storage)
	for id, path := range files {
		if afterFiles[id] != path {
			t.Fatalf("callback failure changed existing blob %s", id)
		}
	}
	if len(afterFiles) != len(files)+1 {
		t.Fatalf("callback failure left %d files, want %d existing and one orphan", len(afterFiles), len(files))
	}
	for id, path := range afterFiles {
		if _, existed := files[id]; existed {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			t.Fatalf("owned orphan %s = %q, %v; want uploaded bytes", id, data, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	checkAssets(t, tm.pool, tm.storage)
	if _, err := tm.pool.Exec(t.Context(), "DROP TRIGGER refuse_source_callback ON asset_blobs"); err != nil {
		t.Fatal(err)
	}
	tm.upload(t, "alice", wiki.id, wiki.raw, "callback.bin", content)
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// A participant observes both the maintained hash and recorded item in the
// real transaction, then fails. No part of that source creation may commit.
func TestSourceHashRollsBackWhenParticipantFails(t *testing.T) {
	part := &sourceRollbackParticipant{saw: make(chan string, 1)}
	tm := newAcmeTeamWiring(t, "", "", wiring{page: func(d *page.Deps) {
		part.pool = d.Pool
		d.Participants = append(d.Participants, part)
	}})
	wiki := tm.llmWiki(t, "Participant rollback")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Kept", "kept source")
	before := sourceRollbackRows(t, tm.pool)
	part.active.Store(true)
	const body = `{"parent_id":"%s","title":"Rejected","content":"participant body"}`
	tm.send(t, request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages", fmt.Sprintf(body, wiki.raw)), http.StatusInternalServerError)
	select {
	case id := <-part.saw:
		if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1", id) != 0 {
			t.Error("participant's refused source node committed")
		}
	default:
		t.Error("participant never observed the maintained hash and recorded item in the transaction")
	}
	if after := sourceRollbackRows(t, tm.pool); !maps.Equal(after, before) {
		t.Error("participant failure changed committed node, content or history rows")
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	part.active.Store(false)
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Rejected", "participant body")
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
}

type sourceRollbackParticipant struct {
	pool   *pgxpool.Pool
	active atomic.Bool
	saw    chan string
}

func (p *sourceRollbackParticipant) Participate(ctx context.Context, s page.Step, _ page.Appender) error {
	if !p.active.Load() {
		return nil
	}
	if !postgres.InTx(ctx) || len(s.Changes) != 1 {
		return errors.New("source participant is outside its one-node transaction")
	}
	var hash []byte
	var content string
	var items int
	id := s.Changes[0].NodeID
	if err := postgres.DB(ctx, p.pool).QueryRow(ctx, `SELECT n.source_hash, c.content,
		(SELECT count(*) FROM changeset_items i WHERE i.node_id = n.id)
		FROM nodes n JOIN page_contents c ON c.node_id = n.id
		WHERE n.id = $1 AND n.notebook_id = $2`, id, s.NotebookID).Scan(&hash, &content, &items); err != nil {
		return err
	}
	want := sha256.Sum256([]byte("participant body"))
	if content != "participant body" || !bytes.Equal(hash, want[:]) || items != 1 {
		return errors.New("source participant did not see the maintained body hash and recorded item")
	}
	p.saw <- id.String()
	return errors.New("injected participant failure after source hash and item")
}

// Capture complete rows rather than deriving expectations through source
// readers or the production membership calculation.
func sourceRollbackRows(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
		var rows string
		if err := pool.QueryRow(t.Context(), "SELECT coalesce(string_agg(to_jsonb(x)::text, E'\\n' ORDER BY to_jsonb(x)::text), '') FROM "+table+" x").Scan(&rows); err != nil {
			t.Fatal(err)
		}
		out[table] = rows
	}
	return out
}
