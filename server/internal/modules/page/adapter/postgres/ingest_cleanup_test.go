package postgresadapter_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func seedIngestGarbage(t *testing.T, f fixture, changesets, sources int, before time.Time) {
	t.Helper()
	f.exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,deleted_at,ingest_count)
		SELECT gen_random_uuid(),$1,'edit','api',$2,$3,$3,$4,$5 FROM generate_series(1,$6::integer)`,
		f.eng, f.alice, now(), before.Add(-time.Microsecond), sources, changesets)
	f.exec(t, `INSERT INTO changeset_sources(changeset_id,seq,node_id)
		SELECT c.id,n,gen_random_uuid() FROM changesets c CROSS JOIN generate_series(1,$1::integer) n
		WHERE c.ingest_count=$1 AND c.id NOT IN (SELECT changeset_id FROM changeset_sources)`, sources)
}

func TestIngestCleanup(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name                   string
		parents, sources, want int
	}{
		{"leaf_loop", 1, 2500, 2501}, {"count_return", 500, 3, 2000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			seedIngestGarbage(t, f, tt.parents, tt.sources, cutoff())
			got, err := f.s.PurgeChangesets(ctx, cutoff(), 1000)
			if err != nil || got != tt.want {
				t.Errorf("one call deleted=%d,%v; want exact leaf+parent%d", got, err, tt.want)
			}
			if leaves, parents := f.count(t, "SELECT count(*) FROM changeset_sources"), f.count(t, "SELECT count(*) FROM changesets"); leaves != 0 || parents != 0 {
				t.Errorf("one call left%d sources/%d parents; want both0", leaves, parents)
			}
		})
	}
	for _, name := range []string{"live", "at_before", "recent", "trimmed_recent", "with_item", "with_version", "deleted", "trimmed"} {
		t.Run("eligible_"+name, func(t *testing.T) {
			f := newFixture(t)
			cs := f.changeset(t, f.eng, f.alice, domain.ClientAPI, now()).ID
			if _, err := f.s.Add(ctx, f.eng, cs, []uuid.UUID{ingestID(1)}); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "at_before":
				f.exec(t, "UPDATE changesets SET deleted_at=$2 WHERE id=$1", cs, cutoff())
			case "recent", "trimmed_recent":
				f.exec(t, "UPDATE changesets SET deleted_at=$2,trimmed=$3 WHERE id=$1", cs, cutoff().Add(time.Microsecond), name == "trimmed_recent")
			case "deleted", "with_item", "with_version":
				f.exec(t, "UPDATE changesets SET deleted_at=$2 WHERE id=$1", cs, cutoff().Add(-time.Microsecond))
			case "trimmed":
				f.exec(t, "UPDATE changesets SET trimmed=true WHERE id=$1", cs)
			}
			if name == "with_item" || name == "with_version" {
				node := f.page(t, f.eng, nil, "Retained", 0)
				if name == "with_item" {
					state := node.State()
					f.item(t, cs, now(), domain.Change{NodeID: node.ID, After: &state})
				} else {
					f.version(t, cs, node.ID, nil, 1, "retained")
				}
			}
			want := 0
			if name == "deleted" || name == "trimmed" {
				want = 2
			}
			if got, err := f.s.PurgeChangesets(ctx, cutoff(), 10); err != nil || got != want {
				t.Errorf("eligibility %s=%d,%v; want%d", name, got, err, want)
			}
			left := 1
			if want == 2 {
				left = 0
			}
			if f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != left ||
				f.count(t, "SELECT count(*) FROM changesets WHERE id=$1", cs) != left {
				t.Errorf("%s deleted a retained mark or left eligible garbage", name)
			}
		})
	}
	t.Run("source_purge_keeps_mark", func(t *testing.T) {
		f := newFixture(t)
		node := f.page(t, f.eng, nil, "Source", 0)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientAPI, now()).ID
		if _, err := f.s.Add(ctx, f.eng, cs, []uuid.UUID{node.ID}); err != nil {
			t.Fatal(err)
		}
		f.exec(t, "DELETE FROM page_contents WHERE node_id=$1", node.ID)
		f.exec(t, "DELETE FROM nodes WHERE id=$1", node.ID)
		if got, err := f.s.PurgeChangesets(ctx, cutoff(), 10); err != nil || got != 0 {
			t.Fatalf("live cs purge=%d,%v", got, err)
		}
		assertIngestRows(t, f, cs, []uuid.UUID{node.ID})
	})
	for _, shape := range []string{"deleted", "trimmed"} {
		for _, held := range []string{"source", "changeset"} {
			t.Run("held_"+held+"_"+shape, func(t *testing.T) {
				f := newFixture(t)
				cs := f.changeset(t, f.eng, f.alice, domain.ClientAPI, now()).ID
				if _, err := f.s.Add(ctx, f.eng, cs, []uuid.UUID{ingestID(1)}); err != nil {
					t.Fatal(err)
				}
				if shape == "deleted" {
					f.exec(t, "UPDATE changesets SET deleted_at=$2 WHERE id=$1", cs, cutoff().Add(-time.Second))
				} else {
					f.exec(t, "UPDATE changesets SET trimmed=true WHERE id=$1", cs)
				}
				seedIngestGarbage(t, f, 500, 3, cutoff())
				tx, err := f.pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = tx.Rollback(ctx) })
				var holder int
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&holder); err != nil {
					t.Fatal(err)
				}
				sql := "SELECT 1 FROM changeset_sources WHERE changeset_id=$1 FOR UPDATE"
				if held == "changeset" {
					sql = "SELECT 1 FROM changesets WHERE id=$1 FOR NO KEY UPDATE"
				}
				if _, err := tx.Exec(ctx, sql, cs); err != nil {
					t.Fatal(err)
				}
				type result struct {
					n   int
					err error
				}
				done := make(chan result, 1)
				go func() { n, err := f.s.PurgeChangesets(ctx, cutoff(), 1000); done <- result{n, err} }()
				var got result
				finished, blocked := false, false
				deadline := time.Now().Add(5 * time.Second)
				for !finished && !blocked && time.Now().Before(deadline) {
					select {
					case got = <-done:
						finished = true
					default:
						blocked = f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))", holder) > 0
						if !blocked {
							time.Sleep(time.Millisecond)
						}
					}
				}
				if blocked {
					t.Errorf("purge visibly waited on held %s (backend%d); SKIP LOCKED must progress", held, holder)
				}
				if finished {
					if got.err != nil || got.n != 2000 {
						t.Errorf("unheld500 cs/1500 marks=%d,%v; want2000", got.n, got.err)
					}
					if f.count(t, "SELECT count(*) FROM changesets WHERE id=$1", cs) != 1 ||
						f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != 1 {
						t.Errorf("held %s did not retain its mark and %s parent", held, shape)
					}
					if n := f.count(t, "SELECT count(*) FROM changesets WHERE id<>$1", cs); n != 0 {
						t.Errorf("%d unrelated parents remain", n)
					}
				}
				if err := tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				if !finished {
					select {
					case got = <-done:
					case <-time.After(5 * time.Second):
						t.Fatal("fixture did not finish after lock release")
					}
					if !blocked {
						t.Fatalf("fixture neither completed nor exposed a lock wait: %+v", got)
					}
					return
				}
				if n, err := f.s.PurgeChangesets(ctx, cutoff(), 1000); err != nil || n != 2 {
					t.Errorf("after release=%d,%v; want mark+parent2", n, err)
				}
				if n := f.count(t, "SELECT count(*) FROM changeset_sources"); n != 0 {
					t.Errorf("after release leaves%d", n)
				}
				t.Logf("held row %s/%s, unheld deletion count=%d, error=%v", held, shape, got.n, got.err)
			})
		}
	}
}
