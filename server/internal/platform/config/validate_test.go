package config

import (
	"net/netip"
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
		Auth: AuthConfig{
			AccessTokenTTL:         15 * time.Minute,
			SessionTTL:             720 * time.Hour,
			RefreshDeadline:        4 * time.Second,
			SessionCleanupInterval: time.Hour,
			JWT:                    JWTConfig{PrivateKeyFile: "/run/secrets/jwt-key.pem"},
			Password: PasswordConfig{
				Argon2MemoryKiB: 19456, Argon2Iterations: 2, Argon2Parallelism: 1,
				MaxConcurrentHashes: 4, MaxWait: 2 * time.Second,
			},
		},
		RateLimit: RateLimitConfig{
			IPv6PrefixLen: 64,
			Anonymous:     BucketConfig{PerMinute: 600, Burst: 100},
			AuthFailure:   BucketConfig{PerMinute: 60, Burst: 60},
			Authenticated: BucketConfig{PerMinute: 1200, Burst: 200},
			LoginIP:       BucketConfig{PerMinute: 30, Burst: 10},
			LoginIPEmail:  BucketConfig{PerMinute: 10, Burst: 5},
			RegisterIP:    BucketConfig{PerMinute: 10, Burst: 5},
			PasswordUser:  BucketConfig{PerMinute: 5, Burst: 5},
			AssetContent:  BucketConfig{PerMinute: 6000, Burst: 1000},
		},
		Workspace: WorkspaceConfig{CreationEnabled: true},
		Page:      PageConfig{EditSessionCleanupInterval: 10 * time.Minute, ParseBudgetBytes: 8 << 20, ParseMaxWait: 2 * time.Second},
		Events:    EventsConfig{HeartbeatInterval: 20 * time.Second},
		Jobs:      JobsConfig{ShutdownTimeout: 10 * time.Second, PurgeInterval: time.Hour, PurgeRetention: 1440 * time.Hour, ExportWorkers: 1, ImportWorkers: 1},
		Storage:   StorageConfig{Dir: "data", MinFreeBytes: 1 << 30},
		Asset:     AssetConfig{MaxBytes: 50 << 20, UploadMinRate: 64 << 10},
		Transfer: TransferConfig{ExportTTL: 24 * time.Hour, JobTimeout: 6 * time.Hour, HeartbeatTimeout: 5 * time.Minute, MaxQueued: 20,
			ImportMaxBytes: 512 << 20, ImportMaxEntries: 50000, ImportMaxUnpackedBytes: 4 << 30},
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
		Storage:  StorageConfig{MinFreeBytes: -1},
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
		"auth.access_token_ttl: must be positive, got 0s",
		"auth.session_ttl: must be positive, got 0s",
		"auth.session_cleanup_interval: must be at least 1s, got 0s",
		"auth.jwt.private_key_file: is required in prod: a PKCS#8 PEM Ed25519 private key, e.g. from openssl genpkey -algorithm ed25519",
		"auth.password.argon2_iterations: must be at least 1, got 0",
		"auth.password.argon2_parallelism: must be at least 1, got 0",
		"auth.password.argon2_memory_kib: must be at least 8 per lane (8), got 0",
		"auth.password.max_concurrent_hashes: must be at least 1, got 0",
		"auth.password.max_wait: must be positive, got 0s",
		"auth.refresh_deadline: must be positive, got 0s",
		"ratelimit.ipv6_prefix_len: must be from 1 to 128, got 0",
		"ratelimit.anonymous.per_minute: must be at least 1, got 0",
		"ratelimit.anonymous.burst: must be at least 1, got 0",
		"ratelimit.auth_failure.per_minute: must be at least 1, got 0",
		"ratelimit.auth_failure.burst: must be at least 1, got 0",
		"ratelimit.authenticated.per_minute: must be at least 1, got 0",
		"ratelimit.authenticated.burst: must be at least 1, got 0",
		"ratelimit.login_ip.per_minute: must be at least 1, got 0",
		"ratelimit.login_ip.burst: must be at least 1, got 0",
		"ratelimit.login_ip_email.per_minute: must be at least 1, got 0",
		"ratelimit.login_ip_email.burst: must be at least 1, got 0",
		"ratelimit.register_ip.per_minute: must be at least 1, got 0",
		"ratelimit.register_ip.burst: must be at least 1, got 0",
		"ratelimit.password_user.per_minute: must be at least 1, got 0",
		"ratelimit.password_user.burst: must be at least 1, got 0",
		"ratelimit.asset_content.per_minute: must be at least 1, got 0",
		"ratelimit.asset_content.burst: must be at least 1, got 0",
		"page.edit_session_cleanup_interval: must be at least 1s, got 0s",
		"page.parse_budget_bytes: must be at least 5242880, a page's largest content, got 0",
		"page.parse_max_wait: must be positive, got 0s",
		"events.heartbeat_interval: must be from 5s to 50s, got 0s",
		"jobs.shutdown_timeout: must be positive, got 0s",
		"jobs.purge_interval: must be at least 1s, got 0s",
		"jobs.purge_retention: must be at least 1h, got 0s",
		"jobs.export_workers: must be from 1 to 8, got 0",
		"jobs.import_workers: must be from 1 to 8, got 0",
		"storage.dir: is required",
		"storage.min_free_bytes: must not be negative, got -1",
		"asset.max_bytes: must be from 1024 (1 KiB) to 4294967296 (4 GiB), got 0",
		"asset.upload_min_rate: must be at least 1, got 0",
		"transfer.export_ttl: must be at least 10m0s, got 0s",
		"transfer.job_timeout: must be from 1m0s to 168h0m0s, got 0s",
		"transfer.heartbeat_timeout: must be at least 1m0s, got 0s",
		"transfer.max_queued: must be at least 1, got 0",
		"transfer.import_max_bytes: must be at least 1048576 (1 MiB), got 0",
		"transfer.import_max_entries: must be from 1 to 100000, got 0",
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
			name: "a request may take less than the write budget",
			mutate: func(c *Config) {
				c.Server.ReadHeaderTimeout, c.Server.ReadTimeout = time.Second, time.Second
				c.Server.RequestTimeout = c.Server.WriteTimeout - 2*time.Second
			},
		},
		{
			name:   "a content's body and its request within the write budget",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout - c.Server.ReadTimeout - time.Second },
		},
		{
			name:   "a content's body and its request beyond the write budget",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout - c.Server.ReadTimeout },
			want:   "server.request_timeout: plus server.read_timeout (30s) must be less than server.write_timeout (1m0s), got 30s",
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
		{
			name:   "a session must outlive its access tokens",
			mutate: func(c *Config) { c.Auth.SessionTTL = c.Auth.AccessTokenTTL },
			want:   "auth.session_ttl: must be longer than auth.access_token_ttl (15m0s), got 15m0s",
		},
		{
			name:   "argon2 needs 8 KiB per lane",
			mutate: func(c *Config) { c.Auth.Password.Argon2Parallelism, c.Auth.Password.Argon2MemoryKiB = 4, 31 },
			want:   "auth.password.argon2_memory_kib: must be at least 8 per lane (32), got 31",
		},
		{
			name:   "argon2 at 8 KiB per lane",
			mutate: func(c *Config) { c.Auth.Password.Argon2Parallelism, c.Auth.Password.Argon2MemoryKiB = 4, 32 },
		},
		{
			name:   "dev and test may sign with an ephemeral key",
			mutate: func(c *Config) { c.Env, c.Auth.JWT.PrivateKeyFile = EnvDev, "" },
		},
		{
			name: "a refresh ends before the web client gives up",
			mutate: func(c *Config) {
				c.Auth.RefreshDeadline, c.Database.CommitTimeout = 5*time.Second, 3*time.Second-time.Millisecond
			},
		},
		{
			name: "a refresh that the web client may give up on",
			mutate: func(c *Config) {
				c.Auth.RefreshDeadline, c.Database.CommitTimeout = 5*time.Second, 3*time.Second
			},
			want: "auth.refresh_deadline: plus database.commit_timeout (3s) must be less than 8s, the web client's refresh timeout, got 5s",
		},
		{
			name:   "a refresh deadline as long as the request timeout",
			mutate: func(c *Config) { c.Auth.RefreshDeadline, c.Server.RequestTimeout = 4*time.Second, 4*time.Second },
		},
		{
			name:   "a refresh deadline beyond the request timeout",
			mutate: func(c *Config) { c.Auth.RefreshDeadline, c.Server.RequestTimeout = 4*time.Second, 3*time.Second },
			want:   "auth.refresh_deadline: must be at most server.request_timeout (3s), got 4s",
		},
		{
			name:   "a hash waits less than its request may take",
			mutate: func(c *Config) { c.Auth.Password.MaxWait = c.Server.RequestTimeout - time.Millisecond },
		},
		{
			name:   "a hash that waits as long as its request may take",
			mutate: func(c *Config) { c.Auth.Password.MaxWait = c.Server.RequestTimeout },
			want:   "auth.password.max_wait: must be less than server.request_timeout (15s), got 15s",
		},
		{
			name:   "a parse waits less than its request may take",
			mutate: func(c *Config) { c.Page.ParseMaxWait = c.Server.RequestTimeout - time.Millisecond },
		},
		{
			name:   "a parse that waits as long as its request may take",
			mutate: func(c *Config) { c.Page.ParseMaxWait = c.Server.RequestTimeout },
			want:   "page.parse_max_wait: must be less than server.request_timeout (15s), got 15s",
		},
		{
			name:   "a heartbeat every 5s",
			mutate: func(c *Config) { c.Events.HeartbeatInterval = 5 * time.Second },
		},
		{
			name:   "a heartbeat every 50s",
			mutate: func(c *Config) { c.Events.HeartbeatInterval = 50 * time.Second },
		},
		{
			name:   "a heartbeat more often than every 5s",
			mutate: func(c *Config) { c.Events.HeartbeatInterval = 5*time.Second - time.Millisecond },
			want:   "events.heartbeat_interval: must be from 5s to 50s, got 4.999s",
		},
		{
			name:   "a heartbeat rarer than every 50s",
			mutate: func(c *Config) { c.Events.HeartbeatInterval = 50*time.Second + time.Millisecond },
			want:   "events.heartbeat_interval: must be from 5s to 50s, got 50.001s",
		},
		{
			name:   "an IPv6 prefix of a single address",
			mutate: func(c *Config) { c.RateLimit.IPv6PrefixLen = 128 },
		},
		{
			name:   "an IPv6 prefix longer than an address",
			mutate: func(c *Config) { c.RateLimit.IPv6PrefixLen = 129 },
			want:   "ratelimit.ipv6_prefix_len: must be from 1 to 128, got 129",
		},
		{
			name:   "trusted proxies",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("10.0.0.0/8", "fd00::/8", "192.0.2.7/32") },
		},
		{
			name:   "a proxy prefix that trusts everyone",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("10.0.0.0/8", "::/0") },
			want:   "server.trusted_proxies: ::/0 trusts every address, so any client could choose its own IP; list only your proxies' addresses",
		},
		{
			name:   "an IPv4-mapped proxy prefix",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("::ffff:10.0.0.0/104") },
			want:   "server.trusted_proxies: ::ffff:10.0.0.0/104 is an IPv4-mapped IPv6 prefix, which no address matches: write the IPv4 prefix",
		},
		{
			name:   "an edit sessions' cleanup every second",
			mutate: func(c *Config) { c.Page.EditSessionCleanupInterval = time.Second },
		},
		{
			name:   "an edit sessions' cleanup more often",
			mutate: func(c *Config) { c.Page.EditSessionCleanupInterval = time.Second - time.Millisecond },
			want:   "page.edit_session_cleanup_interval: must be at least 1s, got 999ms",
		},
		{
			name:   "a parse budget of one largest content",
			mutate: func(c *Config) { c.Page.ParseBudgetBytes = 5 << 20 },
		},
		{
			name:   "a parse budget smaller than the largest content",
			mutate: func(c *Config) { c.Page.ParseBudgetBytes = 5<<20 - 1 },
			want:   "page.parse_budget_bytes: must be at least 5242880, a page's largest content, got 5242879",
		},
		{
			name:   "an attachment of 1 KiB",
			mutate: func(c *Config) { c.Asset.MaxBytes = 1 << 10 },
		},
		{
			name:   "an attachment smaller than 1 KiB",
			mutate: func(c *Config) { c.Asset.MaxBytes = 1<<10 - 1 },
			want:   "asset.max_bytes: must be from 1024 (1 KiB) to 4294967296 (4 GiB), got 1023",
		},
		{
			name: "an attachment of 4 GiB, arriving within the hour, an import's archive of 4 GiB",
			mutate: func(c *Config) {
				c.Asset.MaxBytes, c.Asset.UploadMinRate, c.Transfer.ImportMaxBytes, c.Transfer.ImportMaxUnpackedBytes = 4<<30, 4<<30/3600+1, 4<<30, 4<<30
			},
		},
		{
			name: "an attachment larger than 4 GiB",
			mutate: func(c *Config) {
				c.Asset.MaxBytes, c.Asset.UploadMinRate, c.Transfer.ImportMaxBytes, c.Transfer.ImportMaxUnpackedBytes = 4<<30+1, 4<<30, 4<<30+1, 4<<30+1
			},
			want: "asset.max_bytes: must be from 1024 (1 KiB) to 4294967296 (4 GiB), got 4294967297",
		},
		{
			name: "an upload of the largest attachment in an hour",
			mutate: func(c *Config) {
				c.Asset.MaxBytes, c.Asset.UploadMinRate, c.Transfer.ImportMaxBytes = 3600<<10, 1<<10, 3*3600<<10
			},
		},
		{
			name: "an upload of the largest attachment in more than an hour",
			mutate: func(c *Config) {
				c.Asset.MaxBytes, c.Asset.UploadMinRate, c.Transfer.ImportMaxBytes = 3600<<10+1, 1<<10, 3*3600<<10
			},
			want: "asset.upload_min_rate: must let asset.max_bytes (3686401) arrive within 1h0m0s, at least 1025, got 1024",
		},
		{
			name:   "eight exports and eight imports at once",
			mutate: func(c *Config) { c.Jobs.ExportWorkers, c.Jobs.ImportWorkers, c.Database.MaxConns = 8, 8, 32 },
		},
		{
			name:   "exports and imports holding half the pool",
			mutate: func(c *Config) { c.Jobs.ExportWorkers, c.Jobs.ImportWorkers, c.Database.MaxConns = 2, 1, 6 },
		},
		{
			name:   "exports and imports holding more than half the pool",
			mutate: func(c *Config) { c.Jobs.ExportWorkers, c.Jobs.ImportWorkers, c.Database.MaxConns = 2, 2, 7 },
			want:   "jobs.import_workers: must leave, with jobs.export_workers (2), half of database.max_conns (7) to the requests, at most 1, got 2",
		},
		{
			name:   "more than eight imports at once",
			mutate: func(c *Config) { c.Jobs.ImportWorkers = 9 },
			want:   "jobs.import_workers: must be from 1 to 8, got 9",
		},
		{
			name: "an import of 1 MiB, attachments of 1 MiB",
			mutate: func(c *Config) {
				c.Transfer.ImportMaxBytes, c.Asset.MaxBytes, c.Transfer.ImportMaxUnpackedBytes = 1<<20, 1<<20, 1<<20
			},
		},
		{
			name:   "an import below 1 MiB",
			mutate: func(c *Config) { c.Transfer.ImportMaxBytes, c.Asset.MaxBytes = 1<<20-1, 1<<10 },
			want:   "transfer.import_max_bytes: must be at least 1048576 (1 MiB), got 1048575",
		},
		{
			name:   "an import smaller than an attachment",
			mutate: func(c *Config) { c.Transfer.ImportMaxBytes = c.Asset.MaxBytes - 1 },
			want:   "transfer.import_max_bytes: must be at least asset.max_bytes (52428800), got 52428799",
		},
		{
			name:   "an import's upload in three hours",
			mutate: func(c *Config) { c.Transfer.ImportMaxBytes = 65536 * 3 * 3600 },
		},
		{
			name:   "an import's upload in more than three hours",
			mutate: func(c *Config) { c.Transfer.ImportMaxBytes = 65536*3*3600 + 1 },
			want:   "transfer.import_max_bytes: must arrive within 3h0m0s at asset.upload_min_rate (65536), at most 707788800, got 707788801",
		},
		{
			name:   "a hundred thousand entries",
			mutate: func(c *Config) { c.Transfer.ImportMaxEntries = 100000 },
		},
		{
			name:   "more than a hundred thousand entries",
			mutate: func(c *Config) { c.Transfer.ImportMaxEntries = 100001 },
			want:   "transfer.import_max_entries: must be from 1 to 100000, got 100001",
		},
		{
			name:   "an import unpacking to less than it packs",
			mutate: func(c *Config) { c.Transfer.ImportMaxUnpackedBytes = c.Transfer.ImportMaxBytes - 1 },
			want:   "transfer.import_max_unpacked_bytes: must be at least transfer.import_max_bytes (536870912), got 536870911",
		},
		{
			name:   "exports holding more than half the pool",
			mutate: func(c *Config) { c.Jobs.ExportWorkers, c.Database.MaxConns = 3, 5 },
			want:   "jobs.export_workers: must leave half of database.max_conns (5) to the requests, at most 2, got 3",
		},
		{
			name:   "more than eight exports at once",
			mutate: func(c *Config) { c.Jobs.ExportWorkers = 9 },
			want:   "jobs.export_workers: must be from 1 to 8, got 9",
		},
		{
			name:   "an export kept ten minutes",
			mutate: func(c *Config) { c.Transfer.ExportTTL = 10 * time.Minute },
		},
		{
			name:   "an export kept less than ten minutes",
			mutate: func(c *Config) { c.Transfer.ExportTTL = 10*time.Minute - time.Second },
			want:   "transfer.export_ttl: must be at least 10m0s, got 9m59s",
		},
		{
			name:   "a job of two minutes, with the shortest heartbeat timeout below it",
			mutate: func(c *Config) { c.Transfer.JobTimeout, c.Transfer.HeartbeatTimeout = 2*time.Minute, time.Minute },
		},
		{
			name:   "a job of a week",
			mutate: func(c *Config) { c.Transfer.JobTimeout = 7 * 24 * time.Hour },
		},
		{
			name:   "a job of more than a week",
			mutate: func(c *Config) { c.Transfer.JobTimeout = 7*24*time.Hour + time.Second },
			want:   "transfer.job_timeout: must be from 1m0s to 168h0m0s, got 168h0m1s",
		},
		{
			name:   "a heartbeat timeout below a minute",
			mutate: func(c *Config) { c.Transfer.HeartbeatTimeout = time.Minute - time.Second },
			want:   "transfer.heartbeat_timeout: must be at least 1m0s, got 59s",
		},
		{
			name:   "a heartbeat timeout as long as the job",
			mutate: func(c *Config) { c.Transfer.HeartbeatTimeout = c.Transfer.JobTimeout },
			want:   "transfer.heartbeat_timeout: must be less than transfer.job_timeout (6h0m0s), got 6h0m0s",
		},
		{
			name:   "a single job queued",
			mutate: func(c *Config) { c.Transfer.MaxQueued = 1 },
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

func prefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}
