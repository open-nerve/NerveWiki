package domain

import (
	"cmp"
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

// Edit writes Text over a content's bytes from Start to End.
type Edit struct {
	Start, End int
	Text       string
}

// Rewrite is how the links of content, the page's at from (its path after
// the change), are written again to lead where they led before a rename or
// move (M6/P4 design 2, 3). A link is written again when it resolved to a
// page and resolves to another after, to none, or ambiguously where it did
// not; and, when caseOnly is the page a rename changed the case of the
// title of only, when it names it by its title not as now written. A link
// that is a value of the aliases is left. It returns the edits, which do
// not overlap, and the links of these no writing leads back (none should:
// a page's path from the root always does).
func Rewrite(content string, from []Step, links []Resolved, tree Tree, caseOnly uuid.UUID) ([]Edit, []Link) {
	var edits []Edit
	var left []Link
	for _, r := range links {
		if r.Before.ID == (uuid.UUID{}) || r.Link.Aliases {
			continue
		}
		was, ok := tree.Before[r.Before.ID]
		now, found := tree.After[r.Before.ID]
		if !ok || !found {
			left = append(left, r.Link)
			continue
		}
		moved := r.After.ID != r.Before.ID || r.After.Ambiguous && !r.Before.Ambiguous
		recased := r.Before.ID == caseOnly && byTitle(r.Link, was) && stem(last(r.Link.Target)) != now.name()
		if !moved && !recased {
			continue
		}
		e, ok := relink(content, from, r.Link, was, now, tree)
		if !ok {
			left = append(left, r.Link)
			continue
		}
		edits = append(edits, e...)
	}
	return edits, left
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
// again to lead to now, the page it led to at was: its target; a display
// text that was its target's last name; a Markdown link's text that was the
// page's title; the alias it was written with, as a display text. ok is
// false when no writing leads there.
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
		edits = append(edits, Edit{l.Start, l.End, written})
	}
	if markdownLink {
		if s, e, ok := linkText(content, l.Start); ok {
			text := content[s:e]
			if text == quoted(l, was.name()) || strings.Contains(text, "/") && text == quoted(l, was.path()) {
				edits = append(edits, Edit{s, e, quoted(l, now.name())})
			}
		}
		return edits, true
	}
	pipe, end := wikilinkTail(content, l.End)
	switch {
	case end < 0:
	case pipe < 0 && l.Kind == "wikilink" && !byTitle(l, was):
		separator := "|"
		if l.InTable {
			separator = `\|`
		}
		edits = append(edits, Edit{end, end, separator + content[l.Start:l.End]})
	case pipe >= 0 && l.Anchor == "" && strings.Contains(l.Target, "/"):
		s, e := trimmed(content, pipe+1, end)
		if content[s:e] == quoted(l, stem(last(l.Target))) {
			edits = append(edits, Edit{s, e, quoted(l, now.name())})
		}
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
	t, ok := ParseTarget(l.Target)
	return ok && slices.Contains(t.LastKeys(), was.key())
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

// destination is a Markdown link's target written where it was: as it is
// between angle brackets; with its spaces, '%', '(' and ')' escaped
// otherwise, which would end or change it (the fixture set's rename/002).
func destination(target string, angle bool) string {
	if angle {
		return target
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
// at start: "](" then space, and a '<', may come between.
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
	if opening < 0 || content[opening] != '[' {
		return 0, 0, false
	}
	s, e := trimmed(content, opening+1, closing)
	return s, e, true
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
