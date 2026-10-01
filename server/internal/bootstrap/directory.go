package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// directory is identity's Directory as the workspace module reads it: the
// two modules do not import each other, so their types, alike field by
// field, meet here.
type directory struct {
	identity.Directory
}

// MemberProfiles is the member list's read of the accounts' profiles.
func (d directory) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]workspace.Profile, error) {
	got, err := d.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]workspace.Profile, len(got))
	for id, p := range got {
		out[id] = workspace.Profile(p)
	}
	return out, nil
}
