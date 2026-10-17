package sniff

import (
	"bytes"
	"encoding/binary"
)

// webpHeader is the most bytes of a WebP's start webpSize reads: the RIFF
// header, the first chunk's header and ten bytes of it.
const webpHeader = 30

// isWebP reports whether head starts as a WebP's RIFF container does.
func isWebP(head []byte) bool {
	return len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP"))
}

// webpSize reads a WebP's width and height from the first bytes of its
// RIFF container (RFC 9649), a file of length bytes: "RIFF", a size, "WEBP",
// then the first chunk: VP8 (lossy: the frame tag, the start code 9d 01
// 2a, then 14-bit width and height), VP8L (lossless: the signature 0x2f,
// then 14-bit width-1 and height-1, the alpha bit, and 3 version bits,
// which must be 0) or VP8X (extended, a chunk of 10 bytes: 24-bit canvas
// width-1 and height-1). ok is false for anything else, for a header cut
// short, and for a RIFF larger than the file or a first chunk larger than
// the RIFF: a file cut short, which a decoder refuses (M9/P1 S2 review,
// B's suspicion 1); so is a VP8L of another version or a VP8X chunk of
// another size, which libwebp and x/image refuse (fix check B).
func webpSize(head []byte, length int64) (width, height int, ok bool) {
	if !isWebP(head) || len(head) < 20 {
		return 0, 0, false
	}
	riff, chunk := int64(binary.LittleEndian.Uint32(head[4:8]))+8, int64(binary.LittleEndian.Uint32(head[16:20]))+20
	if riff > length || chunk > riff {
		return 0, 0, false
	}
	data := head[20:]
	switch string(head[12:16]) {
	case "VP8 ":
		// A key frame's tag has its low bit clear; the frame starts with it.
		if len(data) < 10 || data[0]&1 != 0 || !bytes.Equal(data[3:6], []byte{0x9d, 0x01, 0x2a}) {
			return 0, 0, false
		}
		width = int(binary.LittleEndian.Uint16(data[6:8]) & 0x3fff)
		height = int(binary.LittleEndian.Uint16(data[8:10]) & 0x3fff)
	case "VP8L":
		if len(data) < 5 || data[0] != 0x2f {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(data[1:5])
		if bits>>29 != 0 {
			return 0, 0, false
		}
		width, height = int(bits&0x3fff)+1, int(bits>>14&0x3fff)+1
	case "VP8X":
		if len(data) < 10 || binary.LittleEndian.Uint32(head[16:20]) != 10 {
			return 0, 0, false
		}
		width = int(uint32(data[4])|uint32(data[5])<<8|uint32(data[6])<<16) + 1
		height = int(uint32(data[7])|uint32(data[8])<<8|uint32(data[9])<<16) + 1
	default:
		return 0, 0, false
	}
	if width < 1 || height < 1 {
		return 0, 0, false
	}
	return width, height, true
}
