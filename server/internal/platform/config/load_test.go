package config

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const testBase = `
server:
  addr: ":8080"
  read_header_timeout: 5s
  read_timeout: 30s
  write_timeout: 60s
  shutdown_timeout: 20s
  addr_file: ""
database:
  url: ""
  max_conns: 10
  auto_migrate: true
  commit_timeout: 2s
log:
  level: info
  format: json
`

func embedded(profiles map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{"config.yaml": {Data: []byte(testBase)}}
	for name, content := range profiles {
		fsys["config."+name+".yaml"] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAppliesLayersInOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "log:\n  format: text\ndatabase:\n  max_conns: 20\n")
	writeFile(t, dir, "config.dev.yaml", "database:\n  max_conns: 30\nserver:\n  shutdown_timeout: 30s\n")
	local := writeFile(t, t.TempDir(), "config.local.yaml", "server:\n  shutdown_timeout: 40s\n  read_header_timeout: 7s\n")

	cfg, err := Load(Sources{
		Embedded: embedded(map[string]string{"dev": "database:\n  url: postgres://embedded-dev\nlog:\n  level: debug\n"}),
		Environ: []string{
			"NWIKI_CONFIG_DIR=" + dir,
			"NWIKI_SERVER__READ_HEADER_TIMEOUT=9s",
			"NWIKI_DATABASE__AUTO_MIGRATE=false",
			"NWIKI_DATABASE__COMMIT_TIMEOUT=3s",
		},
		LocalFile: local,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Env: EnvDev, // NWIKI_ENV defaults to dev
		Server: ServerConfig{
			Addr:              ":8080",         // built-in config.yaml
			ReadHeaderTimeout: 9 * time.Second, // environment beats config.local.yaml
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			ShutdownTimeout:   40 * time.Second, // config.local.yaml beats the config dir
		},
		Database: DatabaseConfig{
			URL:           "postgres://embedded-dev", // built-in config.dev.yaml
			MaxConns:      30,                        // config dir: config.dev.yaml beats config.yaml
			AutoMigrate:   false,                     // environment
			CommitTimeout: 3 * time.Second,           // environment
		},
		Log: LogConfig{Level: "debug", Format: "text"},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() =\n%+v\nwant\n%+v", cfg, want)
	}
}

func TestLoadSelectsProfile(t *testing.T) {
	local := writeFile(t, t.TempDir(), "config.local.yaml", "log:\n  level: error\n")
	cfg, err := Load(Sources{
		Embedded: embedded(map[string]string{"test": "log:\n  level: warn\n"}),
		Environ:  []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://from-env"},
		// Only the dev profile reads the local file.
		LocalFile: local,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Env != EnvTest || cfg.Log.Level != "warn" || cfg.Database.URL != "postgres://from-env" {
		t.Errorf("Load() = %+v, want test profile, level warn and the URL from the environment", cfg)
	}
}

func TestLoadIgnoresMissingOptionalFiles(t *testing.T) {
	_, err := Load(Sources{
		Embedded:  embedded(map[string]string{"dev": "database:\n  url: postgres://embedded-dev\n"}),
		Environ:   []string{"NWIKI_CONFIG_DIR=" + t.TempDir()},
		LocalFile: filepath.Join(t.TempDir(), "config.local.yaml"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

// Variables without the "__" separator steer loading or belong to other
// tools, such as the compose file's NWIKI_DEV_DB_PORT: they must not be read
// as keys, or the unknown-key check would refuse to start.
func TestLoadSkipsVariablesThatAreNotKeys(t *testing.T) {
	cfg, err := Load(Sources{
		Embedded: embedded(map[string]string{"prod": ""}),
		Environ: []string{
			"NWIKI_ENV=prod",
			"NWIKI_CONFIG_DIR=" + t.TempDir(),
			"NWIKI_DEV_DB_PORT=55434",
			"NWIKI_DATABASE__URL=postgres://from-env",
			"OTHER__VAR=ignored",
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.URL != "postgres://from-env" {
		t.Errorf("database.url = %q, want the value of NWIKI_DATABASE__URL", cfg.Database.URL)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		dirFile string // content of config.yaml in NWIKI_CONFIG_DIR, if any
		want    string
	}{
		{
			name:    "unknown profile",
			environ: []string{"NWIKI_ENV=staging"},
			want:    `NWIKI_ENV must be one of dev, test, prod, got "staging"`,
		},
		{
			name:    "config dir does not exist",
			environ: []string{"NWIKI_ENV=test", "NWIKI_CONFIG_DIR=/does/not/exist"},
			want:    "NWIKI_CONFIG_DIR=/does/not/exist is not a directory",
		},
		{
			name:    "required key missing",
			environ: []string{"NWIKI_ENV=test"},
			want:    "invalid configuration:\ndatabase.url: is required",
		},
		{
			name:    "unknown key in the environment",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_DATABSE__URL=postgres://x"},
			want:    "has invalid keys: databse",
		},
		{
			name:    "unknown key in a file",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "database:\n  max_con: 5\n",
			want:    "'database' has invalid keys: max_con",
		},
		{
			name:    "malformed duration",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_SERVER__SHUTDOWN_TIMEOUT=soon"},
			want:    `'server.shutdown_timeout' time: invalid duration "soon"`,
		},
		{
			name:    "number where a duration is expected",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "server:\n  shutdown_timeout: 20\n",
			want:    `'server.shutdown_timeout' must be a duration such as "5s", got 20`,
		},
		{
			name:    "malformed number",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_DATABASE__MAX_CONNS=many"},
			want:    "'database.max_conns'",
		},
		{
			name:    "malformed YAML",
			environ: []string{"NWIKI_ENV=test"},
			dirFile: "server: [",
			want:    "config.yaml: yaml:",
		},
		// Weakly typed decoding would read an empty value as false or 0.
		{
			name:    "empty boolean in the environment",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_DATABASE__AUTO_MIGRATE="},
			want:    "'database.auto_migrate' must not be empty",
		},
		{
			name:    "empty number in the environment",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_DATABASE__MAX_CONNS="},
			want:    "'database.max_conns' must not be empty",
		},
		{
			name:    "empty duration in the environment",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x", "NWIKI_DATABASE__COMMIT_TIMEOUT="},
			want:    "'database.commit_timeout' must not be empty",
		},
		// A YAML key without a value would leave its key at the zero value,
		// whatever the layers below it say.
		{
			name:    "boolean without a value in YAML",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "database:\n  auto_migrate:\n",
			want:    "invalid configuration:\ndatabase.auto_migrate: must not be null (a key without a value in YAML)",
		},
		{
			name:    "section without a value in YAML",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "log:\n",
			want:    "invalid configuration:\nlog: must not be null (a key without a value in YAML)",
		},
		// A list is one value to koanf: every null inside it is reported by
		// its path, even where no list key exists yet.
		{
			name:    "nulls nested in a list in YAML",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "server:\n  addr: [[a, ~], {via: ~}]\n",
			want: "invalid configuration:\n" +
				"server.addr[0][1]: must not be null (a key without a value in YAML)\n" +
				"server.addr[1].via: must not be null (a key without a value in YAML)",
		},
		// Weakly typed decoding would cut a number its type cannot hold.
		{
			name:    "number too large for its key",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "database:\n  max_conns: 5000000000\n",
			want:    "'database.max_conns' must be a whole number from -2147483648 to 2147483647, got 5000000000",
		},
		{
			name:    "fraction for a whole number",
			environ: []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://x"},
			dirFile: "database:\n  max_conns: 1.5\n",
			want:    "'database.max_conns' must be a whole number from -2147483648 to 2147483647, got 1.5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environ := tt.environ
			if tt.dirFile != "" {
				dir := t.TempDir()
				writeFile(t, dir, "config.yaml", tt.dirFile)
				environ = append(environ, "NWIKI_CONFIG_DIR="+dir)
			}
			_, err := Load(Sources{Embedded: embedded(map[string]string{"test": ""}), Environ: environ})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load() error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// numberHook guards every integer type, including those no M0 key uses yet;
// it is tested directly rather than through keys.
func TestNumberHook(t *testing.T) {
	tests := []struct {
		name string
		to   reflect.Type
		data any
		ok   bool
	}{
		{"int8 at its maximum", reflect.TypeFor[int8](), 127, true},
		{"int8 past its maximum", reflect.TypeFor[int8](), 128, false},
		{"int8 at its minimum", reflect.TypeFor[int8](), -128, true},
		{"uint8 negative", reflect.TypeFor[uint8](), -1, false},
		{"uint8 at its maximum as a float", reflect.TypeFor[uint8](), 255.0, true},
		{"uint8 past its maximum as a float", reflect.TypeFor[uint8](), 256.0, false},
		{"uint32 negative", reflect.TypeFor[uint32](), -1, false},
		{"uint64 from a large unsigned", reflect.TypeFor[uint64](), uint64(math.MaxUint64), true},
		// float64 rounds the int64 maximum up to 2^63, the first number past it.
		{"int64 past its maximum as a float", reflect.TypeFor[int64](), 9223372036854775808.0, false},
		{"int fraction", reflect.TypeFor[int](), 1.5, false},
		{"not a number: left to the decoder", reflect.TypeFor[int](), "12", true},
		{"not an integer key: left alone", reflect.TypeFor[string](), 1.5, true},
		{"a duration: decoded by durationHook", reflect.TypeFor[time.Duration](), 5 * time.Second, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := numberHook(nil, tt.to, tt.data)
			if (err == nil) != tt.ok {
				t.Errorf("numberHook(%v, %v) error = %v, want ok %t", tt.to, tt.data, err, tt.ok)
			}
		})
	}
}

// emptyValueHook covers every non-string scalar kind; the keys only reach
// some of them, so it is tested directly too.
func TestEmptyValueHook(t *testing.T) {
	tests := []struct {
		name string
		to   reflect.Type
		data any
		ok   bool
	}{
		{"empty for a uint", reflect.TypeFor[uint16](), "", false},
		{"empty for a float", reflect.TypeFor[float64](), "", false},
		{"empty for a bool", reflect.TypeFor[bool](), "", false},
		{"empty for a string", reflect.TypeFor[string](), "", true},
		{"a value for a uint", reflect.TypeFor[uint16](), "7", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := emptyValueHook(reflect.TypeOf(tt.data), tt.to, tt.data)
			if (err == nil) != tt.ok {
				t.Errorf("emptyValueHook(%v, %q) error = %v, want ok %t", tt.to, tt.data, err, tt.ok)
			}
		})
	}
}
