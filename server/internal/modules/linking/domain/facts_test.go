package domain_test

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A page's tags and aliases are one a title key, as first written; a
// tag's count is how often it is written.
func TestTagsAndAliasesAreOneAKey(t *testing.T) {
	tags := domain.TagsOf([]string{"ToDo", "x", "todo", "TODO", "Straße", "strasse"})
	if want := []domain.Tag{{Key: "todo", Name: "ToDo", Count: 3}, {Key: "x", Name: "x", Count: 1}, {Key: "strasse", Name: "Straße", Count: 2}}; !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %+v, want %+v", tags, want)
	}
	aliases := domain.AliasesOf([]string{"Al", "AL", "Straße", "STRASSE"})
	if want := []domain.Alias{{Key: "al", Name: "Al"}, {Key: "strasse", Name: "Straße"}}; !reflect.DeepEqual(aliases, want) {
		t.Errorf("aliases = %+v, want %+v", aliases, want)
	}
	if domain.TagsOf(nil) != nil || domain.AliasesOf(nil) != nil {
		t.Error("no names are some tags or aliases")
	}
}

// A link's keys are its target's last segment's, in the forms it may be
// written in; none for a target that resolves to nothing.
func TestALinksKeys(t *testing.T) {
	for target, want := range map[string][]string{
		"A/Note.md": {"note", "note.md"},
		"Note":      {"note"},
		"A//B":      nil,
	} {
		if got := (domain.Link{Target: target}).Keys(); !slices.Equal(got, want) {
			t.Errorf("the keys of %q = %v, want %v", target, got, want)
		}
	}
}

// A key longer than MaxKey is not kept: a link with such a target has no
// keys, or its key without ".md" alone, and such a tag or alias is none.
func TestKeysPastMaxKeyAreNotKept(t *testing.T) {
	long := strings.Repeat("x", domain.MaxKey)
	for target, want := range map[string][]string{
		long:                 {long},
		long + "x":           nil,
		long[3:] + ".md":     {long[3:], long[3:] + ".md"},
		long[2:] + ".md":     {long[2:]},
		"A/" + long + "x.md": nil,
	} {
		if got := (domain.Link{Target: target}).Keys(); !slices.Equal(got, want) {
			t.Errorf("the keys of a target of %d bytes = %d keys, want %d", len(target), len(got), len(want))
		}
	}
	if tags := domain.TagsOf([]string{long + "x", "a"}); len(tags) != 1 || tags[0].Key != "a" {
		t.Errorf("tags = %+v, want a alone", tags)
	}
	if aliases := domain.AliasesOf([]string{long + "x", "a"}); len(aliases) != 1 || aliases[0].Key != "a" {
		t.Errorf("aliases = %+v, want a alone", aliases)
	}
}

// No title's key comes near MaxKey: a title has at most 255 bytes, and the
// key of a title of any one character, as NFC writes it, the most a fold
// or a normalization grows a character by, has at most twice as many.
func TestNoTitlesKeyComesNearMaxKey(t *testing.T) {
	worst, at := 0, rune(0)
	for r := rune(1); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		c := norm.NFC.String(string(r))
		if len(shared.TitleKey(c)) <= len(c) {
			continue // a title of it has a key no longer than itself
		}
		name, f := shared.CheckTitle("title", strings.Repeat(c, 255/len(c)))
		if f != nil {
			continue
		}
		if n := len(shared.TitleKey(name)); n > worst {
			worst, at = n, r
		}
	}
	if worst > 2*255 || worst > domain.MaxKey/2 {
		t.Errorf("a title of %U has a key of %d bytes, past twice its most", at, worst)
	}
}

// Of a page's tags and aliases, the first domain.MaxNames keys are kept, in
// the order written; a key kept counts every time it is written, a later
// one too (M6/P5 review r2-M2).
func TestAPagesFirstNamesAreKept(t *testing.T) {
	var names []string
	for i := range domain.MaxNames + 2 {
		names = append(names, fmt.Sprintf("n%d", i))
	}
	names = append(names, "N0")
	tags := domain.TagsOf(names)
	if len(tags) != domain.MaxNames || tags[0].Count != 2 || tags[len(tags)-1].Key != fmt.Sprintf("n%d", domain.MaxNames-1) {
		t.Errorf("%d tags, the first counted %d, the last %+v", len(tags), tags[0].Count, tags[len(tags)-1])
	}
	aliases := domain.AliasesOf(names)
	if len(aliases) != domain.MaxNames || aliases[len(aliases)-1].Key != fmt.Sprintf("n%d", domain.MaxNames-1) {
		t.Errorf("%d aliases, the last %+v", len(aliases), aliases[len(aliases)-1])
	}
}
