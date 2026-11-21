package main

import (
	"io"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// ops waits for all the background or the notebook's (M12/P4/S7 design 6.3
// item 5): all by default, as before; notebook handed to the operations
// it runs; any other -quiet refused before anything is read (code review
// C, M1).
func TestOpsQuietIsAllOrTheNotebook(t *testing.T) {
	for _, c := range []struct {
		name  string
		args  []string
		quiet bool
	}{
		{"by default all", nil, false},
		{"all", []string{"-quiet", "all"}, false},
		{"notebook", []string{"-quiet", "notebook"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := opsFlags(newDriver("ops"), append([]string{"-container", "c0ffee"}, c.args...))
			if err != nil || f.quietNotebook != c.quiet {
				t.Fatalf("ops %v: quiet notebook %t, %v; want %t", c.args, f.quietNotebook, err, c.quiet)
			}
			if o := f.ops(nil, client.NewPSQL("c0ffee"), "nb", io.Discard); o.QuietNotebook != c.quiet || o.Notebook != "nb" {
				t.Fatalf("ops %v runs %+v; want the notebook's quiet %t", c.args, o, c.quiet)
			}
		})
	}
	err := opsCommand([]string{"-container", "c0ffee", "-quiet", "search"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `-quiet "search": all or notebook`) {
		t.Fatalf("ops -quiet search: %v", err)
	}
}
