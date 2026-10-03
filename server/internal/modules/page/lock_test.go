package page_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// earlier is a clock a minute before testNow.
type earlier struct{}

func (earlier) Now() time.Time { return testNow().Add(-time.Minute) }

// holdingVetoer tells reached when an opening reaches it, then holds the
// opening until release closes: its deletion of the page's expired rows
// is done, uncommitted.
type holdingVetoer struct {
	reached, release chan struct{}
}

func (v holdingVetoer) VetoEditSession(context.Context, page.SessionOpening) error {
	close(v.reached)
	<-v.release
	return nil
}

// page.NewEditLock over the pool, as the opening's vetoer, refuses a
// second opening of a page: 409 page.locked with the lock member, its
// holder named by Deps.Names, and no second session.
func TestTheEditLockRefusesASecondOpening(t *testing.T) {
	f := newFixture(t)
	f.vetoers = []page.EditSessionVetoer{page.NewEditLock(f.pool, sqlNames{f.pool})}
	f.openSession(t)
	rec := f.serve(t, "pat", http.MethodPost, "/api/v0/pages/"+f.notes.String()+"/edit-sessions", "", nil, nil, nil)
	var p struct {
		Code string `json:"code"`
		Lock struct {
			PageID      uuid.UUID `json:"page_id"`
			UserID      uuid.UUID `json:"user_id"`
			DisplayName string    `json:"display_name"`
		} `json:"lock"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusConflict || p.Code != "page.locked" ||
		p.Lock.PageID != f.notes || p.Lock.UserID != f.alice || p.Lock.DisplayName != "Alice" {
		t.Errorf("a second opening = %d %s, want 409 page.locked by Alice", rec.Code, rec.Body)
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions"); n != 1 {
		t.Errorf("%d edit sessions, want the first alone", n)
	}
}

// Interleaving 49 (M5/P1 design 3.8), in its one order: a heartbeat read
// alice's session alive, at a time before it expired, and waits on its
// row, which an opening at a later time has deleted as expired, under the
// page's gate. The opening commits: the heartbeat finds no row to keep
// alive, page.edit_session_not_found, and the opening's session is the
// page's one alive. The whole program has one clock; here each request
// has its module's, on one database, and a vetoer holds the opening after
// its deletion.
func TestALateHeartbeatFindsItsRowDeleted(t *testing.T) {
	f := newFixture(t)
	late := uuid.NewV7()
	f.exec(t, "INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at) "+
		"VALUES ($1, $2, $3, $4, 'web', $5, $6)", late, f.notes, f.eng, f.alice, testNow().Add(-2*time.Minute), testNow())
	v := holdingVetoer{reached: make(chan struct{}), release: make(chan struct{})}
	f.vetoers = []page.EditSessionVetoer{v}
	// The opening holds a connection until it is released: a test that
	// fails before must release it, or closing the pool waits for it.
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(v.release) }) })
	beater := f
	beater.vetoers, beater.clock = nil, earlier{}
	opener, beats := f.router(t, nil, nil, nil), beater.router(t, nil, nil, nil)
	opened := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		opened <- f.request(opener, "session", http.MethodPost, "/api/v0/pages/"+f.notes.String()+"/edit-sessions", "")
	}()
	select {
	case <-v.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the opening did not reach its vetoer")
	}
	beaten := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		beaten <- f.request(beats, "session", http.MethodPost, "/api/v0/edit-sessions/"+late.String()+"/heartbeat", "")
	}()
	pgtest.WaitForLockWaitsOn(t, f.pool, "edit_sessions", 1, 10*time.Second)
	release.Do(func() { close(v.release) })

	if rec := <-opened; rec.Code != http.StatusCreated {
		t.Errorf("the opening = %d %s, want 201", rec.Code, rec.Body)
	}
	if rec := <-beaten; rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"page.edit_session_not_found"`) {
		t.Errorf("the late heartbeat = %d %s, want 404 page.edit_session_not_found", rec.Code, rec.Body)
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1", late); n != 0 {
		t.Error("the late heartbeat kept its session")
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions WHERE ended_reason IS NULL AND expires_at > $1", testNow()); n != 1 {
		t.Errorf("%d alive sessions, want the opening's alone", n)
	}
}

// holdingSubscriber tells reached when an end reaches it, then holds the
// unit until release closes: its deletion of the page's expired rows is
// done, uncommitted.
type holdingSubscriber struct {
	reached, release chan struct{}
}

func (holdingSubscriber) EditSessionOpened(context.Context, page.SessionOpened) error { return nil }

func (s holdingSubscriber) EditSessionEnded(context.Context, page.SessionEnded) error {
	close(s.reached)
	<-s.release
	return nil
}

// An unlock deletes the page's expired rows first, as an opening does
// (M5/P1 review m1): a heartbeat that read alice's expired session alive,
// at a time before it expired, waits on its row; the unlock ends her other
// session and commits, and the heartbeat finds no row to keep alive,
// page.edit_session_not_found, nor a lock to hold beside the unlock.
func TestALateHeartbeatFindsItsRowDeletedByAnUnlock(t *testing.T) {
	f := newFixture(t)
	late, held := uuid.NewV7(), uuid.NewV7()
	insert := "INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7)"
	f.exec(t, insert, late, f.notes, f.eng, f.alice, "web", testNow().Add(-2*time.Minute), testNow())
	f.exec(t, insert, held, f.notes, f.eng, f.alice, "api", testNow().Add(-time.Minute), testNow().Add(time.Minute))
	sub := holdingSubscriber{reached: make(chan struct{}), release: make(chan struct{})}
	f.subscribers = []page.EditSessionSubscriber{sub}
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(sub.release) }) })
	beater := f
	beater.subscribers, beater.clock = nil, earlier{}
	unlocker, beats := f.router(t, nil, nil, nil), beater.router(t, nil, nil, nil)
	unlocked := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		unlocked <- f.request(unlocker, "session", http.MethodDelete, "/api/v0/pages/"+f.notes.String()+"/edit-lock", "")
	}()
	select {
	case <-sub.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the unlock did not reach its subscriber")
	}
	beaten := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		beaten <- f.request(beats, "session", http.MethodPost, "/api/v0/edit-sessions/"+late.String()+"/heartbeat", "")
	}()
	pgtest.WaitForLockWaitsOn(t, f.pool, "edit_sessions", 1, 10*time.Second)
	release.Do(func() { close(sub.release) })

	if rec := <-unlocked; rec.Code != http.StatusNoContent {
		t.Errorf("the unlock = %d %s, want 204", rec.Code, rec.Body)
	}
	if rec := <-beaten; rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"page.edit_session_not_found"`) {
		t.Errorf("the late heartbeat = %d %s, want 404 page.edit_session_not_found", rec.Code, rec.Body)
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1", late); n != 0 {
		t.Error("the late heartbeat kept its session")
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1 AND ended_reason = 'unlocked'", held); n != 1 {
		t.Error("the unlock did not end the alive session")
	}
}
