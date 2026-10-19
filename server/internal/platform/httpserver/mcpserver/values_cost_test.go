//go:build !race

package mcpserver

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// The race detector multiplies what the code allocates: this test runs
// without it, in make test-go-costs.

// A body of Pattern's limit, 37.8 MB of [[],[],…] in _meta, is valid JSON
// of about 12.6 million values. It is refused within a small allocation,
// the same as a body of 1 MiB of them: the check stops at the value past
// MaxBodyValues, before the walk's maps and before any decoder (M9/P2 S1
// review A2-I1); the SDK's generic decode of its _meta cost about 5 GiB.
// The limit of _meta's members, about 2.6 million, is refused within what
// MaxBodyValues of their names cost the scan, which keeps them in a map,
// as 1 MiB of them is: had it read on, it would keep them all. Without
// the bound, bodyshape's walk alone copies the body for each object it
// goes into, the message, params and _meta.
func TestTheValuesBoundStopsTheScanEarly(t *testing.T) {
	const limit = 37_814_272 // bootstrap's mcpBodyLimit
	// body is a call of n bytes whose _meta holds the items next makes,
	// joined by commas, then spaces.
	body := func(n int, open string, next func(i int) string, end string) []byte {
		head := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"write",` +
			`"arguments":{"path":"Big.md","content":"x"},"_meta":{` + open
		tail := end + `}}}`
		b := bytes.NewBuffer(make([]byte, 0, n))
		b.WriteString(head)
		for i := 0; ; i++ {
			item := next(i)
			if i > 0 {
				item = "," + item
			}
			if b.Len()+len(item)+len(tail) > n {
				break
			}
			b.WriteString(item)
		}
		b.Write(bytes.Repeat([]byte(" "), n-b.Len()-len(tail)))
		b.WriteString(tail)
		return b.Bytes()
	}
	arrays := func(n int) []byte { return body(n, `"a":[`, func(int) string { return "[]" }, `]`) }
	members := func(n int) []byte { return body(n, "", func(i int) string { return fmt.Sprintf(`"m%d":1`, i) }, "") }
	check := func(table *bodyshape.Table, b []byte) (uint64, time.Duration, error) {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		err := table.Check(Pattern, b)
		took := time.Since(start)
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc, took, err
	}
	for _, tt := range []struct {
		name  string
		body  []byte
		limit uint64 // what the refusal may allocate, whatever the body's length
	}{
		{"1 MiB of [[],[],…]", arrays(1 << 20), 64 << 10},
		{"the limit of [[],[],…]", arrays(limit), 64 << 10},
		// About 1 MiB: 10,000 names decoded, and a map of them.
		{"1 MiB of members", members(1 << 20), 2 << 20},
		{"the limit of members", members(limit), 2 << 20},
	} {
		got, took, err := check(Bodies(), tt.body)
		var over *bodyshape.TooManyValues
		if !errors.As(err, &over) || over.Most != MaxBodyValues {
			t.Fatalf("%s: Check = %v, want *bodyshape.TooManyValues of MaxBodyValues", tt.name, err)
		}
		if got > tt.limit {
			t.Errorf("%s: Check of %d bytes allocated %d KiB, want at most %d KiB", tt.name, len(tt.body), got>>10, tt.limit>>10)
		}
		t.Logf("%s, %d bytes: refused, allocated %d bytes in %v", tt.name, len(tt.body), got, took.Round(100*time.Microsecond))
	}
	// The control: the same body of the limit with no bound costs at least
	// itself, read on.
	unbound := Bodies()
	unbound.MaxValues = 0
	got, took, err := check(unbound, arrays(limit))
	if err != nil || got < limit {
		t.Errorf("Check of the limit with no bound = %v and %d KiB, want nil and at least the body", err, got>>10)
	}
	t.Logf("with no bound: %v, allocated %d MiB in %v", err, got>>20, took.Round(time.Millisecond))
}
