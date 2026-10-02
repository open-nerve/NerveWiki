package markdown

import (
	"math/rand/v2"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// tokenized is the value x/net/html's tokenizer reads of an attribute
// written v.
func tokenized(t *testing.T, v string) string {
	t.Helper()
	q := `"`
	if strings.Contains(v, q) {
		q = "'"
	}
	z := html.NewTokenizer(strings.NewReader("<a title=" + q + v + q + ">"))
	if z.Next() != html.StartTagToken {
		t.Fatalf("%q: no start tag", v)
	}
	return z.Token().Attr[0].Val
}

// An inline tag's attribute reads its line breaks and references as the
// tokenizer an HTML block goes through does.
func TestAnAttributesReferencesResolveAsTheTokenizersDo(t *testing.T) {
	values := []string{
		"/search?q=x&section=news&not=1&copy=2", "&amp;", "&amp", "&ampx", "&amp=", "&amp;=", "&sect", "&section",
		"&sect=", "&copy;x", "&copyx;", "&fjlig;", "&NotEqualTilde;", "&bne;", "&nosuch;", "&", "&=", "&;", "&#",
		"&#x", "&#;", "&#65", "&#65;", "&#x41", "&#X41;", "&#x41g", "&#0;", "&#128;", "&#xD800;", "&#99999999;",
		"a&lt;b&gt;c", "&LT", "&ltx", "&lt=", "x\x00y", "&#9;", "p\r\nq", "p\rq\r", "&amp\r\n=",
	}
	pieces := []string{"&", "#", "x", "41", ";", "=", "amp", "sect", "ion", "copy", "lt", "fjlig", "a", "1", " ", "?", "\r", "\r\n", "\n"}
	r := rand.New(rand.NewPCG(20261002, 5))
	for range 20000 {
		var b strings.Builder
		for range 1 + r.IntN(8) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		values = append(values, b.String())
	}
	for _, v := range values {
		_, _, attrs := readTag([]byte(`<a title="` + strings.ReplaceAll(v, `"`, "") + `">`))
		for _, got := range attrs {
			if want := tokenized(t, strings.ReplaceAll(v, `"`, "")); got != want {
				t.Errorf("%q: %q, the tokenizer %q", v, got, want)
			}
		}
	}
}
