package app_test

import (
	"bytes"
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
// nodes queued in bulk, the sources the job reads, the rows it writes. Each
// call is recorded, one off the lock's connection, its context not the one
// LockSearch returned, marked so.
type fakeIndex struct {
	calls   []string
	locked  bool // another run holds the lock
	queue   map[uuid.UUID]int64
	bulk    map[uuid.UUID]bool
	sources map[uuid.UUID]app.SearchSource
	texts   map[uuid.UUID]app.PageText
	rows    map[uuid.UUID]app.SearchRow
	// fail fails a write that holds the node, failOnce the batch's write
	// that holds it alone, not the node's own.
	fail     map[uuid.UUID]bool
	failOnce map[uuid.UUID]bool
	// clock moves on by step at each QueuedSearch: a batch's time.
	clock *fakeClock
	step  time.Duration
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func newFakeIndex() *fakeIndex {
	return &fakeIndex{
		queue:    map[uuid.UUID]int64{},
		bulk:     map[uuid.UUID]bool{},
		sources:  map[uuid.UUID]app.SearchSource{},
		texts:    map[uuid.UUID]app.PageText{},
		rows:     map[uuid.UUID]app.SearchRow{},
		fail:     map[uuid.UUID]bool{},
		failOnce: map[uuid.UUID]bool{},
		clock:    &fakeClock{now: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)},
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

func (f *fakeIndex) QueuedSearch(ctx context.Context, bulk bool, after uuid.UUID, limit int) ([]app.QueuedNode, error) {
	queuing := "exact"
	if bulk {
		queuing = "bulk"
	}
	f.record(ctx, "QueuedSearch "+queuing+" "+short(after))
	f.clock.now = f.clock.now.Add(f.step)
	var ids []uuid.UUID
	for id := range f.queue {
		if f.bulk[id] == bulk && id.Compare(after) > 0 {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	var out []app.QueuedNode
	for _, id := range ids[:min(limit, len(ids))] {
		out = append(out, app.QueuedNode{ID: id, Version: f.queue[id]})
	}
	return out, nil
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

func (f *fakeIndex) SearchContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageText, error) {
	f.record(ctx, fmt.Sprintf("SearchContents %d", len(ids)))
	out := map[uuid.UUID]app.PageText{}
	for _, id := range ids {
		if t, ok := f.texts[id]; ok {
			out[id] = t
		}
	}
	return out, nil
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

func short(id uuid.UUID) string {
	if id == uuid.Nil() {
		return "start"
	}
	return id.String()[:8]
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
	title, body := domain.SearchTokens("中国 Fresh", &content)
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

func (r *requeuing) SearchContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageText, error) {
	out, err := r.fakeIndex.SearchContents(ctx, ids)
	r.queue[r.id]++
	return out, err
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
		"DequeueSearch 2", "QueuedSearch exact " + short(ids[103]), "QueuedSearch bulk start", "Release"}
	if !slices.Equal(f.calls, want) || run.Written != len(ids) || len(f.queue) != 0 {
		t.Errorf("calls %v\nwant %v\nrun %+v, %d left queued", f.calls, want, run, len(f.queue))
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
		"QueuedSearch exact " + short(exact[1]), "QueuedSearch bulk start", "SearchSources 100", "SearchContents 100", "WriteSearchRows 100 0",
		"DequeueSearch 100", "QueuedSearch bulk " + short(bulk[99]), "SearchSources 1", "SearchContents 1", "WriteSearchRows 1 0",
		"DequeueSearch 1", "QueuedSearch bulk " + short(bulk[100]), "Release"}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls %v\nwant %v", f.calls, want)
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
	begins int
}

type txNotBegun struct{}

func (txNotBegun) Error() string    { return "begin transaction: conn closed" }
func (txNotBegun) TxNotBegun() bool { return true }

func (t *notBegunTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if t.InTx(ctx) {
		return fn(ctx)
	}
	if t.begins == 0 {
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
		job := app.NewIndexSearch(&notBegunTx{begins: begins}, f, f.clock, slog.New(slog.NewTextHandler(logs, nil)), 0)
		run, err := job.Drain(context.Background())
		if !shared.TxNotBegun(err) || run.Written != 0 || run.Failed != 0 || len(f.queue) != 3 || len(f.rows) != 0 {
			t.Errorf("%d begun: Drain() = %+v, %v; queued %d, rows %d; want begin's error, nothing written or failed, all queued",
				begins, run, err, len(f.queue), len(f.rows))
		}
		if strings.Contains(logs.String(), "not written") || slices.Contains(f.calls, "WriteSearchRows 1 0") {
			t.Errorf("%d begun: logs %q, calls %v; want no node tried alone, none logged", begins, logs.String(), f.calls)
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
