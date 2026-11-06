package bootstrap

import "testing"

// Design 4.14 keeps the source root and the root AGENTS out of "wiki page" on
// all three legs of the proof, and reads a digesting edge narrowly.
func testIngestExclusions(t *testing.T) {
	agents := func(t *testing.T, f ingestFixture) string {
		t.Helper()
		rows := queryStrings(t, f.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND parent_id IS NULL AND name='AGENTS'", f.nb)
		if len(rows) != 1 {
			t.Fatalf("root AGENTS rows %v", rows)
		}
		return rows[0]
	}
	// A write to either one is no ingest.
	t.Run("new_emitter_agents", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		id := agents(t, f)
		cs := f.write(t, id, "[[raw/S]]")
		f.tm.resolves(t, id, f.s)
		assertIngestMarks(t, f.tm, cs)
	})
	t.Run("new_emitter_root", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		cs := f.write(t, f.raw, "[[raw/S]]")
		f.tm.resolves(t, f.raw, f.s)
		assertIngestMarks(t, f.tm, cs)
	})
	// Left alone, neither one digests S for a later P.
	t.Run("untouched_agents", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.write(t, agents(t, f), "[[raw/S]]")
		assertIngestMarks(t, f.tm, f.write(t, f.p, "[[raw/S]]"), f.s)
	})
	t.Run("untouched_root", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.write(t, f.raw, "[[raw/S]]")
		assertIngestMarks(t, f.tm, f.write(t, f.p, "[[raw/S]]"), f.s)
	})
	// Dropping the reference from either one in the unit that adds it to P
	// leaves no old D behind.
	t.Run("before_agents", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		id := agents(t, f)
		f.write(t, id, "[[raw/S]]")
		rev := f.tm.content(t, "alice", id).Revision
		cs := f.batch(t, opWriteOn("AGENTS.md", "no link", versionOf(id, rev)), opWriteOn("P.md", "[[raw/S]]", versionOf(f.p, 1)))
		assertIngestMarks(t, f.tm, cs, f.s)
	})
	t.Run("before_root", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.write(t, f.raw, "[[raw/S]]")
		rev := f.tm.content(t, "alice", f.raw).Revision
		cs := f.batch(t, opWriteOn("raw.md", "no link", versionOf(f.raw, rev)), opWriteOn("P.md", "[[raw/S]]", versionOf(f.p, 1)))
		assertIngestMarks(t, f.tm, cs, f.s)
	})
	// A target known empty at Before was never digested, even when the same
	// unit fills it.
	t.Run("before_known_empty_target", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		e := f.tm.createPage(t, "alice", f.nb, f.raw, "E")
		f.write(t, f.p, "[[raw/E]]")
		rev := f.tm.content(t, "alice", f.p).Revision
		cs := f.batch(t, opWriteOn("raw/E.md", "first bytes", versionOf(e, 1)), opWriteOn("P.md", "[[raw/E]] again", versionOf(f.p, rev)))
		assertIngestMarks(t, f.tm, cs, e)
	})
	// F(A) is a real body edge only: a frontmatter asset edge of a current
	// source leaves A unfollowed, even when untouched Q digests that source.
	t.Run("f_frontmatter_not_followed", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset A")
		source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "---\nthumb: '[A](a.txt)'\n---\nSource body.\n")
		f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Follow]]")
		if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1 AND resolved_id=$2 AND resolved_asset AND property_key IS NOT NULL", source, a.ID); n != 1 {
			t.Fatalf("frontmatter asset edges=%d, want 1", n)
		}
		assertIngestMarks(t, f.tm, f.write(t, f.p, "[A](raw/a.txt) [[raw/T]]"), a.ID, f.t)
	})
	// A slot is a root string or a direct string of the root array: a number
	// at sources[0] leaves a top-level 'sources.0' link no slot.
	t.Run("nonstring_element_no_slot", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		f.tm.createPageWith(t, "alice", f.nb, "", "Q", "---\nsources: [1]\n'sources.0': '[[raw/S]]'\n---\n")
		assertIngestMarks(t, f.tm, f.write(t, f.p, "[[raw/S]]"), f.s)
	})
}
