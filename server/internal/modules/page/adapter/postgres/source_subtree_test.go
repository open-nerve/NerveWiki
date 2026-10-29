package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

type sourceSubtreeRead struct {
	SQL  string
	Args []any
	Rows int64
}

type sourceSubtreeTrace struct {
	reads []*sourceSubtreeRead
}

type sourceSubtreeTraceKey struct{}

func (tr *sourceSubtreeTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(data.SQL, "-- name: SourceChildrenWithin ") {
		return ctx
	}
	r := &sourceSubtreeRead{SQL: data.SQL, Args: slices.Clone(data.Args)}
	for i, arg := range r.Args {
		if ids, ok := arg.([]uuid.UUID); ok {
			r.Args[i] = slices.Clone(ids)
		}
	}
	tr.reads = append(tr.reads, r)
	return context.WithValue(ctx, sourceSubtreeTraceKey{}, r)
}

func (*sourceSubtreeTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if r, ok := ctx.Value(sourceSubtreeTraceKey{}).(*sourceSubtreeRead); ok {
		r.Rows = data.CommandTag.RowsAffected()
	}
}

func (f fixture) tracedSourceSubtree(t *testing.T) (*postgresadapter.Store, *sourceSubtreeTrace, *pgxpool.Pool) {
	t.Helper()
	trace := &sourceSubtreeTrace{}
	pc := f.pool.Config()
	pc.MaxConns, pc.ConnConfig.Tracer = 1, trace
	pool, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), trace, pool
}

func (f fixture) sourceChildren(t *testing.T, notebook, parent uuid.UUID, count int) {
	t.Helper()
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
		created_by_id, updated_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, $2, 'page', 'Child ' || i, 'child ' || lpad(i::text, 8, '0'), $4::integer - i,
		$3, $3, $5, $5 FROM generate_series(1, $4::integer) i`, notebook, parent, f.alice, count, now())
}

// Each complete result must equal the old reader field for field. Rejected
// trees expose no partial result and stop after at most max+1 node rows,
// including the root. The last empty layer is essential at exactly max.
func TestSourceSubtreeReadsWithinItsWorkAndOrder(t *testing.T) {
	ctx := context.Background()
	t.Run("root only and max one", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		s, trace, _ := f.tracedSourceSubtree(t)
		got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, 1)
		if err != nil || !complete || !reflect.DeepEqual(got, domain.Subtree{{Node: root, Level: 1}}) {
			t.Fatalf("root-only read = %+v, complete=%t, %v; want exactly root complete", got, complete, err)
		}
		if len(trace.reads) != 1 || trace.reads[0].Rows != 0 {
			t.Errorf("root-only children probes = %+v, want one empty probe", trace.reads)
		}
		f.page(t, f.eng, &root.ID, "Child", 0)
		trace.reads = nil
		got, complete, err = s.SubtreeWithin(ctx, f.eng, root.ID, 1)
		if err != nil || complete || got != nil {
			t.Fatalf("two nodes with max1 = %d nodes, complete=%t, %v; want nil,false,nil", len(got), complete, err)
		}
		if len(trace.reads) != 1 || trace.reads[0].Rows != 1 {
			t.Errorf("max1 overflow probes = %+v, want one returned child and no later query", trace.reads)
		}
	})
	for _, total := range []int{9999, 10000, 10001} {
		t.Run(fmt.Sprintf("%d nodes", total), func(t *testing.T) {
			f := newFixture(t)
			root := f.page(t, f.eng, nil, "Root", 0)
			f.sourceChildren(t, f.eng, root.ID, total-1)
			s, trace, _ := f.tracedSourceSubtree(t)
			got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, domain.MaxUnitNodes)
			if err != nil || complete != (total <= domain.MaxUnitNodes) {
				t.Fatalf("%d-node read = %d nodes, complete=%t, %v", total, len(got), complete, err)
			}
			materialized := int64(1)
			for _, r := range trace.reads {
				materialized += r.Rows
			}
			if materialized > domain.MaxUnitNodes+1 || len(trace.reads) == 0 {
				t.Errorf("reader materialized %d node rows in %d bounded queries, want <=10001 with actual probes", materialized, len(trace.reads))
			}
			if !complete {
				if got != nil || len(trace.reads) != 1 || materialized != 10001 {
					t.Errorf("overflow returned %d nodes and probes=%d/materialized=%d, want nil/1/10001", len(got), len(trace.reads), materialized)
				}
				return
			}
			want, err := f.s.Subtree(ctx, f.eng, root.ID)
			if err != nil || len(got) != total || !reflect.DeepEqual(got, want) {
				t.Errorf("complete %d-node tree differs from original reader: lengths %d/%d, %v", total, len(got), len(want), err)
			}
			if len(trace.reads) != 2 || trace.reads[1].Rows != 0 {
				t.Errorf("complete %d-node tree omitted final empty leaf probe: %+v", total, trace.reads)
			}
		})
	}
	t.Run("exact max still probes grandchildren", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		child := f.page(t, f.eng, &root.ID, "Child", 0)
		f.page(t, f.eng, &child.ID, "Grandchild", 0)
		s, trace, _ := f.tracedSourceSubtree(t)
		got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, 2)
		if err != nil || complete || got != nil || len(trace.reads) != 2 || trace.reads[1].Rows != 1 {
			t.Errorf("exact-max continuation = %d nodes, complete=%t, %v, probes=%+v; want nil,false with final take1", len(got), complete, err, trace.reads)
		}
	})
	t.Run("9999 already read leaves take two", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		f.sourceChildren(t, f.eng, root.ID, 9998)
		var parent uuid.UUID
		if err := f.pool.QueryRow(ctx, "SELECT id FROM nodes WHERE parent_id = $1 ORDER BY name_key LIMIT 1", root.ID).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		f.sourceChildren(t, f.eng, parent, 100)
		s, trace, _ := f.tracedSourceSubtree(t)
		got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, domain.MaxUnitNodes)
		if err != nil || complete || got != nil || len(trace.reads) != 2 || trace.reads[0].Rows != 9998 || trace.reads[1].Rows != 2 {
			t.Fatalf("remaining-two read = %d nodes, complete=%t, %v, probes=%+v", len(got), complete, err, trace.reads)
		}
	})
	t.Run("multiple parents share the remaining bound", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		a := f.page(t, f.eng, &root.ID, "A", 1)
		b := f.page(t, f.eng, &root.ID, "B", 2)
		f.sourceChildren(t, f.eng, a.ID, 3)
		f.sourceChildren(t, f.eng, b.ID, 3)
		rows, err := gen.New(postgres.Planned(f.pool)).SourceChildrenWithin(ctx, gen.SourceChildrenWithinParams{
			NotebookID: f.eng, Parents: []uuid.UUID{a.ID, b.ID}, MaxRows: 2,
		})
		if err != nil || len(rows) != 2 {
			t.Fatalf("two parents with take2 returned %d children, %v; want exactly two total", len(rows), err)
		}
		got, complete, err := f.s.SubtreeWithin(ctx, f.eng, root.ID, 5)
		if err != nil || complete || got != nil {
			t.Errorf("multi-parent overflow = %d nodes, complete=%t, %v; want nil,false,nil", len(got), complete, err)
		}
	})
	t.Run("direct foreign parent keeps notebook filter", func(t *testing.T) {
		f := newFixture(t)
		foreign := f.page(t, f.ops, nil, "Foreign", 0)
		f.sourceChildren(t, f.ops, foreign.ID, 3)
		q := gen.New(postgres.Planned(f.pool))
		p := gen.SourceChildrenWithinParams{NotebookID: f.eng, Parents: []uuid.UUID{foreign.ID}, MaxRows: 2}
		rows, err := q.SourceChildrenWithin(ctx, p)
		if err != nil || len(rows) != 0 {
			t.Fatalf("foreign parent exposed %d children, %v", len(rows), err)
		}
		p.NotebookID = f.ops
		rows, err = q.SourceChildrenWithin(ctx, p)
		if err != nil || len(rows) != 2 {
			t.Fatalf("foreign parent's own notebook returned %d children, %v; want positive control two", len(rows), err)
		}
		// The immediate composite FK makes mixed-notebook children of
		// one parent unrepresentable. The post-limit notebook filter
		// therefore cannot discard an earlier foreign sibling while
		// hiding a later local one in a valid database.
		_, err = f.pool.Exec(ctx, "UPDATE nodes SET notebook_id = $1 WHERE id = $2", f.eng, rows[0].ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "nodes_notebook_id_parent_id_fkey" {
			t.Fatalf("cross-notebook child update = %v; want the immediate composite parent FK rejection", err)
		}
		local := f.page(t, f.eng, nil, "Local", 0)
		f.sourceChildren(t, f.eng, local.ID, 3)
		p.NotebookID, p.Parents = f.eng, []uuid.UUID{foreign.ID, local.ID}
		rows, err = q.SourceChildrenWithin(ctx, p)
		if err != nil || len(rows) != 2 {
			t.Fatalf("foreign-first/local-second parents returned %d children, %v; want local take2", len(rows), err)
		}
		for _, row := range rows {
			if row.NotebookID != f.eng || row.ParentID == nil || *row.ParentID != local.ID {
				t.Errorf("foreign-first parent list exposed or displaced a local child: %+v", row)
			}
		}
	})
	t.Run("global level order and assets", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		a := f.page(t, f.eng, &root.ID, "A by name", 9)
		b := f.page(t, f.eng, &root.ID, "Z by name", 1)
		f.page(t, f.eng, &a.ID, "Z first of A", 2)
		f.page(t, f.eng, &a.ID, "A tie of A", 20)
		f.asset(t, f.eng, &b.ID, "z-first.txt", 1)
		f.page(t, f.eng, &b.ID, "A tie of B", 20)
		gone := f.page(t, f.eng, &a.ID, "Deleted", -1)
		f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
		f.page(t, f.ops, nil, "Foreign", -2)
		got, complete, err := f.s.SubtreeWithin(ctx, f.eng, root.ID, 7)
		want, oldErr := f.s.Subtree(ctx, f.eng, root.ID)
		if err != nil || oldErr != nil || !complete || len(got) != 7 || !reflect.DeepEqual(got, want) {
			t.Errorf("global order = %+v, complete=%t, %v; old reader %+v, %v", got, complete, err, want, oldErr)
		}
		if len(got) >= 5 && (got[1].Node.ID != b.ID || got[2].Node.ID != a.ID || got[3].Node.Name != "z-first.txt" || got[4].Node.Name != "Z first of A") {
			t.Errorf("tree sorted by parent or name instead of whole level: %+v", got)
		}
	})
	t.Run("deepest legal pages and attachment", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Root", 0)
		last := root
		for i := 1; i < domain.MaxDepth; i++ {
			last = f.page(t, f.eng, &last.ID, fmt.Sprintf("Page %d", i), 0)
		}
		f.asset(t, f.eng, &last.ID, "last.txt", 0)
		got, complete, err := f.s.SubtreeWithin(ctx, f.eng, root.ID, domain.MaxDepth+1)
		if err != nil || !complete || len(got) != domain.MaxDepth+1 || got.Height() != domain.MaxDepth {
			t.Errorf("deepest legal tree = %+v, complete=%t, %v", got, complete, err)
		}
	})
}

func TestSourceSubtreeChecksRootBoundsAndDepth(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	for _, max := range []int{-1, 0, domain.MaxUnitNodes + 1} {
		if got, complete, err := f.s.SubtreeWithin(ctx, f.eng, root.ID, max); err == nil || got != nil || complete {
			t.Errorf("max=%d returned %d nodes, complete=%t, %v; want invalid internal bound error", max, len(got), complete, err)
		}
	}
	foreign := f.page(t, f.ops, nil, "Foreign", 0)
	deleted := f.page(t, f.eng, nil, "Deleted", 1)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", deleted.ID, now())
	for _, id := range []uuid.UUID{foreign.ID, deleted.ID, uuid.NewV7()} {
		if got, complete, err := f.s.SubtreeWithin(ctx, f.eng, id, 100); !errors.Is(err, app.ErrNotFound) || got != nil || complete {
			t.Errorf("unavailable root returned %d nodes, complete=%t, %v; want ErrNotFound", len(got), complete, err)
		}
	}
	last := root
	for i := 1; i < 65; i++ {
		last = f.page(t, f.eng, &last.ID, fmt.Sprintf("Invalid depth %d", i), 0)
	}
	if got, complete, err := f.s.SubtreeWithin(ctx, f.eng, root.ID, 100); err == nil || got != nil || complete {
		t.Errorf("unexhausted 64-level tree returned %d nodes, complete=%t, %v; want internal depth error", len(got), complete, err)
	}
}

func TestSourceSubtreeUsesTheTransactionAndPlannedQueries(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	child := f.page(t, f.eng, &root.ID, "Child", 0)
	s, _, pool := f.tracedSourceSubtree(t)
	for range 8 {
		if got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, 2); err != nil || !complete || len(got) != 2 {
			t.Fatalf("complete read = %d nodes, complete=%t, %v", len(got), complete, err)
		}
	}
	cached := pgtest.CachedStatements(t, pool)
	if slices.Contains(cached, "SourceChildrenWithin") || !slices.Contains(cached, "FindNodeIn") {
		t.Errorf("source child queries cached or root cache control absent: %v", cached)
	}
	failure := errors.New("rollback source subtree fixture")
	err := postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, pool).Exec(ctx, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", child.ID, now()); err != nil {
			return err
		}
		got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, 1)
		if err != nil || !complete || len(got) != 1 || got[0].Node.ID != root.ID {
			t.Errorf("source subtree ignored current transaction: %d nodes, complete=%t, %v", len(got), complete, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if got, complete, err := s.SubtreeWithin(ctx, f.eng, root.ID, 1); err != nil || complete || got != nil {
		t.Errorf("rolled-back child vanished from source bound: %d nodes, complete=%t, %v", len(got), complete, err)
	}
}
