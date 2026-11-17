package vault

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hundredth is the import's limits' hundredth, and the import-limit vault's
// (design 3.4.3: 45,000 pages, 5,000 attachments of 800 KiB, 9,910
// folders, 30 links and 2 KiB a page, -random 12): 450 pages, 50
// attachments of the same size, 100 folders.
func hundredth() (Spec, Limits) {
	return Spec{Pages: 450, Attachments: 50, Fanout: []int{1, 9, 10}, Links: 30, Body: 2 << 10, AttachmentBytes: 800 << 10,
			Random: 12, Seed: 1201},
		Limits{Entries: 500, Nodes: 600, Unpacked: (4 << 30) / 100, Zip: (512 << 20) / 100}
}

// The import-limit shape at a hundredth: within the limits at a hundredth,
// the entries, folders and bytes the design's table has at a hundredth
// (3.90 GiB unpacked, a zip of about 433 MiB), each link a page of the
// vault without a path, nine in ten in the page's folder of the second
// level (design 3.4.3, 7.2's 35).
func TestTheImportLimitVaultAtAHundredth(t *testing.T) {
	s, lim := hundredth()
	path := filepath.Join(t.TempDir(), "vault.zip")
	r, err := Write(path, s, lim)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", r)
	if r.Entries != 500 || r.Folders != 100 || r.Entries+r.Folders > lim.Nodes || r.Unpacked > lim.Unpacked || r.Zip > lim.Zip {
		t.Errorf("%s: past the hundredth's limits %+v", r, lim)
	}
	const unpacked, zipped = 3.90 * (1 << 30) / 100, 433.0 * (1 << 20) / 100
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
	near, all := 0, 0
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
