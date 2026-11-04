package bootstrap

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
	"uuid"
)

func testStableIndexScope(t *testing.T) {
	interleave := &ingestInterleaveControl{}
	control := &stableIndexControl{}
	w := stableIndexWire(t, interleave.wiring(t), control)
	f := newIngestFixture(t, &w)
	if control.pool.Config().MaxConns < 3 {
		t.Fatal("stable scope requires three original app-pool connections")
	}
	q := f.tm.createPage(t, "alice", f.nb, "", "Q")
	f.write(t, f.p, "[[raw/S]] old\n")
	f.write(t, q, "[[raw/S]] old\n")
	// A second notebook on this SAME app and database, actually in LLM mode.
	book := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Stable independent notebook","mode":"llm_wiki"}`)
	if book.Mode != "llm_wiki" || book.SourceRootID == nil {
		t.Fatalf("stable second notebook is not real LLM: %+v", book)
	}
	uID := f.tm.createPageWith(t, "alice", book.ID, *book.SourceRootID, "U", "Independent U evidence.\n")
	vID := f.tm.createPageWith(t, "alice", book.ID, *book.SourceRootID, "V", "Independent V evidence.\n")
	p2 := f.tm.createPage(t, "alice", book.ID, "", "P2")
	f2 := ingestFixture{tm: f.tm, nb: book.ID, raw: *book.SourceRootID, s: uID, t: vID, p: p2}
	f2.write(t, p2, "[[raw/U]] old\n")
	P, Q, N, S, T, P2, N2, U, V := uuid.MustParse(f.p), uuid.MustParse(q), uuid.MustParse(f.nb), uuid.MustParse(f.s), uuid.MustParse(f.t), uuid.MustParse(p2), uuid.MustParse(book.ID), uuid.MustParse(uID), uuid.MustParse(vID)
	stableIndexDistinct(t, P, Q, N, S, T, P2, N2, U, V)
	_, key1 := ingestIndexKey(N)
	_, key2 := ingestIndexKey(N2)
	if key1 == key2 {
		t.Fatal("stable independent notebook fixture has colliding advisory keys")
	}
	oldP := []stableWiringRow{stableIndexRow(P, N, 2, 7, "wikilink", "raw/S", "s", S, false, false)}
	oldQ := []stableWiringRow{stableIndexRow(Q, N, 2, 7, "wikilink", "raw/S", "s", S, false, false)}
	oldP2 := []stableWiringRow{stableIndexRow(P2, N2, 2, 7, "wikilink", "raw/U", "u", U, false, false)}
	stableIndexInitial(t, f.tm, P, oldP)
	stableIndexInitial(t, f.tm, Q, oldQ)
	stableIndexInitial(t, f.tm, P2, oldP2)
	const aBody = "[[raw/S]] new\n[[raw/T]]\n"
	const bBody = "[[raw/S]] new\n"
	const cBody = "[[raw/U]] new\n[[raw/V]]\n"
	a := newIngestInterleaveUnit(t, f.p, q, aBody)
	b := newIngestInterleaveUnit(t, q, f.p, bBody)
	c := newIngestInterleaveUnit(t, p2, uID, cBody)
	a.revision, b.revision, c.revision = 3, 3, 3
	// Keep the original observers/coverage reads, but their old coverage gates
	// are already open. This one case gates only A's actual retained Replace.
	a.release()
	b.release()
	c.release()
	interleave.arm(a, b, c)
	au := newStableIndexUnit(f.p, f.nb, 3, aBody)
	bu := newStableIndexUnit(q, f.nb, 3, bBody)
	cu := newStableIndexUnit(p2, book.ID, 3, cBody)
	au.afterReplace, au.allowFresh = make(chan struct{}), make(chan struct{})
	control.arm(au, bu, cu)
	f1i := ingestInterleaveFixture{tm: f.tm, nb: f.nb, root: f.raw, s: f.s, t: f.t, p: f.p, q: q}
	f2i := ingestInterleaveFixture{tm: f.tm, nb: book.ID, root: *book.SourceRootID, s: uID, t: vID, p: p2, q: uID}
	listener := f.tm.originListen(t)
	var pending []*ingestPending
	t.Cleanup(func() {
		au.release()
		for _, p := range pending {
			select {
			case <-p.done:
			case <-time.After(interleavingWait):
				t.Error("stable owned writer did not finish during cleanup")
			}
		}
	})
	aDone := ingestStartWrite(t, f1i, a)
	pending = append(pending, aDone)
	aw := ingestWritten(t, f1i, a, aDone)
	ingestNext(t, "A after actual retained Replace", au.afterReplace, aDone)
	au.mu.Lock()
	stableIndexCheckInspect(t, au, len(oldP))
	if au.inspectCalls != 1 || !au.inspectAllowed || len(au.replaced) != 2 || au.replaced[0].Resolved == nil || *au.replaced[0].Resolved != S || au.replaced[1].Resolved != nil || stableIndexStage(au, "links") != 0 {
		t.Errorf("stable A gate lacks real retained baseline: Inspect=%d allowed=%t rows=%+v stages=%+v", au.inspectCalls, au.inspectAllowed, au.replaced, au.stages)
	}
	au.mu.Unlock()
	bDone := ingestStartWrite(t, f1i, b)
	pending = append(pending, bDone)
	bw := ingestWritten(t, f1i, b, bDone)
	if aw.pid == bw.pid {
		t.Fatal("stable same-notebook writers share a PID")
	}
	ingestWaitIndex(t, f1i, b, bw, aw.pid, bDone)
	ingestAssertWriterLocks(t, f1i, a.node, b.node)
	var blockedByA bool
	if err := f.tm.pool.QueryRow(t.Context(), `SELECT $1::integer=ANY(pg_blocking_pids($2::integer))`, aw.pid, bw.pid).Scan(&blockedByA); err != nil || !blockedByA {
		t.Fatalf("stable exact B->A PG blocker=%t err=%v", blockedByA, err)
	}
	bu.mu.Lock()
	beforeInspect := bu.inspectCalls
	bu.mu.Unlock()
	if beforeInspect != 0 {
		t.Fatal("stable B inspected while A held the same notebook lock")
	}
	cDone := ingestStartWrite(t, f2i, c)
	pending = append(pending, cDone)
	cw := ingestWritten(t, f2i, c, cDone)
	if cw.pid == aw.pid || cw.pid == bw.pid {
		t.Fatal("stable other-book writer did not use its own live connection")
	}
	ingestAnswer(t, f2i, c, cDone, http.StatusOK)
	select {
	case <-aDone.done:
		t.Fatal("stable A finished before its owned gate was released")
	default:
	}
	select {
	case <-bDone.done:
		t.Fatal("stable B finished before A released the notebook lock")
	default:
	}
	ingestWaitIndex(t, f1i, b, bw, aw.pid, bDone)
	bu.mu.Lock()
	beforeInspect = bu.inspectCalls
	bu.mu.Unlock()
	if beforeInspect != 0 {
		t.Fatal("stable B inspected before the held A ended")
	}
	cReplaced := []stableWiringRow{oldP2[0], stableIndexRow(P2, N2, 16, 21, "wikilink", "raw/V", "v", uuid.Nil(), false, false)}
	cFinal := []stableWiringRow{oldP2[0], stableIndexRow(P2, N2, 16, 21, "wikilink", "raw/V", "v", V, false, false)}
	stableIndexCheck(t, f.tm, cu, stableIndexWant{before: oldP2, replaced: cReplaced, final: cFinal, changed: stableIndexLiteralLinks(cFinal[1:]), dropped: []uuid.UUID{U}, keys: []string{"u", "v"}})
	assertIngestMarks(t, f.tm, ingestCS(t, f.tm, p2, 3), vID)
	stableIndexNotificationUnits(t, ingestNotifications(t, f1i, listener), []stableIndexNotificationWant{{book.ID, p2, []string{uID, vID}}})
	// C has committed through its real Index while A is still held and B still
	// waits. Only now let A finish; B's Inspect must follow A's original commit.
	au.release()
	ingestAnswer(t, f1i, a, aDone, http.StatusOK)
	ingestAnswer(t, f1i, b, bDone, http.StatusOK)
	control.arm()
	interleave.arm()
	aReplaced := []stableWiringRow{oldP[0], stableIndexRow(P, N, 16, 21, "wikilink", "raw/T", "t", uuid.Nil(), false, false)}
	aFinal := []stableWiringRow{oldP[0], stableIndexRow(P, N, 16, 21, "wikilink", "raw/T", "t", T, false, false)}
	stableIndexCheck(t, f.tm, au, stableIndexWant{before: oldP, replaced: aReplaced, final: aFinal, changed: stableIndexLiteralLinks(aFinal[1:]), dropped: []uuid.UUID{S}, keys: []string{"s", "t"}})
	stableIndexCheck(t, f.tm, bu, stableIndexWant{before: oldQ, replaced: oldQ, final: oldQ, dropped: []uuid.UUID{S}, keys: []string{"s"}})
	assertIngestMarks(t, f.tm, ingestCS(t, f.tm, f.p, 3), f.t)
	assertIngestMarks(t, f.tm, ingestCS(t, f.tm, q, 3))
	stableIndexNotificationUnits(t, ingestNotifications(t, f1i, listener), []stableIndexNotificationWant{{f.nb, f.p, []string{f.s, f.t}}, {f.nb, q, []string{f.s}}})
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	f.tm.checkRebuilt(t, f.nb)
	f.tm.checkRebuilt(t, book.ID)
}

type stableIndexNotificationWant struct {
	notebook, page string
	targets        []string
}

func stableIndexNotificationUnits(t *testing.T, got []json.RawMessage, want []stableIndexNotificationWant) {
	t.Helper()
	if len(got) != 2*len(want) {
		t.Fatalf("stable committed notifications=%s want%d units", got, len(want))
	}
	for i, w := range want {
		for j, kind := range []string{"pages", "links"} {
			var event struct {
				Type     string          `json:"type"`
				Notebook string          `json:"notebook_id"`
				Data     json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(got[2*i+j], &event); err != nil || event.Type != kind || event.Notebook != w.notebook {
				t.Fatalf("stable notification envelope=%s err=%v", got[2*i+j], err)
			}
			if kind == "pages" {
				var data struct {
					Tree  bool `json:"tree"`
					Pages []struct {
						ID       string `json:"id"`
						Revision int    `json:"revision"`
					} `json:"pages"`
				}
				if err := json.Unmarshal(event.Data, &data); err != nil || data.Tree || len(data.Pages) != 1 || data.Pages[0].ID != w.page || data.Pages[0].Revision != 3 {
					t.Fatalf("stable committed pages=%s err=%v", event.Data, err)
				}
			} else {
				var data struct {
					Pages   *[]string `json:"pages"`
					Targets *[]string `json:"targets"`
					Aliases bool      `json:"aliases"`
				}
				if err := json.Unmarshal(event.Data, &data); err != nil || !sameSet(data.Pages, []string{}) || !sameSet(data.Targets, w.targets) || data.Aliases {
					t.Fatalf("stable committed links=%s err=%v want targets=%v", event.Data, err, w.targets)
				}
			}
		}
	}
}
