package domain_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// land finds target's candidates, the nodes of its segment before the last
// and its aliased pages as the index does, by their keys, and lands it
// from the page from.
func (c caseTree) land(target domain.Target, from string, maxDepth int) domain.Landing {
	var candidates, parents []domain.Node
	aliased := map[string][]domain.Node{}
	for p, path := range c.paths {
		node := domain.Node{ID: c.ids[p], Path: path, Asset: c.assets[p]}
		key := path[len(path)-1].Key
		if slices.Contains(target.LastKeys(), key) {
			candidates = append(candidates, node)
		}
		if n := len(target.Keys); n > 1 && key == target.Keys[n-2] {
			parents = append(parents, node)
		}
		for _, a := range c.aliases[p] {
			if slices.Contains(target.LastKeys(), a) {
				aliased[a] = append(aliased[a], node)
			}
		}
	}
	return domain.Land(target, c.paths[from], candidates, parents, aliased, maxDepth)
}

// nodes are the case's nodes whose title key is one of keys.
func (c caseTree) nodes(keys []string) []domain.Node {
	var out []domain.Node
	for p, path := range c.paths {
		if slices.Contains(keys, path[len(path)-1].Key) {
			out = append(out, domain.Node{ID: c.ids[p], Path: path, Asset: c.assets[p]})
		}
	}
	return out
}

// pathOf is the page of id, by its path in the case; "" for the root.
func (c caseTree) pathOf(id uuid.UUID) string {
	for p, pid := range c.ids {
		if pid == id {
			return p
		}
	}
	return ""
}

// A target that resolves lands on its page; one that does not, under the
// source folder as a name alone, under the page its path's other segments
// lead to, read from the folder up or the root exactly, else by the
// resolution, not by aliases, titled with its last segment as written
// without ".md"; and nowhere when that is no title, no page is there, the
// page would be too deep, or the target would not lead to the page made
// (M6/P6 design 2).
func TestATargetLandsWhereItWouldLeadToThePageMade(t *testing.T) {
	tree := treeOf(t, []string{"A", "A/src", "A/B", "A/B/C", "B", "P", "Q", "Q/A", "Note", "R", "R/dup", "S", "S/dup", "T", "T/x.md", "U.md"},
		map[string][]string{"P": {"Al"}})
	type want struct {
		node, parent, title string
		reason              domain.Reason
	}
	tests := []struct {
		target, from string
		depth        int
		want         want
	}{
		{"x", "A/src", 10, want{parent: "A", title: "x"}},
		{"x", "A", 10, want{title: "x"}},
		{"y.md", "A/src", 10, want{parent: "A", title: "y"}},
		{"Y.MD", "A/src", 10, want{parent: "A", title: "Y"}},
		// The page titled x.md is one.
		{"x.md", "A/src", 10, want{node: "T/x.md"}},
		{"B/x", "A/src", 10, want{parent: "B", title: "x"}},
		{"A/x", "B", 10, want{parent: "A", title: "x"}},
		{"B/C/x", "Note", 10, want{parent: "A/B/C", title: "x"}},
		{"./x", "A/B", 10, want{parent: "A", title: "x"}},
		{"./B/x", "A/src", 10, want{parent: "A/B", title: "x"}},
		{"../x", "A/B", 10, want{title: "x"}},
		{"../../../x", "A/B/C", 10, want{title: "x"}},
		{"/A/x", "B", 10, want{parent: "A", title: "x"}},
		{"/A/B/x", "B", 10, want{parent: "A/B", title: "x"}},
		// Of two pages alike, the one the resolution takes, the least id.
		{"dup/x", "Note", 10, want{parent: "R/dup", title: "x"}},
		// x is made, which [[A/x.md]] then reads, not T/x.md: a page made may
		// lead other links to it (M6 design 4.4).
		{"A/x.md", "Note", 10, want{parent: "A", title: "x"}},
		{"Note", "A/src", 10, want{node: "Note"}},
		{"Al", "A/src", 10, want{node: "P"}},
		{"/Z/x", "B", 10, want{reason: domain.ParentMissing}},
		// From the root exactly: A/B/C is no C there.
		{"/C/x", "A", 10, want{reason: domain.ParentMissing}},
		// A segment before the last keeps its ".md": the page titled U.md, and
		// no page titled Q.md.
		{"U.md/x", "Note", 10, want{parent: "U.md", title: "x"}},
		{"Q.md/x", "Note", 10, want{reason: domain.ParentMissing}},
		{"Z/x", "B", 10, want{reason: domain.ParentMissing}},
		{"./Z/x", "A/src", 10, want{reason: domain.ParentMissing}},
		// An alias leads from a name alone only.
		{"Al/x", "B", 10, want{reason: domain.ParentMissing}},
		{"a:b", "A", 10, want{reason: domain.TitleInvalid}},
		{"B/x.", "A", 10, want{reason: domain.TitleInvalid}},
		// A title is trimmed, a title key is not: x made, " x" would not lead to it.
		{"B/ x", "A", 10, want{reason: domain.NotResolvable}},
		{"A/B/x", "Note", 3, want{parent: "A/B", title: "x"}},
		{"A/B/C/x", "Note", 3, want{reason: domain.TooDeep}},
	}
	for _, tt := range tests {
		target, ok := domain.ParseTarget(tt.target)
		if !ok {
			t.Fatalf("ParseTarget(%q) refused", tt.target)
		}
		got := tree.land(target, tt.from, tt.depth)
		g := want{node: tree.pathOf(got.Node), parent: tree.pathOf(got.Parent), title: got.Title, reason: got.Reason}
		if got.Node == (uuid.UUID{}) {
			g.node = ""
		}
		if g != tt.want {
			t.Errorf("%s from %s: %+v, want %+v", tt.target, tt.from, g, tt.want)
		}
	}
}

// A target read as an attachment's has no landing, target_is_asset, which
// it leads to or not; one written with ".md", or with no attachment of its
// name, an attachment without an extension, lands as a page's would; a
// segment before the last leads to a page, never to an attachment of its
// name (M7/P3 design 4.5).
func TestATargetReadAsAnAttachmentsHasNoLanding(t *testing.T) {
	tree := treeOf(t, []string{"A", "src", "B", "B/x.png"}, nil, "A/x.png", "A/noext", "A/y.png", "v1.2")
	type want struct {
		node, parent, title string
		reason              domain.Reason
	}
	for _, tt := range []struct {
		target string
		want   want
	}{
		{"x.png", want{reason: domain.TargetIsAsset}},
		{"X.PNG", want{reason: domain.TargetIsAsset}},
		{"B/x.png", want{reason: domain.TargetIsAsset}},
		{"./x.png", want{reason: domain.TargetIsAsset}},
		{"x.png.md", want{node: "B/x.png"}},
		{"noext", want{title: "noext"}},
		{"z.png", want{title: "z.png"}},
		{"B/z.png", want{parent: "B", title: "z.png"}},
		{"y.png/z", want{reason: domain.ParentMissing}},
		{"x.png/z", want{parent: "B/x.png", title: "z"}},
		// Read as a page's, no page titled v1.2 anywhere: the page made, v1.2,
		// would be beside the attachment v1.2 (the random test's seed 148).
		{"v1.2.md", want{reason: domain.TargetIsAsset}},
		{"A/v1.2.md", want{parent: "A", title: "v1.2"}},
		// An attachment of the title as deep, under another page, is not where
		// the page would go.
		{"B/y.png.md", want{parent: "B", title: "y.png"}},
	} {
		target, ok := domain.ParseTarget(tt.target)
		if !ok {
			t.Fatalf("ParseTarget(%q) refused", tt.target)
		}
		got := tree.land(target, "src", 10)
		g := want{node: tree.pathOf(got.Node), parent: tree.pathOf(got.Parent), title: got.Title, reason: got.Reason}
		if got.Node == (uuid.UUID{}) {
			g.node = ""
		}
		if g != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.target, g, tt.want)
		}
	}
}

// In random trees, a random target written from a random page that does
// not resolve lands where the page made is the one it then resolves to,
// alone, and no sibling of it has its title key; one that resolves lands
// on its page; one with no landing gives a reason, and for no parent page
// nor the root, when no page is there, the target would not lead to it or
// it is read as an attachment's, would a page of its title made there be
// the one it then resolves to alone; one read as a page's and still
// target_is_asset has an attachment of its title where the page would go,
// without the attachments of that title (M6/P6 design 2; M7/P3 design 4.5).
func TestALandingLeadsTheTargetToThePageMade(t *testing.T) {
	landed, assets, beside := 0, 0, 0
	for seed := range uint64(20000) {
		r := rand.New(rand.NewPCG(seed, 9))
		c := randomCase(r)
		aliases := map[string][]string{}
		for i, p := range c.pages {
			aliases[p] = c.aliases[i]
		}
		tree := treeOf(t, c.pages, aliases, c.assets...)
		from := c.pages[r.IntN(len(c.pages))]
		written := landingTarget(r, c.pages, aliases, from)
		target, ok := domain.ParseTarget(written)
		if !ok {
			continue
		}
		got := tree.land(target, from, 10)
		switch {
		case got.Node != (uuid.UUID{}):
			if r := tree.resolve(target, from); r.ID != got.Node {
				t.Fatalf("seed %d: %s from %s lands on %s, resolves to %s", seed, written, from, tree.pathOf(got.Node), tree.name(r))
			}
		case got.Title != "":
			landed++
			parent := tree.pathOf(got.Parent)
			made := got.Title
			if parent != "" {
				made = parent + "/" + got.Title
			}
			if slices.ContainsFunc(c.nodes(), func(p string) bool { return siblings(p, made) }) {
				t.Fatalf("seed %d: %s from %s lands as %s, beside a sibling of its key", seed, written, from, made)
			}
			after := treeOf(t, append(slices.Clone(c.pages), made), aliases, c.assets...)
			if r := after.resolve(target, from); r.ID != after.ids[made] || r.Ambiguous {
				t.Fatalf("seed %d: %s from %s lands as %s, then resolves to %s", seed, written, from, made, after.name(r))
			}
		case got.Reason == "":
			t.Fatalf("seed %d: %s from %s: no node, landing nor reason", seed, written, from)
		case got.Reason == domain.TargetIsAsset && !target.ReadsAsAsset(tree.nodes(target.LastKeys())):
			beside++
			key := shared.TitleKey(target.Name)
			others := slices.DeleteFunc(slices.Clone(c.assets), func(a string) bool { return shared.TitleKey(lastOf(a)) == key })
			alone := treeOf(t, c.pages, aliases, others...).land(target, from, 10)
			made := alone.Title
			if parent := tree.pathOf(alone.Parent); parent != "" {
				made = parent + "/" + alone.Title
			}
			if alone.Title == "" && alone.Reason != domain.NotResolvable ||
				alone.Title != "" && !slices.ContainsFunc(c.assets, func(a string) bool { return siblings(a, made) }) {
				t.Fatalf("seed %d: %s from %s, read as a page's: target_is_asset, no attachment of its title where it lands without them: %+v",
					seed, written, from, alone)
			}
		case got.Reason == domain.ParentMissing || got.Reason == domain.NotResolvable || got.Reason == domain.TargetIsAsset:
			if got.Reason == domain.TargetIsAsset {
				assets++
			}
			if made, ok := landsElsewhere(t, c.pages, c.assets, aliases, target, from); ok {
				t.Fatalf("seed %d: %s from %s: %s, but the page %s would lead it", seed, written, from, got.Reason, made)
			}
		}
	}
	if landed < 1000 || assets < 300 || beside < 20 {
		t.Errorf("%d targets landed, %d read as attachments', %d beside one: the targets are not random enough", landed, assets, beside)
	}
}

// landsElsewhere is a page of target's title, made under the root or one
// of pages and beside no sibling of its title key, a page's or one of
// assets', that target, written in the page at from, would then resolve to
// alone, if one would.
func landsElsewhere(t *testing.T, pages, assets []string, aliases map[string][]string, target domain.Target, from string) (string, bool) {
	t.Helper()
	title, problem := shared.CheckTitle("title", target.Name)
	if problem != nil {
		t.Fatalf("%q: a reason after the title, which is none", target.Name)
	}
	for _, parent := range append([]string{""}, pages...) {
		made := title
		if parent != "" {
			made = parent + "/" + title
		}
		if slices.ContainsFunc(slices.Concat(pages, assets), func(p string) bool { return siblings(p, made) }) {
			continue
		}
		after := treeOf(t, append(slices.Clone(pages), made), aliases, assets...)
		if r := after.resolve(target, from); r.ID == after.ids[made] && !r.Ambiguous {
			return made, true
		}
	}
	return "", false
}

// landingTarget is a target written from the page at from to a page none
// may be yet: a new title, or one of pages', alone, after a page's path or
// its end, from the root, from from's folder up, after an alias, with
// ".md" or a space at times.
func landingTarget(r *rand.Rand, pages []string, aliases map[string][]string, from string) string {
	title := []string{"new", "New", "x", "a b", "x.md", "é", "a:b", " pad", "Plan"}[r.IntN(9)]
	switch r.IntN(4) {
	case 0:
		title = randomTitle(r)
	case 1:
		title = randomAssetName(r)
	}
	page := pages[r.IntN(len(pages))]
	var target string
	switch r.IntN(7) {
	case 0:
		target = title
	case 1:
		target = page + "/" + title
	case 2:
		segments := strings.Split(page, "/")
		target = strings.Join(segments[r.IntN(len(segments)):], "/") + "/" + title
	case 3:
		target = "/" + page + "/" + title
	case 4:
		ups := r.IntN(strings.Count(from, "/") + 2)
		target = strings.Repeat("../", ups) + title
		if ups == 0 {
			target = "./" + title
		}
	case 5:
		var all []string
		for _, as := range aliases {
			all = append(all, as...)
		}
		if len(all) == 0 {
			return title
		}
		target = all[r.IntN(len(all))] + "/" + title
	default:
		target = randomTarget(r, pages, from)
	}
	if r.IntN(5) == 0 {
		target += ".md"
	}
	return target
}
