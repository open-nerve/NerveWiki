package domain

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
type Link struct {
	Kind     string
	Property string
	Target   string
	Anchor   string
	Display  string
	Start    int
	End      int
}

// Keys are the title keys a page must have to be l's target: none when its
// target resolves to nothing whatever the tree.
func (l Link) Keys() []string {
	t, ok := ParseTarget(l.Target)
	if !ok {
		return nil
	}
	return t.LastKeys()
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
