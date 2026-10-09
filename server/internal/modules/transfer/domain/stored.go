package domain

import (
	"path"
	"strings"
)

// storedExtensions are the extensions of formats compressed already:
// images, sound and video, archives, and the documents that are zip
// archives.
//
//nolint:gochecknoglobals // read only
var storedExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "avif": true, "heic": true,
	"mp3": true, "m4a": true, "aac": true, "ogg": true, "oga": true, "opus": true, "flac": true,
	"mp4": true, "m4v": true, "mov": true, "webm": true, "ogv": true, "mkv": true, "avi": true,
	"zip": true, "gz": true, "tgz": true, "bz2": true, "xz": true, "zst": true, "7z": true, "rar": true,
	"docx": true, "xlsx": true, "pptx": true, "odt": true, "ods": true, "odp": true, "epub": true, "jar": true,
}

// Stored reports whether a file named name goes into the archive as it is
// (zip's Store): its format, by its extension, is compressed already, and
// deflating it again would cost time for nothing (M7 design 4.10).
func Stored(name string) bool {
	return storedExtensions[strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))]
}
