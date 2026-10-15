package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf16"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/unicode/norm"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The search's statements (M8/P5 design 3.4, 3.5, 3.10): its queue, its
// job's reads and writes, the two tiers, the count, and its patterns as
// PostgreSQL runs them.

// drain runs the search's job once over f's store as the tests run it,
// building a row again however recently it built it (page.SearchIndex's
// Drain), and fails unless it emptied the queue.
func (f fixture) drain(t *testing.T) {
	t.Helper()
	job := app.NewIndexSearch(postgres.NewTxManager(f.pool, 5*time.Second), f.s, clock.System{}, slog.New(slog.DiscardHandler), 0)
	run, err := job.Drain(context.Background())
	if err != nil || run.Locked || run.Failed > 0 {
		t.Fatalf("Drain = %+v, %v", run, err)
	}
	if n := f.count(t, `SELECT count(*) FROM page_search_queue`); n != 0 {
		t.Fatalf("the queue holds %d nodes after a run", n)
	}
}

// queueSearch queues ids of notebook for the search's job, in order, exact
// or in bulk, as a unit, a notebook's deletion or nervewiki reindex does.
func (f fixture) queueSearch(t *testing.T, notebook uuid.UUID, bulk bool, ids ...uuid.UUID) {
	t.Helper()
	if err := f.s.QueueSearch(context.Background(), notebook, slices.SortedFunc(slices.Values(ids), uuid.UUID.Compare), bulk); err != nil {
		t.Fatal(err)
	}
}

// queuedNode is a node's place in the search's queue.
type queuedNode struct {
	notebook uuid.UUID
	version  int64
	bulk     bool
}

// queued reads the search's queue.
func (f fixture) queued(t *testing.T) map[uuid.UUID]queuedNode {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT node_id, notebook_id, version, bulk FROM page_search_queue`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]queuedNode{}
	for rows.Next() {
		var id uuid.UUID
		var q queuedNode
		if err := rows.Scan(&id, &q.notebook, &q.version, &q.bulk); err != nil {
			t.Fatal(err)
		}
		out[id] = q
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// writeRows writes rows and deletes the rows of deleted in a transaction,
// as the job does.
func (f fixture) writeRows(t *testing.T, rows []app.SearchRow, deleted ...uuid.UUID) {
	t.Helper()
	err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
		return f.s.WriteSearchRows(ctx, rows, deleted)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// changedAt makes at the time the node id and its content, if any, last
// changed: its place in a search's order.
func (f fixture) changedAt(t *testing.T, id uuid.UUID, at time.Time) {
	t.Helper()
	f.exec(t, `UPDATE nodes SET updated_at = $2 WHERE id = $1`, id, at)
	f.exec(t, `UPDATE page_contents SET updated_at = $2 WHERE node_id = $1`, id, at)
}

// inSnapshot is what read reads in a read-only snapshot of f's database,
// as the use case reads a search: the store's search reads run in its
// transaction. read's error comes back as it is.
func inSnapshot[T any](f fixture, read func(ctx context.Context) (T, error)) (T, error) {
	var out T
	err := postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = read(ctx)
		return err
	})
	return out, err
}

// scopeOf is the scope of the query q over notebooks, from the top.
func scopeOf(t *testing.T, q string, notebooks ...uuid.UUID) app.SearchScope {
	t.Helper()
	parsed, err := domain.ParseSearchQuery(&q)
	if err != nil {
		t.Fatalf("ParseSearchQuery(%q) = %v", q, err)
	}
	return app.SearchScope{Notebooks: notebooks, Query: parsed, Below: domain.SearchFrom(domain.SearchTierTitle)}
}

// bodies walks the body tier of sc as the use case does: its hits counted
// in ctx's snapshot (SearchHitCount), then read, all of them.
func (f fixture) bodies(ctx context.Context, sc app.SearchScope, walk, limit int) ([]app.SearchBodyRow, error) {
	n, err := f.s.SearchHitCount(ctx, sc.Query)
	if err != nil {
		return nil, err
	}
	sc.Hits = n
	return f.s.SearchBodies(ctx, sc, walk, limit)
}

// titlesOf is the names the title tier finds for q in notebooks, from its
// top, in its order.
func (f fixture) titlesOf(t *testing.T, q string, notebooks ...uuid.UUID) []string {
	t.Helper()
	sc := scopeOf(t, q, notebooks...)
	rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchTitleRow, error) { return f.s.SearchTitles(ctx, sc, 50) })
	if err != nil {
		t.Fatalf("SearchTitles(%q) = %v", q, err)
	}
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Node.Name)
	}
	return out
}

// bodiesOf is the names of the pages that pass the body tier's walk for q
// in notebooks, from its top, in its order.
func (f fixture) bodiesOf(t *testing.T, q string, notebooks ...uuid.UUID) []string {
	t.Helper()
	sc := scopeOf(t, q, notebooks...)
	rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
		return f.bodies(ctx, sc, domain.MaxSearchWalk, 50)
	})
	if err != nil {
		t.Fatalf("SearchBodies(%q) = %v", q, err)
	}
	out := []string{}
	for _, r := range rows {
		if r.Passed {
			out = append(out, r.Node.Name)
		}
	}
	return out
}

// A node queued takes a new version from the sequence, queued again too,
// so that the job's take of an older read misses it. A statement takes
// its ids in ascending order, whatever order they come in (v0.1 design
// 13.1, item 5), which the versions it gives them show; a node stays bulk
// only while each queueing of it is (M8/P5 design 3.4).
func TestQueueSearch(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "N", 3)
	a, b, c := ids[0], ids[1], ids[2]
	if err := f.s.QueueSearch(ctx, f.eng, []uuid.UUID{c, a, b}, true); err != nil {
		t.Fatal(err)
	}
	q := f.queued(t)
	if len(q) != 3 || q[a].version >= q[b].version || q[b].version >= q[c].version {
		t.Fatalf("the queue = %+v; want a, b and c, their versions in the ids' order", q)
	}
	for _, id := range ids {
		if q[id].notebook != f.eng || !q[id].bulk {
			t.Errorf("%v queued as %+v, want in bulk in eng", id, q[id])
		}
	}
	last := q[c].version
	for _, step := range []struct {
		name string
		id   uuid.UUID
		bulk bool
		want bool
	}{
		{"bulk, then exact", b, false, false},
		{"exact, then bulk", b, true, false},
		{"bulk, then bulk", c, true, true},
	} {
		if err := f.s.QueueSearch(ctx, f.eng, []uuid.UUID{step.id}, step.bulk); err != nil {
			t.Fatal(err)
		}
		got := f.queued(t)[step.id]
		if got.bulk != step.want || got.version <= last {
			t.Errorf("queued %s: %+v; want bulk %v at a version past %d", step.name, got, step.want, last)
		}
		last = got.version
	}
	if got := f.queued(t)[a]; got != q[a] {
		t.Errorf("a, not queued again, is %+v; want %+v", got, q[a])
	}
}

// A deletion of notebooks queues exact the nodes it deletes, those deleted
// at its time, by id; not those in the trash before, nor those of another
// notebook deleted at that time. A node queued in bulk before becomes
// exact, at a new version (M8/P5 design 3.4).
func TestQueueSearchOfNotebooks(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := now().Add(time.Hour)
	ops := f.nodes(t, f.ops, "O", 4) // deleted now, deleted now and queued in bulk before, in the trash before, alive
	eng := f.nodes(t, f.eng, "E", 1) // deleted now, of a notebook not deleted
	f.exec(t, `UPDATE nodes SET deleted_at = $2 WHERE id = ANY($1)`, []uuid.UUID{ops[0], ops[1], eng[0]}, at)
	f.exec(t, `UPDATE nodes SET deleted_at = $2 WHERE id = $1`, ops[2], now())
	f.queueSearch(t, f.ops, true, ops[1])
	before := f.queued(t)[ops[1]].version
	if err := f.s.QueueSearchOfNotebooks(ctx, []uuid.UUID{f.ops}, at); err != nil {
		t.Fatal(err)
	}
	q := f.queued(t)
	if len(q) != 2 {
		t.Errorf("the queue = %+v; want the two nodes the deletion deleted", q)
	}
	for _, id := range ops[:2] {
		if got := q[id]; got.notebook != f.ops || got.bulk || got.version <= before {
			t.Errorf("%v queued as %+v; want exact in ops, at a version past %d", id, got, before)
		}
	}
}

// nervewiki reindex queues a notebook's live nodes whose row is missing or
// of another tokenizer than the current, by id, a batch after another (M8/P5
// design 3.4).
func TestStaleSearchNodes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "N", 5)
	f.nodes(t, f.ops, "O", 1) // another notebook's, without a row
	row := func(id uuid.UUID, tokenizer string) app.SearchRow {
		return app.SearchRow{NodeID: id, Name: "N", Tokenizer: tokenizer, BuiltAt: now(), Title: []string{"n"}}
	}
	f.writeRows(t, []app.SearchRow{row(ids[0], domain.SearchTokenizer), row(ids[1], "0/old")})
	f.exec(t, `UPDATE nodes SET deleted_at = $2 WHERE id = $1`, ids[3], now())
	for _, tt := range []struct {
		after uuid.UUID
		limit int
		want  []uuid.UUID
	}{
		{uuid.Nil(), 10, []uuid.UUID{ids[1], ids[2], ids[4]}},
		{uuid.Nil(), 2, []uuid.UUID{ids[1], ids[2]}},
		{ids[2], 10, []uuid.UUID{ids[4]}},
	} {
		got, err := f.s.StaleSearchNodes(ctx, f.eng, tt.after, domain.SearchTokenizer, tt.limit)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("StaleSearchNodes(after %v, %d) = %v, %v; want %v", tt.after, tt.limit, got, err, tt.want)
		}
	}
}

// The job reads the queue by id, a batch after another, with the versions
// queued, the nodes queued exact apart from those in bulk, and takes the
// nodes off at the versions it read: one a unit queued again since stays,
// at its new version (M8/P5 design 3.4).
func TestDequeueSearchTakesTheVersionsRead(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "N", 4)
	imported := f.nodes(t, f.eng, "I", 2)
	f.queueSearch(t, f.eng, true, imported...)
	f.queueSearch(t, f.eng, false, ids...)
	q := f.queued(t)
	bulk, err := f.s.QueuedSearch(ctx, true, uuid.Nil(), 10)
	if want := []app.QueuedNode{{ID: imported[0], Version: q[imported[0]].version}, {ID: imported[1], Version: q[imported[1]].version}}; err != nil ||
		!slices.Equal(bulk, want) {
		t.Fatalf("QueuedSearch(bulk, the start, 10) = %v, %v; want %v, those queued in bulk alone", bulk, err, want)
	}
	first, err := f.s.QueuedSearch(ctx, false, uuid.Nil(), 2)
	if want := []app.QueuedNode{{ID: ids[0], Version: q[ids[0]].version}, {ID: ids[1], Version: q[ids[1]].version}}; err != nil ||
		!slices.Equal(first, want) {
		t.Fatalf("QueuedSearch(exact, the start, 2) = %v, %v; want %v", first, err, want)
	}
	rest, err := f.s.QueuedSearch(ctx, false, ids[1], 10)
	if want := []app.QueuedNode{{ID: ids[2], Version: q[ids[2]].version}, {ID: ids[3], Version: q[ids[3]].version}}; err != nil ||
		!slices.Equal(rest, want) {
		t.Fatalf("QueuedSearch(exact, after the second, 10) = %v, %v; want %v", rest, err, want)
	}
	f.queueSearch(t, f.eng, false, ids[1])
	again := f.queued(t)[ids[1]]
	if err := f.s.DequeueSearch(ctx, append(append(first, rest[0]), bulk...)); err != nil {
		t.Fatal(err)
	}
	got := f.queued(t)
	if want := map[uuid.UUID]queuedNode{ids[1]: again, ids[3]: q[ids[3]]}; len(got) != 2 || got[ids[1]] != want[ids[1]] ||
		got[ids[3]] != want[ids[3]] {
		t.Errorf("the queue after the take = %+v; want %+v: the second queued again since, the last not taken", got, want)
	}
}

// The job's take skips a queued node a unit holds, queueing it again in a
// transaction still open, rather than wait for it (M8/P5 design 3.4): the
// others go, the held one stays, at the version the unit gives it once it
// commits, for the next run to read. A take that waited would end at its
// deadline, which bounds how long a regression hangs the test.
func TestDequeueSearchSkipsAHeldRow(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "N", 3)
	f.queueSearch(t, f.eng, false, ids...)
	read, err := f.s.QueuedSearch(ctx, false, uuid.Nil(), 10)
	if err != nil || len(read) != 3 {
		t.Fatalf("QueuedSearch = %v, %v", read, err)
	}
	unit, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unit.Rollback(ctx) }()
	if err := gen.New(unit).QueueSearch(ctx, gen.QueueSearchParams{Ids: []uuid.UUID{ids[1]}, NotebookID: f.eng}); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	if err := f.s.DequeueSearch(bounded, read); err != nil {
		t.Fatalf("DequeueSearch beside a held row = %v after %s; want it to skip the row at once", err, time.Since(started))
	}
	if got := f.queued(t); len(got) != 1 || got[ids[1]].version != read[1].Version {
		t.Errorf("the queue beside the held row = %+v; want the held node alone, as it was read", got)
	}
	if err := unit.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DequeueSearch(ctx, read); err != nil {
		t.Fatal(err)
	}
	next, err := f.s.QueuedSearch(ctx, false, uuid.Nil(), 10)
	if err != nil || len(next) != 1 || next[0].ID != ids[1] || next[0].Version <= read[1].Version {
		t.Fatalf("the queue after the unit = %v, %v; want the held node at its new version", next, err)
	}
	if err := f.s.DequeueSearch(ctx, next); err != nil {
		t.Fatal(err)
	}
	if got := f.queued(t); len(got) != 0 {
		t.Errorf("the queue after the next run = %+v, want it empty", got)
	}
}

// The job's lock is one run's at a time, whichever connection or process
// asks; an attempt refused gives its connection back; released, the lock
// goes to the next run (M8/P5 design 3.4).
func TestLockSearchIsOneRunsAtATime(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	release, ok, err := f.s.LockSearch(ctx)
	if err != nil || !ok {
		t.Fatalf("LockSearch = %v, %v; want the lock", ok, err)
	}
	if _, ok, err := f.s.LockSearch(ctx); err != nil || ok {
		t.Errorf("LockSearch while a run holds it = %v, %v; want no lock", ok, err)
	}
	if held := f.pool.Stat().AcquiredConns(); held != 1 {
		t.Errorf("%d connections held, want the run's alone: a refused attempt gives its connection back", held)
	}
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	other := postgresadapter.New(pool)
	if _, ok, err := other.LockSearch(ctx); err != nil || ok {
		t.Errorf("another process's LockSearch while a run holds it = %v, %v; want no lock", ok, err)
	}
	release()
	if held := f.pool.Stat().AcquiredConns(); held != 0 {
		t.Errorf("%d connections held after the release, want none", held)
	}
	next, ok, err := other.LockSearch(ctx)
	if err != nil || !ok {
		t.Fatalf("another process's LockSearch after the release = %v, %v; want the lock", ok, err)
	}
	if _, ok, err := f.s.LockSearch(ctx); err != nil || ok {
		t.Errorf("LockSearch while the other process holds it = %v, %v; want no lock", ok, err)
	}
	next()
}

// The job's rows are its lexemes as tsvectors, a page's body with the
// revision it read, none for an attachment's, the name, tokenizer and
// time; a node's row written again is replaced, and a deleted node's goes,
// in the transaction the job opened, which its settings do not outlive.
// Its sources are each node as it is, gone when deleted or not there, a
// page's content's revision and size, and its row as built; its contents
// those of the live pages alone. Its ANALYZE gives @@ its statistics.
func TestTheSearchsRowsAndSources(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	content := "数据库 api"
	p := f.page(t, f.eng, nil, "Page", 0)
	f.write(t, p.ID, content, 3, now())
	x := f.asset(t, f.eng, nil, "x.png", 1)
	d := f.page(t, f.eng, nil, "Gone", 2)
	missing := uuid.NewV7()
	built := now().Add(time.Minute)
	row := func(n domain.Node, revision int, content *string) app.SearchRow {
		title, body := domain.SearchTokens(n.Name, content)
		return app.SearchRow{NodeID: n.ID, Revision: revision, Name: n.Name, Tokenizer: domain.SearchTokenizer, BuiltAt: built, Title: title,
			Body: body}
	}
	empty := ""
	rows := []app.SearchRow{row(p, 3, &content), row(x, 0, nil), row(d, 1, &empty)}
	if err := f.s.WriteSearchRows(ctx, rows, nil); err == nil {
		t.Errorf("WriteSearchRows outside a transaction wrote; want an error, its settings would stay on the pool's connection")
	}
	f.writeRows(t, rows)
	for _, r := range rows {
		var revision *int // an attachment's none
		if r.Revision > 0 {
			revision = &r.Revision
		}
		var same bool
		if err := f.pool.QueryRow(ctx, `SELECT revision IS NOT DISTINCT FROM $2 AND name = $3 AND tokenizer = $4 AND built_at = $5
				AND title = array_to_tsvector($6::text[]) AND body IS NOT DISTINCT FROM array_to_tsvector($7::text[])
			FROM page_search WHERE node_id = $1`, r.NodeID, revision, r.Name, r.Tokenizer, r.BuiltAt, r.Title, r.Body).Scan(&same); err != nil ||
			!same {
			t.Errorf("the row of %s = %v, %v; want it as written", r.Name, same, err)
		}
	}
	if err := f.s.DeleteNodes(ctx, deletion([]uuid.UUID{d.ID}, f.alice, now())); err != nil {
		t.Fatal(err)
	}
	sources, err := f.s.SearchSources(ctx, []uuid.UUID{missing, d.ID, x.ID, p.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]app.SearchSource{
		p.ID: {ID: p.ID, Kind: domain.KindPage, Name: "Page", Revision: 3, Bytes: len(content),
			Built: &app.BuiltSearchRow{Revision: 3, Name: "Page", Tokenizer: domain.SearchTokenizer, At: built}},
		x.ID: {ID: x.ID, Kind: domain.KindAsset, Name: "x.png",
			Built: &app.BuiltSearchRow{Name: "x.png", Tokenizer: domain.SearchTokenizer, At: built}},
		missing: {ID: missing, Gone: true},
	}
	if len(sources) != 4 || !slices.IsSortedFunc(sources, func(a, b app.SearchSource) int { return a.ID.Compare(b.ID) }) {
		t.Errorf("SearchSources = %+v; want the four nodes by id", sources)
	}
	for _, s := range sources {
		switch w, ok := want[s.ID]; {
		case s.ID == d.ID:
			if !s.Gone || s.Built == nil || s.Built.Name != "Gone" {
				t.Errorf("the deleted node's source = %+v; want it gone, with the row built", s)
			}
		case !ok || !sameSource(s, w):
			t.Errorf("the source of %v = %+v (built %+v); want %+v (built %+v)", s.ID, s, s.Built, w, w.Built)
		}
	}
	contents, err := f.s.SearchContents(ctx, []uuid.UUID{p.ID, x.ID, d.ID})
	if want := map[uuid.UUID]app.PageText{p.ID: {Content: content, Revision: 3}}; err != nil || len(contents) != 1 ||
		contents[p.ID] != want[p.ID] {
		t.Errorf("SearchContents = %+v, %v; want the live page's alone, %+v", contents, err, want)
	}
	renamed := p
	renamed.Name = "Renamed"
	f.writeRows(t, []app.SearchRow{row(renamed, 4, &content)}, d.ID)
	if got := f.count(t, `SELECT count(*) FROM page_search WHERE node_id = ANY($1)`, []uuid.UUID{p.ID, x.ID, d.ID}); got != 2 {
		t.Errorf("%d rows after the deleted node's went, want the page's and the attachment's", got)
	}
	if got := f.count(t, `SELECT revision FROM page_search WHERE node_id = $1 AND name = 'Renamed'`, p.ID); got != 4 {
		t.Errorf("the page's row written again is of revision %d, want 4", got)
	}
	if got := f.count(t, `SELECT count(*) FROM pg_stats WHERE tablename = 'page_search'`); got != 0 {
		t.Fatalf("page_search has statistics of %d columns before its ANALYZE", got)
	}
	if err := f.s.AnalyzeSearch(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, `SELECT count(*) FROM pg_stats WHERE tablename = 'page_search' AND attname IN ('title', 'body')`); got != 2 {
		t.Errorf("page_search's lexemes have statistics of %d columns after its ANALYZE, want 2", got)
	}
}

// sameSource reports whether two sources are alike, their times equal.
func sameSource(a, b app.SearchSource) bool {
	if a.Built == nil || b.Built == nil {
		return a == b
	}
	ab, bb := *a.Built, *b.Built
	a.Built, b.Built = nil, nil
	return a == b && ab.Revision == bb.Revision && ab.Name == bb.Name && ab.Tokenizer == bb.Tokenizer && ab.At.Equal(bb.At)
}

// searchScene is eng and ops with pages and attachments, each changed a
// minute after the one before it in the order created, their rows built
// by the job; then, the job not run since, a page of eng deleted (its row
// left), one created and queued exact, as a unit queues it, and one queued
// in bulk, as an import does, neither with a row.
func newSearchScene(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	at := now()
	add := func(notebook uuid.UUID, name, content string) domain.Node {
		var n domain.Node
		if content == "" && strings.HasSuffix(name, ".png") {
			n = f.asset(t, notebook, nil, name, 0)
		} else {
			n = f.page(t, notebook, nil, name, 0)
			f.write(t, n.ID, content, 2, at)
		}
		at = at.Add(time.Minute)
		f.changedAt(t, n.ID, at)
		return n
	}
	byNotebook := map[uuid.UUID][]uuid.UUID{}
	var deleted domain.Node
	for _, n := range []struct {
		notebook      uuid.UUID
		name, content string
	}{
		{f.eng, "Rapid notes", "A design doc."},
		{f.eng, "rapid.png", ""},
		{f.eng, "Plain", "the rapid API design"},
		{f.eng, "Go语言入门", "教程"},
		{f.eng, "gopher", "Let go of it"},
		{f.eng, "Django", "golang"},
		{f.eng, "铁锅", "做饭"},
		{f.eng, "饭", "一口锅"},
		{f.eng, "锅具", "中国制造 #1"},
		{f.eng, "中国锅", "x"},
		{f.eng, "中国", "锅"},
		{f.eng, "Deleted api", "rapid api"},
		{f.ops, "Ops api", "rapid"},
	} {
		node := add(n.notebook, n.name, n.content)
		byNotebook[n.notebook] = append(byNotebook[n.notebook], node.ID)
		if n.name == "Deleted api" {
			deleted = node
		}
	}
	for notebook, ids := range byNotebook {
		f.queueSearch(t, notebook, false, ids...)
	}
	f.drain(t)
	if err := f.s.DeleteNodes(ctx, deletion([]uuid.UUID{deleted.ID}, f.alice, at)); err != nil {
		t.Fatal(err)
	}
	f.queueSearch(t, f.eng, false, deleted.ID)
	f.queueSearch(t, f.eng, false, add(f.eng, "Fresh api", "rapid start, 中国 #2").ID)
	f.queueSearch(t, f.eng, true, add(f.eng, "Bulk api", "rapid").ID)
	if rows := f.count(t, `SELECT count(*) FROM page_search s JOIN nodes n ON n.id = s.node_id WHERE n.name = 'Deleted api'`); rows != 1 {
		t.Fatalf("the deleted page has %d rows, want its row left", rows)
	}
	return f
}

// Each tier finds what its terms say (M8/P5 design 3.3, 3.5): the title
// tier the nodes, pages and attachments, whose names hold every term; the
// body tier the pages whose names miss a term, each term in the name or
// the content, newest first. A term of three letters or more is a
// substring, one of one or two a whole word, a single CJK character is
// found in names alone, and the body tier's candidates are by its titles'
// characters when it has no key of the body. Each finds a node a user's
// write queued, without a row, but none queued in bulk, which the search
// says is building; none deleted, though its row is there, nor of a
// notebook not searched.
func TestTheSearchsTiers(t *testing.T) {
	f := newSearchScene(t)
	for _, tt := range []struct {
		q              string
		notebooks      []uuid.UUID
		titles, bodies []string
	}{
		{"api", []uuid.UUID{f.eng}, []string{"Fresh api", "rapid.png", "Rapid notes"}, []string{"Plain"}},
		{"API", []uuid.UUID{f.eng, f.ops}, []string{"Fresh api", "Ops api", "rapid.png", "Rapid notes"}, []string{"Plain"}},
		{"rapid", []uuid.UUID{f.eng}, []string{"rapid.png", "Rapid notes"}, []string{"Fresh api", "Plain"}},
		{"rapid", []uuid.UUID{f.ops}, []string{}, []string{"Ops api"}},
		{"rapid design", []uuid.UUID{f.eng}, []string{}, []string{"Plain", "Rapid notes"}},
		{"go", []uuid.UUID{f.eng}, []string{"Go语言入门"}, []string{"gopher"}},
		{"锅", []uuid.UUID{f.eng}, []string{"中国锅", "锅具", "铁锅"}, []string{}},
		{"锅 中国", []uuid.UUID{f.eng}, []string{"中国锅"}, []string{"锅具"}},
		{"锅 #", []uuid.UUID{f.eng}, []string{}, []string{"锅具"}},
		{"deleted", []uuid.UUID{f.eng}, []string{}, []string{}},
		{"bulk", []uuid.UUID{f.eng}, []string{}, []string{}},
	} {
		if got := f.titlesOf(t, tt.q, tt.notebooks...); !slices.Equal(got, tt.titles) {
			t.Errorf("the title tier of %q = %q, want %q", tt.q, got, tt.titles)
		}
		if got := f.bodiesOf(t, tt.q, tt.notebooks...); !slices.Equal(got, tt.bodies) {
			t.Errorf("the body tier of %q = %q, want %q", tt.q, got, tt.bodies)
		}
	}
	for _, tt := range []struct {
		notebooks []uuid.UUID
		want      bool
	}{{[]uuid.UUID{f.eng}, true}, {[]uuid.UUID{f.ops}, false}, {[]uuid.UUID{f.ops, f.eng}, true}} {
		got, err := inSnapshot(f, func(ctx context.Context) (bool, error) { return f.s.SearchBuilding(ctx, tt.notebooks) })
		if err != nil || got != tt.want {
			t.Errorf("SearchBuilding(%v) = %v, %v; want %v", tt.notebooks, got, err, tt.want)
		}
	}
}

// The body tier's candidates by their lexemes, counted (page_search_hits,
// M8/P5A): the rows whose body holds the body's keys; with titles-only
// terms too, those whose title also holds their characters; without keys
// of the body, those whose title holds them; none for keys no row holds.
// It counts rows, of any node and notebook: the statements that read the
// hits join the live nodes of the notebooks searched.
func TestSearchHitCount(t *testing.T) {
	f := newSearchScene(t)
	for _, tt := range []struct {
		q    string
		want int
	}{
		{"api", 4},     // Rapid notes, Plain, Deleted api and Ops api: Fresh api has no row yet
		{"锅 中国", 2},    // 锅具 and 中国锅, whose bodies hold 中国 and titles 锅; not 中国, whose title does not
		{"锅 #", 3},     // 铁锅, 锅具 and 中国锅, whose titles hold 锅
		{"deleted", 1}, // Deleted api's row, there until the job runs
		{"ops", 1},     // ops's Ops api
		{"zzzqqq", 0},
	} {
		got, err := inSnapshot(f, func(ctx context.Context) (int, error) { return f.s.SearchHitCount(ctx, scopeOf(t, tt.q, f.eng).Query) })
		if err != nil || got != tt.want {
			t.Errorf("SearchHitCount(%s) = %d, %v; want %d", tt.q, got, err, tt.want)
		}
	}
}

// The first page's count (M8/P5 design 3.5): the title tier's hits, their
// names checked, to a bound, and the pages among them; the body tier's
// candidates, its hits as counted and the queue, live pages of the
// notebooks searched, their names not checked, to a bound. As the tiers,
// they count a node a user's write queued, but none queued in bulk,
// deleted, or of a notebook not searched; the bodies' count no attachment,
// rapid.png queued again here as a rename would.
func TestSearchTitleAndBodyCounts(t *testing.T) {
	f := newSearchScene(t)
	var png uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM nodes WHERE name = 'rapid.png'`).Scan(&png); err != nil {
		t.Fatal(err)
	}
	f.queueSearch(t, f.eng, false, png)
	type counts struct{ titles, pages, bodies int }
	for _, tt := range []struct {
		q         string
		notebooks []uuid.UUID
		limit     int
		want      counts // pages -1: some of them, the titles counted to the limit
	}{
		// The titles Fresh api, rapid.png and Rapid notes; the bodies Rapid
		// notes and Plain, by their keys, and Fresh api, queued.
		{"api", []uuid.UUID{f.eng}, domain.MaxSearchCount + 1, counts{3, 2, 3}},
		{"api", []uuid.UUID{f.eng}, 2, counts{2, -1, 2}},
		// The titles Rapid notes and rapid.png; the bodies Rapid notes,
		// Plain, Ops api, and Fresh api.
		{"rapid", []uuid.UUID{f.eng, f.ops}, 10, counts{2, 1, 4}},
		// The title 中国锅; the bodies 锅具 and 中国锅, and Fresh api.
		{"锅 中国", []uuid.UUID{f.eng}, 10, counts{1, 1, 3}},
		// No title; the bodies whose titles hold 锅, 铁锅, 锅具 and 中国锅,
		// and Fresh api.
		{"锅 #", []uuid.UUID{f.eng}, 10, counts{0, 0, 4}},
	} {
		got, err := inSnapshot(f, func(ctx context.Context) (counts, error) {
			var c counts
			sc := scopeOf(t, tt.q, tt.notebooks...)
			var err error
			if c.titles, c.pages, err = f.s.SearchTitleCount(ctx, sc, tt.limit); err != nil {
				return c, err
			}
			if sc.Hits, err = f.s.SearchHitCount(ctx, sc.Query); err != nil {
				return c, err
			}
			c.bodies, err = f.s.SearchBodyCount(ctx, sc, tt.limit)
			return c, err
		})
		if tt.want.pages < 0 {
			got.pages = -1
		}
		if err != nil || got != tt.want {
			t.Errorf("the counts of %s in %v to %d = %+v, %v; want %+v", tt.q, tt.notebooks, tt.limit, got, err, tt.want)
		}
	}
}

// The body tier walks at most walk pages, newest first, each checked as
// it is now, and answers those that pass, at most limit; and the last it
// walked, passing or not, with how many it took: one more than walk when
// there are more to walk, so that a page ends where its walk did, not one
// before an empty page when the walk took the last (M8/P5 design 3.5).
// Below a cursor it walks on from there.
func TestTheBodyTiersWalk(t *testing.T) {
	f := newFixture(t)
	var pages []domain.Node
	for i, content := range []string{"abc bcd", "abcd", "abc bcd", "abcd", "abc bcd", "abcd"} {
		n := f.page(t, f.eng, nil, "W"+string(rune('1'+i)), 0)
		f.write(t, n.ID, content, 2, now())
		f.changedAt(t, n.ID, now().Add(-time.Duration(i)*time.Minute))
		pages = append(pages, n)
	}
	f.queueSearch(t, f.eng, false, func() []uuid.UUID {
		ids := make([]uuid.UUID, len(pages))
		for i, p := range pages {
			ids[i] = p.ID
		}
		return ids
	}()...)
	f.drain(t)
	type walked struct {
		name        string
		seq         int
		passed      bool
		walkedCount int
	}
	at := func(i int) domain.SearchCursor {
		return domain.SearchCursor{Tier: domain.SearchTierBody, At: now().Add(-time.Duration(i) * time.Minute), ID: pages[i].ID}
	}
	for _, tt := range []struct {
		name        string
		below       domain.SearchCursor
		walk, limit int
		want        []walked
	}{
		{"walking 3 of 6", domain.SearchFrom(domain.SearchTierBody), 3, 10,
			[]walked{{"W2", 2, true, 4}, {"W3", 3, false, 4}}},
		{"walking 4 of 6, the last passing", domain.SearchFrom(domain.SearchTierBody), 4, 10,
			[]walked{{"W2", 2, true, 5}, {"W4", 4, true, 5}}},
		{"walking all 6, exactly", domain.SearchFrom(domain.SearchTierBody), 6, 10,
			[]walked{{"W2", 2, true, 6}, {"W4", 4, true, 6}, {"W6", 6, true, 6}}},
		{"walking all 6, of 10", domain.SearchFrom(domain.SearchTierBody), 10, 10,
			[]walked{{"W2", 2, true, 6}, {"W4", 4, true, 6}, {"W6", 6, true, 6}}},
		{"up to one passing", domain.SearchFrom(domain.SearchTierBody), 10, 1,
			[]walked{{"W2", 2, true, 6}}},
		{"below W2, walking 2", at(1), 2, 10,
			[]walked{{"W4", 2, true, 3}}},
		{"below W4, walking 1", at(3), 1, 10,
			[]walked{{"W5", 1, false, 2}}},
	} {
		sc := scopeOf(t, "abcd", f.eng)
		sc.Below = tt.below
		rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
			return f.bodies(ctx, sc, tt.walk, tt.limit)
		})
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		got := make([]walked, len(rows))
		for i, r := range rows {
			got[i] = walked{r.Node.Name, r.Seq, r.Passed, r.Walked}
			if r.Passed != (r.At == 1 && r.Excerpt == "abcd") || !r.Passed && (r.At != 0 || r.Excerpt != "") {
				t.Errorf("%s: %s passed %v, its first hit at %d, excerpt %q; want one where it passed alone", tt.name, r.Node.Name, r.Passed,
					r.At, r.Excerpt)
			}
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
	// A count of the hits short of theirs leaves candidates out, which is
	// why the statements take it from their own snapshot: one short walks
	// five pages of the six, six short none.
	for _, short := range []int{1, 6} {
		rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
			sc := scopeOf(t, "abcd", f.eng)
			n, err := f.s.SearchHitCount(ctx, sc.Query)
			if err != nil || n != 6 {
				return nil, fmt.Errorf("SearchHitCount = %d, want the six: %w", n, err)
			}
			sc.Hits = n - short
			return f.s.SearchBodies(ctx, sc, 10, 10)
		})
		walked := 0
		if len(rows) > 0 {
			walked = rows[0].Walked
		}
		if err != nil || walked != 6-short {
			t.Errorf("a count %d short walked %d pages, %v; want %d", short, walked, err, 6-short)
		}
	}
}

// A page's row carries its content's first hit, the character from 1 of
// any term's first, with the excerpt the SQL cut around it, from 41
// characters before to 162 characters on, and the content's characters:
// in characters, not bytes. A page of the title tier whose content holds
// no hit, and an attachment, carry none (M8/P5 design 3.5).
func TestTheSearchsExcerpts(t *testing.T) {
	f := newFixture(t)
	before := strings.Repeat("数据é ", 15) // 60 characters of 1 to 3 bytes
	after := " " + strings.Repeat("内容x", 150)
	contents := map[string]string{
		"Far":           before + "target" + after + " zeta",
		"Near":          "ab target" + after,
		"Zeta first":    before + "zeta" + after + " target",
		"target page":   before + "target" + after,
		"target plain":  "nothing here",
		"target.png":    "",
		"Short":         "a target",
		"Other content": "无关",
	}
	var ids []uuid.UUID
	for name, content := range contents {
		var n domain.Node
		if name == "target.png" {
			n = f.asset(t, f.eng, nil, name, 0)
		} else {
			n = f.page(t, f.eng, nil, name, 0)
			f.write(t, n.ID, content, 2, now())
		}
		ids = append(ids, n.ID)
	}
	f.queueSearch(t, f.eng, false, ids...)
	f.drain(t)
	// cut is the excerpt and the characters of the content of name around
	// its character at, as the SQL cuts them.
	cut := func(name string, at int) (string, int) {
		runes := []rune(contents[name])
		if at == 0 {
			return "", len(runes)
		}
		from := max(at-41, 1) - 1
		return string(runes[from:min(from+162, len(runes))]), len(runes)
	}
	type excerpt struct {
		at      int
		excerpt string
		chars   int
	}
	want := func(name string, at int) excerpt {
		e, chars := cut(name, at)
		if name == "target.png" {
			chars = 0
		}
		return excerpt{at, e, chars}
	}
	got := map[string]excerpt{}
	for _, q := range []string{"target", "zeta target"} {
		sc := scopeOf(t, q, f.eng)
		titles, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchTitleRow, error) { return f.s.SearchTitles(ctx, sc, 50) })
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range titles {
			got[q+" "+r.Node.Name] = excerpt{r.At, r.Excerpt, r.Chars}
		}
		bodies, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
			return f.bodies(ctx, sc, domain.MaxSearchWalk, 50)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range bodies {
			if r.Passed {
				got[q+" "+r.Node.Name] = excerpt{r.At, r.Excerpt, r.Chars}
			}
		}
	}
	wants := map[string]excerpt{
		"target Far":             want("Far", 61),
		"target Near":            want("Near", 4),
		"target Zeta first":      want("Zeta first", 61+4+len([]rune(after))+1),
		"target Short":           want("Short", 3),
		"target target page":     want("target page", 61),
		"target target plain":    want("target plain", 0),
		"target target.png":      want("target.png", 0),
		"zeta target Far":        want("Far", 61),
		"zeta target Zeta first": want("Zeta first", 61),
	}
	for key, w := range wants {
		if g, ok := got[key]; !ok || g != w {
			t.Errorf("%s: at %d, excerpt %q of %d characters (found %v); want at %d, %q of %d", key, g.at, g.excerpt, g.chars, ok, w.at,
				w.excerpt, w.chars)
		}
	}
	for key := range got {
		if _, ok := wants[key]; !ok {
			t.Errorf("%s found; want it not", key)
		}
	}
}

// A page's content not in NFC, as keyboards that type Vietnamese's tones
// apart and Hangul's jamo write it, is read in NFC by the search, as the
// job's lexemes are made of it (M8/P5A review A-M4): the query's terms, in
// NFC, find it while it waits in the queue and once its row is built,
// each tier, its excerpt and its first hit's place those of its NFC; a
// term of its base letters alone finds it neither way. The content stays
// as written.
func TestTheSearchReadsContentNotInNFC(t *testing.T) {
	f := newFixture(t)
	contents := map[string]string{
		"Vietnamese":  norm.NFD.String("Tiếng Việt là ngôn ngữ của người Việt"),
		"Hangul":      norm.NFD.String("한국어 데이터베이스"),
		"Tiếng notes": norm.NFD.String("xin chào Tiếng Việt"),
	}
	var ids []uuid.UUID
	for name, content := range contents {
		if norm.NFC.IsNormalString(content) {
			t.Fatalf("the content of %s is in NFC", name)
		}
		n := f.page(t, f.eng, nil, name, 0)
		f.write(t, n.ID, content, 2, now())
		ids = append(ids, n.ID)
	}
	f.queueSearch(t, f.eng, false, ids...)
	for _, stage := range []string{"queued", "built"} {
		if stage == "built" {
			f.drain(t)
		}
		for _, tt := range []struct {
			q              string
			titles, bodies []string
		}{
			{"Việt", []string{}, []string{"Tiếng notes", "Vietnamese"}},
			{"ngữ", []string{}, []string{"Vietnamese"}},
			{"한국어", []string{}, []string{"Hangul"}},
			{"데이터", []string{}, []string{"Hangul"}},
			{"Tiếng", []string{"Tiếng notes"}, []string{"Vietnamese"}},
			// Base letters, which the content as written holds, its marks
			// apart, and its NFC does not.
			{"Vie", []string{"Vietnamese"}, []string{}},
			{"ngu", []string{"Hangul"}, []string{}},
		} {
			titles, bodies := f.titlesOf(t, tt.q, f.eng), f.bodiesOf(t, tt.q, f.eng)
			slices.Sort(bodies)
			if !slices.Equal(titles, tt.titles) || !slices.Equal(bodies, tt.bodies) {
				t.Errorf("%s: %q finds %q and %q, want %q and %q", stage, tt.q, titles, bodies, tt.titles, tt.bodies)
			}
		}
		// The excerpts and hits' places are of the content in NFC: the
		// title tier's, the body tier's.
		sc := scopeOf(t, "Tiếng", f.eng)
		titles, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchTitleRow, error) { return f.s.SearchTitles(ctx, sc, 50) })
		if err != nil || len(titles) != 1 {
			t.Fatalf("%s: SearchTitles(Tiếng) = %+v, %v", stage, titles, err)
		}
		if r, want := titles[0], norm.NFC.String(contents["Tiếng notes"]); r.At != 10 || r.Excerpt != want || r.Chars != len([]rune(want)) {
			t.Errorf("%s: the title tier's Tiếng notes at %d, %q of %d characters; want at 10, %q of %d", stage, r.At, r.Excerpt, r.Chars,
				want, len([]rune(want)))
		}
		walked, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
			return f.bodies(ctx, sc, domain.MaxSearchWalk, 50)
		})
		var bodies []app.SearchBodyRow
		for _, r := range walked {
			if r.Passed {
				bodies = append(bodies, r)
			}
		}
		if err != nil || len(bodies) != 1 {
			t.Fatalf("%s: SearchBodies(Tiếng) = %+v, %v; want Vietnamese", stage, walked, err)
		}
		if r, want := bodies[0], norm.NFC.String(contents["Vietnamese"]); r.At != 1 || r.Excerpt != want || r.Chars != len([]rune(want)) {
			t.Errorf("%s: the body tier's Vietnamese at %d, %q of %d characters; want at 1, %q of %d", stage, r.At, r.Excerpt, r.Chars,
				want, len([]rune(want)))
		} else if e := sc.Query.Excerpt(r.Excerpt, r.At, r.Chars); !slices.Equal(e.Hits, []domain.TextRange{{Start: 0, End: 5}}) {
			t.Errorf("%s: the excerpt's hits %v, want Tiếng's first five characters", stage, e.Hits)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM page_contents WHERE node_id = ANY($1) AND content IS NFC NORMALIZED`, ids); n != 0 {
		t.Errorf("%d contents written in NFC by the search, want them as written", n)
	}
}

// A term of single CJK characters, several of them (M8/P5 design 3.3;
// M8/P5A review A-M2): beside a term with keys of the body, it is found in
// a name or a content, which the body's keys find and the recheck checks;
// in a query without such keys, in names alone. Queued, then built.
func TestTheSearchFindsATermOfSingleCJKCharactersInContents(t *testing.T) {
	f := newFixture(t)
	var ids []uuid.UUID
	for _, n := range []struct{ name, content string }{
		{"Catalog", "这是中·国的数据库"},
		{"Katakana", "タ・ネの api"},
		{"中·国", "x"},
		{"Others", "中国的数据库, タネ api"},
	} {
		node := f.page(t, f.eng, nil, n.name, 0)
		f.write(t, node.ID, n.content, 2, now())
		ids = append(ids, node.ID)
	}
	f.queueSearch(t, f.eng, false, ids...)
	for _, stage := range []string{"queued", "built"} {
		if stage == "built" {
			f.drain(t)
		}
		for _, tt := range []struct {
			q              string
			titles, bodies []string
		}{
			{"中·国 数据库", []string{}, []string{"Catalog"}},
			{"タ・ネ api", []string{}, []string{"Katakana"}},
			{"中·国", []string{"中·国"}, []string{}},
			{"锅 中·国", []string{}, []string{}},
		} {
			titles, bodies := f.titlesOf(t, tt.q, f.eng), f.bodiesOf(t, tt.q, f.eng)
			if !slices.Equal(titles, tt.titles) || !slices.Equal(bodies, tt.bodies) {
				t.Errorf("%s: %q finds %q and %q, want %q and %q", stage, tt.q, titles, bodies, tt.titles, tt.bodies)
			}
		}
	}
}

// A tier's nodes go newest first by when the node, or a page's content,
// last changed, by id below at one time; a page goes on below the last
// one's position (M8/P5 design 3.5).
func TestTheSearchsCursor(t *testing.T) {
	f := newFixture(t)
	at := func(minutes int) time.Time { return now().Add(time.Duration(minutes) * time.Minute) }
	nodes := map[string]domain.Node{}
	var ids []uuid.UUID
	for _, n := range []struct {
		name           string
		node, contents int // the minutes after now() the node and its content last changed
	}{
		{"Note 1", 1, 5},
		{"Note 2", 2, 2},
		{"Note 3", 3, 3},
		{"Note 4", 3, 3},
		{"note.png", 4, 0},
		{"Note 5", 6, 0},
	} {
		var node domain.Node
		if n.name == "note.png" {
			node = f.asset(t, f.eng, nil, n.name, 0)
		} else {
			node = f.page(t, f.eng, nil, n.name, 0)
			f.write(t, node.ID, "memo", 2, at(n.contents))
		}
		f.exec(t, `UPDATE nodes SET updated_at = $2 WHERE id = $1`, node.ID, at(n.node))
		nodes[n.name] = node
		ids = append(ids, node.ID)
	}
	f.queueSearch(t, f.eng, false, ids...)
	f.drain(t)
	third, fourth := "Note 4", "Note 3"
	if nodes["Note 3"].ID.Compare(nodes["Note 4"].ID) > 0 {
		third, fourth = fourth, third
	}
	for _, tt := range []struct {
		q, tier string
		want    []string
	}{
		{"note", "title", []string{"Note 5", "Note 1", "note.png", third, fourth, "Note 2"}},
		{"memo", "body", []string{"Note 5", "Note 1", third, fourth, "Note 2"}},
	} {
		sc := scopeOf(t, tt.q, f.eng)
		var got []string
		for range 4 {
			var page []app.SearchTitleRow
			if tt.tier == "title" {
				rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchTitleRow, error) { return f.s.SearchTitles(ctx, sc, 2) })
				if err != nil {
					t.Fatal(err)
				}
				page = rows
			} else {
				rows, err := inSnapshot(f, func(ctx context.Context) ([]app.SearchBodyRow, error) {
					return f.bodies(ctx, sc, domain.MaxSearchWalk, 2)
				})
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range rows {
					page = append(page, r.SearchTitleRow)
				}
			}
			if len(page) == 0 {
				break
			}
			for _, r := range page {
				got = append(got, r.Node.Name)
			}
			last := page[len(page)-1]
			sc.Below = domain.SearchCursor{At: last.SortAt, ID: last.Node.ID}
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("the %s tier of %q page by page = %q, want %q", tt.tier, tt.q, got, tt.want)
		}
	}
}

// A term's pattern answers in PostgreSQL as in Go (M8/P5 design 3.3,
// 3.10): whether it matches a text under ~, and where its group starts
// under regexp_instr, its character from 1, the first hit Go's highlights
// take. The texts and terms hold each character of an orbit of simple
// folding of more than one member, the ones whose full folding differs,
// CJK, the marks of other scripts, digits, a character beyond the BMP and
// the escapes' characters; a term is a piece of its text, some of its
// characters another of their orbits, or random. Fixed seeds; the cases
// go to the server in batches, a pattern's in a row, which its cache of
// compiled expressions keeps.
func TestThePatternsMatchInPostgreSQLAsInGo(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	var folded []rune // every character of an orbit of more than one member
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) != r {
			folded = append(folded, r)
		}
	}
	if len(folded) < 2800 {
		t.Fatalf("%d characters with case, want the some 2,900 of Unicode", len(folded))
	}
	specials := []rune("ßẞİıſKµμΜǅǄǆςσΣﬃiIsSkKΩω")
	others := []rune("abcxyzABCXYZ0123456789中国数据库ーくア한กन्दी٣Ａⅷⓐ")
	others = append(others, 0x0E31, 0x093F, 0x0940, 0x0301, 0x0345, 0x1F600, 0x10400, 0x1E9E)
	parts := []rune(" \n.#+()[]\\%_-*?^$|{}!'！")
	parts = append(parts, 0x3000)
	pick := func(r *rand.Rand) rune {
		switch n := r.IntN(100); {
		case n < 30:
			return folded[r.IntN(len(folded))]
		case n < 45:
			return specials[r.IntN(len(specials))]
		case n < 75:
			return others[r.IntN(len(others))]
		default:
			return parts[r.IntN(len(parts))]
		}
	}
	text := func(r *rand.Rand, n int) []rune {
		out := make([]rune, n)
		for i := range out {
			out[i] = pick(r)
		}
		return out
	}
	// refold is c as another of its orbit, or itself.
	refold := func(r *rand.Rand, c rune) rune {
		for range r.IntN(4) {
			c = unicode.SimpleFold(c)
		}
		return c
	}
	type matchCase struct {
		text, term string
		pattern    string
		match      bool
		at         int // the character from 1 the group starts at, 0 for none
	}
	var cases []matchCase
	terms := 0
	add := func(text []rune, query string) {
		q := "zz " + query
		parsed, err := domain.ParseSearchQuery(&q)
		if err != nil {
			return
		}
		s := string(text)
		for _, term := range parsed.Terms {
			if term.Text == "zz" {
				continue
			}
			terms++
			hits := domain.SearchQuery{Terms: []domain.SearchTerm{term}}.TitleHits(s)
			c := matchCase{text: s, term: term.Text, pattern: term.Pattern(), match: len(hits) > 0}
			if c.match {
				c.at = characterAt(s, hits[0].Start) + 1
			}
			cases = append(cases, c)
		}
	}
	r := rand.New(rand.NewPCG(20261015, 1))
	// Each character of an orbit as a term, in a text of the next of its
	// orbit between random characters.
	for _, c := range folded {
		add(slices.Concat(text(r, r.IntN(3)), []rune{unicode.SimpleFold(c)}, text(r, r.IntN(3))), string(c))
	}
	// Random texts, each with terms that are pieces of it, some of their
	// characters refolded, and random terms.
	for range 1500 {
		s := text(r, 1+r.IntN(30))
		for range 3 {
			if r.IntN(3) == 0 {
				add(s, string(text(r, 1+r.IntN(6))))
				continue
			}
			from := r.IntN(len(s))
			piece := slices.Clone(s[from : from+1+r.IntN(min(8, len(s)-from))])
			for i := range piece {
				if r.IntN(2) == 0 {
					piece[i] = refold(r, piece[i])
				}
			}
			add(s, string(piece))
		}
	}
	matched := 0
	for _, c := range cases {
		if c.match {
			matched++
		}
	}
	if matched < len(cases)/4 || len(cases)-matched < len(cases)/10 {
		t.Fatalf("%d of %d cases match: want both kinds, for the property to say anything", matched, len(cases))
	}
	slices.SortStableFunc(cases, func(a, b matchCase) int { return strings.Compare(a.pattern, b.pattern) })
	const batch = 5000
	failures := 0
	for from := 0; from < len(cases); from += batch {
		part := cases[from:min(from+batch, len(cases))]
		texts, patterns := make([]string, len(part)), make([]string, len(part))
		for i, c := range part {
			texts[i], patterns[i] = c.text, c.pattern
		}
		rows, err := f.pool.Query(ctx, `SELECT c.t ~ c.p, regexp_instr(c.t, c.p, 1, 1, 0, 'c', 1)
			FROM unnest($1::text[], $2::text[]) WITH ORDINALITY AS c(t, p, i) ORDER BY c.i`, texts, patterns)
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		for rows.Next() {
			var match bool
			var at int
			if err := rows.Scan(&match, &at); err != nil {
				t.Fatal(err)
			}
			if c := part[i]; match != c.match || at != c.at {
				failures++
				if failures <= 20 {
					t.Errorf("%q (%s) in %q: PostgreSQL %v at %d, Go %v at %d", c.term, c.pattern, c.text, match, at, c.match, c.at)
				}
			}
			i++
		}
		if err := rows.Err(); err != nil || i != len(part) {
			t.Fatalf("%d rows of %d, %v", i, len(part), err)
		}
	}
	if failures > 0 {
		t.Errorf("%d of %d cases differ", failures, len(cases))
	}
	t.Logf("%d cases of %d terms, %d matching", len(cases), terms, matched)
}

// characterAt is the character of s at its UTF-16 code unit units, from 0.
func characterAt(s string, units int) int {
	n, k := 0, 0
	for _, r := range s {
		if n >= units {
			return k
		}
		n += utf16.RuneLen(r)
		k++
	}
	return k
}

// A search statement's error carries its SQLSTATE alone (M8/P5 design
// 3.6): PostgreSQL's message and detail, and pgx's own errors, would bring
// the patterns, the keys, or a row's name and lexemes, the user's text,
// into the logs. Each read of a limit fails as its negative limit is
// refused; each read in a transaction a statement failed in; the job's
// write of a row its check refuses, the row in the error's detail; and a
// read whose context ended keeps that end, for its caller to tell apart.
func TestSearchStatementErrorsCarryNoQuery(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	const secret = "Geheimnis"
	sc := scopeOf(t, secret+" 机密文件 ab", f.eng)
	leaks := slices.Concat([]string{secret, "机密", "geh", "nis", "ab", "SELECT", "page_search", "LIMIT", "ERROR"}, sc.Query.Patterns())
	check := func(name string, err error, state string) {
		t.Helper()
		var pgErr *pgconn.PgError
		switch {
		case err == nil:
			t.Errorf("%s did not fail", name)
			return
		case errors.As(err, &pgErr):
			t.Errorf("%s = %q, which holds PostgreSQL's error: its message %q, its detail %q", name, err, pgErr.Message, pgErr.Detail)
		case !strings.HasSuffix(err.Error(), ": SQLSTATE "+state):
			t.Errorf("%s = %q, want its SQLSTATE %s alone", name, err, state)
		}
		for _, l := range leaks {
			if strings.Contains(err.Error(), l) {
				t.Errorf("%s = %q, which holds %q", name, err, l)
			}
		}
	}
	reads := map[string]func(ctx context.Context, limit int) error{
		"SearchTitles": func(ctx context.Context, limit int) error {
			_, err := f.s.SearchTitles(ctx, sc, limit)
			return err
		},
		"SearchHitCount": func(ctx context.Context, _ int) error {
			_, err := f.s.SearchHitCount(ctx, sc.Query)
			return err
		},
		"SearchBodies": func(ctx context.Context, limit int) error {
			_, err := f.s.SearchBodies(ctx, sc, domain.MaxSearchWalk, limit)
			return err
		},
		"SearchTitleCount": func(ctx context.Context, limit int) error {
			_, _, err := f.s.SearchTitleCount(ctx, sc, limit)
			return err
		},
		"SearchBodyCount": func(ctx context.Context, limit int) error {
			_, err := f.s.SearchBodyCount(ctx, sc, limit)
			return err
		},
		"SearchBuilding": func(ctx context.Context, _ int) error {
			_, err := f.s.SearchBuilding(ctx, sc.Notebooks)
			return err
		},
	}
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	rollBack := errors.New("roll back")
	for name, read := range reads {
		if name != "SearchBuilding" && name != "SearchHitCount" {
			check(name+" of a negative limit", tx.WithinSnapshot(ctx, func(ctx context.Context) error { return read(ctx, -1) }), "2201W")
		}
	}
	_ = tx.WithinSnapshot(ctx, func(ctx context.Context) error {
		if err := reads["SearchTitles"](ctx, -1); err == nil {
			t.Fatal("SearchTitles of a negative limit did not fail")
		}
		for name, read := range reads {
			check(name+" in a failed transaction", read(ctx, 10), "25P02")
		}
		return rollBack
	})
	n := f.page(t, f.eng, nil, secret, 0)
	title, body := domain.SearchTokens(secret, new(secret+" 机密文件"))
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		return f.s.WriteSearchRows(ctx, []app.SearchRow{{NodeID: n.ID, Name: secret, Tokenizer: domain.SearchTokenizer, BuiltAt: now(),
			Title: title, Body: body}}, nil)
	})
	check("WriteSearchRows of a body without its revision", err, "23514")
	_ = tx.WithinSnapshot(ctx, func(ctx context.Context) error {
		ended, cancel := context.WithCancel(ctx)
		cancel()
		for name, read := range reads {
			var pgErr *pgconn.PgError
			if err := read(ended, 10); !errors.Is(err, context.Canceled) || errors.As(err, &pgErr) || strings.Contains(err.Error(), secret) {
				t.Errorf("%s of an ended context = %v; want its end alone", name, err)
			}
		}
		return rollBack
	})
}
