package obsidian_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// newMarkdown is the Markdown with the composition root's extensions, its
// links resolving to the pages the tests know.
func newMarkdown(t testing.TB) *markdown.Markdown {
	t.Helper()
	return newMarkdownWith(t, obsidian.Options{Resolve: resolveKnown})
}

// newMarkdownWith is the Markdown with the composition root's extensions,
// the dialect's with o.
func newMarkdownWith(t testing.TB, o obsidian.Options) *markdown.Markdown {
	t.Helper()
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(o)})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// extracted is what m extracts of content.
func extracted(m *markdown.Markdown, content string) obsidian.Extracted {
	got, _ := m.Parse([]byte(content)).Extracted(obsidian.Name).(obsidian.Extracted)
	return got
}

// fixtureLink is a link as a fixture's JSON writes it.
type fixtureLink struct {
	Kind    string  `json:"kind"`
	Target  string  `json:"target"`
	Anchor  *string `json:"anchor"`
	Display *string `json:"display"`
	Key     *string `json:"key"`
	Range   [2]int  `json:"range"`
}

// asFixture is l as a fixture's JSON writes it.
func asFixture(l obsidian.Link) fixtureLink {
	orNull := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	return fixtureLink{
		Kind: string(l.Kind), Target: l.Target, Anchor: orNull(l.Anchor), Display: orNull(l.Display),
		Key: orNull(l.Key), Range: [2]int{l.Range.Start, l.Range.Stop},
	}
}

// show is links, one a line.
func show(links []fixtureLink) string {
	s := ""
	for _, l := range links {
		raw, _ := json.Marshal(l)
		s += fmt.Sprintf("\n  %s", raw)
	}
	return s
}

// Every fixture's links and tags are what its JSON says (rules 4–10; M6/P1
// design 5).
func TestTheFixturesLinksAndTagsAreTheirs(t *testing.T) {
	m := newMarkdown(t)
	for _, f := range markdowntest.Fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			var want struct {
				Links []fixtureLink `json:"links"`
				Tags  []string      `json:"tags"`
			}
			if err := json.Unmarshal(f.JSON, &want); err != nil {
				t.Fatal(err)
			}
			got := extracted(m, string(f.Content))
			links := []fixtureLink{}
			for _, l := range got.Links {
				links = append(links, asFixture(l))
			}
			if want.Links == nil {
				want.Links = []fixtureLink{}
			}
			if !reflect.DeepEqual(links, want.Links) {
				t.Errorf("links%s\nwant%s", show(links), show(want.Links))
			}
			tags := []string{}
			for _, tg := range got.Tags {
				tags = append(tags, tg.Name)
			}
			if want.Tags == nil {
				want.Tags = []string{}
			}
			if !reflect.DeepEqual(tags, want.Tags) {
				t.Errorf("tags %q, want %q", tags, want.Tags)
			}
		})
	}
}
