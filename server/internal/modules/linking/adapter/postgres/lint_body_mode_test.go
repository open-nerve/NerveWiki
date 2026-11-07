package postgresadapter_test

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func lintBodyModeSeed(t *testing.T) (fixture, uuid.UUID, []app.BodyReference) {
	t.Helper()
	f := newFixture(t)
	p := uuid.UUID{}
	a, b, c := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "wikilink", Target: "A", Start: 10, End: 11},
		{Kind: "embed", Target: "B", Start: 20, End: 21},
		{Kind: "link", Target: "C", Start: 30, End: 31},
	}})
	if err := f.s.SetResolutions(t.Context(), []app.Link{
		{SourceID: p, Start: 10, Resolution: domain.Resolution{ID: a}},
		{SourceID: p, Start: 20, Resolution: domain.Resolution{ID: b, Asset: true}},
		{SourceID: p, Start: 30, Resolution: domain.Resolution{ID: c}},
	}); err != nil {
		t.Fatal(err)
	}
	return f, p, []app.BodyReference{
		{EdgePos: app.EdgePos{SourceID: p, Start: 10}, ResolvedID: a},
		{EdgePos: app.EdgePos{SourceID: p, Start: 20}, ResolvedID: b, ResolvedAsset: true},
		{EdgePos: app.EdgePos{SourceID: p, Start: 30}, ResolvedID: c},
	}
}

func TestLintBodyQueryModes(t *testing.T) {
	f, p, want := lintBodyModeSeed(t)
	for _, tc := range []struct {
		name, route                               string
		statement, description, readerDescription int
		mode                                      pgx.QueryExecMode
	}{
		{"pool-positive", "pool", 8, 8, 8, pgx.QueryExecModeDescribeExec},
		{"pool-zero", "pool", 0, 0, 0, pgx.QueryExecModeDescribeExec},
		{"onconn-positive", "onconn", 8, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"tx-positive", "tx", 8, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"onconn-description-zero", "onconn", 8, 0, 0, pgx.QueryExecModeDescribeExec},
		{"tx-description-zero", "tx", 8, 0, 0, pgx.QueryExecModeDescribeExec},
		{"onconn-statement-zero", "onconn", 0, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"tx-statement-zero", "tx", 0, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"onconn-both-zero", "onconn", 0, 0, 0, pgx.QueryExecModeDescribeExec},
		{"tx-both-zero", "tx", 0, 0, 0, pgx.QueryExecModeDescribeExec},
		{"onconn-reader-zero-actual-positive", "onconn", 8, 8, 0, pgx.QueryExecModeCacheDescribe},
		{"onconn-reader-positive-actual-zero", "onconn", 8, 0, 8, pgx.QueryExecModeDescribeExec},
		{"tx-reader-zero-actual-positive", "tx-onconn", 8, 8, 0, pgx.QueryExecModeCacheDescribe},
		{"tx-reader-positive-actual-zero", "tx-onconn", 8, 0, 8, pgx.QueryExecModeDescribeExec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &lintTrace{}
			actual := lintBodyModePool(t, f, tr, tc.statement, tc.description)
			readerPool := actual
			if tc.readerDescription != tc.description {
				readerPool = lintBodyModePool(t, f, &lintTrace{}, 8, tc.readerDescription)
			}
			conn, err := actual.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			cfg := conn.Conn().Config()
			if cfg.StatementCacheCapacity != tc.statement || cfg.DescriptionCacheCapacity != tc.description {
				t.Fatalf("fixture actual capacities=%d/%d", cfg.StatementCacheCapacity, cfg.DescriptionCacheCapacity)
			}
			r := postgresadapter.NewLintReads(readerPool, 5<<20)
			run := func(ctx context.Context) error {
				db := postgres.DB(ctx, readerPool)
				switch tc.route {
				case "pool":
					if got, ok := db.(*pgxpool.Pool); !ok || got != actual {
						t.Fatalf("fixture pool route: %T", db)
					}
				case "onconn":
					if got, ok := db.(*pgxpool.Conn); !ok || got.Conn().PgConn().PID() != pid {
						t.Fatalf("fixture OnConn route: %T", db)
					}
				default:
					if got, ok := db.(pgx.Tx); !ok || got.Conn().PgConn().PID() != pid {
						t.Fatalf("fixture Tx route: %T", db)
					}
				}
				for range 2 {
					first, err := r.BodyReferences(ctx, f.eng, []uuid.UUID{p}, nil, 1)
					if err != nil || !slices.Equal(first, want[:1]) {
						t.Fatalf("Body First independent DTO: %+v %v", first, err)
					}
					after, err := r.BodyReferences(ctx, f.eng, []uuid.UUID{p}, &want[0].EdgePos, 512)
					if err != nil || !slices.Equal(after, want[1:]) {
						t.Fatalf("Body After independent DTO: %+v %v", after, err)
					}
					if heads, err := r.Heads(ctx, f.eng); err != nil || len(heads) != 1 {
						t.Fatalf("unrelated Heads: %+v %v", heads, err)
					}
				}
				got := tr.observations()
				if len(got) != 6 {
					t.Fatalf("actual Body/Heads call count=%d want 6", len(got))
				}
				for i, call := range got {
					name, mode := "LintHeads", pgx.QueryExecModeDescribeExec
					var args []any
					switch i % 3 {
					case 0:
						name, mode = "LintBodyReferencesFirst", tc.mode
						args = []any{mode, f.eng, int32(domain.Extractor), p, p, int32(1), []uuid.UUID{p}}
					case 1:
						name, mode = "LintBodyReferencesAfter", tc.mode
						args = []any{mode, f.eng, int32(domain.Extractor), p, p, p, int32(10), int32(512), []uuid.UUID{p}}
					default:
						args = []any{mode, f.eng}
					}
					lintAssertMode(t, call, name, mode, pid, tc.statement, tc.description)
					if call.Err != nil || !reflect.DeepEqual(call.Args, args) {
						t.Fatalf("actual Go arguments/result for %s: %+v err=%v; want %+v", name, call.Args, call.Err, args)
					}
				}
				lintNoNamedStatements(t, ctx, readerPool)
				t.Logf("Body real route=%s executor=%T PID=%d actual statement/description=%d/%d reader description=%d mode=%v; Go args/DTO observed, wire not observed", tc.route, db, pid, tc.statement, tc.description, tc.readerDescription, tc.mode)
				return nil
			}
			switch tc.route {
			case "pool":
				conn.Release()
				err = run(t.Context())
			case "onconn":
				err = run(postgres.OnConn(t.Context(), conn))
			case "tx":
				conn.Release()
				err = lintBodyTxManager(actual).WithinTx(t.Context(), run)
			case "tx-onconn":
				err = lintBodyTxManager(readerPool).WithinTx(postgres.OnConn(t.Context(), conn), run)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLintBodyDescriptionReuse(t *testing.T) {
	f, p, want := lintBodyModeSeed(t)
	for _, route := range []string{"onconn", "tx"} {
		for _, capacity := range []int{8, 1} {
			t.Run(route+"/capacity="+strconv.Itoa(capacity), func(t *testing.T) {
				tr := &lintTrace{}
				pool := lintBodyModePool(t, f, tr, 0, capacity)
				conn, err := pool.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Release()
				pid := conn.Conn().PgConn().PID()
				r := postgresadapter.NewLintReads(pool, 5<<20)
				run := func(ctx context.Context) error {
					invoke := func(name string, after *app.EdgePos, expected []app.BodyReference, descriptions int) {
						t.Helper()
						before, calls := len(tr.prepareObservations()), len(tr.observations())
						rows, err := r.BodyReferences(ctx, f.eng, []uuid.UUID{p}, after, 512)
						if err != nil || !slices.Equal(rows, expected) {
							t.Fatalf("cold/hot Body DTO: %+v %v", rows, err)
						}
						observed := tr.observations()
						if len(observed) != calls+1 {
							t.Fatalf("Body retried or skipped: %+v", observed[calls:])
						}
						lintAssertMode(t, observed[calls], name, pgx.QueryExecModeCacheDescribe, pid, 0, capacity)
						lintAssertPrepares(t, tr.prepareObservations()[before:], name, pid, descriptions)
					}
					invoke("LintBodyReferencesFirst", nil, want, 1)
					invoke("LintBodyReferencesFirst", nil, want, 0)
					// An unrelated DescribeExec replaces the unnamed statement.
					n := len(tr.observations())
					if _, err := r.Heads(ctx, f.eng); err != nil {
						return err
					}
					calls := tr.observations()
					if len(calls) != n+1 {
						t.Fatal("Heads observer count")
					}
					lintAssertMode(t, calls[n], "LintHeads", pgx.QueryExecModeDescribeExec, pid, 0, capacity)
					invoke("LintBodyReferencesFirst", nil, want, 0)
					invoke("LintBodyReferencesAfter", &want[0].EdgePos, want[1:], 1)
					invoke("LintBodyReferencesAfter", &want[0].EdgePos, want[1:], 0)
					descriptions := 0
					if capacity == 1 {
						descriptions = 1
					}
					invoke("LintBodyReferencesFirst", nil, want, descriptions)
					lintNoNamedStatements(t, ctx, pool)
					t.Logf("Body description reuse route=%s PID=%d capacity=%d prepares=%+v; not a wire/roundtrip measurement", route, pid, capacity, tr.prepareObservations())
					return nil
				}
				ctx := postgres.OnConn(t.Context(), conn)
				if route == "tx" {
					err = lintBodyTxManager(pool).WithinTx(ctx, run)
				} else {
					err = run(ctx)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// These deliberately non-production calls verify that the observer exposes
// missing/duplicate options rather than reporting only the first option.
func TestLintBodyModeObservation(t *testing.T) {
	f, p, want := lintBodyModeSeed(t)
	tr := &lintTrace{}
	pool := lintBodyModePool(t, f, tr, 8, 8)
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if conn.Conn().Config().DefaultQueryExecMode != pgx.QueryExecModeCacheStatement {
		t.Fatal("missing-mode control requires the fixture's genuine default CacheStatement")
	}
	pid := conn.Conn().PgConn().PID()
	r := postgresadapter.NewLintReads(pool, 5<<20)
	if rows, err := r.BodyReferences(postgres.OnConn(t.Context(), conn), f.eng, []uuid.UUID{p}, nil, 1); err != nil || !slices.Equal(rows, want[:1]) {
		t.Fatalf("observer actual Body positive: %+v %v", rows, err)
	}
	positive := tr.observations()
	if len(positive) != 1 {
		t.Fatalf("observer positive calls=%d", len(positive))
	}
	lintAssertMode(t, positive[0], "LintBodyReferencesFirst", pgx.QueryExecModeCacheDescribe, pid, 8, 8)
	consume := func(args []any) {
		t.Helper()
		rows, err := conn.Query(t.Context(), positive[0].SQL, args...)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (app.BodyReference, error) {
			var item app.BodyReference
			err := row.Scan(&item.SourceID, &item.Start, &item.ResolvedID, &item.ResolvedAsset)
			return item, err
		})
		if err != nil || !slices.Equal(got, want[:1]) {
			t.Fatalf("observer control real DTO: %+v %v", got, err)
		}
	}
	// Last leading option wins in pgx: this hot SQL must be described again.
	before := len(tr.prepareObservations())
	duplicate := append([]any{pgx.QueryExecModeCacheDescribe, pgx.QueryExecModeDescribeExec}, positive[0].Args[1:]...)
	consume(duplicate)
	calls := tr.observations()
	if len(calls) != 2 || calls[1].Modes != 2 || !calls[1].Leading || calls[1].Mode != pgx.QueryExecModeCacheDescribe || !calls[1].Ended || calls[1].Err != nil {
		t.Fatalf("observer concealed duplicate leading override: %+v", calls)
	}
	lintAssertPrepares(t, tr.prepareObservations()[before:], "LintBodyReferencesFirst", pid, 1)
	lintNoNamedStatements(t, postgres.OnConn(t.Context(), conn), pool)
	consume(positive[0].Args[1:])
	calls = tr.observations()
	if len(calls) != 3 || calls[2].Modes != 0 || calls[2].Leading || !calls[2].Ended || calls[2].Err != nil {
		t.Fatalf("observer concealed missing explicit mode: %+v", calls)
	}
	var cached int
	if err := postgres.Planned(conn).QueryRow(t.Context(), `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&cached); err != nil || cached == 0 {
		t.Fatalf("missing-mode default named-cache control: count=%d err=%v", cached, err)
	}
	t.Logf("actual observer controls: duplicate modes=%d leading=%t; missing modes=%d leading=%t; default named-Lint=%d", calls[1].Modes, calls[1].Leading, calls[2].Modes, calls[2].Leading, cached)
}
