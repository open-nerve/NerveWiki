package bootstrap

import (
	"context"
	"net/http"
	"testing"
	"uuid"
)

// sourceStored records durable state independently of the response: the
// content, notebook activity, node, all session fields, and row counts.
// A source refusal must roll all of these back, including a session's
// takeover or expired-row cleanup before the veto.
func (tm acmeTeam) sourceStored(t *testing.T, nb, id string) string {
	t.Helper()
	return tm.guardRows(t, nb) + queryStrings(t, tm.pool, `SELECT jsonb_build_object(
		'node', to_jsonb(n), 'content', to_jsonb(p), 'notebook', to_jsonb(b),
		'sessions', (SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.id), '[]'::jsonb)
			FROM edit_sessions s WHERE s.notebook_id = b.id))::text
		FROM nodes n JOIN page_contents p ON p.node_id = n.id JOIN notebooks b ON b.id = n.notebook_id
		WHERE n.id = $1`, id)[0]
}

// sourceUnchanged checks real REST responses and persisted bytes/metadata,
// including successful no-ops. It never replaces a production registrant.
func (tm acmeTeam) sourceUnchanged(t *testing.T, nb guardedNotebook, id string, c step, want cell) {
	t.Helper()
	before := tm.sourceStored(t, nb.id, id)
	tm.guarded(t, nb, c.name()+" as "+c.by, c, want)
	if after := tm.sourceStored(t, nb.id, id); after != before {
		t.Errorf("%s as %s changed durable source state", c.name(), c.by)
	}
}

// Removing mode from pageRegistrants' write guards makes the changed PUT,
// task and version restore succeed. Session registration is checked below
// separately, because an opening writes no changeset.
func TestSourceRESTWritesReachTheRegisteredGuard(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Sources")
	folder := tm.createPage(t, "bob", wiki.id, wiki.raw, "Folder")
	body := "- [ ] evidence\n"
	source := tm.createPageWith(t, "bob", wiki.id, folder, "Source", body)
	empty := tm.createPage(t, "bob", wiki.id, folder, "Empty")
	// Both historical contents existed before this page became a source.
	historical := tm.createPageWith(t, "alice", wiki.id, "", "History", "old")
	tm.send(t, contentWrite("alice", historical, "current", 1, ""), http.StatusOK)
	tm.send(t, nodeMove("alice", historical, folder), http.StatusOK)
	immutable := cell{http.StatusConflict, "page.source_immutable"}
	mismatch := cell{http.StatusConflict, "page.revision_mismatch"}
	for _, by := range []string{"bob", "alice"} {
		t.Run(by, func(t *testing.T) {
			for _, c := range []step{
				contentWrite(by, source, "changed", 1, ""),
				taskToggle(by, source, 1, 3, true),
				versionRestore(by, historical, 1, 2),
				versionRestore(by, historical, 2, 0),
				versionRestore(by, empty, 1, 0),
				sessionOpening(by, source),
				sessionOpening(by, empty),
			} {
				id := source
				switch c.path {
				case "/api/v0/pages/" + historical + "/versions/1/restore", "/api/v0/pages/" + historical + "/versions/2/restore":
					id = historical
				case "/api/v0/pages/" + empty + "/versions/1/restore", "/api/v0/pages/" + empty + "/edit-sessions":
					id = empty
				}
				tm.sourceUnchanged(t, wiki, id, c, immutable)
			}
			tm.sourceUnchanged(t, wiki, source, contentWrite(by, source, "changed", 0, ""), mismatch)
			tm.sourceUnchanged(t, wiki, historical, versionRestore(by, historical, 1, 1), mismatch)
			tm.sourceUnchanged(t, wiki, source, taskToggle(by, source, 0, 3, true), mismatch)
			tm.sourceUnchanged(t, wiki, source, taskToggle(by, source, 1, 2, true), cell{http.StatusUnprocessableEntity, "validation_failed"})
			tm.sourceUnchanged(t, wiki, source, contentWrite(by, source, body, 0, ""), cellOK())
			tm.sourceUnchanged(t, wiki, source, taskToggle(by, source, 1, 3, false), cellOK())
		})
	}
	if tm.sessionsOf(t, source) != 0 || tm.sessionsOf(t, empty) != 0 {
		t.Error("refused source openings created a session")
	}
	tm.send(t, contentWrite("bob", empty, "first content", 1, ""), http.StatusOK)
	tm.sourceUnchanged(t, wiki, empty, contentWrite("bob", empty, "second content", 2, ""), immutable)
	if got := tm.content(t, "bob", empty); got.Content != "first content" || got.Revision != 2 {
		t.Errorf("the source's first write = %+v, want first content at revision 2", got)
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}

// The strict-area boundary leaves root descriptions, AGENTS, and the same
// tree in standard mode editable, through the same real registrants.
func TestSourceRESTKeepsRootAndStandardWrites(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Sources")
	plain := tm.standardTwin(t, "Plain", wiki)
	ordinary := tm.createPageWith(t, "bob", plain.id, plain.raw, "Source-shaped", "before")
	for _, id := range []string{wiki.raw, wiki.agents, ordinary} {
		t.Run(id, func(t *testing.T) {
			first := tm.content(t, "bob", id)
			tm.send(t, contentWrite("bob", id, "- [ ] editable\n", first.Revision, ""), http.StatusOK)
			tm.send(t, taskToggle("bob", id, first.Revision+1, 3, true), http.StatusOK)
			tm.send(t, versionRestore("bob", id, first.Revision, first.Revision+2), http.StatusOK)
			session := tm.openSession(t, "bob", id)
			tm.send(t, contentWrite("bob", id, "saved in a session", first.Revision+3, session), http.StatusOK)
			if got := tm.content(t, "bob", id); got.Content != "saved in a session" || got.Revision != first.Revision+4 {
				t.Errorf("editable page = %+v, want session content at revision %d", got, first.Revision+4)
			}
		})
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}

// Removing the registered mode session veto makes source openings pass;
// registering [lock, mode] answers page.locked for the first openings.
// A takeover mutates the old session before the veto, so comparing every
// stored session field independently proves the transaction rolled back.
func TestSourceSessionsAreVetoedBeforeLocks(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Sessions")
	for _, body := range []string{"", "evidence"} {
		t.Run(map[bool]string{true: "empty", false: "nonempty"}[body == ""], func(t *testing.T) {
			id := tm.createPageWith(t, "bob", wiki.id, "", map[bool]string{true: "Empty session", false: "Written session"}[body == ""], body)
			session := tm.openSession(t, "bob", id)
			tm.send(t, nodeMove("alice", id, wiki.raw), http.StatusOK)
			immutable := cell{http.StatusConflict, "page.source_immutable"}
			ordinary := immutable
			if body == "" {
				ordinary = cell{http.StatusConflict, "page.locked"}
			}
			tm.sourceUnchanged(t, wiki, id, contentWrite("alice", id, "ordinary change", 1, ""), ordinary)
			tm.sourceUnchanged(t, wiki, id, contentWrite("alice", id, body, 0, ""), cellOK())
			for _, c := range []step{
				sessionOpening("alice", id), sessionOpening("bob", id), sessionTakeOver("bob", id),
				contentWrite("bob", id, body, 0, session), contentWrite("bob", id, "changed", 1, session),
			} {
				tm.sourceUnchanged(t, wiki, id, c, immutable)
			}
			tm.sourceUnchanged(t, wiki, id, contentWrite("bob", id, "changed", 0, session), cell{http.StatusConflict, "page.revision_mismatch"})
			for _, c := range []step{
				contentWrite("alice", id, body, 0, session),
				contentWrite("bob", id, body, 0, uuid.NewV7().String()),
			} {
				tm.sourceUnchanged(t, wiki, id, c, cell{http.StatusConflict, "page.edit_session_ended"})
			}
			if reason, exists := tm.endOf(t, session); !exists || reason != "" || tm.sessionsOf(t, id) != 1 {
				t.Errorf("source veto changed the existing session: exists=%t reason=%q", exists, reason)
			}
			tm.send(t, unlock("alice", id), http.StatusNoContent)
			tm.sourceUnchanged(t, wiki, id, contentWrite("bob", id, body, 0, session), cell{http.StatusConflict, "page.edit_session_unlocked"})
		})
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}

func TestSourceSessionVetoRollsBackExpiredCleanup(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Expired session")
	id := tm.createPage(t, "bob", wiki.id, "", "Empty")
	session := tm.openSession(t, "bob", id)
	tm.send(t, nodeMove("alice", id, wiki.raw), http.StatusOK)
	// testConfig's cleanup interval is an hour; no background cleanup can
	// remove this fixture while the rejected opening's own cleanup runs.
	if _, err := tm.pool.Exec(context.Background(), `UPDATE edit_sessions
		SET created_at = now() - interval '2 minutes', expires_at = now() - interval '1 minute' WHERE id = $1`, session); err != nil {
		t.Fatal(err)
	}
	tm.sourceUnchanged(t, wiki, id, sessionTakeOver("bob", id), cell{http.StatusConflict, "page.source_immutable"})
	if tm.sessionsOf(t, id) != 1 || count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE id = $1 AND expires_at < now() AND ended_reason IS NULL", session) != 1 {
		t.Error("the refused opening committed its expired-session deletion")
	}
	tm.sourceUnchanged(t, wiki, id, contentWrite("bob", id, "", 0, session), cell{http.StatusConflict, "page.edit_session_ended"})
	checkPages(t, tm.pool)
}

// interleaveOn observes real PostgreSQL row-lock waits, without sleeps:
// the move is queued first, then the opening has already found the page
// outside raw and is blocked in ShareNotebook. It must read its source
// ancestry only after the move commits. The reverse order proves the
// opening was otherwise valid and leaves a real old session to veto.
func TestSourceSessionChecksAfterTheNotebookLock(t *testing.T) {
	for _, moveFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "move first", false: "open first"}[moveFirst], func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Interleaving")
			id := tm.createPageWith(t, "bob", wiki.id, "", "Outside", "evidence")
			before := tm.content(t, "bob", id)
			sets := tm.changesetsIn(t, wiki.id)
			move, open := nodeMove("alice", id, wiki.raw), sessionOpening("bob", id)
			var moved, opened answer
			if moveFirst {
				moved, opened = tm.interleaveOn(t, notebookRow(wiki.id), move, open)
			} else {
				opened, moved = tm.interleaveOn(t, notebookRow(wiki.id), open, move)
			}
			if !moved.is(http.StatusOK, "") {
				t.Fatalf("move = %d %s", moved.status, moved.body)
			}
			if moveFirst {
				if !opened.is(http.StatusConflict, "page.source_immutable") || tm.sessionsOf(t, id) != 0 {
					t.Fatalf("opening after the move = %d %s, want immutable and no session", opened.status, opened.body)
				}
			} else {
				if !opened.is(http.StatusCreated, "") || tm.sessionsOf(t, id) != 1 {
					t.Fatalf("opening before the move = %d %s, want one session", opened.status, opened.body)
				}
				tm.sourceUnchanged(t, wiki, id, contentWrite("bob", id, before.Content, 0, idOf(t, opened)), cell{http.StatusConflict, "page.source_immutable"})
			}
			if got := tm.content(t, "bob", id); got != before || tm.changesetsIn(t, wiki.id) != sets+1 {
				t.Errorf("session interleaving wrote content or a changeset: before=%+v after=%+v", before, got)
			}
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2", id, wiki.raw) != 1 {
				t.Error("the move did not put the page under the source root")
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}
