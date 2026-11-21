package pgtest

import (
	"os"
	"strings"
	"testing"
)

// The test container, the development database, the end-to-end stories, the
// image smoke test, the deployment sample and the measuring stack of
// server/tools/perf run the same image with the same cluster settings, and
// deploy/README.md names it (v0.1 design 7.1): trigram splitting depends on
// the image's C library, and the measurements are of the release's database.
func TestContainerMatchesTheOtherDatabases(t *testing.T) {
	tests := []struct {
		file string
		want []string
	}{
		{
			file: "deploy/compose.dev.yaml",
			want: []string{"image: " + image + "\n", "POSTGRES_INITDB_ARGS: " + initdbArgs + "\n"},
		},
		{
			file: "e2e/fixtures/db.ts",
			want: []string{`const image = "` + image + `";`, `const initdbArgs = "` + initdbArgs + `";`},
		},
		{
			file: "deploy/image-smoke.sh",
			want: []string{"  " + image + " >/dev/null\n", `POSTGRES_INITDB_ARGS="` + initdbArgs + `"`},
		},
		{
			file: "deploy/compose.yaml",
			want: []string{"image: " + image + "\n", "POSTGRES_INITDB_ARGS: " + initdbArgs + "\n"},
		},
		{
			file: "deploy/README.md",
			want: []string{"`" + image + "`"},
		},
		{
			file: "server/tools/perf/stack.sh",
			want: []string{"\nimage=" + image + "\n", `POSTGRES_INITDB_ARGS="` + initdbArgs + `"`},
		},
	}
	for _, tt := range tests {
		content, err := os.ReadFile("../../../../../" + tt.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range tt.want {
			if !strings.Contains(string(content), want) {
				t.Errorf("%s lacks %q", tt.file, strings.TrimSpace(want))
			}
		}
	}
}
