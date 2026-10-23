package app_test

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeIndex is the search's rows and queue in memory: the queue by id, its
// nodes queued in bulk, their notebooks (notebook unless set), the
// sources the job reads, the rows it writes, in the order written. Each
// call is recorded, one off the lock's connection, its context not the one
// LockSearch returned, marked so.
type fakeIndex struct {
	calls     []string
	locked    bool // another run holds the lock
	queue     map[uuid.UUID]int64
	bulk      map[uuid.UUID]bool
	notebooks map[uuid.UUID]uuid.UUID
	notebook  uuid.UUID // of a node queued without one
	written   []uuid.UUID
	sources   map[uuid.UUID]app.SearchSource
	texts     map[uuid.UUID]app.PageText
	rows      map[uuid.UUID]app.SearchRow
	// fail fails a write that holds the node, failOnce the batch's write
	// that holds it alone, not the node's own.
	fail     map[uuid.UUID]bool
	failOnce map[uuid.UUID]bool
	// clock moves on by step at each QueuedSearch: a batch's time.
	clock *fakeClock
	step  time.Duration
	// read are the bytes of the contents each SearchContents read.
	read []int
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func newFakeIndex() *fakeIndex {
	return &fakeIndex{
		queue:     map[uuid.UUID]int64{},
		bulk:      map[uuid.UUID]bool{},
		notebooks: map[uuid.UUID]uuid.UUID{},
		notebook:  uuid.NewV7(),
		sources:   map[uuid.UUID]app.SearchSource{},
		texts:     map[uuid.UUID]app.PageText{},
		rows:      map[uuid.UUID]app.SearchRow{},
		fail:      map[uuid.UUID]bool{},
		failOnce:  map[uuid.UUID]bool{},
		clock:     &fakeClock{now: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)},
	}
}

// lockKey marks the context LockSearch returns, of the lock's connection.
type lockKey struct{}

// record records the call, marked when ctx is not the lock's.
func (f *fakeIndex) record(ctx context.Context, call string) {
	if ctx.Value(lockKey{}) == nil {
		call += " off the lock's connection"
	}
	f.calls = append(f.calls, call)
}

func (f *fakeIndex) LockSearch(ctx context.Context) (context.Context, func(), bool, error) {
	f.calls = append(f.calls, "LockSearch")
	if f.locked {
		return nil, nil, false, nil
	}
	return context.WithValue(ctx, lockKey{}, true), func() { f.calls = append(f.calls, "Release") }, true, nil
}

func (f *fakeIndex) notebookOf(id uuid.UUID) uuid.UUID {
	if n, ok := f.notebooks[id]; ok {
		return n
	}
	return f.notebook
}

// QueuedSearch reads the exact nodes by id, the bulk ones by notebook, then
// id, as the store does.
func (f *fakeIndex) QueuedSearch(ctx context.Context, bulk bool, after app.QueuedNode, limit int) ([]app.QueuedNode, error) {
	queuing := "exact " + short(after.ID)
	order := func(a, b app.QueuedNode) int { return a.ID.Compare(b.ID) }
	if bulk {
		queuing = "bulk " + short(after.NotebookID) + "/" + short(after.ID)
		order = func(a, b app.QueuedNode) int {
			return cmp.Or(a.NotebookID.Compare(b.NotebookID), a.ID.Compare(b.ID))
		}
	}
	f.record(ctx, "QueuedSearch "+queuing)
	f.clock.now = f.clock.now.Add(f.step)
	var out []app.QueuedNode
	for id, version := range f.queue {
		if n := (app.QueuedNode{ID: id, NotebookID: f.notebookOf(id), Version: version}); f.bulk[id] == bulk && order(n, after) > 0 {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, order)
	return out[:min(limit, len(out))], nil
}

func (f *fakeIndex) SearchSources(ctx context.Context, ids []uuid.UUID) ([]app.SearchSource, error) {
	f.record(ctx, fmt.Sprintf("SearchSources %d", len(ids)))
	var out []app.SearchSource
	for _, id := range ids {
		if s, ok := f.sources[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// SearchContents reads the texts of ids as the statement does: in their
// order while their sizes sum to max at most, the first past it unread.
func (f *fakeIndex) SearchContents(ctx context.Context, ids []uuid.UUID, max int64) (map[uuid.UUID]app.PageText, uuid.UUID, error) {
	f.record(ctx, fmt.Sprintf("SearchContents %d", len(ids)))
	out, total, read := map[uuid.UUID]app.PageText{}, int64(0), 0
	defer func() { f.read = append(f.read, read) }()
	for _, id := range ids {
		if t, ok := f.texts[id]; ok {
			if total += int64(len(t.Content)); total > max {
				return out, id, nil
			}
			out[id], read = t, read+len(t.Content)
		}
	}
	return out, uuid.UUID{}, nil
}

func (f *fakeIndex) WriteSearchRows(ctx context.Context, rows []app.SearchRow, deleted []uuid.UUID) error {
	f.record(ctx, fmt.Sprintf("WriteSearchRows %d %d", len(rows), len(deleted)))
	if ctx.Value(txKey{}) == nil {
		return errors.New("outside a transaction")
	}
	for _, r := range rows {
		if f.fail[r.NodeID] || f.failOnce[r.NodeID] && len(rows) > 1 {
			return errors.New("write search row: SQLSTATE 54000")
		}
	}
	for _, r := range rows {
		f.rows[r.NodeID] = r
		f.written = append(f.written, r.NodeID)
	}
	for _, id := range deleted {
		delete(f.rows, id)
	}
	return nil
}

func (f *fakeIndex) DequeueSearch(ctx context.Context, nodes []app.QueuedNode) error {
	f.record(ctx, fmt.Sprintf("DequeueSearch %d", len(nodes)))
	for _, n := range nodes {
		if f.queue[n.ID] == n.Version {
			delete(f.queue, n.ID)
			delete(f.bulk, n.ID)
		}
	}
	return nil
}

func (f *fakeIndex) AnalyzeSearch(ctx context.Context) error {
	f.record(ctx, "AnalyzeSearch")
	return nil
}

// page queues a page of content as a unit wrote it, with no row yet.
func (f *fakeIndex) page(name, content string) uuid.UUID {
	id := uuid.NewV7()
	f.queue[id] = int64(len(f.queue) + 1)
	f.sources[id] = app.SearchSource{ID: id, Kind: domain.KindPage, Name: name, Revision: 2, Bytes: len(content)}
	f.texts[id] = app.PageText{Content: content, Revision: 2}
	return id
}

// built gives id a row built at at, of revision, name and tokenizer.
func (f *fakeIndex) built(id uuid.UUID, revision int, name, tokenizer string, at time.Time) {
	s := f.sources[id]
	s.Built = &app.BuiltSearchRow{Revision: revision, Name: name, Tokenizer: tokenizer, At: at}
	f.sources[id] = s
	f.rows[id] = app.SearchRow{NodeID: id, Revision: revision, Name: name, Tokenizer: tokenizer}
}

// short labels id in the calls recorded: its last eight hex digits, of the
// random bits of a v7 id, where its first are the bits of its time that
// ids made in the same minute share (R1 fix round 3, R2-B2).
func short(id uuid.UUID) string {
	switch id {
	case uuid.Nil():
		return "start"
	case uuid.Max():
		return "end"
	}
	s := id.String()
	return s[len(s)-8:]
}

func (f *fakeIndex) job(minRebuild time.Duration) (*app.IndexSearch, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	return app.NewIndexSearch(&fakeTx{}, f, f.clock, slog.New(slog.NewTextHandler(logs, nil)), minRebuild), logs
}

// A run another holds the lock of does nothing.
func TestTheSearchJobRunsAloneOrNot(t *testing.T) {
	f := newFakeIndex()
	f.page("a", "x")
	f.locked = true
	job, _ := f.job(0)
	run, err := job.Drain(context.Background())
	if err != nil || !run.Locked || !slices.Equal(f.calls, []string{"LockSearch"}) {
		t.Errorf("Drain() = %+v, %v after %v; want locked, nothing done", run, err, f.calls)
	}
}

// Each kind of queued node (M8/P5 design 3.4): one gone has its row
// deleted; one whose row is current is taken off as it is; one built
// within minRebuild stays queued; one of another revision, name or
// tokenizer, or of no row, is built from its content as read, with its
// revision. The nodes taken are taken at the versions read.
func TestTheSearchJobDecidesEachNode(t *testing.T) {
	f := newFakeIndex()
	now := f.clock.now
	gone := f.page("gone", "")
	f.sources[gone] = app.SearchSource{ID: gone, Gone: true, Built: &app.BuiltSearchRow{Name: "gone", At: now.Add(-time.Hour)}}
	vanished := uuid.NewV7() // not there at all, no row
	f.queue[vanished] = 99
	current := f.page("current", "same")
	f.built(current, 2, "current", domain.SearchTokenizer, now.Add(-time.Hour))
	recent := f.page("recent", "edited")
	f.built(recent, 1, "recent", domain.SearchTokenizer, now.Add(-10*time.Second))
	renamed := f.page("renamed", "x")
	f.built(renamed, 2, "old name", domain.SearchTokenizer, now.Add(-time.Hour))
	retokenized := f.page("retokenized", "x")
	f.built(retokenized, 2, "retokenized", "0/15.0.0", now.Add(-time.Hour))
	fresh := f.page("中国 Fresh", "数据库 go")
	asset := uuid.NewV7()
	f.queue[asset] = 7
	f.sources[asset] = app.SearchSource{ID: asset, Kind: domain.KindAsset, Name: "报告.pdf"}

	job, _ := f.job(30 * time.Second)
	run, err := job.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.SearchRun{Written: 4, Deleted: 1, Kept: 1, Skipped: 1}); run != want {
		t.Errorf("Drain() = %+v, want %+v", run, want)
	}
	if _, ok := f.rows[gone]; ok {
		t.Error("the row of a node gone is still there")
	}
	if left := slices.Collect(maps.Keys(f.queue)); !slices.Equal(left, []uuid.UUID{recent}) {
		t.Errorf("queued after the run: %v, want the page built 10 s ago alone (%s)", left, recent)
	}
	content := "数据库 go"
	title, body, _ := domain.SearchTokens("中国 Fresh", &content)
	if r := f.rows[fresh]; r.Revision != 2 || r.Tokenizer != domain.SearchTokenizer || !slices.Equal(r.Title, title) || !slices.Equal(r.Body, body) ||
		!r.BuiltAt.Equal(f.clock.now) {
		t.Errorf("the fresh page's row = %+v, want its lexemes of revision 2 built now", r)
	}
	if r := f.rows[asset]; r.Revision != 0 || r.Body != nil || len(r.Title) == 0 {
		t.Errorf("the attachment's row = %+v, want a title alone", r)
	}
	for _, id := range []uuid.UUID{renamed, retokenized} {
		if r := f.rows[id]; r.Name != f.sources[id].Name || r.Tokenizer != domain.SearchTokenizer {
			t.Errorf("row %+v not built again", r)
		}
	}
}

// The job takes a node off at the version it read: one queued again since
// stays for the next run, which builds it as it is then (M8/P5 design 3.8,
// interleaving 1).
func TestTheSearchJobLeavesANodeQueuedAgain(t *testing.T) {
	f := newFakeIndex()
	id := f.page("a", "one")
	job, _ := f.job(0)
	f.step = 0
	sources := f.sources[id]
	// The unit writes revision 3 and queues the node again once the job read
	// the content.
	wrap := &requeuing{fakeIndex: f, id: id}
	j := app.NewIndexSearch(&fakeTx{}, wrap, f.clock, slog.New(slog.DiscardHandler), 0)
	if _, err := j.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.queue[id]; !ok || f.rows[id].Revision != 2 {
		t.Fatalf("after the first run: queued %v, row of revision %d; want queued still, the row of 2", ok, f.rows[id].Revision)
	}
	sources.Revision = 3
	f.sources[id] = sources
	f.texts[id] = app.PageText{Content: "two", Revision: 3}
	if _, err := job.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.queue[id]; ok || f.rows[id].Revision != 3 {
		t.Errorf("after the second run: queued %v, row of revision %d; want taken off, the row of 3", ok, f.rows[id].Revision)
	}
}

// requeuing queues id again, a new version, as the job reads its content.
type requeuing struct {
	*fakeIndex
	id uuid.UUID
}

func (r *requeuing) SearchContents(ctx context.Context, ids []uuid.UUID, max int64) (map[uuid.UUID]app.PageText, uuid.UUID, error) {
	out, past, err := r.fakeIndex.SearchContents(ctx, ids, max)
	r.queue[r.id]++
	return out, past, err
}

// A pass reads the queue by node id, a batch of SearchBatchNodes at a time,
// the contents of a batch SearchBatchBytes at most: the batch stops before
// the page that would pass it, and the next goes on from the last node
// decided, each node read once.
func TestTheSearchJobPassesOverTheQueueInBatches(t *testing.T) {
	f := newFakeIndex()
	big := strings.Repeat("x", 3<<20)
	var ids []uuid.UUID
	for i := range 4 {
		ids = append(ids, f.page(fmt.Sprintf("big %d", i), big))
	}
	for i := range domain.SearchBatchNodes {
		ids = append(ids, f.page(fmt.Sprintf("small %d", i), "y"))
	}
	job, _ := f.job(0)
	run, err := job.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Two big pages in the first batch (6 MiB: a third would pass 8), which
	// goes on after the second; the next two and 96 small ones in the
	// second; the last two in the third.
	want := []string{"LockSearch", "QueuedSearch exact start", "SearchSources 100", "SearchContents 2", "WriteSearchRows 2 0",
		"DequeueSearch 2", "QueuedSearch exact " + short(ids[1]), "SearchSources 100", "SearchContents 100", "WriteSearchRows 100 0",
		"DequeueSearch 100", "QueuedSearch exact " + short(ids[101]), "SearchSources 2", "SearchContents 2", "WriteSearchRows 2 0",
		"DequeueSearch 2", "QueuedSearch exact " + short(ids[103]), "QueuedSearch bulk start/start", "Release"}
	if !slices.Equal(f.calls, want) || run.Written != len(ids) || len(f.queue) != 0 {
		t.Errorf("calls %v\nwant %v\nrun %+v, %d left queued", f.calls, want, run, len(f.queue))
	}
}

// A batch's read of the contents holds SearchBatchBytes on the bytes it
// reads, not on the sizes its sources told (M9 Codex review D2): ten pages
// of a byte by their sources, the fourth to sixth grown to 3 MiB since.
// The first batch reads the first five, within 8 MiB, the sixth past it
// read none of: the batch ends before it, which the next batch reads with
// the rest. Every page built, each read within the bound.
func TestTheSearchJobsBatchReadsWithinItsBytes(t *testing.T) {
	f := newFakeIndex()
	var ids []uuid.UUID
	for i := range 10 {
		ids = append(ids, f.page(fmt.Sprintf("page %d", i), "y"))
	}
	for _, id := range ids[3:6] {
		f.texts[id] = app.PageText{Content: strings.Repeat("x", 3<<20), Revision: 2}
	}
	job, _ := f.job(0)
	run, err := job.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LockSearch", "QueuedSearch exact start", "SearchSources 10", "SearchContents 10", "WriteSearchRows 5 0",
		"DequeueSearch 5", "QueuedSearch exact " + short(ids[4]), "SearchSources 5", "SearchContents 5", "WriteSearchRows 5 0",
		"DequeueSearch 5", "QueuedSearch exact " + short(ids[9]), "QueuedSearch bulk start/start", "Release"}
	if !slices.Equal(f.calls, want) || run.Written != len(ids) || len(f.queue) != 0 ||
		slices.ContainsFunc(f.read, func(n int) bool { return n > domain.SearchBatchBytes }) {
		t.Errorf("calls %v\nwant %v\nrun %+v, %d left queued, reads %v", f.calls, want, run, len(f.queue), f.read)
	}
}

// A batch cut before a page grown past its bytes counts and deletes only
// the nodes before the cut (M9 Codex review fix round 1, B-N2): ten pages,
// the fourth to sixth grown to 3 MiB since their sources, the eighth's row
// current, the ninth's built within minRebuild, the tenth gone with a row.
// The first batch builds five, cut at the sixth, and neither counts the
// eighth and the ninth nor deletes the tenth's row; the second builds the
// sixth and the seventh, keeps the eighth, skips the ninth, deletes the
// tenth's row: each counted once.
func TestTheSearchJobsCutBatchCountsItsOwnNodes(t *testing.T) {
	f := newFakeIndex()
	now := f.clock.now
	var ids []uuid.UUID
	for i := range 10 {
		ids = append(ids, f.page(fmt.Sprintf("page %d", i), "y"))
	}
	for _, id := range ids[3:6] {
		f.texts[id] = app.PageText{Content: strings.Repeat("x", 3<<20), Revision: 2}
	}
	f.built(ids[7], 2, "page 7", domain.SearchTokenizer, now.Add(-time.Hour))
	f.built(ids[8], 1, "page 8", domain.SearchTokenizer, now.Add(-10*time.Second))
	f.sources[ids[9]] = app.SearchSource{ID: ids[9], Gone: true, Built: &app.BuiltSearchRow{Name: "page 9", At: now.Add(-time.Hour)}}
	f.rows[ids[9]] = app.SearchRow{NodeID: ids[9], Name: "page 9"}
	job, _ := f.job(30 * time.Second)
	run, err := job.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writes := slices.DeleteFunc(slices.Clone(f.calls), func(c string) bool { return !strings.HasPrefix(c, "WriteSearchRows") })
	if want := (app.SearchRun{Written: 7, Deleted: 1, Kept: 1, Skipped: 1}); run != want ||
		!slices.Equal(writes, []string{"WriteSearchRows 5 0", "WriteSearchRows 2 1"}) {
		t.Errorf("Drain() = %+v, writes %v; want %+v, the tenth's row deleted by the second batch", run, writes, want)
	}
}

// A run takes the nodes queued exact first, a user's writes, by id, then
// those queued in bulk, an import's or a background job's, by id: however
// many the bulk ones and however low their ids, a user's write never waits
// behind them (M8/P5A review C-I1).
func TestTheSearchJobTakesTheExactNodesFirst(t *testing.T) {
	f := newFakeIndex()
	var bulk []uuid.UUID
	for i := range domain.SearchBatchNodes + 1 {
		id := f.page(fmt.Sprintf("imported %d", i), "x")
		f.bulk[id] = true
		bulk = append(bulk, id)
	}
	exact := []uuid.UUID{f.page("written", "y"), f.page("renamed", "z")}
	job, _ := f.job(0)
	run, err := job.Drain(context.Background())
	if err != nil || run.Written != len(bulk)+len(exact) || len(f.queue) != 0 {
		t.Fatalf("Drain() = %+v, %v, %d left; want every node built", run, err, len(f.queue))
	}
	want := []string{"LockSearch", "QueuedSearch exact start", "SearchSources 2", "SearchContents 2", "WriteSearchRows 2 0", "DequeueSearch 2",
		"QueuedSearch exact " + short(exact[1]), "QueuedSearch bulk start/start", "SearchSources 100", "SearchContents 100",
		"WriteSearchRows 100 0", "DequeueSearch 100", "QueuedSearch bulk " + short(f.notebook) + "/" + short(bulk[99]), "SearchSources 1",
		"SearchContents 1", "WriteSearchRows 1 0", "DequeueSearch 1", "QueuedSearch bulk " + short(f.notebook) + "/" + short(bulk[100]),
		"Release"}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls %v\nwant %v", f.calls, want)
	}
}

// The bulk pass goes notebook by notebook, by notebook id, then node id
// (M8 Codex review R1, fix round 2: B-3): pages of three notebooks queued
// in bulk, their ids interleaved, are written a notebook's at a time.
func TestTheSearchJobBuildsTheBulkNotebookByNotebook(t *testing.T) {
	f := newFakeIndex()
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	var want []uuid.UUID
	byNotebook := map[uuid.UUID][]uuid.UUID{}
	for i := range 5 {
		for _, n := range []uuid.UUID{c, a, b} {
			id := f.page(fmt.Sprint(n, i), "x")
			f.bulk[id], f.notebooks[id] = true, n
			byNotebook[n] = append(byNotebook[n], id)
		}
	}
	for _, n := range []uuid.UUID{a, b, c} {
		want = append(want, byNotebook[n]...)
	}
	job, _ := f.job(0)
	if run, err := job.Drain(context.Background()); err != nil || run.Written != 15 || len(f.queue) != 0 {
		t.Fatalf("Drain() = %+v, %v, %d left; want every node built", run, err, len(f.queue))
	}
	if !slices.Equal(f.written, want) {
		t.Errorf("written in the order %v, want %v: a's, b's, then c's, each by id", f.written, want)
	}
}

// bulkPages queues count pages of notebook in bulk.
func (f *fakeIndex) bulkPages(notebook uuid.UUID, count int) []uuid.UUID {
	var ids []uuid.UUID
	for i := range count {
		id := f.page(fmt.Sprint(notebook, i), "x")
		f.bulk[id], f.notebooks[id] = true, notebook
		ids = append(ids, id)
	}
	return ids
}

// left is how many of ids are queued.
func (f *fakeIndex) left(ids []uuid.UUID) int {
	n := 0
	for _, id := range ids {
		if _, ok := f.queue[id]; ok {
			n++
		}
	}
	return n
}

// bulkReads are the calls' reads of the bulk queue.
func bulkReads(calls []string) []string {
	return slices.DeleteFunc(slices.Clone(calls), func(c string) bool { return !strings.HasPrefix(c, "QueuedSearch bulk") })
}

// Each run's bulk pass begins after the notebook the last one's ended in,
// and comes round to it: a notebook queued behind another's thousands
// waits a run, not until they are built (R1 fix round 2, B-3). Runs that
// start one batch alone (the clock past their budget at the second): the
// first builds 100 of a's 150 pages; the next, after a, b's 10; the next,
// after b, finds none, comes round to the start and builds a's last 50.
func TestTheSearchJobComesRoundTheNotebooks(t *testing.T) {
	f := newFakeIndex()
	a, b := uuid.NewV7(), uuid.NewV7()
	inA, inB := f.bulkPages(a, 150), f.bulkPages(b, 10)
	f.step = domain.SearchRunBudget / 2
	job, _ := f.job(0)
	for i, want := range []struct {
		reads         []string
		inA, inB, all int
	}{
		{[]string{"QueuedSearch bulk start/start"}, 50, 10, 100},
		{[]string{"QueuedSearch bulk " + short(a) + "/end"}, 50, 0, 10},
		{[]string{"QueuedSearch bulk " + short(b) + "/end", "QueuedSearch bulk start/start"}, 0, 0, 50},
	} {
		f.calls = nil
		run, err := job.Drain(context.Background())
		if err != nil || !slices.Equal(bulkReads(f.calls), want.reads) || f.left(inA) != want.inA || f.left(inB) != want.inB ||
			run.Written != want.all {
			t.Errorf("run %d = %+v, %v, reads %v, %d of a and %d of b left; want %v, %d written, %d and %d left", i+1, run, err,
				bulkReads(f.calls), f.left(inA), f.left(inB), want.reads, want.all, want.inA, want.inB)
		}
	}
}

// A pass that came round to the start stops at the notebook it began
// after: a node it decided on in this run is not decided on again. The
// last run ended in b; this one begins after it, with c's page, built
// within minRebuild and left queued, comes round to a's, and stops before
// c's again.
func TestTheSearchJobComesRoundOnce(t *testing.T) {
	f := newFakeIndex()
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.bulkPages(b, 1)
	job, _ := f.job(time.Hour)
	if run, err := job.Drain(context.Background()); err != nil || run.Written != 1 {
		t.Fatalf("Drain() = %+v, %v; want b's page built", run, err)
	}
	inA := f.bulkPages(a, 3)
	recent := f.bulkPages(c, 1)[0]
	f.built(recent, 1, f.sources[recent].Name, domain.SearchTokenizer, f.clock.now)
	f.calls = nil
	run, err := job.Drain(context.Background())
	want := []string{"QueuedSearch bulk " + short(b) + "/end", "QueuedSearch bulk " + short(c) + "/" + short(recent),
		"QueuedSearch bulk start/start", "QueuedSearch bulk " + short(a) + "/" + short(inA[2])}
	if err != nil || run.Written != 3 || run.Skipped != 1 || !slices.Equal(bulkReads(f.calls), want) {
		t.Errorf("Drain() = %+v, %v, reads %v; want a's 3 built, c's skipped once, reads %v", run, err, bulkReads(f.calls), want)
	}
}

// A notebook's import longer than a run is built a run at a time (R1 fix
// round 3, R2-B1): each run's pass begins after the notebook, finds no
// other, comes round to the start and builds a batch more of it. Runs that
// start one batch alone build 100, 100, 100, then the last 50 of 350.
func TestTheSearchJobBuildsALongImportOfOneNotebook(t *testing.T) {
	f := newFakeIndex()
	a := uuid.NewV7()
	inA := f.bulkPages(a, 350)
	f.step = domain.SearchRunBudget / 2
	job, _ := f.job(0)
	for i, want := range []struct {
		reads   []string
		written int
	}{
		{[]string{"QueuedSearch bulk start/start"}, 100},
		{[]string{"QueuedSearch bulk " + short(a) + "/end", "QueuedSearch bulk start/start"}, 100},
		{[]string{"QueuedSearch bulk " + short(a) + "/end", "QueuedSearch bulk start/start"}, 100},
		{[]string{"QueuedSearch bulk " + short(a) + "/end", "QueuedSearch bulk start/start"}, 50},
	} {
		f.calls = nil
		run, err := job.Drain(context.Background())
		if err != nil || run.Written != want.written || !slices.Equal(bulkReads(f.calls), want.reads) {
			t.Errorf("run %d = %+v, %v, reads %v; want %d written, reads %v", i+1, run, err, bulkReads(f.calls), want.written, want.reads)
		}
	}
	if left := f.left(inA); left != 0 {
		t.Errorf("%d of a's 350 pages left queued after four runs, want none", left)
	}
}

// Each run's pass begins after the notebook the last one's ended in, which
// moves from run to run (R1 fix round 3, R2-B2): of three notebooks of 150
// pages, runs that start one batch alone begin at the start, then after a,
// after b, after c.
func TestTheSearchJobsPassBeginsWhereTheLastEnded(t *testing.T) {
	f := newFakeIndex()
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, n := range []uuid.UUID{a, b, c} {
		f.bulkPages(n, 150)
	}
	f.step = domain.SearchRunBudget / 2
	job, _ := f.job(0)
	var firsts []string
	for range 4 {
		f.calls = nil
		if _, err := job.Drain(context.Background()); err != nil {
			t.Fatal(err)
		}
		firsts = append(firsts, bulkReads(f.calls)[0])
	}
	want := []string{"QueuedSearch bulk start/start", "QueuedSearch bulk " + short(a) + "/end", "QueuedSearch bulk " + short(b) + "/end",
		"QueuedSearch bulk " + short(c) + "/end"}
	if !slices.Equal(firsts, want) {
		t.Errorf("the runs' first reads of the bulk queue %v, want %v", firsts, want)
	}
}

// A batch whose write fails is written again a node at a time: the node
// that fails then is logged, without its text, and left queued; the
// others are written and taken off. The run goes on.
func TestTheSearchJobWritesAroundAFailingNode(t *testing.T) {
	f := newFakeIndex()
	a := f.page("a", "one")
	bad := f.page("bad", "secret words")
	c := f.page("c", "three")
	f.fail[bad] = true
	job, logs := f.job(0)
	run, err := job.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if run.Written != 2 || run.Failed != 1 {
		t.Errorf("Drain() = %+v, want 2 written, 1 failed", run)
	}
	if _, ok := f.queue[bad]; !ok || len(f.queue) != 1 || f.rows[a].NodeID != a || f.rows[c].NodeID != c {
		t.Errorf("queue %v, rows %v: want the failing node queued alone, the others written", f.queue, f.rows)
	}
	if !strings.Contains(logs.String(), bad.String()) || !strings.Contains(logs.String(), "SQLSTATE 54000") ||
		strings.Contains(logs.String(), "secret") {
		t.Errorf("logs %q: want the node's id and SQLSTATE, not its text", logs.String())
	}
	if !slices.Contains(f.calls, "WriteSearchRows 3 0") || !slices.Contains(f.calls, "WriteSearchRows 1 0") {
		t.Errorf("calls %v: want the batch's write, then one a node", f.calls)
	}
}

// notBegunTx runs its first transactions as fakeTx does, then begins none:
// the run's connection gone, each is refused before fn runs, as
// platform/postgres marks it.
type notBegunTx struct {
	fakeTx
	begins, refused int
}

type txNotBegun struct{}

func (txNotBegun) Error() string    { return "begin transaction: conn closed" }
func (txNotBegun) TxNotBegun() bool { return true }

func (t *notBegunTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if t.InTx(ctx) {
		return fn(ctx)
	}
	if t.begins == 0 {
		t.refused++
		return txNotBegun{}
	}
	t.begins--
	return t.fakeTx.WithinTx(ctx, fn)
}

// A transaction that cannot begin fails the run, with that one error: the
// connection is gone, not any node's write. No node is tried alone on it,
// none logged as not written; all stay queued (M8/P5A fix check M-1).
func TestTheSearchJobEndsWhenATransactionCannotBegin(t *testing.T) {
	for _, begins := range []int{0, 1} { // the batch's transaction refused; the batch's fails, the first node's refused
		f := newFakeIndex()
		f.page("a", "one")
		bad := f.page("bad", "two")
		f.page("c", "three")
		f.fail[bad] = true
		logs := &bytes.Buffer{}
		tx := &notBegunTx{begins: begins}
		job := app.NewIndexSearch(tx, f, f.clock, slog.New(slog.NewTextHandler(logs, nil)), 0)
		run, err := job.Drain(context.Background())
		if !shared.TxNotBegun(err) || run.Written != 0 || run.Failed != 0 || len(f.queue) != 3 || len(f.rows) != 0 {
			t.Errorf("%d begun: Drain() = %+v, %v; queued %d, rows %d; want begin's error, nothing written or failed, all queued",
				begins, run, err, len(f.queue), len(f.rows))
		}
		// The batch's failure stays in the run's error (fix check N2-1).
		if begins == 1 && (err == nil || !strings.Contains(err.Error(), "SQLSTATE 54000")) {
			t.Errorf("%d begun: Drain() = %v, want the batch's failure in it", begins, err)
		}
		if strings.Contains(logs.String(), "not written") || slices.Contains(f.calls, "WriteSearchRows 1 0") || tx.refused != 1 {
			t.Errorf("%d begun: logs %q, calls %v, %d transactions refused; want the first refused alone, no node tried alone, none logged",
				begins, logs.String(), f.calls, tx.refused)
		}
	}
}

// A page deleted between its source's read and its content's is left
// queued: its deletion queued it again, and the next run deletes its row.
func TestTheSearchJobLeavesAPageDeletedMeanwhile(t *testing.T) {
	f := newFakeIndex()
	id := f.page("a", "one")
	delete(f.texts, id)
	job, _ := f.job(0)
	run, err := job.Drain(context.Background())
	if _, ok := f.queue[id]; err != nil || !ok || run.Skipped != 1 || len(f.rows) != 0 {
		t.Errorf("Drain() = %+v, %v; queued %v, rows %v: want it left queued, no row", run, err, ok, f.rows)
	}
}

// The job analyzes page_search each SearchAnalyzeRows rows its runs write,
// counted across them (M8/P5A review B-N4), and starts no batch after
// SearchRunBudget.
func TestTheSearchJobAnalyzesAndStopsOnTime(t *testing.T) {
	f := newFakeIndex()
	for i := range domain.SearchAnalyzeRows + domain.SearchBatchNodes {
		f.page(fmt.Sprint(i), "x")
	}
	job, _ := f.job(0)
	if _, err := job.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := countOf(f.calls, "AnalyzeSearch"); n != 1 || len(f.queue) != 0 {
		t.Errorf("ANALYZE %d times, %d left queued; want once, none", n, len(f.queue))
	}
	// Runs of 3,000 rows each: the fourth analyzes, its 12,000 rows past
	// the bound, and so does the next run past it again.
	h := newFakeIndex()
	job, _ = h.job(0)
	for run := 1; run <= 7; run++ {
		for i := range 3000 {
			h.page(fmt.Sprint(run, i), "x")
		}
		before := countOf(h.calls, "AnalyzeSearch")
		if _, err := job.Drain(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got, want := countOf(h.calls, "AnalyzeSearch")-before, map[int]int{4: 1, 7: 1}[run]; got != want {
			t.Errorf("run %d analyzed %d times, want %d: the rows of the runs counted across them", run, got, want)
		}
	}
	g := newFakeIndex()
	for i := range 5 * domain.SearchBatchNodes {
		g.page(fmt.Sprint(i), "x")
	}
	g.step = domain.SearchRunBudget / 2
	job, _ = g.job(0)
	run, err := job.Drain(context.Background())
	// Two batches start within the budget, the clock at none and at half of
	// it; no third, at all of it.
	if err != nil || run.Written != 2*domain.SearchBatchNodes || len(g.queue) != 3*domain.SearchBatchNodes {
		t.Errorf("Drain() = %+v, %v, %d left; want two batches within the budget", run, err, len(g.queue))
	}
}

func countOf(calls []string, call string) int {
	n := 0
	for _, c := range calls {
		if c == call {
			n++
		}
	}
	return n
}
