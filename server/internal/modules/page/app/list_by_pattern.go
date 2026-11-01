package app

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListByPattern lists a notebook's nodes for MCP's list (M9 design 4.9).
type ListByPattern struct {
	notebooks   Notebooks
	nodes       Nodes
	states      PageStates
	attachments AttachmentInfos
	auth        shared.Authorizer
	// maxWork bounds a glob's matching a call: domain.MaxGlobWork, a
	// test's less.
	maxWork int64
}

// NewListByPattern returns the use case, its globs' work a call at most
// domain.MaxGlobWork.
func NewListByPattern(notebooks Notebooks, nodes Nodes, states PageStates, attachments AttachmentInfos, auth shared.Authorizer,
) *ListByPattern {
	return &ListByPattern{notebooks: notebooks, nodes: nodes, states: states, attachments: attachments, auth: auth,
		maxWork: domain.MaxGlobWork}
}

// Listing is a page of list: each item with the cursor that goes on after
// it, so the adapter can stop at the answer's bound; Next, when more may
// follow, the cursor after the page: its last item found, or the last node
// a glob examined when its work ran out, which may be past every item, or
// with none. Hint is set for a glob without wildcards that matched nothing
// while its path with ".md" is a page.
type Listing struct {
	Items []Listed
	Next  string
	Hint  string
}

// Listed is an item of list: a page with its content's state, or an
// attachment with its type and size.
type Listed struct {
	Virtual     bool
	Path        string
	Node        domain.Node
	HasChildren bool
	Page        *PageState
	Attachment  *AttachmentInfo
	Cursor      string
}

// Execute lists the nodes of notebookID that pattern names, the page that
// limit and cursor ask for, unlocked, without a transaction (M9/P1 design
// 3.10.4). Its order: the actor; the pattern, 422; the cursor, decoded
// into the payload the pattern's kind lists by, 400; the limit, 422; the
// notebook, notebook.not_found, node.list; the tree, read whole; for a
// path ending with "/", its page, page.not_found, and its children in the
// tree's order after the cursor, limit + 1 of them; for a glob, the walk
// in path-key order from the cursor, to limit + 1 matches or the bound of
// its work; then the page's pages' states and attachments' infos. An item
// deleted between the reads is left out. A path is made only for an item
// answered, never for each node of the tree.
func (l *ListByPattern) Execute(ctx context.Context, notebookID uuid.UUID, pattern string, limit *int, cursor *string) (Listing, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Listing{}, err
	}
	var dir domain.Path
	var glob domain.Glob
	children := pattern == "" || strings.HasSuffix(pattern, "/")
	if children {
		dir, err = domain.ParsePath("pattern", pattern, true)
	} else {
		glob, err = domain.ParseGlob("pattern", pattern)
	}
	if err != nil {
		return Listing{}, err
	}
	var after domain.ChildrenCursor
	var from domain.GlobCursor
	if cursor != nil {
		var payload any = &from
		if children {
			payload = &after
		}
		if err := shared.DecodeCursor(*cursor, payload); err != nil {
			return Listing{}, err
		}
	}
	if children {
		if err := after.Check(); err != nil {
			return Listing{}, err
		}
	}
	size, err := domain.CheckListLimit(limit)
	if err != nil {
		return Listing{}, err
	}
	if err := decideNotebook(ctx, l.notebooks, l.auth, actor, domain.ActionList, notebookID); err != nil {
		return Listing{}, err
	}
	state, ok, err := l.notebooks.StateOf(ctx, notebookID)
	if err != nil {
		return Listing{}, err
	}
	if !ok {
		return Listing{}, domain.ErrNotebookNotFound
	}
	nodes, err := l.nodes.ListNodes(ctx, notebookID)
	if err != nil {
		return Listing{}, err
	}
	t := treeOf(nodes)
	t.addVirtual(state)
	var out Listing
	var found []*domain.GlobNode
	if children {
		found, out.Next, err = t.children(dir, cursor != nil, after, size)
	} else {
		found, out.Next, out.Hint, err = l.walk(ctx, t, glob, cursor != nil, from.K, size)
	}
	if err != nil {
		return Listing{}, err
	}
	out.Items, err = l.items(ctx, t, found, children)
	return out, err
}

// walk is the glob's matches after the path key from (from the start
// without a cursor), at most size and one more to tell another page, and
// the cursor after them: the size-th's when there is one more, the last
// node examined when the work ran out; and a literal glob's hint when it
// matched nothing.
func (l *ListByPattern) walk(ctx context.Context, t tree, g domain.Glob, resumed bool, from string, size int) (
	[]*domain.GlobNode, string, string, error,
) {
	var found []*domain.GlobNode
	w, err := g.Walk(ctx, t.roots, from, l.maxWork, func(_ []string, n *domain.GlobNode) bool {
		found = append(found, n)
		return len(found) <= size
	})
	if err != nil {
		return nil, "", "", err
	}
	var next string
	switch {
	case len(found) > size:
		found = found[:size]
		next, err = globCursor(t.keyOf(found[size-1]))
	case w.Stopped:
		next, err = globCursor(w.Last)
	case len(found) == 0 && !resumed:
		return nil, "", t.pageHint(g), nil
	}
	return found, next, "", err
}

// globCursor is the cursor after the path key k.
func globCursor(k string) (string, error) {
	return shared.EncodeCursor(domain.GlobCursor{K: k})
}

// items are the listed of nodes, each with its path and its cursor; a
// page's state and an attachment's info read for them, an item without
// either left out, or an attachment not alive: it was deleted after the
// tree was read.
func (l *ListByPattern) items(ctx context.Context, t tree, nodes []*domain.GlobNode, children bool) ([]Listed, error) {
	var pages, attachments []uuid.UUID
	for _, n := range nodes {
		if t.virtual[n] != 0 {
			continue
		}
		if n.Node.Kind == domain.KindPage {
			pages = append(pages, n.Node.ID)
		} else {
			attachments = append(attachments, n.Node.ID)
		}
	}
	var states map[uuid.UUID]PageState
	var infos map[uuid.UUID]AttachmentInfo
	var alive map[uuid.UUID]bool
	var err error
	if len(pages) > 0 {
		if states, err = l.states.PageStates(ctx, pages); err != nil {
			return nil, err
		}
	}
	if len(attachments) > 0 {
		if infos, err = l.attachments.Of(ctx, attachments); err != nil {
			return nil, err
		}
		if alive, err = l.states.AliveNodes(ctx, attachments); err != nil {
			return nil, err
		}
	}
	out := make([]Listed, 0, len(nodes))
	for _, n := range nodes {
		item := Listed{Path: t.pathOf(n), Node: *n.Node, HasChildren: len(n.Children) > 0}
		if t.virtual[n] != 0 {
			item.Virtual = true
		} else if n.Node.Kind == domain.KindPage {
			state, ok := states[n.Node.ID]
			if !ok {
				continue
			}
			item.Page = &state
		} else {
			info, ok := infos[n.Node.ID]
			if !ok || !alive[n.Node.ID] {
				continue
			}
			item.Attachment = &info
		}
		if children {
			item.Cursor, err = t.childrenCursor(n)
		} else {
			item.Cursor, err = globCursor(t.keyOf(n))
		}
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// tree is a notebook's nodes as list reads them: by parent, once.
type tree struct {
	notebook domain.NotebookState
	virtual  map[*domain.GlobNode]int
	roots    []*domain.GlobNode
	byID     map[uuid.UUID]*domain.GlobNode
}

// treeOf groups nodes by parent; a node whose parent is not among them
// cannot be reached, and is left out.
func treeOf(nodes []domain.Node) tree {
	t := tree{byID: make(map[uuid.UUID]*domain.GlobNode, len(nodes))}
	for i := range nodes {
		t.byID[nodes[i].ID] = &domain.GlobNode{Node: &nodes[i]}
	}
	for i := range nodes {
		n := t.byID[nodes[i].ID]
		switch p := nodes[i].ParentID; {
		case p == nil:
			t.roots = append(t.roots, n)
		case t.byID[*p] != nil:
			t.byID[*p].Children = append(t.byID[*p].Children, n)
		}
	}
	return t
}

// ancestors are n's ancestors from the root.
func (t tree) ancestors(n *domain.GlobNode) []domain.Node {
	var out []domain.Node
	for p := n.Node.ParentID; p != nil; p = t.byID[*p].Node.ParentID {
		out = append(out, *t.byID[*p].Node)
	}
	slices.Reverse(out)
	return out
}

// pathOf is n's path, as its names are.
func (t tree) pathOf(n *domain.GlobNode) string {
	return domain.PathOf(t.ancestors(n), *n.Node)
}

// keyOf is n's path key: its ancestors' keys and its own, a page's with
// ".md", joined by "/".
func (t tree) keyOf(n *domain.GlobNode) string {
	var keys []string
	for _, a := range t.ancestors(n) {
		keys = append(keys, a.NameKey)
	}
	own := n.Node.NameKey
	if n.Node.Kind == domain.KindPage {
		own += ".md"
	}
	return strings.Join(append(keys, own), "/")
}

// children are the children of the page dir names, the root's for none,
// in the tree's order (sort order, then id, as domain.PreOrder and the
// web's tree), after the cursor's item when resumed, size at most, and the
// cursor after the size-th when one more follows.
func (t tree) children(dir domain.Path, resumed bool, after domain.ChildrenCursor, size int) ([]*domain.GlobNode, string, error) {
	level := t.roots
	var chain []domain.Node
	lookup := domain.VirtualLookup(t.notebook, t.lookup)
	for d, key := range dir.Keys {
		n, ok, err := lookup(chain, key)
		if err != nil {
			return nil, "", err
		}
		switch {
		case !ok && d == len(dir.Keys)-1 && strings.HasSuffix(key, ".md"):
			hinted, exists, err := lookup(chain, strings.TrimSuffix(key, ".md"))
			if err != nil {
				return nil, "", err
			}
			if !exists {
				return nil, "", &domain.PathNotFound{Path: dir.Written}
			}
			if hinted.Kind != domain.KindPage {
				return nil, "", &domain.PathNotFound{Path: dir.Written, Hint: domain.AttachmentHint(domain.PathOf(chain, hinted))}
			}
			return nil, "", &domain.PathNotFound{Path: dir.Written, Hint: domain.ChildrenHint(domain.PathOf(chain, hinted))}
		case !ok:
			return nil, "", &domain.PathNotFound{Path: dir.Written}
		case n.Kind != domain.KindPage:
			return nil, "", &domain.PathNotFound{Path: dir.Written, Hint: domain.AttachmentHint(domain.PathOf(chain, n))}
		}
		chain = append(chain, n)
		level = t.byID[n.ID].Children
	}
	level = slices.DeleteFunc(slices.Clone(level), func(n *domain.GlobNode) bool { return t.virtual[n] != 0 })
	sorted := slices.SortedFunc(slices.Values(level), func(a, b *domain.GlobNode) int {
		return cmp.Or(cmp.Compare(a.Node.SortOrder, b.Node.SortOrder), a.Node.ID.Compare(b.Node.ID))
	})
	if resumed && after.V == 0 {
		start, _ := slices.BinarySearchFunc(sorted, after, func(n *domain.GlobNode, c domain.ChildrenCursor) int {
			return cmp.Or(cmp.Compare(n.Node.SortOrder, c.O), n.Node.ID.Compare(c.I), -1)
		})
		sorted = sorted[start:]
	}
	if len(dir.Keys) == 0 && (!resumed || after.V != 0) {
		var prefix []*domain.GlobNode
		// The roots' insertion order is index then log, independent of real sort values.
		for _, n := range t.roots {
			if v := t.virtual[n]; v > after.V {
				prefix = append(prefix, n)
			}
		}
		sorted = append(prefix, sorted...)
	}
	if len(sorted) <= size {
		return sorted, "", nil
	}
	next, err := t.childrenCursor(sorted[size-1])
	return sorted[:size], next, err
}

// pageHint is the hint of a literal glob that matched nothing while its
// path with ".md" is a page: list its children with a trailing "/".
func (t tree) pageHint(g domain.Glob) string {
	keys, literal := g.Literal()
	if !literal {
		return ""
	}
	level := t.roots
	for i, key := range keys {
		j := slices.IndexFunc(level, func(n *domain.GlobNode) bool {
			return t.virtual[n] == 0 && n.Node.NameKey == key && n.Node.Kind == domain.KindPage
		})
		if j < 0 {
			return ""
		}
		if i == len(keys)-1 {
			return domain.ChildrenHint(t.pathOf(level[j]))
		}
		level = level[j].Children
	}
	return ""
}

// addVirtual is only for listing: grep's treeOf remains a tree of entities.
func (t *tree) addVirtual(state domain.NotebookState) {
	t.notebook = state
	if state.Mode != domain.ModeLLMWiki {
		return
	}
	t.virtual = make(map[*domain.GlobNode]int, 2)
	for i, name := range []string{"index", "log"} {
		n := &domain.GlobNode{Node: &domain.Node{Kind: domain.KindPage, Name: name, NameKey: name}}
		t.virtual[n] = i + 1
		t.roots = append(t.roots, n)
	}
}

// childrenCursor keeps generated progress separate from real sort/id values.
func (t tree) childrenCursor(n *domain.GlobNode) (string, error) {
	if v := t.virtual[n]; v != 0 {
		return shared.EncodeCursor(domain.ChildrenCursor{V: v})
	}
	return shared.EncodeCursor(domain.ChildrenCursor{O: n.Node.SortOrder, I: n.Node.ID})
}

// lookup reads only real nodes; VirtualLookup decides generated root keys.
func (t tree) lookup(chain []domain.Node, key string) (domain.Node, bool, error) {
	level := t.roots
	if len(chain) > 0 {
		level = t.byID[chain[len(chain)-1].ID].Children
	}
	for _, n := range level {
		if t.virtual[n] == 0 && n.Node.NameKey == key {
			return *n.Node, true, nil
		}
	}
	return domain.Node{}, false, nil
}
