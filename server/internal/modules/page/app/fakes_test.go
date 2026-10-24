package app_test

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is the clock's first instant.
func now() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }

// tickingClock reads now first, then a microsecond later each time: a unit
// that read it twice for what must be one time would write two.
type tickingClock struct{ reads int }

func (c *tickingClock) Now() time.Time {
	c.reads++
	return now().Add(time.Duration(c.reads-1) * time.Microsecond)
}

type txKey struct{}

// fakeTx runs fn with a context that says it is in a transaction, and
// records whether fn failed (a rollback).
type fakeTx struct{ rolledBack bool }

func (f *fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if f.InTx(ctx) {
		return fn(ctx)
	}
	err := fn(context.WithValue(ctx, txKey{}, true))
	f.rolledBack = err != nil
	return err
}

func (f *fakeTx) InTx(ctx context.Context) bool { return ctx.Value(txKey{}) != nil }

// recorder is the calls of every fake, in order, each with whether it ran
// in the transaction: the unit's order of locks, decision and writes.
// recorder records the calls made, in order; before, when set, runs a
// call's function once, before it: what another transaction commits
// between two reads.
type recorder struct {
	calls  []string
	before map[string]func()
}

func (r *recorder) record(ctx context.Context, call string) {
	if ctx.Value(txKey{}) != nil {
		call += " in tx"
	}
	if f, ok := r.before[call]; ok {
		delete(r.before, call)
		f()
	}
	r.calls = append(r.calls, call)
}

// fakeWorkspaces finds a workspace by its slug and shares it unless it is
// in gone.
type fakeWorkspaces struct {
	*recorder
	slugs map[string]uuid.UUID
	gone  map[uuid.UUID]bool
}

func (f fakeWorkspaces) FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error) {
	f.record(ctx, "FindWorkspace")
	id, ok := f.slugs[slug]
	return id, ok && !f.gone[id], nil
}

func (f fakeWorkspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "ShareWorkspace")
	return !f.gone[id], nil
}

// fakeNotebooks knows each notebook's workspace, and its state in states,
// standard when it has none there; one in gone is deleted by the time it
// is locked, or its state read. waited, when set, runs while a write waits
// for the notebook's lock: what another write commits meanwhile.
type fakeNotebooks struct {
	*recorder
	workspaces map[uuid.UUID]uuid.UUID
	gone       map[uuid.UUID]bool
	states     map[uuid.UUID]domain.NotebookState
	waited     func()
	// err fails WorkspaceOf.
	err error
}

func (f fakeNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	f.record(ctx, "WorkspaceOf")
	if f.err != nil {
		return uuid.UUID{}, false, f.err
	}
	ws, ok := f.workspaces[id]
	return ws, ok, nil
}

func (f fakeNotebooks) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "ShareNotebook")
	_, ok := f.workspaces[id]
	return ok && !f.gone[id], nil
}

func (f fakeNotebooks) LockByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "LockNotebook")
	if f.waited != nil {
		f.waited()
	}
	_, ok := f.workspaces[id]
	return ok && !f.gone[id], nil
}

func (f fakeNotebooks) StateOf(ctx context.Context, id uuid.UUID) (domain.NotebookState, bool, error) {
	f.record(ctx, "StateOf")
	if _, ok := f.workspaces[id]; !ok || f.gone[id] {
		return domain.NotebookState{}, false, nil
	}
	if s, ok := f.states[id]; ok {
		return s, true, nil
	}
	return domain.NotebookState{Mode: domain.ModeStandard}, true, nil
}

// fakeStore is the repository in memory: the nodes not deleted, their
// contents, and what the changesets recorded.
type fakeStore struct {
	*recorder
	nodes      map[uuid.UUID]domain.Node
	contents   map[uuid.UUID]app.Content
	changesets []app.Changeset
	// items are the changesets' items, merged by node as the table's
	// upsert merges them; recorded are all of them, each changeset's
	// merged by node.
	items     map[uuid.UUID]app.Item
	recorded  []app.Item
	revisions []app.Revision
	// touched are the changesets written again, with when; marked their
	// renames and moves, as the units marked them.
	touched map[uuid.UUID]time.Time
	marked  map[uuid.UUID]domain.ChangesetMoves
	// sessions are the edit sessions; held are those another transaction
	// holds.
	sessions map[uuid.UUID]app.EditSession
	held     map[uuid.UUID]bool
	// queued are the search's queueings, in order; queuedNotebooks the
	// notebooks whose deletion queued their nodes.
	queued          []queuing
	queuedNotebooks []uuid.UUID
	// errs fails the calls named: FindNodeIn and Children; WriteContent,
	// RenameNode, MoveNode and RelocateNode.
	errs map[string]error
	// trash are the nodes deleted, with when and whether each is a trash
	// item's root, which FindAnyNode and the trash's reads read; counts
	// are the roots' pages, attachments and height; trashed are the
	// deleted pages' contents.
	trash   map[uuid.UUID]app.AnyNode
	counts  map[uuid.UUID][3]int
	trashed map[uuid.UUID]app.Content
	// deletions are what DeleteNodes was given, restorations what
	// RestoreNodes was; rows, when set, are what RestoreNodes answers it
	// brought back, as if the purge took some. lost are the pages whose
	// contents TrashContents does not find, as if the purge took them
	// after TrashGroup read them; unversioned those whose current version
	// the purge took, which RestoreNodes does not count. history is the
	// rows of a trashed node's versions and items, none unless set.
	deletions    []app.Deletion
	restorations []app.Restoration
	rows         *app.RestoredRows
	lost         map[uuid.UUID]bool
	unversioned  map[uuid.UUID]bool
	history      map[uuid.UUID]int
	// marks are what MarkTrashRoots was given; firsts when each
	// changeset's item of each node was first written, which a merge into
	// it keeps (the table's created_at).
	marks  []app.TrashMark
	firsts map[[2]uuid.UUID]time.Time
	// read are the bytes of the contents the reads within a bound read,
	// a read's each (TrashContentsWithin, PageContentWithin), in their
	// order: 0 for one that read none, past it.
	read []int
}

func (f *fakeStore) FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error) {
	f.record(ctx, "FindNode")
	n, ok := f.nodes[id]
	if !ok {
		return domain.Node{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) FindAnyNode(ctx context.Context, id uuid.UUID) (app.AnyNode, error) {
	f.record(ctx, "FindAnyNode")
	if n, ok := f.nodes[id]; ok {
		return app.AnyNode{Node: n}, nil
	}
	if n, ok := f.trash[id]; ok {
		return n, nil
	}
	return app.AnyNode{}, app.ErrNotFound
}

// versionsOf are the page id's versions, as the version queries join them
// to their changesets, in the order of their revisions.
func (f *fakeStore) versionsOf(id uuid.UUID) []domain.Version {
	var out []domain.Version
	for _, r := range f.revisions {
		if r.NodeID != id {
			continue
		}
		v := domain.Version{ID: r.ID, Base: r.Base, Revision: r.Revision, ByteSize: r.ByteSize, CreatedAt: r.At, UpdatedAt: r.At}
		for _, c := range f.changesets {
			if c.ID == r.ChangesetID {
				v.Changeset = domain.VersionChangeset{ID: c.ID, Kind: c.Kind, Client: c.Client, By: c.By, CreatedAt: c.At}
			}
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b domain.Version) int { return a.Revision - b.Revision })
	return out
}

func (f *fakeStore) ListVersions(ctx context.Context, pageID uuid.UUID, below int, limit int) ([]domain.Version, error) {
	f.record(ctx, "ListVersions")
	var out []domain.Version
	all := f.versionsOf(pageID)
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		if all[i].Revision < below {
			out = append(out, all[i])
		}
	}
	return out, nil
}

func (f *fakeStore) FindVersion(ctx context.Context, pageID uuid.UUID, revision int) (domain.Version, string, error) {
	f.record(ctx, "FindVersion")
	for _, v := range f.versionsOf(pageID) {
		if v.Revision >= revision {
			for _, r := range f.revisions {
				if r.ID == v.ID {
					return v, r.Content, nil
				}
			}
		}
	}
	return domain.Version{}, "", app.ErrNotFound
}

func (f *fakeStore) AliveChangesetOf(ctx context.Context, pageID uuid.UUID, now time.Time) (uuid.UUID, error) {
	f.record(ctx, "AliveChangesetOf")
	for _, s := range f.sessions {
		if s.NodeID == pageID && s.Alive(now) {
			return s.ChangesetID, nil
		}
	}
	return uuid.UUID{}, nil
}

func (f *fakeStore) FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error) {
	f.record(ctx, "FindNodeIn")
	if err := f.errs["FindNodeIn"]; err != nil {
		return domain.Node{}, err
	}
	n, ok := f.nodes[id]
	if !ok || n.NotebookID != notebookID {
		return domain.Node{}, app.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error) {
	f.record(ctx, "ListNodes")
	var out []domain.Node
	for _, n := range f.nodes {
		if n.NotebookID == notebookID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeStore) Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error) {
	f.record(ctx, "Children")
	if err := f.errs["Children"]; err != nil {
		return nil, err
	}
	var out []domain.Node
	for _, n := range f.nodes {
		if n.NotebookID == notebookID && sameParent(n.ParentID, parentID) {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b domain.Node) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), a.ID.Compare(b.ID))
	})
	return out, nil
}

func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (f *fakeStore) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error) {
	f.record(ctx, "Ancestors")
	var out []domain.Ancestor
	for p := f.nodes[id].ParentID; p != nil; p = f.nodes[*p].ParentID {
		out = append(out, domain.Ancestor{ID: *p, Name: f.nodes[*p].Name})
	}
	slices.Reverse(out)
	return out, nil
}

func (f *fakeStore) Subtree(ctx context.Context, notebookID, id uuid.UUID) (domain.Subtree, error) {
	f.record(ctx, "Subtree")
	root, ok := f.nodes[id]
	if !ok || root.NotebookID != notebookID {
		return nil, app.ErrNotFound
	}
	out := domain.Subtree{{Node: root, Level: 1}}
	for i := 0; i < len(out); i++ {
		var children []domain.Node
		for _, n := range f.nodes {
			if n.ParentID != nil && *n.ParentID == out[i].Node.ID {
				children = append(children, n)
			}
		}
		for _, c := range children {
			out = append(out, domain.SubtreeNode{Node: c, Level: out[i].Level + 1})
		}
	}
	slices.SortStableFunc(out, func(a, b domain.SubtreeNode) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.Node.SortOrder, b.Node.SortOrder), a.Node.ID.Compare(b.Node.ID))
	})
	return out, nil
}

func (f *fakeStore) ContentMeta(ctx context.Context, id uuid.UUID) (app.ContentMeta, error) {
	f.record(ctx, "ContentMeta")
	c, ok := f.contents[id]
	if !ok {
		return app.ContentMeta{}, app.ErrNotFound
	}
	return app.ContentMeta{Revision: c.Revision, ByteSize: c.ByteSize, UpdatedBy: c.By, UpdatedAt: c.At}, nil
}

func (f *fakeStore) PageContent(ctx context.Context, id uuid.UUID) (app.PageContent, error) {
	f.record(ctx, "PageContent")
	c, ok := f.contents[id]
	if !ok {
		return app.PageContent{}, app.ErrNotFound
	}
	return app.PageContent{Content: c.Content, Revision: c.Revision, Hash: c.Hash}, nil
}

// PageContentWithin is PageContent while the content holds max bytes at
// most, by its length: a content's size is its bytes'.
func (f *fakeStore) PageContentWithin(ctx context.Context, id uuid.UUID, max int) (app.PageContent, bool, error) {
	f.record(ctx, "PageContentWithin")
	c, ok := f.contents[id]
	switch {
	case !ok:
		return app.PageContent{}, false, app.ErrNotFound
	case len(c.Content) > max:
		f.read = append(f.read, 0)
		return app.PageContent{Revision: c.Revision, Hash: c.Hash}, false, nil
	}
	f.read = append(f.read, len(c.Content))
	return app.PageContent{Content: c.Content, Revision: c.Revision, Hash: c.Hash}, true, nil
}

func (f *fakeStore) LockContent(ctx context.Context, notebookID, id uuid.UUID) (app.ContentLock, error) {
	f.record(ctx, "LockContent")
	n, ok := f.nodes[id]
	c, has := f.contents[id]
	if !ok || !has || n.NotebookID != notebookID || n.Kind != domain.KindPage {
		return app.ContentLock{}, app.ErrNotFound
	}
	return app.ContentLock{Revision: c.Revision, Hash: c.Hash, ByteSize: c.ByteSize}, nil
}

// fakeMarkdown takes a content's facts as the content itself and renders
// it in a <p>, with the page it rendered for; err, when set, is Render's.
type fakeMarkdown struct {
	*recorder
	pages []app.PageRef
	err   error
	// expires is when the views it renders expire.
	expires time.Time
	// panics has Facts panic.
	panics bool
	// tasksRead, when set, runs once after Tasks: someone's write while
	// the tasks are read.
	tasksRead func()
}

func (f *fakeMarkdown) Facts(content string) app.Facts {
	f.record(context.Background(), "Facts")
	if f.panics {
		panic("the parse failed")
	}
	return content
}

// FrontmatterEnd is the end of the content's first line of "---" up to
// its next one, 0 without both.
func (f *fakeMarkdown) FrontmatterEnd(content string) int {
	f.record(context.Background(), "FrontmatterEnd")
	if !strings.HasPrefix(content, "---\n") {
		return 0
	}
	if i := strings.Index(content[3:], "\n---\n"); i >= 0 {
		return 3 + i + len("\n---\n")
	}
	return 0
}

// Links counts the content's "[[".
func (f *fakeMarkdown) Links(facts app.Facts) int {
	f.record(context.Background(), "Links")
	return strings.Count(facts.(string), "[[")
}

// Tasks finds a task item at the start of a line, "- [ ]", "- [\t]",
// "- [x]" or "- [X]", but for a "- [x]: " line, which a link reference definition
// is: ticking "- [ ]: /u" makes one, as in the tasks extension.
func (f *fakeMarkdown) Tasks(facts app.Facts) []app.Task {
	f.record(context.Background(), "Tasks")
	var out []app.Task
	at := 0
	for line := range strings.SplitAfterSeq(facts.(string), "\n") {
		if strings.HasPrefix(line, "- [") && len(line) > 4 && strings.ContainsRune(" \txX", rune(line[3])) && line[4] == ']' &&
			!strings.HasPrefix(line, "- [x]: ") {
			out = append(out, app.Task{Offset: at + 3, Checked: line[3] == 'x' || line[3] == 'X'})
		}
		at += len(line)
	}
	if read := f.tasksRead; read != nil {
		f.tasksRead = nil
		read()
	}
	return out
}

// fakeBudget records each take of the parse budget with its bytes, each
// keeping of the facts' share and each release; err, when set, is Take's.
// A take waits the first of waits, when there are, before it gets its
// bytes, or its context's end first.
type fakeBudget struct {
	*recorder
	err      error
	held     int
	released int
	waits    []time.Duration
	// capacity, when set, is the budget's bytes: a take past what is left
	// of them waits for its context's end, as nothing else gives back.
	capacity int
}

func (b *fakeBudget) Take(ctx context.Context, n int) (app.BudgetHold, error) {
	b.record(ctx, fmt.Sprintf("Take %d", n))
	if b.err != nil {
		return nil, b.err
	}
	if b.capacity > 0 && b.held+n > b.capacity {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if len(b.waits) > 0 {
		wait := b.waits[0]
		b.waits = b.waits[1:]
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b.held += n
	return &fakeHold{budget: b, n: n}, nil
}

// fakeHold is a take of fakeBudget: it records the facts it keeps by their
// size, fakeMarkdown's facts being the content, and keeps nothing back, as
// the bytes they keep do not matter here; it gives all back once.
type fakeHold struct {
	budget *fakeBudget
	n      int
	done   bool
}

func (h *fakeHold) KeepFacts(f app.Facts) {
	content, _ := f.(string)
	h.budget.record(context.Background(), fmt.Sprintf("KeepFacts %d", len(content)))
}

// Takes is n, and the capacity, or no bound: a fake hold keeps its take.
func (b *fakeBudget) Takes(n int) (int, int) {
	if b.capacity > 0 {
		return n, b.capacity
	}
	return n, math.MaxInt
}

func (h *fakeHold) Held() int {
	if h.done {
		return 0
	}
	return h.n
}

func (h *fakeHold) Release() {
	if h.done {
		return
	}
	h.done = true
	h.budget.record(context.Background(), fmt.Sprintf("Release %d", h.n))
	h.budget.held -= h.n
	h.budget.released++
}

func (f *fakeMarkdown) Render(ctx context.Context, content string, page app.PageRef) (app.Rendered, error) {
	f.record(ctx, "Render")
	f.pages = append(f.pages, page)
	return app.Rendered{HTML: "<p>" + content + "</p>", Expires: f.expires}, f.err
}

func (f *fakeStore) CreateNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "CreateNode")
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) CreateContent(ctx context.Context, c app.Content) error {
	f.record(ctx, "CreateContent")
	f.contents[c.NodeID] = c
	return nil
}

func (f *fakeStore) WriteContent(ctx context.Context, c app.Content) error {
	f.record(ctx, "WriteContent")
	if err := f.errs["WriteContent"]; err != nil {
		return err
	}
	f.contents[c.NodeID] = c
	return nil
}

func (f *fakeStore) RenameNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "RenameNode")
	if err := f.errs["RenameNode"]; err != nil {
		return err
	}
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) MoveNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "MoveNode")
	if err := f.errs["MoveNode"]; err != nil {
		return err
	}
	f.nodes[n.ID] = n
	return nil
}

func (f *fakeStore) RelocateNode(ctx context.Context, n domain.Node) error {
	f.record(ctx, "RelocateNode")
	if err := f.errs["RelocateNode"]; err != nil {
		return err
	}
	f.nodes[n.ID] = n
	return nil
}

// DeleteNodes moves the nodes not deleted to the trash, the root an item
// with its counts, with their contents.
func (f *fakeStore) DeleteNodes(ctx context.Context, d app.Deletion) error {
	f.record(ctx, "DeleteNodes")
	f.deletions = append(f.deletions, d)
	for _, id := range d.IDs {
		n, ok := f.nodes[id]
		if !ok {
			continue
		}
		at := d.At
		f.trash[id] = app.AnyNode{Node: n, DeletedAt: &at, TrashRoot: id == d.Root}
		if c, ok := f.contents[id]; ok {
			f.trashed[id] = c
		}
		delete(f.nodes, id)
		delete(f.contents, id)
	}
	f.counts[d.Root] = [3]int{d.Pages, d.Assets, d.Height}
	return nil
}

// RestoreNodes brings the nodes deleted at the restoration's time back
// with their contents, renamed, the root put where it goes, and counts
// them; each content counts as its creation's version, with no gap, and
// as its current version unless unversioned.
func (f *fakeStore) RestoreNodes(ctx context.Context, r app.Restoration) (app.RestoredRows, error) {
	f.record(ctx, "RestoreNodes")
	f.restorations = append(f.restorations, r)
	var rows app.RestoredRows
	for i, id := range r.IDs {
		n, ok := f.trash[id]
		if !ok || !n.DeletedAt.Equal(r.DeletedAt) {
			continue
		}
		node := n.Node
		node.Name, node.NameKey, node.UpdatedBy, node.UpdatedAt = r.Names[i], r.NameKeys[i], r.By, r.At
		if id == r.Root {
			node.ParentID, node.SortOrder = r.ParentID, r.SortOrder
		}
		f.nodes[id] = node
		delete(f.trash, id)
		rows.Nodes++
		if c, ok := f.trashed[id]; ok {
			f.contents[id] = c
			delete(f.trashed, id)
			rows.Contents++
			rows.Creations++
			if !f.unversioned[id] {
				rows.CurrentVersions++
			}
		}
	}
	if f.rows != nil {
		return *f.rows, nil
	}
	return rows, nil
}

// trashRoot is the trash item of the root n, its counts and its page's
// content's revision at its deletion.
func (f *fakeStore) trashRoot(n app.AnyNode) domain.TrashRoot {
	c := f.counts[n.ID]
	return domain.TrashRoot{Node: n.Node, DeletedAt: *n.DeletedAt, Pages: c[0], Assets: c[1], Height: c[2],
		Revision: f.trashed[n.ID].Revision}
}

func (f *fakeStore) ListTrash(ctx context.Context, notebookID uuid.UUID, since time.Time, below domain.TrashCursor, limit int) (
	[]domain.TrashRoot, error,
) {
	f.record(ctx, "ListTrash")
	var out []domain.TrashRoot
	for _, n := range f.trash {
		at := *n.DeletedAt
		if n.TrashRoot && n.NotebookID == notebookID && at.After(since) &&
			(at.Before(below.At) || at.Equal(below.At) && n.ID.Compare(below.ID) < 0) {
			out = append(out, f.trashRoot(n))
		}
	}
	slices.SortFunc(out, func(a, b domain.TrashRoot) int {
		return cmp.Or(b.DeletedAt.Compare(a.DeletedAt), b.Node.ID.Compare(a.Node.ID))
	})
	return out[:min(limit, len(out))], nil
}

func (f *fakeStore) FindTrashRoot(ctx context.Context, id uuid.UUID) (domain.TrashRoot, error) {
	f.record(ctx, "FindTrashRoot")
	n, ok := f.trash[id]
	if !ok || !n.TrashRoot {
		return domain.TrashRoot{}, app.ErrNotFound
	}
	return f.trashRoot(n), nil
}

func (f *fakeStore) TrashRootOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	f.record(ctx, "TrashRootOf")
	for n, ok := f.trash[id]; ok; n, ok = f.trash[*n.ParentID] {
		if n.TrashRoot {
			return n.ID, nil
		}
		if n.ParentID == nil {
			break
		}
	}
	return uuid.UUID{}, app.ErrNotFound
}

func (f *fakeStore) TrashLines(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.TrashAncestor, error) {
	f.record(ctx, fmt.Sprintf("TrashLines %d", len(ids)))
	out := map[uuid.UUID][]domain.TrashAncestor{}
	for _, id := range ids {
		var line []domain.TrashAncestor
		for p := f.anyNode(id).ParentID; p != nil; p = f.anyNode(*p).ParentID {
			n := f.anyNode(*p)
			line = append(line, domain.TrashAncestor{ID: n.ID, Name: n.Name, DeletedAt: n.DeletedAt})
		}
		if len(line) > 0 {
			slices.Reverse(line)
			out[id] = line
		}
	}
	return out, nil
}

// anyNode is the node id alive or in the trash.
func (f *fakeStore) anyNode(id uuid.UUID) app.AnyNode {
	if n, ok := f.nodes[id]; ok {
		return app.AnyNode{Node: n}
	}
	return f.trash[id]
}

func (f *fakeStore) TrashGroup(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.TrashMember, error) {
	f.record(ctx, "TrashGroup")
	return f.group(notebookID, id, at)
}

// group is the trash item of the root id deleted at at: the root, then
// each level, by parent, each parent's children by order and id.
func (f *fakeStore) group(notebookID, id uuid.UUID, at time.Time) ([]domain.TrashMember, error) {
	root, ok := f.trash[id]
	if !ok || root.NotebookID != notebookID || !root.DeletedAt.Equal(at) {
		return nil, app.ErrNotFound
	}
	member := func(n app.AnyNode, level int) domain.TrashMember {
		c := f.trashed[n.ID]
		return domain.TrashMember{Node: n.Node, Level: level, Revision: c.Revision, Bytes: c.ByteSize}
	}
	under := map[uuid.UUID][]app.AnyNode{}
	for _, n := range f.trash {
		if n.ParentID != nil && n.DeletedAt.Equal(at) {
			under[*n.ParentID] = append(under[*n.ParentID], n)
		}
	}
	out := []domain.TrashMember{member(root, 1)}
	for i := 0; i < len(out); i++ {
		var level []domain.TrashMember
		for _, n := range under[out[i].Node.ID] {
			level = append(level, member(n, out[i].Level+1))
		}
		slices.SortFunc(level, func(a, b domain.TrashMember) int {
			return cmp.Or(cmp.Compare(a.Node.SortOrder, b.Node.SortOrder), a.Node.ID.Compare(b.Node.ID))
		})
		out = append(out, level...)
	}
	slices.SortStableFunc(out, func(a, b domain.TrashMember) int { return cmp.Compare(a.Level, b.Level) })
	return out, nil
}

// TrashGroups reads each root's group as TrashGroup does, in one call.
func (f *fakeStore) TrashGroups(ctx context.Context, notebookID uuid.UUID, roots map[uuid.UUID]time.Time) (
	map[uuid.UUID][]domain.TrashMember, error,
) {
	f.record(ctx, fmt.Sprintf("TrashGroups %d", len(roots)))
	out := map[uuid.UUID][]domain.TrashMember{}
	for root, at := range roots {
		if g, err := f.group(notebookID, root, at); err == nil {
			out[root] = g
		}
	}
	return out, nil
}

// TrashShape is the group below its root as TrashGroup reads it.
func (f *fakeStore) TrashShape(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.ShapeNode, error) {
	f.record(ctx, "TrashShape")
	g, err := f.group(notebookID, id, at)
	if err != nil {
		return nil, nil
	}
	out := make([]domain.ShapeNode, 0, len(g)-1)
	for _, m := range g[1:] {
		out = append(out, domain.ShapeNode{ID: m.Node.ID, ParentID: *m.Node.ParentID, Asset: m.Node.Kind == domain.KindAsset, Bytes: m.Bytes})
	}
	return out, nil
}

// TrashNodesAt reads the nodes each in the trash at its time, a root with
// its counts.
func (f *fakeStore) TrashNodesAt(ctx context.Context, notebookID uuid.UUID, at map[uuid.UUID]time.Time) (
	map[uuid.UUID]app.TrashNode, error,
) {
	f.record(ctx, fmt.Sprintf("TrashNodesAt %d", len(at)))
	out := map[uuid.UUID]app.TrashNode{}
	for id, t := range at {
		n, ok := f.trash[id]
		if !ok || n.NotebookID != notebookID || !n.DeletedAt.Equal(t) {
			continue
		}
		c := f.trashed[id]
		node := app.TrashNode{TrashMember: domain.TrashMember{Node: n.Node, Revision: c.Revision, Bytes: c.ByteSize}, Root: n.TrashRoot}
		if n.TrashRoot {
			counts := f.counts[id]
			node.Pages, node.Assets, node.Height = counts[0], counts[1], counts[2]
		}
		out[id] = node
	}
	return out, nil
}

// MarkTrashRoots marks the nodes still in the trash at at and no root
// each a root with its counts, and counts them.
func (f *fakeStore) MarkTrashRoots(ctx context.Context, roots []app.TrashMark, at time.Time) (int, error) {
	f.record(ctx, fmt.Sprintf("MarkTrashRoots %d", len(roots)))
	f.marks = append(f.marks, roots...)
	n := 0
	for _, r := range roots {
		t, ok := f.trash[r.ID]
		if !ok || !t.DeletedAt.Equal(at) || t.TrashRoot {
			continue
		}
		t.TrashRoot = true
		f.trash[r.ID], f.counts[r.ID] = t, [3]int{r.Pages, r.Assets, r.Height}
		n++
	}
	return n, nil
}

func (f *fakeStore) TrashContents(ctx context.Context, ids []uuid.UUID, at time.Time) (map[uuid.UUID]string, error) {
	f.record(ctx, "TrashContents")
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if c, ok := f.trashed[id]; ok && f.trash[id].DeletedAt.Equal(at) && !f.lost[id] {
			out[id] = c.Content
		}
	}
	return out, nil
}

// TrashContentsWithin are TrashContents in the order of ids while their
// lengths sum to max at most, the first past it its size alone.
func (f *fakeStore) TrashContentsWithin(ctx context.Context, ids []uuid.UUID, at time.Time, max int64) (app.ContentsWithin, error) {
	f.record(ctx, "TrashContentsWithin")
	out, total, read := app.ContentsWithin{Contents: map[uuid.UUID]string{}}, int64(0), 0
	defer func() { f.read = append(f.read, read) }()
	for _, id := range ids {
		c, ok := f.trashed[id]
		if !ok || !f.trash[id].DeletedAt.Equal(at) || f.lost[id] {
			continue
		}
		if total += int64(len(c.Content)); total > max {
			out.Past, out.PastBytes = id, len(c.Content)
			return out, nil
		}
		out.Contents[id], read = c.Content, read+len(c.Content)
	}
	return out, nil
}

func (f *fakeStore) TrashHistory(ctx context.Context, ids []uuid.UUID, at time.Time, limit int) (int, error) {
	f.record(ctx, "TrashHistory")
	rows := 0
	for _, id := range ids {
		if n, ok := f.trash[id]; ok && n.DeletedAt.Equal(at) {
			rows += f.history[id]
		}
	}
	return min(rows, limit), nil
}

// fakeAttachments is the attachments' infos, by node: each read is a call.
type fakeAttachments struct {
	*recorder
	infos map[uuid.UUID]app.AttachmentInfo
}

func (f fakeAttachments) Of(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.AttachmentInfo, error) {
	f.record(ctx, fmt.Sprintf("AttachmentInfos %d", len(ids)))
	out := map[uuid.UUID]app.AttachmentInfo{}
	for _, id := range ids {
		if info, ok := f.infos[id]; ok {
			out[id] = info
		}
	}
	return out, nil
}

func (f *fakeStore) SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error {
	f.record(ctx, "SetSortOrder")
	n := f.nodes[id]
	n.SortOrder = order
	f.nodes[id] = n
	return nil
}

func (f *fakeStore) CreateChangeset(ctx context.Context, c app.Changeset) error {
	f.record(ctx, "CreateChangeset")
	f.changesets = append(f.changesets, c)
	return nil
}

// LockChangeset reads the changeset id, reverted when a revert of it was
// made.
func (f *fakeStore) LockChangeset(ctx context.Context, id uuid.UUID) (app.Changeset, error) {
	f.record(ctx, "LockChangeset")
	for _, c := range f.changesets {
		if c.ID == id {
			c.Reverted = slices.ContainsFunc(f.changesets, func(r app.Changeset) bool { return r.RevertsID == id })
			return c, nil
		}
	}
	return app.Changeset{}, app.ErrNotFound
}

func (f *fakeStore) TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error {
	f.record(ctx, "TouchChangeset")
	f.touched[id] = at
	return nil
}

// RecordItems inserts the items, or merges each into its node's of the
// changeset, keeping its before, its restore and the revision it recorded,
// as the table's upsert does; each write is one call, with its items'
// count.
func (f *fakeStore) RecordItems(ctx context.Context, items []app.Item) error {
	f.record(ctx, fmt.Sprintf("RecordItems %d", len(items)))
	for _, it := range items {
		if old, ok := f.items[it.Change.NodeID]; ok && old.ChangesetID == it.ChangesetID {
			it.Change.Before, it.Change.Restored, it.Change.Revision = old.Change.Before, old.Change.Restored, old.Change.Revision
		}
		f.items[it.Change.NodeID] = it
		f.recorded = slices.DeleteFunc(f.recorded, func(r app.Item) bool {
			return r.ChangesetID == it.ChangesetID && r.Change.NodeID == it.Change.NodeID
		})
		f.recorded = append(f.recorded, it)
		if key := [2]uuid.UUID{it.ChangesetID, it.Change.NodeID}; f.firsts[key].IsZero() {
			f.firsts[key] = it.At
		}
	}
	return nil
}

// RecordRevision inserts the version, or updates the page's in the
// changeset, keeping its base and id, as the table's upsert does.
func (f *fakeStore) RecordRevision(ctx context.Context, r app.Revision) error {
	f.record(ctx, "RecordRevision")
	for i, old := range f.revisions {
		if old.ChangesetID == r.ChangesetID && old.NodeID == r.NodeID {
			// The database's: a changeset's version goes on only from its own revision.
			if r.Base == nil || *r.Base != old.Revision {
				return fmt.Errorf("fake: changeset %s's version of %s is at %d, not at the write's base", r.ChangesetID, r.NodeID, old.Revision)
			}
			r.ID, r.Base = old.ID, old.Base
			f.revisions[i] = r
			return nil
		}
	}
	f.revisions = append(f.revisions, r)
	return nil
}

func (f *fakeStore) CreateSession(ctx context.Context, s app.EditSession) error {
	f.record(ctx, "CreateSession")
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeStore) LockSession(ctx context.Context, id uuid.UUID) (app.EditSession, error) {
	f.record(ctx, "LockSession")
	s, ok := f.sessions[id]
	if !ok {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) SetSessionWrite(ctx context.Context, id, changesetID uuid.UUID, revision int) error {
	f.record(ctx, "SetSessionWrite")
	s := f.sessions[id]
	s.ChangesetID, s.Revision = changesetID, revision
	f.sessions[id] = s
	return nil
}

func (f *fakeStore) FindLiveSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	f.record(ctx, "FindLiveSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) HeartbeatSession(ctx context.Context, id, userID uuid.UUID, now, until time.Time) (app.EditSession, error) {
	f.record(ctx, "HeartbeatSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	s.ExpiresAt = until
	f.sessions[id] = s
	return s, nil
}

// EndSession deletes the caller's session alive at now, or their
// tombstone.
func (f *fakeStore) EndSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	f.record(ctx, "EndSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || s.EndedReason == "" && !s.Alive(now) {
		return app.EditSession{}, app.ErrNotFound
	}
	delete(f.sessions, id)
	return s, nil
}

func (f *fakeStore) FindEndedSession(ctx context.Context, id, userID uuid.UUID) (app.EditSession, error) {
	f.record(ctx, "FindEndedSession")
	s, ok := f.sessions[id]
	if !ok || s.UserID != userID || s.EndedReason == "" {
		return app.EditSession{}, app.ErrNotFound
	}
	return s, nil
}

// AliveSessionsOf are the pages' sessions alive at now, by when they
// opened.
func (f *fakeStore) AliveSessionsOf(ctx context.Context, ids []uuid.UUID, now time.Time) ([]app.EditSession, error) {
	f.record(ctx, "AliveSessionsOf")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NodeID) && s.Alive(now) {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b app.EditSession) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	return out, nil
}

// DeleteExpiredSessionsOf deletes the page's sessions expired at now,
// tombstones among them.
func (f *fakeStore) DeleteExpiredSessionsOf(ctx context.Context, id uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteExpiredSessionsOf")
	for sid, s := range f.sessions {
		if s.NodeID == id && !s.ExpiresAt.After(now) {
			delete(f.sessions, sid)
		}
	}
	return nil
}

// EndAliveSessions makes tombstones of the sessions e ends, at e.At or
// their opening, the later; it sorts them by when they opened, for the
// tests' sake, as the SQL does not.
func (f *fakeStore) EndAliveSessions(ctx context.Context, e app.SessionsEnd) ([]app.EditSession, error) {
	f.record(ctx, "EndAliveSessions")
	var out []app.EditSession
	for id, s := range f.sessions {
		if s.NodeID != e.NodeID || !s.Alive(e.At) || e.UserID != nil && s.UserID != *e.UserID {
			continue
		}
		s.EndedReason, s.EndedByID, s.EndedAt = e.Reason, e.By, e.At
		if s.CreatedAt.After(e.At) {
			s.EndedAt = s.CreatedAt
		}
		if e.Until.After(s.ExpiresAt) {
			s.ExpiresAt = e.Until
		}
		f.sessions[id] = s
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b app.EditSession) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

// DeleteExpiredSessions deletes at most batch of the sessions expired at
// now, tombstones among them; held are skipped, as rows another
// transaction holds.
func (f *fakeStore) DeleteExpiredSessions(ctx context.Context, now time.Time, batch int) (int, error) {
	f.record(ctx, "DeleteExpiredSessions")
	n := 0
	for id, s := range f.sessions {
		if n < batch && !s.ExpiresAt.After(now) && !f.held[id] {
			delete(f.sessions, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, _ uuid.UUID, _ time.Time) error {
	f.record(ctx, "DeleteNotebooksPages")
	for id, n := range f.nodes {
		if slices.Contains(ids, n.NotebookID) {
			delete(f.nodes, id)
			delete(f.contents, id)
		}
	}
	return nil
}

func (f *fakeStore) QueueSearchOfNotebooks(ctx context.Context, ids []uuid.UUID, _ time.Time) error {
	f.record(ctx, "QueueSearchOfNotebooks")
	f.queuedNotebooks = append(f.queuedNotebooks, ids...)
	return nil
}

// queuing is a QueueSearch: the nodes of a notebook, in the order given,
// and whether in bulk.
type queuing struct {
	notebook uuid.UUID
	ids      []uuid.UUID
	bulk     bool
}

func (f *fakeStore) QueueSearch(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, bulk bool) error {
	f.record(ctx, "QueueSearch")
	f.queued = append(f.queued, queuing{notebook: notebookID, ids: slices.Clone(ids), bulk: bulk})
	return nil
}

func (f *fakeStore) DeleteNotebookSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	f.record(ctx, "DeleteNotebookSessions")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NotebookID) {
			out = append(out, s)
			delete(f.sessions, s.ID)
		}
	}
	slices.SortFunc(out, func(a, b app.EditSession) int { return a.ID.Compare(b.ID) })
	return out, nil
}

func (f *fakeStore) DeleteNodeSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	f.record(ctx, "DeleteNodeSessions")
	var out []app.EditSession
	for _, s := range f.sessions {
		if slices.Contains(ids, s.NodeID) {
			out = append(out, s)
			delete(f.sessions, s.ID)
		}
	}
	return out, nil
}

// fakeNames gives each account's display name.
type fakeNames map[uuid.UUID]string

func (n fakeNames) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string)
	for _, id := range ids {
		if name, ok := n[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

// fakeAuthorizer grants the actions of grants, answers forbidden for those
// of forbidden, and the rest not visible.
type fakeAuthorizer struct {
	*recorder
	grants    map[shared.Action]bool
	forbidden map[shared.Action]bool
	targets   []shared.Target
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, _ shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	f.record(ctx, "Authorize "+string(action))
	f.targets = append(f.targets, t)
	switch {
	case f.grants[action]:
		return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookEditor}, nil
	case f.forbidden[action]:
		return shared.Grant{}, shared.Forbidden()
	}
	return shared.Grant{}, shared.ErrNotVisible
}

// named is a registrant's call as the recorder keeps it: with its name,
// when it has one, so a test of several sees their order.
func named(call, name string) string {
	if name == "" {
		return call
	}
	return call + " " + name
}

// guard records the steps it sees and refuses them with err when set.
type guard struct {
	*recorder
	name  string
	err   error
	steps []app.Step
}

func (g *guard) GuardWrite(ctx context.Context, s app.Step) error {
	g.record(ctx, named("GuardWrite "+string(s.Operation), g.name))
	g.steps = append(g.steps, s)
	return g.err
}

// observer records the events it follows and answers err.
type observer struct {
	*recorder
	name   string
	err    error
	events []app.Event
}

func (o *observer) PagesChanged(ctx context.Context, e app.Event) error {
	o.record(ctx, named("PagesChanged", o.name))
	o.events = append(o.events, e)
	return o.err
}

// participant records the steps it follows. When rename is set, it
// renames that node to name through the unit; when retitle is, it renames
// the node the step changed, adding " (retitled)" to its name; when write
// is, it writes that content through the unit, and keeps the revision.
// It defers deferred, when set, first.
type participant struct {
	*recorder
	label    string
	steps    []app.Step
	rename   *uuid.UUID
	name     string
	retitle  bool
	write    *app.ContentWrite
	revision int
	deferred func()
}

func (p *participant) Participate(ctx context.Context, s app.Step, u app.Appender) error {
	p.record(ctx, named("Participate "+string(s.Operation), p.label))
	p.steps = append(p.steps, s)
	if p.deferred != nil {
		u.Defer(p.deferred)
	}
	switch {
	case p.rename != nil:
		_, err := u.Rename(ctx, *p.rename, p.name)
		return err
	case p.retitle:
		c := s.Changes[0]
		_, err := u.Rename(ctx, c.NodeID, c.After.Name+" (retitled)")
		return err
	case p.write != nil:
		var err error
		p.revision, err = u.WriteContent(ctx, *p.write)
		return err
	}
	return nil
}

// vetoer records the openings it sees and refuses them with err when set.
type vetoer struct {
	*recorder
	name     string
	err      error
	openings []app.SessionOpening
}

func (v *vetoer) VetoEditSession(ctx context.Context, o app.SessionOpening) error {
	v.record(ctx, named("VetoEditSession", v.name))
	v.openings = append(v.openings, o)
	return v.err
}

// subscriber records the openings and the ends it follows and answers
// err.
type subscriber struct {
	*recorder
	name   string
	err    error
	opened []app.SessionOpened
	ended  []app.SessionEnded
}

func (s *subscriber) EditSessionOpened(ctx context.Context, o app.SessionOpened) error {
	s.record(ctx, named("EditSessionOpened", s.name))
	s.opened = append(s.opened, o)
	return s.err
}

func (s *subscriber) EditSessionEnded(ctx context.Context, e app.SessionEnded) error {
	s.record(ctx, named("EditSessionEnded", s.name))
	s.ended = append(s.ended, e)
	return s.err
}

// fixture is the fakes over one recorder: alice in the workspace acme with
// its notebook eng.
type fixture struct {
	rec        *recorder
	tx         *fakeTx
	clock      *tickingClock
	workspaces fakeWorkspaces
	notebooks  fakeNotebooks
	store      *fakeStore
	log        *fakeHistory
	md         *fakeMarkdown
	budget     *fakeBudget
	auth       *fakeAuthorizer
	logs       *bytes.Buffer
	guards     []app.WriteGuard
	partakers  []app.Participant
	observers  []app.PageObserver
	vetoers    []app.EditSessionVetoer
	enders     []app.EditSessionSubscriber
	names      fakeNames
	infos      map[uuid.UUID]app.AttachmentInfo
	// wait is the parse budget's wait.
	wait      time.Duration
	alice     uuid.UUID
	acme, eng uuid.UUID
}

func newFixture() *fixture {
	rec := &recorder{}
	f := &fixture{
		rec: rec, tx: &fakeTx{}, clock: &tickingClock{},
		workspaces: fakeWorkspaces{recorder: rec, slugs: map[string]uuid.UUID{}, gone: map[uuid.UUID]bool{}},
		store: &fakeStore{recorder: rec, nodes: map[uuid.UUID]domain.Node{}, contents: map[uuid.UUID]app.Content{},
			items: map[uuid.UUID]app.Item{}, touched: map[uuid.UUID]time.Time{}, marked: map[uuid.UUID]domain.ChangesetMoves{},
			sessions: map[uuid.UUID]app.EditSession{}, held: map[uuid.UUID]bool{}, trash: map[uuid.UUID]app.AnyNode{},
			counts: map[uuid.UUID][3]int{}, trashed: map[uuid.UUID]app.Content{}, firsts: map[[2]uuid.UUID]time.Time{}},
		auth: &fakeAuthorizer{recorder: rec, grants: map[shared.Action]bool{}, forbidden: map[shared.Action]bool{}},
		logs: &bytes.Buffer{}, names: fakeNames{}, alice: uuid.NewV7(), acme: uuid.NewV7(), eng: uuid.NewV7(),
	}
	f.notebooks = fakeNotebooks{recorder: rec, workspaces: map[uuid.UUID]uuid.UUID{f.eng: f.acme}, gone: map[uuid.UUID]bool{}}
	f.log = &fakeHistory{recorder: rec, store: f.store, notebook: f.eng, versions: map[domain.VersionRef]string{}}
	f.md = &fakeMarkdown{recorder: rec}
	f.budget = &fakeBudget{recorder: rec}
	f.wait = time.Second
	return f
}

func (f *fixture) logger() *slog.Logger { return slog.New(slog.NewTextHandler(f.logs, nil)) }

func (f *fixture) writer() *app.Writer {
	return app.NewWriter(app.WriterDeps{
		Tx: f.tx, Clock: f.clock, Auth: f.auth, Workspaces: f.workspaces, Notebooks: f.notebooks, Nodes: f.store,
		NodeWriter: f.store, Trash: f.store, History: f.log, Changesets: f.store, SessionWriter: f.store, Search: f.store, Names: f.names,
		Guards: f.guards, Participants: f.partakers, Observers: f.observers, SessionVetoers: f.vetoers,
		SessionSubscribers: f.enders,
	})
}

func (f *fixture) parser() *app.ContentParser {
	return app.NewContentParser(f.writer(), f.md, f.budget, f.wait)
}

// grant has the authorizer allow actions.
func (f *fixture) grant(actions ...shared.Action) {
	for _, a := range actions {
		f.auth.grants[a] = true
	}
}

// page adds a page of eng to the store, under parent, without recording a
// call: it was there before the test.
func (f *fixture) page(name string, parent *uuid.UUID, order float64) domain.Node {
	title, err := domain.CheckTitle("title", name)
	if err != nil {
		panic(err)
	}
	at := now().Add(-time.Hour)
	n := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, ParentID: parent, Kind: domain.KindPage, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: at, UpdatedAt: at}
	f.store.nodes[n.ID] = n
	sum := sha256.Sum256(nil)
	f.store.contents[n.ID] = app.Content{NodeID: n.ID, Revision: 1, Hash: sum[:], By: f.alice, At: at}
	return n
}

// asAlice is a context with alice as the caller by a sign-in session.
func (f *fixture) asAlice() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
}

// called reports whether a call was recorded.
func (f *fixture) called(call string) bool {
	return slices.Contains(f.rec.calls, call)
}

// RootByKey finds a root of the notebook by its key, a call (M9/P1).
func (f *fakeStore) RootByKey(ctx context.Context, notebookID uuid.UUID, key string) (domain.Node, bool, error) {
	f.record(ctx, "RootByKey")
	return f.byKey(notebookID, nil, key)
}

// ChildByKey finds a child of the parent by its key, a call.
func (f *fakeStore) ChildByKey(ctx context.Context, notebookID, parentID uuid.UUID, key string) (domain.Node, bool, error) {
	f.record(ctx, "ChildByKey")
	return f.byKey(notebookID, &parentID, key)
}

// PathsOf writes the paths of the nodes of the notebook among ids, a call
// with their count; errs["PathsOf"] fails it.
func (f *fakeStore) PathsOf(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.record(ctx, fmt.Sprintf("PathsOf %d", len(ids)))
	if err := f.errs["PathsOf"]; err != nil {
		return nil, err
	}
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		n, ok := f.nodes[id]
		if !ok || n.NotebookID != notebookID {
			continue
		}
		var ancestors []domain.Node
		for p := n.ParentID; p != nil; p = f.nodes[*p].ParentID {
			ancestors = append([]domain.Node{f.nodes[*p]}, ancestors...)
		}
		out[id] = domain.PathOf(ancestors, n)
	}
	return out, nil
}

// LinePaths writes the paths of the nodes of the notebook among ids step
// by step, as PathsOf does, a call with their count.
func (f *fakeStore) LinePaths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]domain.PathStep, error) {
	f.record(ctx, fmt.Sprintf("LinePaths %d", len(ids)))
	out := map[uuid.UUID][]domain.PathStep{}
	for _, id := range ids {
		n, ok := f.nodes[id]
		if !ok || n.NotebookID != notebookID {
			continue
		}
		var steps []domain.PathStep
		for cur := &n; cur != nil; {
			steps = append([]domain.PathStep{{ID: cur.ID, Key: cur.NameKey, Name: cur.Name}}, steps...)
			if cur.ParentID == nil {
				break
			}
			p := f.nodes[*cur.ParentID]
			cur = &p
		}
		out[id] = steps
	}
	return out, nil
}

// MarkChangesetMoves marks the changeset's renames and moves, each kept
// once set, as the statement does; each a call.
func (f *fakeStore) MarkChangesetMoves(ctx context.Context, id uuid.UUID, m domain.ChangesetMoves) error {
	f.record(ctx, "MarkChangesetMoves")
	was := f.marked[id]
	f.marked[id] = domain.ChangesetMoves{Renames: was.Renames || m.Renames, Moves: was.Moves || m.Moves}
	return nil
}

// DropUnchanged deletes the changeset's items whose node is where it was,
// then the changeset when it holds no item and no version, as the two
// statements do.
func (f *fakeStore) DropUnchanged(ctx context.Context, id uuid.UUID) (bool, error) {
	f.record(ctx, "DropUnchanged")
	same := func(it app.Item) bool {
		c := it.Change
		return it.ChangesetID == id && c.Before != nil && c.After != nil && c.Before.Same(*c.After)
	}
	f.recorded = slices.DeleteFunc(f.recorded, same)
	maps.DeleteFunc(f.items, func(_ uuid.UUID, it app.Item) bool { return same(it) })
	if slices.ContainsFunc(f.recorded, func(it app.Item) bool { return it.ChangesetID == id }) ||
		slices.ContainsFunc(f.revisions, func(r app.Revision) bool { return r.ChangesetID == id }) {
		return false, nil
	}
	f.changesets = slices.DeleteFunc(f.changesets, func(c app.Changeset) bool { return c.ID == id })
	return true, nil
}

func (f *fakeStore) byKey(notebookID uuid.UUID, parentID *uuid.UUID, key string) (domain.Node, bool, error) {
	for _, n := range f.nodes {
		if n.NotebookID == notebookID && sameParent(n.ParentID, parentID) && n.NameKey == key {
			return n, true, nil
		}
	}
	return domain.Node{}, false, nil
}

// AliveNodes reads which nodes are in the store, a call with their count.
func (f *fakeStore) AliveNodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	f.record(ctx, fmt.Sprintf("AliveNodes %d", len(ids)))
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if _, ok := f.nodes[id]; ok {
			out[id] = true
		}
	}
	return out, nil
}

// AgentsPage finds the notebook's root of the key, with its title and its
// content's revision, a call.
func (f *fakeStore) AgentsPage(ctx context.Context, notebookID uuid.UUID, key string) (uuid.UUID, domain.Kind, string, int, bool, error) {
	f.record(ctx, "AgentsPage")
	n, ok, _ := f.byKey(notebookID, nil, key)
	if !ok {
		return uuid.UUID{}, "", "", 0, false, nil
	}
	return n.ID, n.Kind, n.Name, f.contents[n.ID].Revision, true, nil
}

// PageStates reads the pages' contents' states, a call with their count.
func (f *fakeStore) PageStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageState, error) {
	f.record(ctx, fmt.Sprintf("PageStates %d", len(ids)))
	out := map[uuid.UUID]app.PageState{}
	for _, id := range ids {
		if c, ok := f.contents[id]; ok {
			out[id] = app.PageState{Revision: c.Revision, Bytes: c.ByteSize, UpdatedAt: c.At, Hash: c.Hash}
		}
	}
	return out, nil
}
