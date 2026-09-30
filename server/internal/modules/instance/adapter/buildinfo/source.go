// Package buildinfo implements the instance module's InfoSource port with the
// build metadata of the running binary.
package buildinfo

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"
	platformbuildinfo "github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
)

// Source reports the build of the running binary.
type Source struct{}

// Build implements app.InfoSource.
func (Source) Build() domain.Build {
	info := platformbuildinfo.Get()
	return domain.Build{Version: info.Version, Commit: info.Commit}
}
