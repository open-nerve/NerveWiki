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
// case, with ".md", with what a Markdown link escapes or decodes, and with
// what the Markdown around a link may pair, so that names repeat.
func randomTitle(r *rand.Rand) string {
	titles := []string{"a", "A", "b", "x", "X", "y", "Plan", "a b", "é", "x.md", "Close) 50%", "x%41", "a$b", "Don`t", "a %% b"}
	return titles[r.IntN(len(titles))]
}

// After a random rename or move in a random tree, every link of a random
// page that resolved to a page resolves to it after the rewrite, read back
// (Written), and no more ambiguously; one that did not is as it was, byte
// for byte; the links are the same in number and kind, and none is left;
// every edit is within a link's brackets and the rest of its line (M6/P4
// design 8); the comments' markers where they pair are as many. A writing
// that does not read back as the links leaves the page, only when it
// writes a '$' or a '`' that the Markdown around may pair.
func TestARewriteKeepsWhereEveryLinkLeads(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parse := func(content string) (domain.Facts, error) {
		return markdownadapter.PageFacts(m.Parse([]byte(content)).Facts())
	}
	// Cases with edits, those that write a '$' or a '`', cases left, links
	// whose page moved by kind.
	rewritten, pairing, unwritten, moved := 0, 0, 0, map[string]int{}
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
		tree := rewriteTree(before, after, links, c.recased)
		w := domain.Rewrite(c.content, after.paths[c.pageAfter], links, tree, c.recased)
		written, left, ok, err := w.Written(c.content, facts, after.paths[c.pageAfter], tree, parse)
		if err != nil {
			t.Fatal(err)
		}
		if len(w.Edits) > 0 {
			rewritten++
		}
		for _, l := range links {
			if l.Before.ID != (uuid.UUID{}) && (l.After.ID != l.Before.ID || l.After.Ambiguous && !l.Before.Ambiguous) {
				moved[l.Link.Kind+map[bool]string{true: " property"}[l.Link.Property != ""]]++
			}
		}
		fail := func(format string, args ...any) {
			t.Fatalf("seed %d: %s\ncase %+v\nedits %+v\nwritten %q", seed, fmt.Sprintf(format, args...), c, w.Edits, written)
		}
		if len(w.Left) > 0 || len(left) > 0 {
			fail("left %+v, %+v", w.Left, left)
		}
		for _, e := range w.Edits {
			if !slices.ContainsFunc(facts.Links, func(l domain.Link) bool { return withinLink(c.content, l, e) }) {
				fail("the edit %+v is in no link's brackets", e)
			}
		}
		pairs := slices.ContainsFunc(w.Edits, func(e domain.Edit) bool { return strings.ContainsAny(e.Text, "$`") })
		if pairs {
			pairing++
		}
		if !ok {
			if !pairs {
				fail("no writing reads back as the links")
			}
			unwritten++
			continue
		}
		again, err := markdownadapter.PageFacts(m.Parse([]byte(written)).Facts())
		if err != nil {
			t.Fatal(err)
		}
		if len(again.Links) != len(links) {
			fail("%d links after, %d before", len(again.Links), len(links))
		}
		if got, want := comments(written, again.Links), comments(c.content, facts.Links); got != want {
			fail("%d comments' markers after, %d before", got, want)
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
	// Cases that rewrote nothing would prove nothing; few are left.
	if rewritten < 1000 || slices.ContainsFunc([]string{"wikilink", "embed", "link", "wikilink property"}, func(k string) bool { return moved[k] < 100 }) {
		t.Errorf("edits in %d cases, the links that moved %v: too few", rewritten, moved)
	}
	if pairing < 100 || unwritten > rewritten/10 {
		t.Errorf("%d of %d cases with edits write a '$' or a '`', %d are left: want more, and few left", pairing, rewritten, unwritten)
	}
}

// comments is how many comments' "%%" content holds where they pair:
// outside its links' targets and its wikilinks' brackets. A writing that
// changes it shows or hides text, keeping every link (M6/P4 fix check
// c1-5).
func comments(content string, links []domain.Link) int {
	var outside strings.Builder
	at := 0
	for _, l := range links {
		end := l.End
		if i := strings.Index(content[l.End:], "]]"); (l.Kind == "wikilink" || l.Kind == "embed") && i >= 0 {
			end += i
		}
		if l.Start >= at {
			outside.WriteString(content[at:l.Start] + "\x00")
			at = end
		}
	}
	outside.WriteString(content[at:])
	return strings.Count(outside.String(), "%%")
}

// withinLink tells whether e, an edit of content, is within l's brackets
// and the rest of its line: from the '[' before its target.
func withinLink(content string, l domain.Link, e domain.Edit) bool {
	start := strings.LastIndexByte(content[:l.Start], '[')
	end := strings.IndexByte(content[l.End:], '\n')
	return start >= 0 && end >= 0 && start <= e.Start && e.End <= l.End+end
}

// randomCase is a tree, a page of it whose content has links of every kind
// to its pages and to none, and a rename or a move of one of its pages.
type randomCaseOf struct {
	pages, moved    []string
	aliases         [][]string
	page, pageAfter string
	content         string
	recased         domain.Recased
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
		c.recased = domain.Recased{ID: id, Name: lastOf(to)}
	}
	var body, properties []string
	for range 1 + r.IntN(8) {
		body = append(body, randomLink(r, c.pages, c.page))
		if r.IntN(3) == 0 {
			body = append(body, []string{"**b**", "_i_", "~~s~~", "==h==", "%% c %%", "%%", "`code`"}[r.IntN(7)])
		}
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
