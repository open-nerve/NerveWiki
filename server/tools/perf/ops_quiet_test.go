package main

import (
	"io"
	"strings"
	"testing"
)

// ops waits for all the background or the notebook's (M12/P4/S7 design 6.3
// item 5): any other -quiet is refused before anything is read.
func TestOpsQuietIsAllOrTheNotebook(t *testing.T) {
	err := opsCommand([]string{"-container", "c0ffee", "-quiet", "search"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `-quiet "search": all or notebook`) {
		t.Fatalf("ops -quiet search: %v", err)
	}
}
