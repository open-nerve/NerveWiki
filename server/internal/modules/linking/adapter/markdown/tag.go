package markdownadapter

import (
	"strings"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// TagKey is the key that the tag name, without its '#', is kept by, as
// PageFacts keeps a page's tags (M6/P5 design 5): a tag as Obsidian's tag
// pane counts it (obsidian.CountedTag), whose title key is at most
// domain.MaxKey bytes. ok is false for a name no page's tag has: none of
// those, or not valid UTF-8, or holding U+0000, which PostgreSQL's text
// does not hold.
func TagKey(name string) (string, bool) {
	if !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return "", false
	}
	tag, ok := obsidian.CountedTag(name)
	if !ok {
		return "", false
	}
	key := shared.TitleKey(tag)
	return key, len(key) <= domain.MaxKey
}
