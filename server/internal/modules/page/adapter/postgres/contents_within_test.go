package postgresadapter_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// within is a read of contents within a bound as these tests tell it: the
// pages read, by name, in their order, and the page past the bound with its
// size, "" for none.
func within(names map[uuid.UUID]string, read []uuid.UUID, past uuid.UUID, bytes int) string {
	var out []string
	for _, id := range read {
		out = append(out, names[id])
	}
	slices.Sort(out)
	s := strings.Join(out, " ")
	if past != (uuid.UUID{}) {
		s += fmt.Sprintf(" | %s %d", names[past], bytes)
	}
	return s
}

// The statements that read contents within a bound of bytes (M9 Codex
// review D2) read those of the pages in the order asked while their sizes,
// as each statement reads them, sum to the bound at most; of the first page
// past it its size alone; none after it; a page deleted is not there. A
// page grown since a size read before is past where its size now takes
// the sum. GrepContents and SearchContents of A, B and C, of 10, 20 and 30
// bytes, a deleted page among them; TrashContentsWithin of the same in the
// trash; PageContentWithin of one page.
func TestTheContentsAreReadWithinTheirBound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	names := map[uuid.UUID]string{}
	page := func(name string, bytes int) uuid.UUID {
		p := f.page(t, f.eng, nil, name, float64(len(names)))
		f.write(t, p.ID, strings.Repeat("x", bytes), 2, now())
		names[p.ID] = name
		return p.ID
	}
	a, gone, b, c := page("A", 10), page("Gone", 5), page("B", 20), page("C", 30)
	f.trashed(t, now(), 1, 0, 1, gone)
	ids := []uuid.UUID{a, gone, b, c}
	cases := []struct {
		max  int64
		want string
	}{{60, "A B C"}, {59, "A B | C 30"}, {30, "A B | C 30"}, {29, "A | B 20"}, {10, "A | B 20"}, {9, " | A 10"}, {0, " | A 10"}}
	for _, tt := range cases {
		got, err := f.s.GrepContents(ctx, ids, tt.max)
		if err != nil {
			t.Fatal(err)
		}
		if s := within(names, slices.Collect(maps.Keys(got.Contents)), got.Past, got.PastBytes); s != tt.want {
			t.Errorf("GrepContents within %d = %q, want %q", tt.max, s, tt.want)
		}
		for id, content := range got.Contents {
			if content != strings.Repeat("x", len(content)) || len(content) == 0 {
				t.Errorf("GrepContents within %d read %s as %q", tt.max, names[id], content)
			}
		}
		texts, past, err := f.s.SearchContents(ctx, ids, tt.max)
		if err != nil {
			t.Fatal(err)
		}
		pastBytes := map[uuid.UUID]int{a: 10, b: 20, c: 30}[past]
		if s := within(names, slices.Collect(maps.Keys(texts)), past, pastBytes); s != tt.want ||
			slices.ContainsFunc(slices.Collect(maps.Values(texts)), func(p app.PageText) bool { return p.Revision != 2 }) {
			t.Errorf("SearchContents within %d = %q, %+v; want %q at revision 2", tt.max, s, texts, tt.want)
		}
	}
	// The order asked decides: C first, then A.
	if got, err := f.s.GrepContents(ctx, []uuid.UUID{c, a, b}, 35); err != nil || within(names, slices.Collect(maps.Keys(got.Contents)),
		got.Past, got.PastBytes) != "C | A 10" {
		t.Errorf("GrepContents of C, A, B within 35 = %+v, %v; want C, A past", got, err)
	}
	// B grown since: past at its size now.
	f.write(t, b, strings.Repeat("x", 200), 3, now().Add(time.Second))
	if got, err := f.s.GrepContents(ctx, ids, 60); err != nil || within(names, slices.Collect(maps.Keys(got.Contents)), got.Past,
		got.PastBytes) != "A | B 200" {
		t.Errorf("GrepContents with B grown = %+v, %v; want A, B past at 200", got, err)
	}

	// The trash's at its time alone: R's group, X, Y and Z of 10, 20 and 30
	// bytes, and W of another time.
	r := f.page(t, f.eng, nil, "R", 10)
	trash := []uuid.UUID{r.ID}
	for i, name := range []string{"X", "Y", "Z"} {
		p := f.page(t, f.eng, &r.ID, name, float64(i))
		f.write(t, p.ID, strings.Repeat("y", 10*(i+1)), 2, now())
		names[p.ID] = name
		trash = append(trash, p.ID)
	}
	w := page("W", 1)
	f.trashed(t, now(), 1, 0, 1, w)
	at := now().Add(time.Minute)
	f.trashed(t, at, 4, 0, 2, trash...)
	names[r.ID] = "R"
	for _, tt := range []struct {
		max  int64
		want string
	}{{60, "R X Y Z"}, {29, "R X | Y 20"}, {0, "R | X 10"}} {
		got, err := f.s.TrashContentsWithin(ctx, append(slices.Clone(trash), w), at, tt.max)
		if err != nil {
			t.Fatal(err)
		}
		if s := within(names, slices.Collect(maps.Keys(got.Contents)), got.Past, got.PastBytes); s != tt.want {
			t.Errorf("TrashContentsWithin within %d = %q, want %q", tt.max, s, tt.want)
		}
	}

	// One page: A of 10 bytes within 10, not within 9; a deleted page's
	// not found.
	if got, ok, err := f.s.PageContentWithin(ctx, a, 10); err != nil || !ok || got.Content != strings.Repeat("x", 10) || got.Revision != 2 ||
		len(got.Hash) != 32 {
		t.Errorf("PageContentWithin(A, 10) = %+v, %v, %v; want its content at revision 2, with its hash", got, ok, err)
	}
	if got, ok, err := f.s.PageContentWithin(ctx, a, 9); err != nil || ok || got.Content != "" {
		t.Errorf("PageContentWithin(A, 9) = %+v, %v, %v; want none read", got, ok, err)
	}
	if _, _, err := f.s.PageContentWithin(ctx, gone, 100); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("PageContentWithin(deleted) = %v, want ErrNotFound", err)
	}
}

// The content of a page past the bound is never fetched (M9 Codex review
// D2): each statement reads a row once, and a content held out of line in
// page_contents' TOAST is carried as its pointer, by the window and the
// CASE alike, and not returned, its blocks not read. Big, a page of 1 MiB
// that does not compress, after S of 10 bytes, in the notebook and in the
// trash: within 10 bytes, none of the TOAST's blocks fetched, its table's
// or its index's; within 2 MiB, Big's, read whole.
func TestTheContentsPastTheBoundAreNotFetched(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r := rand.New(rand.NewPCG(20261024, 1))
	raw := make([]byte, 3<<18)
	for i := range raw {
		raw[i] = byte(r.Uint32())
	}
	big := base64.StdEncoding.EncodeToString(raw)
	page := func(parent *uuid.UUID, name, content string) uuid.UUID {
		p := f.page(t, f.eng, parent, name, 0)
		f.write(t, p.ID, content, 2, now())
		return p.ID
	}
	s, b := page(nil, "S", strings.Repeat("x", 10)), page(nil, "Big", big)
	ts := page(nil, "TS", strings.Repeat("x", 10))
	tb := page(&ts, "TBig", big)
	at := now().Add(time.Minute)
	f.trashed(t, at, 2, 0, 2, ts, tb)
	// fetched is the blocks the transaction fetched of page_contents' TOAST
	// table and its index.
	const fetched = `SELECT coalesce(sum(pg_stat_get_xact_blocks_fetched(x.r)), 0)::bigint FROM (
			SELECT c.reltoastrelid AS r FROM pg_class c WHERE c.relname = 'page_contents'
			UNION ALL SELECT i.indexrelid FROM pg_index i JOIN pg_class c ON c.reltoastrelid = i.indrelid
				WHERE c.relname = 'page_contents') x`
	toast := func(read func(ctx context.Context) error) int64 {
		t.Helper()
		var before, after int64
		err := postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(ctx, func(ctx context.Context) error {
			db := postgres.DB(ctx, f.pool)
			if err := db.QueryRow(ctx, fetched).Scan(&before); err != nil {
				return err
			}
			if err := read(ctx); err != nil {
				return err
			}
			return db.QueryRow(ctx, fetched).Scan(&after)
		})
		if err != nil {
			t.Fatal(err)
		}
		return after - before
	}
	for _, tt := range []struct {
		name string
		read func(ctx context.Context, max int64) (bool, error) // whether Big's content came back whole
	}{
		{"GrepContents", func(ctx context.Context, max int64) (bool, error) {
			got, err := f.s.GrepContents(ctx, []uuid.UUID{s, b}, max)
			return got.Contents[b] == big, err
		}},
		{"SearchContents", func(ctx context.Context, max int64) (bool, error) {
			texts, _, err := f.s.SearchContents(ctx, []uuid.UUID{s, b}, max)
			return texts[b].Content == big, err
		}},
		{"TrashContentsWithin", func(ctx context.Context, max int64) (bool, error) {
			got, err := f.s.TrashContentsWithin(ctx, []uuid.UUID{ts, tb}, at, max)
			return got.Contents[tb] == big, err
		}},
		{"PageContentWithin", func(ctx context.Context, max int64) (bool, error) {
			got, ok, err := f.s.PageContentWithin(ctx, b, int(max))
			return ok && got.Content == big, err
		}},
	} {
		for _, max := range []int64{10, 2 << 20} {
			var read bool
			n := toast(func(ctx context.Context) error {
				var err error
				read, err = tt.read(ctx, max)
				return err
			})
			if past := max < int64(len(big)); past && (read || n != 0) || !past && (!read || n < 100) {
				t.Errorf("%s within %d: Big read %v, %d blocks of the TOAST fetched; want it read and its blocks fetched only "+
					"within its size", tt.name, max, read, n)
			}
		}
	}
}
