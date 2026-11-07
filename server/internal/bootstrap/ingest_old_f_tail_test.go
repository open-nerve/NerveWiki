package bootstrap

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// This records only the actual production old-F role for one source, while
// the existing ingestStreamManager owns the real transaction/error/image proof.
// It does not run an extra Capture or replace a query's result set.
type ingestOldFTailState struct {
	source, tail uuid.UUID
	mode         string
	cause        error
	ids          []uuid.UUID
	batches      []*ingestOldFTailBatch
	lastStart    int32
	injected     bool
}
type ingestOldFTailBatch struct {
	after        bool
	rows         int
	eof, errRead bool
	closed       bool
}
type ingestOldFTailRows struct {
	pgx.Rows
	s *ingestOldFTailState
	b *ingestOldFTailBatch
}

func (s *ingestOldFTailState) wrap(rows pgx.Rows, sql string, args []any, replaces int) (pgx.Rows, error) {
	first := strings.HasPrefix(sql, "-- name: IngestOldBodyReferencesFirst ")
	after := strings.HasPrefix(sql, "-- name: IngestOldBodyReferencesAfter ")
	if replaces != 0 || !first && !after {
		return rows, nil
	}
	// The private adapter prepends one execution mode. Find the actual UUID
	// array by type, not an assumed positional argument number.
	selected := false
	for _, arg := range args {
		if ids, ok := arg.([]uuid.UUID); ok && slices.Contains(ids, s.source) {
			if len(ids) != 1 {
				rows.Close()
				return nil, fmt.Errorf("old-F fixture: source role has unexpected emitters %v", ids)
			}
			selected = true
		}
	}
	if !selected {
		return rows, nil
	}
	b := &ingestOldFTailBatch{after: after}
	s.batches = append(s.batches, b)
	return &ingestOldFTailRows{Rows: rows, s: s, b: b}, nil
}
func (r *ingestOldFTailRows) Close() {
	r.Rows.Close()
	r.b.closed = true
}

func (r *ingestOldFTailRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	// Retain the four-column Scan assertion; native generation must bind the
	// IngestOldBodyReferences{First,After} types before this draft is installed.
	if len(dest) != 4 {
		return fmt.Errorf("old-F fixture: columns=%d, want4", len(dest))
	}
	source, okSource := dest[0].(*uuid.UUID)
	start, okStart := dest[1].(*int32)
	target, okTarget := dest[2].(**uuid.UUID)
	asset, okAsset := dest[3].(*bool)
	if !okSource || !okStart || !okTarget || !okAsset || *source != r.s.source || *target == nil || !*asset || *start <= r.s.lastStart {
		return errors.New("old-F fixture: actual row is not the expected advancing source asset edge")
	}
	r.s.lastStart = *start
	r.b.rows++
	r.s.ids = append(r.s.ids, **target)
	if len(r.s.ids) == 513 {
		if r.b.after || **target != r.s.tail {
			return errors.New("old-F fixture: real 513th row is not the independent tail asset in First")
		}
		if r.s.mode == "scan" {
			r.s.injected = true
			return r.s.cause
		}
	}
	return nil
}
func (r *ingestOldFTailRows) Next() bool {
	ok := r.Rows.Next()
	if !ok {
		r.b.eof = true
	}
	return ok
}
func (r *ingestOldFTailRows) Err() error {
	r.b.errRead = true
	if err := r.Rows.Err(); err != nil {
		return err
	}
	if r.s.mode == "rows_err" && !r.b.after && r.b.rows == 513 && r.b.eof && len(r.s.ids) == 513 {
		r.s.injected = true
		return r.s.cause
	}
	return nil
}

func testIngestOldFTail(t *testing.T) {
	for _, mode := range []string{"healthy", "scan", "rows_err"} {
		t.Run(mode, func(t *testing.T) {
			old := &ingestOldFTailState{mode: mode, cause: errors.New("designated old-F First 513th-row failure"), lastStart: -1}
			stream := &ingestStreamState{done: make(chan struct{}), mode: "healthy", oldF: old}
			f := newIngestFixture(t, &wiring{page: func(d *page.Deps) {
				d.Tx = ingestStreamManager{Tx: d.Tx, state: stream}
				// This case owns the original First/After fallback protocol.
				// B18 and prepared_native_boundary cover the default preparation.
				found := 0
				for i, observer := range d.Observers {
					if bridge, ok := observer.(linkIndex); ok {
						bridge.index.Preparations = nil
						d.Observers[i] = bridge
						found++
					}
				}
				if found != 1 {
					t.Fatalf("old-F fallback producer observers=%d", found)
				}
			}})
			ids, wikiBody := ingestManyAssets(t, f, 513)
			wantF := make([]uuid.UUID, len(ids))
			for i, id := range ids {
				wantF[i] = uuid.MustParse(id)
			}
			// Assets are real source siblings. The existing asset fixture uses
			// wiki-root paths; the source body uses the same targets as siblings.
			body := "Complete old F.\n" + strings.ReplaceAll(wikiBody, "[](raw/", "[](")
			source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow513", body)
			q := f.tm.createPageWith(t, "alice", f.nb, "", "Q513", "[[raw/Follow513]]")
			f.tm.resolves(t, source, ids...)
			f.tm.resolves(t, q, source)
			if n := count(t, f.tm.pool, `SELECT count(*) FROM page_links l
				JOIN indexed_pages i ON i.node_id=l.source_id JOIN page_contents c ON c.node_id=l.source_id
				WHERE l.source_id=$1 AND l.property_key IS NULL AND l.resolved_asset
				AND i.extractor=2 AND i.revision=c.revision AND c.revision=1`, source); n != 513 {
				t.Fatalf("old-F real current parser rows=%d, want513", n)
			}
			old.source, old.tail = uuid.MustParse(source), wantF[512]
			m := f.tm.agent(t, "alice", f.nb)
			m.handshakeAs(t, "ingest-old-f-tail")
			before := ingestStreamImage(t, f)
			stream.armed.Store(true)
			answer := m.batch(t,
				opMove("raw/A00000.txt", "raw/Z00000.txt"),
				opWriteOn("P.md", "[tail](raw/A00512.txt) [[raw/T]]", versionOf(f.p, 1)))
			select {
			case <-stream.done:
			case <-time.After(interleavingWait):
				t.Fatal("OLD_F_FIXTURE: original transaction did not finish; timeout is not a kill")
			}
			// This is the complete actual old-source body stream, in raw edge
			// order, compared with independently constructed asset identities.
			// Tail A513 also has an independent persisted-marks business oracle.
			if !slices.Equal(old.ids, wantF) {
				t.Fatalf("OLD_F_FULL_SET: actual old-F IDs=%v, want all513=%v", old.ids, wantF)
			}
			// The private physical read now owns all513 rows. App delivery is
			// still512+1; this fixture no longer claims a nonempty native After.
			if len(old.batches) == 0 || old.batches[0].after || old.batches[0].rows != 513 || !old.batches[0].closed {
				t.Fatalf("OLD_F_BATCHES: real First513 was not closed: %+v", old.batches)
			}
			if mode != "scan" && (!old.batches[0].eof || !old.batches[0].errRead) {
				t.Fatalf("OLD_F_FIRST_EOF: First513 lacks actual EOF/Err: %+v", old.batches[0])
			}
			if mode == "healthy" {
				if len(old.batches) != 2 || !old.batches[1].after || old.batches[1].rows != 0 || !old.batches[1].eof || !old.batches[1].errRead || !old.batches[1].closed {
					t.Fatalf("OLD_F_EOF: missing terminal empty After/Err: %+v", old.batches)
				}
				result := answerOf[mcpBatched](t, answer)
				if old.injected || stream.callbackErr != nil || stream.managerErr != nil || stream.replaces == 0 || stream.calls != 1 || stream.adds != 1 || !result.Changed || result.ChangesetID == nil {
					t.Fatalf("OLD_F_HEALTHY: stream=%+v result=%+v", stream, result)
				}
				assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
				if c := f.tm.content(t, "alice", source); c.Revision != 2 {
					t.Fatalf("OLD_F_REWRITE: source was not actually rewritten: %+v", c)
				}
				if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id=$1 AND revision=2 AND rewrite", source) != 1 {
					t.Fatal("OLD_F_REWRITE: real participant rewrite history absent")
				}
				f.tm.resolves(t, source, ids...)
				f.tm.resolves(t, f.p, ids[512], f.t)
			} else {
				if len(old.batches) != 1 || mode == "scan" && (old.batches[0].eof || old.batches[0].errRead) {
					t.Fatalf("OLD_F_FIRST_ERROR: error queried After or lost Scan/Err boundary: %+v", old.batches)
				}
				if !old.injected || !errors.Is(stream.callbackErr, old.cause) || !errors.Is(stream.managerErr, old.cause) {
					t.Fatalf("OLD_F_ORIGINAL_CAUSE: injected=%v callback=%v manager=%v", old.injected, stream.callbackErr, stream.managerErr)
				}
				if stream.replaces != 0 || stream.calls != 0 || stream.adds != 0 {
					t.Fatalf("OLD_F_NO_FOLLOWUP: Replace=%d Novel=%d Add=%d", stream.replaces, stream.calls, stream.adds)
				}
				if after := ingestStreamImage(t, f); after != before {
					t.Fatalf("OLD_F_ROLLBACK: before=%s after=%s", before, after)
				}
				if answer.status != http.StatusOK || answer.result(t)["isError"] != true {
					t.Fatalf("OLD_F_ERROR_REPLY: status=%d body=%s", answer.status, answer.body)
				}
			}
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}
