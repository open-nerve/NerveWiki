package buildinfo_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/buildinfo"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"
	platformbuildinfo "github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
)

var _ app.InfoSource = buildinfo.Source{}

func TestSourceReportsTheRunningBuild(t *testing.T) {
	info := platformbuildinfo.Get()

	got := buildinfo.Source{}.Build()

	want := domain.Build{Version: info.Version, Commit: info.Commit}
	if got != want {
		t.Errorf("Build() = %+v, want %+v", got, want)
	}
}
