package app_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func rewriteWindowSetContent(t *testing.T, w *rewriting, id uuid.UUID, content string) {
	t.Helper()
	for name, pageID := range w.tree.names {
		if pageID == id {
			w.write(name, content)
			return
		}
	}
	t.Fatal("fixture source is absent")
}

func TestRewriteWindowOptionalRefusalKeepsOriginalBusy(t *testing.T) {
	for _, stage := range []string{"old", "readback"} {
		t.Run(stage, func(t *testing.T) {
			w, ids, old := rewriteWindowFixture(t, 1)
			old[0] = "[[A]]\n" + strings.Repeat("x", 4100-6)
			rewriteWindowSetContent(t, w, ids[0], old[0])
			w.budget = markdown.NewBudget(8<<20, time.Hour, slog.New(slog.NewTextHandler(w.logs, nil)))
			w.rewrite.MaxContent = 5 << 20
			p := rewriteWindowUseParser(t, w, nil)
			contents := &rewriteWindowContents{PageContents: w.contents, reads: map[uuid.UUID]int{}}
			w.rewrite.Contents = contents
			var other *markdown.Hold
			if stage == "old" {
				var err error
				other, err = w.budget.TakeNow(t.Context(), 8<<20)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				// Old owns 4100 credits. Taking the exact rest makes the 249-byte
				// optional growth fail; closing old then frees 4100, not the 4349
				// that the real ordinary readback needs. Its busy must not be retried.
				p.beforeGrow = func(int) {
					var err error
					other, err = w.budget.TakeNow(t.Context(), (8<<20)-4100)
					if err != nil {
						panic(err)
					}
				}
			}
			t.Cleanup(func() {
				if other != nil {
					other.Release()
				}
			})
			target := strings.Repeat("b", 250)
			now := strings.ReplaceAll(old[0], "[[A]]", "[["+target+"]]")
			u := &rewriteWindowAppender{}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err := w.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true,
				Changes: []domain.Change{w.rename("A", target)}, MaxBytes: 1 << 20}, u)
			var problem *shared.Error
			active, _, reserves := p.state()
			if !errors.As(err, &problem) || problem.Code != "server_busy" || ctx.Err() != nil || active != 0 || reserves != 1 || len(u.writes) != 0 || contents.reads[ids[0]] != 1 {
				t.Fatalf("WINDOW_OPTIONAL_BUSY: %s err%v active%d reservations%d writes%d reads%d", stage, err, active, reserves, len(u.writes), contents.reads[ids[0]])
			}
			if p.calls[old[0]] != 1 || p.calls[now] != map[string]int{"old": 0, "readback": 1}[stage] || p.usedReservation(now) {
				t.Fatal("WINDOW_BUSY_RETRY: optional refusal swallowed or retried an actual parse")
			}
			if strings.Count(w.logs.String(), "content parsing is saturated") != 1 {
				t.Fatal("optional admission logged or real busy was repeated")
			}
			other.Release()
			rewriteWindowReleases(t, w, u)
		})
	}
}

func TestRewriteWindowLargestPageAndZeroShareOriginalBudget(t *testing.T) {
	w, ids, old := rewriteWindowFixture(t, 2)
	old[1] = "[[A]]\n" + strings.Repeat("r", (5<<20)-6)
	rewriteWindowSetContent(t, w, ids[1], old[1])
	w.budget = markdown.NewBudget(8<<20, time.Hour, slog.New(slog.DiscardHandler))
	w.rewrite.MaxContent = 5 << 20
	p := rewriteWindowUseParser(t, w, nil)
	u := &rewriteWindowAppender{}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := w.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true,
		Changes: []domain.Change{w.rename("A", "B")}, MaxBytes: 16 << 20}, u)
	active, peak, reserves := p.state()
	if err != nil || active != 0 || peak > 2 || reserves == 0 || len(u.writes) != 2 || len(u.deferred) != 2 {
		t.Fatalf("WINDOW_MAX_PAGE: %v active%d peak%d reservations%d writes%d deferred%d", err, active, peak, reserves, len(u.writes), len(u.deferred))
	}
	for i, value := range u.writes {
		want := strings.ReplaceAll(old[i], "[[A]]", "[[B]]")
		if value.PageID != ids[i] || value.Content != want || p.calls[old[i]] != 1 || p.calls[want] != 1 {
			t.Fatal("largest-page route changed content or skipped one of the two parses")
		}
	}
	// Literal original KeepFacts amounts: the 12-byte first writing keeps
	// 15 credits, and the exact 5 MiB writing keeps 524302. Both stay owned
	// by Unit after the reservations have closed, with no double debit.
	const free = (8 << 20) - 524317
	h, err := w.budget.TakeNow(t.Context(), free)
	if err != nil {
		t.Fatalf("WINDOW_OWNERSHIP: unused reservation credit survived: %v", err)
	}
	h.Release()
	if extra, err := w.budget.TakeNow(t.Context(), free+1); err == nil {
		extra.Release()
		t.Fatal("WINDOW_OWNERSHIP: Unit facts were released early")
	}
	rewriteWindowReleases(t, w, u)
	all, err := w.budget.TakeNow(t.Context(), 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer all.Release()
	empty, ok := p.TryReserve(0)
	if !ok {
		t.Fatal("empty reservation was refused by unrelated full occupancy")
	}
	defer empty.Close()
	zero, err := empty.ParseNow(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if zero.Kept != 0 || len(zero.Facts.Links) != 0 {
		t.Fatal("zero content borrowed positive credit or acquired links")
	}
	zero.Release()
	zero.Release()
	empty.Close()
	if _, err := w.budget.TakeNow(t.Context(), 1); !errors.Is(err, markdown.ErrBusy) {
		t.Fatal("zero parse returned somebody else's credits")
	}
}

// The old broad B3-M1 test intentionally keeps its serial parses wrapper.
// This small exact boundary additionally binds the new coordinator's K:
// three 20487-byte writings keep 2063 each, so the fourth old parse misses
// a 26675-byte budget by one. No fourth ParseNow may be called.
func TestRewriteWindowKeepsTheOriginalOwnedShareBound(t *testing.T) {
	w, ids, _ := rewriteWindowFixture(t, 4)
	old := "[[A]]\n" + strings.Repeat("r", 20<<10) + "\n"
	for _, id := range ids {
		rewriteWindowSetContent(t, w, id, old)
	}
	w.budget = markdown.NewBudget(26675, time.Hour, slog.New(slog.DiscardHandler))
	w.rewrite.MaxContent = 64 << 10
	p := rewriteWindowUseParser(t, w, nil)
	u := &rewriteWindowAppender{}
	err := w.rewrite.Participate(t.Context(), app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true,
		Changes: []domain.Change{w.rename("A", "B")}, MaxBytes: 1 << 20}, u)
	var many *app.TooManyRewrites
	if !errors.As(err, &many) || *many != (app.TooManyRewrites{Kept: 26676, Max: 26675}) || len(u.writes) != 3 || len(u.deferred) != 3 || p.calls[old] != 3 || p.calls[strings.ReplaceAll(old, "[[A]]", "[[B]]")] != 3 {
		t.Fatalf("WINDOW_KEPT: err%v writes%d deferred%d old calls%d", err, len(u.writes), len(u.deferred), p.calls[old])
	}
	if active, peak, reserves := p.state(); active != 0 || peak > 2 || reserves == 0 {
		t.Fatalf("owned-share failure escaped active%d peak%d reservations%d", active, peak, reserves)
	}
	rewriteWindowReleases(t, w, u)
}
