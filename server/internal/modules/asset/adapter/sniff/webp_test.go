package sniff

import (
	"os"
	"testing"
)

// webpFile is a WebP of testdata: 7×5 pixels each, written by libwebp
// (Chromium's canvas encoder), without the color profile it adds.
func webpFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The three kinds of a WebP's first chunk each tell its size: a lossy
// frame's 14 bits, a lossless image's 14 bits less one, an extended
// canvas's 24 bits less one, a canvas wider than 14 bits hold too
// (M9/P1 design 3.11.2).
func TestWebPSize(t *testing.T) {
	for _, name := range []string{"vp8.webp", "vp8l.webp", "vp8x.webp"} {
		if w, h, ok := webpSize(webpFile(t, name)[:webpHeader]); !ok || w != 7 || h != 5 {
			t.Errorf("webpSize(%s) = %d, %d, %v; want 7×5", name, w, h, ok)
		}
	}
	wide := append([]byte{}, webpFile(t, "vp8x.webp")[:webpHeader]...)
	// A canvas of 20,000 × 70,000: 0x4e1f and 0x1116f, less one.
	copy(wide[24:30], []byte{0x1f, 0x4e, 0x00, 0x6f, 0x11, 0x01})
	if w, h, ok := webpSize(wide); !ok || w != 20000 || h != 70000 {
		t.Errorf("webpSize(a wide canvas) = %d, %d, %v; want 20000×70000", w, h, ok)
	}
}

// A header cut short, at any length below what webpSize reads, tells no
// size and does not panic; nor does a chunk of another kind, or a lossy
// frame that is no key frame.
func TestWebPSizeOfAHeaderCutShort(t *testing.T) {
	for _, name := range []string{"vp8.webp", "vp8l.webp", "vp8x.webp"} {
		data := webpFile(t, name)
		need := webpHeader
		if name == "vp8l.webp" {
			need = 25 // the signature and the 4 bytes of the sizes
		}
		for n := range need {
			if _, _, ok := webpSize(data[:n]); ok {
				t.Errorf("webpSize(%s cut to %d bytes) read a size", name, n)
			}
		}
	}
	other := append([]byte{}, webpFile(t, "vp8.webp")[:webpHeader]...)
	copy(other[12:16], "ALPH")
	if _, _, ok := webpSize(other); ok {
		t.Error("webpSize(an ALPH chunk first) read a size")
	}
	inter := append([]byte{}, webpFile(t, "vp8.webp")[:webpHeader]...)
	inter[20] |= 1
	if _, _, ok := webpSize(inter); ok {
		t.Error("webpSize(an interframe) read a size")
	}
}

// Any bytes: no panic, and a size read is within its chunk's bits.
func FuzzWebPSize(f *testing.F) {
	for _, name := range []string{"vp8.webp", "vp8l.webp", "vp8x.webp"} {
		f.Add(webpFile(f, name))
	}
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBPVP8X"))
	f.Fuzz(func(t *testing.T, data []byte) {
		w, h, ok := webpSize(data)
		if !ok {
			return
		}
		limit := 1 << 14
		if string(data[12:16]) == "VP8X" {
			limit = 1 << 24
		}
		if w < 1 || h < 1 || w > limit || h > limit {
			t.Errorf("webpSize(%x) = %d, %d", data, w, h)
		}
	})
}
