package bootstrap

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The interleavings of the edit lock (M5/P1 design 3.8), harnessed as the
// content's (interleavings_content_test.go): a take-over, a forced unlock,
// a save and an opening pass a page's gate, its content row; a heartbeat
// and the end of a take-over or an unlock meet on the session's row; a
// tree's write and an opening meet on the notebook's row. Each runs both
// orders and ends by checking the pages' invariant, at most one alive
// session a page among it, and the notebooks'.

// sessionTakeOver is by's opening of an edit session of the page id that
// takes their own over.
func sessionTakeOver(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/pages/"+id+"/edit-sessions", `{"take_over":true}`)
}

// unlock is by's forced unlock of the page id.
func unlock(by, id string) step {
	return request(by, http.MethodDelete, "/api/v0/pages/"+id+"/edit-lock", "")
}

// heartbeat is by's heartbeat of the edit session id.
func heartbeat(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/edit-sessions/"+id+"/heartbeat", "")
}

// endOf is the session id's end as its row tells it: its reason, "" for
// none, and whether its row is there.
func (tm acmeTeam) endOf(t *testing.T, id string) (reason string, there bool) {
	t.Helper()
	n := count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE id = $1", id)
	if n == 0 {
		return "", false
	}
	return queryStrings(t, tm.pool, "SELECT coalesce(ended_reason, '') FROM edit_sessions WHERE id = $1", id)[0], true
}

// endedBy is the name in a problem's ended_by member, "" for none.
func endedBy(a answer) string {
	var p struct {
		EndedBy struct {
			DisplayName string `json:"display_name"`
		} `json:"ended_by"`
	}
	_ = json.Unmarshal([]byte(a.body), &p)
	return p.EndedBy.DisplayName
}

// lockHolder is the name in a problem's lock member, "" for none.
func lockHolder(a answer) string {
	var p struct {
		Lock struct {
			DisplayName string `json:"display_name"`
		} `json:"lock"`
	}
	_ = json.Unmarshal([]byte(a.body), &p)
	return p.Lock.DisplayName
}

// lockedPage is a page of a notebook open to acme's members, with bob's
// session, and the session.
func lockedPage(t *testing.T, tm acmeTeam) (nb, id, session string) {
	t.Helper()
	nb = tm.openNotebook(t, "alice", "Eng")
	id = tm.createPage(t, "alice", nb, "", "Notes")
	return nb, id, tm.openSession(t, "bob", id)
}

// Interleaving 45: bob takes his session over while a save in it waits at
// the page's gate. The save first: it goes to the session's changeset,
// then the take-over ends the session. The take-over first: the save finds
// it taken over.
func TestTakingOverASessionASaveWaitsIn(t *testing.T) {
	for _, saveFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "the save first", false: "the take-over first"}[saveFirst], func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			_, id, session := lockedPage(t, tm)
			save, take := contentWrite("bob", id, "# One", 1, session), sessionTakeOver("bob", id)
			if saveFirst {
				saved, taken := tm.interleaveOn(t, contentRow(id), save, take)
				if !saved.is(http.StatusOK, "") || !taken.is(http.StatusCreated, "") {
					t.Errorf("the save = %d %s, then the take-over = %d %s; want 200, then 201", saved.status, saved.code, taken.status, taken.code)
				}
				if got := tm.content(t, "bob", id); got.Revision != 2 {
					t.Errorf("revision %d, want the save's 2", got.Revision)
				}
			} else {
				taken, saved := tm.interleaveOn(t, contentRow(id), take, save)
				if !taken.is(http.StatusCreated, "") || !saved.is(http.StatusConflict, "page.edit_session_taken_over") {
					t.Errorf("the take-over = %d %s, then the save = %d %s; want 201, then 409 page.edit_session_taken_over",
						taken.status, taken.code, saved.status, saved.code)
				}
			}
			if reason, _ := tm.endOf(t, session); reason != "taken_over" {
				t.Errorf("the old session ended %q, want taken_over", reason)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 46: alice, the notebook's admin, releases the page's lock
// while bob's save in his session waits at the page's gate. The save
// first: it is written, then the unlock ends the session. The unlock
// first: the save finds it unlocked, by alice.
func TestUnlockingASessionASaveWaitsIn(t *testing.T) {
	for _, saveFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "the save first", false: "the unlock first"}[saveFirst], func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			_, id, session := lockedPage(t, tm)
			save, release := contentWrite("bob", id, "# One", 1, session), unlock("alice", id)
			if saveFirst {
				saved, released := tm.interleaveOn(t, contentRow(id), save, release)
				if !saved.is(http.StatusOK, "") || !released.is(http.StatusNoContent, "") {
					t.Errorf("the save = %d %s, then the unlock = %d %s; want 200, then 204", saved.status, saved.code, released.status,
						released.code)
				}
			} else {
				released, saved := tm.interleaveOn(t, contentRow(id), release, save)
				if !released.is(http.StatusNoContent, "") || !saved.is(http.StatusConflict, "page.edit_session_unlocked") ||
					endedBy(saved) != "alice" {
					t.Errorf("the unlock = %d %s, then the save = %d %s; want 204, then 409 page.edit_session_unlocked by alice",
						released.status, released.code, saved.status, saved.body)
				}
			}
			if reason, _ := tm.endOf(t, session); reason != "unlocked" {
				t.Errorf("the session ended %q, want unlocked", reason)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleavings 47 and 48: bob's heartbeat while his session is taken over
// by him elsewhere, or unlocked by alice: both update the session's row,
// which the test holds. The heartbeat first: it keeps the session, which
// the end then ends. The end first: the heartbeat waits for the row, finds
// it ended and says why, and keeps nothing alive.
func TestABeatAndAnEndOfItsSession(t *testing.T) {
	for _, tt := range []struct {
		name   string
		end    func(id string) step
		status int
		reason string
		code   string
	}{
		{"a take-over", func(id string) step { return sessionTakeOver("bob", id) }, http.StatusCreated, "taken_over", "page.edit_session_taken_over"},
		{"an unlock", func(id string) step { return unlock("alice", id) }, http.StatusNoContent, "unlocked", "page.edit_session_unlocked"},
	} {
		for _, beatFirst := range []bool{true, false} {
			t.Run(tt.name+map[bool]string{true: ", the heartbeat first", false: ", the end first"}[beatFirst], func(t *testing.T) {
				tm := newAcmeTeam(t, "member", "")
				_, id, session := lockedPage(t, tm)
				beat, end := heartbeat("bob", session), tt.end(id)
				var beaten, ended answer
				if beatFirst {
					beaten, ended = tm.interleaveOn(t, sessionRow(session), beat, end)
				} else {
					ended, beaten = tm.interleaveOn(t, sessionRow(session), end, beat)
				}
				wantBeat := map[bool]answer{true: {status: http.StatusOK}, false: {status: http.StatusConflict, code: tt.code}}[beatFirst]
				if !beaten.is(wantBeat.status, wantBeat.code) || !ended.is(tt.status, "") {
					t.Errorf("the heartbeat = %d %s, the end = %d %s; want %d %s and %d", beaten.status, beaten.code, ended.status,
						ended.code, wantBeat.status, wantBeat.code, tt.status)
				}
				if reason, _ := tm.endOf(t, session); reason != tt.reason {
					t.Errorf("the session ended %q, want %s", reason, tt.reason)
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM edit_sessions WHERE id = $1 AND expires_at > ended_at + interval '121 seconds'`,
					session); !beatFirst && n != 0 {
					t.Error("the heartbeat behind the end lengthened the tombstone's lease")
				}
				checkPages(t, tm.pool)
				checkNotebooks(t, tm.pool)
			})
		}
	}
}

// Interleaving 50: bob deletes a page while alice opens its session, both
// waiting for the notebook's row. The deletion first: 404 page.not_found.
// The opening first: the deletion finds alice's lock, 409 page.locked by
// her, and the page stays.
func TestDeletingAPageSomeoneOpens(t *testing.T) {
	for _, deletionFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "the deletion first", false: "the opening first"}[deletionFirst], func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			id := tm.createPage(t, "alice", nb, "", "Notes")
			deletion, opening := nodeDeletion("bob", id), sessionOpening("alice", id)
			if deletionFirst {
				deleted, opened := tm.interleaveOn(t, notebookRow(nb), deletion, opening)
				if !deleted.is(http.StatusNoContent, "") || !opened.is(http.StatusNotFound, "page.not_found") || tm.sessionsOf(t, id) != 0 {
					t.Errorf("the deletion = %d, then the opening = %d %s, %d sessions; want 204, then 404 page.not_found, none",
						deleted.status, opened.status, opened.code, tm.sessionsOf(t, id))
				}
			} else {
				opened, deleted := tm.interleaveOn(t, notebookRow(nb), opening, deletion)
				if !opened.is(http.StatusCreated, "") || !deleted.is(http.StatusConflict, "page.locked") || lockHolder(deleted) != "alice" ||
					tm.sessionsOf(t, id) != 1 {
					t.Errorf("the opening = %d %s, then the deletion = %d %s, %d sessions; want 201, then 409 page.locked by alice, one",
						opened.status, opened.code, deleted.status, deleted.body, tm.sessionsOf(t, id))
				}
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 51: alice writes the page without a session while bob opens
// one, both at the page's gate. The write first: the content changes, and
// the opening follows. The opening first: the write finds bob's lock, 409
// page.locked by him.
func TestWritingAPageSomeoneOpens(t *testing.T) {
	for _, writeFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "the write first", false: "the opening first"}[writeFirst], func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			id := tm.createPage(t, "alice", nb, "", "Notes")
			write, opening := contentWrite("alice", id, "# One", 1, ""), sessionOpening("bob", id)
			if writeFirst {
				written, opened := tm.interleaveOn(t, contentRow(id), write, opening)
				if !written.is(http.StatusOK, "") || !opened.is(http.StatusCreated, "") || tm.content(t, "bob", id).Revision != 2 {
					t.Errorf("the write = %d %s, then the opening = %d %s; want 200, then 201, revision 2", written.status, written.code,
						opened.status, opened.code)
				}
			} else {
				opened, written := tm.interleaveOn(t, contentRow(id), opening, write)
				if !opened.is(http.StatusCreated, "") || !written.is(http.StatusConflict, "page.locked") || lockHolder(written) != "bob" ||
					tm.content(t, "bob", id).Revision != 1 {
					t.Errorf("the opening = %d %s, then the write = %d %s; want 201, then 409 page.locked by bob, revision 1", opened.status,
						opened.code, written.status, written.body)
				}
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}
