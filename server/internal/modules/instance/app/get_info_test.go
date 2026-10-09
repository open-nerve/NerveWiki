package app_test

import (
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"
)

type fixedSource domain.Build

func (s fixedSource) Build() domain.Build { return domain.Build(s) }

func TestGetInfoDescribesTheBuild(t *testing.T) {
	for _, settings := range []app.Settings{{SignupEnabled: true, AssetMaxBytes: 1 << 10, ExportTTL: 10 * time.Minute}, {WorkspaceCreationEnabled: true, AssetMaxBytes: 50 << 20, ExportTTL: 24 * time.Hour}} {
		uc := app.NewGetInfo(fixedSource{Version: "1.2.3", Commit: "4f2a9c1"}, settings)

		got := uc.Execute()

		want := domain.Info{Product: "Nerve Wiki", Version: "1.2.3", Commit: "4f2a9c1", APIVersion: "v0",
			SignupEnabled: settings.SignupEnabled, WorkspaceCreationEnabled: settings.WorkspaceCreationEnabled,
			AssetMaxBytes: settings.AssetMaxBytes, ExportTTL: settings.ExportTTL}
		if got != want {
			t.Errorf("Execute() = %+v, want %+v", got, want)
		}
	}
}
