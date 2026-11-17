package sample

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// GC is a line of the Go runtime's gc trace (GODEBUG=gctrace=1): its
// number, when it began on the wall clock, the heap at its start, at its
// end and live after it, and its goal, in MiB (the runtime shifts them by
// 20, runtime/mgc.go).
type GC struct {
	N                      int
	At                     time.Time
	Start, End, Live, Goal int64
	Forced                 bool
}

// gcLine is a trace line's number, its seconds since the runtime began,
// to the millisecond, and its heaps: "gc 12 @3.456s 1%: …, 5->6->3 MB, 6 MB
// goal, …".
var gcLine = regexp.MustCompile(`^gc (\d+) @(\d+)\.(\d{3})s .*?, (\d+)->(\d+)->(\d+) MB, (\d+) MB goal, .*?( \(forced\))?$`)

// jsonOpen begins a line of the serve's JSON log (log/slog's handler).
const jsonOpen = `{"time":`

// StripJSON takes each of the serve's JSON log lines out of log, wherever
// it begins, with its newline: the runtime writes a trace line in some
// thirty writes (runtime/mgc.go, runtime/print.go), and a request's log
// line lands between two of them, so that taking it out joins the trace
// line's halves (review A's M4). A line is an object from `{"time":` to
// its matching brace, strings and their escapes read; one cut short by the
// log's end is dropped.
func StripJSON(log []byte) []byte {
	out := make([]byte, 0, len(log))
	for i := 0; i < len(log); {
		if !bytes.HasPrefix(log[i:], []byte(jsonOpen)) {
			out = append(out, log[i])
			i++
			continue
		}
		end := objectEnd(log, i)
		if end < 0 {
			break
		}
		i = end
		if i < len(log) && log[i] == '\n' {
			i++
		}
	}
	return out
}

// objectEnd is the index just past the object that begins at from, or -1
// when the log ends first.
func objectEnd(b []byte, from int) int {
	depth, inString, escaped := 0, false, false
	for i := from; i < len(b); i++ {
		c := b[i]
		switch {
		case inString && escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case inString && c == '"':
			inString = false
		case inString:
		case c == '"':
			inString = true
		case c == '{':
			depth++
		case c == '}':
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// ParseGC reads the trace lines of a serve's log, the process started at
// started: each line's time is started and its @T. Its JSON lines go first
// (StripJSON). It answers the lines in order and how many began "gc " but
// did not read as a trace line.
func ParseGC(log []byte, started time.Time) ([]GC, int, error) {
	var gcs []GC
	broken := 0
	sc := bufio.NewScanner(bytes.NewReader(StripJSON(log)))
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 3 || line[:3] != "gc " {
			continue
		}
		m := gcLine.FindStringSubmatch(line)
		if m == nil {
			broken++
			continue
		}
		n := make([]int64, 7)
		for i := range n {
			v, err := strconv.ParseInt(m[i+1], 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("%q: %w", line, err)
			}
			n[i] = v
		}
		at := started.Add(time.Duration(n[1]*1000+n[2]) * time.Millisecond)
		gcs = append(gcs, GC{N: int(n[0]), At: at, Start: n[3], End: n[4], Live: n[5], Goal: n[6], Forced: m[8] != ""})
	}
	return gcs, broken, sc.Err()
}

// Window is what a reading of the trace covers: the base, a time at or
// just before the round's baseline line, the forced collection after its
// idle 130 s (design 3.4.2), and a combination's from and to.
type Window struct{ Base, From, To time.Time }

// Report is a combination's heap, in MiB: the round's baseline, the live
// heap of the first line at or after Base; the peak, the larger of each
// window line's start and end; the upper bound of the window's end, the
// larger of the peak and the goal of the first line after the window (the
// heap grows to it at most before that collection); the largest goal in
// the window; its lines; the numbers missing from the baseline's line to
// the last line read (the line after the window, or the window's last).
type Report struct {
	BaseN, AfterN         int
	Baseline, Peak, Upper int64
	Delta, UpperDelta     int64
	MaxGoal               int64
	GCs                   int
	Missing               []int
}

// Read reads gcs, in order, over w.
func Read(gcs []GC, w Window) (Report, error) {
	var r Report
	base := -1
	for i, g := range gcs {
		if !g.At.Before(w.Base) {
			base = i
			break
		}
	}
	if base < 0 {
		return Report{}, errors.New("no trace line at or after the base")
	}
	r.BaseN, r.Baseline = gcs[base].N, gcs[base].Live
	last := gcs[base].N
	for _, g := range gcs[base:] {
		if g.At.After(w.To) {
			r.AfterN, r.Upper, last = g.N, g.Goal, g.N
			break
		}
		last = g.N
		if g.At.Before(w.From) {
			continue
		}
		r.GCs++
		r.Peak, r.MaxGoal = max(r.Peak, g.Start, g.End), max(r.MaxGoal, g.Goal)
	}
	r.Upper = max(r.Upper, r.Peak)
	r.Delta, r.UpperDelta = r.Peak-r.Baseline, r.Upper-r.Baseline
	seen := map[int]bool{}
	for _, g := range gcs {
		seen[g.N] = true
	}
	for n := r.BaseN; n <= last; n++ {
		if !seen[n] {
			r.Missing = append(r.Missing, n)
		}
	}
	return r, nil
}

// String is the report as key=value, MiB.
func (r Report) String() string {
	after := "none"
	if r.AfterN > 0 {
		after = strconv.Itoa(r.AfterN)
	}
	return fmt.Sprintf("baseline_mib=%d base_gc=%d peak_mib=%d delta_mib=%d upper_mib=%d upper_delta_mib=%d after_gc=%s max_goal_mib=%d gcs=%d missing=%v",
		r.Baseline, r.BaseN, r.Peak, r.Delta, r.Upper, r.UpperDelta, after, r.MaxGoal, r.GCs, r.Missing)
}
