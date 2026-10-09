package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
)

// displayNames is identity's Directory as the page and transfer modules
// read it: the display names of a lock's holder and of who released one
// (M5 design 4.5), of who started a job (M7/P5 design 3.6).
type displayNames struct {
	identity.Directory
}

// DisplayNames are the accounts' display names, by id.
func (d displayNames) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	got, err := d.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(got))
	for id, p := range got {
		out[id] = p.DisplayName
	}
	return out, nil
}
