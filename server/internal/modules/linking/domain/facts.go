package domain

import (
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Extractor is the version of what a page's facts are, which the index
// records of each page (indexed_pages.extractor): a release that changes it
// asks for nervewiki reindex.
const Extractor = 1

// Facts is what the index keeps of a page's content (M6/P3 design 3.2): its
// links, tags, properties and aliases, and whether its frontmatter, if it
// has one, is valid; an invalid one has no properties, tags or aliases.
type Facts struct {
	FrontmatterValid bool
	Links            []Link
	Tags             []Tag
	Properties       []Property
	Aliases          []Alias
}

// Link is a link of a page's content as the fixture set writes it: its
// kind (wikilink, embed, link or image), the path of the property it is the
// value of, its target, anchor and display text, an empty one none; and
// where its target is written, in bytes, one link a range.
//
// What a rewrite reads of it beside (M6/P4 design 2, 3): Aliases tells a
// link that is a value of the page's aliases, which a rewrite leaves; and,
// which the index does not keep, Quote is how the frontmatter writes a
// property link's value, 0 plain, or the single or double quote around it,
// and InTable tells a wikilink in a table's cell.
type Link struct {
	Kind     string
	Property string
	Target   string
	Anchor   string
	Display  string
	Start    int
	End      int
	Quote    byte
	InTable  bool
	Aliases  bool
}

// InFrontmatter tells a property link: the value of a frontmatter's string
// in quotes, as a plain one cannot start with '[', its path maybe empty, as
// a key may be ("": '[[x]]'), which the index keeps as none (M6/P4 fix
// check c6-1).
func (l Link) InFrontmatter() bool {
	return l.Property != "" || l.Quote != 0
}

// MaxKey is the most bytes of a title key the index keeps: twice a title's
// most, which no title's key comes near (a title has at most 255 bytes, and
// its key at most about twice as many). A link whose target's last key is
// longer resolves to nothing whatever the tree, and an alias or a tag with
// a longer key is not kept: the index's keys stay within PostgreSQL's
// bound of a B-tree's entry.
const MaxKey = 1024

// Keys are the title keys a page must have to be l's target, each within
// MaxKey: none when its target resolves to nothing whatever the tree.
func (l Link) Keys() []string {
	t, ok := ParseTarget(l.Target)
	if !ok {
		return nil
	}
	return slices.DeleteFunc(t.LastKeys(), func(k string) bool { return len(k) > MaxKey })
}

// Tag is a tag of a page, one a title key: as first written, and how often
// the page writes it.
type Tag struct {
	Key   string
	Name  string
	Count int
}

// Property is a key of the frontmatter, its value as the fixture set's
// JSON writes it.
type Property struct {
	Key   string
	Value []byte
}

// Alias is an alias of a page, one a title key: as written first.
type Alias struct {
	Key  string
	Name string
}

// TagsOf is the tags of names, a page's in the order written, without
// their '#': one a title key, as first written, with how often; none whose
// key is longer than MaxKey.
func TagsOf(names []string) []Tag {
	var out []Tag
	at := map[string]int{}
	for _, name := range names {
		key := shared.TitleKey(name)
		if len(key) > MaxKey {
			continue
		}
		if i, ok := at[key]; ok {
			out[i].Count++
			continue
		}
		at[key] = len(out)
		out = append(out, Tag{Key: key, Name: name, Count: 1})
	}
	return out
}

// AliasesOf is the aliases of names, a page's in the order written: one a
// title key, as first written; none whose key is longer than MaxKey.
func AliasesOf(names []string) []Alias {
	var out []Alias
	seen := map[string]bool{}
	for _, name := range names {
		key := shared.TitleKey(name)
		if len(key) <= MaxKey && !seen[key] {
			seen[key] = true
			out = append(out, Alias{Key: key, Name: name})
		}
	}
	return out
}
