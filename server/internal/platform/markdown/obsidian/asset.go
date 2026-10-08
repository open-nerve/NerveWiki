package obsidian

import (
	"slices"
	"strconv"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// MaxMedia is how many audio and video elements a reading view writes, in
// the content's order; an embed past them is a link to its attachment
// (M7/P3 design 5.5; v0.1 design 13.1, item 31). Images are not counted:
// they load lazily.
const MaxMedia = 20

// MaxShown is how many addresses of attachments a reading view writes, in
// the content's order, an image's, an audio's, a video's or a link's; one
// past them is text (M7/P3 review B1; v0.1 design 13.1, item 31). Each
// costs a few hundred bytes of HTML, however few bytes write it, in a
// table's cell too: bounded as a whole, they stay within CheckSize's
// headroom, and so do the images a reader's tab loads.
const MaxShown = 2000

// address is what the view writes of the attachment id, counting it and
// the expiry of its address: false for one Assets did not answer, and
// past MaxShown.
func (v view) address(id uuid.UUID) (Asset, bool) {
	a, ok := v.assets[id]
	if !ok || v.shown == nil || v.shown.addresses >= MaxShown {
		return Asset{}, false
	}
	v.shown.addresses++
	if !a.Expires.IsZero() && (v.shown.expires.IsZero() || a.Expires.Before(v.shown.expires)) {
		v.shown.expires = a.Expires
	}
	return a, true
}

// assetLink is the attributes of a link to the attachment id (M7/P3 design
// 5.5), which the view writes: its content's address, whose path names it,
// and its size in bytes, which the front end shows in the reader's
// language; none for one the view writes no address of.
func (v view) assetLink(id uuid.UUID) []markdown.Attr {
	a, ok := v.address(id)
	if !ok {
		return nil
	}
	return linkAttrs(a)
}

// linkAttrs is the attributes of a link to a, which the view writes.
func linkAttrs(a Asset) []markdown.Attr {
	return []markdown.Attr{{Name: "href", Value: a.URL}, {Name: "data-nw-size", Value: strconv.FormatInt(a.Bytes, 10)}}
}

// image is how the Markdown image whose destination starts at start is
// written, if it leads to an attachment: as its embed (M7/P3 design 5.5),
// its caption and size read from the text it shows, its text its caption
// or else its target.
func (v view) image(start int) (markdown.Image, bool) {
	t, ok := v.target(start)
	if !ok || !t.Asset {
		return nil, false
	}
	l := v.links[start]
	return func(w markdown.Writer, shown string, inLink bool) {
		caption, sz := sized(shown)
		if strings.TrimSpace(caption) == "" {
			caption = targetShown(l.Target, l.Anchor)
		}
		v.embed(w, t.Node, nil, caption, sz, inLink)
	}, true
}

// targetShown is the text a link to target#anchor shows without a display
// text, as Obsidian shows it.
func targetShown(target, anchor string) string {
	switch {
	case target == "":
		return anchor
	case anchor == "":
		return target
	}
	return target + " > " + anchor
}

// embed writes, the whole of it, an embed of the attachment id, a
// wikilink's or a Markdown image's, which shows text, its caption or its
// target, and is written with sz (M7/P3 design 5.5): an image as an <img>;
// an audio or a video as its element while the view has written fewer
// than MaxMedia; any other, and one past them, as a link to it. In a link,
// which may hold neither a link nor a control, an image is its <img> and
// any other its text; so is one the view writes no address of. Its
// classes are class and nw-asset, an element's nw-asset alone.
func (v view) embed(w markdown.Writer, id uuid.UUID, class []string, text string, sz size, inLink bool) {
	kind, _, _ := strings.Cut(v.assets[id].MIME, "/")
	a, ok := Asset{}, false
	if !inLink || kind == "image" {
		a, ok = v.address(id)
	}
	cls := strings.Join(slices.Concat(class, []string{"nw-asset"}), " ")
	switch {
	case !ok:
		_, _ = w.WriteString(`<span class="` + cls + `">`)
		escaped(w, []byte(text))
		_, _ = w.WriteString("</span>")
	case kind == "image":
		v.element(w, "img", a, []markdown.Attr{{Name: "alt", Value: text}}, imageSize(sz, a), "")
	case (kind == "audio" || kind == "video") && v.play():
		var dims []markdown.Attr
		if kind == "video" {
			dims = sz.attrs()
		}
		v.element(w, kind, a, []markdown.Attr{{Name: "controls", Value: ""}, {Name: "preload", Value: "none"},
			{Name: "aria-label", Value: text}}, dims, "</"+kind+">")
	default:
		_, _ = w.WriteString(`<a class="` + cls + `"`)
		markdown.WriteAttrs(w, linkAttrs(a))
		_ = w.WriteByte('>')
		escaped(w, []byte(text))
		_, _ = w.WriteString("</a>")
	}
}

// element writes the element of an attachment's image, audio or video, its
// content at its address, whose path names it, then closing; attrs and
// dims come after its src. It is short: it may be written for every few
// bytes of the content (markdowntest.CheckSize).
func (v view) element(w markdown.Writer, element string, a Asset, attrs, dims []markdown.Attr, closing string) {
	_, _ = w.WriteString("<" + element)
	all := slices.Concat([]markdown.Attr{{Name: "class", Value: "nw-asset"}, {Name: "src", Value: a.URL}}, attrs, dims)
	if element == "img" {
		all = append(all, markdown.Attr{Name: "loading", Value: "lazy"})
	}
	markdown.WriteAttrs(w, all)
	_, _ = w.WriteString(">" + closing)
}

// play tells whether the view may write one more audio or video element,
// counting it if so.
func (v view) play() bool {
	if v.shown == nil || v.shown.played >= MaxMedia {
		return false
	}
	v.shown.played++
	return true
}

// imageSize is an image's width and height: those written, or else the
// image's own, when it has them, so that the page does not move as it
// loads.
func imageSize(sz size, a Asset) []markdown.Attr {
	if sz == (size{}) && a.Width > 0 && a.Height > 0 {
		sz = size{width: a.Width, height: a.Height}
	}
	return sz.attrs()
}

// attrs is a size's width and height, those it gives.
func (sz size) attrs() []markdown.Attr {
	var out []markdown.Attr
	if sz.width > 0 {
		out = append(out, markdown.Attr{Name: "width", Value: strconv.Itoa(sz.width)})
	}
	if sz.height > 0 {
		out = append(out, markdown.Attr{Name: "height", Value: strconv.Itoa(sz.height)})
	}
	return out
}
