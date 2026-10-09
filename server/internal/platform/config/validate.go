package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// webRefreshTimeout is how long the web client waits for a refresh (M1/P2
// design 3.5): REQUEST_TIMEOUT_MS in
// web/apps/web/src/session/token-manager.ts; change both together. The
// server must have finished a refresh before that: committed, rolled back,
// or given up on its COMMIT.
const webRefreshTimeout = 8 * time.Second

// validate reports every invalid key at once, one "key: problem" line each.
func (c Config) validate() error {
	var errs []error
	fail := func(key, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: "+format, append([]any{key}, args...)...))
	}

	if _, _, err := net.SplitHostPort(c.Server.Addr); err != nil {
		fail("server.addr", "must be host:port, e.g. \":8080\", got %q", c.Server.Addr)
	}
	switch {
	case c.Server.ReadHeaderTimeout <= 0:
		fail("server.read_header_timeout", "must be positive, got %s", c.Server.ReadHeaderTimeout)
	case c.Server.ReadTimeout > 0 && c.Server.ReadHeaderTimeout > c.Server.ReadTimeout:
		// read_timeout is the budget for the whole request: a longer header limit
		// would let the headers alone outlast it and leave the body no time.
		fail("server.read_header_timeout", "must not exceed server.read_timeout (%s), got %s", c.Server.ReadTimeout, c.Server.ReadHeaderTimeout)
	}
	if c.Server.ReadTimeout <= 0 {
		fail("server.read_timeout", "must be positive, got %s", c.Server.ReadTimeout)
	}
	if c.Server.WriteTimeout <= 0 {
		fail("server.write_timeout", "must be positive, got %s", c.Server.WriteTimeout)
	}
	if c.Server.ShutdownTimeout <= 0 {
		fail("server.shutdown_timeout", "must be positive, got %s", c.Server.ShutdownTimeout)
	}
	switch {
	case c.Server.RequestTimeout <= 0:
		fail("server.request_timeout", "must be positive, got %s", c.Server.RequestTimeout)
	case c.Server.WriteTimeout > 0 && c.Server.RequestTimeout >= c.Server.WriteTimeout:
		// A request that runs into its deadline still has to write its error
		// response before write_timeout cuts the connection.
		fail("server.request_timeout", "must be less than server.write_timeout (%s), got %s", c.Server.WriteTimeout, c.Server.RequestTimeout)
	case c.Server.ReadTimeout > 0 && c.Server.WriteTimeout > 0 && c.Server.ReadTimeout+c.Server.RequestTimeout >= c.Server.WriteTimeout:
		// The routes of a page's content give their body read_timeout to
		// arrive on top of the request's deadline (M4/P4 review P3).
		fail("server.request_timeout", "plus server.read_timeout (%s) must be less than server.write_timeout (%s), got %s",
			c.Server.ReadTimeout, c.Server.WriteTimeout, c.Server.RequestTimeout)
	}
	if c.Server.MaxBodyBytes < 1 {
		fail("server.max_body_bytes", "must be at least 1, got %d", c.Server.MaxBodyBytes)
	}
	for _, p := range c.Server.TrustedProxies {
		switch {
		case p.Bits() == 0:
			// Every client would be a trusted proxy and could write its own
			// address into X-Forwarded-For.
			fail("server.trusted_proxies", "%s trusts every address, so any client could choose its own IP; list only your proxies' addresses", p)
		case p.Addr().Is4In6():
			// Client addresses are compared unmapped, so such a prefix would
			// silently match nothing.
			fail("server.trusted_proxies", "%s is an IPv4-mapped IPv6 prefix, which no address matches: write the IPv4 prefix", p)
		}
	}
	if c.Database.URL == "" {
		fail("database.url", "is required")
	}
	if c.Database.MaxConns < 1 {
		fail("database.max_conns", "must be at least 1, got %d", c.Database.MaxConns)
	}
	if c.Database.CommitTimeout <= 0 {
		fail("database.commit_timeout", "must be positive, got %s", c.Database.CommitTimeout)
	}
	c.Auth.validate(c.Env, fail)
	switch {
	case c.Auth.RefreshDeadline <= 0:
		fail("auth.refresh_deadline", "must be positive, got %s", c.Auth.RefreshDeadline)
	case c.Auth.RefreshDeadline+c.Database.CommitTimeout >= webRefreshTimeout:
		fail("auth.refresh_deadline", "plus database.commit_timeout (%s) must be less than %s, the web client's refresh timeout, got %s",
			c.Database.CommitTimeout, webRefreshTimeout, c.Auth.RefreshDeadline)
	case c.Server.RequestTimeout > 0 && c.Auth.RefreshDeadline > c.Server.RequestTimeout:
		// It is the request deadline of refresh and logout: a shorter one.
		fail("auth.refresh_deadline", "must be at most server.request_timeout (%s), got %s", c.Server.RequestTimeout, c.Auth.RefreshDeadline)
	}
	if wait := c.Auth.Password.MaxWait; wait > 0 && c.Server.RequestTimeout > 0 && wait >= c.Server.RequestTimeout {
		// As page.parse_max_wait: a wait for a hash as long as the request's
		// deadline would end in a 500, not the 503 server_busy.
		fail("auth.password.max_wait", "must be less than server.request_timeout (%s), got %s", c.Server.RequestTimeout, wait)
	}
	c.RateLimit.validate(fail)
	if c.Page.EditSessionCleanupInterval < time.Second {
		fail("page.edit_session_cleanup_interval", "must be at least 1s, got %s", c.Page.EditSessionCleanupInterval)
	}
	if c.Page.ParseBudgetBytes < MinParseBudgetBytes {
		fail("page.parse_budget_bytes", "must be at least %d, a page's largest content, got %d", MinParseBudgetBytes, c.Page.ParseBudgetBytes)
	}
	switch {
	case c.Page.ParseMaxWait <= 0:
		fail("page.parse_max_wait", "must be positive, got %s", c.Page.ParseMaxWait)
	case c.Server.RequestTimeout > 0 && c.Page.ParseMaxWait >= c.Server.RequestTimeout:
		// A wait as long as the request's deadline would end in a 500, not
		// the 503 server_busy that tells the client to come back.
		fail("page.parse_max_wait", "must be less than server.request_timeout (%s), got %s", c.Server.RequestTimeout, c.Page.ParseMaxWait)
	}
	if d := c.Events.HeartbeatInterval; d < MinHeartbeatInterval || d > MaxHeartbeatInterval {
		fail("events.heartbeat_interval", "must be from %s to %s, got %s", MinHeartbeatInterval, MaxHeartbeatInterval, d)
	}
	if c.Jobs.ShutdownTimeout <= 0 {
		fail("jobs.shutdown_timeout", "must be positive, got %s", c.Jobs.ShutdownTimeout)
	}
	if c.Jobs.PurgeInterval < time.Second {
		fail("jobs.purge_interval", "must be at least 1s, got %s", c.Jobs.PurgeInterval)
	}
	if c.Jobs.PurgeRetention < time.Hour {
		fail("jobs.purge_retention", "must be at least 1h, got %s", c.Jobs.PurgeRetention)
	}
	c.Jobs.validate(c.Database.MaxConns, fail)
	if c.Storage.Dir == "" {
		fail("storage.dir", "is required")
	}
	if c.Storage.MinFreeBytes < 0 {
		fail("storage.min_free_bytes", "must not be negative, got %d", c.Storage.MinFreeBytes)
	}
	c.Asset.validate(fail)
	c.Transfer.validate(fail)
	c.Transfer.validateImport(c.Asset, fail)
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Log.Level)); err != nil {
		fail("log.level", "must be one of debug, info, warn, error, got %q", c.Log.Level)
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		fail("log.format", "must be text or json, got %q", c.Log.Format)
	}
	return errors.Join(errs...)
}

func (a AuthConfig) validate(env string, fail func(key, format string, args ...any)) {
	if a.AccessTokenTTL <= 0 {
		fail("auth.access_token_ttl", "must be positive, got %s", a.AccessTokenTTL)
	}
	switch {
	case a.SessionTTL <= 0:
		fail("auth.session_ttl", "must be positive, got %s", a.SessionTTL)
	case a.SessionTTL <= a.AccessTokenTTL:
		fail("auth.session_ttl", "must be longer than auth.access_token_ttl (%s), got %s", a.AccessTokenTTL, a.SessionTTL)
	}
	// River advises periodic intervals of a second or more and does not
	// enforce it.
	if a.SessionCleanupInterval < time.Second {
		fail("auth.session_cleanup_interval", "must be at least 1s, got %s", a.SessionCleanupInterval)
	}
	if env == EnvProd && a.JWT.PrivateKeyFile == "" {
		// The file itself is read when nervewiki starts (bootstrap), not here.
		fail("auth.jwt.private_key_file", "is required in prod: a PKCS#8 PEM Ed25519 private key, e.g. from openssl genpkey -algorithm ed25519")
	}
	p := a.Password
	// golang.org/x/crypto/argon2 panics below one iteration or one lane, and
	// silently raises memory below 8 KiB per lane: reject those instead.
	if p.Argon2Iterations < 1 {
		fail("auth.password.argon2_iterations", "must be at least 1, got %d", p.Argon2Iterations)
	}
	if p.Argon2Parallelism < 1 {
		fail("auth.password.argon2_parallelism", "must be at least 1, got %d", p.Argon2Parallelism)
	}
	if least := max(8*uint32(p.Argon2Parallelism), 8); p.Argon2MemoryKiB < least {
		fail("auth.password.argon2_memory_kib", "must be at least 8 per lane (%d), got %d", least, p.Argon2MemoryKiB)
	}
	if p.MaxConcurrentHashes < 1 {
		fail("auth.password.max_concurrent_hashes", "must be at least 1, got %d", p.MaxConcurrentHashes)
	}
	if p.MaxWait <= 0 {
		fail("auth.password.max_wait", "must be positive, got %s", p.MaxWait)
	}
}

func (r RateLimitConfig) validate(fail func(key, format string, args ...any)) {
	if r.IPv6PrefixLen < 1 || r.IPv6PrefixLen > 128 {
		fail("ratelimit.ipv6_prefix_len", "must be from 1 to 128, got %d", r.IPv6PrefixLen)
	}
	for _, b := range []struct {
		name   string
		bucket BucketConfig
	}{
		{"anonymous", r.Anonymous},
		{"auth_failure", r.AuthFailure},
		{"authenticated", r.Authenticated},
		{"login_ip", r.LoginIP},
		{"login_ip_email", r.LoginIPEmail},
		{"register_ip", r.RegisterIP},
		{"password_user", r.PasswordUser},
		{"asset_content", r.AssetContent},
	} {
		if b.bucket.PerMinute < 1 {
			fail("ratelimit."+b.name+".per_minute", "must be at least 1, got %d", b.bucket.PerMinute)
		}
		if b.bucket.Burst < 1 {
			fail("ratelimit."+b.name+".burst", "must be at least 1, got %d", b.bucket.Burst)
		}
	}
}

func (a AssetConfig) validate(fail func(key, format string, args ...any)) {
	maxOK := a.MaxBytes >= MinAssetBytes && a.MaxBytes <= MaxAssetBytes
	if !maxOK {
		fail("asset.max_bytes", "must be from %d (1 KiB) to %d (4 GiB), got %d", MinAssetBytes, MaxAssetBytes, a.MaxBytes)
	}
	switch {
	case a.UploadMinRate < 1:
		fail("asset.upload_min_rate", "must be at least 1, got %d", a.UploadMinRate)
	case maxOK && float64(a.MaxBytes)/float64(a.UploadMinRate) > MaxAssetTransfer.Seconds():
		// Each upload may hold its connection that long: the bound keeps a
		// slow client from holding one for days.
		fail("asset.upload_min_rate", "must let asset.max_bytes (%d) arrive within %s, at least %d, got %d",
			a.MaxBytes, MaxAssetTransfer, (a.MaxBytes+int64(MaxAssetTransfer.Seconds())-1)/int64(MaxAssetTransfer.Seconds()), a.UploadMinRate)
	}
}

// validate checks the workers of the exports' and imports' queues: each
// export holds a connection through its snapshot, an import as it writes
// a batch; half the pool stays the requests' and River's.
func (j JobsConfig) validate(maxConns int32, fail func(key, format string, args ...any)) {
	exportsOK := false
	switch n := j.ExportWorkers; {
	case n < 1 || n > MaxTransferWorkers:
		fail("jobs.export_workers", "must be from 1 to %d, got %d", MaxTransferWorkers, n)
	case maxConns >= 1 && 2*n > int(maxConns):
		fail("jobs.export_workers", "must leave half of database.max_conns (%d) to the requests, at most %d, got %d", maxConns, maxConns/2, n)
	default:
		exportsOK = true
	}
	switch n := j.ImportWorkers; {
	case n < 1 || n > MaxTransferWorkers:
		fail("jobs.import_workers", "must be from 1 to %d, got %d", MaxTransferWorkers, n)
	case exportsOK && maxConns >= 1 && 2*(j.ExportWorkers+n) > int(maxConns):
		fail("jobs.import_workers", "must leave, with jobs.export_workers (%d), half of database.max_conns (%d) to the requests, at most %d, got %d",
			j.ExportWorkers, maxConns, int(maxConns)/2-j.ExportWorkers, n)
	}
}

// validateImport checks the imports' bounds against each other and the
// attachments': asset.max_bytes <= import_max_bytes <=
// import_max_unpacked_bytes (M7/P6 design 3.15).
func (t TransferConfig) validateImport(a AssetConfig, fail func(key, format string, args ...any)) {
	switch {
	case t.ImportMaxBytes < MinImportBytes:
		fail("transfer.import_max_bytes", "must be at least %d (1 MiB), got %d", MinImportBytes, t.ImportMaxBytes)
	case t.ImportMaxBytes < a.MaxBytes:
		fail("transfer.import_max_bytes", "must be at least asset.max_bytes (%d), got %d", a.MaxBytes, t.ImportMaxBytes)
	case a.UploadMinRate >= 1 && float64(t.ImportMaxBytes)/float64(a.UploadMinRate) > MaxImportTransfer.Seconds():
		// Each import's upload may hold its connection that long.
		fail("transfer.import_max_bytes", "must arrive within %s at asset.upload_min_rate (%d), at most %d, got %d",
			MaxImportTransfer, a.UploadMinRate, a.UploadMinRate*int64(MaxImportTransfer.Seconds()), t.ImportMaxBytes)
	}
	if n := t.ImportMaxEntries; n < 1 || n > MaxImportEntries {
		fail("transfer.import_max_entries", "must be from 1 to %d, got %d", MaxImportEntries, n)
	}
	if t.ImportMaxUnpackedBytes < t.ImportMaxBytes {
		fail("transfer.import_max_unpacked_bytes", "must be at least transfer.import_max_bytes (%d), got %d", t.ImportMaxBytes,
			t.ImportMaxUnpackedBytes)
	}
}

func (t TransferConfig) validate(fail func(key, format string, args ...any)) {
	if t.ExportTTL < MinExportTTL {
		fail("transfer.export_ttl", "must be at least %s, got %s", MinExportTTL, t.ExportTTL)
	}
	jobOK := t.JobTimeout >= MinJobTimeout && t.JobTimeout <= MaxJobTimeout
	if !jobOK {
		fail("transfer.job_timeout", "must be from %s to %s, got %s", MinJobTimeout, MaxJobTimeout, t.JobTimeout)
	}
	switch {
	case t.HeartbeatTimeout < MinTransferHeartbeatTimeout:
		fail("transfer.heartbeat_timeout", "must be at least %s, got %s", MinTransferHeartbeatTimeout, t.HeartbeatTimeout)
	case jobOK && t.HeartbeatTimeout >= t.JobTimeout:
		// A job beats every second: a timeout that long would leave an
		// interrupted job running in its row past its own end.
		fail("transfer.heartbeat_timeout", "must be less than transfer.job_timeout (%s), got %s", t.JobTimeout, t.HeartbeatTimeout)
	}
	if t.MaxQueued < 1 {
		fail("transfer.max_queued", "must be at least 1, got %d", t.MaxQueued)
	}
}
