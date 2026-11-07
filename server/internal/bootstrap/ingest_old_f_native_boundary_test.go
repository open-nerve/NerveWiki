package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type ingestNativeOldFSource struct {
	id, tail uuid.UUID
	rows     int
}

// One concrete two-source fixture. The shared stream/C10 wrappers still own
// the original Writer transaction and its mutation, cause and end observations.
type ingestNativeOldF struct {
	prepared      *ingestPreparedBoundary
	life          *c10DraftState
	stream        *ingestStreamState
	mode          string
	cause         error
	notebook      uuid.UUID
	sources       []ingestNativeOldFSource
	ids           []uuid.UUID
	want, seen    []linkingapp.BodyReference
	batches       []*ingestOldFTailBatch
	first         chan [2]ingestOldFPurgeImage
	resume        chan struct{}
	tx            pgx.Tx
	calls         int
	injected      bool
	before, after [2]ingestOldFPurgeImage
}

type ingestNativeOldFContext struct {
	context.Context
	s *ingestNativeOldF
}

func (c ingestNativeOldFContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return ingestNativeOldFTx{Tx: tx, s: c.s}
	}
	return v
}

type ingestNativeOldFTx struct {
	pgx.Tx
	s *ingestNativeOldF
}

func (tx ingestNativeOldFTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	name := stableIndexSQLName(sql)
	if err != nil || name != "IngestOldBodyReferencesFirst" && name != "IngestOldBodyReferencesAfter" {
		return rows, err
	}
	b := &ingestOldFTailBatch{after: name == "IngestOldBodyReferencesAfter"}
	tx.s.batches = append(tx.s.batches, b)
	return &ingestNativeOldFRows{Rows: rows, s: tx.s, b: b}, nil
}

type ingestNativeOldFRows struct {
	pgx.Rows
	s *ingestNativeOldF
	b *ingestOldFTailBatch
}

func (r *ingestNativeOldFRows) Close() {
	r.Rows.Close()
	r.b.closed = true
}
func (r *ingestNativeOldFRows) Next() bool {
	ok := r.Rows.Next()
	if !ok {
		r.b.eof = true
	}
	return ok
}
func (r *ingestNativeOldFRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	if len(dest) != 4 {
		return fmt.Errorf("native old-F fixture: columns=%d, want4", len(dest))
	}
	source, a := dest[0].(*uuid.UUID)
	start, b := dest[1].(*int32)
	target, c := dest[2].(**uuid.UUID)
	asset, d := dest[3].(*bool)
	if !a || !b || !c || !d || *target == nil {
		return errors.New("native old-F fixture: unexpected Scan destination/value")
	}
	row := linkingapp.BodyReference{EdgePos: linkingapp.EdgePos{SourceID: *source, Start: *start}, ResolvedID: **target, ResolvedAsset: *asset}
	i := len(r.s.seen)
	if i >= len(r.s.want) || row != r.s.want[i] {
		return fmt.Errorf("native old-F fixture: literal row%d differs: %+v", i, row)
	}
	r.s.seen = append(r.s.seen, row)
	r.b.rows++
	if i == 16384 && (!r.b.after || r.b.rows != 1) {
		return errors.New("native old-F fixture: independent high-source tail is not in After")
	}
	if i == 16384 && r.s.mode == "scan" {
		r.s.injected, r.s.life.coverageErr = true, r.s.cause
		return r.s.cause // controlled boundary failure after the real native Scan
	}
	return nil
}
func (r *ingestNativeOldFRows) Err() error {
	r.b.errRead = true
	if err := r.Rows.Err(); err != nil {
		return err
	}
	if r.s.mode == "rows_err" && r.b.after && r.b.rows == 1 && r.b.eof && len(r.s.seen) == 16385 {
		r.s.injected, r.s.life.coverageErr = true, r.s.cause
		return r.s.cause // controlled terminal failure after real native EOF/Err
	}
	return nil
}

type ingestNativeOldFBodies struct {
	linkingapp.IngestOldBodyReads
	s *ingestNativeOldF
}

func (r ingestNativeOldFBodies) OldBodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *linkingapp.EdgePos, take int) ([]linkingapp.BodyReference, error) {
	s := r.s
	if !c10DraftActive(ctx, s.life) {
		return r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok || tx.Conn().PgConn().PID() != s.life.pid || notebook != s.notebook || take != 16384 || !slices.Equal(emitters, s.ids) {
		return nil, errors.New("native old-F fixture: actual Tx/notebook/complete source relation differs")
	}
	s.calls++
	if s.calls == 1 {
		s.tx = tx
		if after != nil {
			return nil, errors.New("native old-F fixture: First has a cursor")
		}
	} else if s.calls > 3 || tx != s.tx || after == nil || after.Compare(s.want[16382+s.calls-1].EdgePos) != 0 {
		return nil, errors.New("native old-F fixture: After changed Tx/cursor or queried past EOF")
	}
	rows, err := r.IngestOldBodyReads.OldBodyReferences(ingestNativeOldFContext{Context: ctx, s: s}, notebook, emitters, after, take)
	if err != nil || after != nil {
		return rows, err
	}
	if len(rows) != 16384 || !slices.Equal(rows, s.want[:16384]) || len(s.batches) != 1 ||
		s.batches[0].after || s.batches[0].rows != 16384 || !s.batches[0].eof || !s.batches[0].errRead || !s.batches[0].closed || s.stream.replaces != 0 {
		return nil, errors.New("native old-F fixture: First16384 did not finish EOF/Err/Close before Replace")
	}
	if err := s.pause(ctx, tx); err != nil {
		return nil, err
	}
	return rows, nil
}

// Both routes pause only after their first actual native EOF/Err/Close.
func (s *ingestNativeOldF) pause(ctx context.Context, tx pgx.Tx) error {
	var err error
	s.before, err = s.image(ctx, tx)
	if err != nil {
		return err
	}
	s.life.xid = s.before[0].xid
	s.first <- s.before
	select {
	case <-s.resume:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.after, err = s.image(ctx, tx)
	return err
}
func (s *ingestNativeOldF) pauseRows() int {
	if s.prepared != nil {
		return s.sources[0].rows
	}
	return 16384
}
func (s *ingestNativeOldF) nativeRead(name string) bool {
	if s.prepared != nil {
		return name == "InspectPageLinksForPreparation"
	}
	return name == "IngestOldBodyReferencesFirst" || name == "IngestOldBodyReferencesAfter"
}

func (s *ingestNativeOldF) image(ctx context.Context, tx pgx.Tx) ([2]ingestOldFPurgeImage, error) {
	var out [2]ingestOldFPurgeImage
	space, key := ingestIndexKey(s.notebook)
	for i, source := range s.sources {
		v := &out[i]
		err := tx.QueryRow(ctx, `SELECT n.deleted_at IS NOT NULL,c.deleted_at IS NOT NULL,
			c.revision,i.revision,i.extractor,(SELECT count(*) FROM page_links l WHERE l.source_id=n.id),
			pg_current_xact_id()::text,EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid()
			AND locktype='advisory' AND granted AND classid::bigint=$2 AND objid::bigint=$3 AND objsubid=2)
			FROM nodes n JOIN page_contents c ON c.node_id=n.id JOIN indexed_pages i ON i.node_id=n.id AND i.notebook_id=n.notebook_id
			WHERE n.id=$1`, source.id, int64(uint32(space)), int64(uint32(key))).
			Scan(&v.deleted, &v.contentDeleted, &v.revision, &v.headRevision, &v.extractor, &v.edges, &v.xid, &v.indexHeld)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func testIngestOldFNativeBoundary(t *testing.T) { testIngestOldFBoundary(t, false) }

func testIngestOldFBoundary(t *testing.T, prepared bool) {
	for _, mode := range []string{"healthy", "scan", "rows_err"} {
		t.Run(mode, func(t *testing.T) {
			life := &c10DraftState{done: make(chan struct{}), leg: mode}
			stream := &ingestStreamState{done: make(chan struct{}), mode: "healthy"}
			s := &ingestNativeOldF{life: life, stream: stream, mode: mode, cause: errors.New("designated native After tail boundary"),
				first: make(chan [2]ingestOldFPurgeImage, 1), resume: make(chan struct{})}
			if prepared {
				s.prepared = &ingestPreparedBoundary{s: s, inputs: map[uuid.UUID][]linkingapp.Link{}}
			}
			trace := &ingestOldFPurgeTracer{}
			wrapped := 0
			f := newIngestFixture(t, &wiring{queryTracer: trace, page: func(d *page.Deps) {
				life.pool = d.Pool
				d.Tx = ingestStreamManager{Tx: c10DraftTx{Tx: d.Tx, s: life}, state: stream}
				for i, observer := range d.Observers {
					if bridge, ok := observer.(linkIndex); ok {
						if bridge.index.Ingest == nil || bridge.index.Ingest.OldBodies == nil {
							t.Fatal("native old-F fixture: missing real NewIndex private reader")
						}
						deps := *bridge.index.Ingest
						deps.OldBodies = ingestNativeOldFBodies{deps.OldBodies, s}
						deps.Marks = c10DraftMarks{IngestMarks: deps.Marks, s: life}
						bridge.index.Ingest = &deps
						bridge.index.Publisher = c10DraftPublisher{Publisher: bridge.index.Publisher, s: life}
						if prepared {
							if bridge.index.Preparations == nil {
								t.Fatal("prepared boundary: real NewIndex capability absent")
							}
							bridge.index.Preparations = ingestPreparedBoundaryFactory{SourcePreparations: bridge.index.Preparations, p: s.prepared}
							bridge.index.Store = ingestPreparedBoundaryStore{Store: bridge.index.Store, p: s.prepared}
						} else {
							bridge.index.Preparations = nil // Keep original First/After fallback coverage explicit.
						}
						d.Observers[i] = bridge
						wrapped++
					}
				}
			}})
			if wrapped != 1 {
				t.Fatalf("native old-F fixture: real producer observers=%d", wrapped)
			}
			s.notebook = uuid.MustParse(f.nb)
			awaitJob(t, f.tm.pool, jobs.PurgeKind)
			trashBook := f.tm.openNotebook(t, "alice", "Native boundary expired trash")
			trash := f.tm.createPageWith(t, "alice", trashBook, "", "Expired", "independent expired content")
			f.tm.send(t, nodeDeletion("alice", trash), http.StatusNoContent)
			f.tm.trashedAgo(t, trash, 61*24*time.Hour)
			if count(t, f.tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
				WHERE n.id=$1 AND n.deleted_at<now()-interval '60 days' AND c.deleted_at<now()-interval '60 days'`, trash) != 1 {
				t.Fatal("native old-F fixture: unrelated expired-trash positive control is absent")
			}
			a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "common asset")
			one := f.tm.upload(t, "alice", f.nb, f.raw, "tail-one.txt", "independent first tail")
			two := f.tm.upload(t, "alice", f.nb, f.raw, "tail-two.txt", "independent second tail")
			for _, spec := range []struct {
				name, tail, target string
				rows               int
			}{{"NativeLong", "tail-one.txt", one.ID, 10000}, {"NativeShort", "tail-two.txt", two.ID, 6385}} {
				body := "Complete old F.\n" + strings.Repeat("[](a.txt)\n", spec.rows-1) + "[](" + spec.tail + ")\n"
				id := f.tm.createPageWith(t, "alice", f.nb, f.raw, spec.name, body)
				q := f.tm.createPageWith(t, "alice", f.nb, "", "Q"+spec.name, "[[raw/"+spec.name+"]]")
				if prepared {
					s.prepared.addQ(q, id, "raw/"+spec.name)
				}
				s.sources = append(s.sources, ingestNativeOldFSource{id: uuid.MustParse(id), tail: uuid.MustParse(spec.target), rows: spec.rows})
				want := make([]string, spec.rows)
				for i := range want {
					want[i] = a.ID
				}
				want[len(want)-1] = spec.target
				f.tm.resolves(t, id, want...)
			}
			// UUID order is observed, never inferred from POST order. The fixed
			// header is 16 bytes; every preceding [](a.txt) line is 10 bytes.
			slices.SortFunc(s.sources, func(a, b ingestNativeOldFSource) int { return a.id.Compare(b.id) })
			for _, source := range s.sources {
				s.ids = append(s.ids, source.id)
				for j := range source.rows {
					target := uuid.MustParse(a.ID)
					if j == source.rows-1 {
						target = source.tail
					}
					s.want = append(s.want, linkingapp.BodyReference{EdgePos: linkingapp.EdgePos{SourceID: source.id, Start: int32(16 + 10*j + 3)}, ResolvedID: target, ResolvedAsset: true})
				}
			}
			if len(s.want) != 16385 || s.want[16384].SourceID != s.sources[1].id || s.want[16384].ResolvedID != s.sources[1].tail {
				t.Fatal("native old-F literal fixture lacks the independent high-source tail")
			}
			if prepared {
				s.prepared.seed(t, f, one.ID, two.ID)
			}
			m := f.tm.agent(t, "alice", f.nb)
			m.handshakeAs(t, "ingest-native-old-f")
			const body = "[one](raw/tail-one.txt) [two](raw/tail-two.txt) [[raw/T]]"
			before := ingestNativeOldFImage(t, f)
			resume := sync.OnceFunc(func() { close(s.resume) })
			done := make(chan struct{})
			var answer mcpAnswer
			t.Cleanup(func() {
				resume()
				select {
				case <-done:
				case <-time.After(interleavingWait):
					t.Error("native old-F Writer did not end during cleanup")
				}
			})
			stream.armed.Store(true)
			life.armed.Store(true)
			go func() {
				defer close(done)
				answer = m.batch(t, opMove("raw/a.txt", "raw/b.txt"), opWriteOn("P.md", body, versionOf(f.p, 1)))
			}()
			var paused [2]ingestOldFPurgeImage
			select {
			case paused = <-s.first:
			case <-done:
				t.Fatalf("NATIVE_OLD_F_GATE: Writer ended before the first complete closed native read: %d %s", answer.status, answer.body)
			case <-time.After(interleavingWait):
				t.Fatal("NATIVE_OLD_F_GATE: deadline is not a semantic success")
			}
			for i, v := range paused {
				if v.deleted || v.contentDeleted || v.revision != 2 || v.headRevision != 1 || v.extractor != 2 || v.edges != s.sources[i].rows || !v.indexHeld || v.xid == "" || v.xid != paused[0].xid || life.pid == 0 {
					t.Fatalf("NATIVE_OLD_F_SOURCES: both real rewrites/old heads must survive: %+v", paused)
				}
			}
			assertPaused := func() {
				t.Helper()
				if s.calls != 1 || len(s.batches) != 1 || !s.batches[0].closed || stream.replaces != 0 || !slices.Equal(s.seen, s.want[:s.pauseRows()]) {
					t.Fatal("NATIVE_OLD_F_GATE: source stream or Replace advanced during purge")
				}
				if prepared {
					s.prepared.checkPaused(t)
				}
				life.mu.Lock()
				events := slices.Clone(life.events)
				life.mu.Unlock()
				firstEnds, nativeStarts := 0, 0
				for _, e := range events {
					if e.Kind == "sql.start" && s.nativeRead(stableIndexSQLName(e.SQL)) {
						nativeStarts++
					}
					if e.Kind == "sql.end" && s.nativeRead(stableIndexSQLName(e.SQL)) && e.PID == life.pid && e.Err == nil {
						firstEnds++
					}
				}
				if nativeStarts != 1 || firstEnds != 1 || ingestNativeOldFImage(t, f) != before {
					t.Fatal("NATIVE_OLD_F_GATE: native First end or notebook isolation differs")
				}
			}
			assertPaused()
			trace.enabled.Store(true)
			ingestOldFPurgeRunRiver(t, f, life.pid, done)
			trace.enabled.Store(false)
			assertPaused()
			checkTrash := func() {
				t.Helper()
				for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items"} {
					column := "node_id"
					if table == "nodes" {
						column = "id"
					}
					if count(t, f.tm.pool, "SELECT count(*) FROM "+table+" WHERE "+column+"=$1", trash) != 0 {
						t.Fatalf("NATIVE_OLD_F_PURGE: committed unrelated %s deletion was lost", table)
					}
				}
			}
			checkTrash()
			trace.mu.Lock()
			purged := slices.Clone(trace.queries)
			trace.mu.Unlock()
			purgeRows := map[string]int64{}
			for _, p := range purged {
				if p.pid == 0 || p.pid == life.pid || p.err != nil {
					t.Fatalf("NATIVE_OLD_F_PURGE: independent actual purger failed: %+v", p)
				}
				purgeRows[p.name] += p.rows
			}
			if len(purgeRows) != 7 || purgeRows["PurgePageContents"] != 1 || purgeRows["PurgeNodeLeaves"] != 1 {
				t.Fatalf("NATIVE_OLD_F_PURGE: missing nonempty real purge work: %v", purgeRows)
			}
			resume()
			select {
			case <-done:
			case <-time.After(interleavingWait):
				t.Fatal("NATIVE_OLD_F_END: resumed Writer did not finish")
			}
			if s.after != paused || !slices.Equal(s.seen, s.want) || life.fixtureErr != nil {
				t.Fatalf("NATIVE_OLD_F_FULL: protected image or all16385 literal native rows differ: after=%+v rows=%d fixture=%v", s.after, len(s.seen), life.fixtureErr)
			}
			if prepared {
				s.prepared.check(t, f)
			} else {
				wantBatches := []int{16384, 1}
				if mode == "healthy" {
					wantBatches = append(wantBatches, 0)
				}
				if s.calls != len(wantBatches) || len(s.batches) != len(wantBatches) {
					t.Fatalf("NATIVE_OLD_F_BATCHES: calls=%d batches=%+v", s.calls, s.batches)
				}
				for i, n := range wantBatches {
					b := s.batches[i]
					ended := mode != "scan" || i == 0
					if b.rows != n || b.after != (i > 0) || !b.closed || b.eof != ended || b.errRead != ended {
						t.Fatalf("NATIVE_OLD_F_BATCHES: native boundary%d=%+v", i, b)
					}
				}
			}
			if mode == "healthy" {
				result := answerOf[mcpBatched](t, answer)
				if s.injected || life.callbackErr != nil || life.managerErr != nil || stream.callbackErr != nil || stream.managerErr != nil || stream.replaces == 0 || stream.calls != 1 || stream.candidates != 3 || stream.parents != 2 || stream.adds != 1 || !result.Changed || result.ChangesetID == nil || result.ResultsCount != 2 {
					t.Fatalf("NATIVE_OLD_F_HEALTHY: original Writer did not commit: life=%+v stream=%+v result=%+v", life, stream, result)
				}
				assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
				for _, source := range s.sources {
					if c := f.tm.content(t, "alice", source.id.String()); c.Revision != 2 || !strings.Contains(c.Content, "[](b.txt)") || strings.Contains(c.Content, "[](a.txt)") {
						t.Fatalf("NATIVE_OLD_F_REWRITE: source was not actually rewritten: %+v", c)
					}
					if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND node_id=$2 AND revision=2 AND rewrite", *result.ChangesetID, source.id) != 1 {
						t.Fatal("NATIVE_OLD_F_REWRITE: committed source history missing")
					}
				}
				f.tm.resolves(t, f.p, one.ID, two.ID, f.t)
			} else {
				if !s.injected || !errors.Is(life.callbackErr, s.cause) || !errors.Is(life.managerErr, s.cause) || !errors.Is(stream.callbackErr, s.cause) || !errors.Is(stream.managerErr, s.cause) {
					t.Fatalf("NATIVE_OLD_F_CAUSE: original After cause lost: injected=%t callback=%v manager=%v", s.injected, life.callbackErr, life.managerErr)
				}
				if stream.replaces != 0 || stream.calls != 0 || stream.adds != 0 || ingestNativeOldFImage(t, f) != before {
					t.Fatal("NATIVE_OLD_F_ROLLBACK: Writer leaked notebook content/index/sidefacts/history/marks")
				}
				if answer.status != http.StatusOK || answer.result(t)["isError"] != true {
					t.Fatalf("NATIVE_OLD_F_REPLY: failure response=%d %s", answer.status, answer.body)
				}
			}
			ingestNativeOldFCheckEnd(t, s)
			c10DraftWaitForRelease(t, f.tm.pool, life.pid, paused[0].xid)
			checkTrash() // Writer rollback must not undo the independent purge commit.
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}

func ingestNativeOldFCheckEnd(t *testing.T, s *ingestNativeOldF) {
	t.Helper()
	s.life.mu.Lock()
	events := slices.Clone(s.life.events)
	s.life.mu.Unlock()
	var bodies []string
	begin, commits, rollbacks, commitEnds, marks, publishers := 0, 0, 0, 0, 0, 0
	nativeErrors := 0
	lockEnd, headEnd, firstBody, lastBodyEnd, mutation, commit, manager := -1, -1, -1, -1, -1, -1, -1
	for i, e := range events {
		switch e.Kind {
		case "marks":
			marks++
		case "publish":
			publishers++
		case "manager.exit":
			manager = i
		}
		if e.Kind != "sql.start" && e.Kind != "sql.end" {
			continue
		}
		name := stableIndexSQLName(e.SQL)
		if e.Err != nil && s.prepared != nil && s.mode != "healthy" && name == "InspectPageLinksForPreparation" && errors.Is(e.Err, s.cause) {
			nativeErrors++
		} else if e.PID != s.life.pid || e.Err != nil {
			t.Fatalf("NATIVE_OLD_F_TX: SQL changed PID or had an unintended native error: %+v", e)
		}
		if e.PID != s.life.pid {
			t.Fatal("NATIVE_OLD_F_TX: native fault changed the actual Writer PID")
		}
		if e.Kind == "sql.end" {
			if name == "LockNotebook" && strings.Contains(e.SQL, "pg_advisory_xact_lock") {
				lockEnd = i
			}
			if name == "IngestIndexedHeads" && firstBody < 0 {
				headEnd = i
			}
			if s.nativeRead(name) {
				lastBodyEnd = i
			}
			if name == "commit" {
				commitEnds++
			}
			continue
		}
		switch name {
		case "begin":
			begin++
		case "commit":
			commits++
			commit = i
		case "rollback":
			rollbacks++
		case "IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter", "InspectPageLinksForPreparation":
			if !s.nativeRead(name) {
				t.Fatal("NATIVE_OLD_F_SQL: prepared and fallback reads were mixed")
			}
			if firstBody < 0 {
				firstBody = i
			}
			bodies = append(bodies, name)
		case "DeleteLinksOf", "DeleteIndexedPages", "DeleteTagsOf", "DeletePropertiesOf", "DeleteAliasesOf", "InsertIndexedPage", "DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered":
			if mutation < 0 {
				mutation = i
			}
		}
	}
	if lockEnd < 0 || headEnd <= lockEnd || firstBody <= headEnd {
		t.Fatalf("NATIVE_OLD_F_BEFORE: lock/head/private read order=%d/%d/%d", lockEnd, headEnd, firstBody)
	}
	want := []string{"IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter"}
	if s.prepared != nil {
		want = []string{"InspectPageLinksForPreparation", "InspectPageLinksForPreparation"}
	}
	if s.mode == "healthy" {
		if s.prepared == nil {
			want = append(want, "IngestOldBodyReferencesAfter")
		}
		if begin != 1 || commits != 1 || commitEnds != 1 || rollbacks != 0 || marks != 1 || publishers != 1 || lastBodyEnd < 0 || mutation <= lastBodyEnd || commit <= mutation || manager <= commit {
			t.Fatalf("NATIVE_OLD_F_COMMIT: bodies=%v end=%d mutation=%d commit=%d manager=%d counts=%d/%d/%d/%d/%d/%d", bodies, lastBodyEnd, mutation, commit, manager, begin, commits, commitEnds, rollbacks, marks, publishers)
		}
	} else {
		if mutation >= 0 {
			t.Fatal("NATIVE_OLD_F_ROLLBACK: index mutation followed failed After")
		}
		c10DraftCheckEnd(t, s.life, events) // original bounded first rollback, zero marks/publish/commit
	}
	wantNativeErrors := 0
	if s.prepared != nil && s.mode != "healthy" {
		wantNativeErrors = 1
	}
	if nativeErrors != wantNativeErrors {
		t.Fatalf("PREPARED_NATIVE_CAUSE: actual native trace errors=%d want%d", nativeErrors, wantNativeErrors)
	}
	if !slices.Equal(bodies, want) {
		t.Fatalf("NATIVE_OLD_F_SQL: actual First/After inventory=%v want%v", bodies, want)
	}
}

// Keep the original full Writer image fields, adding the index side facts.
// Every relation, including marks, is scoped to this notebook. The unrelated
// normal notebook's durable purge is checked independently before and after end.
func ingestNativeOldFImage(t *testing.T, f ingestFixture) string {
	t.Helper()
	var out string
	err := f.tm.pool.QueryRow(t.Context(), `SELECT jsonb_build_object(
		'nodes',(SELECT coalesce(jsonb_agg(to_jsonb(n) ORDER BY id),'[]'::jsonb) FROM nodes n WHERE notebook_id=$1),
		'content',(SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.node_id),'[]'::jsonb) FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE n.notebook_id=$1),
		'index',(SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.node_id),'[]'::jsonb) FROM indexed_pages i WHERE i.notebook_id=$1),
		'links',(SELECT coalesce(jsonb_agg(to_jsonb(l) ORDER BY l.source_id,l.range_start),'[]'::jsonb) FROM page_links l WHERE l.notebook_id=$1),
		'tags',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.source_id,t.tag_key),'[]'::jsonb) FROM page_tags t WHERE t.notebook_id=$1),
		'properties',(SELECT coalesce(jsonb_agg(to_jsonb(p) ORDER BY p.source_id,p.position),'[]'::jsonb) FROM page_properties p WHERE p.notebook_id=$1),
		'aliases',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.source_id,a.alias_key),'[]'::jsonb) FROM page_aliases a WHERE a.notebook_id=$1),
		'changesets',(SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM changesets c WHERE c.notebook_id=$1),
		'revisions',(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]'::jsonb) FROM page_revisions r JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id=$1),
		'marks',(SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY m.changeset_id,m.seq),'[]'::jsonb) FROM changeset_sources m JOIN changesets c ON c.id=m.changeset_id WHERE c.notebook_id=$1)
	)::text`, f.nb).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
