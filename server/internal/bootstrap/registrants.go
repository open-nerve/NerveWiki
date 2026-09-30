package bootstrap

import "github.com/open-nerve/NerveWiki/server/internal/modules/identity"

// deactivationRegistrants are the modules that take part in an account's
// deactivation: the vetoers that may refuse it and the subscribers that
// follow it (M1 design 8). serve and the command line both take them from
// here, so that a registrant a module adds reaches the self-service
// deactivation and the administrator's alike (M1/P4 design 3.6). M1 has
// none: the first ones come with the workspaces (M2).
func deactivationRegistrants() ([]identity.DeactivationVetoer, []identity.DeactivationSubscriber) {
	return nil, nil
}
