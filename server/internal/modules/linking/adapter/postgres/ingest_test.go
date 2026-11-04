package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type ingestBeforeRows map[uuid.UUID]app.IngestNodeBefore

func (m ingestBeforeRows) NodesBefore(_ context.Context, ids []uuid.UUID) ([]app.IngestNodeBefore, error) {
	out := make([]app.IngestNodeBefore, len(ids))
	for i, id := range ids {
		out[i] = m[id]
		out[i].ID = id
	}
	return out, nil
}
func ingestKnownPage(id uuid.UUID, source bool) app.IngestNodeBefore {
	return app.IngestNodeBefore{ID: id, Exists: true, Kind: "page", InSource: source, Revision: 1, RevisionKnown: true, PageBytes: 1, PageBytesKnown: true}
}
func ingestSort(ids ...uuid.UUID) []uuid.UUID { slices.SortFunc(ids, uuid.UUID.Compare); return ids }

// The shared lintTrace sees the legacy Lint queries. Observe the exact heads
// and private old-F names too, so a public Capture cannot hide a private call.
// The inherited TraceQueryEnd marks completion. All fixture writes happen
// later in Capture's direct callback, never on a busy traced connection.
type ingestPublicCaptureTrace struct{ lintTrace }

func (tr *ingestPublicCaptureTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	fields := strings.Fields(data.SQL)
	if len(fields) > 2 && fields[0] == "--" && fields[1] == "name:" {
		switch fields[2] {
		case "IngestIndexedHeads", "IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter":
			tr.mu.Lock()
			i := len(tr.observed)
			tr.observed = append(tr.observed, lintObservedMode(conn, data))
			tr.mu.Unlock()
			return context.WithValue(ctx, lintModeQueryKey{}, i)
		}
	}
	return tr.lintTrace.TraceQueryStart(ctx, conn, data)
}

func TestIngestBefore(t *testing.T) {
	t.Run("possible_full", func(t *testing.T) {
		f := newFixture(t)
		p, s, a := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true, Links: []domain.Link{{Kind: "wikilink", Target: "S", Start: 1, End: 2}, {Kind: "link", Target: "a.txt", Start: 4, End: 9}}})
		f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, domain.Facts{})
		f.resolve(t, p, domain.Resolution{ID: s}, 1)
		f.resolve(t, p, domain.Resolution{ID: a, Asset: true}, 4)
		before := ingestBeforeRows{p: ingestKnownPage(p, false), s: ingestKnownPage(s, true), a: {ID: a, Exists: true, Kind: "asset", InSource: true}}
		source := before[s]
		source.PageBytesKnown = false
		source.RevisionKnown = false
		before[s] = source
		reads := app.QualifiedBefore{Heads: postgresadapter.NewIngestReads(f.pool, 5*1024*1024), Reads: app.NewLintReads(postgresadapter.NewLintReads(f.pool, 5*1024*1024), 5*1024*1024)}
		var direct []app.OldQualifiedFacts
		err := reads.Capture(t.Context(), f.eng, before, []uuid.UUID{p}, func(v app.OldQualifiedFacts) error { direct = append(direct, v); return nil }, func(app.OldSourceFacts) error { return nil })
		if err != nil || len(direct) != 1 {
			t.Fatalf("capture=%+v, %v", direct, err)
		}
		got := direct[0]
		if got.State != app.OldFactsUnknown || got.Unknown.Extent != app.OldUnknownPossibleTargets || !slices.Equal(got.Unknown.PossibleTargets, ingestSort(a, s)) || len(got.References) != 1 || got.References[0].TargetID != a {
			t.Fatalf("finite old upper bound lost known or uncertain member: %+v", got)
		}
	})
	t.Run("old_resolution", func(t *testing.T) {
		f := newFixture(t)
		p, s, replacement := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "[[path/now/T]]"))
		f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, domain.Facts{})
		f.replace(t, app.Page{ID: replacement, NotebookID: f.eng, Revision: 1}, domain.Facts{})
		f.resolve(t, p, domain.Resolution{ID: s}, 2)
		before := ingestBeforeRows{p: ingestKnownPage(p, false), s: ingestKnownPage(s, true), replacement: ingestKnownPage(replacement, true)}
		reads := app.QualifiedBefore{Heads: postgresadapter.NewIngestReads(f.pool, 5*1024*1024), Reads: app.NewLintReads(postgresadapter.NewLintReads(f.pool, 5*1024*1024), 5*1024*1024)}
		err := reads.Capture(t.Context(), f.eng, before, []uuid.UUID{p}, func(v app.OldQualifiedFacts) error {
			if v.State != app.OldFactsKnown || len(v.References) != 1 || v.References[0].TargetID != s {
				t.Fatalf("old resolution was reinterpreted: %+v", v)
			}
			return nil
		}, func(app.OldSourceFacts) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("source_role_independent", func(t *testing.T) {
		f := newFixture(t)
		s, a := uuid.NewV7(), uuid.NewV7()
		f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, originFacts(t, "[a](a.txt)"))
		f.resolve(t, s, domain.Resolution{ID: a, Asset: true}, 4)
		before := ingestBeforeRows{s: ingestKnownPage(s, true), a: {ID: a, Exists: true, Kind: "asset", InSource: true}}
		reads := app.QualifiedBefore{Heads: postgresadapter.NewIngestReads(f.pool, 5*1024*1024), Reads: app.NewLintReads(postgresadapter.NewLintReads(f.pool, 5*1024*1024), 5*1024*1024)}
		order := []string{}
		err := reads.Capture(t.Context(), f.eng, before, []uuid.UUID{s}, func(v app.OldQualifiedFacts) error {
			order = append(order, "D")
			if v.State != app.OldFactsKnownIneligible {
				t.Fatalf("source D=%+v", v)
			}
			return nil
		}, func(v app.OldSourceFacts) error {
			order = append(order, "F")
			if v.State != app.OldFactsKnown || !slices.Equal(v.AssetIDs, []uuid.UUID{a}) {
				t.Fatalf("source F=%+v", v)
			}
			return nil
		})
		if err != nil || !slices.Equal(order, []string{"D", "F"}) {
			t.Fatalf("capture order %v, %v", order, err)
		}
	})
	for _, tc := range []struct {
		name         string
		inTx, locked bool
		changeSQL    string
	}{
		{"public_pool_head_change", false, false, `DELETE FROM indexed_pages WHERE notebook_id=$1 AND node_id=$2`},
		{"public_tx_without_lock_head_change", true, false, `UPDATE indexed_pages SET extractor=1 WHERE notebook_id=$1 AND node_id=$2`},
		{"public_locked_callback_changes_head", true, true, `DELETE FROM indexed_pages WHERE notebook_id=$1 AND node_id=$2`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			s, a := uuid.NewV7(), uuid.NewV7()
			f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, originFacts(t, "[a](a.txt)"))
			f.resolve(t, s, domain.Resolution{ID: a, Asset: true}, 4)
			before := ingestBeforeRows{s: ingestKnownPage(s, true), a: {ID: a, Exists: true, Kind: "asset", InSource: true}}
			tr := &ingestPublicCaptureTrace{}
			pool := lintPool(t, f, tr, 1)
			reads := app.QualifiedBefore{Heads: postgresadapter.NewIngestReads(pool, 5*1024*1024), Reads: app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)}
			changeHead := func(ctx context.Context, db postgres.Querier) {
				t.Helper()
				tag, err := db.Exec(ctx, tc.changeSQL, f.eng, s)
				if err != nil || tag.RowsAffected() != 1 {
					t.Fatalf("change exactly the loaded source head: tag=%v err=%v", tag, err)
				}
				var edges int
				err = db.QueryRow(ctx, `SELECT count(*) FROM page_links
					WHERE notebook_id=$1 AND source_id=$2 AND range_start=4
					AND resolved_id=$3 AND resolved_asset AND property_key IS NULL`, f.eng, s, a).Scan(&edges)
				if err != nil || edges != 1 {
					t.Fatalf("S-to-A body must survive the head change: count=%d err=%v", edges, err)
				}
			}
			run := func(ctx context.Context) error {
				if tc.locked {
					if err := f.s.Lock(ctx, f.eng); err != nil {
						return err
					}
				}
				capture := func(change bool, want app.OldSourceFacts, wantQueries []string) error {
					from := len(tr.observations())
					var order []string
					var readerPID uint32
					err := reads.Capture(ctx, f.eng, before, []uuid.UUID{s}, func(v app.OldQualifiedFacts) error {
						order = append(order, "D")
						if !reflect.DeepEqual(v, app.OldQualifiedFacts{PageID: s, State: app.OldFactsKnownIneligible}) {
							t.Fatalf("source D before callback mutation: %+v", v)
						}
						observed := tr.observations()[from:]
						if len(observed) != 1 || observed[0].Name != "IngestIndexedHeads" || !observed[0].Ended || observed[0].Err != nil {
							t.Fatalf("direct callback must follow closed heads and precede Bodies: %+v", observed)
						}
						readerPID = observed[0].PID
						tx, inTx := postgres.TxFrom(ctx)
						if inTx != tc.inTx || inTx && tx.Conn().PgConn().PID() != readerPID {
							t.Fatalf("capture executor: inTx=%v want=%v heads PID=%d", inTx, tc.inTx, readerPID)
						}
						if !change {
							return nil
						}
						if tc.locked {
							// The public callback can invalidate its own locked Tx's head.
							changeHead(ctx, tx)
							return nil
						}
						// This separate pool owns a different physical connection.
						// Commit before returning, so the next body statement sees it.
						maintainer, err := f.pool.Acquire(ctx)
						if err != nil {
							return err
						}
						defer maintainer.Release()
						if maintainer.Conn().PgConn().PID() == readerPID {
							t.Fatal("head maintainer reused Capture's physical connection")
						}
						write, err := maintainer.Begin(ctx)
						if err != nil {
							return err
						}
						defer func() {
							if err := write.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
								t.Errorf("roll back head maintainer: %v", err)
							}
						}()
						changeHead(ctx, write)
						return write.Commit(ctx)
					}, func(v app.OldSourceFacts) error {
						order = append(order, "F")
						if !reflect.DeepEqual(v, want) {
							t.Fatalf("public source F after change=%v: got %+v want %+v", change, v, want)
						}
						return nil
					})
					if err != nil {
						return err
					}
					if !slices.Equal(order, []string{"D", "F"}) {
						t.Fatalf("public capture callback order: %v", order)
					}
					var names []string
					for _, o := range tr.observations()[from:] {
						if !o.Ended || o.Err != nil || o.PID != readerPID {
							t.Fatalf("public capture query not completed on its reader: %+v", o)
						}
						names = append(names, o.Name)
					}
					if !slices.Equal(names, wantQueries) {
						t.Fatalf("public capture SQL after change=%v: got %v want %v", change, names, wantQueries)
					}
					return nil
				}
				// Nonempty F is a positive control on the same source and route.
				if err := capture(false, app.OldSourceFacts{PageID: s, State: app.OldFactsKnown, AssetIDs: []uuid.UUID{a}},
					[]string{"IngestIndexedHeads", "LintBodyReferencesFirst", "LintBodyReferencesAfter"}); err != nil {
					return err
				}
				// Cached head qualification stays Known; legacy Bodies filters A.
				return capture(true, app.OldSourceFacts{PageID: s, State: app.OldFactsKnown, AssetIDs: []uuid.UUID{}},
					[]string{"IngestIndexedHeads", "LintBodyReferencesFirst"})
			}
			var err error
			if tc.inTx {
				err = lintBodyTxManager(pool).WithinTx(t.Context(), run)
			} else {
				err = run(t.Context())
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIngestCandidates(t *testing.T) {
	for _, corruption := range []string{"target_empty", "asset_without_id"} {
		t.Run("invalid_peer/"+corruption, func(t *testing.T) {
			f := newFixture(t) // each corruption has its own isolated database
			p, s := uuid.NewV7(), uuid.NewV7()
			f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true, Properties: []domain.Property{{Key: "sources", Value: []byte(`"[[S]]"`)}}, Links: []domain.Link{{Kind: "wikilink", Property: "sources", Target: "S", Start: 5, End: 6}, {Kind: "wikilink", Property: "sources", Target: "S", Start: 10, End: 11}}})
			f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, domain.Facts{})
			f.resolve(t, p, domain.Resolution{ID: s}, 5, 10)
			q := app.IngestCandidateQuery{NotebookID: f.eng, Candidates: []app.IngestSource{{ID: s, Revision: 1}}, CurrentSources: []app.IngestRevision{{ID: s, Revision: 1}}, UntouchedSources: []uuid.UUID{s}, EligibleEmitters: []uuid.UUID{p}}
			read := func() (app.IngestCandidateEvidence, error) {
				var got app.IngestCandidateEvidence
				err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
					var err error
					got, err = postgresadapter.NewIngestReads(f.pool, 5*1024*1024).NovelSources(ctx, q)
					return err
				})
				return got, err
			}
			healthy, err := read()
			if err != nil || len(healthy.Candidates) != 1 || healthy.Candidates[0].ID != s || healthy.Candidates[0].Direct || healthy.Candidates[0].FollowedUntouched || len(healthy.Parents) != 0 {
				t.Fatalf("healthy real two-peer precondition: %+v, %v", healthy, err)
			}
			// Simulate an independently damaged historical index. Drop exactly the
			// blocking CHECK in this disposable test DB; production migrations and
			// their rejection tests remain unchanged. Keep the good peer at5 so the
			// relevant page is still reached even when peer10 loses its resolution.
			var constraint, update string
			if corruption == "target_empty" {
				constraint = "page_links_target_check"
				update = "target=''"
			} else {
				constraint = "page_links_resolved_asset_check"
				update = "resolved_id=NULL,resolved_asset=true"
			}
			lintExec(t, f, "ALTER TABLE page_links DROP CONSTRAINT "+constraint)
			lintExec(t, f, "UPDATE page_links SET "+update+" WHERE source_id=$1 AND range_start=10", p)
			rows := f.rows(t, "SELECT range_start,target,resolved_id::text,resolved_asset FROM page_links WHERE source_id=$1 ORDER BY range_start", p)
			if len(rows) != 2 || rows[0][0] != "5" || rows[0][1] != "S" || rows[0][2] != s.String() || rows[0][3] != "false" || rows[1][0] != "10" || corruption == "target_empty" && rows[1][1] != "" || corruption == "asset_without_id" && (rows[1][2] != "NULL" || rows[1][3] != "true") {
				t.Fatalf("exact corrupt peer with intact reverse hit: %v", rows)
			}
			got, err := read()
			if err == nil || !reflect.DeepEqual(got, app.IngestCandidateEvidence{}) {
				t.Fatalf("invalid peer succeeded: %+v, %v", got, err)
			}
		})
	}

	t.Run("parent_evidence_complete", func(t *testing.T) {
		f := newFixture(t)
		p, s, a, novel := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "[[S]]"))
		f.resolve(t, p, domain.Resolution{ID: s}, 2)
		f.replace(t, app.Page{ID: s, NotebookID: f.eng, Revision: 1}, originFacts(t, "[a](a.txt)"))
		f.resolve(t, s, domain.Resolution{ID: a, Asset: true}, 4)
		q := app.IngestCandidateQuery{NotebookID: f.eng, EligibleEmitters: []uuid.UUID{p}, CurrentSources: []app.IngestRevision{{ID: s, Revision: 1}}, UntouchedSources: []uuid.UUID{s}, ParentIDs: []uuid.UUID{s}}
		ids := ingestSort(a, novel)
		for _, id := range ids {
			q.Candidates = append(q.Candidates, app.IngestSource{ID: id, Asset: true})
		}
		var got app.IngestCandidateEvidence
		err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
			var err error
			got, err = postgresadapter.NewIngestReads(f.pool, 5*1024*1024).NovelSources(ctx, q)
			return err
		})
		if err != nil || len(got.Candidates) != 2 || !reflect.DeepEqual(got.Parents, []app.IngestParentFact{{ID: s, Direct: true}}) {
			t.Fatalf("complete parent/candidate evidence=%+v, %v", got, err)
		}
		for i, c := range got.Candidates {
			if c.ID != ids[i] || !c.Asset || c.Direct || c.FollowedUntouched != (c.ID == a) {
				t.Fatalf("wrong hand-written F/D oracle: %+v", got)
			}
		}
		boom := errors.New("transaction failed after complete evidence")
		if err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
			_, err := postgresadapter.NewIngestReads(f.pool, 5*1024*1024).NovelSources(ctx, q)
			if err != nil {
				return err
			}
			return boom
		}); !errors.Is(err, boom) {
			t.Fatal(err)
		}
	})
}
