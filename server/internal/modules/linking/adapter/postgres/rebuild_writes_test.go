package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type rebuildInsertCall struct {
	args []any
	pid  uint32
	mode pgx.QueryExecMode
	sql  string
}

type rebuildInsertTrace struct {
	base     *stableAdapterTrace
	mu       sync.Mutex
	calls    []rebuildInsertCall
	prepares []string
}

func (tr *rebuildInsertTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	ctx = tr.base.TraceQueryStart(ctx, conn, d)
	if ctx.Value(stableAdapterMarkKey{}) == tr.base && strings.HasPrefix(d.SQL, "-- name: InsertRebuiltLinks :exec\n") {
		tr.mu.Lock()
		tr.calls = append(tr.calls, rebuildInsertCall{append([]any(nil), d.Args...), conn.PgConn().PID(), conn.Config().DefaultQueryExecMode, d.SQL})
		tr.mu.Unlock()
	}
	return ctx
}
func (tr *rebuildInsertTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	tr.base.TraceQueryEnd(ctx, conn, d)
}
func (tr *rebuildInsertTrace) TracePrepareStart(ctx context.Context, _ *pgx.Conn, d pgx.TracePrepareStartData) context.Context {
	if strings.HasPrefix(d.SQL, "-- name: InsertRebuiltLinks :exec\n") {
		tr.mu.Lock()
		tr.prepares = append(tr.prepares, d.Name)
		tr.mu.Unlock()
	}
	return ctx
}
func (*rebuildInsertTrace) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}

func rebuildInsertPool(t *testing.T, f stableAdapterFixture, tr *rebuildInsertTrace, mode pgx.QueryExecMode, statements, descriptions int) *pgxpool.Pool {
	t.Helper()
	cfg := f.pool.Config() // Preserve actual AfterConnect codecs.
	cfg.ConnConfig.Tracer, cfg.ConnConfig.DefaultQueryExecMode = tr, mode
	cfg.ConnConfig.StatementCacheCapacity, cfg.ConnConfig.DescriptionCacheCapacity = statements, descriptions
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// These are direct persistent values, not parser/linksParams-derived oracles.
func rebuildInsertValues() ([]domain.Link, []domain.Resolution) {
	return []domain.Link{
		{Kind: "wikilink", Target: "Straße.md", Property: "", Quote: '"', Start: 2, End: 11, Anchor: "節", Display: "shown\"\\", Aliases: true},
		{Kind: "embed", Target: "./Folder/Note.md", Property: "refs.0", Start: 20, End: 36, InTable: true},
		{Kind: "link", Target: "asset.txt", Start: 40, End: 49},
		{Kind: "image", Target: "A//B", Start: 70, End: 74},
		{Kind: "wikilink", Target: "../Up", Start: 90, End: 95},
	}, []domain.Resolution{
		{ID: uuid.UUID{15: 11}, Ambiguous: true}, {ID: uuid.UUID{15: 12}},
		{ID: uuid.UUID{15: 13}, Asset: true}, {}, {},
	}
}

func rebuildInsertArgs(p app.Page, full bool) []any {
	if !full {
		return []any{p.ID, p.NotebookID, []int32(nil), []int32(nil), []string(nil), []string(nil),
			[]bool(nil), []string(nil), []string(nil), []string(nil), []string(nil), []string(nil), []bool(nil),
			[]uuid.UUID(nil), []bool(nil), []bool(nil)}
	}
	return []any{p.ID, p.NotebookID, []int32{2, 20, 40, 70, 90}, []int32{11, 36, 49, 74, 95},
		[]string{"wikilink", "embed", "link", "image", "wikilink"}, []string{"", "refs.0", "", "", ""},
		[]bool{true, true, false, false, false}, []string{"Straße.md", "./Folder/Note.md", "asset.txt", "A//B", "../Up"},
		[]string{"節", "", "", "", ""}, []string{"shown\"\\", "", "", "", ""},
		[]string{"strasse", "note", "asset.txt", "", "up"}, []string{"strasse.md", "note.md", "", "", ""},
		[]bool{true, false, false, false, false}, []uuid.UUID{{15: 11}, {15: 12}, {15: 13}, {}, {}},
		[]bool{true, false, false, false, false}, []bool{false, false, true, false, false}}
}

func rebuildInsertRows(p app.Page) []stableAdapterLink {
	return []stableAdapterLink{
		{Source: p.ID, Notebook: p.NotebookID, Start: 2, End: 11, Kind: "wikilink", Property: stableAdapterValue(""),
			Target: "Straße.md", Anchor: stableAdapterValue("節"), Display: stableAdapterValue("shown\"\\"),
			Key: stableAdapterValue("strasse"), Alt: stableAdapterValue("strasse.md"), Resolved: stableAdapterValue(uuid.UUID{15: 11}), Ambiguous: true, Aliases: true},
		{Source: p.ID, Notebook: p.NotebookID, Start: 20, End: 36, Kind: "embed", Property: stableAdapterValue("refs.0"),
			Target: "./Folder/Note.md", Key: stableAdapterValue("note"), Alt: stableAdapterValue("note.md"), Resolved: stableAdapterValue(uuid.UUID{15: 12})},
		{Source: p.ID, Notebook: p.NotebookID, Start: 40, End: 49, Kind: "link", Target: "asset.txt", Key: stableAdapterValue("asset.txt"), Resolved: stableAdapterValue(uuid.UUID{15: 13}), Asset: true},
		{Source: p.ID, Notebook: p.NotebookID, Start: 70, End: 74, Kind: "image", Target: "A//B"},
		{Source: p.ID, Notebook: p.NotebookID, Start: 90, End: 95, Kind: "wikilink", Target: "../Up", Key: stableAdapterValue("up")},
	}
}

func TestRebuildResolvedInsertUsesCallerTxAndNativeModes(t *testing.T) {
	f := newStableAdapterFixture(t)
	for _, tc := range []struct {
		name                               string
		mode                               pgx.QueryExecMode
		statements, descriptions, prepares int
		encodeError                        bool
	}{
		{"statement", pgx.QueryExecModeCacheStatement, 64, 0, 1, false},
		{"description", pgx.QueryExecModeCacheDescribe, 0, 64, 1, false},
		{"describe", pgx.QueryExecModeDescribeExec, 0, 0, 3, false},
		{"exec", pgx.QueryExecModeExec, 0, 0, 0, true},
		{"simple", pgx.QueryExecModeSimpleProtocol, 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &rebuildInsertTrace{base: f.trace}
			actual := rebuildInsertPool(t, f, tr, tc.mode, tc.statements, tc.descriptions)
			constructorMode := pgx.QueryExecModeCacheStatement
			if tc.mode == constructorMode {
				constructorMode = pgx.QueryExecModeExec
			}
			constructor := rebuildInsertPool(t, f, tr, constructorMode, 64, 64)
			caller, err := actual.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer caller.Release()
			other, err := constructor.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer other.Release()
			pid, otherPID := caller.Conn().PgConn().PID(), other.Conn().PgConn().PID()
			if pid == 0 || pid == otherPID {
				t.Fatal("caller and constructor fixture are not independent connections")
			}
			store := postgresadapter.New(constructor)
			rows, results := rebuildInsertValues()
			originalRows, originalResults := slices.Clone(rows), slices.Clone(results)
			pages := []app.Page{{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}, {ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}, {ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}}
			inputs := [][]domain.Link{rows, nil, {}}
			resolutions := [][]domain.Resolution{results, nil, {}}
			before := f.snapshot(t, t.Context(), nil)
			stop := errors.New("finish the private rebuilt-input probe by rollback")
			f.trace.reset()
			ctx := context.WithValue(postgres.OnConn(t.Context(), caller), stableAdapterMarkKey{}, f.trace)
			err = postgres.NewTxManager(constructor, time.Second).WithinTx(ctx, func(ctx context.Context) error {
				tx, ok := postgres.TxFrom(ctx)
				if !ok || tx.Conn().PgConn().PID() != pid {
					t.Fatal("actual caller transaction was replaced")
				}
				if err := store.Lock(ctx, f.eng); err != nil {
					return err
				}
				for i, p := range pages {
					if err := store.AddRebuildMetadata(ctx, p, app.RebuildMetadata{FrontmatterValid: true}); err != nil {
						return err
					}
					if err := store.AddRebuildResolvedLinks(ctx, p, inputs[i], resolutions[i]); err != nil {
						return err
					}
					var want []stableAdapterLink
					if i == 0 {
						want = rebuildInsertRows(p)
					}
					f.assertLinks(t, ctx, p.ID, want)
				}
				return stop
			})
			wantCalls, failedSQL, wantRows := 3, "", []int64{5, 0, 0}
			if tc.encodeError {
				// The unchanged pool cannot infer an OID for nonempty []uuid.UUID.
				// These are the actual pgx encoder errors, before the INSERT is sent.
				wantCalls, failedSQL, wantRows = 1, "InsertRebuiltLinks", []int64{0}
				var native *pgconn.PgError
				var sqlState interface{ SQLState() string }
				if err == nil || errors.Is(err, stop) || errors.As(err, &native) || errors.As(err, &sqlState) ||
					!strings.HasPrefix(err.Error(), "index page "+pages[0].ID.String()+": ") ||
					!strings.Contains(err.Error(), "unable to encode []uuid.UUID") ||
					!strings.Contains(err.Error(), "into text format for unknown type (OID 0): cannot find encode plan") {
					t.Fatalf("REBUILD_INSERT_ENCODING: mode%s actual error=%v; want original UUID-array OID0 text encoding cause", tc.mode, err)
				}
				if tc.mode == pgx.QueryExecModeExec && !strings.Contains(err.Error(), "failed to encode args[13]: ") {
					t.Fatalf("REBUILD_INSERT_ENCODING: exec did not fail at ResolvedIds argument13: %v", err)
				}
			} else if !errors.Is(err, stop) {
				t.Fatal(err)
			}
			if status := caller.Conn().PgConn().TxStatus(); status != 'I' {
				t.Fatalf("REBUILD_INSERT_ROLLBACK: caller transaction status=%q want idle", status)
			}
			if after := f.snapshot(t, t.Context(), nil); after != before {
				t.Fatal("rebuilt input transaction leaked committed rows")
			}
			if !slices.Equal(rows, originalRows) || !slices.Equal(results, originalResults) {
				t.Fatal("final writer mutated caller rows/resolutions")
			}
			wantSQL := []string{"LockNotebook"}
			for range wantCalls {
				wantSQL = append(wantSQL, "InsertIndexedPage", "InsertTags", "InsertProperties", "InsertAliases", "InsertRebuiltLinks")
			}
			f.assertSQL(t, wantSQL, pid, failedSQL, err)
			tr.mu.Lock()
			calls, prepares := slices.Clone(tr.calls), slices.Clone(tr.prepares)
			tr.mu.Unlock()
			if len(calls) != wantCalls || len(prepares) != tc.prepares {
				t.Fatalf("REBUILD_INSERT_MODE: calls=%d prepares=%v want%d/%d", len(calls), prepares, wantCalls, tc.prepares)
			}
			for i, call := range calls {
				if call.pid != pid || call.mode != tc.mode || !reflect.DeepEqual(call.args, rebuildInsertArgs(pages[i], i == 0)) {
					t.Fatalf("REBUILD_INSERT_ARGS: call%d=%+v want actual pid%d mode%s and literal16args", i, call, pid, tc.mode)
				}
			}
			for _, name := range prepares {
				if (name != "") != (tc.mode == pgx.QueryExecModeCacheStatement) {
					t.Fatalf("REBUILD_INSERT_MODE: actual prepare name=%q mode%s", name, tc.mode)
				}
			}
			var rowsWritten []int64
			var headsWritten int64
			for _, call := range f.trace.snapshot() {
				if call.Name == "InsertRebuiltLinks" {
					rowsWritten = append(rowsWritten, call.Rows)
				}
				if call.Name == "InsertIndexedPage" {
					headsWritten += call.Rows
				}
			}
			if headsWritten != int64(wantCalls) {
				t.Fatalf("REBUILD_INSERT_PREFIX: actual heads=%d want%d before rollback", headsWritten, wantCalls)
			}
			if !slices.Equal(rowsWritten, wantRows) {
				t.Fatalf("REBUILD_INSERT_ROWS: mode%s actual rows=%v want%v", tc.mode, rowsWritten, wantRows)
			}
			if tc.name == "statement" {
				// Separate ABI description after every cold/hot/row assertion.
				// PgConn.Prepare is not an INSERT and is not in those Prepare counts.
				description, err := caller.Conn().PgConn().Prepare(t.Context(), "", calls[0].sql, nil)
				if err != nil {
					t.Fatalf("REBUILD_INSERT_DESCRIBE: %v", err)
				}
				wantOIDs := []uint32{pgtype.UUIDOID, pgtype.UUIDOID, pgtype.Int4ArrayOID, pgtype.Int4ArrayOID,
					pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID, pgtype.TextArrayOID,
					pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID,
					pgtype.BoolArrayOID, pgtype.UUIDArrayOID, pgtype.BoolArrayOID, pgtype.BoolArrayOID}
				if !slices.Equal(description.ParamOIDs, wantOIDs) {
					t.Fatalf("REBUILD_INSERT_OIDS: got%v want%v", description.ParamOIDs, wantOIDs)
				}
				t.Logf("REBUILD_INSERT_OIDS: separate same-PID%d description=%v", pid, description.ParamOIDs)
			}
		})
	}
}

func TestRebuildResolvedInsertKeepsLateNativeErrors(t *testing.T) {
	f := newStableAdapterFixture(t)
	for _, tc := range []struct{ name, code, constraint string }{
		{"unknown_kind", "23514", "page_links_kind_check"},
		{"empty_kind", "23514", "page_links_kind_check"},
		{"duplicate_start", "23505", "page_links_pkey"},
		{"null_ambiguous", "23514", "page_links_ambiguous_check"},
		{"null_asset", "23514", "page_links_resolved_asset_check"},
		{"late_range", "23514", "page_links_range_check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			f.seed(t, p, domain.Facts{FrontmatterValid: true, Links: []domain.Link{{Kind: "link", Target: "old", Start: 1, End: 4}}, Tags: []domain.Tag{{Key: "old", Name: "Old", Count: 1}}, Properties: []domain.Property{{Key: "old", Value: []byte(`"value"`)}}, Aliases: []domain.Alias{{Key: "old", Name: "Old"}}}, nil)
			before := f.snapshot(t, t.Context(), nil)
			prior := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			links := []domain.Link{{Kind: "wikilink", Target: "A", Start: 1, End: 2}, {Kind: "image", Target: "B", Start: 10, End: 11}}
			resolved := []domain.Resolution{{ID: uuid.UUID{15: 11}}, {ID: uuid.UUID{15: 12}}}
			switch tc.name {
			case "unknown_kind":
				links[1].Kind = "unknown"
			case "empty_kind":
				links[1].Kind = ""
			case "duplicate_start":
				links[1].Start = links[0].Start
			case "null_ambiguous":
				resolved[1] = domain.Resolution{Ambiguous: true}
			case "null_asset":
				resolved[1] = domain.Resolution{Asset: true}
			case "late_range":
				links[1].End = links[1].Start
			}
			originalLinks, originalResolved := slices.Clone(links), slices.Clone(resolved)
			pid, err := f.run(t, f.eng, func(ctx context.Context) error {
				if err := f.s.DeleteNotebooks(ctx, []uuid.UUID{f.eng}); err != nil {
					return err
				}
				if err := f.s.AddRebuildMetadata(ctx, prior, app.RebuildMetadata{FrontmatterValid: true}); err != nil {
					return err
				}
				if err := f.s.AddRebuildResolvedLinks(ctx, prior, []domain.Link{{Kind: "link", Target: "Prior", Start: 1, End: 6}}, []domain.Resolution{{ID: uuid.UUID{15: 11}}}); err != nil {
					return err
				}
				f.assertLinks(t, ctx, prior.ID, []stableAdapterLink{{Source: prior.ID, Notebook: f.eng, Start: 1, End: 6, Kind: "link", Target: "Prior", Key: stableAdapterValue("prior"), Resolved: stableAdapterValue(uuid.UUID{15: 11})}})
				if err := f.s.AddRebuildMetadata(ctx, p, app.RebuildMetadata{FrontmatterValid: true}); err != nil {
					return err
				}
				return f.s.AddRebuildResolvedLinks(ctx, p, links, resolved)
			})
			var native *pgconn.PgError
			if !errors.As(err, &native) || native.Code != tc.code || native.ConstraintName != tc.constraint || !strings.HasPrefix(err.Error(), "index page "+p.ID.String()+": ") {
				t.Fatalf("REBUILD_INSERT_CAUSE: got%v want%s/%s in original index-page wrapper", err, tc.code, tc.constraint)
			}
			calls := f.trace.snapshot()
			want := []string{"LockNotebook", "DeleteNotebooksLinks", "DeleteNotebooksTags", "DeleteNotebooksProperties", "DeleteNotebooksAliases", "DeleteNotebooksIndexedPages"}
			for range 2 {
				want = append(want, "InsertIndexedPage", "InsertTags", "InsertProperties", "InsertAliases", "InsertRebuiltLinks")
			}
			if len(calls) != len(want) {
				t.Fatalf("REBUILD_INSERT_STOP: calls=%+v want%v", calls, want)
			}
			for i, call := range calls {
				if !call.Done || call.PID != pid || call.Name != want[i] {
					t.Fatalf("REBUILD_INSERT_STOP: call%d=%+v pid%d want%s", i, call, pid, want[i])
				}
				if i == len(calls)-1 {
					if !errors.Is(err, call.Err) || call.Err == nil {
						t.Fatalf("REBUILD_INSERT_CAUSE: native%v returned%v", call.Err, err)
					}
				} else if call.Err != nil {
					t.Fatalf("REBUILD_INSERT_EARLY_ERROR: %+v", call)
				}
			}
			if calls[10].Rows != 1 {
				t.Fatalf("REBUILD_INSERT_PREFIX: actual prior final INSERT rows=%d want1", calls[10].Rows)
			}
			if after := f.snapshot(t, t.Context(), nil); after != before {
				t.Fatal("REBUILD_INSERT_ROLLBACK: full five-table old image changed")
			}
			if !slices.Equal(links, originalLinks) || !slices.Equal(resolved, originalResolved) {
				t.Fatal("failed final writer changed caller values")
			}
		})
	}
}

func TestRebuildWritesRefuseWithoutCallerTx(t *testing.T) {
	f := newStableAdapterFixture(t)
	p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
	before := f.snapshot(t, t.Context(), nil)
	ctx := context.WithValue(t.Context(), stableAdapterMarkKey{}, f.trace)
	for _, call := range []func() error{
		func() error { return f.s.AddRebuildMetadata(ctx, p, app.RebuildMetadata{}) },
		func() error { return f.s.AddRebuildUnresolvedLinks(ctx, p, nil) },
		func() error { return f.s.AddRebuildResolvedLinks(ctx, p, nil, nil) },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "rebuild writes require a transaction") {
			t.Fatalf("private write outside caller Tx=%v", err)
		}
	}
	if len(f.trace.snapshot()) != 0 || f.snapshot(t, t.Context(), nil) != before {
		t.Fatal("out-of-Tx rejection executed SQL or changed rows")
	}
}
