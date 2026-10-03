package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
)

// pageNames is identity's Directory as the page module reads it: the
// display names of a lock's holder and of who released one (M5 design
// 4.5).
type pageNames struct {
	identity.Directory
}

// DisplayNames are the accounts' display names, by id.
func (d pageNames) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
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
