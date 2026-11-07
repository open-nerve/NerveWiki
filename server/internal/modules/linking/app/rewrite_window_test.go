package app_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The old parses wrapper deliberately stays serial. This observer explicitly
// forwards the optional capability of the actual adapter and synchronizes
// its records; both routes still execute the real Markdown/PageFacts core.
type rewriteWindowParser struct {
	app.RewriteParser
	leases                        app.RewriteReservations
	mu                            sync.Mutex
	work                          sync.WaitGroup
	active, peak, reserves, grows int
	calls                         map[string]int
	reserved                      map[string]bool
	after                         func(string)
	beforeGrow                    func(int)
	beforePlain                   func(string)
}

func (p *rewriteWindowParser) begin(content string, reserved bool) func() {
	p.work.Add(1)
	p.mu.Lock()
	p.active++
	p.peak = max(p.peak, p.active)
	p.calls[content]++
	p.reserved[content] = reserved
	p.mu.Unlock()
	return func() {
		defer p.work.Done()
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
		if p.after != nil {
			p.after(content)
		}
	}
}
func (p *rewriteWindowParser) ParseNow(ctx context.Context, content string) (app.Parsed, error) {
	if p.beforePlain != nil {
		p.beforePlain(content)
	}
	defer p.begin(content, false)()
	return p.RewriteParser.ParseNow(ctx, content)
}
func (p *rewriteWindowParser) TryReserve(upper int) (app.RewriteReservation, bool) {
	p.mu.Lock()
	p.reserves++
	p.mu.Unlock()
	r, ok := p.leases.TryReserve(upper)
	if !ok {
		return nil, false
	}
	return rewriteWindowReservation{RewriteReservation: r, p: p}, true
}

type rewriteWindowReservation struct {
	app.RewriteReservation
	p *rewriteWindowParser
}

func (r rewriteWindowReservation) TryGrow(upper int) bool {
	r.p.mu.Lock()
	r.p.grows++
	r.p.mu.Unlock()
	if r.p.beforeGrow != nil {
		r.p.beforeGrow(upper)
	}
	return r.RewriteReservation.TryGrow(upper)
}
func (r rewriteWindowReservation) ParseNow(ctx context.Context, content string) (app.Parsed, error) {
	defer r.p.begin(content, true)()
	return r.RewriteReservation.ParseNow(ctx, content)
}
func (p *rewriteWindowParser) state() (int, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active, p.peak, p.reserves
}
func (p *rewriteWindowParser) usedReservation(content string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reserved[content]
}

type rewriteWindowContents struct {
	app.PageContents
	mu             sync.Mutex
	reads          map[uuid.UUID]int
	fail           uuid.UUID
	cause          error
	respectContext bool
	before         func(uuid.UUID)
}

func (c *rewriteWindowContents) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	c.mu.Lock()
	c.reads[id]++
	c.mu.Unlock()
	if c.before != nil {
		c.before(id)
	}
	if c.respectContext && ctx.Err() != nil {
		return "", 0, false, ctx.Err()
	}
	if id == c.fail {
		return "", 0, false, c.cause
	}
	return c.PageContents.Content(ctx, id)
}

type rewriteWindowAppender struct {
	appender
	write func(context.Context, app.Rewritten) error
}

func (a *rewriteWindowAppender) WriteContent(ctx context.Context, w app.Rewritten) error {
	a.writes = append(a.writes, w)
	if a.write != nil {
		return a.write(ctx, w)
	}
	return nil
}

type rewriteWindowLocks struct {
	app.Locks
	parser     *rewriteWindowParser
	reads      int
	beforeJoin bool
	held       shared.LockHolder
}

func (l *rewriteWindowLocks) Of(ctx context.Context, ids []uuid.UUID, at time.Time) ([]shared.LockHolder, error) {
	l.reads++
	if l.reads == 2 {
		active, _, _ := l.parser.state()
		l.beforeJoin = active == 1 && ctx.Err() == nil
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return []shared.LockHolder{l.held}, nil
	}
	return l.Locks.Of(ctx, ids, at)
}

func rewriteWindowFixture(t *testing.T, n int) (*rewriting, []uuid.UUID, []string) {
	t.Helper()
	names := []string{"A"}
	for i := range n {
		names = append(names, fmt.Sprintf("R%d", i))
	}
	w := newRewriting(t, names...)
	ids := make([]uuid.UUID, n)
	for i := range n {
		ids[i] = w.id(fmt.Sprintf("R%d", i))
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	old := make([]string, n)
	for i, id := range ids {
		old[i] = fmt.Sprintf("slot%d [[A]]\n", i)
		for name, p := range w.tree.names {
			if p == id {
				w.write(name, old[i])
				break
			}
		}
	}
	return w, ids, old
}
func rewriteWindowUseParser(t *testing.T, w *rewriting, extract func(markdown.Tree) any) *rewriteWindowParser {
	t.Helper()
	exts := []markdown.Extension{obsidian.Extension(obsidian.Options{})}
	if extract != nil {
		exts = append(exts, markdown.Extension{Name: "window-barrier", Extract: extract})
	}
	md, err := markdown.New(exts)
	if err != nil {
		t.Fatal(err)
	}
	real := markdownadapter.NewParser(md, w.budget)
	p := &rewriteWindowParser{RewriteParser: real, leases: real, calls: map[string]int{}, reserved: map[string]bool{}}
	w.rewrite.Parser = p
	return p
}
func rewriteWindowReleases(t *testing.T, w *rewriting, u *rewriteWindowAppender) {
	t.Helper()
	for _, release := range u.deferred {
		release()
		release()
	}
	h, err := w.budget.TakeNow(t.Context(), w.budget.Size())
	if err != nil {
		t.Fatalf("WINDOW_RELEASE: whole original budget not returned: %v", err)
	}
	h.Release()
}

// These are app-order tests with real CPU parses. The A write is an app
// port barrier here; a separate bootstrap test must prove real PG behavior.
func TestRewriteWindowRealAdapterOverlapAndOrderedFailure(t *testing.T) {
	for _, mode := range []string{"healthy", "guard_before_join", "early_error_future_panic", "future_content_error", "last_write_cancel"} {
		t.Run(mode, func(t *testing.T) {
			w, ids, old := rewriteWindowFixture(t, 2)
			now := make([]string, 2)
			for i, s := range old {
				now[i] = strings.ReplaceAll(s, "[[A]]", "[[B]]")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			// A deadline only releases a deliberately held real CPU callback on
			// error. Entered gates, active counts and cause ordering are the oracle.
			if mode == "guard_before_join" || mode == "early_error_future_panic" || mode == "future_content_error" {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 500*time.Millisecond)
				defer cancel()
			}
			readA, oldB := make(chan struct{}), make(chan struct{})
			release := make(chan struct{})
			var aEntered, bEntered, unblock sync.Once
			open := func() { unblock.Do(func() { close(release) }) }
			cause := errors.New("window designated original cause")
			var p *rewriteWindowParser
			p = rewriteWindowUseParser(t, w, func(tree markdown.Tree) any {
				content := string(tree.Content)
				if !p.usedReservation(content) {
					return nil
				}
				switch content {
				case now[0]:
					aEntered.Do(func() { close(readA) })
					if mode == "future_content_error" {
						<-ctx.Done()
					} else {
						select {
						case <-oldB:
						case <-ctx.Done():
						}
					}
				case old[1]:
					bEntered.Do(func() { close(oldB) })
					if mode == "healthy" || mode == "last_write_cancel" {
						select {
						case <-release:
						case <-ctx.Done():
						}
					} else {
						<-ctx.Done()
					}
					if mode == "early_error_future_panic" {
						panic("future original panic")
					}
				}
				return nil
			})
			t.Cleanup(func() { cancel(); open(); p.work.Wait() })
			contents := &rewriteWindowContents{PageContents: w.contents, reads: map[uuid.UUID]int{}}
			faultHadWorker := false
			if mode == "future_content_error" {
				contents.fail, contents.cause = ids[1], cause
				contents.before = func(id uuid.UUID) {
					if id == ids[1] {
						select {
						case <-readA:
						case <-ctx.Done():
						}
						active, _, _ := p.state()
						faultHadWorker = active == 1
					}
				}
			}
			w.rewrite.Contents = contents
			locks := &rewriteWindowLocks{Locks: w.locks, parser: p, held: shared.LockHolder{PageID: ids[0], UserID: uuid.UUID{15: 77}, DisplayName: "held"}}
			if mode == "guard_before_join" {
				w.rewrite.Locks = locks
			}
			u := &rewriteWindowAppender{}
			overlap := false
			u.write = func(ctx context.Context, value app.Rewritten) error {
				_, _, reserves := p.state()
				if reserves == 0 {
					return errors.New("WINDOW_WIRING: configured parser's optional capability was bypassed")
				}
				if value.PageID == ids[0] {
					select {
					case <-oldB:
					case <-ctx.Done():
						return errors.New("WINDOW_OVERLAP: actual B-old did not enter before A")
					}
					active, peak, _ := p.state()
					overlap = active == 1 && peak == 2
					switch mode {
					case "guard_before_join":
						return fmt.Errorf("%w: %w", app.ErrGuardLocked, cause)
					case "early_error_future_panic":
						return cause
					default:
						open()
					}
				}
				if mode == "last_write_cancel" && value.PageID == ids[1] {
					cancel()
				}
				return nil
			}
			change := w.rename("A", "B")
			err := w.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true, Changes: []domain.Change{change}, MaxBytes: 1 << 20}, u)
			active, peak, reserves := p.state()
			if active != 0 || peak > 2 || reserves == 0 {
				t.Fatalf("WINDOW_JOIN: returned with active/peak/reservations %d/%d/%d", active, peak, reserves)
			}
			switch mode {
			case "healthy", "last_write_cancel":
				if err != nil || !overlap || len(u.writes) != 2 {
					t.Fatalf("WINDOW_OVERLAP: %s err%v overlap%t writes%d", mode, err, overlap, len(u.writes))
				}
				for i, value := range u.writes {
					if value.PageID != ids[i] || value.Content != now[i] || p.calls[old[i]] != 1 || p.calls[now[i]] != 1 {
						t.Fatalf("WINDOW_ONCE: source%d writes/parses changed", i)
					}
				}
				if mode == "last_write_cancel" && !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatal("late cancellation did not occur")
				}
			case "guard_before_join":
				var problem *shared.Error
				if !errors.As(err, &problem) || problem.Code != "linking.pages_locked" || !locks.beforeJoin || !overlap || len(u.writes) != 1 {
					t.Fatalf("WINDOW_GUARD_ORDER: %v beforeJoin%t overlap%t writes%d", err, locks.beforeJoin, overlap, len(u.writes))
				}
			case "early_error_future_panic":
				//nolint:errorlint // The earlier write must retain its exact error object across the future panic.
				if err != cause || !overlap || len(u.writes) != 1 || p.calls[old[1]] != 1 {
					t.Fatalf("earlier original error lost to future panic: %v overlap%t", err, overlap)
				}
			case "future_content_error":
				//nolint:errorlint // Future Content failure must return the exact native error object.
				if err != cause || !faultHadWorker || len(u.writes) != 0 || p.calls[old[1]] != 0 {
					t.Fatalf("future Content cause was deferred past A: %v worker%t writes%d", err, faultHadWorker, len(u.writes))
				}
			}
			for _, id := range ids {
				if contents.reads[id] != 1 {
					t.Fatalf("WINDOW_READ_ONCE: %s read%d", id, contents.reads[id])
				}
			}
			rewriteWindowReleases(t, w, u)
		})
	}
}

func TestRewriteWindowOldReadyGrowFailureContinuesWithoutReparse(t *testing.T) {
	w, ids, old := rewriteWindowFixture(t, 2)
	// Different link counts make B need a larger readback grant than A.
	for name, id := range w.tree.names {
		if id == ids[1] {
			old[1] = "slot1 " + strings.Repeat("[[A]] ", 20) + strings.Repeat("x", 4100-6-120)
			w.write(name, old[1])
			break
		}
	}
	w.budget = markdown.NewBudget(12288, time.Second, slog.New(slog.DiscardHandler))
	w.rewrite.MaxContent = 16384
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldB := make(chan struct{})
	var bDone sync.Once
	target := strings.Repeat("b", 250)
	nowA := strings.ReplaceAll(old[0], "[[A]]", "[["+target+"]]")
	var p *rewriteWindowParser
	p = rewriteWindowUseParser(t, w, func(tree markdown.Tree) any {
		if string(tree.Content) == nowA && p.usedReservation(nowA) {
			select {
			case <-oldB:
			case <-ctx.Done():
			}
		}
		return nil
	})
	t.Cleanup(func() { cancel(); p.work.Wait() })
	p.after = func(content string) {
		if content == old[1] {
			bDone.Do(func() { close(oldB) })
		}
	}
	var other *markdown.Hold
	grows := 0
	p.beforeGrow = func(upper int) {
		if upper > 8192 {
			grows++
			var err error
			other, err = w.budget.TakeNow(t.Context(), 6000)
			if err != nil {
				panic(err)
			}
		}
	}
	p.beforePlain = func(content string) {
		if len(content) > 8192 && other != nil {
			other.Release()
			other = nil
		}
	}
	contents := &rewriteWindowContents{PageContents: w.contents, reads: map[uuid.UUID]int{}}
	w.rewrite.Contents = contents
	u := &rewriteWindowAppender{}
	change := w.rename("A", target)
	err := w.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true, Changes: []domain.Change{change}, MaxBytes: 1 << 20}, u)
	if other != nil {
		other.Release()
	}
	if err != nil || grows != 1 || len(u.writes) != 2 || p.calls[old[1]] != 1 || contents.reads[ids[1]] != 1 {
		t.Fatalf("WINDOW_GROW_FALLBACK: err%v grows%d writes%d old parses%d reads%d", err, grows, len(u.writes), p.calls[old[1]], contents.reads[ids[1]])
	}
	for i, value := range u.writes {
		want := strings.ReplaceAll(old[i], "[[A]]", "[["+target+"]]")
		if value.PageID != ids[i] || value.Content != want || p.calls[want] != 1 {
			t.Fatal("fallback changed candidate/actual parse count")
		}
	}
	if p.usedReservation(u.writes[1].Content) {
		t.Fatal("the failed growth did not reach the real serial parser")
	}
	rewriteWindowReleases(t, w, u)
}

// A current readback panic remains the same value, but only after the
// already-started future real parse has completed and relinquished its slot.
func TestRewriteWindowCurrentPanicJoinsFuture(t *testing.T) {
	w, ids, old := rewriteWindowFixture(t, 2)
	now := strings.ReplaceAll(old[0], "[[A]]", "[[B]]")
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	futureEntered, futureDone := make(chan struct{}), make(chan struct{})
	var entered, finished sync.Once
	panicValue := &struct{ label string }{"current original panic"}
	var p *rewriteWindowParser
	p = rewriteWindowUseParser(t, w, func(tree markdown.Tree) any {
		content := string(tree.Content)
		if !p.usedReservation(content) {
			return nil
		}
		switch content {
		case now:
			select {
			case <-futureEntered:
			case <-ctx.Done():
				panic("future did not enter")
			}
			panic(panicValue)
		case old[1]:
			entered.Do(func() { close(futureEntered) })
			<-ctx.Done()
		}
		return nil
	})
	p.after = func(content string) {
		if content == old[1] {
			finished.Do(func() { close(futureDone) })
		}
	}
	t.Cleanup(func() { cancel(); p.work.Wait() })
	contents := &rewriteWindowContents{PageContents: w.contents, reads: map[uuid.UUID]int{}}
	w.rewrite.Contents = contents
	u := &rewriteWindowAppender{}
	var got any
	change := w.rename("A", "B")
	func() {
		defer func() { got = recover() }()
		_ = w.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true, Changes: []domain.Change{change}, MaxBytes: 1 << 20}, u)
	}()
	active, peak, reserves := p.state()
	if got != panicValue || active != 0 || peak != 2 || reserves != 2 || len(u.writes) != 0 || len(u.deferred) != 0 {
		t.Fatalf("WINDOW_CURRENT_PANIC: value%v active/peak/reserves%d/%d/%d writes%d deferred%d", got, active, peak, reserves, len(u.writes), len(u.deferred))
	}
	select {
	case <-futureDone:
	default:
		t.Fatal("WINDOW_JOIN: current panic escaped before future parser completed")
	}
	if p.calls[old[0]] != 1 || p.calls[now] != 1 || p.calls[old[1]] != 1 || contents.reads[ids[0]] != 1 || contents.reads[ids[1]] != 1 {
		t.Fatal("current panic retried work")
	}
	rewriteWindowReleases(t, w, u)
}
