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
// 20, runtime/mgc.go); and whether it was forced: by the runtime after two
// minutes without a collection, which writes a line "GC forced" before it
// (runtime/proc.go's forcegchelper), or by runtime.GC, which ends its line
// with " (forced)".
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

// forcedLine is the line the runtime writes before a collection it forces
// after two minutes without one.
const forcedLine = "GC forced"

// ParseGC reads the trace lines of a serve's log, the process started at
// started: each line's time is started and its @T. Its JSON lines go first
// (StripJSON). It answers the lines in order and how many began "gc " but
// did not read as a trace line.
func ParseGC(log []byte, started time.Time) ([]GC, int, error) {
	var gcs []GC
	broken := 0
	forced := false // the line before was "GC forced"
	sc := bufio.NewScanner(bytes.NewReader(StripJSON(log)))
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == forcedLine {
			forced = true
			continue
		}
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
		gcs = append(gcs, GC{N: int(n[0]), At: at, Start: n[3], End: n[4], Live: n[5], Goal: n[6], Forced: forced || m[8] != ""})
		forced = false
	}
	return gcs, broken, sc.Err()
}

// Window is what a reading of the trace covers: the base, a time at or
// just before the round's baseline line, the collection the runtime forces
// two minutes into the round's idle of 130 s (design 3.4.2), and a
// combination's from and to. The base is not after from.
type Window struct{ Base, From, To time.Time }

// Report is a combination's heap, in MiB: the round's baseline, the live
// heap of the first line at or after Base, and whether that line was a
// forced collection; the peak, the larger of each window line's start and
// end, none when the window has no line; the upper bound of the window's
// end, the larger of the peak and the goal of the first line after the
// window (the heap grows to it at most before that collection); the
// largest goal in the window; its lines; the numbers missing from the
// baseline's line to the last line read (the line after the window, or
// the window's last).
type Report struct {
	BaseN, AfterN         int
	BaseForced            bool
	Baseline, Peak, Upper int64
	Delta, UpperDelta     int64
	MaxGoal               int64
	GCs                   int
	Missing               []int
}

// Read reads gcs, in order, over w: an error when the base is after the
// window's start, or there is no line at or after it.
func Read(gcs []GC, w Window) (Report, error) {
	if w.Base.After(w.From) {
		return Report{}, fmt.Errorf("the base %v is after the window's start %v: the baseline is read before the window", w.Base, w.From)
	}
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
	r.BaseN, r.Baseline, r.BaseForced = gcs[base].N, gcs[base].Live, gcs[base].Forced
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

// String is the report as key=value, MiB. A window without a collection
// says so: its peak and delta are none (no line saw the heap in it), and
// its upper bound is the next collection's goal, none without one.
func (r Report) String() string {
	after, peak, delta, upper, upperDelta := "none", "none", "none", "none", "none"
	if r.AfterN > 0 {
		after = strconv.Itoa(r.AfterN)
	}
	if r.GCs > 0 {
		peak, delta = strconv.FormatInt(r.Peak, 10), strconv.FormatInt(r.Delta, 10)
	}
	if r.GCs > 0 || r.AfterN > 0 {
		upper, upperDelta = strconv.FormatInt(r.Upper, 10), strconv.FormatInt(r.UpperDelta, 10)
	}
	s := fmt.Sprintf("baseline_mib=%d base_gc=%d base_forced=%t peak_mib=%s delta_mib=%s upper_mib=%s upper_delta_mib=%s after_gc=%s "+
		"max_goal_mib=%d gcs=%d missing=%v", r.Baseline, r.BaseN, r.BaseForced, peak, delta, upper, upperDelta, after, r.MaxGoal, r.GCs,
		r.Missing)
	if r.GCs == 0 {
		s += " no_gc_in_window"
	}
	return s
}
