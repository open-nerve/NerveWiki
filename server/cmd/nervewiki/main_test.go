package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

func execute(ctx context.Context, environ []string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(ctx, args, environ, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersion(t *testing.T) {
	code, stdout, _ := execute(context.Background(), nil, "version")

	want := regexp.MustCompile(`^nervewiki \S+ commit=\S* commit_time=\S* modified=(true|false)\n$`)
	if code != 0 || !want.MatchString(stdout) {
		t.Errorf("nervewiki version = %d %q, want 0 and a line matching %s", code, stdout, want)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	code, _, stderr := execute(context.Background(), nil, "bogus")

	if code != 1 || !strings.Contains(stderr, `nervewiki: unknown command "bogus"`) {
		t.Errorf("nervewiki bogus = %d %q, want 1 and an unknown command error", code, stderr)
	}
}

func TestUnknownMigrateSubcommandFails(t *testing.T) {
	code, _, stderr := execute(context.Background(), nil, "migrate", "upp")

	if code != 1 || !strings.Contains(stderr, `nervewiki: unknown command "upp" for "nervewiki migrate"`) {
		t.Errorf("nervewiki migrate upp = %d %q, want 1 and an unknown command error", code, stderr)
	}
}

func TestExtraArgumentsFail(t *testing.T) {
	for _, args := range [][]string{{"serve", "now"}, {"migrate", "up", "all"}, {"version", "-"}} {
		code, _, stderr := execute(context.Background(), nil, args...)

		if code != 1 || !strings.HasPrefix(stderr, "nervewiki: ") {
			t.Errorf("nervewiki %s = %d %q, want 1 and an error", strings.Join(args, " "), code, stderr)
		}
	}
}

func TestBareMigratePrintsHelp(t *testing.T) {
	code, stdout, stderr := execute(context.Background(), nil, "migrate")

	if code != 0 || !strings.Contains(stdout, "Available Commands:") || stderr != "" {
		t.Errorf("nervewiki migrate = %d, stdout %q, stderr %q; want 0 and the help", code, stdout, stderr)
	}
}

func TestInvalidConfigurationIsReported(t *testing.T) {
	for _, args := range [][]string{{"serve"}, {"migrate", "status"}} {
		code, _, stderr := execute(context.Background(), []string{"NWIKI_ENV=test"}, args...)

		if code != 1 || stderr != "nervewiki: invalid configuration:\ndatabase.url: is required\n" {
			t.Errorf("nervewiki %s = %d %q, want 1 and the invalid key", strings.Join(args, " "), code, stderr)
		}
	}
}

// The binary embeds every migration: up applies them all in order, and
// status lists each as applied.
func TestMigrateUpThenStatus(t *testing.T) {
	files, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil || len(files) == 0 {
		t.Fatalf("embedded migrations = %d, %v; want some", len(files), err)
	}
	var applied, status strings.Builder
	status.WriteString(`^VERSION +STATE +APPLIED AT +SOURCE\n`)
	for i, f := range files {
		fmt.Fprintf(&applied, "applied %s\n", f.Name())
		fmt.Fprintf(&status, `%d +applied +\S+ +%s\n`, i+1, regexp.QuoteMeta(f.Name()))
	}
	status.WriteString("$")
	environ := []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=" + pgtest.NewEmptyDatabase(t)}

	code, stdout, stderr := execute(context.Background(), environ, "migrate", "up")
	if code != 0 || stdout != applied.String() {
		t.Errorf("nervewiki migrate up = %d %q (stderr %q), want 0 and\n%s", code, stdout, stderr, applied.String())
	}
	code, stdout, stderr = execute(context.Background(), environ, "migrate", "status")
	if code != 0 || !regexp.MustCompile(status.String()).MatchString(stdout) {
		t.Errorf("nervewiki migrate status = %d %q (stderr %q), want 0 and every migration applied", code, stdout, stderr)
	}
}

// nervewiki serve runs until it is cancelled, as by the first SIGINT or
// SIGTERM, and then exits 0.
func TestServeUntilCancelled(t *testing.T) {
	addrFile := filepath.Join(t.TempDir(), "addr")
	environ := []string{
		"NWIKI_ENV=test",
		"NWIKI_DATABASE__URL=" + pgtest.NewEmptyDatabase(t),
		"NWIKI_SERVER__ADDR=127.0.0.1:0",
		"NWIKI_SERVER__ADDR_FILE=" + addrFile,
		"NWIKI_LOG__LEVEL=info",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		code   int
		stderr string
	}
	done := make(chan result, 1)
	go func() {
		code, _, stderr := execute(ctx, environ, "serve")
		done <- result{code, stderr}
	}()

	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(15 * time.Second); !ready && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		addr, err := os.ReadFile(addrFile)
		if err != nil {
			continue
		}
		if resp, err := client.Get("http://" + string(addr) + "/readyz"); err == nil {
			_ = resp.Body.Close()
			ready = resp.StatusCode == http.StatusOK
		}
	}
	cancel()
	r := <-done

	if !ready {
		t.Fatalf("nervewiki serve never became ready; stderr:\n%s", r.stderr)
	}
	if r.code != 0 {
		t.Errorf("nervewiki serve exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"configuration loaded", "config.database.url=xxxxx", "database pool created", `msg="database pool closed"`} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
}
