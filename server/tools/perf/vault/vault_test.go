package vault

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// importLimitVault is the import-limit vault (design 3.4.3, as 3.12 brought
// it within an import's 45,000 nodes): S3's, 45,000 pages, 5,000
// attachments and -fanout 10,10,98, at three quarters, 33,750 pages, 3,750
// attachments of 800 KiB, 7,410 folders, 30 links and 2 KiB a page,
// -random 12: 44,910 nodes.
func importLimitVault() Spec {
	return Spec{Pages: 33_750, Attachments: 3_750, Fanout: []int{10, 10, 73}, Links: 30, Body: 2 << 10, AttachmentBytes: 800 << 10,
		Random: 12, Seed: 1201}
}

// s3Vault is the vault S3 measured at the import's limits of then (design
// 3.4.3 before 3.12), 59,910 nodes: P4/S7 imports it in two parts.
func s3Vault() Spec {
	return Spec{Pages: 45_000, Attachments: 5_000, Fanout: []int{10, 10, 98}, Links: 30, Body: 2 << 10, AttachmentBytes: 800 << 10,
		Random: 12, Seed: 1201}
}

// hundredth is the import's limits' hundredth, and the import-limit
// vault's: 338 pages, 37 attachments of the same size, 73 folders, of
// which nine of the second level, as the vault's tens hold its pages'
// links, 448 nodes.
func hundredth() (Spec, Limits) {
	return Spec{Pages: 338, Attachments: 37, Fanout: []int{1, 9, 7}, Links: 30, Body: 2 << 10, AttachmentBytes: 800 << 10,
			Random: 12, Seed: 1201},
		Limits{Entries: 500, Nodes: 450, Unpacked: (4 << 30) / 100, Zip: (512 << 20) / 100}
}

// The import's limits are the server's defaults (config.yaml's transfer,
// page.MaxJobItems); the import-limit vault is within them at full size,
// its nodes within 90 of the import's, as S3's of then (S3's own is past
// them: TestEachPartIsWithinAnImport). Counted, not written: contentsOf
// is what Write checks before it writes.
func TestTheImportLimitVaultIsWithinAnImport(t *testing.T) {
	lim := ImportLimits()
	if lim != (Limits{Entries: 50_000, Nodes: 45_000, Unpacked: 4 << 30, Zip: 512 << 20}) {
		t.Errorf("ImportLimits() = %+v, want transfer's defaults and page.MaxJobItems, 45,000 nodes", lim)
	}
	c, err := contentsOf(importLimitVault())
	if err != nil {
		t.Fatal(err)
	}
	r := c.res
	if r.Entries != 37_500 || r.Folders != 7_410 || r.Unpacked != 33_750*(2<<10)+3_750*(800<<10) {
		t.Errorf("the import-limit vault %s, want 37,500 entries, 7,410 folders, %d bytes", r, 33_750*(2<<10)+3_750*(800<<10))
	}
	if n := r.Entries + r.Folders; n > lim.Nodes || n < lim.Nodes-90 || r.Entries > lim.Entries || r.Unpacked > lim.Unpacked {
		t.Errorf("the import-limit vault %s: want within %+v, its nodes within 90 of them", r, lim)
	}
}

// The import's limits are the server's (M12/P4/S6 review A-M1): its nodes
// page.MaxJobItems, the rest config.yaml's transfer defaults, read from
// their text, as server/tools is a module of its own and does not import
// the server's. A change of the constant with its test fails here too; a
// line not found, or not found once, fails, not skips.
func TestImportLimitsAreTheServers(t *testing.T) {
	const jobs, config = "../../../internal/modules/page/domain/jobs.go", "../../../configs/config.yaml"
	value := func(path, pattern string) int64 {
		t.Helper()
		b, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatalf("the server's %s: %v", path, err)
		}
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(string(b), -1)
		if len(m) != 1 {
			t.Fatalf("%s: %d lines match %s, want one", path, len(m), pattern)
		}
		n, err := strconv.ParseInt(strings.ReplaceAll(m[0][1], "_", ""), 10, 64)
		if err != nil {
			t.Fatalf("%s: %s: %v", path, m[0][0], err)
		}
		return n
	}
	server := Limits{
		Entries:  int(value(config, `(?m)^\s+import_max_entries: ([0-9]+)$`)),
		Nodes:    int(value(jobs, `(?m)^const MaxJobItems = ([0-9_]+)$`)),
		Unpacked: value(config, `(?m)^\s+import_max_unpacked_bytes: ([0-9]+)$`),
		Zip:      value(config, `(?m)^\s+import_max_bytes: ([0-9]+)$`),
	}
	if got := ImportLimits(); got != server {
		t.Errorf("ImportLimits() = %+v, the server's %+v (page.MaxJobItems, config.yaml's transfer)", got, server)
	}
}

// The import-limit shape at a hundredth: within the limits at a hundredth,
// the entries, folders and bytes the full shape has at a hundredth
// (2.93 GiB unpacked; a zip of 351,272,583 bytes, 335.0 MiB, as S6 wrote
// it at full size once), each link a page of the vault without a path and
// not the page itself, nine in ten in the page's folder of the second
// level (design 3.4.3, 7.2's 35).
func TestTheImportLimitVaultAtAHundredth(t *testing.T) {
	s, lim := hundredth()
	path := filepath.Join(t.TempDir(), "vault.zip")
	r, err := Write(path, s, lim)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", r)
	if r.Entries != 375 || r.Folders != 73 || r.Entries+r.Folders > lim.Nodes || r.Unpacked > lim.Unpacked || r.Zip > lim.Zip {
		t.Errorf("%s: past the hundredth's limits %+v", r, lim)
	}
	const unpacked, zipped = (33_750*(2<<10) + 3_750*(800<<10)) / 100.0, 351_272_583.0 / 100
	if u := float64(r.Unpacked); u < unpacked*0.97 || u > unpacked*1.03 {
		t.Errorf("unpacked %d, want about %.0f", r.Unpacked, unpacked)
	}
	if z := float64(r.Zip); z < zipped*0.85 || z > zipped*1.15 {
		t.Errorf("zipped %d, want about %.0f", r.Zip, zipped)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != r.Zip {
		t.Errorf("the zip on disk %v, %v; want %d bytes", info, err, r.Zip)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	pages := map[string]string{} // name to its second-level folder
	bodies := map[string]string{}
	attachments := 0
	for _, f := range zr.File {
		dirs := strings.Split(f.Name, "/")
		if len(dirs) != 4 || f.Method != zip.Deflate || strings.HasSuffix(f.Name, "/") {
			t.Fatalf("entry %q: want three folders deep, deflated, no folder entries", f.Name)
		}
		group := dirs[0] + "/" + dirs[1]
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasSuffix(f.Name, ".md"):
			name := strings.TrimSuffix(dirs[3], ".md")
			if _, dup := pages[name]; dup {
				t.Fatalf("page %q twice", name)
			}
			pages[name], bodies[name] = group, string(b)
			if len(b) != s.Body {
				t.Errorf("page %q of %d bytes, want %d", name, len(b), s.Body)
			}
		case strings.HasSuffix(f.Name, ".bin"):
			attachments++
			if len(b) != s.AttachmentBytes {
				t.Errorf("attachment %q of %d bytes", f.Name, len(b))
			}
		default:
			t.Errorf("entry %q", f.Name)
		}
	}
	if len(pages) != s.Pages || attachments != s.Attachments {
		t.Fatalf("%d pages, %d attachments", len(pages), attachments)
	}
	link := regexp.MustCompile(`\[\[([^\]]*)\]\]`)
	near, all, self := 0, 0, 0
	for name, body := range bodies {
		links := link.FindAllStringSubmatch(body, -1)
		if len(links) != s.Links {
			t.Fatalf("page %q has %d links, want %d", name, len(links), s.Links)
		}
		for _, l := range links {
			group, ok := pages[l[1]]
			if !ok || strings.Contains(l[1], "/") {
				t.Fatalf("page %q links %q: no page of the vault, or a path", name, l[1])
			}
			if l[1] == name {
				self++
			}
			all++
			if group == pages[name] {
				near++
			}
		}
	}
	// Nine in ten by the draw, and a tenth of the rest landing in the page's
	// own of nine second-level folders: about 0.9 + 0.1/9.
	if share := float64(near) / float64(all); share < 0.88 || share > 0.94 {
		t.Errorf("%d of %d links in the page's second-level folder, %.3f; want about 0.91", near, all, share)
	}
	// A link of the page's folder is never the page itself; one of the
	// whole vault is, one in its pages: about 30 × 338 / 10 / 338 = 3. Were
	// the folder's links drawn at all, about 30 × 338 × 0.9 / 37.6 = 243.
	t.Logf("%d links of a page to itself", self)
	if self > 10 {
		t.Errorf("%d links of a page to itself, want about 3: the folder's links are another page", self)
	}
}

// A seed writes the same zip; another seed another; past a limit nothing
// is written: the entries, nodes and bytes before the zip, the zip's own
// after it, its temporary file gone (design 3.4.3, 7.2's 32 to 34).
func TestAVaultIsItsSeedAndItsLimits(t *testing.T) {
	small := Spec{Pages: 120, Attachments: 6, Fanout: []int{2, 3, 4}, Links: 5, Body: 512, AttachmentBytes: 16 << 10, Random: 4, Seed: 7}
	dir := t.TempDir()
	write := func(name string, s Spec, lim Limits) (Result, error) { return Write(filepath.Join(dir, name), s, lim) }
	a, err := write("a.zip", small, ImportLimits())
	if err != nil {
		t.Fatal(err)
	}
	b, err := write("b.zip", small, ImportLimits())
	if err != nil || b.SHA256 != a.SHA256 || b.Zip != a.Zip {
		t.Errorf("the same seed: %s and %s, %v", a, b, err)
	}
	other := small
	other.Seed = 8
	if c, err := write("c.zip", other, ImportLimits()); err != nil || c.SHA256 == a.SHA256 {
		t.Errorf("another seed: %s, %v", c, err)
	}
	if a.Entries != 126 || a.Folders != 2+6+24 || a.Unpacked != 120*512+6*(16<<10) {
		t.Errorf("small %s", a)
	}
	// Fewer pages than the deepest folders: some folders would be counted
	// that the import does not make.
	few := small
	few.Pages = 23
	if r, err := write("few.zip", few, ImportLimits()); err == nil {
		t.Errorf("23 pages in 24 deepest folders: %s", r)
	}
	// Two writes within a second are alike whatever their entries' times
	// (a zip keeps them to the second, its DOS fields to two): each entry
	// has the one time, so that a write a day later is the same zip.
	zr, err := zip.OpenReader(filepath.Join(dir, "a.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if !f.Modified.Equal(modified()) {
			t.Fatalf("%s's time %v, want %v", f.Name, f.Modified, modified())
		}
	}
	for _, c := range []struct {
		name string
		lim  Limits
	}{
		{"entries", Limits{Entries: 125, Nodes: 1000, Unpacked: 1 << 30, Zip: 1 << 30}},
		{"nodes", Limits{Entries: 1000, Nodes: 157, Unpacked: 1 << 30, Zip: 1 << 30}},
		{"unpacked", Limits{Entries: 1000, Nodes: 1000, Unpacked: a.Unpacked - 1, Zip: 1 << 30}},
		{"zip", Limits{Entries: 1000, Nodes: 1000, Unpacked: 1 << 30, Zip: a.Zip - 1}},
	} {
		if r, err := write(c.name+".zip", small, c.lim); err == nil {
			t.Errorf("past its %s: %s", c.name, r)
		}
		if _, err := os.Stat(filepath.Join(dir, c.name+".zip")); !os.IsNotExist(err) {
			t.Errorf("past its %s, the zip is there: %v", c.name, err)
		}
	}
	left, err := filepath.Glob(filepath.Join(dir, ".*"))
	if err != nil || len(left) != 0 {
		t.Errorf("temporary files left: %v, %v", left, err)
	}
	if _, err := write("bad.zip", Spec{Pages: 1, Body: 1, Random: 0}, ImportLimits()); err == nil {
		t.Error("a -random of 0 wrote")
	}
}

// partSpec is a vault of five folders of the first level, which two parts
// cut 2 and 3 and three parts 1, 2 and 2; its pages and attachments in
// every deepest folder, some with two attachments, its pages' links of
// the whole vault a tenth.
func partSpec() Spec {
	return Spec{Pages: 120, Attachments: 37, Fanout: []int{5, 2, 3}, Links: 6, Body: 300, AttachmentBytes: 5 << 10, Random: 3, Seed: 41}
}

// contentOf is the SHA-256 of what a zip holds, not of how deflate wrote
// it: each entry in the zip's order, its path, a zero byte, its unpacked
// length in decimal, a zero byte and its unpacked bytes.
func contentOf(t *testing.T, path string) string {
	t.Helper()
	entries, order := entriesOf(t, path)
	h := sha256.New()
	for _, p := range order {
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", p, len(entries[p]))
		_, _ = h.Write(entries[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// entriesOf is the zip's entries: each one's bytes by its path, and the
// paths in the zip's order.
func entriesOf(t *testing.T, path string) (map[string][]byte, []string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	out, order := map[string][]byte{}, []string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := out[f.Name]; dup {
			t.Fatalf("%s twice in %s", f.Name, path)
		}
		out[f.Name], order = b, append(order, f.Name)
	}
	return out, order
}

// The parts of a vault are the vault (design 3.12): the entries of its n
// parts, together, are the whole vault's, path by path and byte by byte,
// none in two parts; each part's in the whole vault's order, its folders
// of the first level a run, in order, the part's entries and folders the
// count Write tells. Its pages link the whole vault's pages: a page of one
// part links pages of another, by the name the whole vault gives them.
func TestTheVaultsPartsAreTheVault(t *testing.T) {
	s, dir := partSpec(), t.TempDir()
	if _, err := Write(filepath.Join(dir, "whole.zip"), s, ImportLimits()); err != nil {
		t.Fatal(err)
	}
	whole, order := entriesOf(t, filepath.Join(dir, "whole.zip"))
	names := map[string]string{} // a page's name to its folder of the first level
	for path := range whole {
		if name, ok := strings.CutSuffix(path[strings.LastIndex(path, "/")+1:], ".md"); ok {
			names[name] = path[:strings.Index(path, "/")]
		}
	}
	link := regexp.MustCompile(`\[\[([^\]]*)\]\]`)
	for n, runs := range map[int]map[string]int{
		2: {"f1": 1, "f2": 1, "f3": 2, "f4": 2, "f5": 2},
		3: {"f1": 1, "f2": 2, "f3": 2, "f4": 3, "f5": 3},
	} {
		union, tops, across := map[string][]byte{}, map[string]int{}, 0
		for k := 1; k <= n; k++ {
			s.Part = Part{K: k, N: n}
			path := filepath.Join(dir, fmt.Sprintf("%dof%d.zip", k, n))
			r, err := Write(path, s, ImportLimits())
			if err != nil {
				t.Fatalf("part %s: %v", s.Part, err)
			}
			part, partOrder := entriesOf(t, path)
			folders := map[string]bool{}
			for p := range part {
				for i := range len(p) {
					if p[i] == '/' {
						folders[p[:i+1]] = true
					}
				}
			}
			if len(part) == 0 || r.Entries != len(part) || r.Folders != len(folders) {
				t.Errorf("part %s: %s, its zip %d entries, %d folders", s.Part, r, len(part), len(folders))
			}
			if want := slices.DeleteFunc(slices.Clone(order), func(p string) bool { return part[p] == nil }); !slices.Equal(partOrder, want) {
				t.Errorf("part %s's entries %v, want the whole vault's order %v", s.Part, partOrder, want)
			}
			for p, b := range part {
				if _, dup := union[p]; dup {
					t.Errorf("%s in two parts of %d", p, n)
				}
				union[p] = b
				top := p[:strings.Index(p, "/")]
				if was, ok := tops[top]; ok && was != k {
					t.Errorf("%s of the first level in parts %d and %d of %d", top, was, k, n)
				}
				tops[top] = k
				for _, l := range link.FindAllStringSubmatch(string(b), -1) {
					if names[l[1]] == "" {
						t.Fatalf("%s links %q, no page of the whole vault", p, l[1])
					}
					if runs[names[l[1]]] != k {
						across++
					}
				}
			}
		}
		if !maps.Equal(tops, runs) {
			t.Errorf("%d parts hold the folders of the first level %v, want %v", n, tops, runs)
		}
		if len(union) != len(whole) {
			t.Errorf("%d parts hold %d entries, the whole vault %d", n, len(union), len(whole))
		}
		for p, b := range whole {
			if !bytes.Equal(union[p], b) {
				t.Errorf("%s in %d parts: %d bytes unlike the whole vault's %d", p, n, len(union[p]), len(b))
			}
		}
		t.Logf("%d parts: %d links to a page of another part", n, across)
		if across == 0 {
			t.Errorf("%d parts: no link to a page of another part", n)
		}
	}
}

// A part of one is the whole vault, the zip byte for byte (the CLI's -part
// 1/1 and no -part, design 3.12), and the whole vault still holds what
// S1's generator wrote from the seed, so that S3's records' vaults are
// still the ones a seed writes (contentOf of the generator's zips before
// the parts, which another Go's deflate does not change; M12/P4/S6 review
// B-N1): with folders, and with every entry at the root, where the vault
// is one part.
func TestAWholePartIsTheWholeVault(t *testing.T) {
	root := Spec{Pages: 9, Attachments: 2, Links: 2, Body: 200, AttachmentBytes: 1 << 10, Random: 2, Seed: 5}
	dir := t.TempDir()
	for _, c := range []struct {
		name, content string
		s             Spec
	}{
		{"folders", "aa4bd4b4adf84bafdabaf61d3ac7651c571dc51d9a52381b29d808445bf33888", partSpec()},
		{"root", "918237ea0784b33b9d1c3da8cbf2cb8ca9e50dd55764416da47fa2b06310e325", root},
	} {
		whole, err := Write(filepath.Join(dir, c.name+".zip"), c.s, ImportLimits())
		if err != nil {
			t.Fatal(err)
		}
		c.s.Part = Part{K: 1, N: 1}
		one, err := Write(filepath.Join(dir, c.name+"-1of1.zip"), c.s, ImportLimits())
		if err != nil {
			t.Fatal(err)
		}
		if one != whole {
			t.Errorf("%s: part 1/1 %s, the whole %s; want the same zip", c.name, one, whole)
		}
		if got := contentOf(t, filepath.Join(dir, c.name+".zip")); got != c.content {
			t.Errorf("%s: the whole vault holds %s, the generator's %s", c.name, got, c.content)
		}
	}
	root.Part = Part{K: 1, N: 2}
	if r, err := Write(filepath.Join(dir, "root-1of2.zip"), root, ImportLimits()); err == nil {
		t.Errorf("part 1/2 of a vault without folders: %s", r)
	}
}

// Each part is within an import when the whole is not: S3's vault in two,
// counted at full size, each within ImportLimits() and together the whole,
// as S3 wrote it (50,000 entries, 9,910 folders, 4,188,160,000 bytes
// unpacked), past the import's nodes (P4/S7's two imports); and Write
// checks the part's entries and nodes, not the whole's, against its
// limits (design 3.12).
func TestEachPartIsWithinAnImport(t *testing.T) {
	lim, s := ImportLimits(), s3Vault()
	whole := Result{Entries: 50_000, Folders: 9_910, Unpacked: 4_188_160_000}
	if whole.Entries+whole.Folders <= lim.Nodes {
		t.Errorf("S3's vault %s within the import's %d nodes: no parts wanted", whole, lim.Nodes)
	}
	var sum Result
	for k, nodes := range []int{34_355, 25_555} {
		s.Part = Part{K: k + 1, N: 2}
		c, err := contentsOf(s)
		if err != nil {
			t.Fatal(err)
		}
		r := c.res
		t.Logf("S3's vault, part %s: %s", s.Part, r)
		if r.Entries+r.Folders != nodes || r.Entries+r.Folders > lim.Nodes || r.Entries > lim.Entries || r.Unpacked > lim.Unpacked {
			t.Errorf("S3's vault, part %s: %s; want %d nodes, within %+v", s.Part, r, nodes, lim)
		}
		sum.Entries, sum.Folders, sum.Unpacked = sum.Entries+r.Entries, sum.Folders+r.Folders, sum.Unpacked+r.Unpacked
	}
	if sum != whole {
		t.Errorf("S3's vault's two parts %s, the whole %s", sum, whole)
	}
	small, dir := partSpec(), t.TempDir()
	most := 0
	for k := 1; k <= 2; k++ {
		small.Part = Part{K: k, N: 2}
		c, err := contentsOf(small)
		if err != nil {
			t.Fatal(err)
		}
		most = max(most, c.res.Entries+c.res.Folders)
	}
	tight := Limits{Entries: most, Nodes: most, Unpacked: 1 << 30, Zip: 1 << 30}
	small.Part = Part{}
	if r, err := Write(filepath.Join(dir, "whole.zip"), small, tight); err == nil {
		t.Errorf("the whole vault within %+v: %s", tight, r)
	}
	for k := 1; k <= 2; k++ {
		small.Part = Part{K: k, N: 2}
		if r, err := Write(filepath.Join(dir, fmt.Sprintf("%dof2.zip", k)), small, tight); err != nil {
			t.Errorf("part %s past %+v: %v, %s", small.Part, tight, err, r)
		}
	}
}

// A part is k of n, 1 <= k <= n, n at most the folders of the first level:
// -part's text read, and a Part set by hand checked before anything is
// written.
func TestAPartIsKOfN(t *testing.T) {
	for s, want := range map[string]Part{"1/1": {1, 1}, "1/2": {1, 2}, "2/2": {2, 2}, "3/10": {3, 10}} {
		if p, err := ParsePart(s); err != nil || p != want {
			t.Errorf("ParsePart(%q) = %v, %v; want %v", s, p, err, want)
		}
	}
	for _, s := range []string{"0/2", "3/2", "1/0", "0/0", "-1/2", "1/-1", "", "1", "/2", "1/", "a/2", "1/b", "1/2/3", " 1/2", "1 / 2"} {
		if p, err := ParsePart(s); err == nil {
			t.Errorf("ParsePart(%q) = %v, want an error", s, p)
		}
	}
	dir := t.TempDir()
	for _, p := range []Part{{0, 2}, {3, 2}, {1, 0}, {0, 1}, {1, 6}, {6, 6}} {
		s := partSpec()
		s.Part = p
		if r, err := Write(filepath.Join(dir, "bad.zip"), s, ImportLimits()); err == nil {
			t.Errorf("part %v of five folders of the first level: %s", p, r)
		}
		if _, err := os.Stat(filepath.Join(dir, "bad.zip")); !os.IsNotExist(err) {
			t.Errorf("part %v: the zip is there: %v", p, err)
		}
	}
	if left, err := filepath.Glob(filepath.Join(dir, ".*")); err != nil || len(left) != 0 {
		t.Errorf("temporary files left: %v, %v", left, err)
	}
}
