// Package vault writes the vault an import measurement uploads (M12/P4
// design 3.4.3): a zip of a page a Markdown file and an attachment a file,
// its folders the directories of their paths, its links [[name]] without a
// path, all from a seed. It checks an import's limits before it writes, and
// the zip's own after.
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
	"strings"
	"time"
)

// Spec is a vault's shape: its pages and attachments, the folders of each
// level (10,10,98: ten at the root, ten in each, 98 in each of those), a
// page's links and bytes, an attachment's bytes, the part of an attachment
// that is random (1/Random; the rest one block of 4 KiB repeated), and the
// seed.
type Spec struct {
	Pages, Attachments int
	Fanout             []int
	Links, Body        int
	AttachmentBytes    int
	Random             int
	Seed               uint64
}

// Limits are what an import takes (server/configs/config.yaml's transfer
// and page.MaxJobItems): its entries, its nodes (the entries and the
// folders their paths imply), its unpacked bytes and its zip's.
type Limits struct {
	Entries, Nodes int
	Unpacked, Zip  int64
}

// ImportLimits are the defaults: 50,000 entries, 60,000 nodes, 4 GiB
// unpacked, a zip of 512 MiB.
func ImportLimits() Limits {
	return Limits{Entries: 50_000, Nodes: 60_000, Unpacked: 4 << 30, Zip: 512 << 20}
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
	s       Spec
	folders []string // every folder's path, parents first
	leaves  []string // the deepest folders, where the items go
	group   []int    // each leaf's folder of the second level, or of the first of one level: its index there
	names   []string // each page's name
	byGroup map[int][]int
}

func newLayout(s Spec) layout {
	l := layout{s: s, byGroup: map[int][]int{}}
	level := []string{""}
	groupOf := []int{0}
	for depth, n := range s.Fanout {
		var next []string
		var nextGroup []int
		for i, parent := range level {
			for k := range n {
				p := fmt.Sprintf("%sf%d/", parent, k+1)
				next = append(next, p)
				l.folders = append(l.folders, p)
				if depth < 2 {
					nextGroup = append(nextGroup, len(next)-1) // a folder of the first two levels is its own group
				} else {
					nextGroup = append(nextGroup, groupOf[i]) // deeper, its parent's
				}
			}
		}
		level, groupOf = next, nextGroup
	}
	l.leaves, l.group = level, groupOf
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
	for _, n := range s.Fanout {
		if n < 1 {
			return fmt.Errorf("a fanout of %v", s.Fanout)
		}
	}
	return nil
}

// Write writes s's vault to path, within lim: the entries, the nodes and
// the bytes unpacked first, and nothing written past them; then the zip
// into a temporary file beside path, removed and an error when it is
// past lim.Zip, else renamed to path.
func Write(path string, s Spec, lim Limits) (Result, error) {
	if err := s.check(); err != nil {
		return Result{}, err
	}
	l := newLayout(s)
	res := Result{Entries: s.Pages + s.Attachments, Folders: len(l.folders)}
	for i := range s.Pages {
		res.Unpacked += int64(len(l.page(i)))
	}
	res.Unpacked += int64(s.Attachments) * int64(s.AttachmentBytes)
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
	for i := range s.Pages {
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
	for j := range s.Attachments {
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
