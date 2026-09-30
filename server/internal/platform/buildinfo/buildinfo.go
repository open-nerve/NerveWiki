// Package buildinfo reports which build of nervewiki is running.
package buildinfo

import "runtime/debug"

// version is the product version. Release builds override it with
//
//	-ldflags "-X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=<version>"
var version = "0.1.0-dev"

const unknown = "unknown"

// Info describes the running binary.
type Info struct {
	Version    string // product version, e.g. "0.1.0"
	Commit     string // git revision the binary was built from
	CommitTime string // time of that revision, RFC 3339
	Modified   bool   // built from a working tree with uncommitted changes
}

// Get returns the build metadata of the running binary. Commit details come
// from the VCS stamp the Go toolchain embeds when building inside a git
// checkout; they are "unknown" for test binaries and `go run`.
func Get() Info {
	var settings []debug.BuildSetting
	if bi, ok := debug.ReadBuildInfo(); ok {
		settings = bi.Settings
	}
	return fromSettings(version, settings)
}

func fromSettings(productVersion string, settings []debug.BuildSetting) Info {
	info := Info{Version: productVersion, Commit: unknown, CommitTime: unknown}
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.time":
			info.CommitTime = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}
