package markdownadapter_test

import (
	"strings"
	"testing"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
)

// A tag's name is kept by its title key, as a page's tags are: without
// one '/' it ends with; a name no tag has, or that PostgreSQL's text would
// not hold, or whose key is too long to be kept, has none (M6/P5 design 5).
func TestATagNameIsKeptByTheKeyOfItsTag(t *testing.T) {
	tests := []struct {
		name string
		key  string
		ok   bool
	}{
		{"Project", "project", true},
		{"a/B", "a/b", true},
		{"a/", "a", true},
		{"a_b-c", "a_b-c", true},
		{"中文", "中文", true},
		{"Straße", "strasse", true},
		{"y1984", "y1984", true},
		{"1984", "", false},
		{"#a", "", false},
		{"a b", "", false},
		{"a.b", "", false},
		{"", "", false},
		{"/", "", false},
		{"a\x00b", "", false},
		{"a\xffb", "", false},
		{strings.Repeat("a", 1024), strings.Repeat("a", 1024), true},
		{strings.Repeat("a", 1025), "", false},
	}
	for _, tt := range tests {
		key, ok := markdownadapter.TagKey(tt.name)
		if key != tt.key && tt.ok || ok != tt.ok {
			t.Errorf("TagKey(%q) = %q, %t; want %q, %t", tt.name, key, ok, tt.key, tt.ok)
		}
	}
}

// A tag a page writes is kept by the key its name has.
func TestATagOfAPageIsKeptByTheKeyOfItsName(t *testing.T) {
	f := factsOf(t, "---\ntags: [Proj/Sub, '#Other']\n---\n#Body/Tag/ and #Straße\n")
	for _, tag := range f.Tags {
		if key, ok := markdownadapter.TagKey(tag.Name); !ok || key != tag.Key {
			t.Errorf("the tag %+v: TagKey(%q) = %q, %t", tag, tag.Name, key, ok)
		}
	}
	if len(f.Tags) != 4 {
		t.Errorf("tags %+v", f.Tags)
	}
}
