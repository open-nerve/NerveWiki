package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// renameCase is a case of the fixture set's rename/ (its README).
type renameCase struct {
	Pages   []string            `json:"pages"`
	Aliases map[string][]string `json:"aliases"`
	Page    string              `json:"page"`
	From    string              `json:"from"`
	To      string              `json:"to"`
}

// Every rename case's page is written as the case has it after the rename
// or move: its links resolved in the case's tree before and after it, and
// written again by Rewrite (M6/P4 design 2, 3), as extracted by the
// application's extensions. The ids go in the pages' order, as in the
// resolution cases.
func TestTheRenameCasesAreWrittenAsTheCasesHaveThem(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range markdowntest.RenameCases(t) {
		t.Run(f.Name, func(t *testing.T) {
			var c renameCase
			if err := json.Unmarshal(f.JSON, &c); err != nil {
				t.Fatal(err)
			}
			if got, left := rewrite(t, m, c, string(f.Content)); got != string(f.Want) || len(left) > 0 {
				t.Errorf("written\n%q\nwant\n%q\nleft %+v", got, f.Want, left)
			}
		})
	}
}

// rewrite is content, c's page's, written again by Rewrite after c's rename
// or move, with the links it left: its links resolved in c's tree before
// and after the change, as extracted by m.
func rewrite(t *testing.T, m *markdown.Markdown, c renameCase, content string) (string, []domain.Link) {
	t.Helper()
	if c.Page == "" {
		c.Page = "src"
	}
	if c.Pages == nil {
		c.Pages = []string{c.From, c.Page}
	}
	moved := func(p string) string {
		if p == c.From || strings.HasPrefix(p, c.From+"/") {
			return c.To + p[len(c.From):]
		}
		return p
	}
	after := make([]string, len(c.Pages))
	aliases := make([][]string, len(c.Pages))
	for i, p := range c.Pages {
		after[i], aliases[i] = moved(p), c.Aliases[p]
	}
	was, now := pagesOf(t, c.Pages, aliases), pagesOf(t, after, aliases)
	facts, err := markdownadapter.PageFacts(m.Parse([]byte(content)).Facts())
	if err != nil {
		t.Fatal(err)
	}
	links := make([]domain.Resolved, len(facts.Links))
	for i, l := range facts.Links {
		links[i] = domain.Resolved{Link: l, Before: was.resolve(l, c.Page), After: now.resolve(l, moved(c.Page))}
	}
	var caseOnly uuid.UUID
	if parentOf(c.From) == parentOf(c.To) && shared.TitleKey(lastOf(c.From)) == shared.TitleKey(lastOf(c.To)) {
		caseOnly = was.ids[c.From]
	}
	tree := domain.Tree{Before: was.byID, After: now.byID, Named: now.named}
	edits, left := domain.Rewrite(content, now.paths[moved(c.Page)], links, tree, caseOnly)
	return domain.Apply(content, edits), left
}

// pages is a tree of pages at their paths, by path, by id, by title key
// and by alias key, as the index would hand them to Resolve.
type pages struct {
	ids     map[string]uuid.UUID
	paths   map[string][]domain.Step
	byID    map[uuid.UUID]domain.Node
	named   map[string][]domain.Node
	aliased map[string][]domain.Node
}

// pagesOf is the tree of paths, the page at paths[i] with the id i+1 and
// the aliases aliases[i]; a page's parent is among them, in any order.
func pagesOf(t *testing.T, paths []string, aliases [][]string) pages {
	t.Helper()
	p := pages{
		ids: map[string]uuid.UUID{}, paths: map[string][]domain.Step{}, byID: map[uuid.UUID]domain.Node{},
		named: map[string][]domain.Node{}, aliased: map[string][]domain.Node{},
	}
	for i, path := range paths {
		var id uuid.UUID
		id[15] = byte(i + 1)
		p.ids[path] = id
	}
	for i, path := range paths {
		var steps []domain.Step
		segments := strings.Split(path, "/")
		for j, name := range segments {
			id, ok := p.ids[strings.Join(segments[:j+1], "/")]
			if !ok {
				t.Fatalf("the page %q has no parent among the pages", path)
			}
			steps = append(steps, domain.Step{ID: id, Key: shared.TitleKey(name), Name: name})
		}
		node := domain.Node{ID: p.ids[path], Path: steps}
		p.paths[path], p.byID[node.ID] = steps, node
		key := steps[len(steps)-1].Key
		p.named[key] = append(p.named[key], node)
		for _, a := range aliases[i] {
			p.aliased[shared.TitleKey(a)] = append(p.aliased[shared.TitleKey(a)], node)
		}
	}
	return p
}

// resolve resolves l from the page at from, its candidates found as the
// index finds them, by the last keys, and its aliased pages by those keys.
func (p pages) resolve(l domain.Link, from string) domain.Resolution {
	target, ok := domain.ParseTarget(l.Target)
	if !ok {
		return domain.Resolution{}
	}
	var candidates []domain.Node
	aliased := map[string][]domain.Node{}
	for _, key := range target.LastKeys() {
		candidates = append(candidates, p.named[key]...)
		aliased[key] = p.aliased[key]
	}
	return domain.Resolve(target, p.paths[from], candidates, aliased)
}

// parentOf is a path's parent's, "" at the root; lastOf its last name.
func parentOf(path string) string {
	return path[:max(strings.LastIndexByte(path, '/'), 0)]
}

func lastOf(path string) string {
	return path[strings.LastIndexByte(path, '/')+1:]
}
