package domain

import (
	"strings"
	"time"
	"uuid"
)

// Blob is an attachment's file: its type, as the server determined it,
// its size and SHA-256, an image's size in pixels when known (0 when not),
// and once it is attached, its node, notebook, uploader and time.
type Blob struct {
	ID         uuid.UUID
	NodeID     uuid.UUID
	NotebookID uuid.UUID
	MIME       string
	Bytes      int64
	SHA256     []byte
	Width      int
	Height     int
	CreatedBy  uuid.UUID
	CreatedAt  time.Time
}

// Area is the store's area of the attachments' files.
const Area = "blobs"

// Key is the store's key of the file of the blob id.
func Key(id uuid.UUID) string {
	return Area + "/" + id.String()
}

// IDOf is the blob whose file is at key, as Key writes it; false for a
// key Key does not write.
func IDOf(key string) (uuid.UUID, bool) {
	name, ok := strings.CutPrefix(key, Area+"/")
	if !ok {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(name)
	if err != nil || id.String() != name {
		return uuid.UUID{}, false
	}
	return id, true
}
