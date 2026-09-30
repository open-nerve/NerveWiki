package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"

// GetInfo tells API clients what this instance runs.
type GetInfo struct {
	source        InfoSource
	signupEnabled bool
}

// NewGetInfo returns the use case, reading the build from source;
// signupEnabled is auth.signup_enabled.
func NewGetInfo(source InfoSource, signupEnabled bool) *GetInfo {
	return &GetInfo{source: source, signupEnabled: signupEnabled}
}

// Execute describes the instance. It does no I/O, so it takes no context and
// cannot fail.
func (uc *GetInfo) Execute() domain.Info {
	build := uc.source.Build()
	return domain.Info{
		Product:       domain.Product,
		Version:       build.Version,
		Commit:        build.Commit,
		APIVersion:    domain.APIVersion,
		SignupEnabled: uc.signupEnabled,
	}
}
