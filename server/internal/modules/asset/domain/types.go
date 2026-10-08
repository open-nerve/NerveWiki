package domain

import (
	"slices"
	"strings"
)

// The types an attachment is served as (M7 design 4.4; M7/P2 design 3.5):
// those a browser shows by itself, each by its extension, and the
// containers that sniffing its first bytes may find for it. Every other
// file is Octet, which is downloaded.

// Octet is the type of a file the server does not show: it is downloaded.
const Octet = "application/octet-stream"

// shown is a type the table names: the type, and what sniffing may find
// for its files, besides nothing (Octet or text/plain).
type shown struct {
	mime       string
	containers []string
}

// byExtension is the table, by lower-case extension.
func byExtension() map[string]shown {
	same := func(mime string) shown { return shown{mime: mime, containers: []string{mime}} }
	return map[string]shown{
		"png":  same("image/png"),
		"jpg":  same("image/jpeg"),
		"jpeg": same("image/jpeg"),
		"gif":  same("image/gif"),
		"webp": same("image/webp"),
		"bmp":  same("image/bmp"),
		"avif": {mime: "image/avif"},
		"svg":  {mime: "image/svg+xml", containers: []string{"text/xml"}},
		"mp3":  same("audio/mpeg"),
		"wav":  same("audio/wave"),
		"flac": {mime: "audio/flac"},
		"ogg":  {mime: "audio/ogg", containers: []string{"application/ogg"}},
		"oga":  {mime: "audio/ogg", containers: []string{"application/ogg"}},
		"m4a":  {mime: "audio/mp4", containers: []string{"video/mp4"}},
		"mp4":  same("video/mp4"),
		"webm": same("video/webm"),
		"ogv":  {mime: "video/ogg", containers: []string{"application/ogg"}},
		"pdf":  same("application/pdf"),
	}
}

// TypeOf is the type of a file named name whose first bytes sniff as
// sniffed (net/http's sniffing, M7/P2 design 3.5): the extension's, when
// the table has it and the bytes are of its container or tell nothing
// (Octet, text/plain); the sniffed type, when the table has no such
// extension but has that type; Octet otherwise. Bytes that sniff as HTML
// are Octet, whatever the name: a browser would run them.
func TypeOf(name, sniffed string) string {
	sniffed, _, _ = strings.Cut(sniffed, ";")
	sniffed = strings.TrimSpace(sniffed)
	ext, known := byExtension()[extension(name)]
	switch {
	case sniffed == "text/html":
		return Octet
	case known:
		if sniffed == Octet || sniffed == "text/plain" || slices.Contains(ext.containers, sniffed) {
			return ext.mime
		}
		return Octet
	case shownType(sniffed):
		return sniffed
	}
	return Octet
}

// shownType reports whether the table has the type mime.
func shownType(mime string) bool {
	for _, s := range byExtension() {
		if s.mime == mime {
			return true
		}
	}
	return false
}

// Inline reports whether a file of type mime is shown in the browser: a
// type of the table, unless the download was asked for (d=1).
func Inline(mime string, download bool) bool {
	return !download && mime != Octet
}

// HasSize reports whether the server reads the size in pixels of a file
// of type mime.
func HasSize(mime string) bool {
	return mime == "image/png" || mime == "image/jpeg" || mime == "image/gif"
}

// MaxSide is the largest width or height kept of an image: a larger one is
// not kept.
const MaxSide = 65535

// extension is name's lower-case extension, after its last dot; "" for
// none.
func extension(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}
