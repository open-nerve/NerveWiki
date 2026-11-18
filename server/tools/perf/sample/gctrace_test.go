package sample

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// traceLog is a serve's log as GODEBUG=gctrace=1 and its JSON log leave
// it: JSON lines alone, the round's collection the runtime forced after
// two minutes of its idle (a line "GC forced" before it, as a real log
// has it), a collection between it and the window, one in the window
// whose end is past its start, one a request's log line cut in two (a
// brace in a string of it), one missing, one after the window whose goal
// is the largest, and one runtime.GC forced.
func traceLog() string {
	return strings.Join([]string{
		`{"time":"2026-11-17T10:00:00.1Z","level":"INFO","msg":"jobs started"}`,
		`gc 1 @0.010s 1%: 0.012+1.1+0.003 ms clock, 0.10+0.2/0.9/0.3+0.02 ms cpu, 4->4->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`GC forced`,
		`gc 2 @130.500s 0%: 0.020+0.9+0.004 ms clock, 0.16+0/0.8/0.2+0.03 ms cpu, 3->3->3 MB, 6 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`gc 3 @140.000s 0%: 0.02+1.0+0.004 ms clock, 0.16+0.1/0.8/0.2+0.03 ms cpu, 6->6->5 MB, 6 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`{"time":"2026-11-17T10:02:30.5Z","level":"INFO","msg":"request","path":"/api/v0/pages/{id}","status":200}`,
		`gc 4 @151.000s 1%: 0.03+2.0+0.005 ms clock, 0.2+0.4/1.9/0.5+0.04 ms cpu, 10->40->20 MB, 12 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`gc 5 @152.000s 2%: 0.04+` + `{"time":"2026-11-17T10:02:32Z","level":"INFO","msg":"req \"x}\"","path":"/a{b}","s":"}"}` + "\n" +
			`3.1+0.006 ms clock, 0.3+0.5/2.9/0.6+0.05 ms cpu, 50->70->22 MB, 41 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`gc 7 @158.000s 1%: 0.03+1.5+0.005 ms clock, 0.2+0.3/1.4/0.4+0.04 ms cpu, 20->21->18 MB, 44 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`{"time":"2026-11-17T10:02:45Z","level":"INFO","msg":"request"}`,
		`gc 8 @170.000s 1%: 0.03+1.5+0.005 ms clock, 0.2+0.3/1.4/0.4+0.04 ms cpu, 30->31->10 MB, 90 MB goal, 0 MB stacks, 0 MB globals, 8 P`,
		`gc 9 @171.000s 1%: 0.03+1.5+0.005 ms clock, 0.2+0.3/1.4/0.4+0.04 ms cpu, 12->12->10 MB, 20 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)`,
		`{"time":"2026-11-17T10:03:00Z","level":"INFO","msg":"cut sho`,
	}, "\n") + "\n"
}

// The trace's reading (design 3.4.2, 7.2's 21 to 26): the times by the
// process's start, a line cut by a JSON log line joined, JSON lines
// skipped, the baseline the base's line, the peak each line's larger of
// start and end, the upper bound the first goal after the window, a
// missing number reported.
func TestTheGCTraceReadsTheWindowAgainstTheBase(t *testing.T) {
	started := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	gcs, broken, err := ParseGC([]byte(traceLog()), started)
	if err != nil || broken != 0 {
		t.Fatalf("parse: %d broken, %v", broken, err)
	}
	var numbers []int
	for _, g := range gcs {
		numbers = append(numbers, g.N)
	}
	if !slices.Equal(numbers, []int{1, 2, 3, 4, 5, 7, 8, 9}) {
		t.Fatalf("lines %v", numbers)
	}
	if g := gcs[4]; g.N != 5 || g.Start != 50 || g.End != 70 || g.Live != 22 || g.Goal != 41 ||
		!g.At.Equal(started.Add(152*time.Second)) || g.Forced {
		t.Errorf("the cut line %+v", g)
	}
	if !gcs[1].Forced || !gcs[1].At.Equal(started.Add(130500*time.Millisecond)) || gcs[2].Forced || !gcs[7].Forced {
		t.Errorf("the forced lines %+v, %+v; the one after the runtime's %+v", gcs[1], gcs[7], gcs[2])
	}
	at := func(s int) time.Time { return started.Add(time.Duration(s) * time.Second) }
	r, err := Read(gcs, Window{Base: at(130), From: at(150), To: at(160)})
	if err != nil {
		t.Fatal(err)
	}
	want := Report{BaseN: 2, AfterN: 8, BaseForced: true, Baseline: 3, Peak: 70, Upper: 90, Delta: 67, UpperDelta: 87, MaxGoal: 44, GCs: 3,
		Missing: []int{6}}
	if r.BaseN != want.BaseN || r.AfterN != want.AfterN || r.BaseForced != want.BaseForced || r.Baseline != want.Baseline ||
		r.Peak != want.Peak || r.Upper != want.Upper || r.Delta != want.Delta || r.UpperDelta != want.UpperDelta ||
		r.MaxGoal != want.MaxGoal || r.GCs != want.GCs || !slices.Equal(r.Missing, want.Missing) {
		t.Errorf("report %+v, want %+v", r, want)
	}
	if got := r.String(); got != "baseline_mib=3 base_gc=2 base_forced=true peak_mib=70 delta_mib=67 upper_mib=90 upper_delta_mib=87 "+
		"after_gc=8 max_goal_mib=44 gcs=3 missing=[6]" {
		t.Errorf("report %q", got)
	}
	// A window without a collection says so (review B's M5): no peak, no
	// delta; its upper bound the next line's goal.
	r, err = Read(gcs, Window{Base: at(130), From: at(160), To: at(165)})
	if err != nil || r.GCs != 0 || r.String() != "baseline_mib=3 base_gc=2 base_forced=true peak_mib=none delta_mib=none upper_mib=90 "+
		"upper_delta_mib=87 after_gc=8 max_goal_mib=0 gcs=0 missing=[6] no_gc_in_window" {
		t.Errorf("a window without a collection: %q, %v", r.String(), err)
	}
	// A base that is no forced collection says so; one after the window's
	// start is an error.
	if r, err := Read(gcs, Window{Base: at(139), From: at(150), To: at(160)}); err != nil || r.BaseN != 3 || r.BaseForced {
		t.Errorf("a base at the third line: %+v, %v", r, err)
	}
	if r, err := Read(gcs, Window{Base: at(155), From: at(150), To: at(160)}); err == nil {
		t.Errorf("a base after the window's start: %+v", r)
	}
	// No line after the window: the upper bound is the peak's, the lines
	// counted to the last.
	r, err = Read(gcs, Window{Base: at(130), From: at(150), To: at(180)})
	if err != nil || r.AfterN != 0 || r.Upper != 70 || r.GCs != 5 || r.MaxGoal != 90 || !slices.Equal(r.Missing, []int{6}) {
		t.Errorf("to the log's end: %+v, %v", r, err)
	}
	if _, err := Read(gcs, Window{Base: at(200), From: at(200), To: at(210)}); err == nil {
		t.Error("a base after every line read")
	}
	if got := string(StripJSON([]byte("a{\"time\":\"x\"}\nb\n{\"time\":\"y\",\"z\":{\"w\":1}}\nc"))); got != "ab\nc" {
		t.Errorf("stripped %q", got)
	}
	if _, broken, _ := ParseGC([]byte("gc 1 @0.0s 0%: cut\n"), started); broken != 1 {
		t.Errorf("a line of gc that is no trace: %d broken", broken)
	}
}
