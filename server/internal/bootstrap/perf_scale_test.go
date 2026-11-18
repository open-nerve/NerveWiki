package bootstrap

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// guardScale is what a cost guard runs at (M12/P4 design 3.3): its size,
// its deadlines and its client's, and whether it judges its bounds.
type guardScale struct {
	Size   int // pages, sources or changesets
	ByHand bool
	// Request is the app's request deadline, Own the guarded call's:
	// lint's worker when Worker, else the start's or the approval's.
	Request, Own time.Duration
	Worker       bool
	Read, Write  time.Duration
	Client       time.Duration // the timed request's client's
}

// The four guards' own, as they ran before M12/P4 but lint's (M12
// decision 475), and run unless a size is given by hand: written out, not
// derived, so that the short test below holds each guard to its numbers
// (05-lessons section 1: a deadline is derived from a bound, then checked
// against the configuration the app starts with, never against itself).
// The check and lint call the module in process: no read or write
// timeout, no client.

// startCostScale is the start's guard's: a notebook of 50,000 pages, a
// 6 s request, the start's three quarters of it, a write timeout pinned at
// 15 s and a client of twice the request.
func startCostScale() guardScale {
	return guardScale{Size: 50_000, Request: 6 * time.Second, Own: 4500 * time.Millisecond, Read: 5 * time.Second,
		Write: 15 * time.Second, Client: 12 * time.Second}
}

// checkCostScale is the check's guard's: 50,000 pages, a 6 s request and
// a 3 s worker, lint's cost configuration as it was before decision 475
// (lint's own guard has twice them).
func checkCostScale() guardScale {
	return guardScale{Size: 50_000, Request: 6 * time.Second, Own: 3 * time.Second, Worker: true}
}

// approveCostScale is the approval's guard's: 47,500 sources, 500
// folders of 95, production's 15 s request, the approval's three quarters
// of it, a write timeout of 45 s and a client of twice the request.
func approveCostScale() guardScale {
	return guardScale{Size: 47_500, Request: 15 * time.Second, Own: 11250 * time.Millisecond, Read: 5 * time.Second,
		Write: 45 * time.Second, Client: 30 * time.Second}
}

// lintCostScale is the lint guard's: 100,000 pages, a 12 s request and a
// 6 s worker, above the summary's wall bound of 5.2 s (M12 decision 475).
func lintCostScale() guardScale {
	return guardScale{Size: 100_000, Request: 12 * time.Second, Own: 6 * time.Second, Worker: true}
}

// productionRequest is config.yaml's server.request_timeout, which a size
// given by hand runs under unless NW_SCALE_REQUEST says otherwise.
const productionRequest = 15 * time.Second

// minScaleSize is the least size by hand: 20,000, the size the design's
// trial ran the four guards at and each passed. The switch is for sizes
// past a guard's own, and below this the guards' checks of correctness do
// not follow the size: lint's orphans reach their cap of 10,000 only past
// 10,003 pages, and the start's seed reaches its 1,000 wanted pages only
// in a larger notebook (69 of 1,000 pages; review C's M6, the same as the
// long log's floor, review A's M1).
const minScaleSize = 20_000

// guardScaleOf is def, a guard's own, unless sizeVar (NW_SCALE_PAGES)
// gives a size by hand: then production's request deadline, or
// NW_SCALE_REQUEST's, the call's own deadline by production's formula
// (bootstrap/wire.go: lint's worker min(10 s, request/2), the start's and
// the approval's request*3/4; the worker uncapped under NW_SCALE_REQUEST,
// which is no production's formula, so that a run sees a time past 10 s),
// the test configuration's read timeout, a write timeout past the read and
// the request (the configuration's check: read + request < write) and a
// client that outwaits the request twice (00-M12 decision 359). A bad
// value, a size under minScaleSize, or NW_SCALE_REQUEST without a size, is
// an error.
func guardScaleOf(getenv func(string) string, sizeVar string, def guardScale) (guardScale, error) {
	size, request := getenv(sizeVar), getenv("NW_SCALE_REQUEST")
	if size == "" {
		if request != "" {
			return guardScale{}, fmt.Errorf("NW_SCALE_REQUEST=%q without %s: the guard's own deadlines judge its own size", request, sizeVar)
		}
		return def, nil
	}
	n, err := strconv.Atoi(size)
	if err != nil || n < 1 {
		return guardScale{}, fmt.Errorf("%s=%q is no positive size", sizeVar, size)
	}
	if n < minScaleSize {
		return guardScale{}, fmt.Errorf("%s=%d is under %d, the size the guards were tried at: below it their checks of "+
			"correctness do not follow the size (lint's capped orphans, the start's 1,000 wanted pages)", sizeVar, n, minScaleSize)
	}
	s := guardScale{Size: n, ByHand: true, Request: productionRequest, Worker: def.Worker, Read: 5 * time.Second}
	if request != "" {
		if s.Request, err = time.ParseDuration(request); err != nil || s.Request <= 0 {
			return guardScale{}, fmt.Errorf("NW_SCALE_REQUEST=%q is no positive duration", request)
		}
	}
	switch {
	case !def.Worker:
		s.Own = s.Request * 3 / 4
	case request != "":
		s.Own = s.Request / 2
	default:
		s.Own = min(10*time.Second, s.Request/2)
	}
	s.Write, s.Client = s.Read+s.Request+15*time.Second, 2*s.Request
	return s, nil
}

// scaleOf is the guard's scale, def unless the environment gives a size;
// a bad value fails t. A scale by hand is logged.
func scaleOf(t testing.TB, sizeVar string, def guardScale) guardScale {
	t.Helper()
	s, err := guardScaleOf(os.Getenv, sizeVar, def)
	if err != nil {
		t.Fatal(err)
	}
	if s.ByHand {
		t.Logf("scale=%d by hand: request %v, the call's own %v, read %v, write %v, client %v; the bounds logged, not judged",
			s.Size, s.Request, s.Own, s.Read, s.Write, s.Client)
	}
	return s
}

// scaleLogged is the size a guard's log line adds, by hand; none, 0,
// unless, so that a line of its own size is as it was.
func scaleLogged(s guardScale) int {
	if s.ByHand {
		return s.Size
	}
	return 0
}

// judge reports value over bound through errorf, unless s runs by hand.
func (s guardScale) judge(errorf func(string, ...any), name string, value, bound uint64) {
	if !s.ByHand && value > bound {
		errorf("%s: %d, past its bound %d", name, value, bound)
	}
}

// waits is nil when the timed request's client outwaits the guarded call
// and the server's write timeout outlasts its read and its request: else
// the client or the server ends the request before the call's deadline
// does, and judges in its place (00-M12 decision 359).
func (s guardScale) waits() error {
	if s.Client <= s.Own || s.Write <= s.Read+s.Request {
		return fmt.Errorf("the client waits %v for a call of %v; the write timeout is %v for a read of %v and a request of %v: "+
			"want the client past the call, the write past both", s.Client, s.Own, s.Write, s.Read, s.Request)
	}
	return nil
}

// checkWaits fails t unless s.waits.
func (s guardScale) checkWaits(t testing.TB) {
	t.Helper()
	if err := s.waits(); err != nil {
		t.Fatal(err)
	}
}

// timedAsk is ask, timed, by a client of s.Client, which outwaits the
// guarded call (client(), 5 s, would judge in its place): it checks the
// answer against the contract and answers its status, its body and how
// long it took.
func timedAsk(t *testing.T, s guardScale, contract *apitest.Contract, method, url, token, body string) (int, string, time.Duration) {
	t.Helper()
	c := &http.Client{Timeout: s.Client}
	if c.Timeout != s.Client || c.Timeout <= s.Own {
		t.Fatalf("the timed request's client waits %v, want %v, past the call's %v", c.Timeout, s.Client, s.Own)
	}
	var payload []byte
	if body != "" {
		payload = []byte(body)
	}
	req := newRequest(t, method, url, token, payload)
	at := time.Now()
	res, answer := sendRequestBy(t, c, req)
	took := time.Since(at)
	contract.CheckResponse(t, req, res)
	return res.StatusCode, string(answer), took
}

// approveCostFoldersOf is the approval guard's folders of s's sources,
// perFolder each (a folder, its pages and its attachments): 500 of 47,500
// in 95s, round(n/95) of n by hand.
func approveCostFoldersOf(s guardScale, perFolder int) int {
	return int(math.Round(float64(s.Size) / float64(perFolder)))
}

// benchLevels are the levels name lists, comma-separated, as manyTops
// reads NERVE_BENCH_TOPS (revert_cost_test.go), each parsed by parse and
// each once; def unless it is set (M12/P4 design 3.3). A level parse does
// not take, an empty one or one twice is an error.
func benchLevels[T comparable](getenv func(string) string, name string, def []T, parse func(string) (T, error)) ([]T, error) {
	s := getenv(name)
	if s == "" {
		return def, nil
	}
	var out []T
	for f := range strings.SplitSeq(s, ",") {
		v, err := parse(f)
		if err != nil {
			return nil, fmt.Errorf("%s=%q: %w", name, s, err)
		}
		if slices.Contains(out, v) {
			return nil, fmt.Errorf("%s=%q names %q twice", name, s, f)
		}
		out = append(out, v)
	}
	return out, nil
}

// trashRestoreLevels are BenchmarkTrashRestore's loads, by name, and its
// attachments: light and full, and 0, 1,000, 2,000, 5,000 and 10,000,
// unless NERVE_BENCH_LOADS or NERVE_BENCH_ATTACHMENTS names others.
func trashRestoreLevels(getenv func(string) string) (loads []string, attachments []int, err error) {
	loads, err = benchLevels(getenv, "NERVE_BENCH_LOADS", []string{"light", "full"}, func(s string) (string, error) {
		if s != "light" && s != "full" {
			return "", fmt.Errorf("%q is no load, light or full", s)
		}
		return s, nil
	})
	if err != nil {
		return nil, nil, err
	}
	attachments, err = benchLevels(getenv, "NERVE_BENCH_ATTACHMENTS", []int{0, 1000, 2000, 5000, 10000}, countOf(0))
	if err != nil {
		return nil, nil, err
	}
	return loads, attachments, nil
}

// trashHistoryLevels are BenchmarkTrashRestoreHistory's versions a page,
// each tier written on the one before: 250, 500, 1,000, 1,500 and 2,000,
// unless NERVE_BENCH_VERSIONS names others, rising from 2 (a page's first
// version is its creation's). One alone writes its versions from 1 at once.
func trashHistoryLevels(getenv func(string) string) ([]int, error) {
	versions, err := benchLevels(getenv, "NERVE_BENCH_VERSIONS", []int{250, 500, 1000, 1500, 2000}, countOf(2))
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(versions); i++ {
		if versions[i] < versions[i-1] {
			return nil, fmt.Errorf("NERVE_BENCH_VERSIONS: %d after %d, want them rising, each tier written on the one before", versions[i],
				versions[i-1])
		}
	}
	return versions, nil
}

// countOf parses a count of at least least.
func countOf(least int) func(string) (int, error) {
	return func(s string) (int, error) {
		n, err := strconv.Atoi(s)
		if err != nil || n < least {
			return 0, fmt.Errorf("%q is no count of %d at least", s, least)
		}
		return n, nil
	}
}

// The guards' scales are their own unless a size is given (M12/P4 design
// 3.3, 7.2's 9 to 18): unset, each guard's literal numbers, by no other
// constant; NW_SCALE_PAGES set, production's request, the call's deadline
// by its formula, a client of twice the request and a write timeout past
// the read and the request, the approval's folders round(n/95), and only
// logged; NW_SCALE_REQUEST its request, lint's worker half of it past
// 10 s; a bad value an error; the timed requests' waits hold, and fail at
// their edges; judge errs past the bound unless by hand.
func TestGuardScaleKeepsEachGuardsOwnUnlessGiven(t *testing.T) {
	const s = time.Second
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	for _, c := range []struct {
		name string
		def  guardScale
		vars map[string]string
		want guardScale
	}{
		{"the start's own", startCostScale(), nil, guardScale{50000, false, 6 * s, 4500 * time.Millisecond, false, 5 * s, 15 * s, 12 * s}},
		{"the check's own", checkCostScale(), nil, guardScale{50000, false, 6 * s, 3 * s, true, 0, 0, 0}},
		{"the approval's own", approveCostScale(), nil, guardScale{47500, false, 15 * s, 11250 * time.Millisecond, false, 5 * s, 45 * s, 30 * s}},
		{"lint's own", lintCostScale(), nil, guardScale{100000, false, 12 * s, 6 * s, true, 0, 0, 0}},
		{"the start by hand", startCostScale(), map[string]string{"NW_SCALE_PAGES": "20000"},
			guardScale{20000, true, 15 * s, 11250 * time.Millisecond, false, 5 * s, 35 * s, 30 * s}},
		{"the check by hand", checkCostScale(), map[string]string{"NW_SCALE_PAGES": "20000"},
			guardScale{20000, true, 15 * s, 7500 * time.Millisecond, true, 5 * s, 35 * s, 30 * s}},
		{"the approval by hand", approveCostScale(), map[string]string{"NW_SCALE_PAGES": "100000"},
			guardScale{100000, true, 15 * s, 11250 * time.Millisecond, false, 5 * s, 35 * s, 30 * s}},
		{"lint by hand", lintCostScale(), map[string]string{"NW_SCALE_PAGES": "300000"},
			guardScale{300000, true, 15 * s, 7500 * time.Millisecond, true, 5 * s, 35 * s, 30 * s}},
		{"lint by hand, its request given", lintCostScale(), map[string]string{"NW_SCALE_PAGES": "300000", "NW_SCALE_REQUEST": "60s"},
			guardScale{300000, true, 60 * s, 30 * s, true, 5 * s, 80 * s, 120 * s}},
		{"the start by hand, its request given", startCostScale(), map[string]string{"NW_SCALE_PAGES": "100000", "NW_SCALE_REQUEST": "2m"},
			guardScale{100000, true, 120 * s, 90 * s, false, 5 * s, 140 * s, 240 * s}},
	} {
		got, err := guardScaleOf(env(c.vars), "NW_SCALE_PAGES", c.def)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	for _, vars := range []map[string]string{
		{"NW_SCALE_PAGES": "lots"}, {"NW_SCALE_PAGES": "0"}, {"NW_SCALE_PAGES": "-20000"}, {"NW_SCALE_PAGES": "2e4"},
		{"NW_SCALE_PAGES": "1000"}, {"NW_SCALE_PAGES": "19999"},
		{"NW_SCALE_PAGES": "20000", "NW_SCALE_REQUEST": "60"}, {"NW_SCALE_PAGES": "20000", "NW_SCALE_REQUEST": "0s"},
		{"NW_SCALE_PAGES": "20000", "NW_SCALE_REQUEST": "-1m"}, {"NW_SCALE_REQUEST": "60s"},
	} {
		if got, err := guardScaleOf(env(vars), "NW_SCALE_PAGES", lintCostScale()); err == nil {
			t.Errorf("%v: %+v, want an error", vars, got)
		}
	}
	for _, c := range []struct {
		def  guardScale
		vars map[string]string
		want int
	}{
		{approveCostScale(), nil, 500}, {approveCostScale(), map[string]string{"NW_SCALE_PAGES": "20000"}, 211},
		{approveCostScale(), map[string]string{"NW_SCALE_PAGES": "100000"}, 1053},
	} {
		got, err := guardScaleOf(env(c.vars), "NW_SCALE_PAGES", c.def)
		if folders := approveCostFoldersOf(got, 95); err != nil || folders != c.want {
			t.Errorf("%v: %d folders, %v; want %d", c.vars, folders, err, c.want)
		}
	}
	// The waits the start's and the approval's guards check before they
	// time a request, their own and by hand; one at each edge is an error.
	for _, c := range []struct {
		def  guardScale
		vars map[string]string
	}{
		{startCostScale(), nil}, {approveCostScale(), nil},
		{startCostScale(), map[string]string{"NW_SCALE_PAGES": "20000"}},
		{approveCostScale(), map[string]string{"NW_SCALE_PAGES": "100000", "NW_SCALE_REQUEST": "2m"}},
	} {
		got, err := guardScaleOf(env(c.vars), "NW_SCALE_PAGES", c.def)
		if err != nil || got.waits() != nil {
			t.Errorf("%v: %+v, %v; its waits %v, want none", c.vars, got, err, got.waits())
		}
	}
	for _, edge := range []guardScale{
		{Request: 6 * s, Own: 4500 * time.Millisecond, Read: 5 * s, Write: 15 * s, Client: 4500 * time.Millisecond},
		{Request: 6 * s, Own: 4500 * time.Millisecond, Read: 5 * s, Write: 11 * s, Client: 12 * s},
	} {
		if edge.waits() == nil {
			t.Errorf("%+v: no error, want one: the client at the call, or the write at the read and the request", edge)
		}
	}
	for _, c := range []struct {
		by           bool
		value, bound uint64
		called       bool
	}{{false, 11, 10, true}, {false, 10, 10, false}, {true, 11, 10, false}} {
		called := false
		guardScale{ByHand: c.by}.judge(func(string, ...any) { called = true }, "x", c.value, c.bound)
		if called != c.called {
			t.Errorf("judge by hand %v of %d against %d called errorf %v, want %v", c.by, c.value, c.bound, called, c.called)
		}
	}
}

// The trash benchmarks' levels are their own unless given (M12/P4 design
// 3.3, 7.2's 19): unset, the loads, the attachments and the versions they
// ran at; set, those named, in order; a bad, empty, repeated or, of the
// versions, falling level an error.
func TestLevelsOfTheTrashBenchmarksAreTheirOwnUnlessGiven(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	loads, attachments, err := trashRestoreLevels(env(nil))
	versions, verr := trashHistoryLevels(env(nil))
	if err != nil || verr != nil || !slices.Equal(loads, []string{"light", "full"}) ||
		!slices.Equal(attachments, []int{0, 1000, 2000, 5000, 10000}) || !slices.Equal(versions, []int{250, 500, 1000, 1500, 2000}) {
		t.Errorf("unset: %v %v %v, %v %v; want their own", loads, attachments, versions, err, verr)
	}
	given := env(map[string]string{"NERVE_BENCH_LOADS": "full", "NERVE_BENCH_ATTACHMENTS": "10000,3000",
		"NERVE_BENCH_VERSIONS": "1000"})
	loads, attachments, err = trashRestoreLevels(given)
	versions, verr = trashHistoryLevels(given)
	if err != nil || verr != nil || !slices.Equal(loads, []string{"full"}) || !slices.Equal(attachments, []int{10000, 3000}) ||
		!slices.Equal(versions, []int{1000}) {
		t.Errorf("given: %v %v %v, %v %v; want full, 10000 and 3000, 1000", loads, attachments, versions, err, verr)
	}
	if versions, err := trashHistoryLevels(env(map[string]string{"NERVE_BENCH_VERSIONS": "2,1200,1800"})); err != nil ||
		!slices.Equal(versions, []int{2, 1200, 1800}) {
		t.Errorf("rising versions: %v, %v", versions, err)
	}
	for _, vars := range []map[string]string{
		{"NERVE_BENCH_LOADS": "heavy"}, {"NERVE_BENCH_LOADS": "light,"}, {"NERVE_BENCH_LOADS": "full,full"},
		{"NERVE_BENCH_LOADS": "Full"}, {"NERVE_BENCH_ATTACHMENTS": "-1"}, {"NERVE_BENCH_ATTACHMENTS": "1e3"},
		{"NERVE_BENCH_ATTACHMENTS": "1000,1000"}, {"NERVE_BENCH_ATTACHMENTS": " 1000"},
	} {
		if loads, attachments, err := trashRestoreLevels(env(vars)); err == nil {
			t.Errorf("%v: %v %v, want an error", vars, loads, attachments)
		}
	}
	for _, s := range []string{"1", "0", "1000,500", "500,500", "many", ",250"} {
		if versions, err := trashHistoryLevels(env(map[string]string{"NERVE_BENCH_VERSIONS": s})); err == nil {
			t.Errorf("NERVE_BENCH_VERSIONS=%q: %v, want an error", s, versions)
		}
	}
}
