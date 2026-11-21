// Package vault writes the vault an import measurement uploads (M12/P4
// design 3.4.3): a zip of a page a Markdown file and an attachment a file,
// its folders the directories of their paths, its links [[name]] without a
// path, all from a seed. It checks an import's limits before it writes, and
// the zip's own after. It writes the whole vault or a part of it (Part), so
// that a vault past an import's limits is imported in parts (M12/P4 design
// 3.12).
package vault

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Spec is a vault's shape: its pages and attachments, the folders of each
// level (10,10,98: ten at the root, ten in each, 98 in each of those), a
// page's links and bytes, an attachment's bytes, the part of an attachment
// that is random (1/Random; the rest one block of 4 KiB repeated), and the
// seed; and the part of the vault written, the whole vault when it is the
// zero Part.
type Spec struct {
	Pages, Attachments int
	Fanout             []int
	Links, Body        int
	AttachmentBytes    int
	Random             int
	Seed               uint64
	Part               Part
}

// Part is the K-th of N parts of a vault: its folders of the first level
// cut in N runs in their order, part K the folders from (K-1)F/N to KF/N
// of the F there, rounded down, 0-based, the upper end excluded, with
// every folder, page and attachment under them. Each is as the whole vault has it, its links to the whole
// vault's pages: the parts imported one after another under one page are
// the whole vault imported, a link to a page of a later part unresolved
// until that part's import (P4/S7 builds S3's top of 59,910 nodes so). The
// zero Part is the whole vault, as 1/1 is.
type Part struct{ K, N int }

// ParsePart reads a part, k/n: 1 <= k <= n.
func ParsePart(s string) (Part, error) {
	k, n, ok := strings.Cut(s, "/")
	pk, errK := strconv.Atoi(k)
	pn, errN := strconv.Atoi(n)
	if !ok || errK != nil || errN != nil || pk < 1 || pk > pn {
		return Part{}, fmt.Errorf("a part %q: want k/n, 1 <= k <= n", s)
	}
	return Part{K: pk, N: pn}, nil
}

func (p Part) String() string { return fmt.Sprintf("%d/%d", p.K, p.N) }

// holds is whether the part holds the folder of the first level top, of
// first: the zero Part each.
func (p Part) holds(top, first int) bool {
	if p == (Part{}) {
		return true
	}
	return (p.K-1)*first/p.N <= top && top < p.K*first/p.N
}

// Limits are what an import takes (server/configs/config.yaml's transfer
// and page.MaxJobItems): its entries, its nodes (the entries and the
// folders their paths imply), its unpacked bytes and its zip's.
type Limits struct {
	Entries, Nodes int
	Unpacked, Zip  int64
}

// ImportLimits are the defaults: 50,000 entries, 45,000 nodes
// (page.MaxJobItems, M12/P4/S6), 4 GiB unpacked, a zip of 512 MiB.
// TestImportLimitsAreTheServers reads them from the server's text.
func ImportLimits() Limits {
	return Limits{Entries: 50_000, Nodes: 45_000, Unpacked: 4 << 30, Zip: 512 << 20}
}

// Result is what was written: the entries, the folders, the bytes
// unpacked and zipped, and the zip's SHA-256.
type Result struct {
	Entries, Folders int
	Unpacked, Zip    int64
	SHA256           string
}

func (r Result) String() string {
	return fmt.Sprintf("entries=%d folders=%d nodes=%d unpacked_bytes=%d zip_bytes=%d sha256=%s", r.Entries, r.Folders,
		r.Entries+r.Folders, r.Unpacked, r.Zip, r.SHA256)
}

// modified is every entry's time, so that a seed's zip is the same zip.
func modified() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// The streams of the seed's generators, one a purpose and an item.
const (
	streamNames uint64 = 1 << 32 * (iota + 1)
	streamLinks
	streamText
	streamAttachment
	streamBlock
)

// layout is a spec's folders and where each item goes.
type layout struct {
	s         Spec
	folders   []string // every folder's path, parents first
	folderTop []int    // each folder's of the first level: its index there
	leaves    []string // the deepest folders, where the items go
	group     []int    // each leaf's folder of the second level, or of the first of one level: its index there
	top       []int    // each leaf's folder of the first level, 0 at the root: its index there
	first     int      // the folders of the first level, 1 of none
	names     []string // each page's name
	byGroup   map[int][]int
}

func newLayout(s Spec) layout {
	l := layout{s: s, first: 1, byGroup: map[int][]int{}}
	if len(s.Fanout) > 0 {
		l.first = s.Fanout[0]
	}
	level := []string{""}
	groupOf, topOf := []int{0}, []int{0}
	for depth, n := range s.Fanout {
		var next []string
		var nextGroup, nextTop []int
		for i, parent := range level {
			for k := range n {
				p := fmt.Sprintf("%sf%d/", parent, k+1)
				next = append(next, p)
				if depth == 0 {
					nextTop = append(nextTop, k)
				} else {
					nextTop = append(nextTop, topOf[i])
				}
				l.folders, l.folderTop = append(l.folders, p), append(l.folderTop, nextTop[len(nextTop)-1])
				if depth < 2 {
					nextGroup = append(nextGroup, len(next)-1) // a folder of the first two levels is its own group
				} else {
					nextGroup = append(nextGroup, groupOf[i]) // deeper, its parent's
				}
			}
		}
		level, groupOf, topOf = next, nextGroup, nextTop
	}
	l.leaves, l.group, l.top = level, groupOf, topOf
	l.names = make([]string, s.Pages)
	for i := range s.Pages {
		r := rand.New(rand.NewPCG(s.Seed, streamNames|uint64(i)))
		l.names[i] = fmt.Sprintf("p%d %s %s", i+1, word(r), word(r))
		g := l.group[i%len(l.leaves)]
		l.byGroup[g] = append(l.byGroup[g], i)
	}
	return l
}

// word is a word of three to nine letters.
func word(r *rand.Rand) string {
	b := make([]byte, 3+r.IntN(7))
	for i := range b {
		b[i] = byte('a' + r.IntN(26))
	}
	return string(b)
}

// pagePath is the page i's path in the zip.
func (l layout) pagePath(i int) string { return l.leaves[i%len(l.leaves)] + l.names[i] + ".md" }

// attachmentPath is the attachment j's.
func (l layout) attachmentPath(j int) string {
	return fmt.Sprintf("%sa%d.bin", l.leaves[j%len(l.leaves)], j+1)
}

// targets are the page i's links' targets: nine in ten a page of its
// folder of the second level, another than itself when there is one; the
// rest any page of the vault.
func (l layout) targets(i int) []int {
	r := rand.New(rand.NewPCG(l.s.Seed, streamLinks|uint64(i)))
	near := l.byGroup[l.group[i%len(l.leaves)]]
	out := make([]int, l.s.Links)
	for k := range out {
		if r.IntN(10) < 9 {
			t := near[r.IntN(len(near))]
			if t == i && len(near) > 1 {
				t = near[(r.IntN(len(near)-1)+1+indexOf(near, i))%len(near)]
			}
			out[k] = t
		} else {
			out[k] = r.IntN(l.s.Pages)
		}
	}
	return out
}

func indexOf(xs []int, x int) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// page is the page i's Markdown: its title, its links, a line each, then
// words to its bytes.
func (l layout) page(i int) []byte {
	var b strings.Builder
	b.WriteString("# " + l.names[i] + "\n\n")
	for _, t := range l.targets(i) {
		b.WriteString("[[" + l.names[t] + "]]\n")
	}
	b.WriteString("\n")
	r := rand.New(rand.NewPCG(l.s.Seed, streamText|uint64(i)))
	for b.Len() < l.s.Body-1 {
		w := word(r)
		if b.Len()+len(w)+1 > l.s.Body-1 {
			w = w[:l.s.Body-1-b.Len()-1]
		}
		b.WriteString(w + " ")
	}
	b.WriteString("\n")
	return []byte(b.String())
}

// attachment is the attachment j's bytes: its first part random, the rest
// block repeated.
func (l layout) attachment(j int, block []byte) []byte {
	out := make([]byte, l.s.AttachmentBytes)
	random := l.s.AttachmentBytes / l.s.Random
	r := rand.New(rand.NewPCG(l.s.Seed, streamAttachment|uint64(j)))
	for i := 0; i < random; i += 8 {
		v := r.Uint64()
		for k := 0; k < 8 && i+k < random; k++ {
			out[i+k] = byte(v >> (8 * k))
		}
	}
	for i := random; i < len(out); i += len(block) {
		copy(out[i:], block)
	}
	return out
}

// check is the spec's own sense.
func (s Spec) check() error {
	switch {
	case s.Pages < 1 || s.Attachments < 0 || s.Links < 0 || s.Body < 1 || s.Random < 1:
		return errors.New("a vault wants a page at least, a body, and -random of 1 at least")
	case s.Attachments > 0 && s.AttachmentBytes < 1:
		return errors.New("attachments want their bytes")
	}
	if s.Part != (Part{}) {
		first := 1
		if len(s.Fanout) > 0 {
			first = s.Fanout[0]
		}
		if s.Part.K < 1 || s.Part.K > s.Part.N || s.Part.N > first {
			return fmt.Errorf("a part %s of %d folders of the first level: want 1 <= k <= n, a folder a part at least", s.Part, first)
		}
	}
	leaves := 1
	for _, n := range s.Fanout {
		if n < 1 {
			return fmt.Errorf("a fanout of %v", s.Fanout)
		}
		leaves *= n
	}
	// A folder without an entry is no folder the import makes: each leaf a
	// page at least, so that the folders counted are the nodes it makes
	// (review B's N4).
	if s.Pages < leaves {
		return fmt.Errorf("%d pages for the %d deepest folders of %v: want a page each at least", s.Pages, leaves, s.Fanout)
	}
	return nil
}

// contents are what s's vault, or its part, holds: its pages and
// attachments by number, in the whole vault's order, and its entries,
// folders and bytes unpacked.
type contents struct {
	l                  layout
	pages, attachments []int
	res                Result
}

// contentsOf is s's contents, s checked.
func contentsOf(s Spec) (contents, error) {
	if err := s.check(); err != nil {
		return contents{}, err
	}
	l := newLayout(s)
	c := contents{l: l}
	held := func(item int) bool { return s.Part.holds(l.top[item%len(l.leaves)], l.first) }
	for i := range s.Pages {
		if held(i) {
			c.pages = append(c.pages, i)
			c.res.Unpacked += int64(len(l.page(i)))
		}
	}
	for j := range s.Attachments {
		if held(j) {
			c.attachments = append(c.attachments, j)
		}
	}
	c.res.Unpacked += int64(len(c.attachments)) * int64(s.AttachmentBytes)
	c.res.Entries = len(c.pages) + len(c.attachments)
	for _, top := range l.folderTop {
		if s.Part.holds(top, l.first) {
			c.res.Folders++
		}
	}
	return c, nil
}

// Write writes s's vault, or its part, to path, within lim: the entries,
// the nodes and the bytes unpacked first, and nothing written past them;
// then the zip into a temporary file beside path, removed and an error
// when it is past lim.Zip, else renamed to path.
func Write(path string, s Spec, lim Limits) (Result, error) {
	c, err := contentsOf(s)
	if err != nil {
		return Result{}, err
	}
	l, res := c.l, c.res
	switch {
	case res.Entries > lim.Entries:
		return Result{}, fmt.Errorf("%d entries, past the import's %d", res.Entries, lim.Entries)
	case res.Entries+res.Folders > lim.Nodes:
		return Result{}, fmt.Errorf("%d entries and %d folders, past the import's %d nodes", res.Entries, res.Folders, lim.Nodes)
	case res.Unpacked > lim.Unpacked:
		return Result{}, fmt.Errorf("%d bytes unpacked, past the import's %d", res.Unpacked, lim.Unpacked)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return Result{}, err
	}
	written := false
	defer func() {
		if !written {
			_ = os.Remove(tmp.Name())
		}
	}()
	hash := sha256.New()
	counted := &counter{}
	zw := zip.NewWriter(io.MultiWriter(tmp, hash, counted))
	add := func(name string, b []byte) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified()})
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	for _, i := range c.pages {
		if err := add(l.pagePath(i), l.page(i)); err != nil {
			_ = tmp.Close()
			return Result{}, err
		}
	}
	block := make([]byte, 4<<10)
	br := rand.New(rand.NewPCG(s.Seed, streamBlock))
	for i := range block {
		block[i] = byte(br.IntN(256))
	}
	for _, j := range c.attachments {
		if err := add(l.attachmentPath(j), l.attachment(j, block)); err != nil {
			_ = tmp.Close()
			return Result{}, err
		}
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return Result{}, err
	}
	if err := tmp.Close(); err != nil {
		return Result{}, err
	}
	res.Zip, res.SHA256 = counted.n, hex.EncodeToString(hash.Sum(nil))
	if res.Zip > lim.Zip {
		return Result{}, fmt.Errorf("a zip of %d bytes, past the import's %d: removed", res.Zip, lim.Zip)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return Result{}, err
	}
	written = true
	return res, nil
}

// counter counts the bytes written through it.
type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
