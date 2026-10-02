package markdown

import (
	"net/url"
	"regexp"
	"strings"
)

var scheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// SafeURL is the one allowlist of addresses (M4 design 4, "rendering"):
// Markdown's links, images and autolinks, a user's <a href>, and what
// extensions write all go through it. It lets through http and https with a
// host, mailto, and addresses on this site: a path, a query, a fragment.
// The host decides what is another site, so //host and /\host are not on
// this one. One with a control character other than those a browser drops
// is not let through. The address it returns has the characters a browser
// drops from one dropped too; it still needs escaping for an attribute.
func SafeURL(raw string) (string, bool) {
	s := strings.TrimFunc(raw, func(r rune) bool { return r <= ' ' })
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	if strings.ContainsFunc(s, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return "", false
	}
	if m := scheme.FindString(s); m != "" {
		switch strings.ToLower(m) {
		case "http:", "https:":
			u, err := url.Parse(strings.ReplaceAll(s, `\`, "/"))
			if err != nil || u.Host == "" {
				return "", false
			}
			return s, true
		case "mailto:":
			return s, true
		}
		return "", false
	}
	path := strings.ReplaceAll(s, `\`, "/")
	if strings.HasPrefix(path, "//") {
		return "", false
	}
	if u, err := url.Parse(path); err != nil || u.Host != "" || u.Scheme != "" {
		return "", false
	}
	return s, true
}
