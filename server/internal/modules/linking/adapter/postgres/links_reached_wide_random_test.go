//go:build !race

package postgresadapter

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The three reads of a reach's links answer the same rows in the same
// order for any reach (v0.1 design 13.4 item 13; M12/P4/S7 design 4.1 item
// 2): LinksReached, which Store.Links reads below 4,000 keys and targets,
// and LinksReachedWide and LinksReachedWideGeneric, which it reads from
// there, each read directly here, whatever the size. The fixture is
// random: two notebooks of pages whose links' keys differ in case, by ß,
// in length, are empty or none; that resolve to a page of either notebook,
// to none, to the zero id or to an id no page has; to a page or an
// attachment, ambiguous or not, a value of their page's aliases or not.
// Each reach's keys, targets and sources are drawn from the fixture with
// misses and NULL elements, or are NULL arrays; their sizes are spread
// logarithmically below the threshold, so most are tens to hundreds, and a
// fixed number past it (the reads production makes there: parallel plans,
// BitmapOr), whose time is about 0.2 s a group of three (review A, round
// 2, N2). Another seed's fixture is read without statistics and analyzed,
// below and past the threshold, and under force_generic_plan with reaches
// of at most 1,000 elements: LinksReached and LinksReachedWide compare each
// row with a parameter array there, which is quadratic (design 1.3 item 4).
func TestLinksReachedWideIsLinksReached(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL comparison of the three reads of a reach's links")
	}
	const (
		seedA, seedB  = 0x5137_0001, 0x5137_0002
		smallA, bigA  = 2000, 80 // 3,000 below took 45 s of its 69 here: past the design's 60 s, fewer (design 4.1 item 2)
		smallB, bigB  = 300, 20
		genericGroups = 300
	)
	t.Run("analyzed", func(t *testing.T) {
		f := newWideRandomFixture(t, seedA)
		f.analyze(t)
		f.compare(t, "auto", seedA, smallA, 0, 3999, false)
		f.compare(t, "auto", seedA+1, bigA, 4000, 10000, false)
	})
	t.Run("another_seed", func(t *testing.T) {
		f := newWideRandomFixture(t, seedB)
		f.compare(t, "no_statistics", seedB, smallB, 0, 3999, false)
		f.compare(t, "no_statistics", seedB+1, bigB, 4000, 10000, false)
		f.analyze(t)
		f.compare(t, "analyzed", seedB+2, smallB, 0, 3999, false)
		f.compare(t, "analyzed", seedB+3, bigB, 4000, 10000, false)
		f.compare(t, "force_generic_plan", seedB+4, genericGroups, 0, 1000, true)
	})
}

type wideRandomFixture struct {
	pool  *pgxpool.Pool
	conn  *pgxpool.Conn
	books [2]uuid.UUID
	pages [2][]uuid.UUID
	keys  []string
	links int
}

// wideRandomKeys is n keys as the index holds them: cased and folded
// alike, by ß and ss, short and long, and the empty key.
func wideRandomKeys(r *rand.Rand, n int) []string {
	keys := []string{"", "ß", "ss", "SS", "straße", "strasse", "STRASSE", "Straße", "ẞ", "a", "A", "é", "é"}
	for len(keys) < n {
		var b strings.Builder
		switch r.IntN(6) {
		case 0:
			b.WriteString(strings.Repeat("ß", 1+r.IntN(4)))
		case 1:
			b.WriteString(strings.Repeat("x", 100+r.IntN(200)))
		case 2:
			b.WriteString("Page ")
		}
		fmt.Fprintf(&b, "p%d", len(keys))
		if r.IntN(3) == 0 {
			keys = append(keys, strings.ToUpper(b.String()))
		}
		keys = append(keys, b.String())
	}
	return keys[:n]
}

func newWideRandomFixture(t *testing.T, seed uint64) *wideRandomFixture {
	t.Helper()
	pool, _ := newLinksCollectorPool(t)
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	f := &wideRandomFixture{pool: pool, books: [2]uuid.UUID{randomID(r), randomID(r)}, keys: wideRandomKeys(r, 5000)}
	if _, err := pool.Exec(t.Context(), "ALTER TABLE page_links SET (autovacuum_enabled=false)"); err != nil {
		t.Fatal(err)
	}
	for b, n := range []int{3000, 1500} {
		for range n {
			f.pages[b] = append(f.pages[b], randomID(r))
		}
	}
	var rows [][]any
	key := func() any {
		if r.IntN(10) == 0 {
			return nil
		}
		return f.keys[r.IntN(len(f.keys))]
	}
	for b, book := range f.books {
		for _, page := range f.pages[b] {
			start := int32(0)
			for range r.IntN(13) {
				start += 1 + int32(r.IntN(40))
				var alt any
				if r.IntN(3) == 0 {
					alt = key()
				}
				var resolved any
				switch r.IntN(10) {
				case 0, 1:
				case 2:
					resolved = uuid.UUID{}
				case 3:
					resolved = randomID(r)
				case 4:
					resolved = f.pages[1-b][r.IntN(len(f.pages[1-b]))]
				default:
					resolved = f.pages[b][r.IntN(len(f.pages[b]))]
				}
				ambiguous, asset := r.IntN(5) == 0, r.IntN(4) == 0 // a resolution's alone (the table's checks)
				rows = append(rows, []any{page, start, start + 1, book, "wikilink", fmt.Sprintf("t%d", start), key(), alt, resolved,
					ambiguous && resolved != nil, asset && resolved != nil, r.IntN(6) == 0})
			}
		}
	}
	linksCollectorCopy(t, pool, rows)
	f.links = len(rows)
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)
	f.conn = conn
	return f
}

func randomID(r *rand.Rand) uuid.UUID {
	var id uuid.UUID
	for i := range id {
		id[i] = byte(r.Uint32())
	}
	return id
}

func (f *wideRandomFixture) analyze(t *testing.T) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), "ANALYZE page_links"); err != nil {
		t.Fatal(err)
	}
}

// wideRandomReach is a reach as native arrays: nil a NULL array, an
// invalid element a NULL element.
type wideRandomReach struct {
	book             uuid.UUID
	keys             []pgtype.Text
	targets, sources []pgtype.UUID
}

func (w wideRandomReach) String() string {
	return fmt.Sprintf("book %s, %d keys, %d targets, %d sources", w.book, len(w.keys), len(w.targets), len(w.sources))
}

// logSize is a size in [from, to], logarithmically spread.
func logSize(r *rand.Rand, from, to int) int {
	return from + int(math.Exp(r.Float64()*math.Log(float64(to-from+1)))) - 1
}

// reach draws a reach of size keys and targets together from f, with as
// many sources as a log-spread draw up to 2,000 gives, or up to limit all
// three together when limit is set.
func (f *wideRandomFixture) reach(r *rand.Rand, size, limit int) wideRandomReach {
	b := r.IntN(4) / 3 // the second notebook a quarter of the time
	w := wideRandomReach{book: f.books[b]}
	id := func(nullable bool) pgtype.UUID {
		switch n := r.IntN(40); {
		case nullable && n == 0:
			return pgtype.UUID{}
		case n == 1:
			return pgtype.UUID{Valid: true} // the zero id
		case n < 5:
			return pgtype.UUID{Bytes: randomID(r), Valid: true}
		case n < 8:
			return pgtype.UUID{Bytes: f.pages[1-b][r.IntN(len(f.pages[1-b]))], Valid: true}
		default:
			return pgtype.UUID{Bytes: f.pages[b][r.IntN(len(f.pages[b]))], Valid: true}
		}
	}
	keys := r.IntN(size + 1)
	sources := logSize(r, 0, 2000)
	if limit > 0 {
		sources = r.IntN(limit - size + 1)
	}
	if keys > 0 || r.IntN(4) != 0 { // else a NULL array
		w.keys = make([]pgtype.Text, keys)
		for i := range w.keys {
			switch n := r.IntN(40); {
			case n == 0:
			case n < 6:
				w.keys[i] = pgtype.Text{String: fmt.Sprintf("miss%d", r.IntN(1000)), Valid: true}
			default:
				w.keys[i] = pgtype.Text{String: f.keys[r.IntN(len(f.keys))], Valid: true}
			}
		}
	}
	if size-keys > 0 || r.IntN(4) != 0 {
		w.targets = make([]pgtype.UUID, size-keys)
		for i := range w.targets {
			w.targets[i] = id(true)
		}
	}
	if sources > 0 || r.IntN(4) != 0 {
		w.sources = make([]pgtype.UUID, sources)
		for i := range w.sources {
			w.sources[i] = id(true)
		}
	}
	return w
}

// compare reads groups reaches of seed, of from to to keys and targets
// together (and, with total, sources too), in the session's mode, through
// each of the three reads, and fails at the first that differs.
func (f *wideRandomFixture) compare(t *testing.T, mode string, seed uint64, groups, from, to int, total bool) {
	t.Helper()
	ctx := t.Context()
	setting := mode
	if mode != "force_generic_plan" {
		setting = "auto"
	}
	if _, err := f.conn.Exec(ctx, "SET plan_cache_mode = "+setting); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := f.conn.Exec(context.Background(), "RESET plan_cache_mode"); err != nil {
			t.Fatal(err)
		}
	}()
	statements := []struct{ name, sql string }{
		{"LinksReached", gen.LinksReached}, {"LinksReachedWide", gen.LinksReachedWide}, {"LinksReachedWideGeneric", gen.LinksReachedWideGeneric},
	}
	r := rand.New(rand.NewPCG(seed, seed^0x2545f4914f6cdd1d))
	began := time.Now()
	rows := 0
	for group := range groups {
		size := logSize(r, from, to)
		limit := 0
		if total {
			size = r.IntN(to + 1)
			limit = to
		}
		w := f.reach(r, size, limit)
		var first []app.Link
		for i, s := range statements {
			native, err := postgres.Planned(f.conn).Query(ctx, s.sql, w.book, w.keys, w.targets, w.sources)
			if err != nil {
				t.Fatalf("seed %#x group %d (%s, %s): %s: %v", seed, group, mode, w, s.name, err)
			}
			var buffer linksReachedBuffer
			got, err := buffer.read(native)
			if err != nil {
				t.Fatalf("seed %#x group %d (%s, %s): %s: %v", seed, group, mode, w, s.name, err)
			}
			if i == 0 {
				first, rows = got, rows+len(got)
				continue
			}
			if at := wideFirstDifference(first, got); at >= 0 {
				t.Fatalf("seed %#x group %d (%s, %s): %s and %s differ first at row %d of %d and %d: %s",
					seed, group, mode, w, statements[0].name, s.name, at, len(first), len(got), wideRowsAt(first, got, at))
			}
		}
	}
	t.Logf("%s: seed %#x, %d groups of %d to %d, %d rows each read, %v; the fixture's %d links",
		mode, seed, groups, from, to, rows, time.Since(began).Round(time.Millisecond), f.links)
}

func wideFirstDifference(a, b []app.Link) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return min(len(a), len(b))
	}
	return -1
}

func wideRowsAt(a, b []app.Link, at int) string {
	row := func(links []app.Link) string {
		if at < len(links) {
			return fmt.Sprintf("%+v", links[at])
		}
		return "none"
	}
	return row(a) + " against " + row(b)
}
