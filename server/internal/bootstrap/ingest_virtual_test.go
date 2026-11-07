package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// These consumers use marks made by real writes, never by INSERT or Add.
// Dropping ingest fields, compressing missing slots, or purging a retained
// changeset's sources must lose a nonempty, independently specified result.
type ingestVirtualFixture struct {
	tm       acmeTeam
	book     guardedNotebook
	m        *mcpApp
	ids      []string
	titles   map[string]string
	note, cs string
}

// empty leaves all six sources empty for the legal first-write overlap case.
func ingestVirtualSources(t *testing.T, tm acmeTeam, book guardedNotebook, empty bool) ingestVirtualFixture {
	t.Helper()
	f := ingestVirtualFixture{tm: tm, book: book, m: tm.agent(t, "alice", book.id), titles: map[string]string{}}
	structured(t, f.m.use(t, "write", map[string]any{"path": "raw.md", "content": "", "base_version": book.raw + ":1"}))
	structured(t, f.m.use(t, "write", map[string]any{"path": "AGENTS.md", "content": "Instructions without links.", "base_version": book.agents + ":1"}))
	for i := range 6 {
		title := fmt.Sprintf("S%02d", i)
		body := "source:" + title + "\n"
		if empty {
			body = ""
		}
		id := tm.createPageWith(t, "alice", book.id, book.raw, title, body)
		f.ids = append(f.ids, id)
		f.titles[id] = title
	}
	slices.Sort(f.ids)
	f.note = tm.createPageWith(t, "alice", book.id, "", "Summary", "before")
	return f
}

func (f *ingestVirtualFixture) produce(t *testing.T) {
	t.Helper()
	written := answerOf[mcpWritten](t, f.m.use(t, "write", map[string]any{
		"path": "Summary.md", "base_version": f.note + ":1",
		"content": "[[raw/S00]]\n[[raw/S01]]\n[[raw/S02]]\n[[raw/S03]]\n[[raw/S04]]\n[[raw/S05]]\n",
	}))
	if !written.Changed || written.NodeID != f.note || written.ChangesetID == nil {
		t.Fatalf("real six-source producer: %+v", written)
	}
	f.cs = *written.ChangesetID
	f.marks(t)
}

func (f ingestVirtualFixture) marks(t *testing.T) {
	t.Helper()
	if f.cs == "" || len(f.ids) != 6 {
		t.Fatal("missing real producer identity")
	}
	want := make([]string, 6)
	for i, id := range f.ids {
		want[i] = fmt.Sprintf("%d:%s", i+1, id)
	}
	if got := queryStrings(t, f.tm.pool, `SELECT notebook_id::text||':'||kind||':'||ingest_count::text
 FROM changesets WHERE id=$1 AND deleted_at IS NULL`, f.cs); !slices.Equal(got, []string{f.book.id + ":edit:6"}) {
		t.Fatalf("retained real changeset/count=%q", got)
	}
	if got := queryStrings(t, f.tm.pool, "SELECT seq::text||':'||node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq", f.cs); !slices.Equal(got, want) {
		t.Fatalf("complete persisted marks=%q, want %q", got, want)
	}
	if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND node_id=$2 AND deleted_at IS NULL", f.cs, f.note) != 1 {
		t.Fatal("producer changeset lost its retained Summary revision")
	}
}

func (f ingestVirtualFixture) log(t *testing.T, body string, purged bool) {
	t.Helper()
	entry := virtualIngestEntry(t, body, f.cs)
	if !strings.HasSuffix(entry[0], "; ingests: 6") {
		t.Fatalf("producer count header=%q", entry[0])
	}
	want := make([]string, 5)
	for i, id := range f.ids[:5] {
		path := "raw/" + f.titles[id] + ".md"
		want[i] = "  - source: [" + path + "](<" + path + ">)"
		if purged && (i == 0 || i == 2) {
			want[i] = "  - source: Missing source"
		}
	}
	var got []string
	for _, line := range entry[1:] {
		if strings.HasPrefix(line, "  - source: ") {
			got = append(got, line)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("complete five source slots=%q, want %q", got, want)
	}
}

func (f ingestVirtualFixture) read(t *testing.T, purged bool) {
	t.Helper()
	a := f.m.use(t, "read", map[string]any{"path": "log.md"})
	r := a.result(t)
	blocks, ok := r["content"].([]any)
	if r["isError"] == true || !ok || len(blocks) != 2 {
		t.Fatalf("real log read: %s", a.body)
	}
	block, ok := blocks[1].(map[string]any)
	body, text := block["text"].(string)
	if !ok || !text {
		t.Fatalf("real log text block: %v", blocks)
	}
	f.log(t, body, purged)
}

func (f ingestVirtualFixture) zip(t *testing.T, name string, purged bool) {
	t.Helper()
	entries := virtualExportEntries(t, f.tm, f.book.id, "")
	f.log(t, entries[name+"/log.md"].data, purged)
	_, _, contributed := metaPaths(t, entries[name+"/.nerve/meta.json"].data)
	if !slices.Equal(contributed, []string{"index.md", "log.md"}) {
		t.Fatalf("real contribution metadata=%q", contributed)
	}
	for i, id := range f.ids {
		entry, exists := entries[name+"/raw/"+f.titles[id]+".md"]
		if purged && (i == 0 || i == 2) {
			if exists {
				t.Fatalf("purged source %s remains in real ZIP", id)
			}
		} else if !exists || entry.data != "source:"+f.titles[id]+"\n" {
			t.Fatalf("live source %s lost its real archive payload: %+v", id, entry)
		}
	}
}

func virtualExportIngestPurgedSource(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	awaitJob(t, tm.pool, jobs.PurgeKind)
	f := ingestVirtualSources(t, tm, tm.llmWiki(t, "PurgeIngestWiki"), false)
	f.produce(t)
	f.read(t, false)
	f.zip(t, "PurgeIngestWiki", false)
	for _, i := range []int{0, 2} {
		tm.send(t, nodeDeletion("alice", f.ids[i]), http.StatusNoContent)
		tm.trashedAgo(t, f.ids[i], 61*24*time.Hour)
	}
	ingestVirtualPurge(t, tm)
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items"} {
		column := "node_id"
		if table == "nodes" {
			column = "id"
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM "+table+" WHERE "+column+"=ANY($1::uuid[])", []uuid.UUID{uuid.MustParse(f.ids[0]), uuid.MustParse(f.ids[2])}); n != 0 {
			t.Fatalf("physical purge left %d rows in %s", n, table)
		}
	}
	if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 AND deleted_at IS NULL", f.ids[5]) != 1 {
		t.Fatal("sixth source is not a live no-refill counterexample")
	}
	f.marks(t)
	f.read(t, true)
	f.zip(t, "PurgeIngestWiki", true)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
}

// Keep the original ingest-cleanup 15s/25ms wait, but identify the actual
// newly inserted job and require its first attempt, not some completed job.
func ingestVirtualPurge(t *testing.T, tm acmeTeam) {
	t.Helper()
	ctx := t.Context()
	prior := count(t, tm.pool, "SELECT coalesce(max(id),0) FROM river_job WHERE kind=$1", jobs.PurgeKind)
	inserter, err := jobs.NewInserter(tm.pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := inserter.InsertTx(ctx, tx, ingestPurgeArgs{}, nil); err != nil {
		t.Fatal(err)
	}
	var n int
	var id int64
	if err := tx.QueryRow(ctx, "SELECT count(*),coalesce(min(id),0) FROM river_job WHERE kind=$1 AND id>$2", jobs.PurgeKind, prior).Scan(&n, &id); err != nil || n != 1 {
		t.Fatalf("exact inserted purge identity count=%d id=%d: %v", n, id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(25 * time.Millisecond) {
		var state string
		var attempt, failures int
		if err := tm.pool.QueryRow(ctx, "SELECT state::text,attempt,coalesce(cardinality(errors),0) FROM river_job WHERE id=$1 AND kind=$2", id, jobs.PurgeKind).Scan(&state, &attempt, &failures); err != nil {
			t.Fatal(err)
		}
		if failures != 0 || attempt > 1 {
			t.Fatalf("purge %d state=%s attempt=%d errors=%d", id, state, attempt, failures)
		}
		if state == "completed" {
			if attempt != 1 {
				t.Fatalf("purge %d completed at attempt%d", id, attempt)
			}
			return
		}
		if state != "available" && state != "pending" && state != "running" {
			t.Fatalf("purge %d unexpected state %s", id, state)
		}
		if time.Now().After(deadline) {
			t.Fatalf("purge %d did not finish its first run", id)
		}
	}
}
