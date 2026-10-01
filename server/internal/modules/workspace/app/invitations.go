package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"

// ListedInvitation is a pending invitation with the token of its link, as
// the workspace's admins see it.
type ListedInvitation struct {
	domain.Invitation
	Token string
}

// listed is inv with its token.
func listed(tokens InvitationTokens, inv domain.Invitation) ListedInvitation {
	return ListedInvitation{Invitation: inv, Token: tokens.Token(inv.ID)}
}
