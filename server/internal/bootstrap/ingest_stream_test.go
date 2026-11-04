package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// These are synthetic read-boundary failures on real SQL and real rows in
// the original write transaction. C10 independently covers real PG failures.
type ingestStreamState struct {
	missingID                              uuid.UUID
	missingReads                           int
	oldSlots                               bool
	oldF                                   *ingestOldFTailState
	sourceBytes                            *ingestSourceBytesState
	oldCalls, oldRows, replaces            int
	armed                                  atomic.Bool
	done                                   chan struct{}
	parent                                 bool
	mode                                   string
	cause                                  error
	calls, adds, candidates, parents, rows int
	injected                               bool
	callbackErr, managerErr                error
}
type ingestStreamManager struct {
	page.Tx
	state *ingestStreamState
}

func (m ingestStreamManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	s := m.state
	if !s.armed.CompareAndSwap(true, false) {
		return m.Tx.WithinTx(ctx, fn)
	}
	err := m.Tx.WithinTx(ctx, func(inner context.Context) error {
		err := fn(ingestStreamContext{Context: inner, state: s})
		s.callbackErr = err
		return err
	})
	s.managerErr = err
	close(s.done)
	return err
}

// Delegate every key. Only the transaction value already supplied by the
// original manager is forwarded through a read observer. The manager keeps
// its original transaction and performs the real commit/rollback itself.
type ingestStreamContext struct {
	context.Context
	state *ingestStreamState
}

func (c ingestStreamContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return ingestStreamTx{Tx: tx, state: c.state}
	}
	return v
}

type ingestStreamTx struct {
	pgx.Tx
	state *ingestStreamState
}

func (tx ingestStreamTx) observe(sql string) {
	if strings.HasPrefix(sql, "-- name: DeleteLinksOf ") || strings.HasPrefix(sql, "-- name: InsertIndexedPage ") {
		tx.state.replaces++
	}
	if strings.HasPrefix(sql, "-- name: AddIngestMarks ") {
		tx.state.adds++
	}
}
func (tx ingestStreamTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.observe(sql)
	return tx.Tx.Exec(ctx, sql, args...)
}
func (tx ingestStreamTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.observe(sql)
	if strings.HasPrefix(sql, "-- name: FindNodeIn ") && len(args) > 0 && args[0] == tx.state.missingID {
		tx.state.missingReads++
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx ingestStreamTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.observe(sql)
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err == nil && tx.state.oldF != nil {
		rows, err = tx.state.oldF.wrap(rows, sql, args, tx.state.replaces)
		if err != nil {
			return nil, err
		}
	}
	if err == nil && tx.state.sourceBytes != nil {
		rows = tx.state.sourceBytes.wrap(rows, sql, args, tx.state.replaces)
	}
	if err == nil && tx.state.oldSlots && strings.HasPrefix(sql, "-- name: LintSourceSlotEdges ") {
		tx.state.oldCalls++
		return &ingestOldSlotRows{Rows: rows, state: tx.state}, nil
	}
	if err != nil || !strings.HasPrefix(sql, "-- name: IngestNovelSources ") {
		return rows, err
	}
	tx.state.calls++
	return &ingestStreamRows{Rows: rows, state: tx.state}, nil
}

type ingestStreamRows struct {
	pgx.Rows
	state           *ingestStreamState
	targetSeen, eof bool
}

func (r *ingestStreamRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	s := r.state
	s.rows++
	if len(dest) != 7 {
		return fmt.Errorf("stream fixture: real result columns=%d", len(dest))
	}
	kind, ok := dest[0].(*string)
	if !ok {
		return fmt.Errorf("stream fixture: row kind type %T", dest[0])
	}
	switch *kind {
	case "candidate":
		s.candidates++
	case "parent":
		s.parents++
	default:
		return fmt.Errorf("stream fixture: real row kind %q", *kind)
	}
	target := !s.parent && *kind == "candidate" && s.candidates == 2 || s.parent && *kind == "parent" && s.parents == 1
	if !target {
		return nil
	}
	r.targetSeen = true
	if s.mode == "healthy" || s.mode == "rows_err" {
		return nil
	}
	s.injected = true
	switch s.mode {
	case "scan":
		return s.cause
	case "dto":
		// A fully decoded row cannot silently disappear from the exact proof.
		*kind = "unexpected-row-kind"
	case "bytes":
		id, ok := dest[1].(*uuid.UUID)
		if !ok {
			return fmt.Errorf("stream fixture: UUID type %T", dest[1])
		}
		*id = uuid.Nil()
	case "flag":
		flag, ok := dest[6].(**bool)
		if !ok {
			return fmt.Errorf("stream fixture: flag type %T", dest[6])
		}
		value := true
		*flag = &value
	default:
		return fmt.Errorf("stream fixture: mode %s", s.mode)
	}
	return nil
}
func (r *ingestStreamRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.eof = true
	}
	return next
}
func (r *ingestStreamRows) Err() error {
	if err := r.Rows.Err(); err != nil {
		return err
	}
	if r.eof && r.targetSeen && r.state.mode == "rows_err" {
		r.state.injected = true
		return r.state.cause
	}
	return nil
}

func testIngestStreamFailure(t *testing.T) {
	for _, parent := range []bool{false, true} {
		role := "candidate"
		if parent {
			role = "last_parent"
		}
		t.Run(role, func(t *testing.T) {
			for _, mode := range []string{"healthy", "scan", "rows_err", "bytes", "dto", "flag"} {
				t.Run(mode, func(t *testing.T) {
					state := &ingestStreamState{done: make(chan struct{}), parent: parent, mode: mode, cause: errors.New("designated ingestion stream failure")}
					f := newIngestFixture(t, &wiring{page: func(d *page.Deps) { d.Tx = ingestStreamManager{Tx: d.Tx, state: state} }})
					var ops []map[string]any
					want := []string{f.s, f.t}
					if parent {
						asset := f.tm.upload(t, "alice", f.nb, f.raw, "old.txt", "asset A")
						source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "[A](old.txt)")
						f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Follow]]")
						// Rename forces a real system rewrite of S. Its old F is retained
						// until the final parent row proves untouched Q still digests S.
						ops = []map[string]any{opMove("raw/old.txt", "raw/new.txt"), opWriteOn("P.md", "[A](raw/new.txt) [[raw/T]]", versionOf(f.p, 1))}
						f.tm.resolves(t, source, asset.ID)
						want = []string{f.t}
					} else {
						ops = []map[string]any{opWriteOn("P.md", "[[raw/S]] [[raw/T]]", versionOf(f.p, 1))}
					}
					m := f.tm.agent(t, "alice", f.nb)
					m.handshakeAs(t, "ingest-stream")
					before := ingestStreamImage(t, f)
					state.armed.Store(true)
					answer := m.batch(t, ops...)
					select {
					case <-state.done:
					case <-time.After(interleavingWait):
						t.Fatal("stream fixture transaction did not finish")
					}
					if state.calls != 1 || state.candidates != 2 || state.parents != map[bool]int{false: 0, true: 1}[parent] {
						t.Fatalf("stream fixture lacks complete actual candidate/parent rows: calls=%d candidates=%d parents=%d", state.calls, state.candidates, state.parents)
					}
					if mode == "healthy" {
						result := answerOf[mcpBatched](t, answer)
						if state.injected || state.callbackErr != nil || state.managerErr != nil || state.adds != 1 || !result.Changed || result.ChangesetID == nil {
							t.Fatalf("stream healthy actual unit: state=%+v result=%+v", state, result)
						}
						assertIngestMarks(t, f.tm, *result.ChangesetID, want...)
					} else {
						if !state.injected {
							t.Fatal("designated stream failure did not execute")
						}
						if state.adds != 0 || state.callbackErr == nil || state.managerErr == nil {
							t.Fatalf("stream failure was swallowed: %+v", state)
						}
						if mode == "scan" || mode == "rows_err" {
							if !errors.Is(state.callbackErr, state.cause) || !errors.Is(state.managerErr, state.cause) {
								t.Fatalf("original stream cause lost: callback=%v manager=%v", state.callbackErr, state.managerErr)
							}
						} else if !strings.Contains(state.callbackErr.Error(), "malformed or incomplete candidate/parent") {
							t.Fatalf("decoded malformed row did not reach adapter validation: %v", state.callbackErr)
						}
						if after := ingestStreamImage(t, f); after != before {
							t.Fatalf("stream failure did not roll back real content/index/cs/marks: before=%s after=%s", before, after)
						}
						if answer.status != http.StatusOK {
							t.Fatalf("MCP transport status=%d", answer.status)
						}
					}
					t.Logf("stream role=%s mode=%s actual_query=%d candidate_rows=%d parent_rows=%d injected=%v Add=%d callback=%v", role, mode, state.calls, state.candidates, state.parents, state.injected, state.adds, state.callbackErr)
					checkPages(t, f.tm.pool)
					checkLinks(t, f.tm.pool)
				})
			}
		})
	}
}

func ingestStreamImage(t *testing.T, f ingestFixture) string {
	t.Helper()
	rows := queryStrings(t, f.tm.pool, "SELECT coalesce(jsonb_agg(to_jsonb(n) ORDER BY id),'[]'::jsonb)::text FROM nodes n WHERE notebook_id=$1", f.nb)
	if len(rows) != 1 {
		t.Fatalf("stream node image: %v", rows)
	}
	return rows[0] + c10DraftImage(t, f.tm.pool, f.nb)
}

// B12 (decision79): a prefix followed by an actual consumption failure is
// neither complete PossibleTargets nor a successful Unknown representation.
type ingestOldSlotRows struct {
	pgx.Rows
	state     *ingestStreamState
	tail, eof bool
}

func (r *ingestOldSlotRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	r.state.oldRows++
	if r.state.oldRows != 513 {
		return nil
	}
	r.tail = true
	if r.state.mode == "scan" {
		r.state.injected = true
		return r.state.cause
	}
	return nil
}
func (r *ingestOldSlotRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.eof = true
	}
	return next
}
func (r *ingestOldSlotRows) Err() error {
	if err := r.Rows.Err(); err != nil {
		return err
	}
	if r.tail && r.eof && r.state.mode == "rows_err" {
		r.state.injected = true
		return r.state.cause
	}
	return nil
}
func testIngestOldSlotsFailure(t *testing.T) {
	for _, mode := range []string{"healthy", "scan", "rows_err"} {
		t.Run(mode, func(t *testing.T) {
			state := &ingestStreamState{oldSlots: true, mode: mode, done: make(chan struct{}), cause: errors.New("designated old-slot tail failure")}
			f := newIngestFixture(t, &wiring{page: func(d *page.Deps) { d.Tx = ingestStreamManager{Tx: d.Tx, state: state} }})
			body := "---\nsources:\n" + strings.Repeat("  - '[[raw/S]]'\n", 513) + "---\n"
			f.write(t, f.p, body)
			if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key IS NOT NULL AND resolved_id=$2", f.p, f.s); n != 513 {
				t.Fatalf("old slots parser/resolver has%d peers want513", n)
			}
			m := f.tm.agent(t, "alice", f.nb)
			m.handshakeAs(t, "ingest-old-slots")
			before := ingestStreamImage(t, f)
			state.armed.Store(true)
			answer := m.batch(t, opWriteOn("P.md", "[[raw/S]] [[raw/T]]", versionOf(f.p, 2)))
			select {
			case <-state.done:
			case <-time.After(interleavingWait):
				t.Fatal("old-slot transaction did not finish")
			}
			if state.oldCalls != 2 || state.oldRows != 513 {
				t.Fatalf("old slots did not consume a full512 prefix then tail: queries%d rows%d", state.oldCalls, state.oldRows)
			}
			if mode == "healthy" {
				result := answerOf[mcpBatched](t, answer)
				if state.injected || state.callbackErr != nil || state.managerErr != nil || state.calls != 1 || state.adds != 1 || state.replaces == 0 || result.ChangesetID == nil {
					t.Fatalf("old-slot healthy state%+v result%+v", state, result)
				}
				assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
			} else {
				if !state.injected || !errors.Is(state.callbackErr, state.cause) || !errors.Is(state.managerErr, state.cause) || state.calls != 0 || state.replaces != 0 || state.adds != 0 {
					t.Fatalf("old prefix failure became success or allowed followup: %+v", state)
				}
				if after := ingestStreamImage(t, f); after != before {
					t.Fatalf("old-slot partial read did not roll back the real unit: before=%s after=%s", before, after)
				}
			}
			t.Logf("B12 mode%s old_chunks%d old_rows%d injected%t Replace%d Novel%d Add%d callback%v", mode, state.oldCalls, state.oldRows, state.injected, state.replaces, state.calls, state.adds, state.callbackErr)
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}
