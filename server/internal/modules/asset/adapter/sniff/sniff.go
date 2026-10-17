// Package sniff tells what a file's bytes are: net/http's sniffing of its
// first bytes, and an image's size from its header (M7/P2 design 3.5): a
// WebP's from its RIFF header (M9/P1 design 3.11.2).
package sniff

import (
	"bufio"
	"image"
	_ "image/gif"  // GIF's header
	_ "image/jpeg" // JPEG's
	_ "image/png"  // PNG's
	"io"
	"net/http"
)

// maxHeader is the most bytes of an image read for its size: a JPEG's
// header may follow its metadata.
const maxHeader = 1 << 20

// Sniffer is app.Sniffer.
type Sniffer struct{}

// Sniff implements app.Sniffer: net/http's DetectContentType.
func (Sniffer) Sniff(head []byte) string {
	return http.DetectContentType(head)
}

// Size implements app.Sniffer: a WebP's RIFF header, or image.DecodeConfig
// over the first MiB of r, for PNG, JPEG and GIF. No format is registered
// with image for WebP: that would change what image.Decode reads in the
// whole process, and only the size is wanted. A file shorter than a
// WebP's header is no error: a GIF may be twenty-odd bytes.
func (Sniffer) Size(r io.Reader) (width, height int, ok bool) {
	b := bufio.NewReader(io.LimitReader(r, maxHeader))
	if head, _ := b.Peek(webpHeader); isWebP(head) {
		return webpSize(head)
	}
	cfg, _, err := image.DecodeConfig(b)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}
