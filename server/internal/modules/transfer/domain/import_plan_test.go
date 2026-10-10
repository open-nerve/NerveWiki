package domain_test

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// importEntries are the archive's entries of paths, in order: a path ending in
// "/" a folder.
func importEntries(paths ...string) []domain.ImportEntry {
	out := make([]domain.ImportEntry, len(paths))
	for i, p := range paths {
		out[i] = domain.ImportEntry{Index: i, Folder: strings.HasSuffix(p, "/"), Path: strings.Split(strings.TrimSuffix(p, "/"), "/"), Name: p}
	}
	return out
}

// planned is the plan as lines: each node "level parent kind name <- path
// [#entry]", the parent a path or "-", then each problem.
func planned(p domain.ImportPlan) []string {
	var out []string
	for _, n := range p.Nodes {
		parent := "-"
		if n.Parent >= 0 {
			parent = p.Nodes[n.Parent].Name
		}
		kind := "page"
		if n.Asset {
			kind = "asset"
		}
		line := fmt.Sprintf("%d %s %s %s <- %s", n.Level, parent, kind, n.Name, n.Path)
		if n.Entry >= 0 {
			line += fmt.Sprintf(" #%d", n.Entry)
		}
		if n.Original != n.Name {
			line += " (" + n.Original + ")"
		}
		out = append(out, line)
	}
	for _, s := range p.Skipped {
		out = append(out, string(s.Code)+": "+s.Path)
	}
	return out
}

func TestNewImportPlan(t *testing.T) {
	// An attachment's name whose cut ends with ".md".
	longCut := strings.Repeat("x", 252) + ".md." + strings.Repeat("y", 300)
	for _, tt := range []struct {
		name  string
		paths []string
		meta  domain.ImportMeta
		depth int
		want  []string
	}{
		{"pages, folders and attachments by level", []string{"Home.md", "Projects/", "Projects/A.md", "Projects/A/spec.md",
			"Projects/A/diagram.png", "image.png", "Projects/A/Deep/x.md"},
			domain.ImportMeta{}, 0, []string{
				"1 - page Home <- Home.md #0",
				"1 - asset image.png <- image.png #5",
				"1 - page Projects <- Projects/",
				"2 Projects page A <- Projects/A.md #2",
				"3 A page Deep <- Projects/A/Deep/",
				"3 A asset diagram.png <- Projects/A/diagram.png #4",
				"3 A page spec <- Projects/A/spec.md #3",
				"4 Deep page x <- Projects/A/Deep/x.md #6",
			}},
		{"a page holds the folder of its key", []string{"x/child.md", "X.md"}, domain.ImportMeta{}, 0,
			[]string{"1 - page X <- X.md #1", "2 X page child <- x/child.md #0"}},
		{"the first page of the key holds the folder", []string{"N.md", "n/c.md", "n.md"}, domain.ImportMeta{}, 0,
			[]string{"1 - page N <- N.md #0", "1 - page n <- n.md #2", "2 N page c <- n/c.md #1"}},
		{"keys before the names are mended", []string{"a:b.md", "a_b/c.md"}, domain.ImportMeta{}, 0,
			[]string{"1 - page a_b <- a_b/", "1 - page a_b <- a:b.md #0 (a:b)", "2 a_b page c <- a_b/c.md #1"}},
		{"a name as in the archive before one mended to it", []string{"What? Why.md", "What_ Why.md", "x|y.png", "X_Y.png"},
			domain.ImportMeta{}, 0, []string{"1 - page What_ Why <- What_ Why.md #1", "1 - page What_ Why <- What? Why.md #0 (What? Why)",
				"1 - asset X_Y.png <- X_Y.png #3", "1 - asset x_y.png <- x|y.png #2 (x|y.png)"}},
		{"names mended", []string{"a#b.md", "con.md", "photo.MD", "x.md.png", "notes.md. ", " /a.md", "nul.txt", "b|c.png"},
			domain.ImportMeta{}, 0, []string{
				"1 - page a_b <- a#b.md #0 (a#b)",
				"1 - asset b_c.png <- b|c.png #7 (b|c.png)",
				"1 - page con_ <- con.md #1 (con)",
				"1 - page notes <- notes.md.  #4",
				"1 - asset nul_.txt <- nul.txt #6 (nul.txt)",
				"1 - page photo <- photo.MD #2",
				"1 - asset x.md.png <- x.md.png #3",
				"1 - page " + domain.Untitled + " <-  / ( )",
				"2 " + domain.Untitled + " page a <-  /a.md #5",
			}},
		// Its extension too long to keep, the name is cut where ".md" ends it.
		{"an attachment's name cut to end with .md", []string{longCut}, domain.ImportMeta{}, 0, []string{
			"1 - asset " + strings.Repeat("x", 251) + ".md_ <- " + longCut + " #0 (" + longCut + ")",
		}},
		{"the order of meta, then by name", []string{"b.md", "a.md", "c/", "c/z.md", "d.png", "e.md"},
			domain.ImportMeta{Order: map[string]float64{"e.md": 1, "c/": 2, "b.md": 3}}, 0, []string{
				"1 - page e <- e.md #5",
				"1 - page c <- c/",
				"1 - page b <- b.md #0",
				"1 - page a <- a.md #1",
				"1 - asset d.png <- d.png #4",
				"2 c page z <- c/z.md #3",
			}},
		{"the contributors' files left out", []string{"index.md", "a.md", "log/2026.md"},
			domain.ImportMeta{Contributed: map[string]bool{"index.md": true, "log/2026.md": true}}, 0, []string{"1 - page a <- a.md #1"}},
		{"deeper than pages go from where the import goes", []string{"A.md", "A/B.md", "A/B/C.md", "A/B/x.png", "A/y.png", "z.png"},
			domain.ImportMeta{}, 9, []string{
				"1 - page A <- A.md #0",
				"1 - asset z.png <- z.png #5",
				"2 A asset y.png <- A/y.png #4",
				"too_deep: A/B.md",
				"too_deep: A/B/C.md",
				"too_deep: A/B/x.png",
			}},
		{"at the deepest page", []string{"a.md", "b.png", "c/d.png"}, domain.ImportMeta{}, domain.MaxDepth, []string{
			"1 - asset b.png <- b.png #1",
			"too_deep: a.md",
			"too_deep: c/",
			"too_deep: c/d.png",
		}},
		{"nothing", nil, domain.ImportMeta{}, 0, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := planned(domain.NewImportPlan(importEntries(tt.paths...), tt.meta, tt.depth)); !slices.Equal(got, tt.want) {
				t.Errorf("NewImportPlan(%q) =\n%s\nwant\n%s", tt.paths, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// allocated is the bytes f allocates.
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// The plan's paths are each entry's joined once: no level of a deep path
// copies it (M7 closeout A-I1). A thousand attachments, each under ten
// folders of its own named with a thousand bytes, hold 10 MB of names;
// copying each folder's path at each level allocated 160 MB, sixteen times
// that, and the plan now 45 MB, the names mended taking most.
func TestThePlanAllocatesASmallMultipleOfItsPaths(t *testing.T) {
	var paths []string
	names := 0
	for i := range 1000 {
		var b strings.Builder
		for level := range 10 {
			fmt.Fprintf(&b, "%04d-%d-%s/", i, level, strings.Repeat("n", 994))
		}
		b.WriteString("x.png")
		paths = append(paths, b.String())
		names += b.Len()
	}
	entries := importEntries(paths...)

	var plan domain.ImportPlan
	bytes := allocated(func() { plan = domain.NewImportPlan(entries, domain.ImportMeta{}, 0) })

	if len(plan.Nodes) != 11_000 || len(plan.Skipped) != 0 {
		t.Fatalf("%d nodes, %d skipped; want 11,000 and none", len(plan.Nodes), len(plan.Skipped))
	}
	if bytes > uint64(6*names) {
		t.Errorf("the plan allocated %d bytes for %d bytes of names; want 6 times at most", bytes, names)
	}
}

// An entry of tens of thousands of folders is skipped as it is classified,
// by its count of names, before the plan makes a folder (M7 closeout A-I1):
// two of 64 KiB took 4.46 GiB of the heap, the plan's folders each holding
// its whole path; a thousand, Classify keeping each of their names, 2.5
// GiB (the fix check, FA-I1). Of each entry the first names are kept.
func TestAnEntryFarTooDeepIsSkippedBeforeItsFolders(t *testing.T) {
	deep := "t/" + strings.Repeat("d/", 32_760)
	raw := []domain.RawEntry{{Index: 0, Name: "ok.md", Method: domain.MethodDeflate}}
	for i := 1; i <= 1000; i++ {
		raw = append(raw, domain.RawEntry{Index: i, Name: fmt.Sprintf("%sx%04d.md", deep, i), Method: domain.MethodDeflate})
	}

	var plan domain.ImportPlan
	var sorted domain.Sorted
	bytes := allocated(func() {
		sorted = domain.Classify(raw)
		plan = domain.NewImportPlan(sorted.Entries, domain.ImportMeta{}, 0)
	})

	if got := planned(plan); !slices.Equal(got, []string{"1 - page ok <- ok.md #0"}) {
		t.Errorf("planned %q; want ok.md alone", got)
	}
	tooDeep := func(p domain.Problem) bool { return p.Code == domain.ProblemTooDeep }
	if len(sorted.Skipped) != 1000 || !slices.ContainsFunc(sorted.Skipped, tooDeep) || slices.IndexFunc(sorted.Skipped, func(p domain.Problem) bool { return !tooDeep(p) }) >= 0 {
		t.Errorf("skipped %d; want the thousand, too deep", len(sorted.Skipped))
	}
	if bytes > 8<<20 {
		t.Errorf("classified and planned in %d bytes; want 8 MiB at most", bytes)
	}
}

// meta.json gives the order of each node by its path, in NFC, and the
// contributors' files; one that is not JSON, or of another format, gives
// nothing.
func TestReadMeta(t *testing.T) {
	m := domain.ReadMeta([]byte(`{"format": 1, "nodes": [{"path": "Cafe\u0301.md", "kind": "page", "sort_order": 2.5},
		{"path": "A/", "kind": "page", "sort_order": -1}], "contributed": ["index.md"], "extra": true}`))
	if m.Order["Café.md"] != 2.5 || m.Order["A/"] != -1 || len(m.Order) != 2 || !m.Contributed["index.md"] {
		t.Errorf("ReadMeta() = %+v", m)
	}
	for _, data := range []string{`{"format": 2, "nodes": [{"path": "a.md", "sort_order": 1}]}`, `not json`, `{"format": 1, "nodes": 3}`} {
		if m := domain.ReadMeta([]byte(data)); len(m.Order) != 0 || len(m.Contributed) != 0 {
			t.Errorf("ReadMeta(%s) = %+v, want nothing", data, m)
		}
	}
}

// An import's job is named as its file, mended as an attachment's name
// is; Untitled for none.
func TestImportName(t *testing.T) {
	for file, want := range map[string]string{
		"vault.zip": "vault.zip", "a:b.zip": "a_b.zip", "  ": domain.Untitled, "": domain.Untitled, "con.zip": "con_.zip", "x/y.zip": "x_y.zip",
	} {
		if got := domain.ImportName(file); got != want {
			t.Errorf("ImportName(%q) = %q, want %q", file, got, want)
		}
	}
}
