package postgresadapter_test

import (
	"context"
	"math/rand/v2"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// randomCJK is n random characters of CJK's unified ideographs: bigrams
// all but alike, so that 240 KiB of them pass MaxSearchLexemeBytes.
func randomCJK(r *rand.Rand, n int) string {
	var b strings.Builder
	for range n {
		b.WriteRune(rune(0x4E00 + r.IntN(0x9FFF-0x4E00)))
	}
	return b.String()
}

// cutContents are a content of each cut (M9/P4 design 3.3): one past
// MaxSearchLexemeBytes, 240 KiB of random CJK; a run of 70 ASCII letters
// and digits; a data: image of 80 characters; none of them holds needle's
// keys among its lexemes, needle written where each leaves text out.
func cutContents(r *rand.Rand) map[string]string {
	return map[string]string{
		"bound": "x\n" + randomCJK(r, 240<<10/3) + " needle",
		"long":  "a run x" + strings.Repeat("ab", 30) + "needle" + strings.Repeat("cd", 10),
		"data":  "![a](data:image/png;base64," + strings.Repeat("Qk", 30) + "needle" + strings.Repeat("Zx", 20) + ")",
	}
}

// keysOf is pattern's grep keys.
func keysOf(t *testing.T, pattern string) []string {
	t.Helper()
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	return domain.GrepKeys(re)
}

// GrepCandidates is, of the notebook alone, the pages whose row holds
// every key, those queued exact, and those whose row is partial, each cut
// (M9/P4 design 3.4.2): a page queued in bulk is none, nor another
// notebook's, holding the keys, cut, queued exact or in bulk while it
// reads (review B-1), nor one whose lexemes lack a key (mutant 152). A
// notebook is building by its own pages in bulk alone. GrepContents is
// export's batch.
func TestGrepCandidates(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r := rand.New(rand.NewPCG(4, 5))
	keyed := f.page(t, f.eng, nil, "Keyed", 0)
	f.write(t, keyed.ID, "a needle in a haystack", 2, now())
	plain := f.page(t, f.eng, nil, "Plain", 1)
	f.write(t, plain.ID, "plain words", 2, now())
	cuts := map[string]uuid.UUID{}
	for kind, content := range cutContents(r) {
		p := f.page(t, f.eng, nil, "Cut "+kind, 2)
		f.write(t, p.ID, content, 2, now())
		cuts[kind] = p.ID
	}
	other := f.page(t, f.ops, nil, "Other", 0)
	f.write(t, other.ID, "a needle in another notebook", 2, now())
	otherCut := f.page(t, f.ops, nil, "Other cut", 1)
	f.write(t, otherCut.ID, cutContents(r)["long"], 2, now())
	all := []uuid.UUID{keyed.ID, plain.ID, cuts["bound"], cuts["long"], cuts["data"]}
	f.queueSearch(t, f.eng, false, all...)
	f.queueSearch(t, f.ops, false, other.ID, otherCut.ID)
	f.drain(t)
	for kind, id := range cuts {
		if f.count(t, `SELECT count(*) FROM page_search WHERE node_id = $1 AND partial`, id) != 1 {
			t.Errorf("the %s page's row is not partial", kind)
		}
	}
	exact := f.page(t, f.eng, nil, "Exact", 3)
	f.write(t, exact.ID, "needle", 2, now())
	bulk := f.page(t, f.eng, nil, "Bulk", 4)
	f.write(t, bulk.ID, "needle", 2, now())
	// Ops's pages queued again, one exact and one in bulk, while eng's are
	// read; building is the notebook's own (review B-1).
	f.queueSearch(t, f.ops, false, other.ID)
	f.queueSearch(t, f.ops, true, otherCut.ID)
	if building, err := f.s.SearchBuilding(ctx, []uuid.UUID{f.eng}); err != nil || building {
		t.Errorf("SearchBuilding(eng) with ops's page alone in bulk = %v, %v; want false", building, err)
	}
	f.queueSearch(t, f.eng, false, exact.ID)
	f.queueSearch(t, f.eng, true, bulk.ID)
	if building, err := f.s.SearchBuilding(ctx, []uuid.UUID{f.eng}); err != nil || !building {
		t.Errorf("SearchBuilding(eng) with its page in bulk = %v, %v; want true", building, err)
	}
	got, err := f.s.GrepCandidates(ctx, f.eng, keysOf(t, "needle"))
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{keyed.ID, cuts["bound"], cuts["long"], cuts["data"], exact.ID}
	slices.SortFunc(got, uuid.UUID.Compare)
	slices.SortFunc(want, uuid.UUID.Compare)
	if !slices.Equal(got, want) {
		t.Errorf("GrepCandidates(needle) = %v, want keyed, the three cut, exact %v", got, want)
	}
	// Keys no page holds: the cut and the exact alone.
	got, err = f.s.GrepCandidates(ctx, f.eng, []string{"zzq", "zqz"})
	if err != nil {
		t.Fatal(err)
	}
	if want := 4; len(got) != want || slices.Contains(got, keyed.ID) {
		t.Errorf("GrepCandidates(zzq) = %v, want the three cut and exact", got)
	}
	contents, err := f.s.GrepContents(ctx, []uuid.UUID{keyed.ID, plain.ID, uuid.NewV7()})
	if err != nil || len(contents) != 2 || contents[keyed.ID] != "a needle in a haystack" {
		t.Errorf("GrepContents = %q, %v", contents, err)
	}
}
