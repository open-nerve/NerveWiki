package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

type ingestPurgeArgs struct{}

func (ingestPurgeArgs) Kind() string { return jobs.PurgeKind }

func TestIngestCleanup(t *testing.T) {
	for _, fixture := range []struct {
		name             string
		parents, sources int
	}{
		{"leaf_loop", 1, 2500}, {"count_return", 500, 3},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			awaitJob(t, tm.pool, jobs.PurgeKind)
			nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Gone wiki","mode":"llm_wiki"}`)
			ctx := context.Background()
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			var actor uuid.UUID
			if err := tm.pool.QueryRow(ctx, "SELECT id FROM users WHERE email='alice@example.com'").Scan(&actor); err != nil {
				t.Fatal(err)
			}
			for range fixture.parents {
				cs := uuid.NewV7()
				exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
					VALUES($1,$2,'edit','api',$3,now(),now())`, cs, nb.ID, actor)
				ids := make([]uuid.UUID, fixture.sources)
				for i := range ids {
					ids[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012x", i+1))
				}
				if got, err := page.NewIngestMarks(tm.pool).Add(ctx, uuid.MustParse(nb.ID), cs, ids); err != nil || got.Added != len(ids) {
					t.Fatalf("seed Add=%+v,%v", got, err)
				}
			}
			status, body := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/notebooks/"+nb.ID, tm.tokens["alice"], "")
			if status != http.StatusNoContent {
				t.Fatalf("delete LLM notebook=%d %s", status, body)
			}
			// The same job cutoff must find every row safely before retention.
			for _, table := range []string{"notebooks", "notebook_members", "nodes", "page_contents", "page_revisions", "changeset_items", "changesets"} {
				if table == "notebooks" {
					exec("UPDATE notebooks SET deleted_at=now()-interval '61 days' WHERE id=$1", nb.ID)
				} else {
					exec("UPDATE " + table + " SET deleted_at=now()-interval '61 days' WHERE deleted_at IS NOT NULL")
				}
			}
			prior := count(t, tm.pool, "SELECT coalesce(max(id),0) FROM river_job WHERE kind=$1", jobs.PurgeKind)
			inserter, err := jobs.NewInserter(tm.pool, slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatal(err)
			}
			tx, err := tm.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := inserter.InsertTx(ctx, tx, ingestPurgeArgs{}, nil); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(25 * time.Millisecond) {
				if n := count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind=$1 AND id>$2 AND cardinality(errors)>0", jobs.PurgeKind, prior); n != 0 {
					t.Fatalf("first purge run errored: %v", queryStrings(t, tm.pool, "SELECT errors::text FROM river_job WHERE kind=$1 AND id>$2", jobs.PurgeKind, prior))
				}
				if count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind=$1 AND id>$2 AND state='completed'", jobs.PurgeKind, prior) > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("purge job did not complete")
				}
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind=$1 AND id>$2 AND state='completed' AND attempt=1", jobs.PurgeKind, prior); n != 1 {
				t.Errorf("completed once without retry=%d, want1", n)
			}
			if leaves, parents, notebooks := count(t, tm.pool, "SELECT count(*) FROM changeset_sources"),
				count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", nb.ID),
				count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id=$1", nb.ID); leaves != 0 || parents != 0 || notebooks != 0 {
				t.Errorf("one River run leaves sources%d/changesets%d/notebooks%d, want0/0/0", leaves, parents, notebooks)
			}
		})
	}
}

func TestIngestInvariants(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Invariant")
	node := tm.createPage(t, "alice", nb, "", "Output")
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	var cs uuid.UUID
	if err := tm.pool.QueryRow(ctx, "SELECT changeset_id FROM page_revisions WHERE node_id=$1", node).Scan(&cs); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, rule string }{
		{"orphan", "ingest sources without a changeset"}, {"count", "ingest count differs from persistent sources"}, {"kind", "ingest sources on a non-edit changeset"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := cs
			if tt.name == "orphan" {
				id = uuid.NewV7()
			}
			exec("INSERT INTO changeset_sources VALUES($1,1,$2)", id, uuid.NewV7())
			if tt.name == "kind" {
				exec("UPDATE changesets SET kind='import',ingest_count=1 WHERE id=$1", cs)
			}
			violations := pageViolations(t, tm.pool)
			found := false
			for _, v := range violations {
				if strings.Contains(v, tt.rule) && strings.Contains(v, id.String()) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s counterexample %s absent from diagnostics: %v", tt.rule, id, violations)
			}
			exec("DELETE FROM changeset_sources WHERE changeset_id=$1", id)
			exec("UPDATE changesets SET kind='edit',ingest_count=0 WHERE id=$1", cs)
		})
	}
}

// Only the explicit same-run cutoff can excuse the short window between
// deleting the last source and its parent; retained history always forbids it.
func TestIngestInvariantCleanupWindow(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	// The purge runs when the server starts, then hourly. Its cutoff is later
	// than before, so its start run takes the changeset of every empty case
	// and of old_with_item before its item: run between a case's writes and
	// its checks, it deletes the row the checks are about. So the cases begin
	// after it.
	awaitJob(t, tm.pool, jobs.PurgeKind)
	nb := tm.openNotebook(t, "alice", "Window")
	node := tm.createPage(t, "alice", nb, "", "Output")
	ctx := context.Background()
	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	var actor, original uuid.UUID
	if err := tm.pool.QueryRow(ctx, "SELECT c.created_by_id,c.id FROM changesets c JOIN page_revisions r ON r.changeset_id=c.id WHERE r.node_id=$1", node).Scan(&actor, &original); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Truncate(time.Microsecond).Add(-1440 * time.Hour)
	for _, tt := range []struct {
		name                      string
		deleted                   *time.Time
		trimmed, history, allowed bool
	}{
		{"old_empty", new(before.Add(-time.Microsecond)), false, false, true},
		{"live_trimmed_empty", nil, true, false, true},
		{"at_cutoff", new(before), false, false, false},
		{"recent_trimmed", new(before.Add(time.Microsecond)), true, false, false},
		{"live_empty", nil, false, false, false},
		{"old_with_item", new(before.Add(-time.Microsecond)), false, true, false},
		{"old_with_revision", new(before.Add(-time.Microsecond)), false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cs := uuid.NewV7()
			if tt.name == "old_with_revision" {
				cs = original
			} else {
				exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
					VALUES($1,$2,'edit','api',$3,now(),now())`, cs, nb, actor)
			}
			exec(t, "UPDATE changesets SET ingest_count=1,deleted_at=$2,trimmed=$3 WHERE id=$1", cs, tt.deleted, tt.trimmed)
			if tt.name == "old_with_item" {
				exec(t, `INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,created_at,updated_at)
					VALUES(gen_random_uuid(),$1,$2,'Output',0,now(),now())`, cs, node)
			}
			has := func(found []string) bool {
				for _, v := range found {
					if strings.Contains(v, "ingest count differs from persistent sources") && strings.Contains(v, cs.String()) {
						return true
					}
				}
				return false
			}
			if !has(pageViolations(t, tm.pool)) {
				t.Error("default checker silently assumed a purge window")
			}
			if got := has(pageViolationsAt(t, tm.pool, &before)); got == tt.allowed {
				t.Errorf("same cutoff diagnostic=%v, cleanup exemption=%v", got, tt.allowed)
			}
			if cs == original {
				exec(t, "UPDATE changesets SET ingest_count=0,deleted_at=NULL,trimmed=false WHERE id=$1", cs)
			} else {
				exec(t, "DELETE FROM changeset_items WHERE changeset_id=$1", cs)
				exec(t, "DELETE FROM changesets WHERE id=$1", cs)
			}
		})
	}
	t.Run("existing_deleted_cs_and_purged_source", func(t *testing.T) {
		cs := uuid.NewV7()
		exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,deleted_at,ingest_count)
			VALUES($1,$2,'edit','api',$3,now(),now(),now(),1)`, cs, nb, actor)
		exec(t, "INSERT INTO changeset_sources VALUES($1,1,$2)", cs, uuid.NewV7())
		for _, v := range pageViolations(t, tm.pool) {
			if strings.Contains(v, "ingest ") {
				t.Errorf("valid persistent mark flagged: %s", v)
			}
		}
	})
}

// The held changeset belongs to a live notebook. Another deleted notebook is
// independently eligible and deleted through REST; SQL gives the held cs its
// deleted/trimmed eligibility, so this claims cleanup interleaving only.
// A retained cross-module FK intentionally makes a
// deleted parent retry, so it must not be mistaken for a skip-locked leaf.
func TestIngestCleanupHeldChangesetJob(t *testing.T) {
	for _, shape := range []string{"deleted", "trimmed"} {
		t.Run(shape, func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			awaitJob(t, tm.pool, jobs.PurgeKind)
			heldBook := tm.openNotebook(t, "alice", "Held live notebook")
			gone := tm.createWith(t, tm.tokens["alice"], `{"name":"Other gone wiki","mode":"llm_wiki"}`)
			ctx := context.Background()
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := tm.pool.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			var actor uuid.UUID
			if err := tm.pool.QueryRow(ctx, "SELECT id FROM users WHERE email='alice@example.com'").Scan(&actor); err != nil {
				t.Fatal(err)
			}
			held := uuid.NewV7()
			exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
    VALUES($1,$2,'edit','api',$3,now(),now())`, held, heldBook, actor)
			ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
			if got, err := page.NewIngestMarks(tm.pool).Add(ctx, uuid.MustParse(heldBook), held, ids); err != nil || got.Count != 3 {
				t.Fatalf("held real Add=%+v,%v", got, err)
			}
			exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,ingest_count)
    SELECT uuidv7(),$1,'edit','api',$2,now(),now(),3 FROM generate_series(1,500)`, gone.ID, actor)
			exec(`INSERT INTO changeset_sources(changeset_id,seq,node_id)
    SELECT c.id,s.k,($2::uuid[])[s.k] FROM changesets c,generate_series(1,3) s(k)
    WHERE c.notebook_id=$1 AND c.ingest_count=3`, gone.ID, ids)
			status, body := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/notebooks/"+gone.ID, tm.tokens["alice"], "")
			if status != http.StatusNoContent {
				t.Fatalf("delete other notebook=%d %s", status, body)
			}
			exec("UPDATE notebooks SET deleted_at=now()-interval '61 days' WHERE id=$1", gone.ID)
			for _, table := range []string{"notebook_members", "nodes", "page_contents", "page_revisions", "changeset_items", "changesets"} {
				exec("UPDATE " + table + " SET deleted_at=now()-interval '61 days' WHERE deleted_at IS NOT NULL")
			}
			if shape == "deleted" {
				exec("UPDATE changesets SET deleted_at=now()-interval '61 days' WHERE id=$1", held)
			} else {
				exec("UPDATE changesets SET trimmed=true WHERE id=$1", held)
				exec("UPDATE changesets SET deleted_at=NULL,trimmed=true WHERE notebook_id=$1", gone.ID)
			}
			holder, err := tm.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(ctx) }()
			var pid int
			if err := holder.QueryRow(ctx, "SELECT pg_backend_pid() FROM changesets WHERE id=$1 FOR NO KEY UPDATE", held).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			run := func(observeHeld bool) {
				t.Helper()
				prior := count(t, tm.pool, "SELECT coalesce(max(id),0) FROM river_job WHERE kind=$1", jobs.PurgeKind)
				inserter, err := jobs.NewInserter(tm.pool, slog.New(slog.DiscardHandler))
				if err != nil {
					t.Fatal(err)
				}
				tx, err := tm.pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if err = inserter.InsertTx(ctx, tx, ingestPurgeArgs{}, nil); err != nil {
					_ = tx.Rollback(ctx)
					t.Fatal(err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(25 * time.Millisecond) {
					if observeHeld && count(t, tm.pool, "SELECT count(*) FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))", pid) > 0 {
						t.Fatal("River purge waits for held changeset instead of clearing independent 500x3")
					}
					if count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind=$1 AND id>$2 AND cardinality(errors)>0", jobs.PurgeKind, prior) > 0 {
						t.Fatalf("River purge first attempt errored: %v", queryStrings(t, tm.pool, "SELECT errors::text FROM river_job WHERE kind=$1 AND id>$2", jobs.PurgeKind, prior))
					}
					if count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind=$1 AND id>$2 AND state='completed' AND attempt=1", jobs.PurgeKind, prior) == 1 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("River purge did not complete")
					}
				}
			}
			run(true)
			if leaves, parents, notebooks := count(t, tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id<>$1", held),
				count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", gone.ID),
				count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id=$1", gone.ID); leaves != 0 || parents != 0 || notebooks != 0 {
				t.Errorf("same River run left independent sources%d/parents%d/notebooks%d, want0/0/0", leaves, parents, notebooks)
			}
			if marks, parent := count(t, tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", held),
				count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id=$1 AND ingest_count=3", held); marks != 3 || parent != 1 {
				t.Errorf("held sources%d/parent%d, want3/1", marks, parent)
			}
			if err := holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			run(false)
			if n := count(t, tm.pool, "SELECT count(*) FROM changeset_sources"); n != 0 {
				t.Errorf("next River run left%d sources", n)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id=$1", held); n != 0 {
				t.Errorf("next River run left%d held changesets", n)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id=$1 AND deleted_at IS NULL", heldBook); n != 1 {
				t.Errorf("live notebook count%d, want1", n)
			}
		})
	}
}
