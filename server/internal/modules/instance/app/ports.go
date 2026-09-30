// Package app holds the instance module's use cases and the ports they need.
package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"

// InfoSource reports the build this instance runs. GetInfo declares it;
// adapter/buildinfo implements it.
type InfoSource interface {
	Build() domain.Build
}
