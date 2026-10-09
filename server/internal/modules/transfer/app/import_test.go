package app_test

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// vault is an Obsidian vault in its folder: its settings, meta.json
// ordering B before a, a page with children and an attachment, a page
// that is only a folder, a name to mend, macOS's litter; and a file meta
// says the server added, which the import does not read: its content is
// none a page holds.
func vault(t *testing.T) []byte {
	meta := `{"format": 1, "nodes": [{"path": "B.md", "sort_order": 1}, {"path": "a.md", "sort_order": 2}], "contributed": ["gen.md"]}`
	return zipOf(t, append([]zipEntry{{name: "V/"}, {name: "V/.obsidian/app.json", data: "{}"}, {name: "V/.nerve/meta.json", data: meta},
		{name: "__MACOSX/V/._a.md", data: "x"}, {name: "V/B/"}},
		files("V/a.md", "# A\n[[B]] [[c]]", "V/B.md", "b", "V/B/c.md", "c", "V/B/x.png", "PNG", "V/D/e.md", "e", "V/n:m.md", "", "V/gen.md", "g\x00")...)...)
}

// An import creates the vault's nodes where it goes: each page with its
// content, the attachments with their files' rows, in the order meta.json
// gives, then by name; the names mended reported renamed. It succeeds
// with its counts, as its starter acting through the job, in one unit of
// the import's kind; the statistics are refreshed, the parse budget given
// back, the archive deleted.
func TestAnImportWritesTheVault(t *testing.T) {
	w := newImportWorld()
	j := w.queuedImport(nil, vault(t))

	got := w.run(t, j)

	if got.State != domain.StateSucceeded || got.Progress != (domain.Progress{Done: 7, Total: 7}) || got.ResultBytes != nil {
		t.Errorf("job = %+v, want succeeded with 7 of 7 nodes", got)
	}
	if r := got.Report; r == nil || r.Failure != "" || r.Counts != (domain.Counts{Pages: 6, Attachments: 1, Renamed: 1}) ||
		!slices.Equal(problems(r), []string{"renamed: n:m.md -> n_m"}) {
		t.Errorf("report = %+v", got.Report)
	}
	want := []string{"-/B", "-/a", "-/D", "-/n_m", "B/c", "B/x.png", "D/e"}
	if lines := w.tree.lines(); !slices.Equal(lines, want) {
		t.Errorf("made %q, want %q", lines, want)
	}
	if a := w.tree.named(t, "a"); a.content != "# A\n[[B]] [[c]]" || a.parent != nil {
		t.Errorf("a = %+v", a)
	}
	if d := w.tree.named(t, "D"); d.content != "" {
		t.Errorf("D = %+v, want a page without content", d)
	}
	png := w.tree.named(t, "x.png")
	if o, ok := w.files.attached[png.file.ID]; !ok || o.NodeID != png.id || o.NotebookID != w.eng || o.CreatedBy != w.bob ||
		string(w.files.files[png.file.ID]) != "PNG" {
		t.Errorf("x.png's file %+v attached %+v, %v", png.file, o, ok)
	}
	if len(w.tree.units) != 1 || w.tree.units[0] != (app.ImportSpec{NotebookID: w.eng, Action: domain.ActionImport, Client: domain.ClientAPI}) {
		t.Errorf("units %+v, want one of the job's", w.tree.units)
	}
	if job := (shared.Actor{UserID: w.bob, JobID: j.ID}); !slices.Equal(w.auth.asked, []shared.Actor{job}) ||
		!slices.Equal(w.tree.actors, []shared.Actor{job}) {
		t.Errorf("asked %+v, units of %+v; want bob through the job", w.auth.asked, w.tree.actors)
	}
	if w.stats.count() != 1 || w.tree.parses != 6 || w.tree.released != 6 || w.archives.imported(j.ID) {
		t.Errorf("statistics %d, parses %d, released %d, archive kept %v", w.stats.count(), w.tree.parses, w.tree.released,
			w.archives.imported(j.ID))
	}
}

// A meta.json of domain.MaxMeta bytes is read; one larger is not, the
// order then by name only.
func TestAnImportReadsAMetaOfAtMostMaxMeta(t *testing.T) {
	for _, tt := range []struct {
		name string
		size int
		want []string
	}{
		{"of the most bytes", domain.MaxMeta, []string{"-/b", "-/a"}},
		{"larger", domain.MaxMeta + 1, []string{"-/a", "-/b"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			meta := `{"format": 1, "nodes": [{"path": "b.md", "sort_order": 1}, {"path": "a.md", "sort_order": 2}]}`
			meta += strings.Repeat(" ", tt.size-len(meta))
			w := newImportWorld()
			j := w.queuedImport(nil, zipOf(t, zipEntry{name: ".nerve/meta.json", data: meta}, zipEntry{name: "a.md", data: "a"},
				zipEntry{name: "b.md", data: "b"}))
			if got := w.run(t, j); got.State != domain.StateSucceeded {
				t.Fatalf("job %s", got.State)
			}
			if lines := w.tree.lines(); !slices.Equal(lines, tt.want) {
				t.Errorf("made %q, want %q", lines, tt.want)
			}
		})
	}
}

// An import goes under a page: its nodes are the page's children after
// those it has, a name a child holds numbered and reported renamed, its
// path from the page, under its parents as they were named.
func TestAnImportGoesUnderAPage(t *testing.T) {
	w := newImportWorld()
	w.tree.children[w.spec] = []string{"Folder", "x.png"}
	j := w.queuedImport(&w.spec, zipOf(t, files("Folder.md", "f", "Folder/x.png", "1", "x.png", "2", "Folder/a:b.md", "n")...))

	got := w.run(t, j)

	if got.State != domain.StateSucceeded || got.Report.Counts != (domain.Counts{Pages: 2, Attachments: 2, Renamed: 3}) ||
		!slices.Equal(problems(got.Report), []string{"renamed: Folder.md -> Folder 2", "renamed: x.png -> x 2.png",
			"renamed: Folder/a:b.md -> Folder 2/a_b"}) {
		t.Errorf("job %+v, report %+v", got, got.Report)
	}
	if f := w.tree.named(t, "Folder 2"); f.parent == nil || *f.parent != w.spec {
		t.Errorf("Folder 2's parent %v, want the page", f.parent)
	}
	if x := w.tree.named(t, "x.png"); x.parent == nil || *x.parent != w.tree.named(t, "Folder 2").id {
		t.Errorf("x.png's parent %v, want Folder 2", x.parent)
	}
}

// A name numbered because a sibling holds it takes none a later sibling of
// the archive holds as its own, which keeps it: the vault's links to it
// still reach it.
func TestAnImportNumbersNoNameALaterSiblingHolds(t *testing.T) {
	w := newImportWorld()
	w.tree.children[uuid.UUID{}] = []string{"Untitled"}
	got := w.run(t, w.queuedImport(nil, zipOf(t, files("Untitled.md", "u", "Untitled 1.md", "u1", "Untitled 2.md", "u2",
		"a.md", "a", "A.md", "A", "a 2.md", "a2")...)))

	if got.State != domain.StateSucceeded || !sameProblems(problems(got.Report), []string{"renamed: Untitled.md -> Untitled 3",
		"renamed: a.md -> a 3"}) {
		t.Errorf("job %s, report %+v", got.State, got.Report)
	}
	for name, content := range map[string]string{"Untitled 3": "u", "Untitled 1": "u1", "Untitled 2": "u2", "A": "A", "a 3": "a", "a 2": "a2"} {
		if m := w.tree.named(t, name); m.content != content {
			t.Errorf("%s holds %q, want %q", name, m.content, content)
		}
	}
}

// The entries that cannot be imported are skipped, each reported: the rest
// is imported.
func TestAnImportSkipsTheEntriesItCannotRead(t *testing.T) {
	w := newImportWorld()
	big := noise(5<<20 + 1)
	bomb := strings.Repeat("0", 3<<20)
	entries := append(files("ok.md", "ok", "nul.md", "a\x00b", "bad.md", "\xff\xfe", "big.md", big, "big.bin", strings.Repeat("b", 1<<20+1),
		"bomb.md", bomb, "dup.md", "1", "dup.md", "2", "../up.md", "x", "\xffname.md", "x"),
		zipEntry{name: "link.md", data: "ok.md", mode: fs.ModeSymlink | 0o777},
		zipEntry{name: "secret.md", data: "x", flags: 0x1, raw: true, crc: crc32.ChecksumIEEE([]byte("x"))},
		zipEntry{name: "bz.md", data: "x", method: 12, raw: true, crc: crc32.ChecksumIEEE([]byte("x"))},
		zipEntry{name: "broken.md", data: "x", raw: true, crc: 1})
	j := w.queuedImport(nil, zipOf(t, entries...))

	got := w.run(t, j)

	want := []string{"duplicate: dup.md", "unsafe_path: ../up.md", "name_not_utf8: \ufffdname.md", "special_file: link.md", "encrypted: secret.md",
		"unsupported_method: bz.md", "invalid_content: nul.md", "invalid_content: bad.md", "too_large: big.md", "too_large: big.bin",
		"too_compressed: bomb.md", "unreadable: broken.md"}
	if got.State != domain.StateSucceeded || got.Report.Counts != (domain.Counts{Pages: 2, Skipped: 12}) || !sameProblems(problems(got.Report), want) {
		t.Errorf("job %s, report %+v; want\n%q", got.State, got.Report, want)
	}
	if lines := w.tree.lines(); !slices.Equal(lines, []string{"-/dup", "-/ok"}) {
		t.Errorf("made %q", lines)
	}
}

// sameProblems reports whether a and b hold the same problems, in any
// order.
func sameProblems(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// An archive that cannot be imported as a whole fails the import, nothing
// written, its archive deleted.
func TestAnImportFailsAsAWhole(t *testing.T) {
	for _, tt := range []struct {
		name    string
		archive func(t *testing.T) []byte
		setup   func(w *importWorld)
		want    domain.Failure
	}{
		{"not a zip", func(*testing.T) []byte { return []byte("not a zip") }, nil, domain.FailureNotZip},
		{"its directory refused", func(t *testing.T) []byte { return vault(t) },
			func(w *importWorld) { w.archives.openErr = app.ErrNotZip }, domain.FailureNotZip},
		{"too many entries", func(t *testing.T) []byte { return vault(t) }, func(w *importWorld) { w.max.MaxEntries = 3 }, domain.FailureTooManyEntries},
		{"unpacked too large", func(t *testing.T) []byte { return zipOf(t, files("a.md", strings.Repeat("a", 600), "b.md", "b")...) },
			func(w *importWorld) { w.max.MaxUnpacked = 600 }, domain.FailureUnpackedTooLarge},
		{"its archive missing", func(t *testing.T) []byte { return vault(t) },
			func(w *importWorld) { w.archives.openErr = app.ErrFileMissing }, domain.FailureInternal},
		{"its place gone", func(t *testing.T) []byte { return vault(t) }, func(w *importWorld) { w.nodes.all = w.nodes.all[1:] },
			domain.FailureRootNotFound},
		{"its starter no writer", func(t *testing.T) []byte { return vault(t) },
			func(w *importWorld) { w.auth.roles[w.eng][w.bob] = shared.NotebookReader }, domain.FailureForbidden},
		{"its starter no reader", func(t *testing.T) []byte { return vault(t) },
			func(w *importWorld) { delete(w.auth.roles[w.eng], w.bob) }, domain.FailureForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newImportWorld()
			w.auth.rules = map[shared.Action][]shared.NotebookRole{domain.ActionImport: {shared.NotebookEditor, shared.NotebookAdmin}}
			if tt.setup != nil {
				tt.setup(w)
			}
			j := w.queuedImport(&w.spec, tt.archive(t))
			got := w.run(t, j)
			if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != tt.want || len(w.tree.made) != 0 || w.archives.imported(j.ID) {
				t.Errorf("job %s, report %+v, made %d, archive kept %v; want failed %s, nothing made, the archive deleted", got.State, got.Report,
					len(w.tree.made), w.archives.imported(j.ID), tt.want)
			}
		})
	}
}

// A page deeper than pages go from where the import goes is skipped, with
// everything under it; so is one its unit finds too deep, the place moved
// deeper since, its attachments' files deleted.
func TestAnImportSkipsWhatIsTooDeep(t *testing.T) {
	w := newImportWorld()
	parent := &w.deep
	for i := 0; i < 6; i++ {
		id := uuid.NewV7()
		w.nodes.all = append(w.nodes.all, domain.Node{ID: id, ParentID: parent, Name: fmt.Sprint(i)})
		w.tree.pages[id] = 4 + i
		parent = &id
	}
	archive := zipOf(t, files("a.md", "a", "a/b.md", "b", "a/b/c.md", "c", "a/b/x.png", "x", "a/y.png", "y")...)

	got := w.run(t, w.queuedImport(parent, archive))

	if got.State != domain.StateSucceeded || got.Report.Counts != (domain.Counts{Pages: 1, Attachments: 1, Skipped: 3}) ||
		!sameProblems(problems(got.Report), []string{"too_deep: a/b.md", "too_deep: a/b/c.md", "too_deep: a/b/x.png"}) {
		t.Errorf("job %s, report %+v", got.State, got.Report)
	}
	if lines := w.tree.lines(); !slices.Equal(lines, []string{"-/a", "a/y.png"}) {
		t.Errorf("made %q", lines)
	}
	// The plan leaves them out: never parsed, their files never put, not
	// counted.
	if got.Progress != (domain.Progress{Done: 2, Total: 2}) || w.tree.parses != 1 || len(w.files.dropped) != 0 {
		t.Errorf("progress %+v, parses %d, files dropped %v; want the two nodes, a parsed, no file put", got.Progress, w.tree.parses,
			w.files.dropped)
	}

	w = newImportWorld()
	w.tree.deeper = 9
	got = w.run(t, w.queuedImport(&w.spec, archive))
	if got.State != domain.StateSucceeded || got.Report.Counts != (domain.Counts{Skipped: 5}) || len(w.tree.made) != 0 ||
		got.Progress != (domain.Progress{Done: 5, Total: 5}) {
		t.Errorf("job %+v, report %+v, made %q; want everything skipped", got, got.Report, w.tree.lines())
	}
	if w.files.kept() != 0 || len(w.files.dropped) != 2 {
		t.Errorf("files kept %d, dropped %v; want the two attachments' dropped", w.files.kept(), w.files.dropped)
	}
}

// many is an archive of n pages, each of content.
func many(t *testing.T, n int, content func(i int) string) []byte {
	var entries []zipEntry
	for i := range n {
		entries = append(entries, zipEntry{name: fmt.Sprintf("p%05d.md", i), data: content(i)})
	}
	return zipOf(t, entries...)
}

// The nodes are written in batches, each its unit: at most 100 nodes, 8
// MiB of contents, 20,000 links, a page past them alone; the later units
// merge into the first one's changeset.
func TestAnImportWritesInBatches(t *testing.T) {
	for _, tt := range []struct {
		name    string
		archive func(t *testing.T) []byte
		want    []int
	}{
		{"nodes", func(t *testing.T) []byte { return many(t, 250, func(int) string { return "x" }) }, []int{100, 100, 50}},
		{"bytes", func(t *testing.T) []byte { return many(t, 5, func(int) string { return noise(3 << 20) }) }, []int{2, 2, 1}},
		{"links", func(t *testing.T) []byte {
			return many(t, 5, func(int) string { return strings.Repeat("[[x]]", 9000) })
		}, []int{2, 2, 1}},
		{"a page past them", func(t *testing.T) []byte {
			return many(t, 2, func(int) string { return strings.Repeat("[[x]]", 30000) })
		},
			[]int{1, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newImportWorld()
			w.max.MaxUnpacked = 1 << 30
			got := w.run(t, w.queuedImport(nil, tt.archive(t)))
			if got.State != domain.StateSucceeded {
				t.Fatalf("job %+v, report %+v", got, got.Report)
			}
			changesets := map[uuid.UUID]bool{}
			for _, m := range w.tree.made {
				changesets[m.changeset] = true
			}
			for i, u := range w.tree.units {
				if first := w.tree.made[0].changeset; i > 0 && u.Changeset != first {
					t.Errorf("unit %d merges into %v, want the first's %v", i, u.Changeset, first)
				}
			}
			if len(changesets) != 1 || !slices.Equal(w.tree.committed, tt.want) {
				t.Errorf("%d changesets, units of %v; want one, of %v", len(changesets), w.tree.committed, tt.want)
			}
			if w.tree.parses != w.tree.released {
				t.Errorf("parsed %d, released %d", w.tree.parses, w.tree.released)
			}
		})
	}
}

// stopping is an archive of 249 pages, then an attachment.
func stopping(t *testing.T) []byte {
	entries := []zipEntry{{name: "z.png", data: "z"}}
	for i := range 249 {
		entries = append(entries, zipEntry{name: fmt.Sprintf("p%03d.md", i), data: "x"})
	}
	return zipOf(t, entries...)
}

// stopAt makes the import's unit n stop it with stop, waiting there until
// a heartbeat reads it, the unit then ending: the next batch's attachment
// and parse wait for the run to stop, which its own checks then see.
func stopAt(w *importWorld, j domain.Job, n int, stop func(), stopped func(r *row) bool) {
	read := make(chan struct{})
	w.rows.onBeat = func() {
		w.rows.mu.Lock()
		seen := stopped(w.rows.jobs[j.ID])
		w.rows.mu.Unlock()
		if seen {
			select {
			case <-read:
			default:
				close(read)
			}
		}
	}
	w.tree.onUnit = func(at int) {
		if at == n {
			stop()
			<-read
			w.files.mu.Lock()
			w.files.wait = true
			w.files.mu.Unlock()
			w.tree.mu.Lock()
			w.tree.waitParse = true
			w.tree.mu.Unlock()
		}
	}
}

// A cancel asked as a unit runs lets it end, then stops the import: the
// units written kept, counted, the job cancelled, its archive deleted.
func TestACancelledImportStopsBetweenTwoUnits(t *testing.T) {
	w := newImportWorld()
	j := w.queuedImport(nil, stopping(t))
	stopAt(w, j, 2, func() {
		if _, err := w.rows.RequestCancel(context.Background(), j.ID, now()); err != nil {
			t.Error(err)
		}
	}, func(r *row) bool { return r.job.CancelRequested != nil })

	got := w.run(t, j)

	if got.State != domain.StateCancelled || got.Report.Counts.Pages != 200 || got.Progress != (domain.Progress{Done: 200, Total: 250}) ||
		len(w.tree.made) != 200 || w.archives.imported(j.ID) || len(w.tree.units) != 2 {
		t.Errorf("job %+v, report %+v, made %d, units %d; want cancelled after two units", got, got.Report, len(w.tree.made), len(w.tree.units))
	}
}

// A cancel stops an import between two units though the next would read
// nothing of the archive: its pages are folders.
func TestACancelledImportOfFoldersStops(t *testing.T) {
	w := newImportWorld()
	var entries []zipEntry
	for i := range 250 {
		entries = append(entries, zipEntry{name: fmt.Sprintf("f%03d/", i)})
	}
	j := w.queuedImport(nil, zipOf(t, entries...))
	stopAt(w, j, 2, func() {
		if _, err := w.rows.RequestCancel(context.Background(), j.ID, now()); err != nil {
			t.Error(err)
		}
	}, func(r *row) bool { return r.job.CancelRequested != nil })

	got := w.run(t, j)

	if got.State != domain.StateCancelled || len(w.tree.made) != 200 || len(w.tree.units) != 2 {
		t.Errorf("job %s, made %d, units %d; want cancelled after two units", got.State, len(w.tree.made), len(w.tree.units))
	}
}

// The heartbeat writes the report as the import goes, each time it
// changed: the plan's skipped entries before the first unit, then the
// counts and problems of the units written, what the rescue keeps of an
// import interrupted, ten beats apart at least. A write that failed is
// tried again; one that succeeded is not, the beats after it writing none
// until the next.
func TestAnImportsHeartbeatWritesItsReport(t *testing.T) {
	w := newImportWorld()
	entries := []zipEntry{{name: "../out.md", data: "out"}}
	for i := range 150 {
		entries = append(entries, zipEntry{name: fmt.Sprintf("p%03d.md", i), data: "x"})
	}
	j := w.queuedImport(nil, zipOf(t, entries...))
	// Each unit waits for the report before it to be written; the second,
	// for two beats after it too.
	before := map[int]chan struct{}{1: make(chan struct{}), 2: make(chan struct{})}
	written := func(at int) {
		select {
		case <-before[at]:
		default:
			close(before[at])
		}
	}
	after := -1
	w.rows.onBeat = func() {
		w.rows.mu.Lock()
		defer w.rows.mu.Unlock()
		n := len(w.rows.reported)
		if n == 0 {
			return
		}
		switch last := w.rows.reported[n-1]; {
		case last.Counts == domain.Counts{Skipped: 1} && len(last.Problems) == 1 && last.Problems[0].Code == domain.ProblemUnsafePath:
			written(1)
		case last.Counts.Pages == 100:
			if after++; after == 2 {
				written(2)
			}
		}
	}
	w.tree.onUnit = func(at int) {
		select {
		case <-before[at]:
		case <-time.After(5 * time.Second):
			t.Errorf("the report before unit %d not written", at)
		}
		if at == 1 {
			// The first unit's report fails to be written once.
			w.rows.mu.Lock()
			w.rows.failReports = 1
			w.rows.mu.Unlock()
		}
	}

	if got := w.run(t, j); got.State != domain.StateSucceeded {
		t.Fatalf("job %s", got.State)
	}
	w.rows.mu.Lock()
	defer w.rows.mu.Unlock()
	for i, r := range w.rows.reported {
		if r.Failure != "" || i > 0 && reflect.DeepEqual(r, w.rows.reported[i-1]) {
			t.Errorf("report %d written %+v, after %+v; want each a change, without a failure", i, r, w.rows.reported[max(i-1, 0)])
		}
		if at := w.rows.reportedAt; i > 0 && at[i]-at[i-1] < 10 {
			t.Errorf("reports written at beats %v, want ten apart", at)
		}
	}
}

// expiring is a context the test ends as its deadline would.
type expiring struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newExpiring() *expiring {
	return &expiring{Context: context.Background(), done: make(chan struct{})}
}

func (e *expiring) Done() <-chan struct{} { return e.done }

func (e *expiring) Err() error {
	select {
	case <-e.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (e *expiring) expire() { e.once.Do(func() { close(e.done) }) }

// River's context ends an import between units or in one: its timeout
// fails it so, the server's stop as interrupted; the units committed kept
// and counted, the archive deleted, the statistics not refreshed.
func TestRiversContextEndsAnImport(t *testing.T) {
	for _, tt := range []struct {
		name string
		want domain.Failure
	}{{"timeout", domain.FailureTimeout}, {"stop", domain.FailureInterrupted}} {
		t.Run(tt.name, func(t *testing.T) {
			w := newImportWorld()
			j := w.queuedImport(nil, stopping(t))
			// The context ends as the second unit begins: its deadline
			// passed, or the server stopping.
			deadline := newExpiring()
			ctx, cancel := context.WithCancel(deadline)
			defer cancel()
			end := cancel
			if tt.want == domain.FailureTimeout {
				end = deadline.expire
			}
			w.tree.onUnit = func(at int) {
				if at == 2 {
					end()
					<-ctx.Done()
				}
			}
			if err := w.importer().Run(ctx, j.ID); err != nil {
				t.Fatal(err)
			}
			got := w.rows.get(j.ID)
			if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != tt.want || got.Report.Counts.Pages != 100 ||
				len(w.tree.made) != 100 || w.archives.imported(j.ID) || w.stats.count() != 0 {
				t.Errorf("job %s, report %+v, made %d, statistics %d; want failed %s after a unit", got.State, got.Report, len(w.tree.made),
					w.stats.count(), tt.want)
			}
		})
	}
}

// A job deleted as it runs, with its notebook, stops between two units,
// writing no end.
func TestADeletedImportStops(t *testing.T) {
	w := newImportWorld()
	j := w.queuedImport(nil, stopping(t))
	stopAt(w, j, 1, func() { w.rows.set(j.ID, func(r *row) { r.deleted = true }) }, func(r *row) bool { return r.deleted })
	got := w.run(t, j)
	if got.State != domain.StateRunning || len(w.tree.made) != 100 || w.archives.imported(j.ID) {
		t.Errorf("job %s, made %d; want it left running, one unit written", got.State, len(w.tree.made))
	}
}

// A job no longer queued, cancelled or deleted before it ran, is left, its
// archive deleted.
func TestAnImportNoLongerQueuedDeletesItsArchive(t *testing.T) {
	w := newImportWorld()
	j := w.queuedImport(nil, vault(t))
	w.rows.set(j.ID, func(r *row) { r.job.State = domain.StateCancelled })
	if got := w.run(t, j); got.State != domain.StateCancelled || len(w.tree.units) != 0 || w.archives.imported(j.ID) {
		t.Errorf("job %s, units %d, archive kept %v; want it left, its archive deleted", got.State, len(w.tree.units), w.archives.imported(j.ID))
	}
}

// A parent gone as the import runs fails it after the units written:
// root_not_found for where it goes, tree_changed for a page it created,
// the files of the unit refused deleted.
func TestAnImportWhoseParentIsGoneFails(t *testing.T) {
	under := func(t *testing.T) []byte {
		entries := []zipEntry{{name: "a.md", data: "a"}}
		for i := range 120 {
			entries = append(entries, zipEntry{name: fmt.Sprintf("a/p%03d.md", i), data: "x"}, zipEntry{name: fmt.Sprintf("a/x%03d.png", i), data: "x"})
		}
		return zipOf(t, entries...)
	}
	for _, tt := range []struct {
		name    string
		archive func(t *testing.T) []byte
		gone    func(t *testing.T, w *importWorld) uuid.UUID
		want    domain.Failure
	}{
		{"its place", func(t *testing.T) []byte { return many(t, 150, func(int) string { return "x" }) },
			func(_ *testing.T, w *importWorld) uuid.UUID { return w.spec }, domain.FailureRootNotFound},
		{"a page it created", under, func(t *testing.T, w *importWorld) uuid.UUID { return w.tree.named(t, "a").id }, domain.FailureTreeChanged},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newImportWorld()
			w.tree.onUnit = func(n int) {
				if n == 2 {
					id := tt.gone(t, w)
					w.tree.mu.Lock()
					defer w.tree.mu.Unlock()
					w.tree.gone[id] = true
				}
			}
			got := w.run(t, w.queuedImport(&w.spec, tt.archive(t)))
			if got.State != domain.StateFailed || got.Report.Failure != tt.want || len(w.tree.made) != 100 {
				t.Errorf("job %s, report %+v, made %d; want failed %s after the first unit", got.State, got.Report, len(w.tree.made), tt.want)
			}
			if kept := w.files.kept(); kept != len(w.files.attached) {
				t.Errorf("%d files kept, %d attached; want the second unit's dropped", kept, len(w.files.attached))
			}
		})
	}
}

// A parse or a unit the server is too busy for is tried again; a parse
// refused as a batch is held writes the batch first.
func TestAnImportWaitsForABusyServer(t *testing.T) {
	w := newImportWorld()
	w.tree.busyParses, w.tree.busyUnits = 3, 2
	got := w.run(t, w.queuedImport(nil, vault(t)))
	if got.State != domain.StateSucceeded || len(w.tree.made) != 7 || !slices.Equal(w.tree.committed, []int{7}) || len(w.files.dropped) != 0 {
		t.Errorf("job %s, report %+v, units %v, dropped %v; want one unit, its file kept", got.State, got.Report, w.tree.committed, w.files.dropped)
	}

	w = newImportWorld()
	w.tree.busyCalls = map[int]bool{2: true}
	got = w.run(t, w.queuedImport(nil, many(t, 3, func(int) string { return "x" })))
	if got.State != domain.StateSucceeded || !slices.Equal(w.tree.committed, []int{1, 2}) || w.tree.parses != w.tree.released {
		t.Errorf("job %s, units %v; want the first page's written as the second's parse waits", got.State, w.tree.committed)
	}
}

// A store out of room fails the import storage_full, the batch's files
// put before deleted; a unit refused fails it so, its files deleted; one
// whose commit's answer was lost fails it internal, its files left to the
// sweep.
func TestAnImportsUnitFails(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   func(w *importWorld)
		want    domain.Failure
		dropped bool
	}{
		{"store full", func(w *importWorld) { w.files.full = true }, domain.FailureStorageFull, true},
		{"store full after a file", func(w *importWorld) { w.files.full, w.files.room = true, 1 }, domain.FailureStorageFull, true},
		{"refused", func(w *importWorld) { w.tree.refuse = shared.Forbidden() }, domain.FailureForbidden, true},
		{"notebook not visible", func(w *importWorld) { w.tree.refuse = domain.ErrNotebookNotFound }, domain.FailureForbidden, true},
		{"invalid", func(w *importWorld) { w.tree.refuse = &shared.Error{Kind: shared.KindInvalid} }, domain.FailureInternal, true},
		{"uncertain", func(w *importWorld) { w.tree.uncertain = errors.New("the commit's answer was lost") }, domain.FailureInternal, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newImportWorld()
			tt.setup(w)
			// Two attachments, in one batch.
			archive := zipOf(t, append(files("V/a.png", "A", "V/b.png", "B"), zipEntry{name: "V/.obsidian/app.json", data: "{}"})...)
			got := w.run(t, w.queuedImport(nil, archive))
			if got.State != domain.StateFailed || got.Report.Failure != tt.want || len(w.tree.made) != 0 {
				t.Errorf("job %s, report %+v; want failed %s", got.State, got.Report, tt.want)
			}
			if kept := w.files.kept(); (kept == 0) != tt.dropped {
				t.Errorf("%d files kept, dropped %v; want them dropped: %v", kept, w.files.dropped, tt.dropped)
			}
		})
	}
}

// The statistics are refreshed every 10,000 nodes and at the end; a
// refresh that fails is logged, the import going on.
func TestAnImportRefreshesTheStatistics(t *testing.T) {
	w := newImportWorld()
	var l logs
	w.logger = l.logger()
	w.stats.err = errors.New("permission denied")
	w.max.MaxEntries = 20000
	got := w.run(t, w.queuedImport(nil, many(t, 10050, func(int) string { return "" })))
	if got.State != domain.StateSucceeded || w.stats.count() != 2 || strings.Count(l.String(), "import statistics not refreshed") != 2 {
		t.Errorf("job %s, statistics refreshed %d times; logs %q", got.State, w.stats.count(), l.String())
	}
}

// The import's logs name the job, not the archive's file nor its paths.
func TestAnImportLogsTheIDsNotTheNames(t *testing.T) {
	w := newImportWorld()
	var l logs
	w.logger = l.logger()
	j := w.queuedImport(nil, vault(t))
	w.rows.set(j.ID, func(r *row) { r.job.Name = "secret-vault.zip" })
	w.run(t, j)
	text := l.String()
	if !strings.Contains(text, "import started") || !strings.Contains(text, "import ended") || !strings.Contains(text, "skipped=0") ||
		!strings.Contains(text, j.ID.String()) || strings.Contains(text, "secret-vault") || strings.Contains(text, "n:m") || strings.Contains(text, "B.md") {
		t.Errorf("logs %q", text)
	}
}
