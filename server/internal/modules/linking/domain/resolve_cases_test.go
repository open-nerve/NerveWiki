package domain_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// resolveCase is a case of the fixture set's resolve/ (its README).
type resolveCase struct {
	Pages   []string            `json:"pages"`
	Aliases map[string][]string `json:"aliases"`
	Links   []struct {
		From      string  `json:"from"`
		Link      string  `json:"link"`
		To        *string `json:"to"`
		Ambiguous bool    `json:"ambiguous"`
	} `json:"links"`
}

// Every link of the fixture set's resolution cases resolves to its page, as
// extracted by the application's dialect: the cases' ids go in the pages'
// order, as they would were the pages made in it.
func TestTheResolutionCasesResolveAsWritten(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{obsidian.Extension()})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range markdowntest.ResolveCases(t) {
		t.Run(f.Name, func(t *testing.T) {
			var c resolveCase
			if err := json.Unmarshal(f.JSON, &c); err != nil {
				t.Fatal(err)
			}
			tree := treeOf(c.Pages, c.Aliases)
			for _, l := range c.Links {
				links := m.Parse([]byte(l.Link)).Facts().Extracted(obsidian.Name).(obsidian.Extracted).Links
				if len(links) != 1 {
					t.Fatalf("%s: %d links extracted", l.Link, len(links))
				}
				var got domain.Resolution
				if target, ok := domain.ParseTarget(links[0].Target); ok {
					got = tree.resolve(target, l.From)
				}
				want := domain.Resolution{Ambiguous: l.Ambiguous}
				if l.To != nil {
					want.ID = tree.ids[*l.To]
				}
				if got != want {
					t.Errorf("%s from %s: %s, want %s", l.Link, l.From, tree.name(got), tree.name(want))
				}
			}
		})
	}
}

// caseTree is a case's pages, as the index would hand them to Resolve.
type caseTree struct {
	ids     map[string]uuid.UUID
	paths   map[string][]domain.Step
	aliases map[string][]string // a page's alias keys
}

func treeOf(pages []string, aliases map[string][]string) caseTree {
	tree := caseTree{ids: map[string]uuid.UUID{}, paths: map[string][]domain.Step{}, aliases: map[string][]string{}}
	for i, p := range pages {
		var id uuid.UUID
		id[15] = byte(i + 1)
		tree.ids[p] = id
		parent := p[:max(strings.LastIndex(p, "/"), 0)]
		step := domain.Step{ID: id, Key: shared.TitleKey(p[strings.LastIndex(p, "/")+1:])}
		tree.paths[p] = append(slices.Clone(tree.paths[parent]), step)
		for _, a := range aliases[p] {
			tree.aliases[p] = append(tree.aliases[p], shared.TitleKey(a))
		}
	}
	return tree
}

// resolve finds target's candidates as the index does, by the last key, and
// its aliased pages for a name alone, and resolves it from the page from.
func (c caseTree) resolve(target domain.Target, from string) domain.Resolution {
	var candidates, aliased []domain.Node
	for p, path := range c.paths {
		node := domain.Node{ID: c.ids[p], Path: path}
		if slices.Contains(target.LastKeys(), path[len(path)-1].Key) {
			candidates = append(candidates, node)
		}
		if slices.ContainsFunc(c.aliases[p], func(a string) bool { return slices.Contains(target.LastKeys(), a) }) {
			aliased = append(aliased, node)
		}
	}
	return domain.Resolve(target, c.paths[from], candidates, aliased)
}

// name is the page r resolves to, by its path in the case.
func (c caseTree) name(r domain.Resolution) string {
	for p, id := range c.ids {
		if id == r.ID && r.ID != (uuid.UUID{}) {
			if r.Ambiguous {
				return p + " (ambiguous)"
			}
			return p
		}
	}
	return "nothing"
}
