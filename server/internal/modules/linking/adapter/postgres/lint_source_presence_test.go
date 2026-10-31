package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func testLintSourcePresence(t *testing.T) {
	t.Run("exact-qualification-and-no-FK", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5<<20)
		for n, key := range []string{"SoUrCeS", "sources", "sources", "sources", "sources", "ſources", "sources.0", "Sources", "sources"} {
			book := f.eng
			if n == 4 {
				book = f.ops
			}
			f.replace(t, app.Page{ID: lintID(uint64(n)), NotebookID: book, Revision: 1}, domain.Facts{FrontmatterValid: n != 3, Properties: []domain.Property{{Key: key, Value: []byte(`"first"`)}, {Key: key, Value: []byte(`"second"`)}}})
		}
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, lintID(2))
		lintExec(t, f, `UPDATE page_properties SET notebook_id=$2 WHERE source_id=$1`, lintID(7), f.ops)
		lintExec(t, f, `DELETE FROM indexed_pages WHERE node_id=$1`, lintID(8))
		for _, tc := range []struct {
			name string
			ids  []uuid.UUID
			want bool
		}{
			{"empty", nil, false}, {"zero-ASCII", []uuid.UUID{lintID(0)}, true},
			{"current", []uuid.UUID{lintID(1)}, true}, {"stale", []uuid.UUID{lintID(2)}, false},
			{"invalid", []uuid.UUID{lintID(3)}, false}, {"foreign", []uuid.UUID{lintID(4)}, false},
			{"unicode", []uuid.UUID{lintID(5)}, false}, {"dotted", []uuid.UUID{lintID(6)}, false},
			{"wrong-property-notebook", []uuid.UUID{lintID(7)}, true}, {"orphan-property", []uuid.UUID{lintID(8)}, false},
			{"noneligible-real-sources", []uuid.UUID{lintID(5), lintID(6)}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := r.HasSourceProperties(t.Context(), f.eng, tc.ids)
				if err != nil || got != tc.want {
					t.Fatalf("exact source presence %s=%t err=%v want=%t", tc.name, got, err, tc.want)
				}
			})
		}
		heads, err := r.SourcePropertyHeads(t.Context(), f.eng, []uuid.UUID{lintID(0), lintID(7)})
		if err != nil || len(heads) != 2 || heads[0].PageID != lintID(0) || heads[1].PageID != lintID(7) || heads[0].Position != 0 || heads[1].Position != 0 {
			t.Fatalf("original first-head and wrong-property-book witness: %+v %v", heads, err)
		}
	})
	t.Run("complete-1025-last-source-and-planned", func(t *testing.T) {
		f := newFixture(t)
		ids := make([]uuid.UUID, 1025)
		for i := range ids {
			ids[i] = lintID(uint64(i))
		}
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,2,true`, ids, f.eng)
		lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) VALUES($1,0,$2,'sources','"last"')`, ids[1024], f.eng)
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		conn, err := pool.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		ctx := postgres.OnConn(t.Context(), conn)
		r := postgresadapter.NewLintReads(pool, 5<<20)
		for range 7 {
			has, err := r.HasSourceProperties(ctx, f.eng, ids)
			if err != nil || !has {
				t.Fatalf("complete 1025 relation lost final source: has=%t err=%v", has, err)
			}
		}
		for _, bad := range [][]uuid.UUID{{ids[1], ids[0]}, {ids[0], ids[0]}} {
			if _, err := r.HasSourceProperties(ctx, f.eng, bad); err == nil {
				t.Fatalf("unordered/duplicate source relation accepted: %v", bad)
			}
		}
		if has, err := r.HasSourceProperties(ctx, f.eng, nil); has || err != nil {
			t.Fatalf("empty relation: %t %v", has, err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if has, err := r.HasSourceProperties(canceled, f.eng, nil); has || !errors.Is(err, context.Canceled) {
			t.Fatalf("empty adapter ignored cancellation: %t %v", has, err)
		}
		var cached int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: LintHasSourceProperties%'`).Scan(&cached); err != nil {
			t.Fatal(err)
		}
		if cached != 0 || len(tr.names()) != 7 || len(tr.modes) != 7 {
			t.Fatalf("presence bypassed Planned or rejected inputs queried: cached=%d calls=%v modes=%v", cached, tr.names(), tr.modes)
		}
		for _, mode := range tr.modes {
			if mode != pgx.QueryExecModeDescribeExec {
				t.Fatalf("presence mode=%v", mode)
			}
		}
	})
	t.Run("caller-snapshot-and-query-error", func(t *testing.T) {
		f := newFixture(t)
		id := uuid.UUID{}
		f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		r := postgresadapter.NewLintReads(pool, 5<<20)
		err := postgres.NewTxManager(pool, time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
			if has, err := r.HasSourceProperties(ctx, f.eng, []uuid.UUID{id}); err != nil || has {
				return fmt.Errorf("initial presence=%t: %w", has, err)
			}
			f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 2}, originFacts(t, "---\nsources: value\n---\n"))
			if has, err := r.HasSourceProperties(ctx, f.eng, []uuid.UUID{id}); err != nil || has {
				return fmt.Errorf("presence escaped caller snapshot=%t: %w", has, err)
			}
			heads, err := r.SourcePropertyHeads(ctx, f.eng, []uuid.UUID{id})
			if err != nil || len(heads) != 0 {
				return fmt.Errorf("head snapshot differs: %v %w", heads, err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if has, err := r.HasSourceProperties(t.Context(), f.eng, []uuid.UUID{id}); err != nil || !has {
			t.Fatalf("next snapshot did not see committed source: %t %v", has, err)
		}
		if !slices.Equal(tr.names(), []string{"LintHasSourceProperties", "LintHasSourceProperties", "LintFirstSourceHeads", "LintHasSourceProperties"}) {
			t.Fatalf("snapshot queries bypassed the traced pool: %v", tr.names())
		}
		lintExec(t, f, `ALTER TABLE page_properties RENAME TO unavailable_properties`)
		has, err := r.HasSourceProperties(t.Context(), f.eng, []uuid.UUID{id})
		var pgerr *pgconn.PgError
		if has || !errors.As(err, &pgerr) || pgerr.Code != "42P01" {
			t.Fatalf("actual SQL error became false absence: has=%t err=%v", has, err)
		}
	})
}
