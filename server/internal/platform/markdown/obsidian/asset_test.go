package obsidian_test

import (
	"context"
	"errors"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// attachment is an attachment the tests know: its id, and what Assets
// answers of it; one without a type is one Assets does not answer.
type attachment struct {
	id    uuid.UUID
	asset obsidian.Asset
}

// expiry is when the tests' attachments' addresses expire.
var expiry = time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // read only

// attachments are the attachments the tests know, by name: a link whose
// target is one of the names, in any folder, resolves to it. The browser
// shows all but a.zip.
//
//nolint:gochecknoglobals // read only
var attachments = map[string]attachment{
	"x.png": {uuid.MustParse("00000000-0000-4000-8000-0000000000a1"), obsidian.Asset{MIME: "image/png", Bytes: 70, Inline: true}},
	"dims.png": {
		uuid.MustParse("00000000-0000-4000-8000-0000000000a2"),
		obsidian.Asset{MIME: "image/png", Bytes: 71, Width: 7, Height: 5, Inline: true},
	},
	"a.mp3":    {uuid.MustParse("00000000-0000-4000-8000-0000000000a3"), obsidian.Asset{MIME: "audio/mpeg", Bytes: 72, Inline: true}},
	"v.webm":   {uuid.MustParse("00000000-0000-4000-8000-0000000000a4"), obsidian.Asset{MIME: "video/webm", Bytes: 73, Inline: true}},
	"doc.pdf":  {uuid.MustParse("00000000-0000-4000-8000-0000000000a5"), obsidian.Asset{MIME: "application/pdf", Bytes: 74, Inline: true}},
	"gone.png": {uuid.MustParse("00000000-0000-4000-8000-0000000000a6"), obsidian.Asset{}},
	"a.zip":    {uuid.MustParse("00000000-0000-4000-8000-0000000000a7"), obsidian.Asset{MIME: "application/zip", Bytes: 75}},
}

// contentURL is the address the tests' Assets gives the attachment id's
// content: with an '&', which the HTML escapes.
func contentURL(id uuid.UUID) string {
	return "/api/v0/assets/" + id.String() + "/content?b=1&e=2&s=3"
}

// resolveAttachments resolves the links to the pages known has and to the
// attachments attachments has, in any folder.
func resolveAttachments(ctx context.Context, p markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
	to, _ := resolveKnown(ctx, p, links)
	for _, l := range links {
		if a, ok := attachments[path.Base(l.Target)]; ok && l.Target != "" {
			to[l.Range.Start] = obsidian.Target{Node: a.id, Asset: true}
		}
	}
	return to, nil
}

// shownAssets answers what attachments has of ids, but for those without a
// type, each at its contentURL, expiring at expiry.
func shownAssets(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
	out := map[uuid.UUID]obsidian.Asset{}
	for _, a := range attachments {
		if slices.Contains(ids, a.id) && a.asset.MIME != "" {
			shown := a.asset
			shown.URL, shown.Expires = contentURL(a.id), expiry
			out[a.id] = shown
		}
	}
	return out, nil
}

// withAttachments is the Markdown whose links lead to the pages and the
// attachments the tests know.
func withAttachments(t testing.TB) *markdown.Markdown {
	t.Helper()
	return newMarkdownWith(t, obsidian.Options{Resolve: resolveAttachments, Assets: shownAssets})
}

// The markup of the attachment named name: its address, its id, and the
// attributes of a link to it, which downloads one the browser does not
// show.
func src(name string) string {
	return `src="` + strings.ReplaceAll(contentURL(attachments[name].id), "&", "&amp;") + `"`
}

func assetLink(name string) string {
	attrs := `href="` + strings.ReplaceAll(contentURL(attachments[name].id), "&", "&amp;") + `" data-nw-size="` +
		strconv.FormatInt(attachments[name].asset.Bytes, 10) + `"`
	if !attachments[name].asset.Inline {
		attrs += ` download=""`
	}
	return attrs
}

// img, audio and video are the elements of the attachment named name,
// with their other attributes.
func img(name, alt, dims string) string {
	return `<img class="nw-asset" ` + src(name) + ` alt="` + alt + `"` + dims + ` loading="lazy">`
}

func audio(name, label string) string {
	return `<audio class="nw-asset" ` + src(name) + ` controls="" preload="none" aria-label="` + label + `"></audio>`
}

func video(name, label, dims string) string {
	return `<video class="nw-asset" ` + src(name) + ` controls="" preload="none" aria-label="` + label + `"` + dims + `></video>`
}

// An embed and a Markdown image of an attachment are its image, audio or
// video, any other type a link to it; a wikilink, a Markdown link and a
// property link to one are a link to it, which downloads one the browser
// does not show (M7/P4 design 4.2). An image's text is its caption,
// else its target as written (Obsidian 1.12.7; the empty caption's
// nerve-defined); the last '|' of an embed's display text or an image's
// caption may give its width, or width and height, which an image without
// one takes from the image (M7/P3 design 5.5).
func TestAnAttachmentIsItsImageAudioVideoOrALink(t *testing.T) {
	page := `data-nw-node="` + known["Page"].String() + `"`
	p := func(s string) string { return "<p>" + s + "</p>\n" }
	checkRendersWith(t, withAttachments(t), []renderCase{
		{"an embed of an image", "![[x.png]]", p(img("x.png", "x.png", ""))},
		{"of an image of known size", "![[dims.png]]", p(img("dims.png", "dims.png", ` width="7" height="5"`))},
		{"a width", "![[dims.png|300]]", p(img("dims.png", "dims.png", ` width="300"`))},
		{"a width and a height", "![[x.png|300x200]]", p(img("x.png", "x.png", ` width="300" height="200"`))},
		{"a caption", "![[x.png|说明]]", p(img("x.png", "说明", ""))},
		{"a caption and a width", "![[x.png|说明|300]]", p(img("x.png", "说明", ` width="300"`))},
		{
			"captions without the spaces around them", "![[x.png| 说明 | 300]] ![[x.png|\t说明 ]] ![ 说明 ](x.png) ![ 说明 |300](x.png)",
			p(img("x.png", "说明", ` width="300"`) + " " + img("x.png", "说明", "") + " " + img("x.png", "说明", "") + " " +
				img("x.png", "说明", ` width="300"`)),
		},
		{
			"a blank caption, the target", "![[x.png| ]] ![ ](a.mp3) ![ |300](x.png) ![&#10;](a.mp3) ![&nbsp;](a.mp3) ![[x.png|\u3000|300]]",
			p(img("x.png", "x.png", "") + " " + audio("a.mp3", "a.mp3") + " " + img("x.png", "x.png", ` width="300"`) + " " + audio("a.mp3", "a.mp3") +
				" " + audio("a.mp3", "a.mp3") + " " + img("x.png", "x.png", ` width="300"`)),
		},
		{"in a folder", "![[A/x.png]] ![](A/x.png)", p(img("x.png", "A/x.png", "") + " " + img("x.png", "A/x.png", ""))},
		{"captions of '|' and a size", "![[x.png|a|b | 300x200 ]]", p(img("x.png", "a|b", ` width="300" height="200"`))},
		{"half a size, a caption", "![[x.png|300x]] ![[x.png|x200]]", p(img("x.png", "300x", "") + " " + img("x.png", "x200", ""))},
		{
			"sizes out of range, not given", "![[dims.png|0]] ![[x.png|20000]] ![[x.png|0x200]] ![[x.png|99999999999999999999]]",
			p(img("dims.png", "dims.png", ` width="7" height="5"`) + " " + img("x.png", "x.png", "") + " " +
				img("x.png", "x.png", ` height="200"`) + " " + img("x.png", "x.png", "")),
		},
		{"the largest and the least size", "![[x.png|10000x1]]", p(img("x.png", "x.png", ` width="10000" height="1"`))},
		{"an anchor", "![[x.png#a]]", p(img("x.png", "x.png &gt; a", ""))},
		{"escaped", `![[x.png|<b>&"]]`, p(img("x.png", "&lt;b&gt;&amp;&quot;", ""))},
		{"a Markdown image", "![](x.png)", p(img("x.png", "x.png", ""))},
		{"its caption and size", "![说明|300x200](x.png) ![300](x.png)", p(img("x.png", "说明", ` width="300" height="200"`) + " " + img("x.png", "x.png", ` width="300"`))},
		{"its caption's emphasis", "![*说明*](x.png)", p(img("x.png", "说明", ""))},
		{"a reference's", "![说明][r]\n\n[r]: x.png", p(img("x.png", "说明", ""))},
		{"inline", "a ![[x.png]] b", p("a " + img("x.png", "x.png", "") + " b")},
		{"audio", "![[a.mp3]] ![说明](a.mp3)", p(audio("a.mp3", "a.mp3") + " " + audio("a.mp3", "说明"))},
		{"video", "![[v.webm|说明|300x200]] ![](v.webm)", p(video("v.webm", "说明", ` width="300" height="200"`) + " " + video("v.webm", "v.webm", ""))},
		{
			"another type, a link", "![[doc.pdf]] ![[doc.pdf|说明|300]] ![](doc.pdf)",
			p(`<a class="nw-wikilink nw-embed nw-asset" ` + assetLink("doc.pdf") + `>doc.pdf</a> ` +
				`<a class="nw-wikilink nw-embed nw-asset" ` + assetLink("doc.pdf") + `>说明</a> ` +
				`<a class="nw-asset" ` + assetLink("doc.pdf") + `>doc.pdf</a>`),
		},
		{
			"one the browser does not show, a download", "![[a.zip]] ![z](a.zip) [[a.zip|z]] [z](a.zip)",
			p(`<a class="nw-wikilink nw-embed nw-asset" ` + assetLink("a.zip") + `>a.zip</a> ` +
				`<a class="nw-asset" ` + assetLink("a.zip") + `>z</a> <a class="nw-wikilink nw-asset" ` + assetLink("a.zip") + `>z</a> ` +
				`<a class="nw-asset" ` + assetLink("a.zip") + `>z</a>`),
		},
		{
			"a wikilink and a Markdown link", "[[x.png]] [[a.mp3|听]] [t](doc.pdf \"T\")",
			p(`<a class="nw-wikilink nw-asset" ` + assetLink("x.png") + `>x.png</a> ` +
				`<a class="nw-wikilink nw-asset" ` + assetLink("a.mp3") + `>听</a> ` +
				`<a class="nw-asset" ` + assetLink("doc.pdf") + ` title="T">t</a>`),
		},
		{
			"in a link, an image alone", "[![](x.png) ![[dims.png|9]]](https://x.example)",
			p(`<a href="https://x.example">` + img("x.png", "x.png", "") + " " + img("dims.png", "dims.png", ` width="9"`) + `</a>`),
		},
		{
			"in a link, the rest text", "[![](a.mp3) ![[v.webm]] ![[doc.pdf|d]] [[x.png]]](https://x.example)",
			p(`<a href="https://x.example"><span class="nw-asset">a.mp3</span> <span class="nw-wikilink nw-embed nw-asset">v.webm</span> ` +
				`<span class="nw-wikilink nw-embed nw-asset">d</span> <span class="nw-wikilink">x.png</span></a>`),
		},
		{
			"one Assets does not answer, text", "![[gone.png|g]] ![](gone.png) [[gone.png]] [t](gone.png)",
			p(`<span class="nw-wikilink nw-embed nw-asset">g</span> <span class="nw-asset">gone.png</span> ` +
				`<span class="nw-wikilink nw-asset">gone.png</span> <a class="nw-asset">t</a>`),
		},
		{
			"a page's embed and image, as before", "![[Page]] ![i](Page)",
			p(`<a class="nw-wikilink nw-embed" ` + page + `>Page</a> <span class="nw-image">i <a ` + page + `>Page</a></span>`),
		},
		{
			"a property link", "---\na: \"[[x.png|see]]\"\nb: \"[t](doc.pdf)\"\nc: \"[[gone.png]]\"\nd: \"[[a.zip]]\"\n---\n",
			`<div class="nw-scroll"><table class="nw-props"><tr><th>a</th><td><a class="nw-wikilink nw-asset" ` + assetLink("x.png") +
				`>see</a></td></tr><tr><th>b</th><td><a class="nw-asset" ` + assetLink("doc.pdf") + `>t</a></td></tr>` +
				`<tr><th>c</th><td><a class="nw-wikilink nw-asset">gone.png</a></td></tr>` +
				`<tr><th>d</th><td><a class="nw-wikilink nw-asset" ` + assetLink("a.zip") + `>a.zip</a></td></tr></table></div>` + "\n",
		},
	})
}

// A view writes at most MaxMedia audio and video elements, in the
// content's order, embeds and Markdown images alike; one past them is a
// link to its attachment. Images are not counted (M7/P3 design 5.5).
func TestAViewPlaysAtMostMaxMedia(t *testing.T) {
	m := withAttachments(t)
	var src strings.Builder
	for i := range obsidian.MaxMedia + 2 {
		if i%2 == 0 {
			src.WriteString("![[a.mp3]] ![[x.png]] ")
		} else {
			src.WriteString("![](v.webm) ")
		}
	}
	view, err := m.Render(context.Background(), m.Parse([]byte(src.String())), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	played := strings.Count(view.HTML, "<audio ") + strings.Count(view.HTML, "<video ")
	links := strings.Count(view.HTML, `<a class="nw-wikilink nw-embed nw-asset" `+assetLink("a.mp3")+`>a.mp3</a>`) +
		strings.Count(view.HTML, `<a class="nw-asset" `+assetLink("v.webm")+`>v.webm</a>`)
	if played != obsidian.MaxMedia || links != 2 || strings.Count(view.HTML, "<img ") != obsidian.MaxMedia/2+1 {
		t.Errorf("%d played, %d links, %d images:\n%s", played, links, strings.Count(view.HTML, "<img "), view.HTML)
	}
	// The last are the links.
	if first := strings.Index(view.HTML, ` href="`); strings.LastIndex(view.HTML, "<audio ") > first || strings.LastIndex(view.HTML, "<video ") > first {
		t.Errorf("an element after a link:\n%s", view.HTML)
	}
	// In a link, an audio or a video is text, not counted (M7/P3 review T1).
	inLinks := "[" + strings.Repeat("![[a.mp3]] ![](v.webm) ", obsidian.MaxMedia) + "](https://x.example) ![[a.mp3]] ![](v.webm)"
	linked, err := m.Render(context.Background(), m.Parse([]byte(inLinks)), markdown.Page{})
	if err != nil || !strings.HasSuffix(linked.HTML, "</a> "+audio("a.mp3", "a.mp3")+" "+video("v.webm", "v.webm", "")+"</p>\n") ||
		strings.Count(linked.HTML, "<audio ")+strings.Count(linked.HTML, "<video ") != 2 || strings.Count(linked.HTML, "nw-asset") != 2*obsidian.MaxMedia+2 {
		t.Errorf("media in a link: %v\n%s", err, linked.HTML)
	}
	// Another rendering counts afresh.
	again, err := m.Render(context.Background(), m.Parse([]byte(src.String())), markdown.Page{})
	if err != nil || again.HTML != view.HTML {
		t.Errorf("rendered again: %v\n%s", err, again.HTML)
	}
}

// Fetch asks Assets once, for the page's notebook, of the attachments the
// links lead to, each once, in the content's order, and of none for a page
// whose links lead to none; Assets' error is Render's. The view expires at
// the earliest of the addresses it writes: one it does not write, of an
// attachment in a link or a comment, does not count (M7/P3 review A1).
func TestFetchAsksAssetsOfTheAttachmentsTheLinksLeadTo(t *testing.T) {
	var asked [][]uuid.UUID
	var notebooks []uuid.UUID
	down := errors.New("down")
	fail := false
	early := expiry.Add(-time.Hour)
	m := newMarkdownWith(t, obsidian.Options{Resolve: resolveAttachments, Assets: func(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
		asked, notebooks = append(asked, ids), append(notebooks, nb)
		if fail {
			return nil, down
		}
		out, _ := shownAssets(ctx, nb, ids)
		a := out[attachments["a.mp3"].id]
		a.Expires = early
		out[attachments["a.mp3"].id] = a
		return out, nil
	}})
	page := markdown.Page{NotebookID: uuid.New(), PageID: uuid.New()}
	view, err := m.Render(context.Background(), m.Parse([]byte("![[x.png]] [[Page]] ![](a.mp3) [[x.png]] [[gone.png]] [[none]]")), page)
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{attachments["x.png"].id, attachments["a.mp3"].id, attachments["gone.png"].id}
	if len(asked) != 1 || !slices.Equal(asked[0], want) || notebooks[0] != page.NotebookID {
		t.Errorf("asked %v of %v, want %v of %v once", asked, notebooks, want, page.NotebookID)
	}
	if !view.Expires.Equal(early) {
		t.Errorf("Expires = %v, want %v", view.Expires, early)
	}
	for src, want := range map[string]time.Time{
		"[![](a.mp3)](https://x.example) ![[x.png]]": expiry,
		"[![](a.mp3)](https://x.example)":            {},
		"%%![[a.mp3]]%% ![[x.png]]":                  expiry,
		"[[Page]] [[none]]":                          {},
	} {
		asked = nil
		view, err := m.Render(context.Background(), m.Parse([]byte(src)), page)
		if err != nil || (asked == nil) != strings.HasPrefix(src, "[[Page") || !view.Expires.Equal(want) {
			t.Errorf("%s: asked %v, %v, expires %v, want %v", src, asked, err, view.Expires, want)
		}
	}
	fail = true
	if _, err := m.Render(context.Background(), m.Parse([]byte("![[x.png]]")), page); !errors.Is(err, down) {
		t.Errorf("Assets down: %v", err)
	}
}

// Without Assets, a link or an embed that leads to an attachment is text,
// never a page's link: no data-nw-node names an attachment (M7/P3 design
// 5.3).
func TestWithoutAssetsAnAttachmentIsText(t *testing.T) {
	m := newMarkdownWith(t, obsidian.Options{Resolve: resolveAttachments})
	src := "---\na: \"[[x.png]]\"\n---\n![[x.png]] [[x.png]] ![](a.mp3) [t](doc.pdf)\n"
	view, err := m.Render(context.Background(), m.Parse([]byte(src)), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<p><span class="nw-wikilink nw-embed nw-asset">x.png</span> <span class="nw-wikilink nw-asset">x.png</span> ` +
		`<span class="nw-asset">a.mp3</span> <a class="nw-asset">t</a></p>`
	if !strings.Contains(view.HTML, `<td><a class="nw-wikilink nw-asset">x.png</a></td>`) || !strings.Contains(view.HTML, want) ||
		strings.Contains(view.HTML, "data-nw-node") || strings.Contains(view.HTML, "src=") ||
		!view.Expires.IsZero() {
		t.Errorf("got %s, expires %v", view.HTML, view.Expires)
	}
	if err := markdowntest.CheckHTML(view.HTML, tasks.Extension(), obsidian.Extension(obsidian.Options{})); err != nil {
		t.Error(err)
	}
}

// A view writes at most MaxShown addresses of attachments, in the
// content's order, images', audios', videos' and links' alike, a link's
// once; one past them is text (M7/P3 review B1), and the view expires as
// those written do.
func TestAViewWritesAtMostMaxShownAddresses(t *testing.T) {
	m := withAttachments(t)
	src := strings.Repeat("![[x.png]] ", obsidian.MaxShown-3) + "[t](doc.pdf) [[doc.pdf]] ![](a.mp3) ![[x.png|i]] [u](doc.pdf) ![](v.webm)"
	view, err := m.Render(context.Background(), m.Parse([]byte(src)), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<a class="nw-asset" ` + assetLink("doc.pdf") + `>t</a> <a class="nw-wikilink nw-asset" ` + assetLink("doc.pdf") + `>doc.pdf</a> ` +
		audio("a.mp3", "a.mp3") + ` <span class="nw-wikilink nw-embed nw-asset">i</span> <a class="nw-asset">u</a> <span class="nw-asset">v.webm</span></p>` + "\n"
	if !strings.HasSuffix(view.HTML, want) || strings.Count(view.HTML, "/api/v0/assets/") != obsidian.MaxShown || !view.Expires.Equal(expiry) {
		t.Errorf("%d addresses, expires %v, ends %q", strings.Count(view.HTML, "/api/v0/assets/"), view.Expires, view.HTML[len(view.HTML)-600:])
	}
}
