package domain

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Resolved is a link of a page with where it resolved before a rename or
// move, and where it resolves after.
type Resolved struct {
	Link   Link
	Before Resolution
	After  Resolution
}

// Tree is what a rewrite reads of a notebook's pages: those the links
// resolved to before the change, at their paths before it and after it, by
// id; and, after it, the pages with each of the title keys a writing of
// them may be read with (WrittenKeys), by key.
type Tree struct {
	Before map[uuid.UUID]Node
	After  map[uuid.UUID]Node
	Named  map[string][]Node
}

// Edit writes Text over a content's bytes from Start to End; Shown when it
// writes what a link shows, its display or its text, not where it leads;
// Frontmatter when it writes a property link.
type Edit struct {
	Start, End  int
	Text        string
	Shown       bool
	Frontmatter bool
}

// Rewriting is how Rewrite writes a page's content again: the edits, which
// do not overlap; the pages the links it writes again lead to, by where
// each link's target starts; and the links no writing leads back (none
// should: a page's path from the root always does).
type Rewriting struct {
	Edits []Edit
	Leads map[int]Node
	Left  []Link
}

// Rewrites tells whether a rename or move writes l again (M6/P4 design 2):
// l, no value of the aliases, resolved to a page before it, and resolves
// after it to another, to none, or ambiguously where it did not; or l
// resolved to recased, the page a rename changed the case of the title of
// only, and names it by its title not as now written, with ".md" or
// without, a title that ends with ".md" too.
func Rewrites(l Link, before, after Resolution, recased Recased) bool {
	switch {
	case before.ID == (uuid.UUID{}) || l.Aliases:
		return false
	case after.ID != before.ID || after.Ambiguous && !before.Ambiguous:
		return true
	}
	written := last(l.Target)
	return before.ID == recased.ID && byKey(l, shared.TitleKey(recased.Name)) &&
		written != recased.Name && stem(written) != recased.Name
}

// Rewrite is how the links of content, the page's at from (its path after
// the change), are written again to lead where they led before a rename or
// move (M6/P4 design 2, 3): those Rewrites tells, recased the page a rename
// changed the case of the title of only.
func Rewrite(content string, from []Step, links []Resolved, tree Tree, recased Recased) Rewriting {
	w := Rewriting{Leads: map[int]Node{}}
	for _, r := range links {
		if !Rewrites(r.Link, r.Before, r.After, recased) {
			continue
		}
		was, ok := tree.Before[r.Before.ID]
		now, found := tree.After[r.Before.ID]
		if !ok || !found {
			w.Left = append(w.Left, r.Link)
			continue
		}
		e, ok := relink(content, from, r.Link, was, now, tree)
		if !ok {
			w.Left = append(w.Left, r.Link)
			continue
		}
		for i := range e {
			e[i].Frontmatter = r.Link.Property != ""
		}
		w.Edits, w.Leads[r.Link.Start] = append(w.Edits, e...), now
	}
	return w
}

// ErrTooLarge is parse's answer, for Written, to a writing that would hold
// more than a page may: none, the next tried (M6/P4 fix check c1-3).
var ErrTooLarge = errors.New("linking: the page written again would hold more than a page may")

// Written is content written again with w's edits, as parse reads its
// facts back (M6/P4 design 3.1): with them all when its links are those of
// was, content's facts, in their places, its aliases was's (Kept); else
// with those of the targets alone, what the links show left as it was;
// else with those of the body's targets alone, the property links left as
// they were too (left): a writing of the frontmatter may change more than
// its links, a value of the aliases that a YAML alias repeats from the key
// a link is written in (aliases: *x; M6/P4 fix check c5-1). Else ok is
// false, no writing keeping content's: a title the Markdown around a link
// reads into it, such as a '$' or a '`' that pairs with another. A writing
// parse answers ErrTooLarge is not content's either.
func (w Rewriting) Written(content string, was Facts, from []Step, tree Tree,
	parse func(content string) (Facts, error),
) (written string, left []Link, ok bool, err error) {
	type try struct {
		edits []Edit
		body  bool // the body's alone
	}
	tries := []try{{edits: w.Edits}}
	targets := slices.DeleteFunc(slices.Clone(w.Edits), func(e Edit) bool { return e.Shown })
	if len(targets) > 0 && len(targets) < len(w.Edits) {
		tries = append(tries, try{edits: targets})
	}
	body := slices.DeleteFunc(slices.Clone(targets), func(e Edit) bool { return e.Frontmatter })
	if len(body) > 0 && len(body) < len(targets) {
		tries = append(tries, try{edits: body, body: true})
	}
	for _, t := range tries {
		leads, left := w.Leads, []Link(nil)
		if t.body {
			leads, left = w.inBody(was.Links)
		}
		written = Apply(content, t.edits)
		now, err := parse(written)
		switch {
		case errors.Is(err, ErrTooLarge):
			continue
		case err != nil:
			return "", nil, false, err
		}
		if kept(was, now, leads, from, tree) {
			return written, left, true, nil
		}
	}
	return "", nil, false, nil
}

// inBody is w.Leads but for the property links of was, which a writing of
// the body's edits alone leaves.
func (w Rewriting) inBody(was []Link) (map[int]Node, []Link) {
	leads := maps.Clone(w.Leads)
	var left []Link
	for _, l := range was {
		if _, ok := leads[l.Start]; ok && l.Property != "" {
			delete(leads, l.Start)
			left = append(left, l)
		}
	}
	return leads, left
}

// Kept tells whether now, the facts of a writing of content with some of
// w's edits, read back, keep was, content's: its links in their places, as
// many, each of the same kind, property and anchor; each that w writes
// again leading, from from, to its page alone; each other as it was
// written, with its display. And its aliases, which a rewrite leaves (M6/P4
// design 2): a link in a key's value that the aliases repeat with a YAML
// alias (aliases: *x) is not flagged as theirs (M6/P4 fix check c3 F1).
func (w Rewriting) Kept(was, now Facts, from []Step, tree Tree) bool {
	return kept(was, now, w.Leads, from, tree)
}

// kept is Kept of a writing that writes again the links leads has, each to
// its page.
func kept(was, now Facts, leads map[int]Node, from []Step, tree Tree) bool {
	if len(now.Links) != len(was.Links) || !slices.Equal(now.Aliases, was.Aliases) {
		return false
	}
	for i, l := range was.Links {
		n := now.Links[i]
		if n.Kind != l.Kind || n.Property != l.Property || n.Anchor != l.Anchor {
			return false
		}
		page, written := leads[l.Start]
		switch {
		case written && !tree.leads(n.Target, from, page):
			return false
		case !written && (n.Target != l.Target || n.Display != l.Display):
			return false
		}
	}
	return true
}

// Apply is content with edits made, which do not overlap.
func Apply(content string, edits []Edit) string {
	sorted := slices.SortedFunc(slices.Values(edits), func(a, b Edit) int {
		return cmp.Or(a.Start-b.Start, a.End-b.End)
	})
	var out strings.Builder
	at := 0
	for _, e := range sorted {
		out.WriteString(content[at:e.Start])
		out.WriteString(e.Text)
		at = e.End
	}
	out.WriteString(content[at:])
	return out.String()
}

// Written is how a wikilink is written to lead to n, a page at its path, from
// anywhere, as Obsidian's fileToLinktext writes it (M6/P4 design 3.1): its
// name when named, the pages with its title key, holds no other page; its
// path from the root otherwise, which is its name at the root, where the
// root's page comes first.
func Written(n Node, named []Node) string {
	if slices.ContainsFunc(named, func(o Node) bool { return o.ID != n.ID }) {
		return n.path()
	}
	return n.name()
}

// WrittenKeys are the title keys of the pages a writing of n may be read
// with, which a rewrite reads (Tree.Named): its name's, with ".md" after it,
// and without one it ends with.
func WrittenKeys(n Node) []string {
	name := n.name()
	keys := []string{n.key(), shared.TitleKey(name + ".md")}
	if s := stem(name); s != name {
		keys = append(keys, shared.TitleKey(s))
	}
	return keys
}

// relink is the edits that write l, a link of content in the page at from,
// again to lead to now, the page it led to at was: its target; a
// wikilink's display text that was its target's last name; a Markdown
// link's text that was the page's title; the alias it was written with, as
// a wikilink's display text, an empty one too. An embed's display is its
// size or its caption, left; so is a display past an escape of a
// double-quoted string, which the content's bytes do not tell. ok is false
// when no writing leads there.
func relink(content string, from []Step, l Link, was, now Node, tree Tree) ([]Edit, bool) {
	var targets []string
	markdownLink := l.Kind == "link" || l.Kind == "image"
	switch {
	case !markdownLink:
		targets = []string{Written(now, tree.Named[now.key()]), now.path(), now.path() + ".md"}
	case strings.HasPrefix(l.Target, "./") || strings.HasPrefix(l.Target, "../"):
		targets = []string{relative(from, now) + ".md"}
	case strings.HasPrefix(l.Target, "/"):
		targets = []string{"/" + now.path() + ".md"}
	default:
		targets = []string{Written(now, tree.Named[now.key()]) + ".md", now.path() + ".md"}
	}
	at := slices.IndexFunc(targets, func(t string) bool { return tree.leads(t, from, now) })
	if at < 0 {
		return nil, false
	}
	target := targets[at]
	if markdownLink {
		target = destination(target, l.Start > 0 && content[l.Start-1] == '<')
	}
	var edits []Edit
	if written := quoted(l, target); written != content[l.Start:l.End] {
		edits = append(edits, Edit{Start: l.Start, End: l.End, Text: written})
	}
	if markdownLink {
		if s, e, ok := linkText(content, l.Start); ok {
			text, shown := content[s:e], quoted(l, now.name())
			// A link's text is Markdown, where a comment's "%%" pairs with
			// the page's markers past the link: one written or taken out
			// would show or hide text there and keep every link, which
			// Written would not tell (M6/P4 fix check c1-5).
			follows := text == quoted(l, was.name()) || strings.Contains(text, "/") && text == quoted(l, was.path())
			if follows && !strings.Contains(text, "%%") && !strings.Contains(shown, "%%") {
				edits = append(edits, Edit{Start: s, End: e, Text: shown, Shown: true})
			}
		}
		return edits, true
	}
	pipe, end := wikilinkTail(content, l.End)
	if l.Kind != "wikilink" || end < 0 || l.Quote == '"' && strings.IndexByte(content[l.End:end], '\\') >= 0 {
		return edits, true
	}
	alias := !byTitle(l, was)
	if pipe < 0 {
		if alias {
			separator := "|"
			if l.InTable {
				separator = `\|`
			}
			edits = append(edits, Edit{Start: end, End: end, Text: separator + content[l.Start:l.End], Shown: true})
		}
		return edits, true
	}
	s, e := trimmed(content, pipe+1, end)
	switch {
	case alias && s == e:
		edits = append(edits, Edit{Start: s, End: e, Text: content[l.Start:l.End], Shown: true})
	case l.Anchor == "" && strings.Contains(l.Target, "/") && content[s:e] == quoted(l, stem(last(l.Target))):
		edits = append(edits, Edit{Start: s, End: e, Text: quoted(l, now.name()), Shown: true})
	}
	return edits, true
}

// leads tells whether target, written in the page at from, resolves to n
// and no other page alike.
func (t Tree) leads(target string, from []Step, n Node) bool {
	parsed, ok := ParseTarget(target)
	if !ok {
		return false
	}
	var candidates []Node
	for _, key := range parsed.LastKeys() {
		candidates = append(candidates, t.Named[key]...)
	}
	r := Resolve(parsed, from, candidates, nil)
	return r.ID == n.ID && !r.Ambiguous
}

// byTitle tells whether l names was, the page it led to, by its title,
// not by one of its aliases.
func byTitle(l Link, was Node) bool {
	return byKey(l, was.key())
}

// byKey tells whether l names a page by the title key key.
func byKey(l Link, key string) bool {
	t, ok := ParseTarget(l.Target)
	return ok && slices.Contains(t.LastKeys(), key)
}

// relative is n's path from the folder of the page at from: "./", then
// down, when n is in the folder or below it; up to where their paths part,
// then down, otherwise.
func relative(from []Step, n Node) string {
	folder := from[:max(len(from)-1, 0)]
	parent := n.Path[:len(n.Path)-1]
	common := 0
	for common < len(folder) && common < len(parent) && folder[common].ID == parent[common].ID {
		common++
	}
	down := names(n.Path[common:])
	if common == len(folder) {
		return "./" + down
	}
	return strings.Repeat("../", len(folder)-common) + down
}

// destination is a Markdown link's target written where it was, its '%'
// escaped, which the reading decodes: between angle brackets, as it is
// otherwise; with its spaces, '(' and ')' escaped too otherwise, which
// would end or change it (the fixture set's rename/002).
func destination(target string, angle bool) string {
	if angle {
		return strings.ReplaceAll(target, "%", "%25")
	}
	return strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29").Replace(target)
}

// quoted is s written in l's place: a ' in a single-quoted YAML string is
// two.
func quoted(l Link, s string) string {
	if l.Quote == '\'' {
		return strings.ReplaceAll(s, "'", "''")
	}
	return s
}

// linkText is where a Markdown link's text is written, without the space
// around it, when it is on one line with no bracket before the destination
// at start: "](" then space, and a '<', may come between. A text that
// starts past an escaped '[' is none: its bytes are not its text.
func linkText(content string, start int) (int, int, bool) {
	at := start
	if at > 0 && content[at-1] == '<' {
		at--
	}
	for at > 0 && strings.IndexByte(" \t\r\n", content[at-1]) >= 0 {
		at--
	}
	if at < 2 || content[at-1] != '(' || content[at-2] != ']' {
		return 0, 0, false
	}
	closing := at - 2
	opening := strings.LastIndexAny(content[:closing], "[]\r\n")
	if opening < 0 || content[opening] != '[' || escaped(content, opening) {
		return 0, 0, false
	}
	s, e := trimmed(content, opening+1, closing)
	return s, e, true
}

// escaped tells whether the byte at i follows an odd run of backslashes.
func escaped(content string, i int) bool {
	n := 0
	for i-n > 0 && content[i-n-1] == '\\' {
		n++
	}
	return n%2 == 1
}

// wikilinkTail is where the '|' that starts a wikilink's display text is,
// past end, where its target is written to, if it has one; and where its
// "]]" is, or -1.
func wikilinkTail(content string, end int) (int, int) {
	closing := strings.Index(content[end:], "]]")
	if closing < 0 {
		return -1, -1
	}
	closing += end
	pipe := strings.IndexByte(content[end:closing], '|')
	if pipe >= 0 {
		pipe += end
	}
	return pipe, closing
}

// trimmed is content's span from s to e without the spaces and tabs around
// it, as a wikilink's parse trims its parts.
func trimmed(content string, s, e int) (int, int) {
	for s < e && (content[s] == ' ' || content[s] == '\t') {
		s++
	}
	for e > s && (content[e-1] == ' ' || content[e-1] == '\t') {
		e--
	}
	return s, e
}

// last is the last segment of a target.
func last(target string) string {
	return target[strings.LastIndexByte(target, '/')+1:]
}

// stem is s without a ".md" it ends with, in any case.
func stem(s string) string {
	if n := len(s) - len(".md"); n > 0 && strings.EqualFold(s[n:], ".md") {
		return s[:n]
	}
	return s
}

// name is n's title.
func (n Node) name() string {
	return n.Path[len(n.Path)-1].Name
}

// path is n's path from the root, its names joined by '/'.
func (n Node) path() string {
	return names(n.Path)
}

// names are steps' names joined by '/'.
func names(steps []Step) string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Name
	}
	return strings.Join(out, "/")
}
