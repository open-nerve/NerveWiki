package sniff

import (
	"encoding/binary"
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

// sizeOf is webpSize of a whole file.
func sizeOf(data []byte) (int, int, bool) {
	return webpSize(data[:min(len(data), webpHeader)], int64(len(data)))
}

// edited is name's bytes with edit made to them.
func edited(t testing.TB, name string, edit func(data []byte) []byte) []byte {
	t.Helper()
	return edit(append([]byte{}, webpFile(t, name)...))
}

// The three kinds of a WebP's first chunk each tell its size: a lossy
// frame's 14 bits, its 2 scale bits apart; a lossless image's 14 bits less
// one, its alpha bit apart; an extended canvas's 24 bits less one, a
// canvas wider than 14 bits hold too (M9/P1 design 3.11.2). A broken start
// code or signature tells none; nor does a RIFF that is no WebP's, a lossy
// chunk first or not (M9/P1 S2 review, B-M2); nor a lossless image of a
// version but 0, nor an extended chunk of a size but 10 (fix check B).
func TestWebPSize(t *testing.T) {
	scaled := edited(t, "vp8.webp", func(d []byte) []byte { d[27] |= 0xc0; d[29] |= 0xc0; return d })
	alpha := edited(t, "vp8l.webp", func(d []byte) []byte { d[24] |= 0x10; return d })
	wide := edited(t, "vp8x.webp", func(d []byte) []byte {
		// A canvas of 20,000 × 70,000: 0x4e1f and 0x1116f, less one.
		copy(d[24:30], []byte{0x1f, 0x4e, 0x00, 0x6f, 0x11, 0x01})
		return d
	})
	for _, tt := range []struct {
		name string
		data []byte
		w, h int
	}{
		{"vp8.webp", webpFile(t, "vp8.webp"), 7, 5},
		{"vp8l.webp", webpFile(t, "vp8l.webp"), 7, 5},
		{"vp8x.webp", webpFile(t, "vp8x.webp"), 7, 5},
		{"a lossy frame scaled", scaled, 7, 5},
		{"a lossless image with alpha", alpha, 7, 5},
		{"a wide canvas", wide, 20000, 70000},
		{"a lossy frame's start code broken", edited(t, "vp8.webp", func(d []byte) []byte { d[24] = 0x9c; return d }), 0, 0},
		{"a lossless image's signature 0x2e", edited(t, "vp8l.webp", func(d []byte) []byte { d[20] = 0x2e; return d }), 0, 0},
		{"an AVI with a lossy chunk first", edited(t, "vp8.webp", func(d []byte) []byte { copy(d[8:12], "AVI "); return d }), 0, 0},
		{"a lossless image of version 1", edited(t, "vp8l.webp", func(d []byte) []byte { d[24] |= 0x20; return d }), 0, 0},
		{"a lossless image of version 4", edited(t, "vp8l.webp", func(d []byte) []byte { d[24] |= 0x80; return d }), 0, 0},
		{"an extended chunk of 12 bytes", edited(t, "vp8x.webp", func(d []byte) []byte {
			binary.LittleEndian.PutUint32(d[16:20], 12)
			return d
		}), 0, 0},
		{"an extended chunk of 9 bytes", edited(t, "vp8x.webp", func(d []byte) []byte {
			binary.LittleEndian.PutUint32(d[16:20], 9)
			return d
		}), 0, 0},
	} {
		w, h, ok := sizeOf(tt.data)
		if want := tt.w > 0; ok != want || w != tt.w || h != tt.h {
			t.Errorf("webpSize(%s) = %d, %d, %v; want %d×%d", tt.name, w, h, ok, tt.w, tt.h)
		}
	}
}

// A header cut short, at any length below what webpSize reads, tells no
// size and does not panic; nor does a chunk of another kind, or a lossy
// frame that is no key frame; nor a file shorter than its RIFF says, as
// one cut short after its header, nor a first chunk longer than its RIFF:
// a decoder refuses both. A file longer than its RIFF tells its size
// (M9/P1 S2 review, B's suspicion 1).
func TestWebPSizeOfAHeaderCutShort(t *testing.T) {
	for _, name := range []string{"vp8.webp", "vp8l.webp", "vp8x.webp"} {
		data := webpFile(t, name)
		need := webpHeader
		if name == "vp8l.webp" {
			need = 25 // the signature and the 4 bytes of the sizes
		}
		for n := range need {
			if _, _, ok := webpSize(data[:n], int64(len(data))); ok {
				t.Errorf("webpSize(%s's header cut to %d bytes) read a size", name, n)
			}
		}
		if _, _, ok := sizeOf(data[:len(data)-1]); ok {
			t.Errorf("webpSize(%s cut by a byte) read a size", name)
		}
		if _, _, ok := sizeOf(data[:webpHeader]); ok {
			t.Errorf("webpSize(%s cut after its header) read a size", name)
		}
		if _, _, ok := sizeOf(append(append([]byte{}, data...), 0, 0)); !ok {
			t.Errorf("webpSize(%s and two bytes more) read no size", name)
		}
		long := edited(t, name, func(d []byte) []byte {
			binary.LittleEndian.PutUint32(d[16:20], binary.LittleEndian.Uint32(d[4:8])-11)
			return d
		})
		if _, _, ok := sizeOf(long); ok {
			t.Errorf("webpSize(%s, its first chunk past its RIFF) read a size", name)
		}
	}
	for name, edit := range map[string]func(d []byte) []byte{
		"an ALPH chunk first": func(d []byte) []byte { copy(d[12:16], "ALPH"); return d },
		"an interframe":       func(d []byte) []byte { d[20] |= 1; return d },
	} {
		if _, _, ok := sizeOf(edited(t, "vp8.webp", edit)); ok {
			t.Errorf("webpSize(%s) read a size", name)
		}
	}
}

// Any bytes: no panic, and a size read is within its chunk's bits. The
// seeds, which go test runs, have the scale, alpha and version bits set too.
func FuzzWebPSize(f *testing.F) {
	for _, name := range []string{"vp8.webp", "vp8l.webp", "vp8x.webp"} {
		f.Add(webpFile(f, name))
	}
	f.Add(edited(f, "vp8.webp", func(d []byte) []byte { d[27] |= 0xc0; d[29] |= 0xc0; return d }))
	f.Add(edited(f, "vp8l.webp", func(d []byte) []byte { d[24] |= 0x10; return d }))
	f.Add(edited(f, "vp8l.webp", func(d []byte) []byte { d[24] |= 0xe0; return d }))
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBPVP8X"))
	f.Fuzz(func(t *testing.T, data []byte) {
		w, h, ok := sizeOf(data)
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
