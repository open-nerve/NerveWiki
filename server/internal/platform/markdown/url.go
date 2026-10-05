package markdown

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
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

// reserved are the characters whose escapes DecodeURI keeps.
const reserved = ";/?:@&=+$,#"

// DecodeURI is JavaScript's decodeURI: each %XX decoded, an escaped
// reserved character's kept as it is; s as it is if an escape is malformed
// or its bytes are not UTF-8.
func DecodeURI(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '%' {
			b.WriteByte(s[i])
			i++
			continue
		}
		c, ok := unhex(s, i)
		if !ok {
			return s
		}
		if c < utf8.RuneSelf {
			if strings.IndexByte(reserved, c) >= 0 {
				b.WriteString(s[i : i+3])
			} else {
				b.WriteByte(c)
			}
			i += 3
			continue
		}
		var size int
		switch {
		case c&0xE0 == 0xC0:
			size = 2
		case c&0xF0 == 0xE0:
			size = 3
		case c&0xF8 == 0xF0:
			size = 4
		default:
			return s
		}
		bs := []byte{c}
		for k := 1; k < size; k++ {
			c, ok := unhex(s, i+3*k)
			if !ok || c&0xC0 != 0x80 {
				return s
			}
			bs = append(bs, c)
		}
		if r, n := utf8.DecodeRune(bs); r == utf8.RuneError && n <= 1 {
			return s
		}
		b.Write(bs)
		i += 3 * size
	}
	return b.String()
}

// unhex is the byte the escape %XX at s[i] writes.
func unhex(s string, i int) (byte, bool) {
	if i+2 >= len(s) || s[i] != '%' {
		return 0, false
	}
	hi, ok1 := hexDigit(s[i+1])
	lo, ok2 := hexDigit(s[i+2])
	return hi<<4 | lo, ok1 && ok2
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
