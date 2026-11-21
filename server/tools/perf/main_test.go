package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// perf vault -part writes the part it names (design 3.12): no -part and
// -part 1/1 print the same vault, -part 1/2 a part of it; a -part that is
// no k of n, or past the folders of the first level, is an error and no
// zip.
func TestVaultCommandWritesItsPart(t *testing.T) {
	dir := t.TempDir()
	run := func(out string, part ...string) (string, error) {
		args := append([]string{"-out", filepath.Join(dir, out), "-pages", "40", "-attachments", "4", "-fanout", "4,2", "-links", "3",
			"-body", "256", "-attachment-bytes", "1KiB", "-random", "4", "-seed", "9"}, part...)
		var b strings.Builder
		err := vaultCommand(args, &b)
		return b.String(), err
	}
	whole, err := run("whole.zip")
	if err != nil {
		t.Fatal(err)
	}
	one, err := run("one.zip", "-part", "1/1")
	if err != nil || one != whole {
		t.Errorf("-part 1/1 printed %q, %v; want the whole vault's %q", one, err, whole)
	}
	half, err := run("half.zip", "-part", "1/2")
	// Pages 1 to 40 by turns in eight deepest folders, f1/f1 to f4/f2: the
	// first four hold 20 pages and the four attachments; two folders of the
	// first level and their four.
	if err != nil || !strings.HasPrefix(half, "entries=24 folders=6 nodes=30 ") {
		t.Errorf("-part 1/2 printed %q, %v; want 24 entries, 6 folders", half, err)
	}
	for _, bad := range []string{"0/2", "3/2", "1/0", "1/5", "1", "a/2"} {
		if out, err := run("bad.zip", "-part", bad); err == nil {
			t.Errorf("-part %s printed %q, want an error", bad, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "bad.zip")); !os.IsNotExist(err) {
			t.Errorf("-part %s: the zip is there: %v", bad, err)
		}
	}
}
