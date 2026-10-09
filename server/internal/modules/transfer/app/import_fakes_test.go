package app_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// zipEntry is an entry of an archive a test writes: deflated, unless its
// method is another; a folder when its name ends in "/"; written raw, its
// data as packed, when raw is set, with crc as its checksum.
type zipEntry struct {
	name   string
	data   string
	method uint16
	mode   fs.FileMode
	flags  uint16
	raw    bool
	crc    uint32
}

// zipOf writes entries as a zip archive.
func zipOf(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Flags: e.flags}
		if e.method != 0 || e.raw || strings.HasSuffix(e.name, "/") {
			h.Method = e.method
		}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		var w io.Writer
		var err error
		if e.raw {
			h.CRC32, h.CompressedSize64, h.UncompressedSize64 = e.crc, uint64(len(e.data)), uint64(len(e.data))
			w, err = zw.CreateRaw(h)
		} else {
			w, err = zw.CreateHeader(h)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// noise is n letters that do not pack.
func noise(n int) string {
	b := make([]byte, n)
	x := uint32(2463534242)
	for i := range b {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		b[i] = 'a' + byte(x%26)
	}
	return string(b)
}

// files are entries of names and contents, in pairs.
func files(pairs ...string) []zipEntry {
	out := make([]zipEntry, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, zipEntry{name: pairs[i], data: pairs[i+1]})
	}
	return out
}

// made is a node a unit of the fake tree created.
type made struct {
	id, changeset uuid.UUID
	parent        *uuid.UUID
	asset         bool
	name, content string
	file          app.File
}

// tree is a notebook's tree as an import's units write it. pages are the
// pages there before, by id, at their depth, children the names under
// each parent there before (uuid.Nil: the root). busyParses and busyUnits
// answer as many parses and units 503 first, busyCalls the parses of
// those numbers, from 1; refuse answers the next unit
// before its do, uncertain after it, writing nothing; gone are parents
// gone; deeper is how much deeper the pages are than they were; onUnit
// runs as each unit starts, its number from 1.
type tree struct {
	mu         sync.Mutex
	pages      map[uuid.UUID]int
	children   map[uuid.UUID][]string
	made       []made
	units      []app.ImportSpec
	committed  []int
	busyParses int
	busyCalls  map[int]bool
	calls      int
	busyUnits  int
	refuse     error
	uncertain  error
	gone       map[uuid.UUID]bool
	deeper     int
	onUnit     func(n int)
	parses     int
	released   int
}

func newTree() *tree {
	return &tree{pages: map[uuid.UUID]int{}, children: map[uuid.UUID][]string{}, gone: map[uuid.UUID]bool{}}
}

func (t *tree) CheckContent(content string) error {
	if len(content) > 5<<20 || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return &shared.Error{Kind: shared.KindInvalid, Code: shared.CodeValidationFailed}
	}
	return nil
}

func (t *tree) Parse(_ context.Context, content string) (app.Parsed, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	if t.busyParses > 0 || t.busyCalls[t.calls] {
		t.busyParses = max(t.busyParses-1, 0)
		return nil, shared.ServerBusy(time.Second)
	}
	t.parses++
	return &parsed{t: t, links: strings.Count(content, "[[")}, nil
}

// parsed is a content parsed: its links are its "[[".
type parsed struct {
	t        *tree
	links    int
	released bool
}

func (p *parsed) Links() int { return p.links }

func (p *parsed) Release() {
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	if !p.released {
		p.released = true
		p.t.released++
	}
}

func (t *tree) Import(ctx context.Context, spec app.ImportSpec, do func(ctx context.Context, u app.ImportUnit) error) (uuid.UUID, error) {
	t.mu.Lock()
	t.units = append(t.units, spec)
	n, onUnit := len(t.units), t.onUnit
	t.mu.Unlock()
	if onUnit != nil {
		onUnit(n)
	}
	t.mu.Lock()
	switch {
	case t.busyUnits > 0:
		t.busyUnits--
		t.mu.Unlock()
		return uuid.UUID{}, shared.ServerBusy(time.Second)
	case t.refuse != nil:
		err := t.refuse
		t.refuse = nil
		t.mu.Unlock()
		return uuid.UUID{}, err
	}
	t.mu.Unlock()
	u := &unit{t: t}
	if err := do(ctx, u); err != nil {
		return uuid.UUID{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.uncertain != nil {
		err := t.uncertain
		t.uncertain = nil
		return uuid.UUID{}, err
	}
	if len(u.made) == 0 {
		return uuid.UUID{}, nil
	}
	cs := spec.Changeset
	if cs == (uuid.UUID{}) {
		cs = uuid.NewV7()
	}
	for _, m := range u.made {
		m.changeset = cs
		t.made = append(t.made, m)
		t.children[under(m.parent)] = append(t.children[under(m.parent)], m.name)
	}
	t.committed = append(t.committed, len(u.made))
	return cs, nil
}

// unit is a unit of the fake tree: what it made, committed once its do
// ends well.
type unit struct {
	t    *tree
	made []made
}

func (u *unit) CreatePage(_ context.Context, p app.ImportedPage) (app.CreatedNode, error) {
	depth, err := u.depth(p.ParentID)
	if err != nil {
		return app.CreatedNode{}, err
	}
	if depth+1 > domain.MaxDepth {
		return app.CreatedNode{}, app.ErrTooDeep
	}
	return u.add(made{parent: p.ParentID, name: p.Name, content: p.Content}), nil
}

func (u *unit) CreateAsset(ctx context.Context, a app.ImportedAsset, after func(ctx context.Context, n app.CreatedNode) error) (app.CreatedNode, error) {
	if _, err := u.depth(a.ParentID); err != nil {
		return app.CreatedNode{}, err
	}
	n := u.add(made{parent: a.ParentID, asset: true, name: a.Name, file: a.File})
	if err := after(ctx, n); err != nil {
		return app.CreatedNode{}, err
	}
	return n, nil
}

// depth is the depth of a parent: ErrNoParent for one gone, or no page.
func (u *unit) depth(parent *uuid.UUID) (int, error) {
	u.t.mu.Lock()
	defer u.t.mu.Unlock()
	all := append(append([]made(nil), u.t.made...), u.made...)
	depth := 0
	for at := parent; at != nil; depth++ {
		if u.t.gone[*at] {
			return 0, app.ErrNoParent
		}
		if d, ok := u.t.pages[*at]; ok {
			return depth + d + u.t.deeper, nil
		}
		i := slices.IndexFunc(all, func(m made) bool { return m.id == *at && !m.asset })
		if i < 0 {
			return 0, app.ErrNoParent
		}
		at = all[i].parent
	}
	return depth, nil
}

// under is the key of a parent's children: uuid.Nil for the root.
func under(parent *uuid.UUID) uuid.UUID {
	if parent == nil {
		return uuid.UUID{}
	}
	return *parent
}

// add makes m, numbered when a sibling's name has its key.
func (u *unit) add(m made) app.CreatedNode {
	u.t.mu.Lock()
	defer u.t.mu.Unlock()
	taken := map[string]bool{}
	for _, name := range u.t.children[under(m.parent)] {
		taken[strings.ToLower(name)] = true
	}
	for _, x := range u.made {
		if under(x.parent) == under(m.parent) {
			taken[strings.ToLower(x.name)] = true
		}
	}
	name := m.name
	for n := 2; taken[strings.ToLower(name)]; n++ {
		if ext := path.Ext(m.name); m.asset && ext != "" {
			name = strings.TrimSuffix(m.name, ext) + " " + strconv.Itoa(n) + ext
		} else {
			name = m.name + " " + strconv.Itoa(n)
		}
	}
	m.id, m.name = uuid.NewV7(), name
	u.made = append(u.made, m)
	return app.CreatedNode{ID: m.id, Name: m.name, CreatedAt: now()}
}

// named is the node made named name; it fails the test when there is
// none.
func (t *tree) named(tb testing.TB, name string) made {
	tb.Helper()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, m := range t.made {
		if m.name == name {
			return m
		}
	}
	tb.Fatalf("no node %q made", name)
	return made{}
}

// lines are the nodes made, each "parent/name", the parent's name or "-"
// for the place the import went, in the order made.
func (t *tree) lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	names := map[uuid.UUID]string{}
	var out []string
	for _, m := range t.made {
		names[m.id] = m.name
		parent := "-"
		if m.parent != nil {
			if p, ok := names[*m.parent]; ok {
				parent = p
			}
		}
		out = append(out, parent+"/"+m.name)
	}
	return out
}

// attachments keep the files Put wrote, in memory: full answers every Put
// as a store out of room; wait makes Put wait for its context's end.
type attachments struct {
	mu       sync.Mutex
	files    map[uuid.UUID][]byte
	attached map[uuid.UUID]app.Owner
	dropped  []uuid.UUID
	full     bool
	wait     bool
}

func newAttachments() *attachments {
	return &attachments{files: map[uuid.UUID][]byte{}, attached: map[uuid.UUID]app.Owner{}}
}

func (a *attachments) Put(ctx context.Context, _ string, r io.Reader, maxBytes int64) (app.File, error) {
	a.mu.Lock()
	wait := a.wait
	a.mu.Unlock()
	if wait {
		<-ctx.Done()
		return app.File{}, context.Cause(ctx)
	}
	if a.full {
		return app.File{}, domain.ErrStorageFull
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	switch {
	case err != nil:
		return app.File{}, err
	case int64(len(data)) > maxBytes:
		return app.File{}, app.ErrTooLarge
	}
	sum := sha256.Sum256(data)
	f := app.File{ID: uuid.NewV7(), MIME: "application/octet-stream", Bytes: int64(len(data)), SHA256: sum[:]}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[f.ID] = data
	return f, nil
}

func (a *attachments) Attach(_ context.Context, f app.File, o app.Owner) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attached[f.ID] = o
	return nil
}

func (a *attachments) Drop(_ context.Context, f app.File) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dropped = append(a.dropped, f.ID)
	delete(a.files, f.ID)
	return nil
}

// kept is how many files Put wrote and no Drop deleted.
func (a *attachments) kept() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.files)
}

// analyzer counts the refreshes of its statistics, failing with err.
type analyzer struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (a *analyzer) Analyze(context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	return a.err
}

func (a *analyzer) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// importWorld is an import's surroundings: the export's world, its tree,
// the attachments' files and statistics, bob, eng's admin, importing.
type importWorld struct {
	*world
	tree  *tree
	files *attachments
	stats *analyzer
	max   app.ImportDeps
}

func newImportWorld() *importWorld {
	w := &importWorld{world: newWorld(), tree: newTree(), files: newAttachments(), stats: &analyzer{}}
	w.max = app.ImportDeps{MaxEntries: 1000, MaxUnpacked: 64 << 20, MaxContent: 5 << 20, MaxAsset: 1 << 20}
	for _, n := range w.nodes.all {
		if !n.Asset {
			depth, _, _ := w.nodes.Depth(context.Background(), w.eng, n.ID)
			w.tree.pages[n.ID] = depth
		}
	}
	return w
}

func (w *importWorld) importer() *app.Import {
	return app.NewImport(app.ImportDeps{Authorizer: w.auth, Notebooks: w.notebooks, Nodes: w.nodes, Tree: w.tree, Attachments: w.files,
		Statistics: []app.Analyzer{w.stats}, Archives: w.archives, Rows: w.rows, Clock: fixedClock{now()}, Logger: w.logger, Beat: 5 * time.Millisecond,
		MaxEntries: w.max.MaxEntries, MaxUnpacked: w.max.MaxUnpacked, MaxContent: w.max.MaxContent, MaxAsset: w.max.MaxAsset, Backoff: time.Millisecond})
}

// queuedImport adds bob's import of archive into eng, under root when
// set, queued.
func (w *importWorld) queuedImport(root *uuid.UUID, archive []byte) domain.Job {
	j := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, RootID: root, Kind: domain.KindImport, State: domain.StateQueued, Name: "vault.zip",
		CreatedBy: w.bob, Client: domain.ClientAPI, CreatedAt: now()}
	w.rows.add(j)
	w.archives.imports[j.ID] = archive
	return j
}

// run runs the import job, and answers its row as it ended.
func (w *importWorld) run(t *testing.T, j domain.Job) domain.Job {
	t.Helper()
	if err := w.importer().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	return w.rows.get(j.ID)
}

// problems are the report's problems, each "code: path", and "-> to" for
// one renamed.
func problems(r *domain.Report) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, p := range r.Problems {
		s := string(p.Code) + ": " + p.Path
		if p.To != "" {
			s += " -> " + p.To
		}
		out = append(out, s)
	}
	return out
}
