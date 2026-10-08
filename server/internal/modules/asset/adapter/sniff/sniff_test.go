package sniff_test

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/sniff"
)

// encoded is a w×h image in each of the formats whose size is read.
func encoded(t *testing.T, w, h int) map[string][]byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	out := map[string][]byte{}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	out["image/png"] = bytes.Clone(b.Bytes())
	b.Reset()
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	out["image/jpeg"] = bytes.Clone(b.Bytes())
	b.Reset()
	if err := gif.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	out["image/gif"] = bytes.Clone(b.Bytes())
	return out
}

// Sniff is net/http's; Size reads PNG, JPEG and GIF headers.
func TestSniffAndSize(t *testing.T) {
	var s sniff.Sniffer
	for mime, data := range encoded(t, 7, 5) {
		if got := s.Sniff(data[:min(len(data), 512)]); got != mime {
			t.Errorf("Sniff(%s) = %q", mime, got)
		}
		if w, h, ok := s.Size(bytes.NewReader(data)); !ok || w != 7 || h != 5 {
			t.Errorf("Size(%s) = %d, %d, %v; want 7×5", mime, w, h, ok)
		}
	}
	if got := s.Sniff([]byte("<html><body>")); got != "text/html; charset=utf-8" {
		t.Errorf("Sniff(HTML) = %q", got)
	}
	png := encoded(t, 7, 5)["image/png"]
	for name, data := range map[string][]byte{"no image": []byte("not an image"), "a cut PNG": png[:20], "nothing": nil} {
		if _, _, ok := s.Size(bytes.NewReader(data)); ok {
			t.Errorf("Size(%s) read a size, want none", name)
		}
	}
}

// A JPEG whose size comes after more than a MiB of comments is not read:
// Size reads at most a MiB.
func TestSizeReadsAtMostAMiB(t *testing.T) {
	data := encoded(t, 7, 5)["image/jpeg"]
	comment := append([]byte{0xFF, 0xFE, 0xFF, 0xFF}, bytes.Repeat([]byte{'x'}, 0xFFFF-2)...)
	padded := append([]byte{}, data[:2]...) // SOI
	for range 17 {
		padded = append(padded, comment...)
	}
	padded = append(padded, data[2:]...)
	if _, _, ok := (sniff.Sniffer{}).Size(bytes.NewReader(padded)); ok {
		t.Error("Size read a JPEG's size past a MiB of comments, want none")
	}
	if w, h, ok := (sniff.Sniffer{}).Size(bytes.NewReader(append(append(append([]byte{}, data[:2]...), comment...), data[2:]...))); !ok ||
		w != 7 || h != 5 {
		t.Errorf("Size(a JPEG after one comment) = %d, %d, %v; want 7×5", w, h, ok)
	}
}
