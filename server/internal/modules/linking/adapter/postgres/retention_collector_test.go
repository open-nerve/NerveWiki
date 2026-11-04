package postgresadapter

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

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Only the connection/Rows observers are shared with LinksReached. These
// tests call the actual Store on the original manager's maintenance Tx.
// The original stable_page_links tests remain the complete fact/legacy matrix.
type retentionCollectorWatch struct {
	rows                               *linksCollectorRows
	pid                                uint32
	calls                              []string
	inspectArgs, writeArgs, deleteArgs []any
	queryErr, execErr                  error
	earlySQL                           bool
}

type retentionCollectorContext struct {
	context.Context
	watch *retentionCollectorWatch
}

func (c retentionCollectorContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return retentionCollectorTx{Tx: tx, watch: c.watch}
	}
	return v
}

type retentionCollectorTx struct {
	pgx.Tx
	watch *retentionCollectorWatch
}

func (tx retentionCollectorTx) note(sql string, args []any) {
	w := tx.watch
	if !strings.HasPrefix(sql, "-- name: ") {
		panic("retention test saw non-generated Store SQL")
	}
	w.calls = append(w.calls, strings.Fields(sql)[2])
	if tx.Conn().PgConn().PID() != w.pid || len(w.calls) > 1 && w.rows.closes != 1 {
		w.earlySQL = true
	}
	switch sql {
	case gen.InspectPageLinksForRetention:
		w.inspectArgs = slices.Clone(args)
	case gen.DeleteMissingPageLinks:
		w.deleteArgs = slices.Clone(args)
	case gen.WritePageLinksRetainingEqualPrefiltered:
		w.writeArgs = slices.Clone(args)
	}
}

func (tx retentionCollectorTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.note(sql, args)
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if sql == gen.InspectPageLinksForRetention {
		tx.watch.queryErr = err
		if rows != nil {
			tx.watch.rows.Rows = rows
			return tx.watch.rows, err
		}
	}
	return rows, err
}
func (tx retentionCollectorTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.note(sql, args)
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	if err != nil {
		tx.watch.execErr = err
	}
	return tag, err
}
func (tx retentionCollectorTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.note(sql, args)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func retentionCollectorCheckCalls(t *testing.T, w *retentionCollectorWatch, p app.Page, want []string) {
	t.Helper()
	if w.earlySQL || !slices.Equal(w.calls, want) || !reflect.DeepEqual(w.inspectArgs, []any{p.ID, p.NotebookID, int32(2)}) {
		t.Fatalf("actual Store calls=%v inspect args=%#v early SQL/PID=%t; want %v, original three args, Close before successor", w.calls, w.inspectArgs, w.earlySQL, want)
	}
}

func retentionCollectorStages() []string {
	return []string{
		"InspectPageLinksForRetention", "DeleteMissingPageLinks", "DeleteAliasesOf", "DeleteTagsOf",
		"DeletePropertiesOf", "DeleteIndexedPages", "InsertIndexedPage", "WritePageLinksRetainingEqualPrefiltered",
		"InsertTags", "InsertProperties", "InsertAliases",
	}
}

func retentionCollectorCheckTrace(t *testing.T, tr *linksCollectorTrace, pid uint32, p app.Page, cause error) linksCollectorCall {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.calls) == 0 {
		t.Fatal("actual native Inspect query was not traced")
	}
	c := tr.calls[len(tr.calls)-1]
	// No execution-mode option is added to the original three arguments.
	//nolint:errorlint // Observe the exact native error object.
	if c.pid != pid || c.ends != 1 || c.err != cause || tr.active != nil || !reflect.DeepEqual(c.args, []any{p.ID, p.NotebookID, int32(2)}) {
		t.Fatalf("actual Inspect trace pid=%d ends=%d err=%v args=%#v active=%t", c.pid, c.ends, c.err, c.args, tr.active != nil)
	}
	return *c
}

func retentionCollectorSeed(t *testing.T, pool *pgxpool.Pool, n int) (app.Page, domain.Facts, []app.Link) {
	t.Helper()
	p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
	if _, err := pool.Exec(t.Context(), "TRUNCATE page_links,indexed_pages"); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).AddPage(t.Context(), p, domain.Facts{FrontmatterValid: true}); err != nil {
		t.Fatal(err)
	}
	facts := domain.Facts{FrontmatterValid: true, Links: make([]domain.Link, n)}
	want, data := make([]app.Link, n), make([][]any, n)
	for i := range n {
		id := uuid.UUID{15: 11}
		if n > 1 && i == n-1 {
			id = uuid.UUID{15: 12} // Only the last row supplies this old D.
		}
		facts.Links[i] = domain.Link{Kind: "wikilink", Target: "Note", Start: 2 * i, End: 2*i + 1}
		want[i] = app.Link{SourceID: p.ID, Start: 2 * i, Target: "Note", Resolution: domain.Resolution{ID: id, Ambiguous: true, Asset: true}}
		data[i] = []any{p.ID, int32(2 * i), int32(2*i + 1), p.NotebookID, "wikilink", "Note", "note", nil, id, true, true, false}
	}
	slices.Reverse(data)
	linksCollectorCopy(t, pool, data)
	return p, facts, want
}

// All five linking tables, including full nullable rows and the head. The
// tests change the head in the Tx so a successful rollback is non-vacuous.
func retentionCollectorImage(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	var image []string
	for _, table := range []string{"indexed_pages", "page_links", "page_tags", "page_properties", "page_aliases"} {
		var value string
		if err := pool.QueryRow(t.Context(), "SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb)::text FROM "+table+" r").Scan(&value); err != nil {
			t.Fatal(err)
		}
		image = append(image, table+":"+value)
	}
	return image
}

func TestRetentionCollectorActualStoreReadsCompleteSource(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForRetention
	for _, n := range []int{0, 1, 511, 512, 513, 10001} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p, facts, want := retentionCollectorSeed(t, pool, n)
			original := slices.Clone(facts.Links)
			before := retentionCollectorImage(t, pool)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			restore := linksCollectorObserveUUID(t, conn.Conn(), trace)
			defer restore()
			rows := &linksCollectorRows{stable: true, columns: 13}
			w := &retentionCollectorWatch{rows: rows, pid: conn.Conn().PgConn().PID()}
			rollback := errors.New("retention baseline must not commit")
			calls := trace.count()
			err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				store := New(pool)
				if err := store.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				p.Revision = 2
				dropped, err := store.ReplacePage(retentionCollectorContext{Context: ctx, watch: w}, p, facts)
				if err != nil {
					return err
				}
				var targets []uuid.UUID
				if n > 0 {
					targets = []uuid.UUID{{15: 11}}
				}
				if n > 1 {
					targets = append(targets, uuid.UUID{15: 12})
				}
				if !reflect.DeepEqual(dropped, app.Dropped{Targets: targets}) || !reflect.DeepEqual(facts.Links, original) {
					t.Fatalf("complete D/input: dropped=%+v want=%v, input unchanged=%t", dropped, targets, reflect.DeepEqual(facts.Links, original))
				}
				got, err := store.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("actual retained baseline len=%d want=%d, error=%v", len(got), len(want), err)
				}
				return rollback
			})
			if !errors.Is(err, rollback) || trace.count() != calls+1 {
				t.Fatalf("actual manager=%v Inspect queries=%d, want original rollback and one", err, trace.count()-calls)
			}
			rows.check(t, max(1, n), 1, max(1, n), 1)
			retentionCollectorCheckCalls(t, w, p, retentionCollectorStages())
			if !reflect.DeepEqual(w.deleteArgs, []any{p.ID, []int32(nil)}) || len(w.writeArgs) != 13 || w.writeArgs[0] != p.ID || w.writeArgs[1] != p.NotebookID {
				t.Fatalf("same facts actual Delete/Write args=%#v/%#v", w.deleteArgs, w.writeArgs)
			}
			for i, arg := range w.writeArgs[2:] {
				value := reflect.ValueOf(arg)
				if value.Kind() != reflect.Slice || value.Len() != 0 || value.IsNil() != (n == 0) {
					t.Fatalf("same facts actual Write array %d=%#v, want empty with nil=%t", i, arg, n == 0)
				}
			}
			call := retentionCollectorCheckTrace(t, trace, w.pid, p, nil)
			if call.sourceScans != n || n > 0 && (len(call.sourceDest) != 1 || len(call.resolvedDest) == 0) || n == 0 && len(call.sourceDest) != 0 {
				t.Fatalf("actual Store NotebookID scans/destinations=%d/%d resolved=%d; want %d scans at one scratch address (zero for metadata)", call.sourceScans, len(call.sourceDest), len(call.resolvedDest), n)
			}
			for dst := range call.sourceDest {
				if call.resolvedDest[dst] {
					t.Fatal("NotebookID and nullable ResolvedID used the same Scan destination")
				}
			}
			if after := retentionCollectorImage(t, pool); !slices.Equal(after, before) {
				t.Fatal("original manager did not restore the complete five-table image")
			}
			linksCollectorReusable(t, conn)
		})
	}
}

func TestRetentionCollectorActualStorePreservesNativeErrors(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForRetention
	for _, mode := range []string{"query_error", "scan_513", "err_after_512", "ineligible_scan_513", "cancel_after_eof"} {
		t.Run(mode, func(t *testing.T) {
			p, facts, _ := retentionCollectorSeed(t, pool, 1025)
			before := retentionCollectorImage(t, pool)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			rows := &linksCollectorRows{stable: true, columns: 13, mode: mode}
			if mode == "ineligible_scan_513" {
				rows.mode = "scan_513"
			}
			w := &retentionCollectorWatch{rows: rows, pid: conn.Conn().PgConn().PID()}
			var returned error
			var dropped app.Dropped
			err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				store := New(pool)
				if err := store.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, ok := postgres.TxFrom(ctx)
				if !ok || tx.Conn() != conn.Conn() {
					t.Fatal("retention error test lost original Tx/connection")
				}
				if _, err := tx.Exec(ctx, "UPDATE indexed_pages SET revision=99 WHERE node_id=$1", p.ID); err != nil {
					return err
				}
				switch mode {
				case "query_error":
					// Native Parse cannot resolve the relation. Rollback restores
					// it; the actual generated SQL is never changed or replaced.
					_, err = tx.Exec(ctx, "ALTER TABLE page_links RENAME TO retention_unavailable")
				case "ineligible_scan_513":
					_, err = tx.Exec(ctx, "DELETE FROM indexed_pages WHERE node_id=$1", p.ID)
				}
				if err != nil {
					return err
				}
				queryCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				rows.cancel = cancel
				dropped, returned = store.ReplacePage(retentionCollectorContext{Context: queryCtx, watch: w}, p, facts)
				if mode != "query_error" && queryCtx.Err() != context.Canceled {
					t.Fatal("actual callback context was not canceled at the native boundary")
				}
				return returned
			})
			cause, prefix := rows.cause, fmt.Sprintf("inspect the retained links of page %s: ", p.ID)
			wantCalls := []string{"InspectPageLinksForRetention"}
			switch mode {
			case "query_error":
				cause = w.queryErr
				var pg *pgconn.PgError
				if !errors.As(cause, &pg) || pg.Code != "42P01" || rows.scans != 0 || rows.nextTrue != 0 || rows.nextFalse != 0 || rows.errs != 0 || rows.closes != 0 {
					t.Fatalf("native Query failure=%v followed by Rows operations: %+v", cause, rows)
				}
			case "scan_513", "ineligible_scan_513":
				rows.check(t, 513, 0, 513, 0)
			case "err_after_512":
				rows.check(t, 512, 1, 512, 1)
			case "cancel_after_eof":
				rows.check(t, 1025, 1, 1025, 1)
				cause, prefix = w.execErr, "delete the links of 1 pages: "
				wantCalls = append(wantCalls, "DeleteMissingPageLinks")
				if !errors.Is(cause, context.Canceled) {
					t.Fatalf("EOF cancellation did not reach original DeleteMissing stage: %v", cause)
				}
			}
			if cause == nil || !errors.Is(returned, cause) || !errors.Is(err, cause) || returned.Error() != prefix+cause.Error() || !reflect.DeepEqual(dropped, app.Dropped{}) {
				t.Fatalf("actual Store/manager=%v/%v cause=%v Drops=%+v, want original wrapper/cause and zero output", returned, err, cause, dropped)
			}
			if mode != "cancel_after_eof" && errors.Is(returned, context.Canceled) {
				t.Fatal("later cancellation replaced the original native cause")
			}
			retentionCollectorCheckCalls(t, w, p, wantCalls)
			readCause := cause
			if mode == "cancel_after_eof" {
				readCause = nil
			}
			retentionCollectorCheckTrace(t, trace, w.pid, p, readCause)
			if after := retentionCollectorImage(t, pool); !slices.Equal(after, before) {
				t.Fatal("native failure or EOF cancellation leaked the head marker or any indexed row")
			}
			linksCollectorReusable(t, conn)
		})
	}
}

func TestRetentionCollectorKeepsLiteralOrderAndAllArrays(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
	ids := []uuid.UUID{{15: 1}, {0: 0x7f, 15: 0xff}, {0: 0x80}, {0: 0xff}, {0: 0xff, 15: 1}}
	old := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "wikilink", Target: "Keep.md", Property: "p.keep", Anchor: "K", Display: "label", Start: 0, End: 7},
		{Kind: "link", Target: "Old.md", Start: 20, End: 26},
		{Kind: "link", Target: "Drop", Start: 30, End: 34},
		{Kind: "link", Target: "AlsoKeep", Start: 40, End: 48},
		{Kind: "link", Target: "Gone", Start: 60, End: 64},
	}}
	if err := New(pool).AddPage(t.Context(), p, old); err != nil {
		t.Fatal(err)
	}
	for i, start := range []int{0, 20, 30, 40, 60} {
		if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$3,ambiguous=true,resolved_asset=true WHERE source_id=$1 AND range_start=$2", p.ID, start, ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	// Literal UUID boundaries, also checked against PostgreSQL's own ordering.
	rows, err := pool.Query(t.Context(), "SELECT DISTINCT resolved_id FROM page_links WHERE source_id=$1 ORDER BY resolved_id", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var pgIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		pgIDs = append(pgIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || !slices.Equal(pgIDs, ids) {
		t.Fatalf("literal UUID order=%v PG=%v err=%v", ids, pgIDs, err)
	}
	incoming := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "image", Property: "p.new", Target: "Fresh.png", Anchor: "newA", Display: "newD", Start: 90, End: 99, Quote: '"', Aliases: true},
		old.Links[3],
		{Kind: "embed", Target: "New.md", Anchor: "changedA", Display: "changedD", Start: 20, End: 26, Quote: '\''},
		old.Links[0],
		{Kind: "link", Target: "A//B", Start: 70, End: 74},
	}}
	original := slices.Clone(incoming.Links)
	want := gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID,
		RangeStarts: []int32{90, 20, 70}, RangeEnds: []int32{99, 26, 74}, Kinds: []string{"image", "embed", "link"},
		PropertyKeys: []string{"p.new", "", ""}, InFrontmatter: []bool{true, true, false}, Targets: []string{"Fresh.png", "New.md", "A//B"},
		Anchors: []string{"newA", "changedA", ""}, Displays: []string{"newD", "changedD", ""},
		TargetKeys: []string{"fresh.png", "new", ""}, TargetAltKeys: []string{"", "new.md", ""}, Aliases: []bool{true, false, false}}
	before := retentionCollectorImage(t, pool)
	rollback := errors.New("end literal retention baseline")
	err = postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
		store := New(pool)
		if err := store.Lock(ctx, p.NotebookID); err != nil {
			return err
		}
		tx, _ := postgres.TxFrom(ctx)
		w := &retentionCollectorWatch{rows: &linksCollectorRows{stable: true, columns: 13}, pid: tx.Conn().PgConn().PID()}
		p.Revision = 2
		dropped, err := store.ReplacePage(retentionCollectorContext{Context: ctx, watch: w}, p, incoming)
		if err != nil {
			return err
		}
		retentionCollectorCheckCalls(t, w, p, retentionCollectorStages())
		w.rows.check(t, 5, 1, 5, 1)
		if !reflect.DeepEqual(w.deleteArgs, []any{p.ID, []int32{30, 60}}) || !reflect.DeepEqual(w.writeArgs, []any{p.ID, p.NotebookID, want.RangeStarts, want.RangeEnds, want.Kinds, want.PropertyKeys, want.InFrontmatter, want.Targets, want.Anchors, want.Displays, want.TargetKeys, want.TargetAltKeys, want.Aliases}) {
			t.Fatalf("actual missing/write arrays=%#v/%#v differ from literal original-order compact values", w.deleteArgs, w.writeArgs)
		}
		got, err := store.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
		wantLinks := []app.Link{
			{SourceID: p.ID, Start: 0, Target: "Keep.md", Resolution: domain.Resolution{ID: ids[0], Ambiguous: true, Asset: true}},
			{SourceID: p.ID, Start: 20, Target: "New.md"},
			{SourceID: p.ID, Start: 40, Target: "AlsoKeep", Resolution: domain.Resolution{ID: ids[3], Ambiguous: true, Asset: true}},
			{SourceID: p.ID, Start: 70, Target: "A//B"},
			{SourceID: p.ID, Start: 90, Target: "Fresh.png", Aliases: true},
		}
		if err != nil || !slices.Equal(got, wantLinks) || !reflect.DeepEqual(dropped, app.Dropped{Targets: ids}) || !reflect.DeepEqual(incoming.Links, original) {
			t.Fatalf("actual mixed Store links=%+v D=%+v err=%v input unchanged=%t", got, dropped, err, reflect.DeepEqual(incoming.Links, original))
		}
		return rollback
	})
	if !errors.Is(err, rollback) || !slices.Equal(retentionCollectorImage(t, pool), before) {
		t.Fatalf("literal Store rollback/image=%v", err)
	}
	// After the actual Store path and rollback, inspect the private output's
	// exact allocation and all eleven arrays against independent literals.
	params := linksParams(p, incoming.Links)
	plan, err := inspectPageLinksForRetention(t.Context(), pool, gen.InspectPageLinksForRetentionParams{SourceID: p.ID, NotebookID: p.NotebookID, Extractor: 2}, &params)
	if err != nil || !plan.canRetain || !slices.Equal(plan.targets, ids) || !slices.Equal(plan.missing, []int32{30, 60}) || !reflect.DeepEqual(params, want) || cap(plan.targets) != len(ids) {
		t.Fatalf("literal compact plan=%+v params=%+v error=%v; want all five D, missing30/60, exact targets capacity and aligned 90/20/70 arrays", plan, params, err)
	}
}
