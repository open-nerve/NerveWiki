package domain

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A batch's plan (M9 design 4.11; M9/P3 design 3.4): its ops simulated in
// their order on the notebook's tree as read unlocked, in two passes, the
// tree's and then the contents', the first op that fails deciding the
// answer, before its unit does them again under the notebook's lock.

const (
	// MaxBatchReadBytes bounds the contents a batch's plan reads whole,
	// those of the pages whose first content op is an edit, or a write
	// that the renames and moves before it reach (ReadWhole), counted by
	// their sizes before any is read (M9/P3 design 3.4.4, 3.8), and none of
	// a page grown past its size since read (M9 Codex review D2): what a
	// unit writes and what a batch brings, MaxUnitBytes and
	// MaxBatchStringBytes. A page read past them would have to lose more
	// than its edits bring for the unit to be within its bounds.
	MaxBatchReadBytes = MaxUnitBytes + MaxBatchStringBytes
	// MaxBatchParseBytes bounds the contents a batch's plan parses to
	// rewrite links before the final ones, each page and hash once, a lone
	// page's too (M9/P3 design 3.4.4): the largest page rewritten once,
	// its content and the read back of what the rewrite wrote.
	MaxBatchParseBytes = 2 * MaxContentBytes
	// MaxBatchRewrites bounds the pages a batch's deferred rewrite writes
	// again (M9/P3 design 3.8.2): the pages of the index whose links its
	// renames and moves reach, and the pages its content ops decided with a
	// link their own changes meet, each they wrote a version of counting
	// WrittenRewriteWeight, each they put back to the bytes it held before
	// one, as the page of the index it is in separate calls (Codex review
	// R1), and one whose links they meet none of nothing (M9/P3 review
	// B1-M2). Counted before any is parsed; past it the batch is
	// page.batch_too_large. A single move is not bounded, as REST's. M8's
	// rule sets it: the largest tier of bootstrap's BenchmarkBatchRewrites
	// whose unit takes about 4 s at most here, pages of the full load (84
	// KB, 200 links), the median of three: 100 pages 0.86 s, 200 1.90 s, 250
	// 2.24 s, 300 2.70 s, 400 3.56 s, 500 4.44 s; 1,000 answered
	// server_busy, the shares of the parse budget the unit holds for the
	// pages it wrote, until its end, past the budget (2026-10-20, Apple M5
	// Max).
	MaxBatchRewrites = 400
	// WrittenRewriteWeight is what a page the batch wrote counts in
	// MaxBatchRewrites, in pages of the index (M9/P3 review B-M3): its
	// rewrite finds the links its own changes reach from its facts, on the
	// tree its last content op left, and costs more than an index page's.
	// BenchmarkBatchRewrites' mix, pages of some 76 KB with 100 links each
	// into distinct nodes of a folder of 10,000 moved after the edits, the
	// median of three, in two runs: 99 written pages and 301 of the index
	// 5.59 s, past the 4 s the bound is set for; the 99 written alone 2.21
	// and 2.12 s, 400 of the index alone 5.17 and 4.46 s; a written page
	// 1.4 to 1.9 times an index page, rounded up. So counted, the bound's
	// 99 written and 202 of the index take 4.36 s (2026-10-20, Apple M5
	// Max).
	WrittenRewriteWeight = 2
	// MaxBatchRewriteLinks bounds the links a batch's deferred rewrite
	// writes again (M9/P3 review B1-M3: the unit's time follows them): those
	// of the index's pages its renames and moves reach that domain.Rewrites
	// writes, and those of the pages its content ops decided that their own
	// changes meet, each whose target the batch did not delete, counted as
	// MaxBatchRewrites' pages are, before any is parsed; past it the batch
	// is page.batch_too_large. A single move is not bounded, as REST's.
	// M8's rule sets it: the largest dense tier of BenchmarkBatchRewrites
	// whose unit takes about 4 s at most here, 400 pages of the index of
	// bare links, some 18 bytes each, into a folder of 10,000 moved alone,
	// the median of three: 100 links a page 2.14 s, 250 3.06 and 3.09 s,
	// 300 3.44 and 3.45 s, 375 4.28 and 4.25 s, 500 4.74 and 4.82 s, 750
	// 6.75 s, 1,000 8.30 s, 1,500 11.47 s; 2,000 and 2,500 ran out of the
	// request's 15 s (2026-10-20, Apple M5 Max). A page's bytes add their
	// parse: 400 of 76 KB with 100 links each took 4.2 to 4.6 s.
	MaxBatchRewriteLinks = 120_000
	// MaxBatchRewriteBytes bounds the bytes of the pages a batch's deferred
	// rewrite writes again (M9/P3 fix round 5, B2-M1): those
	// MaxBatchRewrites counts, each by its content's size, a page the
	// batch's content ops decided by its final content's, once (its rewrite
	// parses that once, its facts the batch's), counted after their links,
	// before any is parsed; past it the batch is page.batch_too_large. A
	// single move is not bounded, as REST's. Each page's rewrite keeps its
	// facts' share of the parse budget, some tenth of its bytes, to the
	// unit's end, and parses the next with the budget taken now, server_busy
	// when it is not free, which no retry frees. So the bound leaves room
	// for a page's largest, MaxContentBytes, parsed last beside the shares
	// of the rest: of the bound's 32.8 MiB, the 399 pages before a page of 5
	// MiB keep some 2.8 MiB, 7.8 MiB with its parse, below the
	// configuration's budget, 8 MiB (a smaller budget narrows it, as every
	// rewrite's). The 219 KiB left take a writing's ordinary growth, some
	// tens of bytes a link, and the shares of frontmatters' values. The
	// bound counts the pages, not their writings, which grow by the path of
	// each link's new place, up to some 2.5 KiB a link in a deep tree of
	// long names: what it does not see the rewrite refuses,
	// page.batch_too_large too, at a parse that would not fit in the budget
	// beside the shares its own pages keep, before it takes it (fix round 6,
	// B3-M1). M8's rule alone sets more: the bytes' tiers of bootstrap's
	// BenchmarkBatchRewrites, 400 pages of the index of one link each into
	// a folder of 10,000 moved alone, the median of three, in two runs: 76
	// KiB a page 3.50 and 3.54 s, 84 3.39 and 3.49, 96 3.42 and 3.50, 112
	// 3.61 and 4.08, 128 3.98 and 4.29, 160 4.71 and 4.90, 192 5.24 and
	// 5.75; 256 KiB and more answered server_busy, after 4.3 to 5.0 s. The
	// bound is the largest tier within both, 84 KiB a page: its bytes with
	// a page's largest last took 3.76 s; the 96 KiB tier's so answered
	// server_busy (2026-10-20, Apple M5 Max).
	MaxBatchRewriteBytes = MaxBatchRewrites * 84 << 10
)

// PathStep is a node on a path from the root: its id, title key and name.
type PathStep struct {
	ID        uuid.UUID
	Key, Name string
}

// Lead is where a link led at a moment of a batch's plan: the node it
// resolved to, none for nothing; whether ambiguously, the least id
// deciding between paths equally short; whether to an attachment. As
// linking's Resolution, which bootstrap copies field by field: a link
// ambiguous before and after is not written again (M9/P3 review A-M3).
type Lead struct {
	ID        uuid.UUID
	Ambiguous bool
	Asset     bool
}

// LinksBefore is where a page's links led at a moment of a batch's plan
// (M9/P3 design 3.4.4, 3.7): each link, by where its target starts, and
// the nodes they led to as they were then, their paths from the root.
type LinksBefore struct {
	Leads   map[int]Lead
	Targets map[uuid.UUID][]PathStep
}

// PageText is a page's content and its revision, read in one statement.
type PageText struct {
	Content  string
	Revision int
}

// PageHead is a page's revision, size and content hash, read in one
// statement without its content: a page whose first content op in a batch
// is a write, which replaces it whole with base_version, and without one
// holds its bytes already or is PathTaken, as write tells by size and hash.
type PageHead struct {
	Revision int
	Bytes    int
	Hash     []byte
}

// LinkRewriter writes again, for a batch's plan, a page's links that the
// renames and moves since its content was read reach (M9 design 4.11).
// Its parses are the plan's (ContentParser.Planned): facts kept by page
// and hash, so that a content parsed once is parsed no more; each parse
// is admitted by the plan first (BatchPlan.Admit), within its bounds.
type LinkRewriter interface {
	// Indexed is where the links of the page id led by the index as the
	// batch began, at revision: false when the index is not of revision or
	// not of the current extractor (M6's skips), its links then not
	// written again.
	Indexed(id uuid.UUID, revision int) (LinksBefore, bool, error)
	// Resolve is where the links of content, the page id's, lead on the
	// tree after the op at: the before of the page's next rewrite, and of
	// the deferred one (M9/P3 design 3.7).
	Resolve(id uuid.UUID, content string, at int) (LinksBefore, error)
	// Rewrite is content, the page id's, its links resolved as before
	// tells at the op since (-1: the index before the batch), written
	// again so that each leads where it led on the tree after the op
	// until: as the participant writes them (M6/P4), through the renames
	// and moves between the two (BatchPlan.MovesBetween).
	Rewrite(id uuid.UUID, content string, before LinksBefore, since, until int) (string, error)
}

// BatchPlan is a batch planned (M9 design 4.11): each op on the tree its
// plan simulates, the first op that fails and why, what the unit
// changes and parses, and, after Contents, each page's content.
type BatchPlan struct {
	Steps  []PlannedStep
	Failed *OpFailed // the first op that fails, of either pass; nil for none
	Items  int       // the changeset's items: created, deleted (merged deletions' too), moved and renamed nodes
	// Needs is the first op whose step the notebook's contract lets by only
	// with an action beyond the batch's own decision (M10/P1 design 3.22),
	// for the use case to ask before it reads, and to Refuse at that op when
	// denied; nil for none.
	Needs    *Need
	tree     *batchTree
	steps    []BatchStep
	notebook NotebookState
	// final is a failure that ends the plan where it is, its contents'
	// pass not run: the items' bound, the reads'.
	final     bool
	pages     map[uuid.UUID]*plannedPage
	order     []uuid.UUID // the pages, by their first content op
	createdBy map[uuid.UUID]int
	movedBy   map[uuid.UUID]int
	// parses are the contents the plan parsed, by page and hash, and their
	// bytes; multi tells that the batch writes more pages than one, whose
	// parses the unit's bounds count too; at is the op being planned.
	parses     map[parseKey]int
	parseBytes int
	multi      bool
	at         int
}

// PlannedStep is an op on the simulated tree: the node it acts on (a
// write's page, created or there; an edit's; what a move moves; what a
// delete deletes) and its path after the op, as written; the nodes its
// path, or a move's from, walked, the last the node when it was there;
// the pages a move's to's parent's line walked; the pages a write creates,
// root first, each with the id the use case gave it; the subtree a delete
// takes with the deletions merged into it, and its pages and attachments;
// where it merged into a later delete; where a move puts its node; for a
// content op, the content op of the same page before it (-1 for none) and
// whether a rename or a move between them reaches the page's links
// (Contents rewrites them first); what the op's tool would answer as
// changed at its step, and an edit's replacements.
type PlannedStep struct {
	// Facts keeps this item's operations at their own tree and content
	// state; a creation has one step per new page, root first.
	Facts         []StepFacts
	Node          uuid.UUID
	Path          string
	Found         []uuid.UUID
	Chain         []uuid.UUID
	Created       []CreatedPage
	Subtree       []uuid.UUID
	Pages, Assets int
	MergedInto    int // -1 for none
	To            Destination
	Previous      int
	Moved         bool
	Changed       bool
	Replacements  int
}

// Need is an action an op's step needs beyond the batch's own decision,
// as CheckContract's Verdict names it (M10/P1 design 3.22), and the node
// the step is about.
type Need struct {
	Op     int
	Action shared.Action
	NodeID uuid.UUID
}

// CreatedPage is a page a batch's write creates: its id, given by the use
// case, its parent, nil for the root, and its title.
type CreatedPage struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Title    string
}

// Destination is where a batch's move puts its node: under ParentID, nil
// for the root, named Name. (Placement is a trash item's.)
type Destination struct {
	ParentID *uuid.UUID
	Name     string
}

// PlannedPage is a page whose content a batch's ops write: its first and
// last content ops; whether the batch created it; its revision as the
// plan read it, none for one it created; and its content after its last
// content op, which the unit writes at its first.
type PlannedPage struct {
	ID          uuid.UUID
	First, Last int
	Created     bool
	Revision    int
	Content     string
	// Merged counts user byte changes made while this page was a source.
	Merged int
}

// plannedPage is a page's content as the contents' pass goes: its head
// for a page whose first content op is a write, until a write gives it
// its content; its text for one read whole (ReadWhole), as an edit's.
type plannedPage struct {
	id          uuid.UUID
	created     bool
	first, last int
	revision    int
	content     string
	merged      int
	known       bool
	whole       bool
	head        *PageHead
}

// parseKey is a content the plan parsed: its page and its hash.
type parseKey struct {
	page uuid.UUID
	hash [sha256.Size]byte
}

// deletion is a delete's group: its op, the node it deleted and that
// node's parent then, nil at the root, the nodes it and the deletions
// merged into it take, their pages and attachments, and the later delete
// it merged into, -1 for none.
type deletion struct {
	op            int
	parent        *uuid.UUID
	group         []uuid.UUID
	pages, assets int
	into          int
}

// PlanBatch simulates steps in their order on nodes, the notebook's tree
// alive as read unlocked (M9 design 4.11, the tree's pass), as each tool
// would on the tree the steps before it leave: a step's paths found on it,
// P2's rules (CreationOf, ParentChainOf, MoveKinds, RenameTitle, RelocationOf,
// DeletionOf), a delete that takes a subtree an earlier delete took from
// merged into it, B1 and B3. ids are the use case's, a UUIDv7 a segment
// of each write's path: a page a write creates at its segment d takes
// ids[i][d], the id its unit creates it with. A write without
// base_version at a page there is left to Contents, which compares its
// content. It counts the items as it goes, and the nodes the moves to
// another parent carry, their subtrees, each move's again (M9/P3 review
// A-I2: the plan and the unit read and change each move's subtree again):
// past MaxUnitNodes either fails the op that passes it, the second for
// more moves than one, a lone move as P2's move. Each op's step, after its
// own checks, is judged by the contract of the notebook n, as the mode
// guard judges it in the unit (M10/P1 design 3.22): a refusal fails the op,
// marked with its node; the first that needs an action beyond page.batch
// is Needs. The first op that fails ends the pass, its error at its op in
// Failed.
func PlanBatch(nodes []Node, steps []BatchStep, ids [][]uuid.UUID, n NotebookState) *BatchPlan {
	p := &BatchPlan{Steps: make([]PlannedStep, len(steps)), tree: newBatchTree(nodes), steps: steps, notebook: n,
		pages: map[uuid.UUID]*plannedPage{}, createdBy: map[uuid.UUID]int{}, movedBy: map[uuid.UUID]int{},
		parses: map[parseKey]int{}}
	for i := range p.Steps {
		p.Steps[i].MergedInto, p.Steps[i].Previous = -1, -1
	}
	pass := &treePass{p: p, cur: p.tree.view(-1), ids: ids, counted: map[uuid.UUID]bool{}, notebook: n}
	for i, s := range steps {
		err := pass.plan(i, s)
		if err == nil && p.Items > MaxUnitNodes {
			err, p.final = BatchTooLarge("nodes changed", p.Items, MaxUnitNodes, "split the batch"), true
		}
		if err != nil {
			p.Failed = &OpFailed{Op: i, Tool: s.Kind, Err: err}
			break
		}
	}
	return p
}

// treePass is the tree's pass as it goes: the view it walks, the ids
// given, the nodes counted as items, the deletions so far, the moves to
// another parent and the nodes they carried, and the notebook's state its
// contract is judged by.
type treePass struct {
	p         *BatchPlan
	cur       *treeView
	ids       [][]uuid.UUID
	counted   map[uuid.UUID]bool
	deletions []*deletion
	moves     int
	carried   int
	notebook  NotebookState
}

// judge is the contract on the op i's step, op of nodes, after the op's own
// checks and before the pass records it (M10/P1 design 3.22): the facts the
// unit's step will have, judged as the mode guard judges them. A refusal
// is the op's error, marked with its node; the first op that needs an
// action is Needs, and the pass goes on.
func (t *treePass) judge(i int, op Operation, nodes []NodeFacts) error {
	facts := StepFacts{Operation: op, Kind: ChangesetEdit, Nodes: nodes}
	if t.notebook.Mode == ModeLLMWiki && len(nodes) > 0 {
		var before, after []Ancestor
		if nodes[0].Before != nil {
			before = t.cur.line(nodes[0].Before.ParentID)
		}
		if nodes[0].After != nil {
			after = t.cur.line(nodes[0].After.ParentID)
		}
		// cur is the item-before tree. Its unchanged destination ancestors
		// and SourceFacts' changed parent graph give the item-after side.
		facts = SourceFacts(facts, t.notebook.SourceRootID, before, after, nil)
	}
	if op == OpCreate {
		for _, node := range facts.Nodes {
			t.p.Steps[i].Facts = append(t.p.Steps[i].Facts, StepFacts{Operation: op, Kind: ChangesetEdit, Nodes: []NodeFacts{node}})
		}
	} else {
		t.p.Steps[i].Facts = append(t.p.Steps[i].Facts, facts)
	}
	v := CheckContract(t.notebook, facts)
	switch {
	case v.Err != nil:
		return Refuse(v.Err, v.NodeID)
	case v.Needs != "" && t.p.Needs == nil:
		t.p.Needs = &Need{Op: i, Action: v.Needs, NodeID: v.NodeID}
	}
	return nil
}

// nodePlace is where n is, as FactsOf places a node.
func nodePlace(n Node) *NodePlace {
	return &NodePlace{ParentID: n.ParentID, Key: shared.TitleKey(n.Name)}
}

// unmoved is the facts of n's content step, or of a node a step carries:
// where it was, where it is.
func unmoved(n Node) NodeFacts {
	return NodeFacts{ID: n.ID, Kind: n.Kind, Before: nodePlace(n), After: nodePlace(n)}
}

// plan is the op i, s, on the tree: its error is the op's.
func (t *treePass) plan(i int, s BatchStep) error {
	switch {
	case s.Kind == OpKindWrite && s.Base == nil:
		return t.create(i, s)
	case s.Kind == OpKindWrite || s.Kind == OpKindEdit:
		return t.content(i, s)
	case s.Kind == OpKindMove:
		return t.move(i, s)
	}
	return t.delete(i, s)
}

// count counts id once as an item.
func (t *treePass) count(id uuid.UUID) {
	if !t.counted[id] {
		t.counted[id] = true
		t.p.Items++
	}
}

// create is write without base_version: the pages its path lacks, the
// last with its content, or the page there, whose content Contents
// compares.
func (t *treePass) create(i int, s BatchStep) error {
	ps := &t.p.Steps[i]
	found, err := WalkPath(s.Path.Keys, t.cur.lookup)
	if on := (*AttachmentOnPath)(nil); err != nil && !errors.As(err, &on) {
		return err
	}
	c, err := CreationOf(s.Args, s.Path, found)
	if err != nil {
		return err
	}
	ps.Found = idsOf(found)
	names := make([]string, 0, len(s.Path.Keys))
	for _, n := range found {
		names = append(names, n.Name)
	}
	if c.Leaf != nil {
		if err := t.judge(i, OpContent, []NodeFacts{unmoved(*c.Leaf)}); err != nil {
			return err
		}
		ps.Node, ps.Path = c.Leaf.ID, PathOfNames(names, KindPage)
		t.p.contentOp(i, c.Leaf.ID)
		return nil
	}
	facts, under := make([]NodeFacts, len(c.Titles)), c.Parent
	for d, title := range c.Titles {
		id := t.ids[i][len(found)+d]
		facts[d], under = NodeFacts{ID: id, Kind: KindPage, After: &NodePlace{ParentID: under, Key: shared.TitleKey(title)}}, &id
	}
	if err := t.judge(i, OpCreate, facts); err != nil {
		return err
	}
	parent := c.Parent
	for d, title := range c.Titles {
		n := Node{ID: t.ids[i][len(found)+d], ParentID: parent, Kind: KindPage, Name: title,
			NameKey: s.Path.Keys[len(found)+d], SortOrder: t.cur.lastOrder(parent)}
		t.cur.record(treeEdit{op: i, after: &n, updateLinks: true})
		t.p.createdBy[n.ID] = i
		t.count(n.ID)
		ps.Created = append(ps.Created, CreatedPage{ID: n.ID, ParentID: parent, Title: title})
		names = append(names, title)
		parent = &n.ID
	}
	ps.Node, ps.Path, ps.Changed = *parent, PathOfNames(names, KindPage), true
	t.p.contentOp(i, ps.Node)
	return nil
}

// content is write with base_version or edit: the page its path names,
// which a base_version refers to before the batch: one the batch created
// is 422, another page's 409, its current version of the page's node
// alone: the tree holds no revision, which the use case reads once the
// plan chose this error (Codex review R2), revision 0 until then, which no
// token holds.
func (t *treePass) content(i int, s BatchStep) error {
	ps := &t.p.Steps[i]
	n, ancestors, err := FindPath(s.Path, t.cur.lookup)
	if err != nil {
		return err
	}
	if s.Base != nil {
		if k, created := t.p.createdBy[n.ID]; created {
			return NotAllowed(s.Args.Field("base_version"),
				fmt.Sprintf("refers to the notebook before the batch: ops[%d] created this page", k))
		}
		if s.Base.Node != n.ID {
			return &VersionMismatch{Path: s.Path.Written, Current: VersionToken{Node: n.ID}, Why: OtherNode}
		}
	}
	if err := t.judge(i, OpContent, []NodeFacts{unmoved(n)}); err != nil {
		return err
	}
	ps.Node, ps.Found, ps.Path = n.ID, append(idsOf(ancestors), n.ID), PathOf(ancestors, n)
	t.p.contentOp(i, n.ID)
	return nil
}

// move is move: the node from names, to's parent's line, the new name,
// then Unit.Relocate's checks: under its parent the name alone, reading no
// subtree; under another, RelocationOf, then the nodes the batch's moves
// carry (MaxUnitNodes). One there and so named already is unchanged.
func (t *treePass) move(i int, s BatchStep) error {
	ps := &t.p.Steps[i]
	n, ancestors, err := FindPath(s.From, t.cur.lookup)
	if err != nil {
		return err
	}
	chain, err := WalkPath(s.To.Keys[:len(s.To.Keys)-1], t.cur.lookup)
	var on *AttachmentOnPath
	if err != nil && !errors.As(err, &on) {
		return err
	}
	if err := ParentChainOf(s.Args, s.To, chain, on); err != nil {
		return err
	}
	var parent *uuid.UUID
	if len(chain) > 0 {
		parent = new(chain[len(chain)-1].ID)
	}
	title, err := RenameTitle(s.Args.Field("to"), n, s.To.Titles[len(s.To.Titles)-1])
	if err != nil {
		return err
	}
	ps.Node, ps.Found, ps.Chain = n.ID, append(idsOf(ancestors), n.ID), idsOf(chain)
	ps.To, ps.Path = Destination{ParentID: parent, Name: title.Name}, PathOf(ancestors, n)
	moved := n
	switch {
	case SameParent(n.ParentID, parent) && title.Name == n.Name:
		return nil
	case SameParent(n.ParentID, parent):
		if h, held := HolderOf(t.cur.children(parent), title, n.ID); held && title.Key != n.NameKey {
			return &PathTaken{Path: s.To.Written, Asset: h.Kind == KindAsset}
		}
		// Under its parent, a rename: the unit's Relocate renames it.
		renamed := NodeFacts{ID: n.ID, Kind: n.Kind, Before: nodePlace(n), After: &NodePlace{ParentID: parent, Key: shared.TitleKey(title.Name)}}
		if err := t.judge(i, OpRename, []NodeFacts{renamed}); err != nil {
			return err
		}
	default:
		var holder *Node
		if h, held := HolderOf(t.cur.children(parent), title, n.ID); held {
			holder = &h
		}
		sub := t.cur.subtree(n.ID)
		if err := RelocationOf(sub, parent, t.cur.line(parent), holder); err != nil {
			if taken := (*PathTaken)(nil); errors.As(err, &taken) {
				taken.Path = s.To.Written // to is the whole new path, which the answer says
			}
			return err
		}
		// The node moved, then each node it carries, as the unit's step lists
		// them (its subtree, the node first).
		facts := []NodeFacts{{ID: n.ID, Kind: n.Kind, Before: nodePlace(n), After: &NodePlace{ParentID: parent, Key: shared.TitleKey(title.Name)}}}
		for _, m := range sub[1:] {
			facts = append(facts, unmoved(m.Node))
		}
		if err := t.judge(i, OpMove, facts); err != nil {
			return err
		}
		// After the move's own checks, as the items' bound waits for its op
		// (M9/P3 review A's N4): a cycle past it answers its 409.
		if t.moves, t.carried = t.moves+1, t.carried+len(sub); t.moves > 1 && t.carried > MaxUnitNodes {
			t.p.final = true
			return BatchTooLarge("nodes the batch's moves carry", t.carried, MaxUnitNodes,
				"move each folder once, or in separate calls")
		}
		moved.ParentID, moved.SortOrder = parent, t.cur.lastOrder(parent)
	}
	moved.Name, moved.NameKey = title.Name, title.Key
	t.cur.record(treeEdit{op: i, after: &moved, before: &n, updateLinks: s.UpdateLinks})
	if _, ok := t.p.movedBy[n.ID]; !ok {
		t.p.movedBy[n.ID] = i
	}
	t.count(n.ID)
	ps.Path, ps.Changed = PathOf(chain, moved), true
	return nil
}

// delete is delete: the node its path names and its subtree, recursive's
// rule on it, the earlier deletions under it merged into it (M9 design
// 4.11's normalization), then B1 and B3 on all it takes.
func (t *treePass) delete(i int, s BatchStep) error {
	ps := &t.p.Steps[i]
	n, ancestors, err := FindPath(s.Path, t.cur.lookup)
	if err != nil {
		return err
	}
	sub := t.cur.subtree(n.ID)
	if err := DeletionOf(s.Args, sub, s.Recursive); err != nil {
		return err
	}
	d := &deletion{op: i, parent: n.ParentID, group: make([]uuid.UUID, 0, len(sub)), into: -1}
	d.pages, d.assets = sub.Counts()
	in := make(map[uuid.UUID]bool, len(sub))
	for _, m := range sub {
		in[m.Node.ID] = true
		d.group = append(d.group, m.Node.ID)
	}
	var merged []*deletion
	for _, e := range t.deletions {
		if e.into < 0 && e.parent != nil && in[*e.parent] {
			merged = append(merged, e)
			d.group = append(d.group, e.group...)
			d.pages, d.assets = d.pages+e.pages, d.assets+e.assets
		}
	}
	if err := t.p.deletesWhatItChanged(s.Args, d.group); err != nil {
		return err
	}
	facts := make([]NodeFacts, len(sub))
	for k, m := range sub {
		facts[k] = NodeFacts{ID: m.Node.ID, Kind: m.Node.Kind, Before: nodePlace(m.Node)}
	}
	if err := t.judge(i, OpDelete, facts); err != nil {
		return err
	}
	for _, e := range merged {
		e.into = i
		for k := range t.p.Steps {
			if k == e.op || t.p.Steps[k].MergedInto == e.op {
				t.p.Steps[k].MergedInto, t.p.Steps[k].Pages, t.p.Steps[k].Assets = i, 0, 0
			}
		}
	}
	for _, m := range sub {
		t.cur.record(treeEdit{op: i, before: &m.Node, updateLinks: true})
		t.count(m.Node.ID)
	}
	t.deletions = append(t.deletions, d)
	ps.Node, ps.Found, ps.Path = n.ID, append(idsOf(ancestors), n.ID), PathOf(ancestors, n)
	ps.Subtree, ps.Pages, ps.Assets, ps.Changed = d.group, d.pages, d.assets, true
	return nil
}

// deletesWhatItChanged is B1 and B3 (M9 design 4.11), 422 at the op a: a
// node of group, what it deletes, that the batch created, or else moved
// or renamed: the change would hold a node neither before nor after it,
// or one whose revert goes two ways.
func (p *BatchPlan) deletesWhatItChanged(a Args, group []uuid.UUID) error {
	for _, id := range group {
		if k, ok := p.createdBy[id]; ok {
			return NotAllowed(a.Op(), fmt.Sprintf("deletes a page that ops[%d] created: a batch may not delete what it creates "+
				"(the change would hold a node that is neither before nor after it); leave both out, or delete it in a later call", k))
		}
	}
	for _, id := range group {
		if k, ok := p.movedBy[id]; ok {
			what := "a page"
			if n, ok := p.tree.base[id]; ok && n.Kind == KindAsset {
				what = "an attachment"
			}
			return NotAllowed(a.Op(), fmt.Sprintf("deletes %s that ops[%d] moved or renamed: a batch may not delete what it moves "+
				"or renames; leave both out, or delete it in a later call", what, k))
		}
	}
	return nil
}

// contentOp records the op i as a content op of the page id.
func (p *BatchPlan) contentOp(i int, id uuid.UUID) {
	pg := p.pages[id]
	if pg == nil {
		_, created := p.createdBy[id]
		pg = &plannedPage{id: id, created: created, first: i, last: -1}
		p.pages[id] = pg
		p.order = append(p.order, id)
	}
	p.Steps[i].Previous, pg.last = pg.last, i
}

// idsOf is the ids of nodes, in their order.
func idsOf(nodes []Node) []uuid.UUID {
	out := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// TreeAt is the plan's tree after the op k, the batch's tree before any
// for -1: the tree before the batch with the edits of the ops up to k.
func (p *BatchPlan) TreeAt(k int) PlanTree { return PlanTree{v: p.tree.view(k)} }

// MovesBetween is the renames and moves of the ops after k up to j,
// merged node by node, the first before and the last after (as
// Change.Then merges them), with no revision: of the nodes alive after k,
// or created since, and alive after j, that no op among them moves with
// update_links false; a node created among them has its place after its
// creation as its before (M9/P3 design 3.7.1). A node back where it was,
// so named, is left out: no link follows it.
func (p *BatchPlan) MovesBetween(k, j int) []Change {
	return p.mergedMoves(k, j, false)
}

// FrozenMovesBetween is the renames and moves of the ops after k up to j of
// the nodes FrozenBetween names, merged as MovesBetween merges them: not
// followed, but where those nodes were and are, which the paths before of
// the nodes under them that a rewrite follows take their names and former
// parents from (M9/P3 review B1-N1).
func (p *BatchPlan) FrozenMovesBetween(k, j int) []Change {
	return p.mergedMoves(k, j, true)
}

// mergedMoves is MovesBetween's, of the nodes an op among them moves with
// update_links false when frozen, of the others when not.
func (p *BatchPlan) mergedMoves(k, j int, frozen bool) []Change {
	type merged struct {
		before, after *Node
		off, gone     bool
	}
	m := map[uuid.UUID]*merged{}
	var order []uuid.UUID
	for _, e := range p.tree.edits {
		if e.op <= k {
			continue
		}
		if e.op > j {
			break
		}
		id := idOf(e)
		x := m[id]
		if x == nil {
			x = &merged{}
			m[id], order = x, append(order, id)
		}
		switch {
		case e.after == nil:
			x.gone = true
		case e.before == nil:
			x.before, x.after = e.after, e.after
		default:
			if x.before == nil {
				x.before = e.before
			}
			x.after, x.off = e.after, x.off || !e.updateLinks
		}
	}
	var out []Change
	for _, id := range order {
		x := m[id]
		if x.gone || x.off != frozen || x.before.Name == x.after.Name && SameParent(x.before.ParentID, x.after.ParentID) {
			continue
		}
		before, after := x.before.State(), x.after.State()
		out = append(out, Change{NodeID: id, Before: &before, After: &after})
	}
	return out
}

// FrozenBetween is the nodes an op after k up to j renames or moves with
// update_links false, in id order: those MovesBetween leaves out for it.
// The links that led to them are left as they are, whatever their
// ancestors' changes (M9 design 4.11; M9/P3 review B-M1).
func (p *BatchPlan) FrozenBetween(k, j int) []uuid.UUID {
	var out []uuid.UUID
	for _, e := range p.tree.edits {
		if e.op > j {
			break
		}
		if e.op > k && e.before != nil && e.after != nil && !e.updateLinks {
			out = append(out, e.after.ID)
		}
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return slices.Compact(out)
}

// CreatedAfterMoves is, for each node an op after k up to j renames or
// moves with update_links, the nodes the ops after the last such op of it,
// up to j, create under it, as the tree after j holds them, in id order; a
// node with none is not in it. The rewrites that follow it reach its
// subtree as that op left it, these nodes out of it: no link reached them
// then, and their creation only resolves links again (M9 design 4.11;
// M9/P3 review B1-M1). A node created before the op, and one created
// after but renamed or moved itself, is reached as the op would reach it
// (3.14.4 (1a); 3.14.3's 17). Those created elsewhere are not in the
// subtree the rewrites read, on that tree, and are not listed: the reach
// of the same changes, keyed by them, serves the content ops between
// creations elsewhere (M9/P3 fix check 2, A's F2-1).
func (p *BatchPlan) CreatedAfterMoves(k, j int) map[uuid.UUID][]uuid.UUID {
	last := map[uuid.UUID]int{}
	var created []treeEdit
	for _, e := range p.tree.edits {
		if e.op > j {
			break
		}
		switch {
		case e.op <= k:
		case e.before == nil && e.after != nil:
			created = append(created, e)
		case e.before != nil && e.after != nil && e.updateLinks:
			last[e.after.ID] = e.op
		}
	}
	out := map[uuid.UUID][]uuid.UUID{}
	if len(last) == 0 || len(created) == 0 {
		return out
	}
	tree := p.tree.view(j)
	for id, op := range last {
		for _, c := range created {
			if c.op > op && tree.under(c.after.ID, id) {
				out[id] = append(out[id], c.after.ID)
			}
		}
		if ids, ok := out[id]; ok {
			slices.SortFunc(ids, uuid.UUID.Compare)
		}
	}
	return out
}

// Reads is what the contents' pass reads of the pages before the batch
// that the ops before Failed's write: texts, those whose first content op
// is an edit, or a write ReadWhole names, read whole; heads, those whose
// first is a write, of which the head alone; each in their first content
// ops' order. A plan past its items' bound reads none.
func (p *BatchPlan) Reads() (texts, heads []uuid.UUID) {
	if p.final {
		return nil, nil // past the items' bound: nothing is read
	}
	for _, id := range p.order {
		pg := p.pages[id]
		switch {
		case pg.created:
		case p.steps[pg.first].Kind == OpKindEdit || pg.whole:
			texts = append(texts, id)
		default:
			heads = append(heads, id)
		}
	}
	return texts, heads
}

// ReadWhole has the plan read the page id whole, though its first content
// op is a write: one that the renames and moves of the ops before it
// reach by the index, as its caller tells (M9/P3 review A-I1). Reads then
// lists it with the texts, CheckReads counts it, and Contents compares the
// write with the page's content rewritten for them, as an edit's.
func (p *BatchPlan) ReadWhole(id uuid.UUID) {
	if pg, ok := p.pages[id]; ok && !pg.created {
		pg.whole = true
	}
}

// CheckReads is the bound on the contents the plan reads whole (M9/P3
// design 3.4.4): the sizes of Reads' texts, by their heads, read before
// any of them, summed in their first content ops' order; past
// MaxBatchReadBytes, page.batch_too_large at the op of the page that
// passes it, in Failed, and the contents' pass is not run. Among them a
// write read whole for the renames and moves before it (ReadWhole), the
// refusal says why, a read the agent did not ask for (M9/P3 review A's
// N3). It answers that failure, nil for none; a plan that failed for good
// already is left as it is.
func (p *BatchPlan) CheckReads(heads map[uuid.UUID]PageHead) *OpFailed {
	if p.final {
		return nil
	}
	texts, _ := p.Reads()
	sum, whole := 0, false
	for _, id := range texts {
		first := p.pages[id].first
		whole = whole || p.steps[first].Kind != OpKindEdit
		if sum += heads[id].Bytes; sum > MaxBatchReadBytes {
			hint := "split the batch"
			if whole {
				hint = "a write after a rename or move that reaches its page reads the page whole: send the renames or moves in another batch"
			}
			p.Failed = &OpFailed{Op: first, Tool: p.steps[first].Kind,
				Err: BatchTooLarge("bytes in the pages its ops read", sum, MaxBatchReadBytes, hint)}
			p.final = true
			return p.Failed
		}
	}
	return nil
}

// Refuse records err, a refusal its caller decided of the op at, as the
// plan's failure when it comes before the plan's own (M10/P1 design
// 3.22): Reads, CheckReads and Contents then stop before at, as they do
// at a failure of the tree's pass, so that a batch answers the op that
// separate calls would be refused at first. A failure of the plan's at
// the same op, a bound of the batch's, gives way to it: separate calls
// have none.
func (p *BatchPlan) Refuse(at int, err error) {
	if p.Failed != nil && p.Failed.Op < at {
		return
	}
	p.Failed, p.final = &OpFailed{Op: at, Tool: p.steps[at].Kind, Err: err}, false
	p.order = slices.DeleteFunc(p.order, func(id uuid.UUID) bool { return p.pages[id].first >= at })
}

// Admit counts content, the page id's, as one the plan parses, before the
// parse (M9/P3 design 3.4.4): each page and hash once. Past
// MaxBatchParseBytes, or, when the batch writes more pages than one, past
// the unit's pages or bytes, it is page.batch_too_large, the parse not
// done, at the op being planned.
func (p *BatchPlan) Admit(id uuid.UUID, content string) error {
	key := parseKey{page: id, hash: sha256.Sum256([]byte(content))}
	if _, ok := p.parses[key]; ok {
		return nil
	}
	bytes := p.parseBytes + len(content)
	switch {
	case bytes > MaxBatchParseBytes:
		return BatchTooLarge("bytes of contents the plan parses to rewrite links", bytes, MaxBatchParseBytes,
			"send the renames and moves in a call apart from the edits after them")
	case p.multi && len(p.parses)+1 > MaxUnitPages:
		return BatchTooLarge("contents of the pages the batch writes and the plan parses to rewrite their links",
			len(p.parses)+1, MaxUnitPages, "send the renames and moves in a call apart from the edits after them")
	case p.multi && bytes > MaxUnitBytes:
		return BatchTooLarge("bytes in the pages the batch writes and the contents its plan parses to rewrite their links",
			bytes, MaxUnitBytes, "send the renames and moves in a call apart from the edits after them")
	}
	p.parses[key], p.parseBytes = len(content), bytes
	return nil
}

// CheckUnit is the unit's bounds on what the batch parses (M9/P3 design
// 3.4.4, 3.8): its pages' final contents and the plan's parses, each page
// and hash once, past MaxUnitPages or MaxUnitBytes unless it writes a
// lone page, page.batch_too_large at no op.
func (p *BatchPlan) CheckUnit() error {
	copies := make(map[parseKey]int, len(p.parses)+len(p.order))
	for k, n := range p.parses {
		copies[k] = n
	}
	for _, id := range p.order {
		pg := p.pages[id]
		copies[parseKey{page: id, hash: sha256.Sum256([]byte(pg.content))}] = len(pg.content)
	}
	bytes := 0
	for _, n := range copies {
		bytes += n
	}
	hint := "split the batch"
	if len(p.parses) > 0 {
		hint = "send the renames and moves in a call apart from the edits after them"
	}
	switch {
	case !UnitTooLarge(len(p.order), len(copies), bytes, 0):
		return nil
	case len(copies) > MaxUnitPages:
		return BatchTooLarge("contents of the pages the batch writes and the plan parses to rewrite their links",
			len(copies), MaxUnitPages, hint)
	}
	return BatchTooLarge("bytes in the pages the batch writes and the contents its plan parses to rewrite their links",
		bytes, MaxUnitBytes, hint)
}

// Pages is the pages whose content the batch writes, by their first
// content ops, as Contents left them.
func (p *BatchPlan) Pages() []PlannedPage {
	out := make([]PlannedPage, 0, len(p.order))
	for _, id := range p.order {
		pg := p.pages[id]
		out = append(out, PlannedPage{ID: id, First: pg.first, Last: pg.last, Created: pg.created, Revision: pg.revision,
			Content: pg.content, Merged: pg.merged})
	}
	return out
}
