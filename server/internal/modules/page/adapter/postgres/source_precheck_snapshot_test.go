package postgresadapter_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type sourceSnapshotObservation struct {
	stage, snapshot, isolation, readOnly string
	backend                              int
}

// All source calls still delegate to actual SQL. The hooks only commit a
// concurrent change or inject a read failure at a named, observed boundary.
type sourceSnapshotReads struct {
	app.SourceReads
	pool       *pgxpool.Pool
	observed   []sourceSnapshotObservation
	probes     int
	afterHeads func(context.Context) error
	afterProbe func(context.Context) error
}

func (s *sourceSnapshotReads) observe(ctx context.Context, stage string) error {
	o := sourceSnapshotObservation{stage: stage}
	err := postgres.DB(ctx, s.pool).QueryRow(ctx, `SELECT pg_backend_pid(), pg_current_snapshot()::text,
		current_setting('transaction_isolation'), current_setting('transaction_read_only')`).Scan(&o.backend, &o.snapshot, &o.isolation, &o.readOnly)
	if err == nil {
		s.observed = append(s.observed, o)
	}
	return err
}

func (s *sourceSnapshotReads) SourceHeads(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	if err := s.observe(ctx, "heads"); err != nil {
		return nil, err
	}
	got, err := s.SourceReads.SourceHeads(ctx, nb, ids)
	if err == nil && s.afterHeads != nil {
		err = s.afterHeads(ctx)
	}
	return got, err
}

func (s *sourceSnapshotReads) ProbeSources(ctx context.Context, nb uuid.UUID, probes []app.SourceProbe) ([]app.SourceMatch, error) {
	if err := s.observe(ctx, "probe"); err != nil {
		return nil, err
	}
	got, err := s.SourceReads.ProbeSources(ctx, nb, probes)
	s.probes++
	if err == nil && s.afterProbe != nil {
		err = s.afterProbe(ctx)
	}
	return got, err
}

type sourceSnapshotScene struct {
	f                                 fixture
	raw, excluded, incoming, external domain.Node
	pool                              *pgxpool.Pool
	tx                                *postgres.TxManager
	reads                             *sourceSnapshotReads
	precheck                          *app.SourcePrecheck
}

func newSourceSnapshotScene(t *testing.T) sourceSnapshotScene {
	t.Helper()
	f := newFixture(t)
	raw := f.page(t, f.eng, nil, "raw", 0)
	p := f.page(t, f.eng, &raw.ID, "Excluded", 0)
	q := f.page(t, f.eng, &raw.ID, "External", 1)
	b := f.page(t, f.eng, nil, "Incoming", 1)
	if p.ID.Compare(q.ID) >= 0 {
		t.Fatal("excluded candidate must precede external candidate")
	}
	f.exec(t, "UPDATE notebooks SET mode='llm_wiki', source_root_id=$2 WHERE id=$1", f.eng, raw.ID)
	for _, n := range []domain.Node{p, q, b} {
		f.exec(t, "UPDATE page_contents SET content='same', content_hash=$2, byte_size=4 WHERE node_id=$1", n.ID, sourceSum("same"))
	}
	f.exec(t, "UPDATE nodes SET source_hash=$3 WHERE id IN ($1,$2)", p.ID, q.ID, sourceSum("same"))
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx := postgres.NewTxManager(pool, time.Second)
	reads := &sourceSnapshotReads{SourceReads: postgresadapter.New(pool), pool: pool}
	return sourceSnapshotScene{f: f, raw: raw, excluded: p, incoming: b, external: q, pool: pool, tx: tx, reads: reads,
		precheck: app.NewSourcePrecheck(app.SourcePrecheckDeps{Snapshots: tx, Sources: reads})}
}

func (s sourceSnapshotScene) facts() iter.Seq[domain.StepFacts] {
	return func(yield func(domain.StepFacts) bool) {
		inside, outside := domain.NodePlace{InArea: true, ParentID: &s.raw.ID}, domain.NodePlace{}
		if !yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: s.excluded.ID, Kind: domain.KindPage, Before: &inside, After: &outside}}}) {
			return
		}
		yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: s.incoming.ID, Kind: domain.KindPage, Before: &outside, After: &inside}}})
	}
}

func (s sourceSnapshotScene) check(ctx context.Context) error {
	return s.precheck.Check(ctx, s.f.eng, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: s.raw.ID}, s.facts(), func(context.Context, shared.Action) error { return nil })
}

func (s sourceSnapshotScene) reusable(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("snapshot did not return its only connection: %v", err)
	}
	defer conn.Release()
	var one int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 || conn.Conn().PgConn().TxStatus() != 'I' {
		t.Errorf("connection after snapshot: one=%d err=%v txstatus=%c", one, err, conn.Conn().PgConn().TxStatus())
	}
}

// Permission decisions also read the database. Even a source-out-only plan
// has a management check inside the source snapshot, so it cannot acquire
// another connection from a pool whose only connection the snapshot holds.
func TestSourcePrecheckAuthorizesOnItsSnapshotConnection(t *testing.T) {
	for _, admission := range []bool{false, true} {
		t.Run(fmt.Sprintf("admission_%t", admission), func(t *testing.T) {
			s := newSourceSnapshotScene(t)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			facts := s.facts()
			if !admission {
				facts = func(yield func(domain.StepFacts) bool) {
					inside, outside := domain.NodePlace{InArea: true, ParentID: &s.raw.ID}, domain.NodePlace{}
					yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: s.excluded.ID, Kind: domain.KindPage, Before: &inside, After: &outside}}})
				}
			}
			var authorized sourceSnapshotObservation
			calls := 0
			err := s.precheck.Check(ctx, s.f.eng, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: s.raw.ID}, facts, func(ctx context.Context, action shared.Action) error {
				calls++
				if action != domain.ActionManageSource {
					return fmt.Errorf("unexpected source permission %s", action)
				}
				return postgres.DB(ctx, s.pool).QueryRow(ctx, `SELECT pg_backend_pid(), pg_current_snapshot()::text,
					current_setting('transaction_isolation'), current_setting('transaction_read_only') FROM notebooks WHERE id=$1`, s.f.eng).
					Scan(&authorized.backend, &authorized.snapshot, &authorized.isolation, &authorized.readOnly)
			})
			if admission {
				var problem *shared.Error
				if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" {
					t.Errorf("source authorization before duplicate = %v, want completed permission then duplicate", err)
				}
			} else if err != nil {
				t.Errorf("source-out permission could not use the held snapshot connection: %v", err)
			}
			if calls != 1 || authorized.backend == 0 || authorized.isolation != "repeatable read" || authorized.readOnly != "on" {
				t.Errorf("authorization calls=%d state=%+v, want one actual read on the source snapshot", calls, authorized)
			}
			if admission && len(s.reads.observed) > 0 && (authorized.backend != s.reads.observed[0].backend || authorized.snapshot != s.reads.observed[0].snapshot) {
				t.Errorf("source metadata and permission used different connections/snapshots: %+v vs %+v", s.reads.observed[0], authorized)
			}
			s.reusable(t)
		})
	}
}

func TestSourcePrecheckKeepsOneSnapshot(t *testing.T) {
	for _, onConn := range []bool{false, true} {
		t.Run(fmt.Sprintf("on_connection_%t", onConn), func(t *testing.T) {
			s := newSourceSnapshotScene(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var conn *pgxpool.Conn
			var pinnedBackend int
			if onConn {
				var err error
				conn, err = s.pool.Acquire(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if conn != nil {
						conn.Release()
					}
				}()
				if err := conn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pinnedBackend); err != nil {
					t.Fatal(err)
				}
				ctx = postgres.OnConn(ctx, conn)
			}
			changed := false
			s.reads.afterProbe = func(context.Context) error {
				if s.reads.probes != 1 {
					return nil
				}
				// This separate connection commits after the first candidate,
				// before the cursor continues to the external witness.
				err := postgres.NewTxManager(s.f.pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
					if _, err := postgres.DB(ctx, s.f.pool).Exec(ctx, "UPDATE page_contents SET content='changed', content_hash=$2, byte_size=7 WHERE node_id=$1", s.external.ID, sourceSum("changed")); err != nil {
						return err
					}
					_, err := postgres.DB(ctx, s.f.pool).Exec(ctx, "UPDATE nodes SET source_hash=$2 WHERE id=$1", s.external.ID, sourceSum("changed"))
					return err
				})
				changed = err == nil
				return err
			}
			err := s.check(ctx)
			var problem *shared.Error
			var refusal *domain.Refused
			if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" || problem.DuplicateOf == nil || *problem.DuplicateOf != s.external.ID ||
				!errors.As(err, &refusal) || refusal.NodeID != s.incoming.ID {
				t.Errorf("snapshot refusal=%v, want incoming denied by original external source", err)
			}
			if !changed || !bytes.Equal(s.f.sourceHash(t, s.external.ID), sourceSum("changed")) {
				t.Error("concurrent external rewrite did not actually commit")
			}
			if len(s.reads.observed) != 3 || s.reads.probes != 2 {
				t.Fatalf("observed=%+v probes=%d, want heads and two real cursor queries", s.reads.observed, s.reads.probes)
			}
			first := s.reads.observed[0]
			for _, o := range s.reads.observed {
				if o.backend != first.backend || o.snapshot != first.snapshot || o.isolation != "repeatable read" || o.readOnly != "on" || onConn && o.backend != pinnedBackend {
					t.Errorf("source query escaped its single read-only snapshot: first=%+v current=%+v pinned=%d", first, o, pinnedBackend)
				}
			}
			if conn != nil {
				conn.Release()
				conn = nil
			}
			s.reusable(t)
		})
	}
	for _, failure := range []string{"read-error", "cancel", "write", "nested", "panic"} {
		t.Run(failure, func(t *testing.T) {
			s := newSourceSnapshotScene(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			sentinel := errors.New("source read failed")
			s.reads.afterHeads = func(ctx context.Context) error {
				switch failure {
				case "read-error":
					return sentinel
				case "cancel":
					cancel()
					return ctx.Err()
				case "write":
					_, err := postgres.DB(ctx, s.pool).Exec(ctx, "UPDATE nodes SET source_hash=NULL WHERE id=$1", s.external.ID)
					return err
				case "panic":
					panic(sentinel)
				}
				return nil
			}
			var err error
			switch failure {
			case "nested":
				err = s.tx.WithinTx(ctx, func(ctx context.Context) error { return s.check(ctx) })
			case "panic":
				var recovered any
				func() { defer func() { recovered = recover() }(); err = s.check(ctx) }()
				if recoveredError, ok := recovered.(error); !ok || !errors.Is(recoveredError, sentinel) {
					t.Errorf("snapshot panic=%v, want source sentinel", recovered)
				}
			default:
				err = s.check(ctx)
			}
			switch failure {
			case "read-error":
				if !errors.Is(err, sentinel) {
					t.Errorf("source read error=%v", err)
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) {
					t.Errorf("canceled source read=%v", err)
				}
			case "nested":
				if !errors.Is(err, postgres.ErrNestedSnapshot) || len(s.reads.observed) != 0 {
					t.Errorf("nested source check=%v reads=%v", err, s.reads.observed)
				}
			case "write":
				var pe *pgconn.PgError
				if !errors.As(err, &pe) || pe.Code != "25006" {
					t.Errorf("write in source snapshot=%v, want read-only SQL error", err)
				}
			}
			if s.reads.probes != 0 {
				t.Errorf("failed metadata continued to %d probe calls", s.reads.probes)
			}
			if !bytes.Equal(s.f.sourceHash(t, s.external.ID), sourceSum("same")) {
				t.Error("failed/read-only source check changed source data")
			}
			s.reusable(t)
		})
	}
}
