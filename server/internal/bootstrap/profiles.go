package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// memberProfiles is identity's Profiles as the workspace module's member
// list reads them: the two modules do not import each other, so their
// types, alike field by field, meet here.
type memberProfiles struct {
	profiles identity.Profiles
}

func (m memberProfiles) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]workspace.Profile, error) {
	got, err := m.profiles.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]workspace.Profile, len(got))
	for id, p := range got {
		out[id] = workspace.Profile(p)
	}
	return out, nil
}
