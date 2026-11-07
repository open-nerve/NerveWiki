package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func ingestID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012x", n))
}

// A no-op, a reordering append, a lost transaction or a swallowed database
// error each breaks one of these independent persisted-state assertions.
func TestIngestMarks(t *testing.T) {
	ctx := context.Background()
	t.Run("append", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		a, b, c := ingestID(20), ingestID(30), ingestID(10)
		for _, tt := range []struct {
			ids  []uuid.UUID
			want app.IngestMarkResult
			rows []uuid.UUID
		}{
			{[]uuid.UUID{a, b}, app.IngestMarkResult{Added: 2, Count: 2}, []uuid.UUID{a, b}},
			{[]uuid.UUID{c}, app.IngestMarkResult{Added: 1, Count: 3}, []uuid.UUID{a, b, c}},
			{[]uuid.UUID{c, a, b}, app.IngestMarkResult{Added: 0, Count: 3}, []uuid.UUID{a, b, c}},
			{nil, app.IngestMarkResult{Added: 0, Count: 3}, []uuid.UUID{a, b, c}},
		} {
			got, err := f.s.Add(ctx, f.eng, cs, tt.ids)
			if err != nil || got != tt.want {
				t.Fatalf("Add(%v) = %+v, %v; want %+v", tt.ids, got, err, tt.want)
			}
			assertIngestRows(t, f, cs, tt.rows)
		}
	})
	t.Run("cap", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		f.exec(t, "INSERT INTO changeset_sources SELECT $1, n, gen_random_uuid() FROM generate_series(1,9998) n", cs)
		f.exec(t, "UPDATE changesets SET ingest_count=9998 WHERE id=$1", cs)
		ids := []uuid.UUID{ingestID(1), ingestID(2), ingestID(3), ingestID(4)}
		got, err := f.s.Add(ctx, f.eng, cs, ids)
		if err != nil || got != (app.IngestMarkResult{Added: 2, Count: 10000}) {
			t.Fatalf("near cap = %+v, %v", got, err)
		}
		if f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != 10000 ||
			f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1 AND ((seq=9999 AND node_id=$2) OR (seq=10000 AND node_id=$3))", cs, ids[0], ids[1]) != 2 ||
			f.count(t, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != 10000 {
			t.Fatal("cap did not persist exactly the smallest two new sources")
		}
		got, err = f.s.Add(ctx, f.eng, cs, ids)
		if err != nil || got != (app.IngestMarkResult{Count: 10000}) {
			t.Fatalf("full cap = %+v, %v", got, err)
		}
	})
	t.Run("owned_edit", func(t *testing.T) {
		f := newFixture(t)
		source := []uuid.UUID{ingestID(1)}
		for _, kind := range []domain.ChangesetKind{domain.ChangesetImport, domain.ChangesetVersionRestore, domain.ChangesetTrashRestore, domain.ChangesetRevert} {
			cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
			var reverts *uuid.UUID
			if kind == domain.ChangesetRevert {
				reverts = new(uuid.NewV7())
			}
			f.exec(t, "UPDATE changesets SET kind=$2,reverts_id=$3 WHERE id=$1", cs, string(kind), reverts)
			if _, err := f.s.Add(ctx, f.eng, cs, source); err == nil {
				t.Errorf("non-edit %s accepted", kind)
			}
		}
		live := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		if _, err := f.s.Add(ctx, f.ops, live, source); err == nil {
			t.Error("other notebook accepted")
		}
		if _, err := f.s.Add(ctx, f.eng, uuid.NewV7(), source); err == nil {
			t.Error("missing changeset accepted")
		}
		f.exec(t, "UPDATE changesets SET deleted_at=$2 WHERE id=$1", live, now())
		if _, err := f.s.Add(ctx, f.eng, live, source); err == nil {
			t.Error("deleted changeset accepted")
		}
		if f.count(t, "SELECT count(*) FROM changeset_sources") != 0 {
			t.Fatal("refused changesets gained marks")
		}
	})
	t.Run("sorted_unique", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		for _, ids := range [][]uuid.UUID{{ingestID(2), ingestID(1)}, {ingestID(1), ingestID(1)}} {
			if _, err := f.s.Add(ctx, f.eng, cs, ids); err == nil {
				t.Errorf("non-canonical input %v accepted", ids)
			}
		}
		assertIngestRows(t, f, cs, []uuid.UUID{})
	})
	t.Run("rollback", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		abort := errors.New("abort ingest")
		err := postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(txctx context.Context) error {
			got, err := f.s.Add(txctx, f.eng, cs, []uuid.UUID{ingestID(1)})
			if err != nil {
				return err
			}
			var persisted int
			if err := postgres.DB(txctx, f.pool).QueryRow(txctx, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs).Scan(&persisted); err != nil {
				return err
			}
			if got != (app.IngestMarkResult{Added: 1, Count: 1}) || persisted != 1 {
				t.Errorf("transaction append = %+v, rows=%d", got, persisted)
			}
			if f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != 0 {
				t.Error("another connection saw uncommitted marks")
			}
			return abort
		})
		if !errors.Is(err, abort) {
			t.Fatalf("rollback error=%v", err)
		}
		assertIngestRows(t, f, cs, []uuid.UUID{})
	})
	t.Run("driver_error", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		f.exec(t, "INSERT INTO changeset_sources VALUES ($1,1,$2)", cs, ingestID(1))
		_, err := f.s.Add(ctx, f.eng, cs, []uuid.UUID{ingestID(2)})
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23505" || pg.ConstraintName != "changeset_sources_pkey" {
			t.Fatalf("duplicate seq = %v, want original 23505 primary-key cause", err)
		}
		if f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != 1 || f.count(t, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != 0 {
			t.Fatal("failed append changed persisted state")
		}
	})
}

func assertIngestRows(t *testing.T, f fixture, cs uuid.UUID, want []uuid.UUID) {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), "SELECT seq,node_id FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq", cs)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := []uuid.UUID{}
	for rows.Next() {
		var seq int
		var id uuid.UUID
		if err := rows.Scan(&seq, &id); err != nil {
			t.Fatal(err)
		}
		if seq != len(got)+1 {
			t.Errorf("seq=%d at position %d", seq, len(got)+1)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) || f.count(t, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != len(want) {
		t.Fatalf("persistent marks=%v, want %v with matching count", got, want)
	}
}

// A preview limits seq before testing liveness, so neither null compression
// nor UUID sorting can substitute a later source for a missing first one.
func TestIngestHistory(t *testing.T) {
	t.Run("preview_order", func(t *testing.T) {
		f := newFixture(t)
		cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		ordinary := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
		want := make([]*uuid.UUID, 25)
		for i := 1; i <= 25; i++ {
			id := ingestID(100 - i)
			f.exec(t, "INSERT INTO changeset_sources VALUES ($1,$2,$3)", cs, i, id)
			if i != 3 {
				kind := "page"
				if i == 2 {
					kind = "asset"
				}
				f.exec(t, "INSERT INTO nodes (id,notebook_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$4,$5,$6,$6,$7,$7)", id, f.eng, kind, fmt.Sprintf("source-%d", i), i, f.alice, now())
				want[i-1] = new(id)
			}
		}
		f.exec(t, "UPDATE changesets SET ingest_count=25 WHERE id=$1", cs)
		f.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", ingestID(99), now())
		want[0] = nil
		// A foreign node with the historical UUID is not a live source here.
		f.exec(t, "UPDATE nodes SET notebook_id=$2 WHERE id=$1", ingestID(96), f.ops)
		want[3] = nil
		for _, n := range []int{3, 5, 20} {
			got, err := f.s.FirstIngestSources(context.Background(), []uuid.UUID{cs, ordinary}, n)
			if err != nil || !reflect.DeepEqual(got[cs], want[:n]) || len(got[ordinary]) != 0 {
				t.Errorf("preview(%d)=%v,%v; want %v and zero ordinary", n, got, err, want[:n])
			}
		}
		if f.count(t, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", cs) != 25 || f.count(t, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != 25 {
			t.Fatal("preview changed persisted marks/count")
		}
	})
}

func TestIngestHistoryFilters(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	node := f.page(t, f.eng, nil, "Changed", 0)
	bob := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id,email,password,display_name,created_at,updated_at) VALUES ($1,'bob-ingest@corp.com','x','Bob',$2,$2)", bob, now())
	ordinary := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now()).ID
	marked := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now().Add(time.Second)).ID
	agent := f.changeset(t, f.eng, bob, "mcp:agent", now().Add(2*time.Second)).ID
	last := f.changeset(t, f.eng, bob, "mcp:other", now().Add(3*time.Second)).ID
	outside := f.changeset(t, f.ops, f.alice, domain.ClientWeb, now().Add(4*time.Second)).ID
	for _, cs := range []uuid.UUID{marked, agent, outside} {
		f.exec(t, "INSERT INTO changeset_sources VALUES ($1,1,$2)", cs, ingestID(1))
		f.exec(t, "UPDATE changesets SET ingest_count=1 WHERE id=$1", cs)
	}
	state := node.State()
	for _, cs := range []uuid.UUID{ordinary, marked} {
		f.item(t, cs, now(), domain.Change{NodeID: node.ID, After: &state})
	}
	f.version(t, agent, node.ID, nil, 1, "body")
	for _, tt := range []struct {
		name        string
		filter      domain.LogFilter
		all, ingest []uuid.UUID
	}{
		{"notebook", domain.LogFilter{}, []uuid.UUID{last, agent, marked, ordinary}, []uuid.UUID{agent, marked}},
		{"actor", domain.LogFilter{Actor: &f.alice}, []uuid.UUID{marked, ordinary}, []uuid.UUID{marked}},
		{"page", domain.LogFilter{PageID: &node.ID}, []uuid.UUID{agent, marked, ordinary}, []uuid.UUID{agent, marked}},
		{"page_actor", domain.LogFilter{PageID: &node.ID, Actor: &bob}, []uuid.UUID{agent}, []uuid.UUID{agent}},
		{"mcp", domain.LogFilter{AllMCP: true}, []uuid.UUID{last, agent}, []uuid.UUID{agent}},
		{"exact_client", domain.LogFilter{Client: "mcp:other"}, []uuid.UUID{last}, []uuid.UUID{}},
		{"source_is_not_touched", domain.LogFilter{PageID: new(ingestID(1))}, []uuid.UUID{}, []uuid.UUID{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, on := range []bool{false, true} {
				fl := tt.filter
				fl.Ingest = on
				got, err := f.s.ListChangesets(ctx, f.eng, fl, domain.ChangesetFrom(), 10)
				want := tt.all
				if on {
					want = tt.ingest
				}
				if err != nil || !slices.Equal(changesetIDs(got), want) {
					t.Errorf("ingest=%v log=%v, %v; want %v", on, changesetIDs(got), err, want)
				}
				for _, c := range got {
					n := 0
					if c.ID == marked || c.ID == agent {
						n = 1
					}
					if c.IngestCount != n {
						t.Errorf("changeset %s count=%d, want %d", c.ID, c.IngestCount, n)
					}
				}
			}
		})
	}
	got, err := f.s.ListChangesets(ctx, f.eng, domain.LogFilter{Ingest: true}, domain.ChangesetCursor{At: now().Add(2 * time.Second), ID: agent}, 2)
	if err != nil || !slices.Equal(changesetIDs(got), []uuid.UUID{marked}) {
		t.Errorf("ingest cursor=%v,%v", changesetIDs(got), err)
	}
}
