package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
)

// notebookProfiles is identity's Directory as the notebook module reads it:
// the two modules do not import each other, so their types, alike field by
// field, meet here.
type notebookProfiles struct {
	identity.Directory
}

// MemberProfiles is the member list's read of the accounts' profiles.
func (d notebookProfiles) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]notebook.Profile, error) {
	got, err := d.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]notebook.Profile, len(got))
	for id, p := range got {
		out[id] = notebook.Profile(p)
	}
	return out, nil
}
