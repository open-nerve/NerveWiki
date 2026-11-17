// Command perf is M12/P4's measuring tools (M12/P4 design 3.4).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/sample"
	"github.com/open-nerve/NerveWiki/server/tools/perf/vault"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// commands are perf's subcommands by name.
func commands() map[string]func(args []string, stdout io.Writer) error {
	return map[string]func(args []string, stdout io.Writer) error{
		"sample":  sampleCommand,
		"gctrace": gctraceCommand,
		"vault":   vaultCommand,
	}
}

// run runs the subcommand args name: 0, 1 when it fails, 2 for a usage.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || commands()[args[0]] == nil {
		names := slices.Sorted(func(yield func(string) bool) {
			for name := range commands() {
				if !yield(name) {
					return
				}
			}
		})
		_, _ = fmt.Fprintf(stderr, "usage: perf <%s> [flags]; perf <command> -h for its flags\n", strings.Join(names, "|"))
		return 2
	}
	if err := commands()[args[0]](args[1:], stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 2
		}
		_, _ = fmt.Fprintf(stderr, "perf %s: %v\n", args[0], err)
		return 1
	}
	return 0
}

// sampleCommand writes a row of the serve's /proc and the container's
// cgroup every -every until SIGINT, then the container's final values
// (design 3.4.2).
func sampleCommand(args []string, _ io.Writer) error {
	fs := flag.NewFlagSet("sample", flag.ContinueOnError)
	pid := fs.Int("pid", 0, "the serve's process id")
	container := fs.String("container", "", "the PostgreSQL container's full id")
	every := fs.Duration("every", 100*time.Millisecond, "how often to sample")
	out := fs.String("out", "", "the CSV to write")
	proc := fs.String("proc", "/proc", "proc's mount")
	cgroups := fs.String("cgroups", "/sys/fs/cgroup/system.slice", "the directory of docker's container scopes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid <= 0 || *container == "" || *out == "" || *every <= 0 {
		return errors.New("-pid, -container, -out and a positive -every are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tick, err := sample.ClockTicks(ctx)
	if err != nil {
		return err
	}
	s, err := sample.Open(*proc, sample.CgroupOf(*cgroups, *container), *pid, tick, time.Now())
	if err != nil {
		return err
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	runErr := s.Run(ctx, f, *every, time.Now)
	if err := f.Close(); err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

// gctraceCommand reads the serve's log's gc trace over a window against the
// round's base (design 3.4.2).
func gctraceCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("gctrace", flag.ContinueOnError)
	log := fs.String("log", "", "the serve's log, its JSON lines and its gc trace")
	var start, base, from, to timeFlag
	fs.Var(&start, "start", "the serve process's start (sample's started_unix_ns): Unix nanoseconds or RFC 3339")
	fs.Var(&base, "base", "the round's base, at or just before its forced collection after the idle")
	fs.Var(&from, "from", "the window's start")
	fs.Var(&to, "to", "the window's end")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *log == "" || start.IsZero() || base.IsZero() || from.IsZero() || to.IsZero() {
		return errors.New("-log, -start, -base, -from and -to are required")
	}
	b, err := os.ReadFile(*log)
	if err != nil {
		return err
	}
	gcs, broken, err := sample.ParseGC(b, start.Time)
	if err != nil {
		return err
	}
	r, err := sample.Read(gcs, sample.Window{Base: base.Time, From: from.Time, To: to.Time})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s broken=%d\n", r, broken)
	if err == nil && (len(r.Missing) > 0 || broken > 0) {
		err = fmt.Errorf("gc lines missing %v, %d broken: measure the combination again", r.Missing, broken)
	}
	return err
}

// timeFlag is a time given as Unix nanoseconds or RFC 3339.
type timeFlag struct{ time.Time }

func (t *timeFlag) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func (t *timeFlag) Set(s string) error {
	if ns, err := strconv.ParseInt(s, 10, 64); err == nil {
		t.Time = time.Unix(0, ns)
		return nil
	}
	v, err := time.Parse(time.RFC3339Nano, s)
	t.Time = v
	return err
}

// vaultCommand writes an import's vault from a seed (design 3.4.3).
func vaultCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("vault", flag.ContinueOnError)
	out := fs.String("out", "", "the zip to write")
	var s vault.Spec
	fs.IntVar(&s.Pages, "pages", 0, "the pages")
	fs.IntVar(&s.Attachments, "attachments", 0, "the attachments")
	fanout := fs.String("fanout", "", "the folders of each level, comma-separated: 10,10,98")
	fs.IntVar(&s.Links, "links", 0, "a page's links")
	body := fs.String("body", "2KiB", "a page's bytes")
	attachment := fs.String("attachment-bytes", "800KiB", "an attachment's bytes")
	fs.IntVar(&s.Random, "random", 12, "1/random of an attachment is random, the rest a block of 4 KiB repeated")
	fs.Uint64Var(&s.Seed, "seed", 0, "the seed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required")
	}
	var err error
	if s.Body, err = bytesOf(*body); err != nil {
		return err
	}
	if s.AttachmentBytes, err = bytesOf(*attachment); err != nil {
		return err
	}
	if *fanout != "" {
		for f := range strings.SplitSeq(*fanout, ",") {
			n, err := strconv.Atoi(f)
			if err != nil {
				return fmt.Errorf("-fanout %q: %w", *fanout, err)
			}
			s.Fanout = append(s.Fanout, n)
		}
	}
	r, err := vault.Write(*out, s, vault.ImportLimits())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, r)
	return err
}

// bytesOf reads a size: a count of bytes, or of KiB, MiB or GiB.
func bytesOf(s string) (int, error) {
	for _, unit := range []struct {
		suffix string
		shift  int
	}{{"KiB", 10}, {"MiB", 20}, {"GiB", 30}} {
		if n, ok := strings.CutSuffix(s, unit.suffix); ok {
			v, err := strconv.Atoi(n)
			return v << unit.shift, err
		}
	}
	return strconv.Atoi(s)
}
