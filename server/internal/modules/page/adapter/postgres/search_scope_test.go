package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
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
// (Codex's A→B→A); eng, which the
// search reads alone, 300 pages of the corpus and neither word. Each of
// the search's reads in eng, for either word, fetches the same blocks of
// each of the search's tables, their indexes and TOAST whichever word ops
// holds: no posting list it reads holds a row of ops, and what a notebook
// the reader does not see, or does not search, holds changes neither what
// a search reads nor, so, how long it takes. Blocks, not time: the count
// is exact where a time is noise, and the test runs in the suite. But for
// the GIN indexes' entry trees, which every notebook's keys share: ops's
// writes may split a page beside eng's keys and add a step to a descent,
// one block (seen in the first phase, once in a few runs), where a posting
// list of ops's pages adds hundreds; the indexes are allowed 2. A read of
// ops's posting lists, or eng's keys behind ops's prefix, change the
// counts. (The indexes' pending lists, off since 00041, the migrations'
// test checks: one holds as many entries for either word.)
func TestHiddenNotebooksLeaveTheSearchsReadsAlone(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	corpus := newSearchCorpus(20261016)
	eng := corpus.pages(t, f, f.eng, 300, 2000)
	ops := corpus.pages(t, f, f.ops, 3000, 200)
	f.queueSearch(t, f.eng, false, eng...)
	f.queueSearch(t, f.ops, false, ops...)
	f.drain(t)
	words := []string{"quartzalpha", "quartzborta"}
	// hold gives every page of ops the word, in its name and its content,
	// its rows built again.
	hold := func(word string) {
		t.Helper()
		content := strings.Repeat("lorem ipsum "+word+" dolor ", 8)
		sum := sha256.Sum256([]byte(content))
		f.exec(t, `UPDATE page_contents SET content = $2, content_hash = $3, byte_size = octet_length($2), revision = revision + 1
			WHERE node_id = ANY($1)`, ops, content, sum[:])
		f.exec(t, `UPDATE nodes SET name = $2 || ' ' || sort_order, name_key = $2 || ' ' || sort_order WHERE id = ANY($1)`, ops, word)
		f.queueSearch(t, f.ops, false, ops...)
		f.drain(t)
		f.exec(t, "VACUUM ANALYZE")
	}
	// blocks is the blocks read's transaction fetched of each of the
	// search's tables, their indexes and TOAST, hits and reads alike, those
	// it fetched any of: the backend's counts after the read less those
	// before, in one transaction, as a backend's counts not yet reported are
	// the last second's as well.
	const fetched = `SELECT r::regclass::text, pg_stat_get_xact_blocks_fetched(r) FROM unnest(ARRAY(
			SELECT c.oid FROM pg_class c WHERE c.relname IN ('page_search', 'page_search_queue', 'page_search_tags', 'nodes',
				'page_contents')
			UNION SELECT c.reltoastrelid FROM pg_class c WHERE c.relname IN ('page_search', 'page_contents')
			UNION SELECT i.indexrelid FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid
				WHERE c.relname IN ('page_search', 'page_search_queue', 'page_search_tags', 'nodes', 'page_contents'))) r`
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
	// same is whether two reads' blocks are the same, but for a step or two
	// of the GIN indexes' entry trees.
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
	}
	// got is the blocks of each read and word, a map a phase.
	var got []map[string]map[string]int64
	for _, held := range []string{words[0], words[1], words[0]} {
		hold(held)
		phase := map[string]map[string]int64{}
		// Every read once to warm the caches and settle the rows' hint
		// bits, then every read counted.
		for pass := range 2 {
			for name, read := range reads {
				for _, q := range words {
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
	if back, err := m.Down(ctx); err != nil || back == nil || back.Version != 41 {
		t.Fatalf("Down() = %+v, %v; want 00041 undone", back, err)
	}
	// The instance before: the rows as 00040's job wrote them, an edit of
	// Beta's queued exact.
	f.exec(t, `DELETE FROM page_search_queue`)
	for _, n := range []domain.Node{a, b} {
		content := "the kakapo calls"
		if n.ID == b.ID {
			content = "the kakapo sleeps"
		}
		title, body := domain.SearchTokens(n.Name, &content)
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
