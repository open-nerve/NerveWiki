package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Env: EnvTest,
		Server: ServerConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			ShutdownTimeout:   20 * time.Second,
			RequestTimeout:    15 * time.Second,
			MaxBodyBytes:      1 << 20,
		},
		Database: DatabaseConfig{
			URL:           "postgres://nervewiki:secret@localhost:5432/nervewiki",
			MaxConns:      10,
			AutoMigrate:   true,
			CommitTimeout: 2 * time.Second,
		},
		Log: LogConfig{Level: "info", Format: "json"},
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

func TestValidateReportsEveryInvalidKey(t *testing.T) {
	cfg := Config{
		Env:      EnvProd,
		Server:   ServerConfig{Addr: "8080", ReadHeaderTimeout: 0, ReadTimeout: 0, WriteTimeout: -time.Second, ShutdownTimeout: -time.Second},
		Database: DatabaseConfig{URL: "", MaxConns: 0},
		Log:      LogConfig{Level: "verbose", Format: "xml"},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("validate() = nil, want errors")
	}
	want := []string{
		`server.addr: must be host:port, e.g. ":8080", got "8080"`,
		"server.read_header_timeout: must be positive, got 0s",
		"server.read_timeout: must be positive, got 0s",
		"server.write_timeout: must be positive, got -1s",
		"server.shutdown_timeout: must be positive, got -1s",
		"server.request_timeout: must be positive, got 0s",
		"server.max_body_bytes: must be at least 1, got 0",
		"database.url: is required",
		"database.max_conns: must be at least 1, got 0",
		"database.commit_timeout: must be positive, got 0s",
		`log.level: must be one of debug, info, warn, error, got "verbose"`,
		`log.format: must be text or json, got "xml"`,
	}
	if got := strings.Split(err.Error(), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("validate() =\n%s\nwant\n%s", err, strings.Join(want, "\n"))
	}
}

func TestValidateCrossKeyRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string // "" means valid
	}{
		{
			name:   "headers may take the whole read budget",
			mutate: func(c *Config) { c.Server.ReadHeaderTimeout = c.Server.ReadTimeout },
		},
		{
			name:   "headers may not outlast the whole request",
			mutate: func(c *Config) { c.Server.ReadHeaderTimeout = c.Server.ReadTimeout + time.Second },
			want:   "server.read_header_timeout: must not exceed server.read_timeout (30s), got 31s",
		},
		{
			name:   "a request may take less than the write budget",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout - time.Second },
		},
		{
			name:   "a request must leave time to write its answer",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout },
			want:   "server.request_timeout: must be less than server.write_timeout (1m0s), got 1m0s",
		},
		{
			name:   "a host is optional in the address",
			mutate: func(c *Config) { c.Server.Addr = "127.0.0.1:0" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			err := cfg.validate()
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("validate() = %v, want nil", err)
			case tt.want != "" && (err == nil || err.Error() != tt.want):
				t.Errorf("validate() = %v, want %q", err, tt.want)
			}
		})
	}
}
