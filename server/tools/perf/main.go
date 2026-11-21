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

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
	"github.com/open-nerve/NerveWiki/server/tools/perf/drive"
	"github.com/open-nerve/NerveWiki/server/tools/perf/sample"
	"github.com/open-nerve/NerveWiki/server/tools/perf/vault"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// commands are perf's subcommands by name.
func commands() map[string]func(args []string, stdout io.Writer) error {
	return map[string]func(args []string, stdout io.Writer) error{
		"sample":   sampleCommand,
		"gctrace":  gctraceCommand,
		"vault":    vaultCommand,
		"prepare":  prepareCommand,
		"bodies":   bodiesCommand,
		"mcp":      mcpCommand,
		"editors":  editorsCommand,
		"tabs":     tabsCommand,
		"ops":      opsCommand,
		"transfer": transferCommand,
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

// prepareCommand signs in as the account users create made, makes the
// workspace perf, the notebooks named and the tokens, and writes them
// for the drivers (design 3.4.4).
func prepareCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	base := fs.String("base", "", "the serve's address, http://127.0.0.1:<port>")
	email := fs.String("email", "perf@example.com", "the account's email")
	password := fs.String("password", "", "the account's password (or PERF_PASSWORD)")
	notebooks := fs.String("notebooks", "", "the notebooks to make, comma-separated")
	pats := fs.Int("pats", 50, "the personal access tokens to make: one a reader, writer, editor, tab or agent")
	out := fs.String("out", "", "the file to write, <dir>/pats.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *password == "" {
		*password = os.Getenv("PERF_PASSWORD")
	}
	if *base == "" || *password == "" || *out == "" {
		return errors.New("-base, -password (or PERF_PASSWORD) and -out are required")
	}
	var names []string
	if *notebooks != "" {
		names = strings.Split(*notebooks, ",")
	}
	p, err := client.Prepare(context.Background(), *base, *email, *password, names, *pats)
	if err != nil {
		return err
	}
	if err := p.Save(*out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "workspace=%s notebooks=%d pats=%d\n", p.Workspace, len(p.Notebooks), len(p.PATs))
	return err
}

// driver is what each driver's command shares: the prepared file, the
// records' file, and its clients.
type driver struct {
	fs       *flag.FlagSet
	prepared *string
	records  *string
}

func newDriver(name string) driver {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	return driver{fs: fs, prepared: fs.String("prepared", "", "prepare's file, <dir>/pats.json"),
		records: fs.String("records", "", "the requests' records to write, a JSON line each")}
}

// open reads the prepared file and opens the records: the clients wanted,
// the log, and its closing.
func (d driver) open(want int) (client.Prepared, []*client.Client, *client.Log, func() error, error) {
	if *d.prepared == "" || *d.records == "" {
		return client.Prepared{}, nil, nil, nil, errors.New("-prepared and -records are required")
	}
	p, err := client.LoadPrepared(*d.prepared)
	if err != nil {
		return client.Prepared{}, nil, nil, nil, err
	}
	f, err := os.Create(*d.records)
	if err != nil {
		return client.Prepared{}, nil, nil, nil, err
	}
	log := client.NewLog(f)
	cs, err := p.Clients(want, log)
	if err != nil {
		_ = f.Close()
		return client.Prepared{}, nil, nil, nil, err
	}
	closeLog := func() error {
		if err := log.Err(); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
	return p, cs, log, closeLog, nil
}

// notebooks are the ids of names, made with c when not prepared.
func notebooks(ctx context.Context, p client.Prepared, c *client.Client, names ...string) ([]string, error) {
	out := make([]string, len(names))
	for i, name := range names {
		if id := p.Notebooks[name]; id != "" {
			out[i] = id
			continue
		}
		var made struct {
			ID string `json:"id"`
		}
		if _, err := c.JSON(ctx, "setup-notebook", "POST", "/api/v0/workspaces/"+p.Workspace+"/notebooks", map[string]string{"name": name},
			&made, 201); err != nil {
			return nil, err
		}
		out[i] = made.ID
	}
	return out, nil
}

// bodiesCommand is B2's run (design 3.6).
func bodiesCommand(args []string, stdout io.Writer) error {
	d := newDriver("bodies")
	k := d.fs.Int("k", 2, "the credentials: ceil(k/2) readers, floor(k/2) writers")
	kind := d.fs.String("kind", "normal", "normal or pathological")
	warmup := d.fs.Duration("warmup", 10*time.Second, "the warmup, not summarized")
	runFor := d.fs.Duration("for", 60*time.Second, "the measured run")
	same := d.fs.Bool("same-notebook", false, "the writers in one notebook, not one each")
	seed := d.fs.Uint64("seed", 2, "the normal body's seed")
	if err := d.fs.Parse(args); err != nil {
		return err
	}
	p, cs, log, closeLog, err := d.open(*k)
	if err != nil {
		return err
	}
	ctx := context.Background()
	names := []string{"bodies read", "bodies shared"}
	if !*same {
		names = names[:1]
		for w := range *k / 2 {
			names = append(names, fmt.Sprintf("bodies writer %d", w))
		}
	}
	ids, err := notebooks(ctx, p, cs[0], names...)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	b := drive.Bodies{Clients: cs, Kind: *kind, Size: drive.MaxContentBytes, Seed: *seed, Read: ids[0], Writers: ids[1:],
		Warmup: *warmup, For: *runFor, Heartbeat: 20 * time.Second}
	from, err := b.Run(ctx)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	busy, early := drive.BodiesSummary(log.Records(), from)
	if _, err := fmt.Fprintf(stdout, "measured_from_unix_ns=%d busy=%d early_busy=%d\n", from.UnixNano(), busy, early); err != nil {
		return errors.Join(err, closeLog())
	}
	return errors.Join(drive.Summarize(stdout, log.Records(), from), closeLog())
}

// mcpCommand is B3's run (design 3.6).
func mcpCommand(args []string, stdout io.Writer) error {
	d := newDriver("mcp")
	shape := d.fs.String("shape", "1", "1, 2a, 2b, 2c, 3 or 4; with -bound, L too")
	bound := d.fs.Bool("bound", false, "each MCP body at /mcp's bound, 37,814,272 bytes, as M9/P2 measured")
	reps := d.fs.Int("reps", 3, "the reps")
	gap := d.fs.Duration("gap", 20*time.Second, "the time between reps")
	seed := d.fs.Uint64("seed", 3, "the normal body's seed")
	if err := d.fs.Parse(args); err != nil {
		return err
	}
	p, cs, log, closeLog, err := d.open(3)
	if err != nil {
		return err
	}
	ctx := context.Background()
	ids, err := notebooks(ctx, p, cs[0], "mcp")
	if err != nil {
		return errors.Join(err, closeLog())
	}
	start := time.Now()
	m := drive.MCPRun{Clients: [3]*client.Client{cs[0], cs[1], cs[2]}, Notebook: ids[0], Shape: *shape, Seed: *seed,
		Size: drive.MaxContentBytes, Bound: *bound, Reps: *reps, Gap: *gap}
	if err := m.Run(ctx); err != nil {
		return errors.Join(err, closeLog())
	}
	return errors.Join(drive.Summarize(stdout, log.Records(), start), closeLog())
}

// editorsCommand is the editors' load (design 3.4.5, 3.7).
func editorsCommand(args []string, stdout io.Writer) error {
	d := newDriver("editors")
	n := d.fs.Int("n", 5, "the editors; a tree writer besides")
	runFor := d.fs.Duration("for", 120*time.Second, "the run")
	heavy := d.fs.Int("heavy-links", 0, "the first editor's page's links, 10000 at the bound; 0 for its 20")
	container := d.fs.String("container", "", "the PostgreSQL container's id")
	monitor := d.fs.String("monitor", "", "psql's monitor lines to write")
	if err := d.fs.Parse(args); err != nil {
		return err
	}
	if *container == "" || *monitor == "" {
		return errors.New("-container and -monitor are required")
	}
	p, cs, log, closeLog, err := d.open(*n + 1)
	if err != nil {
		return err
	}
	ctx := context.Background()
	ids, err := notebooks(ctx, p, cs[0], "editors")
	if err != nil {
		return errors.Join(err, closeLog())
	}
	mf, err := os.Create(*monitor)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	start := time.Now()
	e := drive.Editors{Clients: cs, Notebook: ids[0], Bytes: 10 << 10, Links: 20, HeavyLinks: *heavy, Seed: 6, For: *runFor,
		Save: 2 * time.Second, Heartbeat: 20 * time.Second, Renew: 60 * time.Second, Tree: time.Second, Monitor: time.Second,
		PSQL: client.NewPSQL(*container), Monitored: mf}
	before, after, err := e.Run(ctx)
	err = errors.Join(err, mf.Close())
	if err == nil {
		perVersion := int64(0)
		if after.Rows > before.Rows {
			perVersion = (after.Bytes - before.Bytes) / (after.Rows - before.Rows)
		}
		_, err = fmt.Fprintf(stdout, "page_revisions before=%d bytes/%d rows after=%d bytes/%d rows per_version_bytes=%d\n", before.Bytes,
			before.Rows, after.Bytes, after.Rows, perVersion)
	}
	if err == nil {
		err = drive.Summarize(stdout, log.Records(), start)
	}
	return errors.Join(err, closeLog())
}

// tabsCommand is the tree's reread (design 3.4.5, 3.7).
func tabsCommand(args []string, stdout io.Writer) error {
	d := newDriver("tabs")
	k := d.fs.Int("k", 1, "the tabs; an agent besides")
	runFor := d.fs.Duration("for", 60*time.Second, "the run")
	notebook := d.fs.String("notebook", "", "the notebook's name in the prepared file")
	if err := d.fs.Parse(args); err != nil {
		return err
	}
	p, cs, log, closeLog, err := d.open(*k + 1)
	if err != nil {
		return err
	}
	id, err := p.Notebook(*notebook)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	start := time.Now()
	tb := drive.Tabs{Clients: cs, Notebook: id, For: *runFor, Every: time.Second, Interval: drive.TreeInterval}
	if err := tb.Run(context.Background()); err != nil {
		return errors.Join(err, closeLog())
	}
	return errors.Join(drive.Summarize(stdout, log.Records(), start), closeLog())
}

// opsCommand is B4's (design 3.6).
func opsCommand(args []string, stdout io.Writer) error {
	d := newDriver("ops")
	f, err := opsFlags(d, args)
	if err != nil {
		return err
	}
	p, cs, log, closeLog, err := d.open(1)
	if err != nil {
		return err
	}
	id, err := p.Notebook(f.notebook)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	start := time.Now()
	if err := f.ops(cs[0], client.NewPSQL(f.container), id, stdout).Run(context.Background()); err != nil {
		return errors.Join(err, closeLog())
	}
	return errors.Join(drive.Summarize(stdout, log.Records(), start), closeLog())
}

// opsOptions are ops's flags but the driver's, read.
type opsOptions struct {
	notebook, root, container string
	reps                      int
	idle                      time.Duration
	quietNotebook             bool // -quiet notebook
}

// opsFlags reads ops's flags in args, on d's: -container is required, and
// -quiet is all, its default, which waits for the whole background before
// each operation, or notebook, for the notebook's alone (M12/P4/S7 design
// 6.3 item 5); another is refused before anything is read.
func opsFlags(d driver, args []string) (opsOptions, error) {
	var f opsOptions
	d.fs.StringVar(&f.notebook, "notebook", "vault", "the notebook's name in the prepared file")
	d.fs.StringVar(&f.root, "root", "vault", "the page at the notebook's root the vault was imported under")
	d.fs.IntVar(&f.reps, "reps", 3, "each operation's reps")
	d.fs.StringVar(&f.container, "container", "", "the PostgreSQL container's id")
	d.fs.DurationVar(&f.idle, "idle", 30*time.Second, "the idle after the background is quiet")
	quiet := d.fs.String("quiet", "all", "the background waited for before each operation: all, or the notebook's alone "+
		"(notebook: its search queue, River's jobs but the search's; under the editors' load)")
	if err := d.fs.Parse(args); err != nil {
		return opsOptions{}, err
	}
	if f.container == "" {
		return opsOptions{}, errors.New("-container is required")
	}
	switch *quiet {
	case "all":
	case "notebook":
		f.quietNotebook = true
	default:
		return opsOptions{}, fmt.Errorf("-quiet %q: all or notebook", *quiet)
	}
	return f, nil
}

// ops is the operations f runs, as c, on the notebook id in psql's
// container.
func (f opsOptions) ops(c *client.Client, psql client.PSQL, id string, stdout io.Writer) drive.Ops {
	return drive.Ops{Client: c, PSQL: psql, Notebook: id, Root: f.root, Reps: f.reps, Idle: f.idle, Poll: 2 * time.Second,
		Out: stdout, QuietNotebook: f.quietNotebook}
}

// transferCommand is B8's (design 3.6).
func transferCommand(args []string, stdout io.Writer) error {
	d := newDriver("transfer")
	notebook := d.fs.String("notebook", "vault", "the notebook's name in the prepared file")
	root := d.fs.String("root", "vault", "the page at the notebook's root to import under and export")
	importZip := d.fs.String("import", "", "the vault's zip to import")
	export := d.fs.String("export", "", "where to download the export's zip")
	container := d.fs.String("container", "", "the PostgreSQL container's id")
	storage := d.fs.String("storage", "", "the serve's storage directory, <dir>/data")
	if err := d.fs.Parse(args); err != nil {
		return err
	}
	if *container == "" || *storage == "" || (*importZip == "") == (*export == "") {
		return errors.New("-container, -storage and one of -import and -export are required")
	}
	p, cs, _, closeLog, err := d.open(1)
	if err != nil {
		return err
	}
	id, err := p.Notebook(*notebook)
	if err != nil {
		return errors.Join(err, closeLog())
	}
	t := drive.Transfer{Client: cs[0], PSQL: client.NewPSQL(*container), Notebook: id, Root: *root, Poll: 2 * time.Second,
		Storage: *storage, Out: stdout}
	if *importZip != "" {
		err = t.Import(context.Background(), *importZip)
	} else {
		err = t.Export(context.Background(), *export)
	}
	return errors.Join(err, closeLog())
}
