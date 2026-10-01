package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"

// Settings are the parts of the configuration an instance reports.
type Settings struct {
	SignupEnabled            bool // auth.signup_enabled
	WorkspaceCreationEnabled bool // workspace.creation_enabled
}

// GetInfo tells API clients what this instance runs.
type GetInfo struct {
	source   InfoSource
	settings Settings
}

// NewGetInfo returns the use case, reading the build from source.
func NewGetInfo(source InfoSource, settings Settings) *GetInfo {
	return &GetInfo{source: source, settings: settings}
}

// Execute describes the instance. It does no I/O, so it takes no context and
// cannot fail.
func (uc *GetInfo) Execute() domain.Info {
	build := uc.source.Build()
	return domain.Info{
		Product:                  domain.Product,
		Version:                  build.Version,
		Commit:                   build.Commit,
		APIVersion:               domain.APIVersion,
		SignupEnabled:            uc.settings.SignupEnabled,
		WorkspaceCreationEnabled: uc.settings.WorkspaceCreationEnabled,
	}
}
