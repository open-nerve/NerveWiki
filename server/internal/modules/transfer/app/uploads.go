package app

import (
	"sync"
	"uuid"
)

// Uploads are the imports' archives being uploaded, by notebook, from
// StartImport's Check to its Release, in this process: v0.1 runs one
// (M7/P6 design 3.9). A notebook takes one at a time. Once its Check
// admitted it, and until its job's row is written, each counts as a job
// in the queue and the bytes it declared but has not stored yet as
// written in the store, for the imports and the exports alike, so that an
// upload that will be refused is refused before it is sent. The Checks
// decide one at a time, so that two together do not refuse each other.
type Uploads struct {
	checking  sync.Mutex
	mu        sync.Mutex
	notebooks map[uuid.UUID]*upload
}

// upload is an upload under way: its declared bytes and those stored so
// far, whether its Check admitted it, and whether its job's row was
// written, which counts for it from then.
type upload struct {
	bytes, stored int64
	admitted      bool
	written       bool
}

// NewUploads returns none under way.
func NewUploads() *Uploads {
	return &Uploads{notebooks: map[uuid.UUID]*upload{}}
}

// claim claims the notebook id for an upload of bytes, not admitted yet:
// false when one holds it.
func (u *Uploads) claim(id uuid.UUID, bytes int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.notebooks[id] != nil {
		return false
	}
	u.notebooks[id] = &upload{bytes: bytes}
	return true
}

// admit tells that the notebook id's Check admitted its upload.
func (u *Uploads) admit(id uuid.UUID) {
	u.with(id, func(up *upload) { up.admitted = true })
}

// store adds n bytes to those the notebook id's upload stored.
func (u *Uploads) store(id uuid.UUID, n int64) {
	u.with(id, func(up *upload) { up.stored += n })
}

// rowWritten tells that the notebook id's upload has its job's row, which
// counts for it from then.
func (u *Uploads) rowWritten(id uuid.UUID) {
	u.with(id, func(up *upload) { up.written = true })
}

// with runs do on the notebook id's upload, when there is one.
func (u *Uploads) with(id uuid.UUID, do func(up *upload)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if up := u.notebooks[id]; up != nil {
		do(up)
	}
}

// release ends the notebook id's upload.
func (u *Uploads) release(id uuid.UUID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.notebooks, id)
}

// others are the uploads that count into notebooks but id: how many, and
// the bytes they declared and have not stored yet.
func (u *Uploads) others(id uuid.UUID) (int, int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	n, bytes := 0, int64(0)
	for notebook, up := range u.notebooks {
		if notebook != id && up.admitted && !up.written {
			n, bytes = n+1, bytes+max(up.bytes-up.stored, 0)
		}
	}
	return n, bytes
}
