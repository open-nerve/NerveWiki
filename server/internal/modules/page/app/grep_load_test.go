//go:build !race

package app_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// systemClock is the time as it passes: the load's measurements are of
// the use case's own clock reads.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// loadPages are a notebook's pages at its root and their contents, as the
// use case reads them: ListNodes, PageStates and GrepReads in memory, so
// that a call's time is its matching's, no database's.
type loadPages struct {
	app.Nodes
	nodes    []domain.Node
	contents map[uuid.UUID]string
}

func newLoadPages(notebook uuid.UUID, contents []string) loadPages {
	p := loadPages{contents: map[uuid.UUID]string{}}
	for i, c := range contents {
		n := domain.Node{ID: uuid.NewV7(), NotebookID: notebook, Kind: domain.KindPage, Name: fmt.Sprintf("p%05d", i)}
		n.NameKey = shared.TitleKey(n.Name)
		p.nodes = append(p.nodes, n)
		p.contents[n.ID] = c
	}
	return p
}

func (p loadPages) ListNodes(context.Context, uuid.UUID) ([]domain.Node, error) { return p.nodes, nil }

func (p loadPages) PageStates(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageState, error) {
	out := make(map[uuid.UUID]app.PageState, len(ids))
	for _, id := range ids {
		out[id] = app.PageState{Revision: 1, Bytes: len(p.contents[id])}
	}
	return out, nil
}

func (loadPages) AliveNodes(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return nil, nil
}

// GrepCandidates are every page: a keyed pattern's walk is a keyless one's.
func (p loadPages) GrepCandidates(context.Context, uuid.UUID, []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(p.nodes))
	for i, n := range p.nodes {
		out[i] = n.ID
	}
	return out, nil
}

func (p loadPages) GrepContents(_ context.Context, ids []uuid.UUID, _ int64) (app.ContentsWithin, error) {
	out := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		out[id] = p.contents[id]
	}
	return app.ContentsWithin{Contents: out}, nil
}

func (loadPages) SearchBuilding(context.Context, []uuid.UUID) (bool, error) { return false, nil }

// loadAlternation is 3.6.4's pattern, the 36 branches.
func loadAlternation() string {
	alts := make([]string, 0, 36)
	for n := 70; n >= 35; n-- {
		alts = append(alts, `[\pL\pN\pM\pS\pP]{`+strconv.Itoa(n)+`}QZ`)
	}
	return "(?:" + strings.Join(alts, "|") + ")"
}

// denseText is n bytes of lowercase letters and digits, no Q: the class
// matches everywhere and the pattern nowhere.
func denseText(n int, rng *rand.Rand) string {
	const a = "abcdefghijklmnopqrstuvw0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = a[rng.IntN(len(a))]
	}
	return string(b)
}

// linesOf is a content of size bytes or so in lines of n bytes, each
// line's text of mk.
func linesOf(n, size int, mk func(int) string) string {
	var b strings.Builder
	for b.Len() < size {
		b.WriteString(mk(n))
		b.WriteByte('\n')
	}
	return b.String()
}

// quantileOf is x's p-quantile.
func quantileOf(x []float64, p float64) float64 {
	s := slices.Clone(x)
	slices.Sort(s)
	return s[int(p*float64(len(s)-1))]
}

// BenchmarkGrepAtFullLoad re-measures grep's bounds through the use case,
// S1's step 9 (M9/P4 design 3.6.4, 6.2): max(1, GOMAXPROCS/2) calls at
// once, every slot taken, beside as many goroutines of the search's and
// list's CPU work on the same contents (SearchTokens of them, and the
// worst glob's walk over P1's guard tree); NERVE_GREP_LOAD_SECS seconds a
// shape, 3 by default (a burst, the host's reading of full load), in
// NERVE_GREP_LOAD_ROUNDS rounds, 2 by default; the calibration
// NERVE_GREP_CALIB_SECS, 45 by default, as step 1's. The use case reads its
// pages from memory, so that a call is its matching. Run by hand through
// heavy.sh, never in CI:
//
//	go test -run '^$' -bench '^BenchmarkGrepAtFullLoad$' -benchtime 1x ./internal/modules/page/app/
//
// MaxGrepWork's shapes, lines of 80 bytes and of 4 KiB, each a call of
// 5 × 10⁷ units, ns a unit; MaxGrepLineWork's, a line of 5 MiB cut at
// 1.5 and 0.75 × 10⁸, a call each; GrepLineOverhead's, the calibration's
// cells (22 of exp/lineoh's 24 line patterns and its lines, the hits
// left out, calibrate says why) through calls over pages of their lines,
// L at the median and p90 against U, the long line's ns a unit.
func BenchmarkGrepAtFullLoad(b *testing.B) {
	secs, rounds, calib := 3, 2, 45
	if v, err := strconv.Atoi(os.Getenv("NERVE_GREP_LOAD_SECS")); err == nil {
		secs = v
	}
	if v, err := strconv.Atoi(os.Getenv("NERVE_GREP_LOAD_ROUNDS")); err == nil {
		rounds = v
	}
	if v, err := strconv.Atoi(os.Getenv("NERVE_GREP_CALIB_SECS")); err == nil {
		calib = v
	}
	procs := runtime.GOMAXPROCS(0)
	workers := max(1, procs/2)
	f := newFixture()
	f.grant(domain.ActionRead)
	rng := rand.New(rand.NewPCG(9, 10))
	alt := loadAlternation()
	parsed, err := domain.ParseGrepPattern(alt, false)
	if err != nil {
		b.Fatal(err)
	}
	k := parsed.Insts
	guard := newGuardNodes(f.eng)
	roots := globRoots(guard.nodes)
	glob, err := domain.ParseGlob("glob", guardGlob())
	if err != nil {
		b.Fatal(err)
	}
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	// call is one grep of pattern over pages, its bounds work and
	// lineWork: its time, its work and the lines it matched.
	grepOf := func(pages loadPages, work, lineWork int64) *app.Grep {
		return app.NewGrep(app.GrepDeps{Notebooks: f.notebooks, Nodes: pages, States: pages, Reads: pages, Auth: f.auth,
			Parses: app.NewGrepParses(procs), Slots: app.NewGrepSlots(workers), Logger: discard, Clock: systemClock{},
			MaxWork: work, MaxLineWork: lineWork, Walk: 1 << 20})
	}
	// atLoad runs each in workers goroutines for secs, rounds times,
	// beside as many of the other CPU work over contents.
	atLoad := func(contents []string, secs, rounds int, each func(w int)) {
		for range rounds {
			stop := make(chan struct{})
			var wg sync.WaitGroup
			for w := range workers {
				wg.Add(2)
				go func() {
					defer wg.Done()
					for i := w; ; i++ {
						select {
						case <-stop:
							return
						default:
						}
						if i%2 == 0 {
							c := contents[i%len(contents)]
							domain.SearchTokens("a title", &c)
							continue
						}
						_, _ = glob.Walk(context.Background(), roots, "", domain.MaxGlobWork, func([]string, *domain.GlobNode) bool { return true })
					}
				}()
				go func() {
					defer wg.Done()
					for {
						select {
						case <-stop:
							return
						default:
						}
						each(w)
					}
				}()
			}
			time.Sleep(time.Duration(secs) * time.Second)
			close(stop)
			wg.Wait()
		}
	}
	b.Logf("GOMAXPROCS %d, %d grep + %d CPU workers; pattern %d bytes, GrepInsts %d; %d s x %d rounds a shape",
		procs, workers, workers, len(alt), k, secs, rounds)
	var unitP99 float64 // ns a unit at p99, the worst of the long lines' shapes
	for _, n := range []int{80, 4096} {
		var contents []string
		for range 8 {
			contents = append(contents, linesOf(n-1, 64<<10, func(n int) string { return denseText(n, rng) }))
		}
		g := grepOf(newLoadPages(f.eng, contents), 50_000_000, 0)
		var mu sync.Mutex
		var per []float64
		atLoad(contents, secs, rounds, func(int) {
			t0 := time.Now()
			got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: alt})
			d := time.Since(t0)
			if err != nil || got.Walk.Work == 0 {
				panic(fmt.Sprint(err))
			}
			mu.Lock()
			per = append(per, float64(d.Nanoseconds())/float64(got.Walk.Work))
			mu.Unlock()
		})
		q := func(p float64) float64 { return quantileOf(per, p) }
		unitP99 = max(unitP99, q(0.99))
		tier := 0
		for _, t := range []int{2, 3, 4, 5, 6} {
			if float64(t)*1e8*q(0.99) <= 5e9 {
				tier = t
			}
		}
		b.Logf("%d-byte lines: %d calls of 5e7, ns a unit median %.2f, p90 %.2f, p99 %.2f, max %.2f; a call at p99: 2e8 %.2f s, "+
			"3e8 %.2f s; MaxGrepWork's tier within 5 s: %d e8", n, len(per), q(0.5), q(0.9), q(0.99), quantileOf(per, 1),
			2e8*q(0.99)/1e9, 3e8*q(0.99)/1e9, tier)
	}
	line := denseText(domain.MaxContentBytes-1, rng) + "\n"
	for _, lineWork := range []int64{150_000_000, 75_000_000} {
		g := grepOf(newLoadPages(f.eng, []string{line}), domain.MaxGrepWork, lineWork)
		var mu sync.Mutex
		var per, times []float64
		atLoad([]string{line}, secs, rounds, func(int) {
			t0 := time.Now()
			got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: alt})
			d := time.Since(t0)
			if err != nil || got.Walk.Lines != 1 {
				panic(fmt.Sprint(err, got.Walk))
			}
			mu.Lock()
			per, times = append(per, float64(d.Nanoseconds())/float64(got.Walk.Work)), append(times, d.Seconds())
			mu.Unlock()
		})
		q := func(p float64) float64 { return quantileOf(per, p) }
		unitP99 = max(unitP99, q(0.99))
		tier := 0.0
		for _, t := range []float64{0.5, 0.75, 1, 1.5, 2, 3} {
			if t*1e8*q(0.99) <= 2e9 {
				tier = t
			}
		}
		b.Logf("a 5 MiB line cut at %.2ge8 (%d B): %d calls, ns a unit median %.2f, p90 %.2f, p99 %.2f, max %.2f; a line's time p99 "+
			"%.2f s; at p99 0.5e8 %.2f s, 0.75e8 %.2f s, 1e8 %.2f s; MaxGrepLineWork's tier within 2 s: %.2f e8", float64(lineWork)/1e8,
			domain.GrepLineCut(k, lineWork), len(per), q(0.5), q(0.9), q(0.99), quantileOf(per, 1), quantileOf(times, 0.99),
			0.5e8*q(0.99)/1e9, 0.75e8*q(0.99)/1e9, 1e8*q(0.99)/1e9, tier)
	}
	calibrate(b, f, grepOf, func(contents []string, each func(int)) { atLoad(contents, calib, 1, each) }, rng, alt)
	b.Logf("GrepUnitTime at p99, the worst of the long lines' shapes: %.2f ns", unitP99)
}

// globRoots are nodes' trees, as the use case builds them.
func globRoots(nodes []domain.Node) []*domain.GlobNode {
	byID := make(map[uuid.UUID]*domain.GlobNode, len(nodes))
	for i := range nodes {
		byID[nodes[i].ID] = &domain.GlobNode{Node: &nodes[i]}
	}
	var roots []*domain.GlobNode
	for i := range nodes {
		n := byID[nodes[i].ID]
		if p := nodes[i].ParentID; p == nil {
			roots = append(roots, n)
		} else {
			byID[*p].Children = append(byID[*p].Children, n)
		}
	}
	return roots
}

// calibrate is GrepLineOverhead at full load through the use case: U,
// the long line's ns a unit for each of the costly patterns, a call over
// a page of one line of 64 KiB each; and each cell of 22 of exp/lineoh's
// 24 line patterns over lines of 0 to 200 bytes, prose and dense, its ns a
// line a call over a page of 16 KiB of them; L is the largest over the
// cells of (ns a line) / (insts × U) - bytes, at the median and at p90.
// The two left out (review C-n1), the six-branch alternation of
// class{60}Q to class{55}QZZZZZ and class{999}class{990}Q, are programs
// of hundreds of instructions and more, whose fixed cost a line is a
// small part of a unit (L under 0.4 in exp/lineoh); the largest L is the
// few-instruction patterns', and the 36-branch alternation stands for the
// largest program; class{999}class{990}Q is one of U's cells.
func calibrate(b *testing.B, f *fixture, grepOf func(loadPages, int64, int64) *app.Grep, atLoad func([]string, func(int)),
	rng *rand.Rand, alt string,
) {
	type cell struct {
		name    string
		pattern string
		n       int
		insts   int64
		unit    bool
		g       *app.Grep
		lines   int
		hits    bool // a line of it matches: the call stops at its limit
		samples []float64
	}
	classLong := `[\p{L}\p{N}\p{M}\p{S}\p{P}]`
	words := strings.Fields("the of and to in is that for it as was with be by on not he this are or his from at which but have an")
	prose := func(n int) string {
		var s strings.Builder
		for s.Len() < n {
			if s.Len() > 0 {
				s.WriteByte(' ')
			}
			s.WriteString(words[rng.IntN(len(words))])
		}
		return s.String()[:n]
	}
	dense := func(n int) string { return denseText(n, rng) }
	insts := func(p string) int64 {
		parsed, err := domain.ParseGrepPattern(p, false)
		if err != nil {
			b.Fatal(err)
		}
		return parsed.Insts
	}
	var cells []*cell
	var contents []string
	long := denseText(64<<10, rng) + "\n"
	for _, p := range []string{classLong + "{200}Q", classLong + "{999}" + classLong + "{990}Q", alt, `\pL{200}Q`, `[a-z]{200}Q`} {
		cells = append(cells, &cell{name: "unit " + p[:min(len(p), 24)], pattern: p, insts: insts(p), unit: true,
			g: grepOf(newLoadPages(f.eng, []string{long}), math.MaxInt64/4, math.MaxInt64/4)})
	}
	patterns := []string{
		`Q`, `QZ`, `foo`, `(?i)qz`, `.`, `\w`, `^$`, `^Q`, `Q$`, `\bQ`, `data:image`,
		`TODO: \w+`, `(?i)todo:\s+\w+`, `(?i)(error|warning|fail)Q`, `\d{4}-\d{2}-\d{2}Q`,
		`^#{1,6} Q`, `[a-z]{17}Q`, classLong + `{17}Q`, `\pL+Q`, `(?:a|b|c|d|e|f|g|h){8}Q`,
		classLong + `{60}Q`, alt,
	}
	for _, kind := range []string{"prose", "dense"} {
		mk := prose
		if kind == "dense" {
			mk = dense
		}
		for _, p := range patterns {
			for _, n := range []int{0, 1, 4, 16, 40, 80, 200} {
				size := 16 << 10
				if insts(p) > 500 && n > 16 {
					size = 4 << 10
				}
				content := linesOf(n, size, mk)
				contents = append(contents, content)
				name := p
				if p == alt {
					name = "alternation"
				}
				re := regexp.MustCompile(p)
				hits := slices.ContainsFunc(strings.Split(content, "\n"), re.MatchString)
				cells = append(cells, &cell{name: fmt.Sprintf("%s, %s lines of %d", name[:min(len(name), 30)], kind, n), pattern: p, n: n,
					insts: insts(p), lines: strings.Count(content, "\n"), hits: hits,
					g: grepOf(newLoadPages(f.eng, []string{content}), math.MaxInt64/4, math.MaxInt64/4)})
			}
		}
	}
	var mu sync.Mutex
	atLoad(contents, func(w int) {
		c := cells[rand.IntN(len(cells))]
		t0 := time.Now()
		got, err := c.g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: c.pattern})
		d := float64(time.Since(t0).Nanoseconds())
		if err != nil {
			panic(err)
		}
		v := d / float64(got.Walk.Lines)
		if c.unit {
			v = d / float64(got.Walk.Work)
		}
		mu.Lock()
		c.samples = append(c.samples, v)
		mu.Unlock()
	})
	least := math.MaxInt
	for _, c := range cells {
		least = min(least, len(c.samples))
	}
	if least == 0 {
		b.Logf("a cell has no sample: run longer (NERVE_GREP_LOAD_SECS)")
		return
	}
	b.Logf("calibration: %d cells, %d samples a cell at least", len(cells), least)
	for _, q := range []float64{0.5, 0.9} {
		U, uAt := 0.0, ""
		for _, c := range cells {
			if u := quantileOf(c.samples, q); c.unit && u > U {
				U, uAt = u, c.name
			}
		}
		worst, worstAt, hitLine, hitAt, skipped := math.Inf(-1), "", 0.0, "", 0
		for _, c := range cells {
			switch {
			case c.unit:
				continue
			case c.hits:
				// A hit's line costs its cursor and its answer's item
				// besides: the limit, not the work, bounds those, 500 a
				// call at most (M9/P4 design 3.6.3), so it is no cell of L.
				if t := quantileOf(c.samples, q); t > hitLine {
					hitLine, hitAt = t, c.name
				}
				skipped++
				continue
			}
			if l := quantileOf(c.samples, q)/(float64(c.insts)*U) - float64(c.n); l > worst {
				worst, worstAt = l, fmt.Sprintf("%s, %.0f ns a line", c.name, quantileOf(c.samples, q))
			}
		}
		b.Logf("quantile %.1f: U %.2f ns (%s); the largest L %.2f (%s); GrepLineOverhead %d; %d cells of hits left out, their "+
			"lines %.0f ns at most, the call's own time among them (%s)", q, U, uAt, worst, worstAt, int64(math.Ceil(worst)), skipped,
			hitLine, hitAt)
	}
}

// BenchmarkGrepParsesOfTheWorst re-measures the parse queue (M9/P4 design
// 3.6.1, 6.2; exp/parseq's way, through GrepParses and ParseGrepPattern):
// 200 calls at once, one token's requests, of each of review A's worst
// patterns, with the queue's GOMAXPROCS turns and without it (200): the
// heap's peak over its base, the bytes allocated, the burst's time and the
// longest and the median time a call takes. Run by hand, never in CI:
//
//	go test -run '^$' -bench '^BenchmarkGrepParsesOfTheWorst$' -benchtime 1x ./internal/modules/page/app/
func BenchmarkGrepParsesOfTheWorst(b *testing.B) {
	const calls = 200
	wide := `(?i:[\x{100}-\x{10ffff}])`
	for _, p := range []struct{ name, pattern string }{
		{`\pL x 341`, strings.Repeat(`\pL`, 341)}, {`\PL x 341`, strings.Repeat(`\PL`, 341)},
		{`(?i)\pL x 340`, "(?i)" + strings.Repeat(`\pL`, 340)}, {`^\pL? x 255`, "^" + strings.Repeat(`\pL?`, 255)},
		{`[\pL0] x 170`, strings.Repeat(`[\pL0]`, 170)}, {wide + ` x 40`, strings.Repeat(wide, 40)},
	} {
		for _, turns := range []int{runtime.GOMAXPROCS(0), calls} {
			q := app.NewGrepParses(turns)
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)
			peak := before.HeapAlloc
			stop, sampled := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(sampled)
				var m runtime.MemStats
				for {
					select {
					case <-stop:
						return
					default:
					}
					runtime.ReadMemStats(&m)
					peak = max(peak, m.HeapAlloc)
					time.Sleep(500 * time.Microsecond)
				}
			}()
			took := make([]time.Duration, calls)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range calls {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					t0 := time.Now()
					if _, err := q.Parse(context.Background(), p.pattern, false); err != nil {
						panic(err)
					}
					took[i] = time.Since(t0)
				}()
			}
			t0 := time.Now()
			close(start)
			wg.Wait()
			burst := time.Since(t0)
			close(stop)
			<-sampled
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			slices.Sort(took)
			b.Logf("%s (%d bytes), %d calls, %d turns: burst %v, a call's longest %v, median %v; peak heap over base %.0f MB, "+
				"allocated %.0f MB", p.name, len(p.pattern), calls, turns, burst.Round(time.Millisecond),
				took[calls-1].Round(time.Millisecond), took[calls/2].Round(time.Millisecond), float64(peak-before.HeapAlloc)/1e6,
				float64(after.TotalAlloc-before.TotalAlloc)/1e6)
		}
	}
}
