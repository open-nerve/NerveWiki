package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"

// GetInfo tells API clients what this instance runs.
type GetInfo struct {
	source InfoSource
}

// NewGetInfo returns the use case, reading the build from source.
func NewGetInfo(source InfoSource) *GetInfo {
	return &GetInfo{source: source}
}

// Execute describes the instance. It does no I/O, so it takes no context and
// cannot fail.
func (uc *GetInfo) Execute() domain.Info {
	build := uc.source.Build()
	return domain.Info{
		Product:    domain.Product,
		Version:    build.Version,
		Commit:     build.Commit,
		APIVersion: domain.APIVersion,
	}
}
