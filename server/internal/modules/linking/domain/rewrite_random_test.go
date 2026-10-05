package domain_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// randomTitle is one of the titles the random trees take: alike but for
// case, with ".md" and with what a Markdown link escapes, so that names
// repeat.
func randomTitle(r *rand.Rand) string {
	titles := []string{"a", "A", "b", "x", "X", "y", "Plan", "a b", "é", "x.md", "Close) 50%"}
	return titles[r.IntN(len(titles))]
}

// After a random rename or move in a random tree, every link of a random
// page that resolved to a page resolves to it after the rewrite, and no
// more ambiguously; one that did not is as it was, byte for byte; the
// links are the same in number and kind, and none is left (M6/P4 design 8).
func TestARewriteKeepsWhereEveryLinkLeads(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	rewritten, moved := 0, map[string]int{} // cases with edits, links whose page moved by kind
	for seed := range uint64(4000) {
		r := rand.New(rand.NewPCG(seed, 7))
		c := randomCase(r)
		before, after := pagesOf(t, c.pages, c.aliases), pagesOf(t, c.moved, c.aliases)
		facts, err := markdownadapter.PageFacts(m.Parse([]byte(c.content)).Facts())
		if err != nil {
			t.Fatal(err)
		}
		links := make([]domain.Resolved, len(facts.Links))
		for i, l := range facts.Links {
			links[i] = domain.Resolved{Link: l, Before: before.resolve(l, c.page), After: after.resolve(l, c.pageAfter)}
		}
		tree := domain.Tree{Before: before.byID, After: after.byID, Named: after.named}
		edits, left := domain.Rewrite(c.content, after.paths[c.pageAfter], links, tree, c.caseOnly)
		written := domain.Apply(c.content, edits)
		if len(edits) > 0 {
			rewritten++
		}
		for _, l := range links {
			if l.Before.ID != (uuid.UUID{}) && (l.After.ID != l.Before.ID || l.After.Ambiguous && !l.Before.Ambiguous) {
				moved[l.Link.Kind+map[bool]string{true: " property"}[l.Link.Property != ""]]++
			}
		}
		again, err := markdownadapter.PageFacts(m.Parse([]byte(written)).Facts())
		if err != nil {
			t.Fatal(err)
		}
		fail := func(format string, args ...any) {
			t.Fatalf("seed %d: %s\ncase %+v\nwritten %q", seed, fmt.Sprintf(format, args...), c, written)
		}
		if len(left) > 0 {
			fail("left %+v", left)
		}
		if len(again.Links) != len(links) {
			fail("%d links after, %d before", len(again.Links), len(links))
		}
		for i, l := range again.Links {
			was := links[i]
			if l.Kind != was.Link.Kind || l.Property != was.Link.Property {
				fail("link %d is %s %q after, %s %q before", i, l.Kind, l.Property, was.Link.Kind, was.Link.Property)
			}
			if was.Before.ID == (uuid.UUID{}) {
				if got, want := written[l.Start:l.End], c.content[was.Link.Start:was.Link.End]; got != want {
					fail("link %d resolved to nothing, and is written %q, not %q", i, got, want)
				}
				continue
			}
			if was.Link.Aliases {
				continue
			}
			got := after.resolve(l, c.pageAfter)
			if got.ID != was.Before.ID || got.Ambiguous && !was.Before.Ambiguous {
				fail("link %d %q leads to %v after (ambiguous %t), %v before", i, written[l.Start:l.End], got.ID, got.Ambiguous, was.Before.ID)
			}
		}
	}
	// Cases that rewrote nothing would prove nothing.
	if rewritten < 1000 || slices.ContainsFunc([]string{"wikilink", "embed", "link", "wikilink property"}, func(k string) bool { return moved[k] < 100 }) {
		t.Errorf("edits in %d cases, the links that moved %v: too few", rewritten, moved)
	}
}

// randomCase is a tree, a page of it whose content has links of every kind
// to its pages and to none, and a rename or a move of one of its pages.
type randomCaseOf struct {
	pages, moved    []string
	aliases         [][]string
	page, pageAfter string
	content         string
	caseOnly        uuid.UUID
}

func randomCase(r *rand.Rand) randomCaseOf {
	var c randomCaseOf
	for len(c.pages) < 3+r.IntN(9) {
		parent := ""
		if len(c.pages) > 0 && r.IntN(3) > 0 {
			parent = c.pages[r.IntN(len(c.pages))] + "/"
		}
		path := parent + randomTitle(r)
		if !slices.ContainsFunc(c.pages, func(p string) bool { return siblings(p, path) }) {
			c.pages = append(c.pages, path)
		}
	}
	c.aliases = make([][]string, len(c.pages))
	for i := range c.pages {
		if r.IntN(5) == 0 {
			c.aliases[i] = []string{randomTitle(r)}
		}
	}
	c.page = c.pages[r.IntN(len(c.pages))]
	from, to := randomChange(r, c.pages)
	moved := func(p string) string {
		if p == from || strings.HasPrefix(p, from+"/") {
			return to + p[len(from):]
		}
		return p
	}
	for _, p := range c.pages {
		c.moved = append(c.moved, moved(p))
	}
	c.pageAfter = moved(c.page)
	if parentOf(from) == parentOf(to) && shared.TitleKey(lastOf(from)) == shared.TitleKey(lastOf(to)) {
		var id uuid.UUID
		id[15] = byte(slices.Index(c.pages, from) + 1)
		c.caseOnly = id
	}
	var body, properties []string
	for range 1 + r.IntN(8) {
		body = append(body, randomLink(r, c.pages, c.page))
	}
	if r.IntN(3) == 0 {
		body = append(body, "\n\n| h |\n| --- |\n| "+strings.ReplaceAll(randomWikilink(r, c.pages, c.page), "|", `\|`)+" |\n")
	}
	for i := range r.IntN(3) {
		value := randomWikilink(r, c.pages, c.page)
		key := []string{"related", "aliases", "up"}[i]
		if r.IntN(2) == 0 && !strings.Contains(value, "'") {
			properties = append(properties, fmt.Sprintf("%s: '%s'", key, value))
		} else {
			properties = append(properties, fmt.Sprintf("%s: %q", key, value))
		}
	}
	if len(properties) > 0 {
		c.content = "---\n" + strings.Join(properties, "\n") + "\n---\n"
	}
	c.content += strings.Join(body, " ") + "\n"
	return c
}

// randomChange is a rename of one of pages, to a title no sibling has, or a
// move of one to the root or a page out of its subtree with no child of its
// title: as from and to.
func randomChange(r *rand.Rand, pages []string) (string, string) {
	for {
		from := pages[r.IntN(len(pages))]
		var to string
		if r.IntN(2) == 0 {
			to = parentOf(from)
			if to != "" {
				to += "/"
			}
			to += randomTitle(r)
		} else {
			parent := pages[r.IntN(len(pages))] + "/"
			if r.IntN(3) == 0 {
				parent = ""
			}
			to = parent + lastOf(from)
		}
		inside := to == from || strings.HasPrefix(to, from+"/")
		if !inside && !slices.ContainsFunc(pages, func(p string) bool { return p != from && siblings(p, to) }) {
			return from, to
		}
	}
}

// siblings tells whether two paths are of one parent and one title key.
func siblings(a, b string) bool {
	return parentOf(a) == parentOf(b) && shared.TitleKey(lastOf(a)) == shared.TitleKey(lastOf(b))
}

// randomLink is a wikilink, an embed or a Markdown link, to one of pages
// or to none, written from the page at from in one of the ways a page is.
func randomLink(r *rand.Rand, pages []string, from string) string {
	if r.IntN(3) > 0 {
		link := randomWikilink(r, pages, from)
		if r.IntN(4) == 0 {
			link = "!" + link
		}
		return link
	}
	target := randomTarget(r, pages, from)
	name := lastOf(strings.TrimSuffix(target, ".md"))
	if !strings.HasSuffix(target, ".md") {
		target += ".md"
	}
	text := []string{"t", name, strings.TrimPrefix(target, "/")}[r.IntN(3)]
	if r.IntN(3) == 0 {
		return fmt.Sprintf("[%s](<%s>)", text, target)
	}
	return fmt.Sprintf("[%s](%s)", text, strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29").Replace(target))
}

// randomWikilink is a wikilink to one of pages or to none, with an anchor
// or a display text, the last name of its target among them, at times.
func randomWikilink(r *rand.Rand, pages []string, from string) string {
	target := randomTarget(r, pages, from)
	switch r.IntN(4) {
	case 0:
		target += "#h"
	case 1:
		target += "|" + lastOf(strings.TrimSuffix(target, ".md"))
	case 2:
		target += "|t"
	}
	return "[[" + target + "]]"
}

// randomTarget is where a link to one of pages, or to a title no page may
// have, is written from the page at from: its name, a path's end, its path
// from the root with or without '/', its path from from's folder, with
// ".md" at times.
func randomTarget(r *rand.Rand, pages []string, from string) string {
	if r.IntN(8) == 0 {
		return "Nowhere"
	}
	page := pages[r.IntN(len(pages))]
	var target string
	switch r.IntN(5) {
	case 0:
		target = lastOf(page)
	case 1:
		segments := strings.Split(page, "/")
		target = strings.Join(segments[r.IntN(len(segments)):], "/")
	case 2:
		target = page
	case 3:
		target = "/" + page
	default:
		folder := strings.Split(parentOf(from), "/")
		if parentOf(from) == "" {
			folder = nil
		}
		ups := r.IntN(len(folder) + 1)
		base := strings.Join(folder[:len(folder)-ups], "/")
		switch {
		case ups > 0:
			target = strings.Repeat("../", ups) + strings.TrimPrefix(strings.TrimPrefix(page, base), "/")
		case base == "" || strings.HasPrefix(page, base+"/"):
			target = "./" + strings.TrimPrefix(strings.TrimPrefix(page, base), "/")
		default:
			target = "./" + lastOf(page)
		}
	}
	if r.IntN(4) == 0 {
		target += ".md"
	}
	return target
}
