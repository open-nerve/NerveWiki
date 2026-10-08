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

// assetLink is the attributes of a link to the attachment id (M7/P3 design
// 5.5): its content's address, whose path names it, and its size in bytes,
// which the front end shows in the reader's language; none for one Assets
// did not answer.
func (v view) assetLink(id uuid.UUID) []markdown.Attr {
	a, ok := v.assets[id]
	if !ok {
		return nil
	}
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
		if caption == "" {
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
// any other its text; so is one Assets did not answer anywhere. Its
// classes are class and nw-asset, an element's nw-asset alone.
func (v view) embed(w markdown.Writer, id uuid.UUID, class []string, text string, sz size, inLink bool) {
	a, ok := v.assets[id]
	kind, _, _ := strings.Cut(a.MIME, "/")
	switch {
	case ok && kind == "image":
		v.element(w, "img", a, []markdown.Attr{{Name: "alt", Value: text}}, imageSize(sz, a), "")
		return
	case ok && !inLink && (kind == "audio" || kind == "video") && v.play():
		var dims []markdown.Attr
		if kind == "video" {
			dims = sz.attrs()
		}
		v.element(w, kind, a, []markdown.Attr{{Name: "controls", Value: ""}, {Name: "preload", Value: "none"},
			{Name: "aria-label", Value: text}}, dims, "</"+kind+">")
		return
	}
	cls := strings.Join(slices.Concat(class, []string{"nw-asset"}), " ")
	if !ok || inLink {
		_, _ = w.WriteString(`<span class="` + cls + `">`)
		escaped(w, []byte(text))
		_, _ = w.WriteString("</span>")
		return
	}
	_, _ = w.WriteString(`<a class="` + cls + `"`)
	markdown.WriteAttrs(w, v.assetLink(id))
	_ = w.WriteByte('>')
	escaped(w, []byte(text))
	_, _ = w.WriteString("</a>")
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
	if v.played == nil || *v.played >= MaxMedia {
		return false
	}
	*v.played++
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
