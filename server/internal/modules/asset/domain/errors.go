package domain

import (
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotFound is an attachment that does not exist, is deleted, or whose
// notebook the caller has no role in.
var ErrNotFound = shared.NewError(shared.KindNotFound, "asset.not_found", "No such attachment.")

// ErrStorageFull is a file the store has no room for: 507 storage_full.
var ErrStorageFull = shared.StorageFull()

// ErrTooLarge is a file larger than the largest attachment
// (asset.max_bytes): the HTTP adapter answers it 413 payload_too_large, the
// platform's answer to a body over its limit.
var ErrTooLarge = errors.New("asset: the file is larger than the largest attachment")
