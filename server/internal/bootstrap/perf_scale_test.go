package bootstrap

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
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

// checkCostScale is the check's guard's: 50,000 pages, lint's cost
// configuration of a 6 s request and a 3 s worker.
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

// guardScaleOf is def, a guard's own, unless sizeVar (NW_SCALE_PAGES)
// gives a size by hand: then production's request deadline, or
// NW_SCALE_REQUEST's, the call's own deadline by production's formula
// (bootstrap/wire.go: lint's worker min(10 s, request/2), the start's and
// the approval's request*3/4; the worker uncapped under NW_SCALE_REQUEST,
// which is no production's formula, so that a run sees a time past 10 s),
// the test configuration's read timeout, a write timeout past the read and
// the request (the configuration's check: read + request < write) and a
// client that outwaits the request twice (00-M12 decision 359). A bad
// value, or NW_SCALE_REQUEST without a size, is an error.
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

// checkWaits fails t unless the timed request's client outwaits the
// guarded call and the server's write timeout outlasts its read and its
// request: else the client or the server ends the request before the
// call's deadline does, and judges in its place (00-M12 decision 359).
func (s guardScale) checkWaits(t testing.TB) {
	t.Helper()
	if s.Client <= s.Own || s.Write <= s.Read+s.Request {
		t.Fatalf("the client waits %v for a call of %v; the write timeout is %v for a read of %v and a request of %v: "+
			"want the client past the call, the write past both", s.Client, s.Own, s.Write, s.Read, s.Request)
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

// The guards' scales are their own unless a size is given (M12/P4 design
// 3.3, 7.2's 9 to 18): unset, each guard's literal numbers, by no other
// constant; NW_SCALE_PAGES set, production's request, the call's deadline
// by its formula, a client of twice the request and a write timeout past
// the read and the request, the approval's folders round(n/95), and only
// logged; NW_SCALE_REQUEST its request, lint's worker half of it past
// 10 s; a bad value an error; judge errs past the bound unless by hand.
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
