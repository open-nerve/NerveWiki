package markdowntest

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// allowed is each element a reading view may hold, with the attributes it
// may carry: what goldmark's renderers and the package's marks write, and
// what a user's HTML may keep. It is written apart from the sanitizer's
// allowlist, so that the check does not judge the code by the code.
//
//nolint:gochecknoglobals // read only
var allowed = map[string][]string{
	"a": {"href", "title", "class", "role"}, "abbr": {"title"}, "b": nil, "bdi": nil, "bdo": {"dir"},
	"blockquote": nil, "br": nil, "caption": nil, "cite": nil, "code": {"class"}, "dd": nil, "del": nil,
	"details": {"open"}, "dfn": nil, "div": {"class", "role"}, "dl": nil, "dt": nil, "em": nil,
	"figcaption": nil, "figure": nil, "h1": {"id"}, "h2": {"id"}, "h3": {"id"}, "h4": {"id"}, "h5": {"id"},
	"h6": {"id"}, "hr": nil, "i": nil, "ins": nil, "kbd": nil,
	"li": {"id"}, "mark": nil, "ol": {"start", "reversed"}, "p": nil, "pre": nil, "q": nil, "rp": nil,
	"rt": nil, "ruby": nil, "s": nil, "samp": nil, "small": nil, "span": {"class"}, "strong": nil,
	"sub": nil, "summary": nil, "sup": {"id"}, "table": {"class"}, "tbody": nil,
	"td": {"align", "colspan", "rowspan"}, "tfoot": nil, "th": {"align", "colspan", "rowspan"},
	"thead": nil, "tr": nil, "u": nil, "ul": nil, "var": nil, "wbr": nil,
}

// classes are the classes the renderers give, besides a code block's
// language-*.
//
//nolint:gochecknoglobals // read only
var (
	classes  = []string{"footnote-ref", "footnote-backref", "footnotes", "nw-image", "nw-props", "nw-scroll"}
	language = regexp.MustCompile(`^language-[a-z0-9_+#.-]+$`)
)

// void are HTML's elements that have no end tag, an extension's among
// them.
//
//nolint:gochecknoglobals // read only
var void = []string{"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"}

// CheckHTML checks the HTML of a reading view rendered with exts (M4/P3
// design 3.10): read back by x/net/html's tokenizer, it holds only the
// elements and attributes above and the extensions' Markup, no comment or
// doctype; every address is on this site, http or https with a host, or
// mailto, and every src, which only an extension's elements carry, a path
// of this site (M7/P3 design 5.9), so that nothing loads from elsewhere
// (overall design 4.3); every id starts with "nw-"; every class is the renderers' or the
// extensions'; each end tag closes the innermost element open, and none is
// left open, so a user's HTML stays inside where it was written; and no
// link is in a link, which a browser would take apart. It reads
// tokens, not a tree: the tree builder refuses more than 512 elements open,
// which a user's nested tags reach, and a check of the tags needs no tree.
func CheckHTML(s string, exts ...markdown.Extension) error {
	elements, urls := map[string][]string{}, []string{"href"}
	known := slices.Clone(classes)
	for name, attrs := range allowed {
		elements[name] = attrs
	}
	for _, e := range exts {
		for name, attrs := range e.Markup.Elements {
			elements[name] = append(slices.Clone(elements[name]), attrs...)
		}
		urls = append(urls, e.Markup.URLs...)
		known = append(known, e.Markup.Classes...)
	}
	var errs []error
	var open []string
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch tt := z.Next(); tt {
		case html.ErrorToken:
			if !errors.Is(z.Err(), io.EOF) {
				errs = append(errs, z.Err())
			}
			if len(open) > 0 {
				errs = append(errs, fmt.Errorf("elements left open: %v", open))
			}
			return errors.Join(errs...)
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			t := z.Token()
			attrs, ok := elements[t.Data]
			if !ok {
				errs = append(errs, fmt.Errorf("element <%s>", t.Data))
			}
			for _, a := range t.Attr {
				if err := checkAttr(t.Data, a, attrs, urls, known); err != nil {
					errs = append(errs, err)
				}
			}
			if tt == html.StartTagToken && t.Data == "a" && slices.Contains(open, "a") {
				errs = append(errs, errors.New("a link in a link"))
			}
			var err error
			if open, err = nest(open, tt, t.Data); err != nil {
				errs = append(errs, err)
			}
		case html.CommentToken, html.DoctypeToken:
			errs = append(errs, fmt.Errorf("a comment or doctype %q", z.Token().Data))
		}
	}
}

// nest is the elements open after a tag of name: a start tag opens one
// unless it is void, an end tag must close the innermost.
func nest(open []string, tt html.TokenType, name string) ([]string, error) {
	switch {
	case tt == html.StartTagToken && !slices.Contains(void, name):
		return append(open, name), nil
	case tt != html.EndTagToken:
		return open, nil
	case len(open) == 0:
		return open, fmt.Errorf("</%s> with no element open", name)
	case open[len(open)-1] != name:
		return open, fmt.Errorf("</%s> closes <%s>", name, open[len(open)-1])
	}
	return open[:len(open)-1], nil
}

func checkAttr(element string, a html.Attribute, attrs, urls, known []string) error {
	switch {
	case !slices.Contains(attrs, a.Key):
		return fmt.Errorf("attribute %s of <%s>", a.Key, element)
	case slices.Contains(urls, a.Key) && !onThisSiteOrAllowed(a.Val):
		return fmt.Errorf("address %q of <%s>", a.Val, element)
	case a.Key == "src" && !aPathHere(a.Val):
		return fmt.Errorf("src %q of <%s>, not a path of this site", a.Val, element)
	case a.Key == "id" && !strings.HasPrefix(a.Val, "nw-"):
		return fmt.Errorf("id %q of <%s>", a.Val, element)
	case a.Key == "class":
		for _, c := range strings.Fields(a.Val) {
			ofCode := element == "code" && language.MatchString(c)
			if !ofCode && !slices.Contains(known, c) {
				return fmt.Errorf("class %q of <%s>", c, element)
			}
		}
	}
	return nil
}

// onThisSiteOrAllowed tells whether an address is http or https with a
// host, mailto, or a path, query or fragment of this site, as a browser
// reads it.
func onThisSiteOrAllowed(addr string) bool {
	if strings.ContainsFunc(addr, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return false
	}
	addr = strings.ReplaceAll(strings.Trim(addr, " "), `\`, "/")
	u, err := url.Parse(addr)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return true
	case "":
		return !strings.HasPrefix(addr, "//")
	}
	return false
}

// aPathHere tells whether an address is a path of this site from its root,
// as a browser reads it: what it loads comes from here.
func aPathHere(addr string) bool {
	if !onThisSiteOrAllowed(addr) {
		return false
	}
	addr = strings.ReplaceAll(strings.Trim(addr, " "), `\`, "/")
	u, err := url.Parse(addr)
	return err == nil && u.Scheme == "" && u.Host == "" && strings.HasPrefix(addr, "/") && !strings.HasPrefix(addr, "//")
}

// The HTML of a reading view is at most Amplification times its content's
// size plus Headroom (M4/P3 design 3.10). A footnote's reference and its
// back link, the most per byte, are about 48 times theirs. Below their
// budgets reference links and a frontmatter's aliases repeat up to about
// 2 MB however short the content: an image writes its address twice and
// each '&' as five bytes, so references up to 10 times internal/harden's
// MinExpansion; the aliases' scalars up to 5 times their budget, and
// 10 000 nodes of about 40 bytes of table each. Headroom keeps twice that.
const (
	Amplification = 64
	Headroom      = 4 << 20
)

// CheckSize checks that out, the HTML of a reading view of content, is at
// most Amplification times its size plus Headroom.
func CheckSize(content []byte, out string) error {
	if limit := Amplification*len(content) + Headroom; len(out) > limit {
		return fmt.Errorf("%d bytes of HTML for %d of content, more than %d", len(out), len(content), limit)
	}
	return nil
}
