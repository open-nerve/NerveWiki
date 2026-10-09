package app

import (
	"sync"
	"uuid"
)

// Uploads are the imports' archives being uploaded, by notebook, from
// StartImport's Check to its Release, in this process: v0.1 runs one
// (M7/P6 design 3.9). A notebook takes one at a time. Until its job's row
// is written, each counts as a job in the queue and its declared bytes as
// written in the store, for the imports and the exports alike, so that an
// upload that will be refused is refused before it is sent.
type Uploads struct {
	mu        sync.Mutex
	notebooks map[uuid.UUID]*upload
}

// upload is an upload under way: its declared bytes, and whether its
// job's row was written, which counts for it from then.
type upload struct {
	bytes   int64
	written bool
}

// NewUploads returns none under way.
func NewUploads() *Uploads {
	return &Uploads{notebooks: map[uuid.UUID]*upload{}}
}

// claim claims the notebook id for an upload of bytes: false when one
// holds it.
func (u *Uploads) claim(id uuid.UUID, bytes int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.notebooks[id] != nil {
		return false
	}
	u.notebooks[id] = &upload{bytes: bytes}
	return true
}

// release ends the notebook id's upload.
func (u *Uploads) release(id uuid.UUID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.notebooks, id)
}

// rowWritten tells that the notebook id's upload has its job's row, which
// counts for it from then.
func (u *Uploads) rowWritten(id uuid.UUID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if up := u.notebooks[id]; up != nil {
		up.written = true
	}
}

// others are the uploads that count into notebooks but id: how many, and
// their declared bytes.
func (u *Uploads) others(id uuid.UUID) (int, int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	n, bytes := 0, int64(0)
	for notebook, up := range u.notebooks {
		if notebook != id && !up.written {
			n, bytes = n+1, bytes+up.bytes
		}
	}
	return n, bytes
}
