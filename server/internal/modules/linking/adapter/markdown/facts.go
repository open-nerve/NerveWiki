// Package markdownadapter reads the platform's Markdown for the link
// index (M6/P3 design 3.1): what it keeps of a page, from the facts of its
// content's parse.
package markdownadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// PageFacts is what the index keeps of facts, the platform's facts of a
// page's content: the links the obsidian extension took, the frontmatter's
// tags before the body's, its aliases, and its properties. Facts of
// another kind are an error.
//
// The frontmatter's tags and aliases are read as Obsidian 1.12.7 reads
// them (parseFrontMatterTags, parseFrontMatterAliases): the first key that
// is "tags" or "aliases" but for ASCII case; a string is one, trimmed, and
// a list's strings are; an empty one is none. A tag is one by rule 9,
// after one '#' it may start with, as Obsidian's tag pane counts them.
func PageFacts(facts any) (domain.Facts, error) {
	f, ok := facts.(markdown.Facts)
	if !ok {
		return domain.Facts{}, fmt.Errorf("linking: a page's facts are %T, not the Markdown's", facts)
	}
	fm := f.Frontmatter()
	out := domain.Facts{FrontmatterValid: !fm.Present || fm.Valid}
	x, _ := f.Extracted(obsidian.Name).(obsidian.Extracted)
	for _, l := range x.Links {
		out.Links = append(out.Links, domain.Link{
			Kind: string(l.Kind), Property: l.Key, Target: l.Target, Anchor: l.Anchor, Display: l.Display,
			Start: l.Range.Start, End: l.Range.Stop,
		})
	}
	var tags, aliases []string
	for _, s := range stringsOf(fm.Properties, "tags") {
		if s = strings.TrimPrefix(s, "#"); obsidian.IsTag(s) {
			tags = append(tags, s)
		}
	}
	for _, t := range x.Tags {
		tags = append(tags, t.Name)
	}
	for _, s := range stringsOf(fm.Properties, "aliases") {
		if s != "" {
			aliases = append(aliases, s)
		}
	}
	out.Tags, out.Aliases = domain.TagsOf(tags), domain.AliasesOf(aliases)
	for _, p := range fm.Properties {
		value, err := json.Marshal(jsonOf(p.Value))
		if err != nil {
			return domain.Facts{}, fmt.Errorf("linking: the property %q: %w", p.Key, err)
		}
		out.Properties = append(out.Properties, domain.Property{Key: p.Key, Value: value})
	}
	return out, nil
}

// stringsOf is the strings of the first of props whose key is key but for
// ASCII case, trimmed as JavaScript trims: its value when a string, its
// value's strings when a list.
func stringsOf(props []markdown.Property, key string) []string {
	for _, p := range props {
		if !asciiFold(p.Key, key) {
			continue
		}
		var out []string
		switch v := p.Value.(type) {
		case string:
			out = append(out, jsTrim(v))
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					out = append(out, jsTrim(s))
				}
			}
		}
		return out
	}
	return nil
}

// asciiFold tells whether a and b are one but for ASCII case, as a
// JavaScript expression with the i flag and not the u flag compares them.
func asciiFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lower(a[i]) != lower(b[i]) {
			return false
		}
	}
	return true
}

func lower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// jsTrim is s without the white space and line terminators around it, as
// JavaScript's String.prototype.trim takes them: Go's unicode.IsSpace
// takes U+0085 and leaves U+FEFF.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == '\uFEFF' || r != '\u0085' && unicode.IsSpace(r)
	})
}

// jsonOf is a property's value as the fixture set's JSON writes it: a
// mapping is an object.
func jsonOf(v any) any {
	switch v := v.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = jsonOf(item)
		}
		return out
	case []markdown.Property:
		out := make(map[string]any, len(v))
		for _, p := range v {
			out[p.Key] = jsonOf(p.Value)
		}
		return out
	}
	return v
}
