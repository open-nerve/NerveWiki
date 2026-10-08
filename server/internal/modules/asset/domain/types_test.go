package domain_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// Each extension of the table, with the bytes of its container or bytes
// that tell nothing, is its type; with another container's, Octet. An
// extension the table lacks takes a sniffed type the table has. HTML is
// never shown.
func TestTypeOf(t *testing.T) {
	const (
		octet = "application/octet-stream"
		plain = "text/plain; charset=utf-8"
		html  = "text/html; charset=utf-8"
	)
	for _, tt := range []struct {
		name, sniffed, want string
	}{
		{"a.png", "image/png", "image/png"},
		{"a.jpg", "image/jpeg", "image/jpeg"},
		{"a.jpeg", "image/jpeg", "image/jpeg"},
		{"a.gif", "image/gif", "image/gif"},
		{"a.webp", "image/webp", "image/webp"},
		{"a.bmp", "image/bmp", "image/bmp"},
		{"a.avif", octet, "image/avif"},
		{"a.svg", "text/xml; charset=utf-8", "image/svg+xml"},
		{"a.svg", plain, "image/svg+xml"},
		{"a.mp3", "audio/mpeg", "audio/mpeg"},
		{"a.wav", "audio/wave", "audio/wave"},
		{"a.flac", octet, "audio/flac"},
		{"a.ogg", "application/ogg", "audio/ogg"},
		{"a.oga", "application/ogg", "audio/ogg"},
		{"a.m4a", "video/mp4", "audio/mp4"},
		{"a.mp4", "video/mp4", "video/mp4"},
		{"a.webm", "video/webm", "video/webm"},
		{"a.ogv", "application/ogg", "video/ogg"},
		{"a.pdf", "application/pdf", "application/pdf"},
		{"a.png", octet, "image/png"},
		{"a.png", plain, "image/png"},
		{"a.PNG", "image/png", "image/png"},
		{"a.Jpeg", octet, "image/jpeg"},
		{"a.png", "image/jpeg", octet},
		{"a.svg", "image/png", octet},
		{"a.pdf", "application/zip", octet},
		{"a.mp3", "application/ogg", octet},
		{"a.png", html, octet},
		{"a.svg", html, octet},
		{"a.html", html, octet},
		{"a.txt", plain, octet},
		{"a.zip", "application/zip", octet},
		{"a.bin", "image/png", "image/png"},
		{"README", "application/pdf", "application/pdf"},
		{"README", "application/ogg", octet},
		{"README", "text/xml; charset=utf-8", octet},
		{"README", octet, octet},
		{"a.tar.gz", "application/x-gzip", octet},
	} {
		if got := domain.TypeOf(tt.name, tt.sniffed); got != tt.want {
			t.Errorf("TypeOf(%q, %q) = %q, want %q", tt.name, tt.sniffed, got, tt.want)
		}
	}
}

func TestInline(t *testing.T) {
	for _, tt := range []struct {
		mime     string
		download bool
		want     bool
	}{
		{"image/png", false, true},
		{"image/svg+xml", false, true},
		{"application/pdf", false, true},
		{"image/png", true, false},
		{"application/octet-stream", false, false},
		{"application/octet-stream", true, false},
		// A type out of the table, which TypeOf never tells but a row may
		// hold: downloaded, as Octet.
		{"text/html", false, false},
	} {
		if got := domain.Inline(tt.mime, tt.download); got != tt.want {
			t.Errorf("Inline(%q, %v) = %v, want %v", tt.mime, tt.download, got, tt.want)
		}
	}
}

// A type of the table is served as itself; any other as Octet.
func TestServed(t *testing.T) {
	for mime, want := range map[string]string{"image/png": "image/png", "image/svg+xml": "image/svg+xml", "application/pdf": "application/pdf",
		"application/octet-stream": domain.Octet, "text/html": domain.Octet, "text/plain": domain.Octet} {
		if got := domain.Served(mime); got != want {
			t.Errorf("Served(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestHasSize(t *testing.T) {
	for mime, want := range map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": false,
		"image/svg+xml": false, "application/octet-stream": false} {
		if got := domain.HasSize(mime); got != want {
			t.Errorf("HasSize(%q) = %v, want %v", mime, got, want)
		}
	}
}

// IDOf reads back the keys Key writes, and no other.
func TestKey(t *testing.T) {
	id := uuid.MustParse("0192b7c4-5e7a-7d2f-9b1e-3c4d5e6f7a8b")
	if got := domain.Key(id); got != "blobs/0192b7c4-5e7a-7d2f-9b1e-3c4d5e6f7a8b" {
		t.Errorf("Key() = %q", got)
	}
	if got, ok := domain.IDOf(domain.Key(id)); !ok || got != id {
		t.Errorf("IDOf(Key()) = %v, %v; want the id", got, ok)
	}
	for _, key := range []string{"blobs/0192B7C4-5E7A-7D2F-9B1E-3C4D5E6F7A8B", "blobs/0192b7c45e7a7d2f9b1e3c4d5e6f7a8b", "other/" + id.String(),
		id.String(), "blobs/notes.txt", "blobs/"} {
		if got, ok := domain.IDOf(key); ok {
			t.Errorf("IDOf(%q) = %v, want none", key, got)
		}
	}
}
