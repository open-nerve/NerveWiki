package bootstrap

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// This changes one decoded byte-length scalar of a REAL SourceValues head.
// It is not an oversized submitted page and not a fabricated Capture result.
type ingestSourceBytesState struct {
	page                 uuid.UUID
	fault                bool
	headRows, valueReads int
	originalBytes        int64
	injected             bool
}
type ingestSourceBytesRows struct {
	pgx.Rows
	s *ingestSourceBytesState
}

func (s *ingestSourceBytesState) wrap(rows pgx.Rows, sql string, args []any, replaces int) pgx.Rows {
	if replaces != 0 {
		return rows
	}
	if strings.HasPrefix(sql, "-- name: LintFirstSourceValues ") {
		s.valueReads++
	}
	if !strings.HasPrefix(sql, "-- name: LintFirstSourceHeads ") {
		return rows
	}
	for _, arg := range args {
		if ids, ok := arg.([]uuid.UUID); ok && len(ids) == 1 && slices.Contains(ids, s.page) {
			return &ingestSourceBytesRows{Rows: rows, s: s}
		}
	}
	return rows
}
func (r *ingestSourceBytesRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	if len(dest) != 3 {
		return errors.New("SourceValues byte fixture: wrong real head projection")
	}
	id, okID := dest[0].(*uuid.UUID)
	length, okLength := dest[2].(*int64)
	if !okID || !okLength || *id != r.s.page || *length <= 0 || *length > linkingapp.MaxLintPayloadBytes(page.MaxContentBytes) {
		return errors.New("SourceValues byte fixture: actual healthy head is invalid")
	}
	r.s.headRows++
	r.s.originalBytes = *length
	if r.s.fault {
		*length = linkingapp.MaxLintPayloadBytes(page.MaxContentBytes) + 1
		r.s.injected = true
	}
	return nil
}
func testIngestSourceValueBytes(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(fmt.Sprintf("byte_bound_%v", fault), func(t *testing.T) {
			bytes := &ingestSourceBytesState{fault: fault}
			stream := &ingestStreamState{done: make(chan struct{}), mode: "healthy", sourceBytes: bytes}
			f := newIngestFixture(t, &wiring{page: func(d *page.Deps) { d.Tx = ingestStreamManager{Tx: d.Tx, state: stream} }})
			f.write(t, f.p, "---\nsources: ['[[raw/S]]']\n---\n")
			bytes.page = uuid.MustParse(f.p)
			m := f.tm.agent(t, "alice", f.nb)
			m.handshakeAs(t, "ingest-source-byte-bound")
			before := ingestStreamImage(t, f)
			stream.armed.Store(true)
			answer := m.batch(t, opWriteOn("P.md", "[[raw/S]] [[raw/T]]", versionOf(f.p, 2)))
			select {
			case <-stream.done:
			case <-time.After(interleavingWait):
				t.Fatal("SOURCE_BYTES_FIXTURE: unfinished transaction; timeout is not a kill")
			}
			if bytes.headRows != 1 || bytes.originalBytes <= 0 {
				t.Fatalf("SOURCE_BYTES_REAL_HEAD: %+v", bytes)
			}
			if fault {
				if !bytes.injected || stream.callbackErr == nil || !strings.Contains(stream.callbackErr.Error(), "lint reads: JSON exceeds derived bound") || !errors.Is(stream.managerErr, stream.callbackErr) {
					t.Fatalf("SOURCE_BYTES_ORIGINAL_ERROR: bytes=%+v callback=%v manager=%v", bytes, stream.callbackErr, stream.managerErr)
				}
				if bytes.valueReads != 0 || stream.replaces != 0 || stream.calls != 0 || stream.adds != 0 {
					t.Fatalf("SOURCE_BYTES_NO_FOLLOWUP: bytes=%+v stream=%+v", bytes, stream)
				}
				if after := ingestStreamImage(t, f); after != before {
					t.Fatalf("SOURCE_BYTES_ROLLBACK: before=%s after=%s", before, after)
				}
			} else {
				if bytes.injected || bytes.valueReads != 1 || stream.callbackErr != nil || stream.managerErr != nil {
					t.Fatalf("SOURCE_BYTES_HEALTHY: bytes=%+v stream=%+v", bytes, stream)
				}
				result := answerOf[mcpBatched](t, answer)
				if result.ChangesetID == nil {
					t.Fatal("SOURCE_BYTES_HEALTHY: no actual changeset")
				}
				assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
			}
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}
