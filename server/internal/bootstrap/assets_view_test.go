package bootstrap

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"
)

// The reading view's part in the attachments (M7/P3 design 5.10): the
// attachments its links lead to shown at their contents' addresses, which
// the composition root's Assets gives the obsidian extension.

// sevenByFive is a PNG image 7 pixels wide and 5 high.
const sevenByFive = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x07\x00\x00\x00\x05\x08\x06\x00\x00\x00\x89\x9a\xf6\xd8\x00\x00\x00\x12" +
	"IDATx\xdac8ac\xf3\x1f\x17f\x18\x00I\x00:\nN\x9e\xe88\xd8\x1a\x00\x00\x00\x00IEND\xaeB`\x82"

// readingView is the reading view of the page id as by reads it: its HTML
// and when its attachments' addresses expire, nil for none.
func (tm acmeTeam) readingView(t *testing.T, by, id string) (string, *time.Time) {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/view", tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("%s's reading view of %s = %d %s", by, id, status, body)
	}
	var v struct {
		HTML    string     `json:"html"`
		Expires *time.Time `json:"assets_expire_at"`
	}
	decodeAnswer(t, body, &v)
	return v.HTML, v.Expires
}

// element is an element of a reading view: its name, its attributes and
// the text it holds.
type element struct {
	name  string
	attrs map[string]string
	text  string
}

// assetElements are the elements of html that show an attachment
// (nw-asset), in order.
func assetElements(html string) []element {
	var out []element
	open := -1 // the element whose text is being read
	z := xhtml.NewTokenizer(strings.NewReader(html))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return out
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			tok := z.Token()
			e := element{name: tok.Data, attrs: map[string]string{}}
			for _, a := range tok.Attr {
				e.attrs[a.Key] = a.Val
			}
			if slices.Contains(strings.Fields(e.attrs["class"]), "nw-asset") {
				out = append(out, e)
				if tok.Data != "img" {
					open = len(out) - 1
				}
			}
		case xhtml.TextToken:
			if open >= 0 {
				out[open].text += string(z.Text())
			}
		case xhtml.EndTagToken:
			open = -1
		}
	}
}

// A reading view shows the attachments its links lead to (M7/P3 design
// 5.5, 5.10): an image embedded as its <img>, of its caption and the size
// written, else of its own, an audio as its <audio>, a PDF embedded and an
// image linked as links to them, and a property link to one as a link,
// each at its content's address, which downloads its bytes; the view
// expires when the addresses do, as the content route signs them, and a
// view that shows none never, though its links lead to some. Deleted, an attachment's links lead
// nowhere and the others stay. The properties answer a property link to
// one with its kind and its address. Each address is the content's shown,
// not downloaded. Its Assets nil, the composition root fails it: the links
// would be text.
func TestAReadingViewShowsTheAttachmentsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	x := tm.upload(t, "alice", nb, a, "x.png", sevenByFive)
	sound := tm.upload(t, "alice", nb, a, "a.mp3", "ID3 a sound")
	doc := tm.upload(t, "alice", nb, "", "doc.pdf", "%PDF-1.4 a document")
	src := tm.createPageWith(t, "alice", nb, "", "Src",
		"---\ncover: \"[[doc.pdf]]\"\n---\n![[x.png|说明|300]] ![[x.png]] ![[a.mp3]] ![[doc.pdf]] [t](A/x.png)\n")

	before := time.Now()
	html, expires := tm.readingView(t, "bob", src)
	got := assetElements(html)
	type shown struct{ name, class, text, asset string }
	want := []shown{
		{"a", "nw-wikilink nw-asset", "doc.pdf", doc.ID}, {"img", "nw-asset", "", x.ID}, {"img", "nw-asset", "", x.ID},
		{"audio", "nw-asset", "", sound.ID},
		{"a", "nw-wikilink nw-embed nw-asset", "doc.pdf", doc.ID}, {"a", "nw-asset", "t", x.ID},
	}
	if len(got) != len(want) || strings.Contains(html, "data-nw-node") {
		t.Fatalf("the reading view shows %+v, want %+v, and no page:\n%s", got, want, html)
	}
	bytes := map[string]string{x.ID: sevenByFive, sound.ID: "ID3 a sound", doc.ID: "%PDF-1.4 a document"}
	for i, e := range got {
		address := e.attrs["src"] + e.attrs["href"]
		u, err := url.Parse(address)
		if err != nil || e.name != want[i].name || e.attrs["class"] != want[i].class || e.text != want[i].text ||
			u.Path != "/api/v0/assets/"+want[i].asset+"/content" || u.Query().Has("d") {
			t.Errorf("element %d is <%s %v>%s, want %+v", i, e.name, e.attrs, e.text, want[i])
			continue
		}
		if expires == nil || u.Query().Get("e") != strconv.FormatInt(expires.Unix(), 10) {
			t.Errorf("%s expires at %s, the view at %v", address, u.Query().Get("e"), expires)
		}
		if status, body := tm.download(t, address); status != http.StatusOK || body != bytes[want[i].asset] {
			t.Errorf("%s = %d %q, want the attachment's bytes", address, status, body)
		}
		if e.name == "a" && e.attrs["data-nw-size"] != strconv.Itoa(len(bytes[want[i].asset])) {
			t.Errorf("%s is of %s bytes, want %d", address, e.attrs["data-nw-size"], len(bytes[want[i].asset]))
		}
	}
	if img := got[1].attrs; img["alt"] != "说明" || img["width"] != "300" || img["height"] != "" || img["loading"] != "lazy" {
		t.Errorf("the image's attributes are %v, want its caption and width", img)
	}
	if img := got[2].attrs; img["alt"] != "x.png" || img["width"] != "7" || img["height"] != "5" {
		t.Errorf("the image's attributes are %v, want its target and its own size", img)
	}
	if expires != nil && (expires.Before(before.Add(time.Hour)) || expires.After(time.Now().Add(2*time.Hour))) {
		t.Errorf("the view expires at %v, not one to two hours from now", expires)
	}
	// Its links to attachments in a comment and in a link's text show none.
	hidden := tm.createPageWith(t, "alice", nb, "", "Hidden", "%%![[x.png]]%% [![[a.mp3]] [[doc.pdf]]](https://x.example)\n")
	if html, expires := tm.readingView(t, "bob", hidden); expires != nil || strings.Contains(html, "/api/v0/assets/") {
		t.Errorf("a view without attachments shown expires at %v, want never:\n%s", expires, html)
	}

	var props struct {
		Links []struct {
			Kind *string `json:"kind"`
			URL  *string `json:"url"`
		} `json:"links"`
	}
	tm.get(t, "bob", "/api/v0/pages/"+src+"/properties", &props)
	if len(props.Links) != 1 || props.Links[0].Kind == nil || *props.Links[0].Kind != "asset" || props.Links[0].URL == nil {
		t.Fatalf("the property links %+v, want one to an attachment with its address", props.Links)
	}
	if status, body := tm.download(t, *props.Links[0].URL); status != http.StatusOK || body != bytes[doc.ID] {
		t.Errorf("the property link's address = %d %q, want the document's bytes", status, body)
	}

	tm.send(t, nodeDeletion("alice", x.ID), http.StatusNoContent)
	html, _ = tm.readingView(t, "bob", src)
	if strings.Contains(html, x.ID) || !strings.Contains(html, `data-nw-target="x.png"`) || !strings.Contains(html, `data-nw-target="A/x.png"`) ||
		!strings.Contains(html, "/api/v0/assets/"+sound.ID+"/content") {
		t.Errorf("after the image's deletion, the reading view %s", html)
	}
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}
