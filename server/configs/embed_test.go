package configs_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/configs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

func TestBuiltInProfiles(t *testing.T) {
	const devURL = "postgres://nervewiki:nervewiki@localhost:55433/nervewiki?sslmode=disable"
	tests := []struct {
		env         string
		addr        string // expected server.addr
		url         string // expected database.url
		autoMigrate bool
		level       string
		format      string
	}{
		{env: "dev", addr: "127.0.0.1:8080", url: devURL, autoMigrate: true, level: "debug", format: "text"},
		{env: "test", addr: ":8080", url: "postgres://from-env", autoMigrate: true, level: "warn", format: "text"},
		{env: "prod", addr: ":8080", url: "postgres://from-env", autoMigrate: false, level: "info", format: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			environ := []string{"NWIKI_ENV=" + tt.env}
			if tt.env != "dev" {
				environ = append(environ, "NWIKI_DATABASE__URL=postgres://from-env")
			}
			cfg, err := config.Load(config.Sources{Embedded: configs.FS(), Environ: environ})
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			want := config.Config{
				Env: tt.env,
				Server: config.ServerConfig{
					Addr:              tt.addr,
					ReadHeaderTimeout: 5 * time.Second,
					ReadTimeout:       30 * time.Second,
					WriteTimeout:      60 * time.Second,
					ShutdownTimeout:   20 * time.Second,
				},
				Database: config.DatabaseConfig{URL: tt.url, MaxConns: 10, AutoMigrate: tt.autoMigrate, CommitTimeout: 2 * time.Second},
				Log:      config.LogConfig{Level: tt.level, Format: tt.format},
			}
			if !reflect.DeepEqual(cfg, want) {
				t.Errorf("Load() =\n%+v\nwant\n%+v", cfg, want)
			}
		})
	}
}

func TestProdRequiresDatabaseURL(t *testing.T) {
	_, err := config.Load(config.Sources{Embedded: configs.FS(), Environ: []string{"NWIKI_ENV=prod"}})
	if err == nil || !strings.Contains(err.Error(), "database.url: is required") {
		t.Fatalf("Load() error = %v, want database.url to be required", err)
	}
}
