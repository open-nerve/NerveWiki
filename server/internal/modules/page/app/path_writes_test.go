package app_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lockPaths is err's LockPaths and its shared.Error, a lock's refusal.
func lockPaths(err error) (*domain.LockPaths, *shared.Error, bool) {
	var paths *domain.LockPaths
	var e *shared.Error
	ok := errors.As(err, &paths) && errors.As(err, &e) && errors.Is(err, domain.ErrLocked)
	return paths, e, ok
}

// A refusal of edit locks carries the paths of its pages, read by id once
// the unit is over, unlocked (M9/P2 design 3.8): the guard's refusal of a
// write, and an agent's deletion naming each alive session under it, in
// the order of what the deletion changes, the first MaxNamedLocks of them
// read in one call. edit's precheck is EditPage's own test.
func TestALockRefusalCarriesItsPagesPaths(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite, domain.ActionDelete)
	bob := uuid.NewV7()
	f.names[f.alice], f.names[bob] = "Alice", "Bob"
	f.guards = []app.WriteGuard{app.NewEditLock(f.store, f.names)}
	top := f.page("Top", nil, 0)
	mid := f.page("Mid", &top.ID, 0)
	low := f.page("Low", &mid.ID, 0)
	f.session(top.ID, f.alice, now().Add(time.Minute))
	f.session(low.ID, bob, now().Add(time.Minute))

	_, err := f.writePath(f.eng, app.PathWrite{Path: "top.md", Content: "new", Base: tokenOf(top.ID, 1)})
	if paths, e, ok := lockPaths(err); !ok || e.Lock.PageID != top.ID || len(paths.Paths) != 1 || paths.Paths[top.ID] != "Top.md" ||
		!f.called("PathsOf 1") || f.rec.calls[len(f.rec.calls)-1] != "PathsOf 1" {
		t.Errorf("a write of a page in alice's session = %v after %v, want page.locked with Top's path read last", err, f.rec.calls)
	}

	_, err = f.deletePath(f.eng, app.PathDelete{Path: "Top.md", Recursive: true})
	if paths, e, ok := lockPaths(err); !ok || len(e.Locks) != 2 || e.Locks[0].PageID != top.ID || e.Locks[1].PageID != low.ID ||
		paths.Paths[top.ID] != "Top.md" || paths.Paths[low.ID] != "Top/Mid/Low.md" || !f.called("PathsOf 2") {
		t.Errorf("a deletion over two sessions = %v, want page.locked naming both with their paths", err)
	}

	many := f.page("Many", nil, 1)
	for i := range domain.MaxNamedLocks + 1 {
		p := f.page(fmt.Sprintf("P%02d", i), &many.ID, float64(i))
		f.session(p.ID, bob, now().Add(time.Minute))
	}
	f.rec.calls = nil
	_, err = f.deletePath(f.eng, app.PathDelete{Path: "Many.md", Recursive: true})
	if paths, e, ok := lockPaths(err); !ok || len(e.Locks) != domain.MaxNamedLocks+1 || len(paths.Paths) != domain.MaxNamedLocks ||
		!f.called(fmt.Sprintf("PathsOf %d", domain.MaxNamedLocks)) || paths.Paths[e.Locks[0].PageID] != "Many/P00.md" {
		t.Errorf("a deletion over 21 sessions = %v after %v, want all named, the first 20 with their paths", err, f.rec.calls)
	}
}

// When the paths cannot be read the refusal is answered all the same,
// page.locked with its holder, no page with a path, and a warn line says
// why with how many pages it was to read (M9/P2 design 3.8).
func TestALockRefusalWhosePathsFailIsStillTheRefusal(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	f.names[f.alice] = "Alice"
	notes := f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	f.session(notes.ID, f.alice, now().Add(time.Minute))
	f.store.errs = map[string]error{"PathsOf": errors.New("the pool is closed")}
	_, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2"})
	paths, e, ok := lockPaths(err)
	if !ok || paths.Paths != nil || e.Lock.DisplayName != "Alice" {
		t.Errorf("an edit whose lock paths fail = %v, want page.locked by Alice with no paths", err)
	}
	if logs := f.logs.String(); !strings.Contains(logs, `level=WARN msg="lock paths not read"`) ||
		!strings.Contains(logs, `error="the pool is closed"`) || !strings.Contains(logs, "pages=1") {
		t.Errorf("logs = %s, want a warn line with the error and the pages", logs)
	}
}
