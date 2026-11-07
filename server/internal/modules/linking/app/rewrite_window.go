package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// One of two fixed preparation slots. A completed task still owns this
// slot and its full reservation until ordered promotion or disposal.
type rewriteWindowPage struct {
	id                                    uuid.UUID
	revision, current                     int
	content                               string
	reservation                           RewriteReservation
	old                                   Parsed
	oldDone, oldFit, promoted, leftLogged bool
	rewriting                             domain.Rewriting
	facts                                 domain.Facts
	result                                rewriteReadBack
	err                                   error
	panicked                              bool
	panicValue                            any
	done                                  chan struct{}
}

func (p *rewriteWindowPage) ready() bool {
	if p.done == nil {
		return true
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}
func (p *rewriteWindowPage) wait() {
	if p.done != nil {
		<-p.done
	}
}
func (p *rewriteWindowPage) capture(f func()) {
	completed := false
	defer func() {
		if !completed {
			p.panicked = true
			p.panicValue = recover()
		}
	}()
	f()
	completed = true
}
func (p *rewriteWindowPage) start(f func()) {
	done := make(chan struct{})
	p.done = done
	go func() { defer close(done); p.capture(f) }()
}
func (p *rewriteWindowPage) dispose() {
	if p == nil {
		return
	}
	p.wait()
	p.old.release()
	p.result.now.release()
	p.old = Parsed{}
	p.result = rewriteReadBack{}
	p.facts = domain.Facts{}
	p.rewriting = domain.Rewriting{}
	p.content = ""
	if p.reservation != nil {
		p.reservation.Close()
		p.reservation = nil
	}
}

// rewriteUpper bounds only legal candidates that may reach ParseNow.
// Validate every interval even after the byte sum has saturated. Invalid
// edits retain Written/Apply's original behavior through the serial path.
func rewriteUpper(old, limit int, edits []domain.Edit) (int, bool) {
	if old < 0 || limit < 0 || old > limit {
		return 0, false
	}
	ordered := slices.Clone(edits)
	slices.SortFunc(ordered, func(a, b domain.Edit) int { return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End)) })
	upper, at := old, 0
	for _, e := range ordered {
		if e.Start < at || e.End < e.Start || e.End > old {
			return 0, false
		}
		at = e.End
		span := e.End - e.Start
		if len(e.Text) > span {
			delta := len(e.Text) - span
			upper += min(delta, limit-upper)
		}
	}
	return upper, true
}

func keptCopy(shares *keptShares) *keptShares {
	if shares == nil {
		return nil
	}
	copyOf := *shares
	return &copyOf
}

// Only this goroutine reads Content, logs, updates the Unit/shares, or
// applies a write. Future CPU failures are values until their page's turn.
func (r Rewrite) rewriteWindow(ctx context.Context, m Moved, index followedPages, all []uuid.UUID, shares *keptShares, u Appender, reservations RewriteReservations) error {
	var front, future *rewriteWindowPage
	var apply *Rewritten // already handed to Unit; no old/domain facts
	next := 0
	defer func() { front.dispose(); future.dispose() }() // also joins on coordinator panic
	load := func() (*rewriteWindowPage, error) {
		id := index.pages[next]
		next++
		content, current, ok, err := r.Contents.Content(ctx, id)
		if err != nil {
			return nil, err
		} // Early SQL failure may already abort this Tx.
		p := &rewriteWindowPage{id: id, revision: index.indexed[id].Revision, current: current, content: content}
		if !ok {
			p.err = fmt.Errorf("the content of page %s, which the rewrite reads, is gone", id)
		}
		return p, nil
	}
	logLeft := func(p *rewriteWindowPage) {
		if !p.leftLogged {
			for _, l := range p.rewriting.Left {
				r.left(ctx, "no writing leads where it led", p.id, l)
			}
			p.leftLogged = true
		}
	}
	applyNow := func() error {
		err := u.WriteContent(ctx, *apply)
		apply = nil
		if errors.Is(err, ErrGuardLocked) {
			return r.guardLocked(ctx, all, m, err)
		}
		return err // fixed before the deferred joins, including guardLocked above
	}
	for front != nil || next < len(index.pages) || apply != nil {
		if apply != nil && (front == nil || !front.ready() || !front.oldDone && front.err == nil && front.current == front.revision) {
			// Do not wait for B-old or run its unadmitted CPU before A's SQL.
			if err := applyNow(); err != nil {
				return err
			}
		}
		if front == nil {
			if next == len(index.pages) {
				continue
			}
			var err error
			front, err = load()
			if err != nil {
				return err
			}
		}
		front.wait()
		if front.err == nil && !front.panicked && front.current == front.revision && !front.oldDone {
			// Current-page work, never skipped merely because ctx is cancelled.
			front.capture(func() {
				if err := shares.fit(len(front.content)); err != nil {
					front.err = err
					return
				}
				front.oldFit = true
				if ctx.Err() == nil && len(front.content) <= r.MaxContent {
					front.reservation, _ = reservations.TryReserve(len(front.content))
				}
				parse := r.Parser.ParseNow
				if front.reservation != nil {
					parse = front.reservation.ParseNow
				}
				front.old, front.err = parse(ctx, front.content)
				front.oldDone = true
			})
		}
		if front.err == nil && !front.panicked && front.current == front.revision && !front.promoted {
			front.capture(func() {
				// Recheck the original old fit at its ordered position, using actual K.
				if !front.oldFit {
					if err := shares.fit(len(front.content)); err != nil {
						front.err = err
						return
					}
					front.oldFit = true
				}
				front.old.Release()
				front.facts = domain.Facts{Links: front.old.Facts.Links, Aliases: front.old.Facts.Aliases}
				front.old = Parsed{}
				front.rewriting = rewriteOf(front.content, front.facts, index.reached[front.id], index.from[front.id], index.tree, index.recased)
				front.promoted = true
			})
			if front.err == nil && !front.panicked && len(front.rewriting.Edits) > 0 {
				upper, valid := rewriteUpper(len(front.content), r.MaxContent, front.rewriting.Edits)
				admitted := false
				if valid && ctx.Err() == nil {
					if front.reservation == nil {
						front.reservation, admitted = reservations.TryReserve(upper)
					} else {
						admitted = front.reservation.TryGrow(upper)
					}
				}
				if !admitted && front.reservation != nil {
					front.reservation.Close()
					front.reservation = nil
				}
				if !admitted && apply != nil {
					if err := applyNow(); err != nil {
						return err
					}
				}
				if apply == nil {
					logLeft(front)
				} // Before any ordinary busy or current-page panic.
				p := front
				snapshot := keptCopy(shares)
				parse := r.Parser.ParseNow
				if admitted {
					parse = p.reservation.ParseNow
				}
				work := func() {
					p.result, p.err = r.readBack(ctx, p.content, p.facts, index.from[p.id], index.tree, p.rewriting, parse, snapshot)
				}
				if admitted && ctx.Err() == nil {
					p.start(work)
				} else {
					p.capture(work)
				}
				// The foreground grant is secured before a future page can borrow.
				if admitted && ctx.Err() == nil && future == nil && next < len(index.pages) {
					var err error
					future, err = load()
					if err != nil {
						return err
					}
					if future.err == nil && future.current == future.revision && len(future.content) <= r.MaxContent {
						foreground, size := r.Parser.Takes(upper)
						take, _ := r.Parser.Takes(len(future.content))
						room := size - foreground
						if shares != nil {
							room -= shares.n
						}
						if take <= room {
							future.reservation, _ = reservations.TryReserve(len(future.content))
							if future.reservation != nil {
								q := future
								q.start(func() { q.old, q.err = q.reservation.ParseNow(ctx, q.content); q.oldDone = true })
							}
						}
					}
				}
			}
		}
		if apply != nil {
			if err := applyNow(); err != nil {
				return err
			}
		}
		logLeft(front) // Now ordered, even if the readback task already panicked.
		front.wait()
		if front.panicked {
			panic(front.panicValue)
		}
		if front.current != front.revision && front.err == nil {
			r.Logger.LogAttrs(ctx, slog.LevelWarn, "the links of a page are not rewritten: its content is not the index's",
				slog.String("page_id", front.id.String()), slog.Int("revision", front.current), slog.Int("indexed", front.revision))
		} else {
			if front.err != nil {
				return front.err
			}
			if len(front.rewriting.Edits) > 0 && r.logReadBack(ctx, front.id, len(front.content), front.result) {
				u.Defer(front.result.now.Release)
				shares.add(front.result.now)
				apply = &Rewritten{PageID: front.id, Base: front.revision, Content: front.result.content, Facts: front.result.now.Written}
				front.result.now = Parsed{} // Unit alone owns this child, even if Write fails.
			}
		}
		front.dispose()
		front, future = future, nil
	}
	return nil
}
