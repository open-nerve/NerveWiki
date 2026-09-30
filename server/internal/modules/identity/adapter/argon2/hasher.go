// Package argon2adapter hashes passwords with argon2id (M1/P1 design 3.4),
// with a cap on concurrent hashes and on how long a caller waits for one.
package argon2adapter

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Params are auth.password in the configuration.
type Params struct {
	MemoryKiB     uint32        // argon2_memory_kib
	Iterations    uint32        // argon2_iterations
	Parallelism   uint8         // argon2_parallelism
	MaxConcurrent int           // max_concurrent_hashes
	MaxWait       time.Duration // max_wait
}

const (
	saltLen = 16
	keyLen  = 32
	// retryAfter is the Retry-After of the 503 when no slot frees up.
	retryAfter = time.Second
)

// Hasher implements app.PasswordHasher and app.PasswordVerifier.
type Hasher struct {
	p       Params
	logger  *slog.Logger
	slots   chan struct{}
	waiting atomic.Int64
	// unusable stands in for a stored hash in another format: Verify does
	// the same work against it and matches no password.
	unusable phc
}

// New returns a hasher with params p.
func New(p Params, logger *slog.Logger) *Hasher {
	unusable := phc{memoryKiB: p.MemoryKiB, iterations: p.Iterations, parallelism: p.Parallelism, salt: make([]byte, saltLen), key: make([]byte, keyLen)}
	_, _ = rand.Read(unusable.salt) // never fails since Go 1.24
	_, _ = rand.Read(unusable.key)
	return &Hasher{p: p, logger: logger, slots: make(chan struct{}, p.MaxConcurrent), unusable: unusable}
}

// secret is what argon2id hashes of a password: its NFKC form (NIST SP
// 800-63B 5.1.1.2), so that the same password typed as precomposed or
// decomposed accents, or in full-width letters, is the same credential on
// every device.
func secret(password string) []byte {
	return []byte(norm.NFKC.String(password))
}

// Hash returns the PHC string of password's NFKC form:
// $argon2id$v=19$m=<KiB>,t=<iterations>,p=<lanes>$<salt>$<key>, base64
// without padding. When no slot frees up within MaxWait it returns 503
// server_busy with Retry-After: 1 and logs the queue length.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer func() { <-h.slots }()
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt) // never fails since Go 1.24
	key := argon2.IDKey(secret(password), salt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.p.MemoryKiB, h.p.Iterations, h.p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password, in its NFKC form, matches hash, a PHC
// string that Hash wrote, and whether hash has other parameters than the current ones, so
// that login hashes the password again. It takes a slot like
// Hash, with the same wait and the same 503.
//
// A hash in another format is an unusable password (v0.1 writes none): it
// matches no password, and Verify says so only after the work of a real
// verification at the current parameters, so that a login's answer takes
// as long as for any other account (M1 design 10). It is logged as a
// warning, without the hash.
func (h *Hasher) Verify(ctx context.Context, password, hash string) (ok, rehash bool, err error) {
	p, parseErr := parsePHC(hash)
	if parseErr != nil {
		p = h.unusable
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer func() { <-h.slots }()
	key := argon2.IDKey(secret(password), p.salt, p.iterations, p.memoryKiB, p.parallelism, keyLen)
	if parseErr != nil {
		h.logger.WarnContext(ctx, "a stored password hash is not an argon2id PHC string: no password matches it")
		return false, false, nil
	}
	ok = subtle.ConstantTimeCompare(key, p.key) == 1
	rehash = p.memoryKiB != h.p.MemoryKiB || p.iterations != h.p.Iterations || p.parallelism != h.p.Parallelism
	return ok, rehash, nil
}

// phc is a PHC string's parameters, salt and key.
type phc struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
	salt, key   []byte
}

// errNotOurHash is parsePHC's error for a hash in another format.
var errNotOurHash = errors.New("password hash is not an argon2id PHC string of this hasher")

// parsePHC reads $argon2id$v=19$m=<KiB>,t=<iterations>,p=<lanes>$<salt>$<key>
// with a 16-byte salt and a 32-byte key, the only form Hash writes.
func parsePHC(s string) (phc, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return phc{}, errNotOurHash
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return phc{}, errNotOurHash
	}
	m, okM := param(params[0], "m", 32)
	t, okT := param(params[1], "t", 32)
	l, okP := param(params[2], "p", 8)
	salt, errSalt := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	key, errKey := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if !okM || !okT || !okP || errSalt != nil || errKey != nil || len(salt) != saltLen || len(key) != keyLen {
		return phc{}, errNotOurHash
	}
	return phc{memoryKiB: uint32(m), iterations: uint32(t), parallelism: uint8(l), salt: salt, key: key}, nil
}

// param reads name=<n> with 0 < n < 2^bits: argon2 panics at zero rounds
// or lanes.
func param(s, name string, bits int) (uint64, bool) {
	value, found := strings.CutPrefix(s, name+"=")
	n, err := strconv.ParseUint(value, 10, bits)
	return n, found && err == nil && n > 0
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	default:
	}
	queued := h.waiting.Add(1)
	defer h.waiting.Add(-1)
	timer := time.NewTimer(h.p.MaxWait)
	defer timer.Stop()
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-timer.C:
		h.logger.LogAttrs(ctx, slog.LevelInfo, "password hashing is saturated", slog.Int64("queued", queued))
		return shared.ServerBusy(retryAfter)
	case <-ctx.Done():
		return ctx.Err()
	}
}
