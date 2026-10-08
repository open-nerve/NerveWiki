// Package sniff tells what a file's bytes are: net/http's sniffing of its
// first bytes, and an image's size from its header (M7/P2 design 3.5).
package sniff

import (
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

// Size implements app.Sniffer: image.DecodeConfig over the first MiB of r,
// for PNG, JPEG and GIF.
func (Sniffer) Size(r io.Reader) (width, height int, ok bool) {
	cfg, _, err := image.DecodeConfig(io.LimitReader(r, maxHeader))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}
