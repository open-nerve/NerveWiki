package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// A search reads nothing of the notebooks it does not search (M8 Codex
// review R1): ops, another notebook, holds 3,000 pages of one word, in
// their names and contents, then of another, then of the first again
// (Codex's A→B→A), and in their names a character found in names alone with
// each word, 锅, then 囧, then 锅; eng, which the search reads alone, 300
// pages of the corpus and neither word, 30 of them 锅 in their names. Each
// of the search's reads in eng fetches the same blocks of each of the
// search's tables, their indexes and TOAST, and its GIN indexes' scans
// return the same rows, whichever ops holds: for either word; for 锅 with
// the first, and with lorem, which every page of ops holds; and for 锅
// alone: page_search_hits's three branches, body keys alone, titled keys
// besides them, titled keys alone (fix round 2, A-2). No posting list it
// reads holds a row of ops, and what a notebook the reader does not see,
// or does not search, holds changes neither what a search reads nor, so,
// how long it takes. Blocks and rows, not time: the counts are exact where
// a time is noise, and the test runs in the suite. But for the GIN
// indexes' entry trees, which every notebook's keys share: ops's writes may
// split a page beside eng's keys and add a step to a descent, one block
// (seen in the first phase, once in a few runs), where a posting list of
// ops's pages adds hundreds; the indexes are allowed 2 blocks, and no row.
// A read of ops's posting lists, or eng's keys behind ops's prefix, change
// the counts. Eng's titles hold 锅 so that the titled branch's bitmap of
// titles, read first, is not empty, and its bitmap of bodies is read too;
// the rows count it where ops's posting list of a word fits one page,
// fetched alike whether it holds 3,000 rows or none (fix round 3, C-1).
// (The indexes' pending lists, off since 00041, the migrations' test
// checks: one holds as many entries for either word.)
//
// Grep's reads too (M9/P4 design 6.1): the tree, the candidates by keys
// and by !cut alone, building, the pages' states and contents. Ops writes
// three pages alternately not cut and cut, each cut in the second phase
// (past the bound by different CJK bigrams, a long run, a long data:
// string), so that a read of !cut beyond eng's prefix reads their bitmaps
// (mutant 151); and its queue keeps nodes queued in bulk and exact while
// the reads are counted, which the queue's branch walks past: 1,000 of
// each in the first and the third phase and 1,500 in the second, so that a
// read of them, by a range of the queue's index or by its heap, fetches
// other blocks in the second phase (review B-1 = C-I1). The queue is
// vacuumed after each drain, so that the held rows fill its first pages
// whatever an autovacuum did, and its heap is some 20-30 pages, where the
// index wins: run 39638203738 failed on CI when an autovacuum between the
// drain and 200 held rows left a heap of 2 pages, which a sequential scan
// read whole. The B-trees of nodes and of the queue are held exact, every
// read's as main held M8's (review B-3 = C-m3): with the queue vacuumed and
// its rows held, no read here fetched another block of them in any phase
// (5 runs, 2 under race), and a step of a descent is a change of a
// B-tree's height, which bt_metap reads (pageinspect, created in the
// test's own database): each is the same in every phase.
func TestHiddenNotebooksLeaveTheSearchsReadsAlone(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	corpus := newSearchCorpus(20261016)
	eng := corpus.pages(t, f, f.eng, 300, 2000)
	ops := corpus.pages(t, f, f.ops, 3000, 200)
	f.exec(t, `UPDATE nodes SET name = '锅 ' || name, name_key = '锅 ' || name_key WHERE id = ANY($1)`, eng[:30])
	f.queueSearch(t, f.eng, false, eng...)
	f.queueSearch(t, f.ops, false, ops...)
	f.drain(t)
	words := []string{"quartzalpha", "quartzborta"}
	chars := map[string]string{words[0]: "锅", words[1]: "囧"}
	// cut are ops's three pages written cut in the second phase alone.
	var cut []uuid.UUID
	for i := range 3 {
		p := f.page(t, f.ops, nil, fmt.Sprintf("Cut %d", i), float64(10_000+i))
		cut = append(cut, p.ID)
	}
	cutRand, rev := rand.New(rand.NewPCG(20261021, 1)), 1
	// hold gives every page of ops the word, in its name and its content,
	// and the word's character in its name, its rows built again; its
	// three pages cut or not; then queues 1,000 of its pages in bulk and
	// 1,000 exact (1,500 and 1,500 with cuts, the second phase), left there
	// while the reads are counted.
	hold := func(word string, cuts bool) {
		t.Helper()
		for i, id := range cut {
			content := strings.Repeat("plain words of a page ", 40)
			if cuts {
				content = []string{"x\n" + randomCJK(cutRand, 240<<10/3), "x" + strings.Repeat("ab", 40),
					"![a](data:image/png;base64," + strings.Repeat("Qk", 50) + ")"}[i]
			}
			rev++
			f.write(t, id, content, rev, now())
		}
		f.queueSearch(t, f.ops, false, cut...)
		content := strings.Repeat("lorem ipsum "+word+" dolor ", 8)
		sum := sha256.Sum256([]byte(content))
		f.exec(t, `UPDATE page_contents SET content = $2, content_hash = $3, byte_size = octet_length($2), revision = revision + 1
			WHERE node_id = ANY($1)`, ops, content, sum[:])
		f.exec(t, `UPDATE nodes SET name = $3 || ' ' || $2 || ' ' || sort_order, name_key = $3 || ' ' || $2 || ' ' || sort_order
			WHERE id = ANY($1)`, ops, word, chars[word])
		f.queueSearch(t, f.ops, false, ops...)
		f.drain(t)
		f.exec(t, "VACUUM page_search_queue")
		if n := f.count(t, `SELECT count(*) FROM page_search WHERE node_id = ANY($1) AND partial`, cut); n != map[bool]int{true: 3}[cuts] {
			t.Fatalf("%d of ops's cut pages partial", n)
		}
		held := 1000
		if cuts {
			held = 1500
		}
		f.queueSearch(t, f.ops, true, ops[:held]...)
		f.queueSearch(t, f.ops, false, ops[held:2*held]...)
		f.exec(t, "VACUUM ANALYZE")
	}
	// blocks is the blocks read's transaction fetched of each of the
	// search's tables, their indexes and TOAST, hits and reads alike, and
	// the rows the GIN indexes' scans returned (":rows"), those it fetched
	// or returned any of: the backend's counts after the read less those
	// before, in one transaction, as a backend's counts not yet reported are
	// the last second's as well.
	const fetched = `SELECT r::regclass::text, pg_stat_get_xact_blocks_fetched(r) FROM unnest(ARRAY(
			SELECT c.oid FROM pg_class c WHERE c.relname IN ('page_search', 'page_search_queue', 'page_search_tags', 'nodes',
				'page_contents')
			UNION SELECT c.reltoastrelid FROM pg_class c WHERE c.relname IN ('page_search', 'page_contents')
			UNION SELECT i.indexrelid FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid
				WHERE c.relname IN ('page_search', 'page_search_queue', 'page_search_tags', 'nodes', 'page_contents'))) r
		UNION ALL SELECT c.relname || ':rows', pg_stat_get_xact_tuples_returned(c.oid) FROM pg_class c
			WHERE c.relname IN ('page_search_title_idx', 'page_search_body_idx')`
	counts := func(ctx context.Context) (map[string]int64, error) {
		rows, err := postgres.DB(ctx, f.pool).Query(ctx, fetched)
		if err != nil {
			return nil, err
		}
		out := map[string]int64{}
		for rows.Next() {
			var name string
			var n int64
			if err := rows.Scan(&name, &n); err != nil {
				return nil, err
			}
			out[name] = n
		}
		return out, rows.Err()
	}
	blocks := func(read func(ctx context.Context) error) map[string]int64 {
		t.Helper()
		var before, after map[string]int64
		err := postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(ctx, func(ctx context.Context) error {
			// Once first on the transaction's connection, uncounted: a
			// connection's first plan of a statement reads its indexes'
			// metapages into its cache, one block each, which another
			// connection of the pool has read before (M9/P4 S1).
			if err := read(ctx); err != nil {
				return err
			}
			var err error
			if before, err = counts(ctx); err != nil {
				return err
			}
			if err := read(ctx); err != nil {
				return err
			}
			after, err = counts(ctx)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int64{}
		for name, n := range after {
			if d := n - before[name]; d != 0 {
				out[name] = d
			}
		}
		return out
	}
	// same is whether two reads' blocks and rows are the same, but for a
	// step or two of the GIN indexes' entry trees.
	same := func(a, b map[string]int64) bool {
		names := maps.Clone(a)
		maps.Copy(names, b)
		for name := range names {
			d := a[name] - b[name]
			if name == "page_search_title_idx" || name == "page_search_body_idx" {
				if d < -2 || d > 2 {
					return false
				}
			} else if d != 0 {
				return false
			}
		}
		return true
	}
	format := func(m map[string]int64) string {
		var out []string
		for _, name := range slices.Sorted(maps.Keys(m)) {
			out = append(out, fmt.Sprintf("%s %d", name, m[name]))
		}
		return strings.Join(out, ", ")
	}
	reads := map[string]func(ctx context.Context, sc app.SearchScope) error{
		"SearchHitCount": func(ctx context.Context, sc app.SearchScope) error {
			_, err := f.s.SearchHitCount(ctx, sc)
			return err
		},
		"SearchTitles": func(ctx context.Context, sc app.SearchScope) error {
			_, err := f.s.SearchTitles(ctx, sc, 21)
			return err
		},
		"SearchBodies": func(ctx context.Context, sc app.SearchScope) error {
			sc.Below = domain.SearchFrom(domain.SearchTierBody)
			_, err := f.bodies(ctx, sc, 500, 21)
			return err
		},
		"SearchTitleCount": func(ctx context.Context, sc app.SearchScope) error {
			_, _, err := f.s.SearchTitleCount(ctx, sc, 1001)
			return err
		},
		"SearchBodyCount": func(ctx context.Context, sc app.SearchScope) error {
			n, err := f.s.SearchHitCount(ctx, sc)
			if err != nil {
				return err
			}
			sc.Hits = n
			_, err = f.s.SearchBodyCount(ctx, sc, 20000)
			return err
		},
		// grep's (M9/P4 design 3.4), keyed by the query's words.
		"ListNodes": func(ctx context.Context, _ app.SearchScope) error {
			_, err := f.s.ListNodes(ctx, f.eng)
			return err
		},
		"GrepCandidates": func(ctx context.Context, sc app.SearchScope) error {
			_, err := f.s.GrepCandidates(ctx, f.eng, sc.Query.BodyKeys())
			return err
		},
		"GrepCandidates !cut": func(ctx context.Context, _ app.SearchScope) error {
			_, err := f.s.GrepCandidates(ctx, f.eng, []string{domain.SearchCutKey})
			return err
		},
		"SearchBuilding": func(ctx context.Context, _ app.SearchScope) error {
			_, err := f.s.SearchBuilding(ctx, []uuid.UUID{f.eng})
			return err
		},
		"PageStates": func(ctx context.Context, _ app.SearchScope) error {
			_, err := f.s.PageStates(ctx, eng)
			return err
		},
		"GrepContents": func(ctx context.Context, _ app.SearchScope) error {
			_, err := f.s.GrepContents(ctx, eng, domain.GrepBatchBytes)
			return err
		},
	}
	// levels is the height of each B-tree of nodes and of the queue: its
	// root's level, 0 for a root that is a leaf.
	f.exec(t, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	levels := func() map[string]int {
		t.Helper()
		rows, err := f.pool.Query(ctx, `SELECT c.relname, (bt_metap(c.oid::regclass::text)).level FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid JOIN pg_am a ON a.oid = c.relam
			WHERE i.indrelid IN ('nodes'::regclass, 'page_search_queue'::regclass) AND a.amname = 'btree'`)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		for rows.Next() {
			var name string
			var level int64
			if err := rows.Scan(&name, &level); err != nil {
				t.Fatal(err)
			}
			out[name] = int(level)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	var sizes []map[string]int
	// got is the blocks of each read and word, a map a phase.
	var got []map[string]map[string]int64
	for phase, held := range []string{words[0], words[1], words[0]} {
		hold(held, phase == 1)
		sizes = append(sizes, levels())
		phase := map[string]map[string]int64{}
		// Every read once to warm the caches and settle the rows' hint
		// bits, then every read counted.
		for pass := range 2 {
			for name, read := range reads {
				for _, q := range []string{words[0], words[1], "锅 " + words[0], "锅 #", "锅 lorem"} {
					sc := scopeOf(t, q, f.eng)
					n := blocks(func(ctx context.Context) error { return read(ctx, sc) })
					if pass == 1 {
						phase[name+" "+q] = n
					}
				}
			}
		}
		for _, k := range slices.Sorted(maps.Keys(phase)) {
			t.Logf("ops holds %s: %s: %s", held, k, format(phase[k]))
		}
		got = append(got, phase)
	}
	t.Logf("the B-trees' heights in each phase: %v", sizes)
	if !maps.Equal(sizes[0], sizes[1]) || !maps.Equal(sizes[0], sizes[2]) {
		t.Errorf("the B-trees of nodes and the queue changed heights between the phases: %v", sizes)
	}
	for _, k := range slices.Sorted(maps.Keys(got[0])) {
		if !same(got[0][k], got[1][k]) || !same(got[0][k], got[2][k]) {
			t.Errorf("%s in eng fetches [%s], [%s] and [%s] as ops holds %s, %s and %s; want the same", k, format(got[0][k]),
				format(got[1][k]), format(got[2][k]), words[0], words[1], words[0])
		}
	}
}

// An instance that upgrades to 00041 (M8 Codex review R1), its rows of
// lexemes without prefixes, builds its search again: the rows go and the
// live nodes are queued in bulk, one a user wrote just before staying
// exact; until the job has built them, a search says the index is being
// built and finds that page alone, by the queue; after, it finds both, and
// says no more that it is being built.
func TestAnUpgradedInstanceBuildsItsSearchAgain(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "Alpha", 0)
	f.write(t, a.ID, "the kakapo calls", 2, now())
	b := f.page(t, f.eng, nil, "Beta", 1)
	f.write(t, b.ID, "the kakapo sleeps", 2, now())
	m, err := postgres.NewMigrator(f.pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	for {
		back, err := m.Down(ctx)
		if err != nil || back == nil {
			t.Fatalf("Down() = %+v, %v; want down to 00040", back, err)
		}
		if back.Version == 41 {
			break
		}
	}
	// The instance before: the rows as 00040's job wrote them, an edit of
	// Beta's queued exact.
	f.exec(t, `DELETE FROM page_search_queue`)
	for _, n := range []domain.Node{a, b} {
		content := "the kakapo calls"
		if n.ID == b.ID {
			content = "the kakapo sleeps"
		}
		title, body, _ := domain.SearchTokens(n.Name, &content)
		f.exec(t, `INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body)
			VALUES ($1, 2, $2, '2/17.0.0/17.0.0', now(), array_to_tsvector($3::text[]), array_to_tsvector($4::text[]))`,
			n.ID, n.Name, title, body)
	}
	f.queueSearch(t, f.eng, false, b.ID)
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}
	search := app.NewSearch(app.SearchDeps{Snapshots: postgres.NewTxManager(f.pool, 5*time.Second), Workspaces: searchWorkspace{uuid.NewV7()},
		Visible: searchVisible{f.eng}, Auth: searchAllowed{}, Reads: f.s, Trash: f.s, Attachments: noAttachments{}})
	actor := shared.WithActor(ctx, shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
	find := func() ([]string, bool) {
		t.Helper()
		q := "kakapo"
		page, err := search.Execute(actor, "acme", app.SearchRequest{Q: &q})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, h := range page.Hits {
			names = append(names, h.Node.Name)
		}
		slices.Sort(names)
		return names, page.Building
	}
	if rows := f.count(t, `SELECT count(*) FROM page_search`); rows != 0 {
		t.Errorf("%d rows after the upgrade, want none", rows)
	}
	if names, building := find(); !slices.Equal(names, []string{"Beta"}) || !building {
		t.Errorf("after the upgrade, the search finds %q, building %v; want Beta alone, by the queue, building", names, building)
	}
	f.drain(t)
	if names, building := find(); !slices.Equal(names, []string{"Alpha", "Beta"}) || building {
		t.Errorf("once the job ran, the search finds %q, building %v; want both, built", names, building)
	}
}

// A row at its bound fits PostgreSQL's tsvector behind the longest prefix
// a notebook can have (M8 Codex review R1, fix round 2: A-1): a page of
// random CJK text past MaxSearchLexemeBytes, queued in bulk in a notebook
// whose prefix is of SearchPrefixBytes, the longest page_search_prefix
// gives, is written by the job, its notebook no more building, and found by
// its name and its content. Counted without the prefixes, its lexemes made
// a tsvector of 1.3 MB, which PostgreSQL refused at every run: the page was
// never found, and its notebook said it was being built for good.
func TestARowAtItsBoundFitsBehindTheLongestPrefix(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	var longest int
	if err := f.pool.QueryRow(ctx, `SELECT length(page_search_prefix(9223372036854775807))`).Scan(&longest); err != nil ||
		longest != domain.SearchPrefixBytes {
		t.Fatalf("the longest prefix is of %d bytes, %v; want SearchPrefixBytes, %d", longest, err, domain.SearchPrefixBytes)
	}
	// eng's prefix is the sequence's next, the first of seven bytes.
	f.exec(t, `SELECT setval('page_search_tags_seq', 56800235583, false)`)
	r := rand.New(rand.NewPCG(1, 2))
	var b strings.Builder
	for b.Len() < 4*domain.MaxSearchLexemeBytes/3 {
		b.WriteRune(rune(0x4E00 + r.IntN(0x5000)))
	}
	content := b.String()
	_, body, _ := domain.SearchTokens("Big", &content)
	sum := 0
	for _, x := range body {
		sum += len(x) + domain.SearchPrefixBytes
	}
	if sum < domain.MaxSearchLexemeBytes-6-domain.SearchPrefixBytes {
		t.Fatalf("the body's lexemes hold %d bytes with their prefixes, want the bound's, %d", sum, domain.MaxSearchLexemeBytes)
	}
	p := f.page(t, f.eng, nil, "Big", 0)
	f.write(t, p.ID, content, 2, now())
	f.queueSearch(t, f.eng, true, p.ID)
	f.drain(t)
	if prefix := f.count(t, `SELECT length(prefix) FROM page_search_tags WHERE notebook_id = $1`, f.eng); prefix != domain.SearchPrefixBytes {
		t.Fatalf("eng's prefix is of %d bytes, want %d", prefix, domain.SearchPrefixBytes)
	}
	if n := f.count(t, `SELECT length(body) FROM page_search WHERE node_id = $1`, p.ID); n != len(body) {
		t.Errorf("the row holds %d lexemes, want %d", n, len(body))
	}
	building, err := inSnapshot(f, func(ctx context.Context) (bool, error) { return f.s.SearchBuilding(ctx, []uuid.UUID{f.eng}) })
	if err != nil || building {
		t.Errorf("eng building = %v, %v; want its rows built", building, err)
	}
	if got := f.titlesOf(t, "Big", f.eng); !slices.Equal(got, []string{"Big"}) {
		t.Errorf("the title tier finds %q, want Big", got)
	}
	if got := f.bodiesOf(t, body[1], f.eng); !slices.Equal(got, []string{"Big"}) {
		t.Errorf("the body tier finds %q for %q, want Big", got, body[1])
	}
}
