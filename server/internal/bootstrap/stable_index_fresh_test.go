package bootstrap

import (
	"net/http"
	"reflect"
	"slices"
	"testing"
	"uuid"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

func stableIndexInitial(t *testing.T, tm acmeTeam, p uuid.UUID, want []stableWiringRow) {
	t.Helper()
	got, err := stableWiringRows(t.Context(), tm.pool, p)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("stable API-created initial DTO=%+v err=%v want=%+v", got, err, want)
	}
}
func stableIndexDistinct(t *testing.T, ids ...uuid.UUID) {
	t.Helper()
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil() || seen[id] {
			t.Fatalf("stable fixture IDs not distinct/nonzero: %v", ids)
		}
		seen[id] = true
	}
}

func TestStableLinkRetentionRefreshesAllDerivedResolutions(t *testing.T) {
	t.Run("source_path", testStableIndexSourcePath)
	t.Run("alias_subtree", testStableIndexAliasSubtree)
	t.Run("wrong_derived", testStableIndexWrongDerived)
}

func testStableIndexSourcePath(t *testing.T) {
	control := &stableIndexControl{}
	tm := newAcmeTeamWiring(t, "", "", stableIndexWire(t, wiring{}, control))
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	b := tm.createPage(t, "alice", nb, "", "B")
	xa := tm.createPage(t, "alice", nb, a, "X")
	xb := tm.createPage(t, "alice", nb, b, "X")
	p := tm.createPage(t, "alice", nb, a, "P")
	marker := tm.createPage(t, "alice", nb, "", "Stable marker")
	const oldBody = "[[./X]] old\n"
	const newBody = "[[./X]] new\n"
	tm.send(t, contentWrite("alice", p, oldBody, 1, ""), http.StatusOK)
	P, N, A, B := uuid.MustParse(p), uuid.MustParse(nb), uuid.MustParse(xa), uuid.MustParse(xb)
	stableIndexDistinct(t, P, N, A, B)
	old := []stableWiringRow{stableIndexRow(P, N, 2, 5, "wikilink", "./X", "x", A, false, false)}
	final := []stableWiringRow{stableIndexRow(P, N, 2, 5, "wikilink", "./X", "x", B, false, false)}
	stableIndexInitial(t, tm, P, old)
	tm.settle(t, nb, marker)
	stream := openStream(t, tm.base, "alice", tm.tokens["alice"])
	u := newStableIndexUnit(p, nb, 3, newBody)
	control.arm(u)
	// Default rewrite remains wired. As the original batch put-back test does,
	// the final explicit write uses the unit's BEFORE version, after the move.
	f := ingestFixture{tm: tm, nb: nb}
	f.batch(t, opMove("A/P.md", "B/P.md"), opWriteOn("B/P.md", newBody, versionOf(p, 2)))
	control.arm()
	stableIndexCheck(t, tm, u, stableIndexWant{before: old, replaced: old, final: final, changed: stableIndexLiteralLinks(final), dropped: []uuid.UUID{A}, keys: []string{"x"}})
	stableIndexCheckPath(t, u, P, "B", "P")
	stableIndexCheckCandidates(t, u, A, B)
	stableIndexEvents(t, tm, nb, marker, stream, true, 3, p, xa, xb)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	tm.checkRebuilt(t, nb)
}

func testStableIndexAliasSubtree(t *testing.T) {
	control := &stableIndexControl{}
	tm := newAcmeTeamWiring(t, "", "", stableIndexWire(t, wiring{}, control))
	nb := tm.openNotebook(t, "alice", "Eng")
	const aliasBody = "---\naliases: [Nick]\n---\n"
	x := tm.createPageWith(t, "alice", nb, "", "X", aliasBody)
	y := tm.createPageWith(t, "alice", nb, "", "Y", aliasBody)
	tm.createPage(t, "alice", nb, "", "Box")
	p := tm.createPage(t, "alice", nb, "", "P")
	marker := tm.createPage(t, "alice", nb, "", "Stable marker")
	P, N, L, H := uuid.MustParse(p), uuid.MustParse(nb), uuid.MustParse(x), uuid.MustParse(y)
	lowName, highName := "X", "Y"
	if L.Compare(H) > 0 {
		L, H = H, L
		lowName, highName = highName, lowName
	}
	stableIndexDistinct(t, P, N, L, H)
	const oldBody = "[[Nick]] old\n"
	const newBody = "[[Nick]] new\n"
	tm.send(t, contentWrite("alice", p, oldBody, 1, ""), http.StatusOK)
	old := []stableWiringRow{stableIndexRow(P, N, 2, 6, "wikilink", "Nick", "nick", L, true, false)}
	final := []stableWiringRow{stableIndexRow(P, N, 2, 6, "wikilink", "Nick", "nick", H, false, false)}
	stableIndexInitial(t, tm, P, old)
	tm.settle(t, nb, marker)
	stream := openStream(t, tm.base, "alice", tm.tokens["alice"])
	u := newStableIndexUnit(p, nb, 3, newBody)
	control.arm(u)
	f := ingestFixture{tm: tm, nb: nb}
	f.batch(t, opMove(lowName+".md", "Box/"+lowName+".md"), opWriteOn("P.md", newBody, versionOf(p, 2)))
	control.arm()
	stableIndexCheck(t, tm, u, stableIndexWant{before: old, replaced: old, final: final, changed: stableIndexLiteralLinks(final), dropped: []uuid.UUID{L}, keys: []string{"nick"}})
	stableIndexCheckCandidates(t, u)
	stableIndexCheckPath(t, u, P, "P")
	stableIndexCheckPath(t, u, L, "Box", lowName)
	stableIndexCheckPath(t, u, H, highName)
	u.mu.Lock()
	aliases := slices.Clone(u.aliases)
	u.mu.Unlock()
	slices.SortFunc(aliases, func(a, b linkingapp.Alias) int { return a.PageID.Compare(b.PageID) })
	if want := []linkingapp.Alias{{PageID: L, Key: "nick"}, {PageID: H, Key: "nick"}}; !slices.Equal(aliases, want) {
		t.Errorf("stable fresh alias candidates=%+v want=%+v", aliases, want)
	}
	stableIndexEvents(t, tm, nb, marker, stream, true, 3, p, L.String(), H.String())
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	tm.checkRebuilt(t, nb)
}

func testStableIndexWrongDerived(t *testing.T) {
	control := &stableIndexControl{}
	tm := newAcmeTeamWiring(t, "", "", stableIndexWire(t, wiring{}, control))
	nb := tm.openNotebook(t, "alice", "Eng")
	raw := tm.createPage(t, "alice", nb, "", "raw")
	s := tm.createPage(t, "alice", nb, raw, "S")
	to := tm.createPage(t, "alice", nb, raw, "T")
	asset := tm.upload(t, "alice", nb, raw, "x.png", pngFile)
	p := tm.createPage(t, "alice", nb, "", "P")
	marker := tm.createPage(t, "alice", nb, "", "Stable marker")
	P, N, S, T, X := uuid.MustParse(p), uuid.MustParse(nb), uuid.MustParse(s), uuid.MustParse(to), uuid.MustParse(asset.ID)
	stableIndexDistinct(t, P, N, S, T, X)
	const oldBody = "[[raw/S]]\n[[raw/T]]\n![[raw/x.png]] old\n"
	const newBody = "[[raw/S]]\n[[raw/T]]\n![[raw/x.png]] new\n"
	tm.send(t, contentWrite("alice", p, oldBody, 1, ""), http.StatusOK)
	final := []stableWiringRow{
		stableIndexRow(P, N, 2, 7, "wikilink", "raw/S", "s", S, false, false),
		stableIndexRow(P, N, 12, 17, "wikilink", "raw/T", "t", T, false, false),
		stableIndexRow(P, N, 23, 32, "embed", "raw/x.png", "x.png", X, false, true),
	}
	stableIndexInitial(t, tm, P, final)
	// Only derived columns are damaged; these are an intentionally incorrect
	// committed index baseline, not an API-created healthy business state.
	ingestExec(t, tm, `UPDATE page_links SET resolved_id=$2 WHERE source_id=$1 AND range_start=2`, P, T)
	ingestExec(t, tm, `UPDATE page_links SET ambiguous=true WHERE source_id=$1 AND range_start=12`, P)
	ingestExec(t, tm, `UPDATE page_links SET resolved_asset=false WHERE source_id=$1 AND range_start=23`, P)
	old := []stableWiringRow{
		stableIndexRow(P, N, 2, 7, "wikilink", "raw/S", "s", T, false, false),
		stableIndexRow(P, N, 12, 17, "wikilink", "raw/T", "t", T, true, false),
		stableIndexRow(P, N, 23, 32, "embed", "raw/x.png", "x.png", X, false, false),
	}
	stableIndexInitial(t, tm, P, old)
	tm.settle(t, nb, marker)
	stream := openStream(t, tm.base, "alice", tm.tokens["alice"])
	u := newStableIndexUnit(p, nb, 3, newBody)
	control.arm(u)
	tm.send(t, contentWrite("alice", p, newBody, 2, ""), http.StatusOK)
	control.arm()
	stableIndexCheck(t, tm, u, stableIndexWant{before: old, replaced: old, final: final, changed: stableIndexLiteralLinks(final), dropped: stableIndexIDs(T, X), keys: []string{"s", "t", "x.png"}})
	stableIndexCheckCandidates(t, u, S, T, X)
	stableIndexCheckPath(t, u, P, "P")
	stableIndexEvents(t, tm, nb, marker, stream, false, 3, p, s, to, asset.ID)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	tm.checkRebuilt(t, nb)
}
