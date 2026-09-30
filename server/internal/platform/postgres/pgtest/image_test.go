package pgtest

import (
	"os"
	"strings"
	"testing"
)

// The test container and the development database run the same image with
// the same cluster settings (v0.1 design 7.1): trigram splitting depends on
// the image's C library.
func TestContainerMatchesTheDevelopmentDatabase(t *testing.T) {
	compose, err := os.ReadFile("../../../../../deploy/compose.dev.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"image: " + image + "\n", "POSTGRES_INITDB_ARGS: " + initdbArgs + "\n"} {
		if !strings.Contains(string(compose), want) {
			t.Errorf("deploy/compose.dev.yaml lacks %q", strings.TrimSpace(want))
		}
	}
}
