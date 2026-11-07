package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

type ingestFixture struct {
	tm               acmeTeam
	nb, raw, s, t, p string
}

func newIngestFixture(t *testing.T, w *wiring) ingestFixture {
	t.Helper()
	var tm acmeTeam
	if w == nil {
		tm = newAcmeTeam(t, "", "")
	} else {
		tm = newAcmeTeamWiring(t, "", "", *w)
	}
	return ingestFixtureOn(t, tm)
}
func ingestFixtureOn(t *testing.T, tm acmeTeam) ingestFixture {
	t.Helper()
	book := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest semantics","mode":"llm_wiki"}`)
	f := ingestFixture{tm: tm, nb: book.ID, raw: *book.SourceRootID}
	f.s = tm.createPageWith(t, "alice", f.nb, f.raw, "S", "Source S evidence.")
	f.t = tm.createPageWith(t, "alice", f.nb, f.raw, "T", "Independent source T evidence.")
	f.p = tm.createPage(t, "alice", f.nb, "", "P")
	return f
}
func ingestExec(t *testing.T, tm acmeTeam, query string, args ...any) {
	t.Helper()
	if _, err := tm.pool.Exec(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}
func ingestCS(t *testing.T, tm acmeTeam, id string, revision int) string {
	t.Helper()
	rows := queryStrings(t, tm.pool, "SELECT changeset_id::text FROM page_revisions WHERE node_id=$1 AND revision=$2", id, revision)
	if len(rows) != 1 {
		t.Fatalf("exact revision %s:%d changesets %v", id, revision, rows)
	}
	return rows[0]
}
func assertIngestMarks(t *testing.T, tm acmeTeam, cs string, want ...string) {
	t.Helper()
	want = slices.Clone(want)
	slices.Sort(want)
	got := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY node_id", cs)
	if !slices.Equal(got, want) {
		t.Errorf("sources=%v, want %v for %s", got, want, cs)
	}
	if n := count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", cs); n != len(want) {
		t.Errorf("ingest_count=%d, want%d", n, len(want))
	}
}
func (f ingestFixture) write(t *testing.T, id, content string) string {
	t.Helper()
	before := f.tm.content(t, "alice", id)
	f.tm.send(t, contentWrite("alice", id, content, before.Revision, ""), http.StatusOK)
	after := f.tm.content(t, "alice", id)
	if after.Revision != before.Revision+1 || after.Content != content {
		t.Fatalf("write was not real: before%+v after%+v", before, after)
	}
	return ingestCS(t, f.tm, id, after.Revision)
}
func (f ingestFixture) batch(t *testing.T, ops ...map[string]any) string {
	t.Helper()
	m := f.tm.agent(t, "alice", f.nb)
	m.handshakeAs(t, "ingest-semantics")
	result := answerOf[mcpBatched](t, m.batch(t, ops...))
	if !result.Changed || result.ChangesetID == nil || result.ResultsCount != len(ops) {
		t.Fatalf("batch %+v", result)
	}
	return *result.ChangesetID
}

func TestIngestBefore(t *testing.T) {
	t.Run("old_f_tail", testIngestOldFTail)
	t.Run("private_old_f_purge", testIngestOldFPurge)
	t.Run("private_old_f_batch_cancel", testIngestOldFBatchCancel)
	t.Run("private_old_f_native_boundary", testIngestOldFNativeBoundary)
	t.Run("prepared_source_native_boundary", testIngestPreparedSourceBoundary)
	t.Run("source_value_bytes", testIngestSourceValueBytes)
	testIngestBeforeBusiness(t)
	t.Run("exclusions", testIngestExclusions)
	t.Run("missing_ancestor", testIngestMissingAncestor)
	t.Run("possible_incomplete", testIngestOldSlotsFailure)
	t.Run("has_negative", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.write(t, f.p, "[[raw/S]]")
		cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
		assertIngestMarks(t, f.tm, cs, f.t)
		checkPages(t, f.tm.pool)
		checkLinks(t, f.tm.pool)
	})
	for _, kind := range []string{"capture_before_replace", "capture_before_delete"} {
		t.Run(kind, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/S]]")
			if got := f.tm.resolvedFrom(t, q); !slices.Equal(got, []string{f.s}) {
				t.Fatalf("old Q targets=%v", got)
			}
			change := opWriteOn("Q.md", "old reference removed", versionOf(q, 1))
			if kind == "capture_before_delete" {
				change = opDelete("Q.md")
			}
			cs := f.batch(t, change, opWriteOn("P.md", "[[raw/S]] [[raw/T]]", versionOf(f.p, 1)))
			assertIngestMarks(t, f.tm, cs, f.t)
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
	for _, state := range []string{"current", "literal_v1", "missing_head", "revision"} {
		t.Run("target_"+state, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			switch state {
			case "literal_v1":
				ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", f.s)
			case "missing_head":
				ingestExec(t, f.tm, "DELETE FROM indexed_pages WHERE node_id=$1", f.s)
			case "revision":
				ingestExec(t, f.tm, "UPDATE indexed_pages SET revision=9 WHERE node_id=$1", f.s)
			}
			cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
			want := []string{f.t}
			if state == "current" {
				want = append(want, f.s)
			}
			assertIngestMarks(t, f.tm, cs, want...)
			f.tm.reindex(t, f.nb)
			checkPages(t, f.tm.pool)
		})
	}
	// An untouched source's stale head hides its F. Q left alone gives no
	// D_U for it, so A stands alone as lint reads it; Q written carries old
	// D_W evidence, which keeps A from being marked. T is marked either way.
	for _, q := range []string{"q_untouched", "q_written"} {
		t.Run("stale_parent_"+q, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "followed asset")
			parent := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Parent", "Source body [A](a.txt).")
			id := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Parent]]")
			ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", parent)
			ops := []map[string]any{opWriteOn("P.md", "[A](raw/a.txt) [[raw/T]]", versionOf(f.p, 1))}
			want := []string{f.t, a.ID}
			if q == "q_written" {
				ops = append(ops, opWriteOn("Q.md", "changed [[raw/Parent]]", versionOf(id, 1)))
				want = []string{f.t}
			}
			assertIngestMarks(t, f.tm, f.batch(t, ops...), want...)
		})
	}
	t.Run("asset_without_page_head", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "independent asset")
		if n := count(t, f.tm.pool, "SELECT count(*) FROM indexed_pages WHERE node_id=$1", a.ID); n != 0 {
			t.Fatal("fixture asset has page head")
		}
		cs := f.write(t, f.p, "[asset](raw/a.txt)")
		assertIngestMarks(t, f.tm, cs, a.ID)
		checkPages(t, f.tm.pool)
	})
	t.Run("absent_distinct", func(t *testing.T) {
		for _, state := range []string{"new", "missing", "stale"} {
			t.Run(state, func(t *testing.T) {
				f := newIngestFixture(t, nil)
				if state == "new" {
					id := f.tm.createPageWith(t, "alice", f.nb, "", "New", "[[raw/S]]")
					assertIngestMarks(t, f.tm, ingestCS(t, f.tm, id, 1), f.s)
					return
				}
				f.write(t, f.p, "[[raw/S]]")
				if state == "missing" {
					ingestExec(t, f.tm, "DELETE FROM indexed_pages WHERE node_id=$1", f.p)
				} else {
					ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", f.p)
				}
				cs := f.write(t, f.p, "changed [[raw/S]]")
				assertIngestMarks(t, f.tm, cs)
			})
		}
	})
}

func TestIngestCandidates(t *testing.T) {
	t.Run("stream_failure", testIngestStreamFailure)
	for _, tt := range []struct {
		name, old, new string
		wantS          bool
	}{
		{"object_to_array", "---\nsources: {nested: '[[raw/S]]'}\n---\n", "---\nsources: ['[[raw/S]]']\n---\n", true},
		{"array_same", "---\nsources: ['[[raw/S]]']\n---\n", "---\nsources: ['[[raw/S]]']\n---\nchanged", false},
		{"conflict_to_unique", "---\nsources: ['[[raw/S]]']\n'sources.0': '[[raw/T]]'\n---\n", "---\nsources: ['[[raw/S]]']\n---\n", true},
		{"unique_to_conflict", "---\nsources: ['[[raw/S]]']\n---\n", "---\nsources: ['[[raw/S]]']\n'sources.0': '[[raw/T]]'\n---\n", false},
	} {
		t.Run("old_json_shapes/"+tt.name, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			f.write(t, f.p, tt.old)
			cs := f.write(t, f.p, tt.new)
			want := []string{}
			if tt.wantS {
				want = append(want, f.s)
			}
			assertIngestMarks(t, f.tm, cs, want...)
		})
	}
	for _, tt := range []struct{ name, value string }{
		{"all_peers", "[[raw/T]]"}, {"duplicate_peer", "[[raw/S]]"}, {"unresolved_peer", "[[Missing]]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "---\nsources: ['[[raw/S]]']\n'sources.0': '"+tt.value+"'\n---\n")
			if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key='sources.0'", q); n != 2 {
				t.Fatalf("slot peers=%d, want2", n)
			}
			cs := f.write(t, f.p, "[[raw/S]]")
			assertIngestMarks(t, f.tm, cs, f.s)
		})
	}
	for _, body := range []bool{false, true} {
		t.Run(fmt.Sprintf("body_empty_property/%t", body), func(t *testing.T) {
			f := newIngestFixture(t, nil)
			text := "---\n'': '[[raw/S]]'\n---\n"
			if body {
				text = "[[raw/S]]"
			}
			f.tm.createPageWith(t, "alice", f.nb, "", "Q", text)
			cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
			want := []string{f.t}
			if !body {
				want = append(want, f.s)
			}
			assertIngestMarks(t, f.tm, cs, want...)
		})
	}
	for _, name := range []string{"late_invalid", "invalid_no_novel"} {
		t.Run(name, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "---\nsources: ['[[raw/S]]']\n---\n")
			ingestExec(t, f.tm, `UPDATE page_properties SET value=(SELECT jsonb_agg('[[raw/S]]'::text) FROM generate_series(1,10001)) WHERE source_id=$1 AND key='sources'`, q)
			text := "[[raw/S]] [[raw/T]]"
			if name == "invalid_no_novel" {
				text = "[[raw/S]]"
			}
			before := count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb)
			f.tm.send(t, contentWrite("alice", f.p, text, 1, ""), http.StatusInternalServerError)
			if got := f.tm.content(t, "alice", f.p); got.Revision != 1 || got.Content != "" {
				t.Fatalf("failed evidence persisted %+v", got)
			}
			if count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb) != before {
				t.Fatal("failed evidence committed changeset")
			}
			if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1", f.p); n != 0 {
				t.Fatalf("failed evidence persisted%d index links", n)
			}
		})
	}
}

func runIngestUnitCases(t *testing.T) {
	t.Run("kind_gate", testIngestKindGate)
	testIngestUnknownFollow(t)
	for _, name := range []string{"old_follow_untouched", "old_follow_changed_q", "old_follow_deleted_q", "old_follow_delete_both", "old_follow_rewrite_s", "f_without_d", "untouched_deleted_source_approx"} {
		t.Run(name, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset A")
			source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "Source F body [A](a.txt).")
			if got := f.tm.resolvedFrom(t, source); !slices.Equal(got, []string{a.ID}) {
				t.Fatalf("source body targets=%v", got)
			}
			var q string
			if name != "f_without_d" {
				q = f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Follow]]")
			}
			var ops []map[string]any
			target := "a.txt"
			switch name {
			case "old_follow_changed_q":
				ops = append(ops, opWriteOn("Q.md", "removed", versionOf(q, 1)))
			case "old_follow_deleted_q":
				ops = append(ops, opDelete("Q.md"))
			case "old_follow_delete_both":
				ops = append(ops, opDelete("Q.md"), opDelete("raw/Follow.md"))
			case "old_follow_rewrite_s", "f_without_d":
				ops = append(ops, opMove("raw/a.txt", "raw/b.txt"))
				target = "b.txt"
			case "untouched_deleted_source_approx":
				ops = append(ops, opDelete("raw/Follow.md"))
			}
			ops = append(ops, opWriteOn("P.md", "[A](raw/"+target+") [[raw/T]]", versionOf(f.p, 1)))
			cs := f.batch(t, ops...)
			want := []string{f.t}
			if name == "f_without_d" || name == "untouched_deleted_source_approx" {
				want = append(want, a.ID)
			}
			assertIngestMarks(t, f.tm, cs, want...)
			if name == "old_follow_rewrite_s" || name == "f_without_d" {
				if current := f.tm.content(t, "alice", source); current.Revision != 2 {
					t.Fatalf("source was not actually system rewritten: %+v", current)
				}
				if n := count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id=$1 AND revision=2 AND rewrite", source); n != 1 {
					t.Fatalf("actual system rewrite history rows=%d", n)
				}
			}
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
	t.Run("marks_append", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		session := f.tm.openSession(t, "alice", f.p)
		var cs string
		for i, text := range []string{"[[raw/S]]", "removed", "[[raw/S]]", "[[raw/S]] [[raw/T]]"} {
			f.tm.send(t, contentWrite("alice", f.p, text, i+1, session), http.StatusOK)
			got := ingestCS(t, f.tm, f.p, i+2)
			if i == 0 {
				cs = got
			} else if cs != got {
				t.Fatal("session did not reuse changeset")
			}
			want := []string{f.s}
			if i == 3 {
				want = append(want, f.t)
			}
			assertIngestMarks(t, f.tm, cs, want...)
		}
	})
	t.Run("reindex_no_history", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.write(t, f.p, "[[raw/S]]")
		before := count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb)
		marks := count(t, f.tm.pool, "SELECT count(*) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1", f.nb)
		ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE notebook_id=$1", f.nb)
		f.tm.reindex(t, f.nb)
		if count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb) != before || count(t, f.tm.pool, "SELECT count(*) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1", f.nb) != marks {
			t.Fatal("reindex backfilled edit history")
		}
		cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
		assertIngestMarks(t, f.tm, cs, f.t)
	})
	t.Run("reindex_legacy_no_history", testIngestReindexLegacyNoHistory)
	t.Run("ordinary_mode", func(t *testing.T) {
		tm := newAcmeTeam(t, "", "")
		nb := tm.openNotebook(t, "alice", "ordinary")
		s := tm.createPageWith(t, "alice", nb, "", "Source", "evidence")
		p := tm.createPageWith(t, "alice", nb, "", "Output", "[[Source]]")
		if got := tm.resolvedFrom(t, p); !slices.Equal(got, []string{s}) {
			t.Fatalf("nonempty standard refs=%v", got)
		}
		assertIngestMarks(t, tm, ingestCS(t, tm, p, 1))
	})
}

// Keep an ordinary context import tied to the real production port boundary.
var _ page.IngestBefore = ingestBeforeFunc(nil)

type ingestBeforeFunc func(context.Context, []uuid.UUID) ([]page.IngestNodeBefore, error)

func (f ingestBeforeFunc) NodesBefore(ctx context.Context, ids []uuid.UUID) ([]page.IngestNodeBefore, error) {
	return f(ctx, ids)
}
