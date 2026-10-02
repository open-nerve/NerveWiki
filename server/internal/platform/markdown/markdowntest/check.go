package markdowntest

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

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
	"h6": {"id"}, "hr": nil, "i": nil, "input": {"type", "checked", "disabled"}, "ins": nil, "kbd": nil,
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
	classes  = []string{"footnote-ref", "footnote-backref", "footnotes", "nw-image", "nw-props"}
	language = regexp.MustCompile(`^language-[a-z0-9_+#.-]+$`)
)

// CheckHTML checks the HTML of a reading view rendered with exts (M4/P3
// design 3.10): parsed back as a fragment, it holds only the elements and
// attributes above and the extensions' Markup, no comment; every address
// is on this site, http or https with a host, or mailto; every id starts
// with "nw-"; every class is the renderers' or the extensions'.
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
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return err
	}
	var errs []error
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			attrs, ok := elements[n.Data]
			if !ok {
				errs = append(errs, fmt.Errorf("element <%s>", n.Data))
			}
			for _, a := range n.Attr {
				if err := checkAttr(n.Data, a, attrs, urls, known); err != nil {
					errs = append(errs, err)
				}
			}
		case html.CommentNode, html.DoctypeNode:
			errs = append(errs, fmt.Errorf("a comment or doctype %q", n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return errors.Join(errs...)
}

func checkAttr(element string, a html.Attribute, attrs, urls, known []string) error {
	switch {
	case a.Namespace != "" || !slices.Contains(attrs, a.Key):
		return fmt.Errorf("attribute %s of <%s>", a.Key, element)
	case slices.Contains(urls, a.Key) && !onThisSiteOrAllowed(a.Val):
		return fmt.Errorf("address %q of <%s>", a.Val, element)
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
