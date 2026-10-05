// Package markdownadapter reads the platform's Markdown for the link
// index (M6/P3 design 3.1): what it keeps of a page, from the facts of its
// content's parse, and, for nervewiki reindex, the parse itself.
package markdownadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PageFacts is what the index keeps of facts, the platform's facts of a
// page's content: the links the obsidian extension took, the tags, the
// frontmatter's before the body's, its aliases, and its properties, each
// U+0000 written as U+FFFD, which PostgreSQL's text does not hold. Facts
// of another kind, or without the obsidian extension's, are an error.
//
// A property link has its value's quote, and tells whether it is one of
// the aliases. The frontmatter's tags and aliases are read as Obsidian
// 1.12.7 reads them (parseFrontMatterTags, parseFrontMatterAliases): the first key that
// is "tags" or "aliases" but for ASCII case; a string is one, trimmed, and
// a list's strings are; an empty one is none. A tag, after one '#' it may
// start with, is one as Obsidian's tag pane counts it (obsidian.CountedTag),
// a body's too.
func PageFacts(facts any) (domain.Facts, error) {
	f, ok := facts.(markdown.Facts)
	if !ok {
		return domain.Facts{}, fmt.Errorf("linking: a page's facts are %T, not the Markdown's", facts)
	}
	x, ok := f.Extracted(obsidian.Name).(obsidian.Extracted)
	if !ok {
		return domain.Facts{}, fmt.Errorf("linking: a page's facts have no %s extraction", obsidian.Name)
	}
	fm := f.Frontmatter()
	out := domain.Facts{FrontmatterValid: !fm.Present || fm.Valid}
	aliasesProperty, _ := propertyOf(fm.Properties, "aliases")
	for _, l := range x.Links {
		link := domain.Link{
			Kind: string(l.Kind), Property: text(l.Key), Target: text(l.Target), Anchor: text(l.Anchor), Display: text(l.Display),
			Start: l.Range.Start, End: l.Range.Stop, InTable: l.InTable,
		}
		if s, ok := scalarOf(fm.Scalars, l); ok {
			link.Quote, link.Aliases = s.Quote, valueOf(s, aliasesProperty)
		}
		out.Links = append(out.Links, link)
	}
	var tags, aliases []string
	for _, s := range stringsOf(fm.Properties, "tags") {
		if tag, ok := obsidian.CountedTag(strings.TrimPrefix(s, "#")); ok {
			tags = append(tags, tag)
		}
	}
	for _, t := range x.Tags {
		if tag, ok := obsidian.CountedTag(t.Name); ok {
			tags = append(tags, tag)
		}
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
		out.Properties = append(out.Properties, domain.Property{Key: text(p.Key), Value: value})
	}
	return out, nil
}

// text is s with U+0000, which a Markdown link's %00 or a YAML string's
// escape writes, as U+FFFD.
func text(s string) string {
	return strings.ReplaceAll(s, "\x00", "\uFFFD")
}

// scalarOf is the frontmatter's string that l, a property link, is: of its
// path, its range within the string's. A body's link is in none.
func scalarOf(scalars []markdown.Scalar, l obsidian.Link) (markdown.Scalar, bool) {
	for _, s := range scalars {
		if s.Path == l.Key && s.Offset(0) <= l.Range.Start && l.Range.Stop <= s.Offset(len(s.Value)) {
			return s, true
		}
	}
	return markdown.Scalar{}, false
}

// valueOf tells whether s, a frontmatter's string, is one of p's strings as
// stringsOf reads them: p's value when a string, one of its list's when a
// list, by s's depth too, as a key may hold a '.' ("aliases.0" is a key's
// path as well as a list's first string's).
func valueOf(s markdown.Scalar, p markdown.Property) bool {
	switch p.Value.(type) {
	case string:
		return s.Path == p.Key
	case []any:
		index, ok := strings.CutPrefix(s.Path, p.Key+".")
		return s.Depth == 2 && ok && index != "" && strings.Trim(index, "0123456789") == ""
	}
	return false
}

// propertyOf is the first of props whose key is key but for ASCII case, if
// one is: the one Obsidian reads.
func propertyOf(props []markdown.Property, key string) (markdown.Property, bool) {
	for _, p := range props {
		if asciiFold(p.Key, key) {
			return p, true
		}
	}
	return markdown.Property{}, false
}

// stringsOf is the strings of the first of props whose key is key but for
// ASCII case, trimmed as JavaScript trims, as text: its value when a
// string, its value's strings when a list.
func stringsOf(props []markdown.Property, key string) []string {
	p, _ := propertyOf(props, key)
	var out []string
	switch v := p.Value.(type) {
	case string:
		out = append(out, text(jsTrim(v)))
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, text(jsTrim(s)))
			}
		}
	}
	return out
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

// jsonOf is a property's value as the fixture set's JSON writes it, its
// strings as text: a mapping is an object.
func jsonOf(v any) any {
	switch v := v.(type) {
	case string:
		return text(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = jsonOf(item)
		}
		return out
	case []markdown.Property:
		out := make(map[string]any, len(v))
		for _, p := range v {
			out[text(p.Key)] = jsonOf(p.Value)
		}
		return out
	}
	return v
}

// Parser takes a content's facts as the index keeps them, holding its
// share of the parse budget while it parses (nervewiki reindex).
type Parser struct {
	md     *markdown.Markdown
	budget *markdown.Budget
}

// NewParser returns the parser of md within budget.
func NewParser(md *markdown.Markdown, budget *markdown.Budget) Parser {
	return Parser{md: md, budget: budget}
}

// Facts is content's facts as the index keeps them.
func (p Parser) Facts(ctx context.Context, content string) (domain.Facts, error) {
	hold, err := p.budget.Take(ctx, len(content))
	if err != nil {
		return domain.Facts{}, err
	}
	defer hold.Release()
	return PageFacts(p.md.Parse([]byte(content)).Facts())
}

// retryAfter is the Retry-After of a rewrite's 503 when the budget is not
// free.
const retryAfter = time.Second

// ParseNow implements app.RewriteParser: content's facts, as the index
// keeps them and as the page module writes them, holding their share of
// the budget, taken now, until released; server_busy, to retry after a
// second, when it is not free (M6/P4 design 4.2).
func (p Parser) ParseNow(ctx context.Context, content string) (app.Parsed, error) {
	hold, err := p.budget.TakeNow(ctx, len(content))
	switch {
	case errors.Is(err, markdown.ErrBusy):
		return app.Parsed{}, shared.ServerBusy(retryAfter)
	case err != nil:
		return app.Parsed{}, err
	}
	f := p.md.Parse([]byte(content)).Facts()
	hold.KeepFacts(f)
	facts, err := PageFacts(f)
	if err != nil {
		hold.Release()
		return app.Parsed{}, err
	}
	return app.Parsed{Facts: facts, Written: f, Release: hold.Release}, nil
}
