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

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Reuse the real native Rows forwarding/fault helper. This wrapper merely
// selects the newly generated statement; it never changes Next or the SQL.
type preparedWatch struct {
	rows              *linksCollectorRows
	calls             []string
	residualArgs      []any
	pid               uint32
	queryErr, execErr error
	early             bool
}
type preparedContext struct {
	context.Context
	watch *preparedWatch
}

func (c preparedContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return preparedTx{Tx: tx, watch: c.watch}
	}
	return v
}

type preparedTx struct {
	pgx.Tx
	watch *preparedWatch
}

func (tx preparedTx) note(sql string) {
	w := tx.watch
	w.calls = append(w.calls, strings.Fields(sql)[2])
	if tx.Conn().PgConn().PID() != w.pid || len(w.calls) > 1 && w.rows.closes != 1 {
		w.early = true
	}
}
func (tx preparedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.note(sql)
	if sql == gen.LinksReachedExcept {
		tx.watch.residualArgs = slices.Clone(args)
	}
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if sql == gen.InspectPageLinksForPreparation {
		tx.watch.queryErr = err
		if rows != nil {
			tx.watch.rows.Rows = rows
			if strings.HasPrefix(tx.watch.rows.mode, "null_") {
				return preparedFlagRows{tx.watch.rows}, err
			}
			return tx.watch.rows, err
		}
	}
	return rows, err
}

// Native NULL-to-bool Scan has already closed Rows before cancellation.
type preparedFlagRows struct{ *linksCollectorRows }

func (r preparedFlagRows) Scan(dest ...any) error {
	err := r.linksCollectorRows.Scan(dest...)
	if err != nil {
		r.cancel()
	}
	return err
}

func (tx preparedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.note(sql)
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	if err != nil {
		tx.watch.execErr = err
	}
	return tag, err
}
func (tx preparedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.note(sql)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestPreparedSourceActualStoreCompleteAndEmpty(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForPreparation
	for _, n := range []int{0, 1, 513, 10000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p, facts, want := retentionCollectorSeed(t, pool, n)
			before := retentionCollectorImage(t, pool)
			rollback := errors.New("prepared source baseline rollback")
			calls := trace.count()
			err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
				s := New(pool)
				if err := s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				w := &preparedWatch{rows: &linksCollectorRows{stable: true, columns: 15}, pid: tx.Conn().PgConn().PID()}
				watched := preparedContext{Context: ctx, watch: w}
				unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
				p.Revision = 2
				plan := s.NewSource(watched, unit, p, facts)
				if plan == nil || len(w.calls) != 0 || trace.count() != calls {
					t.Fatal("PREPARE_NO_SQL: factory absent or ran SQL")
				}
				if assets, err := plan.ReadBefore(watched, &app.SourcePreparationUnit{NotebookID: p.NotebookID}); err == nil || assets != nil || len(w.calls) != 0 {
					t.Fatal("wrong unit performed native read")
				}
				assets, err := plan.ReadBefore(watched, unit)
				if err != nil {
					return err
				}
				if len(assets) != n || n > 0 && assets[0] != (uuid.UUID{15: 11}) || n > 1 && assets[n-1] != (uuid.UUID{15: 12}) {
					t.Fatalf("PREPARED_F_COMPLETE: %d assets for%d", len(assets), n)
				}
				w.rows.check(t, max(1, n), 1, max(1, n), 1)
				actual := plan.(*preparedSource)
				if !actual.read.canRetain || !slices.Equal(actual.read.inputs, want) {
					t.Fatal("PREPARED_INPUTS: complete native baseline lost")
				}
				// Observe actual long-lived plan arrays, not an allocation model.
				values := reflect.ValueOf(actual.read.links)
				for i := 0; i < values.NumField(); i++ {
					v := values.Field(i)
					if v.Kind() == reflect.Slice && (v.Len() != 0 || v.Cap() != 0 || v.IsNil() != (n == 0)) {
						t.Fatalf("PREPARED_SPARSE_CAP: %s retains %d/%d nil=%t; want 0/0 nil=%t", values.Type().Field(i).Name, v.Len(), v.Cap(), v.IsNil(), n == 0)
					}
				}
				callsBeforeApply := len(w.calls)
				if _, supplied, err := plan.Apply(watched, &app.SourcePreparationUnit{NotebookID: p.NotebookID}); err == nil || supplied != nil || len(w.calls) != callsBeforeApply {
					t.Fatal("wrong unit consumed prepared Apply")
				}
				old, supplied, err := plan.Apply(watched, unit)
				if err != nil {
					return err
				}
				var targets []uuid.UUID
				if n > 0 {
					targets = append(targets, uuid.UUID{15: 11})
				}
				if n > 1 {
					targets = append(targets, uuid.UUID{15: 12})
				}
				if !slices.Equal(old.Targets, targets) || supplied == nil || supplied.SourceID != p.ID || !slices.Equal(supplied.Links, want) || supplied.Links == nil {
					t.Fatal("PREPARED_APPLY: full baseline/empty evidence lost")
				}
				if len(actual.read.inputs) != 0 || actual.read.links.RangeStarts != nil || actual.facts.Links != nil {
					t.Fatal("consumed handle retains prepared arrays")
				}
				count := len(w.calls)
				if _, _, err := plan.Apply(watched, unit); err == nil || len(w.calls) != count {
					t.Fatal("prepared handle consumed twice")
				}
				got, err := s.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("actual persisted comparison baseline: %v", err)
				}
				residual, err := s.LinksExcept(watched, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}}, []uuid.UUID{p.ID})
				if err != nil || residual == nil || len(residual) != 0 {
					t.Fatal("complete empty residual was skipped/changed")
				}
				if len(w.residualArgs) != 6 || w.residualArgs[0] != pgx.QueryExecModeDescribeExec || len(w.residualArgs[5].([]uuid.UUID)) != 0 || !slices.Equal(w.residualArgs[1].([]uuid.UUID), []uuid.UUID{p.ID}) {
					t.Fatalf("PREPARED_RESIDUAL_ARGS: actual Planned arguments retain supplied source: %#v", w.residualArgs)
				}
				wantCalls := retentionCollectorStages()
				wantCalls[0] = "InspectPageLinksForPreparation"
				wantCalls = append(wantCalls, "LinksReachedExcept")
				if w.early || !slices.Equal(w.calls, wantCalls) || trace.count() != calls+1 {
					t.Fatalf("PREPARED_ONCE: calls%v early%t", w.calls, w.early)
				}
				return rollback
			})
			if !errors.Is(err, rollback) || !slices.Equal(retentionCollectorImage(t, pool), before) {
				t.Fatalf("prepared rollback/image: %v", err)
			}
		})
	}
}

func TestPreparedSourceValuesSparseOrderAndResidual(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	s := New(pool)
	p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
	a, prop, tail := uuid.UUID{15: 11}, uuid.UUID{15: 12}, uuid.UUID{15: 13}
	old := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "image", Target: "A.png", Display: "label", Start: 0, End: 5},
		{Kind: "link", Target: "Page", Quote: '"', Aliases: true, Start: 10, End: 14},
		{Kind: "link", Target: "Unresolved", Start: 20, End: 30},
		{Kind: "link", Target: "Gone", Start: 30, End: 34},
		{Kind: "link", Target: "Change", Start: 40, End: 46},
	}}
	if err := s.AddPage(t.Context(), p, old); err != nil {
		t.Fatal(err)
	}
	for i, id := range []any{a, prop, nil, tail, a} {
		if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$3,ambiguous=$4,resolved_asset=$5 WHERE source_id=$1 AND range_start=$2", p.ID, []int{0, 10, 20, 30, 40}[i], id, id != nil, id != nil && i != 4); err != nil {
			t.Fatal(err)
		}
	}
	other := uuid.UUID{15: 60}
	if err := s.AddPage(t.Context(), app.Page{ID: other, NotebookID: p.NotebookID, Revision: 1}, domain.Facts{Links: []domain.Link{{Kind: "link", Target: "A.png", Start: 3, End: 8}}}); err != nil {
		t.Fatal(err)
	}
	incoming := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "embed", Target: "New.md", Anchor: "newA", Display: "newD", Start: 50, End: 56}, old.Links[2], old.Links[1], old.Links[0],
		{Kind: "image", Property: "p", Target: "Changed.md", Anchor: "a", Display: "d", Start: 40, End: 49, Aliases: true},
	}}
	original := slices.Clone(incoming.Links)
	want := []app.Link{
		{SourceID: p.ID, Start: 0, Target: "A.png", Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
		{SourceID: p.ID, Start: 10, Target: "Page", Aliases: true, Resolution: domain.Resolution{ID: prop, Ambiguous: true, Asset: true}},
		{SourceID: p.ID, Start: 20, Target: "Unresolved"},
		{SourceID: p.ID, Start: 40, Target: "Changed.md", Aliases: true},
		{SourceID: p.ID, Start: 50, Target: "New.md"},
	}
	before := retentionCollectorImage(t, pool)
	rollback := errors.New("literal preparation rollback")
	err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.Lock(ctx, p.NotebookID); err != nil {
			return err
		}
		unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
		p.Revision = 2
		plan := s.NewSource(ctx, unit, p, incoming)
		assets, err := plan.ReadBefore(ctx, unit)
		if err != nil {
			return err
		}
		if !slices.Equal(assets, []uuid.UUID{a, tail}) {
			t.Fatalf("literal F projection=%v", assets)
		}
		read := plan.(*preparedSource).read
		wantParams := gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID, RangeStarts: []int32{50, 40}, RangeEnds: []int32{56, 49},
			Kinds: []string{"embed", "image"}, PropertyKeys: []string{"", "p"}, InFrontmatter: []bool{false, true},
			Targets: []string{"New.md", "Changed.md"}, Anchors: []string{"newA", "a"}, Displays: []string{"newD", "d"},
			TargetKeys: []string{"new", "changed"}, TargetAltKeys: []string{"new.md", "changed.md"}, Aliases: []bool{false, true}}
		if !reflect.DeepEqual(read.links, wantParams) || !slices.Equal(read.missing, []int32{30}) || !slices.Equal(read.inputs, want) {
			t.Fatalf("PREPARED_LITERAL: params%+v missing%v inputs%+v", read.links, read.missing, read.inputs)
		}
		values := reflect.ValueOf(read.links)
		for i := 0; i < values.NumField(); i++ {
			v := values.Field(i)
			if v.Kind() == reflect.Slice && v.Cap() != 2 {
				t.Fatal("sparse arrays retained old full capacity")
			}
		}
		dropped, supplied, err := plan.Apply(ctx, unit)
		if err != nil {
			return err
		}
		if !slices.Equal(dropped.Targets, []uuid.UUID{a, prop, tail}) || !slices.Equal(supplied.Links, want) {
			t.Fatal("old D or complete fresh baseline lost")
		}
		got, err := s.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("SQL baseline differs from supplied inputs: %v", err)
		}
		reach := domain.Reach{Sources: []uuid.UUID{p.ID, other}, Keys: []string{"a.png", "page", "changed", "changed.md"}, Targets: []uuid.UUID{a}}
		// p's row zero simultaneously satisfies source, key and old target.
		// The alt-key arm also reaches Changed.md. Its outer exclusion must
		// hold independently of the key/resolved arms' inner range filters.
		tx, _ := postgres.TxFrom(ctx)
		w := &preparedWatch{rows: &linksCollectorRows{closes: 1}, pid: tx.Conn().PgConn().PID()}
		residual, err := s.LinksExcept(preparedContext{Context: ctx, watch: w}, p.NotebookID, reach, []uuid.UUID{p.ID})
		if err != nil || !slices.Equal(residual, []app.Link{{SourceID: other, Start: 3, Target: "A.png"}}) {
			t.Fatalf("global OR/exclusion residual%+v err%v", residual, err)
		}
		if len(w.residualArgs) != 6 || w.residualArgs[0] != pgx.QueryExecModeDescribeExec || !slices.Equal(w.residualArgs[5].([]uuid.UUID), []uuid.UUID{other}) || !slices.Equal(reach.Sources, []uuid.UUID{p.ID, other}) {
			t.Fatalf("residual actual source arm/caller relation changed: %#v / %v", w.residualArgs, reach.Sources)
		}
		all, err := s.Links(ctx, p.NotebookID, reach)
		if err != nil {
			return err
		}
		for _, excluded := range [][]uuid.UUID{nil, {}} {
			got, err := s.LinksExcept(ctx, p.NotebookID, reach, excluded)
			if err != nil || !slices.Equal(got, all) {
				t.Fatalf("nil/empty excluded lost complete relation: %v", err)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) || !slices.Equal(retentionCollectorImage(t, pool), before) || !slices.Equal(incoming.Links, original) {
		t.Fatal("literal preparation changed caller facts or leaked writes")
	}
}

func TestPreparedSourceNativeFailuresAndNegative(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForPreparation
	for _, mode := range []string{"query_error", "scan_513", "err_after_512", "ineligible_scan_513", "cancel_after_eof", "negative", "null_asset", "null_ambiguous", "null_aliases"} {
		t.Run(mode, func(t *testing.T) {
			p, facts, _ := retentionCollectorSeed(t, pool, 1025)
			before := retentionCollectorImage(t, pool)
			rows := &linksCollectorRows{stable: true, columns: 15, mode: mode}
			if mode == "ineligible_scan_513" {
				rows.mode = "scan_513"
			}
			w := &preparedWatch{rows: rows}
			var returned error
			var assets []uuid.UUID
			var supplied *app.SourceLinks
			rollback := errors.New("negative preparation rollback")
			err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
				s := New(pool)
				if err := s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				w.pid = tx.Conn().PgConn().PID()
				if _, err := tx.Exec(ctx, "UPDATE indexed_pages SET frontmatter_valid=false WHERE node_id=$1", p.ID); err != nil {
					return err
				}
				if mode == "query_error" {
					if _, err := tx.Exec(ctx, "ALTER TABLE page_links RENAME TO prepared_unavailable"); err != nil {
						return err
					}
				}
				if mode == "negative" || mode == "ineligible_scan_513" {
					if _, err := tx.Exec(ctx, "INSERT INTO page_tags(source_id,tag_key,notebook_id,tag,count) VALUES($1,'foreign',$2,'Foreign',1)", p.ID, uuid.UUID{15: 99}); err != nil {
						return err
					}
				}
				if mode == "null_asset" {
					if _, err := tx.Exec(ctx, "ALTER TABLE page_links ALTER COLUMN resolved_asset DROP NOT NULL"); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, "UPDATE page_links SET resolved_asset=NULL WHERE source_id=$1 AND range_start=2048", p.ID); err != nil {
						return err
					}
				}
				if mode == "null_ambiguous" {
					if _, err := tx.Exec(ctx, "ALTER TABLE page_links ALTER COLUMN ambiguous DROP NOT NULL"); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, "UPDATE page_links SET ambiguous=NULL WHERE source_id=$1 AND range_start=2048", p.ID); err != nil {
						return err
					}
				}
				if mode == "null_aliases" {
					if _, err := tx.Exec(ctx, "ALTER TABLE page_links ALTER COLUMN aliases DROP NOT NULL"); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, "UPDATE page_links SET aliases=NULL WHERE source_id=$1 AND range_start=2048", p.ID); err != nil {
						return err
					}
				}
				callback, cancel := context.WithCancel(ctx)
				defer cancel()
				rows.cancel = cancel
				watched := preparedContext{Context: callback, watch: w}
				unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
				p.Revision = 2
				plan := s.NewSource(watched, unit, p, facts)
				assets, returned = plan.ReadBefore(watched, unit)
				if mode == "negative" {
					if returned != nil || len(assets) != 1025 || plan.(*preparedSource).read.canRetain {
						t.Fatal("negative complete native read/F lost")
					}
					_, supplied, returned = plan.Apply(watched, unit)
					if returned != nil || supplied != nil {
						t.Fatal("negative preparation supplied retained baseline")
					}
					if slices.Contains(w.calls, "InspectPageLinksForRetention") || slices.Contains(w.calls, "WritePageLinksRetainingEqualPrefiltered") || !slices.Contains(w.calls, "InsertLinks") {
						t.Fatalf("negative did not directly run original legacy: %v", w.calls)
					}
					return rollback
				}
				if mode == "cancel_after_eof" {
					if returned != nil || len(assets) != 1025 {
						t.Fatal("EOF success was overwritten by eager cancellation")
					}
					_, supplied, returned = plan.Apply(watched, unit)
				} else if assets != nil {
					t.Fatal("native failure returned partial old F")
				}
				return returned
			})
			if w.early || !slices.Equal(retentionCollectorImage(t, pool), before) {
				t.Fatal("native failure/negative leaked image or ran successor before Close")
			}
			if mode == "negative" {
				if !errors.Is(err, rollback) {
					t.Fatal(err)
				}
				rows.check(t, 1025, 1, 1025, 1)
				return
			}
			cause := rows.cause
			prefix := fmt.Sprintf("inspect the retained links of page %s: ", p.ID)
			wantCalls := []string{"InspectPageLinksForPreparation"}
			switch mode {
			case "query_error":
				cause = w.queryErr
				var pg *pgconn.PgError
				if !errors.As(cause, &pg) || pg.Code != "42P01" || rows.scans != 0 || rows.closes != 0 {
					t.Fatalf("actual Query failure %v", cause)
				}
			case "scan_513", "ineligible_scan_513":
				rows.check(t, 513, 0, 513, 0)
			case "err_after_512":
				rows.check(t, 512, 1, 512, 1)
			case "null_asset", "null_ambiguous", "null_aliases":
				rows.check(t, 1025, 0, 1025, 0)
				cause = rows.Rows.Err()
				if cause == nil || !strings.Contains(cause.Error(), "NULL") {
					t.Fatalf("real nullable bool fault was normalized away: %v", cause)
				}
			case "cancel_after_eof":
				rows.check(t, 1025, 1, 1025, 1)
				cause = w.execErr
				prefix = "delete the links of 1 pages: "
				wantCalls = append(wantCalls, "DeleteMissingPageLinks")
				if !errors.Is(cause, context.Canceled) {
					t.Fatal("EOF cancellation missed original SQL stage")
				}
			}
			if cause == nil || !errors.Is(returned, cause) || !errors.Is(err, cause) || returned.Error() != prefix+cause.Error() || supplied != nil || !slices.Equal(w.calls, wantCalls) {
				t.Fatalf("PREPARED_CAUSE: returned%v manager%v calls%v", returned, err, w.calls)
			}
			if mode != "cancel_after_eof" && errors.Is(returned, context.Canceled) {
				t.Fatal("later cancellation replaced native cause")
			}
		})
	}
}

func TestPreparedSourceRejectsAnotherTransactionOnSameConnection(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForPreparation
	p, facts, _ := retentionCollectorSeed(t, pool, 1)
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	manager := postgres.NewTxManager(pool, time.Second)
	unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
	var plan, readPlan app.PreparedSource
	rollback := errors.New("prepared scope ends")
	err = manager.WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		s := New(pool)
		if err := s.Lock(ctx, p.NotebookID); err != nil {
			return err
		}
		p.Revision = 2
		plan = s.NewSource(ctx, unit, p, facts)
		if plan == nil {
			t.Fatal("actual transaction was not admitted")
		}
		readPlan = s.NewSource(ctx, unit, p, facts)
		if _, err := readPlan.ReadBefore(ctx, unit); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	calls := trace.count()
	err = manager.WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		if tx.Conn() != conn.Conn() {
			t.Fatal("scope test did not reuse exact physical connection")
		}
		if assets, err := plan.ReadBefore(ctx, unit); err == nil || assets != nil {
			t.Fatal("closed transaction's plan read in a new transaction")
		}
		if _, supplied, err := readPlan.Apply(ctx, unit); err == nil || supplied != nil {
			t.Fatal("prepared plan applied in a later transaction on the same connection")
		}
		return rollback
	})
	if !errors.Is(err, rollback) || trace.count() != calls {
		t.Fatal("wrong transaction touched native Inspect")
	}
}

func TestPreparedSourceWitnessCapStillReadsLateZero(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	for _, lateZero := range []bool{false, true} {
		t.Run(fmt.Sprint(lateZero), func(t *testing.T) {
			p, facts, _ := retentionCollectorSeed(t, pool, 26)
			for i := range 26 {
				id := uuid.UUID{15: byte(100 + i)}
				if lateZero && i == 25 {
					id = uuid.Nil()
				}
				if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$2 WHERE source_id=$1 AND range_start=$3", p.ID, id, 2*i); err != nil {
					t.Fatal(err)
				}
			}
			rollback := errors.New("witness preparation rollback")
			err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
				s := New(pool)
				if err := s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				w := &preparedWatch{rows: &linksCollectorRows{stable: true, columns: 15}, pid: tx.Conn().PgConn().PID()}
				watched := preparedContext{Context: ctx, watch: w}
				unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
				p.Revision = 2
				plan := s.NewSource(watched, unit, p, facts)
				assets, err := plan.ReadBefore(watched, unit)
				if err != nil {
					return err
				}
				read := plan.(*preparedSource).read
				w.rows.check(t, 26, 1, 26, 1)
				if len(assets) != 26 || read.canRetain == lateZero {
					t.Fatal("target witness cap truncated F or skipped late zero eligibility")
				}
				if !lateZero {
					if len(read.targets) != 21 {
						t.Fatalf("target witness length=%d, want21", len(read.targets))
					}
					for i, id := range read.targets {
						if id != (uuid.UUID{15: byte(100 + i)}) {
							t.Fatal("native first 21 distinct witnesses changed")
						}
					}
				} else if len(read.targets) != 0 || read.inputs != nil {
					t.Fatal("negative preparation retained partial eligible output")
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
		})
	}
}
