package bootstrap

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The batches of TestTheIndexIsItsRebuild (M9/P3 design 3.14.6): random
// batches through MCP, of the writes the writer makes, and directed ones,
// a page's link to a title and then that title's rename; each made on a
// model of the notebook's tree the test reads before it and keeps through
// its ops, which tells where the links the batch writes lead when it
// writes them. After a batch done, M9 design 4.11's last rule is checked:
// a link that led to a node leads to it still, unless the batch deleted
// that node, or the link leads to a node it created or by an alias it
// wrote.

// batcher makes a notebook's batches as alice's agent.
type batcher struct {
	w    writer // the vocabulary, of the batches' own random source
	m    *mcpApp
	logs *syncBuffer
}

// modelNode is a node alive in a batch's model: its parent ("" for the
// root), name and kind; a page's content, and its revision before the
// batch, 0 for a page the batch creates.
type modelNode struct {
	parent, name string
	asset        bool
	content      string
	revision     int
}

// linkCheck is a link of a page a batch writes that 4.11's rule follows:
// the page, the op of its content, the link's place among the wikilinks of
// its body and their number, and the node its title named then.
type linkCheck struct {
	page   string
	op     int
	at, of int
	to     string
}

// batchRun is a batch being made: its ops, and the model as they leave it.
type batchRun struct {
	b       *batcher
	nodes   map[string]*modelNode
	ops     []map[string]any
	created map[string]int  // the nodes it creates, by their placeholder "new" and the op, the op
	changed map[string]bool // the nodes it creates, renames or moves: none of them it may delete (B1, B3)
	deleted map[string]bool
	written map[string]int // the pages of its content ops, their last op
	checks  []linkCheck
	moves   []int // the ops that rename or move a node
	backs   []backCheck
}

// backCheck is a page a batch puts back (backed): the op that writes its
// bytes from before the batch again, after the rename of a node its links
// led to by its title; the places of those links among the wikilinks of
// its body, and their number; and its content after the batch, "" when a
// rename after the write may rewrite it.
type backCheck struct {
	page, to string
	op       int
	at       []int
	of       int
	content  string
}

// load is a batch's model, the notebook's nodes alive.
func (b *batcher) load(t *testing.T) *batchRun {
	t.Helper()
	r := &batchRun{b: b, nodes: map[string]*modelNode{}, created: map[string]int{}, changed: map[string]bool{},
		deleted: map[string]bool{}, written: map[string]int{}}
	rows, err := b.w.tm.pool.Query(context.Background(), `SELECT n.id::text, coalesce(n.parent_id::text, ''), n.name, n.kind = 'asset',
			coalesce(c.content, ''), coalesce(c.revision, 0)
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL`, b.w.nb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		n := &modelNode{}
		if err := rows.Scan(&id, &n.parent, &n.name, &n.asset, &n.content, &n.revision); err != nil {
			t.Fatal(err)
		}
		r.nodes[id] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *batchRun) intn(n int) int { return r.b.w.rnd.IntN(n) }

// ids are the model's nodes, pages alone when pages, in a fixed order.
func (r *batchRun) ids(pages bool) []string {
	var out []string
	for id, n := range r.nodes {
		if !pages || !n.asset {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// path is the node id's path from the root, a page's with .md.
func (r *batchRun) path(id string) string {
	var names []string
	for n := id; n != ""; n = r.nodes[n].parent {
		names = append(names, r.nodes[n].name)
	}
	slices.Reverse(names)
	if r.nodes[id].asset {
		return strings.Join(names, "/")
	}
	return strings.Join(names, "/") + ".md"
}

// taken tells whether a node under parent other than except has name's
// key.
func (r *batchRun) taken(parent, name, except string) bool {
	key := shared.TitleKey(name)
	for id, n := range r.nodes {
		if id != except && n.parent == parent && shared.TitleKey(n.name) == key {
			return true
		}
	}
	return false
}

// depth is the pages from the root to id, 0 for the root.
func (r *batchRun) depth(id string) int {
	d := 0
	for n := id; n != ""; n = r.nodes[n].parent {
		d++
	}
	return d
}

// subtree is id and the nodes under it.
func (r *batchRun) subtree(id string) []string {
	out := []string{id}
	for _, c := range r.ids(false) {
		if r.nodes[c].parent == id {
			out = append(out, r.subtree(c)...)
		}
	}
	return out
}

// height is the levels of id's subtree, itself one.
func (r *batchRun) height(id string) int {
	h := 1
	for _, c := range r.ids(false) {
		if r.nodes[c].parent == id {
			h = max(h, 1+r.height(c))
		}
	}
	return h
}

// titled is the one page alive whose title has target's key, when target
// is a name alone, no attachment has its key and exactly one page does:
// the page a link [[target]] resolves to, where it is (M6/P3 design 2).
// "" otherwise.
func (r *batchRun) titled(target string) string {
	if strings.ContainsAny(target, "/.") {
		return ""
	}
	key, found := shared.TitleKey(target), ""
	for _, id := range r.ids(false) {
		if shared.TitleKey(r.nodes[id].name) != key {
			continue
		}
		if r.nodes[id].asset || found != "" {
			return ""
		}
		found = id
	}
	return found
}

// content is a random content of the page id, of the writer's forms, for
// the next op: the links of its body to a title one page alone has are
// checked after the batch, those of an earlier content of the page not.
func (r *batchRun) content(id string) string {
	w := r.b.w
	var b strings.Builder
	if w.rnd.IntN(2) == 0 {
		b.WriteString("---\n")
		if w.rnd.IntN(2) == 0 {
			b.WriteString("aliases: [" + strings.Join(w.aliases(), ", ") + "]\n")
		}
		b.WriteString("tags: [t]\n---\n")
	}
	var links []string
	for range w.rnd.IntN(5) {
		target := w.target()
		switch w.rnd.IntN(3) {
		case 0:
			b.WriteString("[[" + target + "]] ")
			links = append(links, target)
		case 1:
			b.WriteString("![[" + target + "]] ")
		default:
			b.WriteString("[x](" + target + ") ")
		}
	}
	r.checks = slices.DeleteFunc(r.checks, func(c linkCheck) bool { return c.page == id })
	for i, target := range links {
		if to := r.titled(target); to != "" {
			r.checks = append(r.checks, linkCheck{page: id, op: len(r.ops), at: i, of: len(links), to: to})
		}
	}
	r.written[id] = len(r.ops)
	return b.String()
}

// The ops a random batch draws from, each on the model as the ops before
// it left it, and false when the one drawn would be refused: a page
// created under a page or the root; a page's content written on its
// version before the batch, or edited whole, before any rename or move,
// which may rewrite it; a page renamed; a page or an attachment moved, its
// links following; a page or an attachment deleted, with what is under it.

func (r *batchRun) create() bool {
	parents := append([]string{""}, r.ids(true)...)
	parent, title := parents[r.intn(len(parents))], r.b.w.title()
	if r.taken(parent, title, "") || r.depth(parent) >= 10 {
		return false
	}
	id := fmt.Sprint("new", len(r.ops))
	r.nodes[id] = &modelNode{parent: parent, name: title}
	r.nodes[id].content = r.content(id)
	r.created[id], r.changed[id] = len(r.ops), true
	r.ops = append(r.ops, opWrite(r.path(id), r.nodes[id].content))
	return true
}

func (r *batchRun) overwrite() bool {
	var before []string
	for _, id := range r.ids(true) {
		if r.nodes[id].revision > 0 {
			before = append(before, id)
		}
	}
	if len(before) == 0 {
		return false
	}
	id := before[r.intn(len(before))]
	n := r.nodes[id]
	n.content = r.content(id)
	r.ops = append(r.ops, opWriteOn(r.path(id), n.content, versionOf(id, n.revision)))
	return true
}

func (r *batchRun) edit() bool {
	pages := r.ids(true)
	if len(r.moves) > 0 || len(pages) == 0 {
		return false
	}
	id := pages[r.intn(len(pages))]
	n := r.nodes[id]
	if n.content == "" {
		return false
	}
	old := n.content
	if n.content = r.content(id); n.content == old {
		return false
	}
	r.ops = append(r.ops, opEdit(r.path(id), old, n.content))
	return true
}

func (r *batchRun) rename() bool {
	pages := r.ids(true)
	if len(pages) == 0 {
		return false
	}
	id, name := pages[r.intn(len(pages))], r.b.w.title()
	n := r.nodes[id]
	if name == n.name || r.taken(n.parent, name, id) {
		return false
	}
	from := r.path(id)
	n.name = name
	r.relocated(id, from)
	return true
}

func (r *batchRun) move() bool {
	all, parents := r.ids(false), append([]string{""}, r.ids(true)...)
	if len(all) == 0 {
		return false
	}
	id, parent := all[r.intn(len(all))], parents[r.intn(len(parents))]
	n := r.nodes[id]
	if parent == n.parent || slices.Contains(r.subtree(id), parent) || r.taken(parent, n.name, id) || r.depth(parent)+r.height(id) > 10 {
		return false
	}
	from := r.path(id)
	n.parent = parent
	r.relocated(id, from)
	return true
}

// relocated adds the move of id from the path from to where the model has
// it now, its links following.
func (r *batchRun) relocated(id, from string) {
	r.changed[id] = true
	r.moves = append(r.moves, len(r.ops))
	r.ops = append(r.ops, opMove(from, r.path(id)))
}

func (r *batchRun) remove() bool {
	all := r.ids(false)
	if len(all) == 0 {
		return false
	}
	id := all[r.intn(len(all))]
	sub := r.subtree(id)
	if slices.ContainsFunc(sub, func(s string) bool { return r.changed[s] }) {
		return false
	}
	r.ops = append(r.ops, opDeleteAll(r.path(id)))
	for _, s := range sub {
		delete(r.nodes, s)
		r.deleted[s] = true
	}
	r.checks = slices.DeleteFunc(r.checks, func(c linkCheck) bool { return r.deleted[c.page] })
	return true
}

// random is a batch of 2 to 6 ops drawn at random, nil when the model
// gave fewer.
func (b *batcher) random(t *testing.T) *batchRun {
	t.Helper()
	r := b.load(t)
	ops := []func() bool{r.create, r.overwrite, r.edit, r.rename, r.move, r.remove}
	for want, tries := 2+b.w.rnd.IntN(5), 0; len(r.ops) < want && tries < 60; tries++ {
		ops[b.w.rnd.IntN(len(ops))]()
	}
	if len(r.ops) < 2 {
		return nil
	}
	return r
}

// directed is a batch of a page's link to a title one page alone has,
// added at the end of its content, then that page's rename (M9/P3 review
// C-M6): a written page's link with a rename after it, which the random
// batches seldom make. nil when no page has such a title.
func (b *batcher) directed(t *testing.T, step int) *batchRun {
	t.Helper()
	r := b.load(t)
	pages := r.ids(true)
	var titled []string
	for _, id := range pages {
		if r.titled(r.nodes[id].name) == id {
			titled = append(titled, id)
		}
	}
	if len(titled) == 0 {
		return nil
	}
	to, p := titled[r.intn(len(titled))], pages[r.intn(len(pages))]
	n := r.nodes[p]
	at := count(t, b.w.tm.pool, `SELECT count(*) FROM page_links WHERE source_id = $1 AND kind = 'wikilink' AND property_key IS NULL
		AND NOT aliases`, p)
	link := "[[" + r.nodes[to].name + "]]"
	if n.content == "" {
		r.ops = append(r.ops, opWriteOn(r.path(p), link, versionOf(p, n.revision)))
	} else {
		r.ops = append(r.ops, opEdit(r.path(p), n.content, n.content+" "+link))
	}
	r.written[p] = 0
	r.checks = append(r.checks, linkCheck{page: p, at: at, of: at + 1, to: to})
	name := b.w.title()
	if name == r.nodes[to].name || r.taken(r.nodes[to].parent, name, to) {
		name = fmt.Sprint("Renamed ", step)
	}
	from := r.path(to)
	r.nodes[to].name = name
	r.relocated(to, from)
	return r
}

// batchSeen is what the batches of a run did, over its seeds: those done,
// those whose deferred rewrite wrote a page, the links of pages they wrote
// that 4.11's rule checked, and of them those with a rename or a move after
// the write.
type batchSeen struct{ done, rewrote, checked, followed, backs int }

// run makes the batch r, and when it is done checks 4.11's rule on the
// notebook before and after it, counting into seen. Any refusal fails t:
// the model makes only batches it foresees done, and a refusal of a
// version, which a batch alone in its notebook never meets, would be the
// plan and the unit disagreeing (v0.1 design 13.4, item 13; M9/P3 review
// C-N1).
func (b *batcher) run(t *testing.T, r *batchRun, seen *batchSeen) string {
	t.Helper()
	tm, nb := b.w.tm, b.w.nb
	led, aliases := tm.linkRows(t, nb), tm.aliasesOf(t, nb)
	a := b.m.batch(t, r.ops...)
	what := fmt.Sprintf("a batch of %v", r.ops)
	if a.result(t)["isError"] == true {
		t.Fatalf("%s = %.500s; want it done, as its model foresees", what, a.body)
	}
	done := answerOf[mcpBatched](t, a)
	seen.done++
	if line := (&mcpApp{logs: b.logs}).loggedLast(t, "batch applied"); line == nil {
		t.Fatalf("%s logged no line", what)
	} else if line["rewritten"].(float64) > 0 {
		seen.rewrote++
	}
	actual := func(id string) string {
		if k, ok := r.created[id]; ok {
			return done.Results[k].NodeID
		}
		return id
	}
	created, written := map[string]bool{}, map[string]bool{}
	for id := range r.created {
		created[actual(id)] = true
	}
	for id := range r.written {
		written[actual(id)] = true
	}
	// aliased are the pages whose aliases the batch wrote.
	now, after, aliased := tm.linkRows(t, nb), tm.aliasesOf(t, nb), map[string]bool{}
	for page := range written {
		if !slices.Equal(after[page], aliases[page]) {
			aliased[page] = true
		}
	}
	// kept tells whether a link that led to was leads to got, or may not:
	// was deleted by the batch, got created by it or aliased by it.
	kept := func(was, got string) bool {
		return got == was || r.deleted[was] || created[got] || aliased[got]
	}
	for page, links := range led {
		if r.deleted[page] || written[page] {
			continue
		}
		if len(now[page]) != len(links) {
			t.Fatalf("after %s, the page %s has %d links, had %d", what, page, len(now[page]), len(links))
		}
		for i, l := range links {
			// A link that led to a page by an alias the batch took away
			// resolves again, as one whose target it created or deleted
			// (M9 design 4.11): a write's aliases are no rename. One by an
			// alias the page kept is checked (M9/P3 review B-N3).
			takenAway := func(key string) bool {
				return slices.Contains(aliases[l.to], key) && !slices.Contains(after[l.to], key)
			}
			byAlias := aliased[l.to] && (takenAway(l.key) || takenAway(l.alt))
			if l.to != "" && !l.alias && !byAlias && !kept(l.to, now[page][i].to) {
				t.Fatalf("after %s, the link %d of %s, not written, leads to %q, led to %q", what, i, page, now[page][i].to, l.to)
			}
		}
	}
	for _, c := range r.checks {
		page := actual(c.page)
		body := queryStrings(t, tm.pool, `SELECT coalesce(resolved_id::text, '') FROM page_links WHERE source_id = $1 AND kind = 'wikilink'
			AND property_key IS NULL AND NOT aliases ORDER BY range_start`, page)
		if len(body) != c.of {
			t.Fatalf("after %s, %s has %d wikilinks in its body, its content %d", what, page, len(body), c.of)
		}
		if to := actual(c.to); !kept(to, body[c.at]) {
			t.Fatalf("after %s, the wikilink %d of %s, written by ops[%d], leads to %q, led to %q then", what, c.at, page, c.op, body[c.at], to)
		}
		seen.checked++
		if slices.ContainsFunc(r.moves, func(op int) bool { return op > c.op }) {
			seen.followed++
		}
	}
	// A page put back: its links follow from its last content op, the
	// bytes it held before the batch, not the rename's rewrite before it
	// (M9 Codex review R1): none of those that led to the renamed node by
	// its title leads to it, and with no rename after, its content is the
	// bytes it held.
	for _, c := range r.backs {
		body := queryStrings(t, tm.pool, `SELECT coalesce(resolved_id::text, '') FROM page_links WHERE source_id = $1 AND kind = 'wikilink'
			AND property_key IS NULL AND NOT aliases ORDER BY range_start`, c.page)
		if len(body) != c.of {
			t.Fatalf("after %s, %s, put back, has %d wikilinks in its body, had %d", what, c.page, len(body), c.of)
		}
		for _, at := range c.at {
			if body[at] == c.to {
				t.Fatalf("after %s, the wikilink %d of %s, put back by ops[%d], leads to %s renamed before: the rename's rewrite, "+
					"not its last content op", what, at, c.page, c.op, c.to)
			}
		}
		if c.content != "" {
			if now := queryStrings(t, tm.pool, `SELECT content FROM page_contents WHERE node_id = $1 AND deleted_at IS NULL`, c.page); len(now) != 1 ||
				now[0] != c.content {
				t.Fatalf("after %s, %s, put back by ops[%d], holds %q, want the bytes it held, %q", what, c.page, c.op, now, c.content)
			}
		}
		if done.Results[c.op].Changed {
			seen.backs++
		}
	}
	return what
}

// backed is a batch that puts a page back (M9 Codex review fix round 1,
// A-N3; fix check A's shape): the rename of a node one of the page's
// wikilinks leads to by its title, which rewrites it, then the page's
// bytes before the batch written on its version before it (putBack); at
// times the rename of another node it links to after. The random batches
// seldom make one. nil when no page links to another by its title.
func (b *batcher) backed(t *testing.T, step int) *batchRun {
	t.Helper()
	r := b.load(t)
	rows := queryStrings(t, b.w.tm.pool, `SELECT l.source_id::text || ' ' || l.resolved_id::text FROM page_links l
			JOIN nodes n ON n.id = l.resolved_id AND n.kind = 'page' AND l.target_key = n.name_key
		WHERE l.notebook_id = $1 AND l.kind = 'wikilink' AND NOT l.aliases AND l.property_key IS NULL AND l.resolved_id <> l.source_id
		ORDER BY 1`, b.w.nb)
	if len(rows) == 0 {
		return nil
	}
	p, to, _ := strings.Cut(rows[r.intn(len(rows))], " ")
	if r.nodes[p] == nil || r.nodes[to] == nil || r.nodes[p].revision == 0 {
		return nil
	}
	rename := func(id string, k int) {
		name := b.w.title()
		if shared.TitleKey(name) == shared.TitleKey(r.nodes[id].name) || r.taken(r.nodes[id].parent, name, id) {
			name = fmt.Sprint("Back ", step, " ", k)
		}
		from := r.path(id)
		r.nodes[id].name = name
		r.relocated(id, from)
	}
	if !b.putBack(t, r, p, to, func() { rename(to, 0) }) {
		return nil
	}
	if r.intn(2) == 0 {
		for _, row := range rows {
			if s, other, _ := strings.Cut(row, " "); s == p && other != to && r.nodes[other] != nil {
				rename(other, 1)
				r.backs[len(r.backs)-1].content = ""
				break
			}
		}
	}
	return r
}

// putBack adds to r the rename of to, which rename makes, then the page
// p's bytes before the batch written on its version before it, with their
// check (backCheck) of the wikilinks of p's body that the rename leaves
// leading elsewhere: those that led to to whose target's key is its
// title's, and whose other key, of a name written with .md, is not the new
// name's (M9 Codex review fix round 2, A2-M1: a link by an alias or a
// path, or by the new name's key, leads to to still, rightly). false, r
// unchanged, when to has its title's key as an alias.
func (b *batcher) putBack(t *testing.T, r *batchRun, p, to string, rename func()) bool {
	t.Helper()
	old := shared.TitleKey(r.nodes[to].name)
	if slices.Contains(b.w.tm.aliasesOf(t, b.w.nb)[to], old) {
		return false
	}
	rows, err := b.w.tm.pool.Query(context.Background(), `SELECT coalesce(resolved_id::text, ''), coalesce(target_key, ''),
			coalesce(target_alt_key, '')
		FROM page_links WHERE source_id = $1 AND kind = 'wikilink' AND property_key IS NULL AND NOT aliases ORDER BY range_start`, p)
	if err != nil {
		t.Fatal(err)
	}
	type bodyLink struct{ to, key, alt string }
	var body []bodyLink
	for rows.Next() {
		var l bodyLink
		if err := rows.Scan(&l.to, &l.key, &l.alt); err != nil {
			t.Fatal(err)
		}
		body = append(body, l)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	back := backCheck{page: p, to: to, of: len(body), content: r.nodes[p].content}
	rename()
	renamed := shared.TitleKey(r.nodes[to].name)
	for i, l := range body {
		if l.to == to && l.key == old && (l.alt == "" || l.alt != renamed) {
			back.at = append(back.at, i)
		}
	}
	r.checks = slices.DeleteFunc(r.checks, func(c linkCheck) bool { return c.page == p })
	r.written[p], back.op = len(r.ops), len(r.ops)
	r.ops = append(r.ops, opWriteOn(r.path(p), back.content, versionOf(p, r.nodes[p].revision)))
	r.backs = append(r.backs, back)
	return true
}

// linkRow is a link of the index: where it resolves ("" for nowhere), its
// target's keys, and whether it is a value of the aliases.
type linkRow struct {
	to, key, alt string
	alias        bool
}

// linkRows are the notebook nb's links by page, in their order.
func (tm acmeTeam) linkRows(t *testing.T, nb string) map[string][]linkRow {
	t.Helper()
	rows, err := tm.pool.Query(context.Background(), `SELECT source_id::text, coalesce(resolved_id::text, ''), coalesce(target_key, ''),
			coalesce(target_alt_key, ''), aliases
		FROM page_links WHERE notebook_id = $1 ORDER BY source_id, range_start`, nb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]linkRow{}
	for rows.Next() {
		var page string
		var l linkRow
		if err := rows.Scan(&page, &l.to, &l.key, &l.alt, &l.alias); err != nil {
			t.Fatal(err)
		}
		out[page] = append(out[page], l)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// aliasesOf are the notebook nb's aliases, their keys by page.
func (tm acmeTeam) aliasesOf(t *testing.T, nb string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, row := range queryStrings(t, tm.pool, `SELECT source_id::text || ' ' || alias_key FROM page_aliases WHERE notebook_id = $1
		ORDER BY 1`, nb) {
		page, key, _ := strings.Cut(row, " ")
		out[page] = append(out[page], key)
	}
	return out
}

// loggedLast is the last log line of msg, or nil.
func (m *mcpApp) loggedLast(t testing.TB, msg string) map[string]any {
	t.Helper()
	lines := m.logLines(t)
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i]["msg"] == msg {
			return lines[i]
		}
	}
	return nil
}
