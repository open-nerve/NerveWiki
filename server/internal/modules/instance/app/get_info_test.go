package app_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"
)

type fixedSource domain.Build

func (s fixedSource) Build() domain.Build { return domain.Build(s) }

func TestGetInfoDescribesTheBuild(t *testing.T) {
	uc := app.NewGetInfo(fixedSource{Version: "1.2.3", Commit: "4f2a9c1"})

	got := uc.Execute()

	want := domain.Info{Product: "Nerve Wiki", Version: "1.2.3", Commit: "4f2a9c1", APIVersion: "v0"}
	if got != want {
		t.Errorf("Execute() = %+v, want %+v", got, want)
	}
}
