package app

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// turns is a semaphore of cap(t) turns, a buffered channel: page/app
// imports only the standard library (archtest), so not x/sync's
// semaphore.Weighted, which the parse budget in platform/markdown uses.
type turns chan struct{}

// take is a turn, waiting for one within ctx; ctx's error when ctx has
// ended, first, or ends while it waits.
func (t turns) take(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case t <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// tryTake is a turn if one is free now.
func (t turns) tryTake() bool {
	select {
	case t <- struct{}{}:
		return true
	default:
		return false
	}
}

// give gives a turn back.
func (t turns) give() { <-t }

// GrepParses is the process's queue of grep's parses (M9/P4 design
// 3.6.1): a pattern's parse, its estimate and its keys, at most
// GOMAXPROCS at once, so that the memory of the parses is at most so many
// of the worst (about 7.3 MB each) and their CPU at most so many cores. A
// call waits for its turn within its ctx and is never refused: what it
// answers does not depend on the load, only when.
type GrepParses struct {
	turns turns
	// parse is domain.ParseGrepPattern; a test's fake counts the parses
	// in flight.
	parse func(pattern string, ignoreCase bool) (domain.GrepPattern, error)
}

// NewGrepParses is a queue of n turns, one at least.
func NewGrepParses(n int) *GrepParses {
	return &GrepParses{turns: make(turns, max(1, n)), parse: domain.ParseGrepPattern}
}

// Parse is q.parse in its turn, the turn held until it returns; ctx's
// error when ctx ends while it waits, which mcpserver answers as any call
// past its deadline.
func (q *GrepParses) Parse(ctx context.Context, pattern string, ignoreCase bool) (domain.GrepPattern, error) {
	if err := q.turns.take(ctx); err != nil {
		return domain.GrepPattern{}, err
	}
	defer q.turns.give()
	return q.parse(pattern, ignoreCase)
}

// GrepSlots are the calls that read and match at once in the process,
// max(1, GOMAXPROCS/2) as the parse budget's (M9 design 4.13): one waits
// GrepSlotWait at most for its slot, then answers 503 server_busy.
type GrepSlots struct {
	turns turns
	wait  time.Duration
}

// NewGrepSlots is n slots, one at least.
func NewGrepSlots(n int) *GrepSlots {
	return &GrepSlots{turns: make(turns, max(1, n)), wait: domain.GrepSlotWait}
}

// Take is a slot, released by the func it answers: shared.ServerBusy(1 s)
// after the slots' wait, ctx's error when ctx ends first.
func (s *GrepSlots) Take(ctx context.Context) (func(), error) {
	release := s.turns.give
	if s.turns.tryTake() {
		return release, nil
	}
	waited, cancel := context.WithTimeout(ctx, s.wait)
	defer cancel()
	if err := s.turns.take(waited); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, shared.ServerBusy(time.Second)
	}
	return release, nil
}

// GrepRequest is a grep's arguments (M9 design 4.13).
type GrepRequest struct {
	Pattern    string
	Glob       *string
	IgnoreCase bool
	Limit      *int
	Cursor     *string
}

// Grepped is a page of grep: its hits and partial lines in path-key and
// line order, each with the cursor after it, so that the adapter can stop
// at the answer's bound; Next, when more may follow, the cursor after the
// page; Building, when the pattern had keys and the notebook holds pages
// queued in bulk, which are not searched; Incomplete, when the walk
// stopped at one of its bounds before it checked every candidate (not at
// limit); and the walk's tally, for the log.
type Grepped struct {
	Found      []GrepFound
	Next       string
	Building   bool
	Incomplete bool
	Walk       GrepWalk
}

// GrepFound is a line grep found: a hit, with its text, or, Hit nil, a
// line it matched only the start of (partial_lines).
type GrepFound struct {
	Path   string
	Line   int
	Hit    *domain.GrepHit
	Cursor string
}

// GrepWalk is what a grep's walk did, as its log line tells it (M9/P4
// design 3.7.5): its keys, its candidates (-1 without keys), the pages it
// read, their bytes, its work, the lines it matched, the lines it found,
// and why it stopped: done, limit, pages, bytes, work, deadline or glob.
type GrepWalk struct {
	Keys, Candidates, Pages int
	Bytes, Work             int64
	Lines, Found            int
	Stop                    string
}

// The stops of a walk.
const (
	grepDone     = "done"
	grepLimit    = "limit"
	grepPages    = "pages"
	grepBytes    = "bytes"
	grepWork     = "work"
	grepDeadline = "deadline"
	grepGlob     = "glob"
)

// GrepDeps are what grep reads with; the bounds 0 are the domain's, which
// production leaves them.
type GrepDeps struct {
	Notebooks                     Notebooks
	Nodes                         Nodes
	States                        PageStates
	Reads                         GrepReads
	Auth                          shared.Authorizer
	Parses                        *GrepParses
	Slots                         *GrepSlots
	Logger                        *slog.Logger
	Clock                         Clock
	MaxWork, MaxLineWork, MaxRead int64
	Walk                          int
	GlobWork                      int64
}

// Grep finds the lines of a notebook's pages' contents a pattern matches
// (M9 design 4.13; M9/P4 design 3.7).
type Grep struct {
	d GrepDeps
	// lines is domain.GrepLines; a test's advances its clock as it scans
	// for each line's end, which no unit's time may count.
	lines func(content string, from int, yield func(line int, text string, column int) bool)
}

// NewGrep is the use case, its bounds 0 the domain's.
func NewGrep(d GrepDeps) *Grep {
	if d.MaxWork == 0 {
		d.MaxWork = domain.MaxGrepWork
	}
	if d.MaxLineWork == 0 {
		d.MaxLineWork = domain.MaxGrepLineWork
	}
	if d.MaxRead == 0 {
		d.MaxRead = domain.MaxGrepRead
	}
	if d.Walk == 0 {
		d.Walk = domain.MaxSearchWalk
	}
	if d.GlobWork == 0 {
		d.GlobWork = domain.MaxGlobWork
	}
	return &Grep{d: d, lines: domain.GrepLines}
}

// Turns are how many calls g's slots let read and match at once, and how
// many patterns its parses' queue lets parse at once.
func (g *Grep) Turns() (slots, parses int) {
	return cap(g.d.Slots.turns), cap(g.d.Parses.turns)
}

// allPages is the glob of a grep without one: every page.
const allPages = "**/*.md"

// Execute is the page of grep r in notebookID asks for (M9/P4 design
// 3.7.1): the actor; the cursor, 400; the pattern, 422 (Parses.Parse,
// which waits for its turn and refuses nothing); the glob, 422
// (domain.ParseGlob); the limit, 422; the notebook, notebook.not_found,
// page.read; the slot, 503; then the reads and the match, unlocked,
// without a transaction, no connection held while it matches.
func (g *Grep) Execute(ctx context.Context, notebookID uuid.UUID, r GrepRequest) (Grepped, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Grepped{}, err
	}
	var from domain.GrepCursor
	if r.Cursor != nil {
		if from, err = domain.DecodeGrepCursor(*r.Cursor); err != nil {
			return Grepped{}, err
		}
	}
	pattern, err := g.d.Parses.Parse(ctx, r.Pattern, r.IgnoreCase)
	if err != nil {
		return Grepped{}, err
	}
	globText := allPages
	if r.Glob != nil {
		globText = *r.Glob
	}
	glob, err := domain.ParseGlob("glob", globText)
	if err != nil {
		return Grepped{}, err
	}
	limit, err := domain.CheckGrepLimit(r.Limit)
	if err != nil {
		return Grepped{}, err
	}
	if err := decideNotebook(ctx, g.d.Notebooks, g.d.Auth, actor, domain.ActionRead, notebookID); err != nil {
		return Grepped{}, err
	}
	waitFrom := time.Now()
	release, err := g.d.Slots.Take(ctx)
	if err != nil {
		if se := (*shared.Error)(nil); errors.As(err, &se) && se.Code == shared.CodeServerBusy {
			g.d.Logger.LogAttrs(ctx, slog.LevelInfo, "grep is saturated", slog.Duration("waited", time.Since(waitFrom)))
		}
		return Grepped{}, err
	}
	defer release()
	re, err := pattern.Compile()
	if err != nil {
		return Grepped{}, err
	}
	w := grepWalker{g: g, ctx: ctx, notebookID: notebookID, re: re, pattern: pattern, glob: glob, limit: limit,
		cut: domain.GrepLineCut(pattern.Insts, g.d.MaxLineWork)}
	out, err := w.run(from, r.Cursor != nil)
	if err != nil {
		return Grepped{}, err
	}
	g.d.Logger.LogAttrs(ctx, slog.LevelInfo, "grep walked", slog.String("notebook_id", notebookID.String()),
		slog.Int("keys", out.Walk.Keys), slog.Int("candidates", out.Walk.Candidates), slog.Int("pages", out.Walk.Pages),
		slog.Int64("bytes", out.Walk.Bytes), slog.Int64("work", out.Walk.Work), slog.Int("lines", out.Walk.Lines),
		slog.Int("found", out.Walk.Found), slog.String("stop", out.Walk.Stop), slog.Bool("building", out.Building))
	return out, nil
}

// grepWalker is a call's walk (M9/P4 design 3.7.3): its pattern compiled
// and its bounds, what it has read and done so far, and the time it has
// spent matching its lines, which tells a unit's time (3.6.3).
type grepWalker struct {
	g          *Grep
	ctx        context.Context
	notebookID uuid.UUID
	re         *regexp.Regexp
	pattern    domain.GrepPattern
	glob       domain.Glob
	limit, cut int
	out        Grepped
	// soft is the call's deadline less GrepSoftMargin, zero without one.
	soft        time.Time
	read, work  int64
	lines, hits int
	// matched is the time the call's lines took to match, as 3.6.3
	// counts it: each line's from before its NFC within the cut to after
	// its match, the time of the work counted of it. Nothing between two
	// lines counts: not GrepLines' scan for the next line's end, nor a
	// batch's read. Counted from one line to the next, a long line's scan
	// fell to the short line before it, whose few units then told a
	// unit's time that stopped the long line at the soft deadline (review
	// C2-m3).
	matched    time.Duration
	candidates map[uuid.UUID]bool
	// stopped is why the walk stopped within its pages, and its cursor;
	// pending why its pages stopped, the 501st or the glob's work, which
	// counts once every page was checked.
	stopped, pending grepStop
}

// grepStop is why a walk stopped and the cursor it answers.
type grepStop struct{ why, cursor string }

// grepPage is a page the walk checks: its node, its path key, the line it
// starts at.
type grepPage struct {
	n    *domain.GlobNode
	key  string
	from int
}

// run reads the tree and the candidates and walks them from the cursor
// from (resumed when the call had one).
func (w *grepWalker) run(from domain.GrepCursor, resumed bool) (Grepped, error) {
	d := w.g.d
	if deadline, ok := w.ctx.Deadline(); ok {
		w.soft = deadline.Add(-domain.GrepSoftMargin)
	}
	nodes, err := d.Nodes.ListNodes(w.ctx, w.notebookID)
	if err != nil {
		return Grepped{}, err
	}
	t := treeOf(nodes)
	w.out.Walk = GrepWalk{Keys: len(w.pattern.Keys), Candidates: -1}
	if len(w.pattern.Keys) > 0 {
		// building first: a page queued in bulk whose row the job writes
		// between the two statements is then told (M9/P4 review A-m7).
		if w.out.Building, err = d.Reads.SearchBuilding(w.ctx, []uuid.UUID{w.notebookID}); err != nil {
			return Grepped{}, err
		}
		ids, err := d.Reads.GrepCandidates(w.ctx, w.notebookID, w.pattern.Keys)
		if err != nil {
			return Grepped{}, err
		}
		w.candidates = make(map[uuid.UUID]bool, len(ids))
		for _, id := range ids {
			w.candidates[id] = true
		}
		w.out.Walk.Candidates = len(w.candidates)
	}
	pages, err := w.pages(t, from, resumed)
	if err != nil {
		return Grepped{}, err
	}
	if err := w.check(t, pages); err != nil {
		return Grepped{}, err
	}
	stop := w.stopped
	if stop.why == "" {
		stop = w.pending
	}
	if stop.why == "" {
		stop.why = grepDone
	}
	w.out.Walk.Stop, w.out.Next = stop.why, stop.cursor
	w.out.Incomplete = stop.why != grepDone && stop.why != grepLimit
	w.out.Walk.Bytes, w.out.Walk.Work, w.out.Walk.Lines = w.read, w.work, w.lines
	w.out.Walk.Found = len(w.out.Found)
	return w.out, nil
}

// candidate reports whether the walk checks the page n: every page
// without keys, those of the candidates with.
func (w *grepWalker) candidate(n *domain.GlobNode) bool {
	return n.Node.Kind == domain.KindPage && (w.candidates == nil || w.candidates[n.Node.ID])
}

// pages are the pages the walk checks, in path-key order: the cursor's
// page from its line when it goes on within it and is still a candidate
// the glob matches, then those after its key the glob matches among the
// candidates, the glob's walk taking one more than d.Walk at most. It
// sets the stop when there was one more, or the glob's work ran out.
func (w *grepWalker) pages(t tree, from domain.GrepCursor, resumed bool) ([]grepPage, error) {
	d := w.g.d
	var out []grepPage
	if resumed && from.N >= 1 {
		if n := t.byKey(from.K); n != nil && w.candidate(n) && w.matches(t, n) {
			out = append(out, grepPage{n: n, key: from.K, from: from.N})
		}
	}
	after := ""
	if resumed {
		after = from.K
	}
	walk, err := w.glob.Walk(w.ctx, t.roots, after, d.GlobWork, func(keys []string, n *domain.GlobNode) bool {
		if !w.candidate(n) {
			return true
		}
		out = append(out, grepPage{n: n, key: strings.Join(keys, "/"), from: 1})
		return len(out) <= d.Walk
	})
	if err != nil {
		return nil, err
	}
	switch {
	case len(out) > d.Walk:
		out = out[:d.Walk]
		w.pending, err = stopAt(grepPages, out[d.Walk-1].key, 0)
	case walk.Stopped:
		w.pending, err = stopAt(grepGlob, walk.Last, 0)
	}
	return out, err
}

// matches reports whether the glob matches the page n, its path's keys
// stepped from the start.
func (w *grepWalker) matches(t tree, n *domain.GlobNode) bool {
	var work int64
	st := w.glob.Start()
	ancestors := t.ancestors(n)
	for depth, a := range ancestors {
		st = w.glob.Step(st, a.NameKey, depth, &work)
	}
	return w.glob.Accepts(w.glob.Step(st, n.Node.NameKey+".md", len(ancestors), &work))
}

// stopAt is a stop with its cursor (key, line).
func stopAt(why, key string, line int) (grepStop, error) {
	c, err := domain.GrepCursor{K: key, N: line}.Encode()
	return grepStop{why: why, cursor: c}, err
}

// stop stops the walk within its pages, at (key, line).
func (w *grepWalker) stop(why, key string, line int) error {
	var err error
	w.stopped, err = stopAt(why, key, line)
	return err
}

// check reads the pages' states, then their contents batch by batch, and
// matches their lines, until a bound stops it.
func (w *grepWalker) check(t tree, pages []grepPage) error {
	d := w.g.d
	if len(pages) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(pages))
	for i, p := range pages {
		ids[i] = p.n.Node.ID
	}
	states, err := d.States.PageStates(w.ctx, ids)
	if err != nil {
		return err
	}
	var alive []grepPage
	sizes := make(map[uuid.UUID]int64, len(states))
	for _, p := range pages {
		if s, ok := states[p.n.Node.ID]; ok {
			alive = append(alive, p)
			sizes[p.n.Node.ID] = int64(s.Bytes)
		}
	}
	for start := 0; start < len(alive); {
		// A batch: GrepBatchBytes at most, by the sizes known; a page
		// larger alone.
		end, bytes := start+1, sizes[alive[start].n.Node.ID]
		for end < len(alive) && bytes+sizes[alive[end].n.Node.ID] <= domain.GrepBatchBytes {
			bytes += sizes[alive[end].n.Node.ID]
			end++
		}
		// The bounds before the read, not after it: the bytes once a page
		// was read; the soft deadline once a line was matched.
		first := alive[start]
		switch {
		case w.read > 0 && w.read+bytes > d.MaxRead:
			return w.stop(grepBytes, first.key, first.from)
		case w.lines > 0 && w.pastSoft(d.Clock.Now()):
			return w.stop(grepDeadline, first.key, first.from)
		}
		batch := make([]uuid.UUID, end-start)
		for i, p := range alive[start:end] {
			batch[i] = p.n.Node.ID
		}
		got, err := d.Reads.GrepContents(w.ctx, batch, w.readBound(end-start))
		if err != nil {
			return err
		}
		next := end
		for k, p := range alive[start:end] {
			if p.n.Node.ID == got.Past {
				// Grown past the bound since its state was read: batched
				// again by its size now, where the bytes' bound may stop
				// the walk at it (M9 Codex review D2).
				sizes[p.n.Node.ID], next = int64(got.PastBytes), start+k
				break
			}
			content, ok := got.Contents[p.n.Node.ID]
			if !ok {
				continue // deleted since the states were read
			}
			w.read += int64(len(content))
			w.out.Walk.Pages++
			if err := w.checkPage(t, p, content); err != nil || w.stopped.why != "" {
				return err
			}
		}
		start = next
	}
	return nil
}

// readBound is the most bytes the read of a batch of n pages may read, in
// its statement (M9 Codex review D2): GrepBatchBytes for more than one
// page, any for a page alone, which the batch's bound lets be larger; and
// what d.MaxRead leaves once a page was read. The statement reads none of
// a page that grew past it since its state, which the walk batches again
// by its size then: the bounds hold on the bytes read, not on the sizes
// read before them, whatever the writes between.
func (w *grepWalker) readBound(n int) int64 {
	bound := int64(math.MaxInt64)
	if n > 1 {
		bound = domain.GrepBatchBytes
	}
	if w.read > 0 {
		bound = min(bound, w.g.d.MaxRead-w.read)
	}
	return bound
}

// pastSoft reports whether now is past the soft deadline.
func (w *grepWalker) pastSoft(now time.Time) bool {
	return !w.soft.IsZero() && now.After(w.soft)
}

// checkPage matches the lines of the page p's content from its line on,
// each judged before it starts (M9/P4 design 3.6.3), but the call's
// first, until the walk stops.
func (w *grepWalker) checkPage(t tree, p grepPage, content string) error {
	d := w.g.d
	path := ""
	var err error
	w.g.lines(content, p.from, func(line int, text string, column int) bool {
		now := d.Clock.Now()
		// The line as it is matched: its NFC within the cut. The line is
		// judged by those bytes, the ones its work then counts, so that
		// the work counted is never more than the work judged: NFC can
		// make a line three times longer (U+1D160), and judged by its own
		// bytes a line could pass MaxWork by two thirds of a line's bound
		// (review C-I2).
		g := domain.NewGrepNFC(text, w.cut)
		if w.lines > 0 {
			cost := domain.GrepWork(len(g.Text), w.pattern.Insts)
			// A unit's time: GrepUnitTime until the call's timing rests on
			// GrepUnitWarmUp of work, then its own when slower (3.6.3).
			unit := domain.GrepUnitTime
			if w.work >= domain.GrepUnitWarmUp {
				unit = max(unit, w.matched/time.Duration(w.work))
			}
			switch {
			case w.work+cost > d.MaxWork:
				err = w.stop(grepWork, p.key, line)
				return false
			case w.pastSoft(now.Add(time.Duration(cost) * unit)):
				err = w.stop(grepDeadline, p.key, line)
				return false
			}
		}
		m := g.Match(w.re, text, column)
		w.matched += d.Clock.Now().Sub(now)
		w.work += domain.GrepWork(m.Bytes, w.pattern.Insts)
		w.lines++
		if m.Hit == nil && !m.Partial {
			return true
		}
		if path == "" {
			path = t.pathOf(p.n)
		}
		var cursor string
		if cursor, err = (domain.GrepCursor{K: p.key, N: line + 1}).Encode(); err != nil {
			return false
		}
		w.out.Found = append(w.out.Found, GrepFound{Path: path, Line: line, Hit: m.Hit, Cursor: cursor})
		if m.Hit != nil {
			if w.hits++; w.hits == w.limit {
				w.stopped = grepStop{why: grepLimit, cursor: cursor}
				return false
			}
		}
		return true
	})
	return err
}

// byKey is the node of the path key k: its segments' keys, a page's last
// with ".md"; nil for none.
func (t tree) byKey(k string) *domain.GlobNode {
	level := t.roots
	segments := strings.Split(k, "/")
	for i, seg := range segments {
		last := i == len(segments)-1
		var next *domain.GlobNode
		for _, n := range level {
			switch {
			case last && n.Node.Kind == domain.KindPage && n.Node.NameKey+".md" == seg:
				return n
			case !last && n.Node.Kind == domain.KindPage && n.Node.NameKey == seg:
				next = n
			}
		}
		if next == nil {
			return nil
		}
		level = next.Children
	}
	return nil
}
