package bodyshape

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// At most maxProblems problems come back, and the same ones every time: the
// first in the body for the ambiguities, the first by name for the
// structure (the members are written in reverse order here). A problem the
// body repeats is reported once and counted once.
func TestCheckListsAtMostSixteenProblems(t *testing.T) {
	var twice, undeclared []string
	var wantTwice, wantUndeclared []FieldError
	for i := range 20 {
		name := fmt.Sprintf("x%02d", i)
		twice = append(twice, `"`+name+`":1,"`+name+`":2`)
		undeclared = slices.Insert(undeclared, 0, `"`+name+`":1`)
		if i < 16 {
			wantTwice = append(wantTwice, FieldError{name, "duplicate"})
			wantUndeclared = append(wantUndeclared, FieldError{name, "not_allowed"})
		}
	}
	const valid = `"name":"a","nested":{"a":"y"},`
	repeated := strings.Repeat("\"r\":\"\xff\",", 20)
	wantRepeated := append([]FieldError{{"r", "duplicate"}, {"r", "invalid_format"}}, wantTwice[:14]...)
	tests := []struct {
		name string
		body string
		want []FieldError
	}{
		{"twenty names twice", `{` + valid + strings.Join(twice, ",") + `}`, wantTwice},
		{"twenty undeclared names", `{` + valid + strings.Join(undeclared, ",") + `}`, wantUndeclared},
		{"a problem repeated twenty times, then twenty names twice", `{` + valid + repeated + strings.Join(twice, ",") + `}`,
			wantRepeated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shape *Error
			if err := things().Check(pattern, []byte(tt.body)); !errors.As(err, &shape) {
				t.Fatalf("Check = %v, want a *Error", err)
			}
			if !slices.Equal(shape.Fields, tt.want) {
				t.Errorf("Check = %s, want %s", fields(shape.Fields), fields(tt.want))
			}
		})
	}
}

// A body can nest values as deep as json.Valid allows, and have a problem
// every few bytes under a long name. Check keeps the path as text cut after
// maxPath+1 bytes, reports each problem once at a path of at most maxPath
// bytes, and lists at most maxProblems problems, so what it allocates grows
// with the body, and its answer does not. A path string for every value
// costs the square of the depth, and every problem with its path the number
// of problems times the name: the first four bodies make such code allocate
// hundreds of megabytes, and gigabytes at the 1 MiB body limit. With whole
// paths, the next three would be answered with 16 copies of a path of about
// 1 MiB. The last repeats one problem, which is reported once and not
// counted, so each repeat must cost little. Not parallel: TotalAlloc counts
// the whole process. Under the race detector only the answer's size is
// checked: it multiplies what the code allocates.
func TestCheckCostsAboutTheBody(t *testing.T) {
	// An open map of closed objects: a client's key is part of every path under it.
	groups := &Table{
		Nodes: []Node{
			{Types: Object, Extra: 1, Items: Open, Props: map[string]int{}},
			{Types: Object, Extra: Closed, Items: Open, Props: map[string]int{}},
		},
		Roots: map[string]int{pattern: 0},
	}
	long := strings.Repeat("n", 1<<16)
	var members []string
	for i := range 8000 {
		members = append(members, fmt.Sprintf(`"m%04d":1`, i))
	}
	// maxProblems problems, each at a path of its own, under a path of about
	// 1 MiB. The names are of the byte that the problem's encoding writes as
	// six.
	angles := strings.Repeat("<", 1<<20-400)
	var twice, notUTF8 []string
	for i := range maxProblems {
		key := fmt.Sprintf(`"k%02d"`, i)
		twice = append(twice, key+":1,"+key+":1")
		notUTF8 = append(notUTF8, key+":\"\xff\"")
	}
	longTwice := `{"` + angles + `":{` + strings.Join(twice, ",") + `}}`
	longNotUTF8 := `{"` + angles + `":{` + strings.Join(notUTF8, ",") + `}}`
	deepTwice := strings.Repeat(`{"`+angles[:90]+`":`, 9990) + `{` + strings.Join(twice, ",") + `}` + strings.Repeat("}", 9990)
	// One problem 200,000 times: the paths are cut short to the same text, so
	// the problem is reported once, and each repeat is compared, not counted.
	repeated := `{"` + angles[:300] + `":[` + strings.Repeat("\"\xff\",", 200000) + "\"\xff\"]}"
	tests := []struct {
		name  string
		table *Table
		body  string
		limit int // what Check may allocate, in bytes
	}{
		// 64 MiB guards the path written for every value and the uncapped
		// problems: hundreds of megabytes and more on these bodies.
		{"objects nested as deep as JSON allows", things(),
			strings.Repeat(`{"nnnnnnnnnnnnnnnn":`, 10000) + "1" + strings.Repeat("}", 10000), 64 << 20},
		{"arrays nested as deep as JSON allows", things(), strings.Repeat("[", 10000) + strings.Repeat("]", 10000), 64 << 20},
		{"strings that are not UTF-8 under a long name", things(),
			`{"` + long + `":[` + strings.Repeat("\"\xff\",", 7999) + "\"\xff\"]}", 64 << 20},
		{"undeclared members under a long map key", groups, `{"` + long + `":{` + strings.Join(members, ",") + `}}`, 64 << 20},
		// 6 times the body guards the whole path in each problem: maxProblems
		// copies of it are about 16 times the body, while reading the body
		// costs about twice it (the long name is decoded).
		{"names twice under a name of 1 MiB", things(), longTwice, 6 * len(longTwice)},
		{"strings that are not UTF-8 under a name of 1 MiB", things(), longNotUTF8, 6 * len(longNotUTF8)},
		{"names twice under 9,990 names of 90 bytes", things(), deepTwice, 6 * len(deepTwice)},
		// 6 times the body guards the repeats too: a path string for each would
		// be about 50 times the body.
		{"a string that is not UTF-8, 200,000 times under a name of 300 bytes", things(), repeated, 6 * len(repeated)},
	}
	for _, tt := range tests {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		err := tt.table.Check(pattern, []byte(tt.body))
		took := time.Since(start)
		runtime.ReadMemStats(&after)

		var shape *Error
		if !errors.As(err, &shape) {
			t.Fatalf("%s: Check = %v, want a *Error", tt.name, err)
		}
		got := after.TotalAlloc - before.TotalAlloc
		if !raceEnabled && got > uint64(tt.limit) {
			t.Errorf("%s: Check of %d bytes allocated %d KiB, want at most %d KiB", tt.name, len(tt.body), got>>10, tt.limit>>10)
		}
		// The answer's paths are at most maxProblems × maxPath bytes, whatever the body.
		paths := 0
		for _, f := range shape.Fields {
			paths += len(f.Field)
		}
		if paths > maxProblems*maxPath {
			t.Errorf("%s: the paths of the answer are %d bytes, want at most %d", tt.name, paths, maxProblems*maxPath)
		}
		t.Logf("%s: %d bytes, %d problems, paths of %d bytes, allocated %d KiB in %v", tt.name, len(tt.body), len(shape.Fields),
			paths, got>>10, took.Round(100*time.Microsecond))
	}
}
