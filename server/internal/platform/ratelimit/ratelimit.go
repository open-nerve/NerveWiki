// Package ratelimit is Nerve Wiki's in-process rate limiter (M1/P2 design 3.2):
// token buckets per key, each with a rate and a burst. A request takes one
// unit from a bucket; AllowAll takes one from several buckets or from none,
// and Reserve takes one that the caller may give back afterwards. A key
// whose bucket has filled up again is dropped from time to time, so the
// memory follows the callers of the last minutes. It uses only the
// standard library.
package ratelimit

import (
	"slices"
	"sync"
	"time"
)

// sweepEvery is how often AllowAll drops the keys whose bucket is full.
const sweepEvery = time.Minute

// Rate is a bucket's refill rate and size. Both are at least 1 (the
// configuration validates them).
type Rate struct {
	PerMinute int // units the bucket gains a minute
	Burst     int // units the bucket holds at most
}

// Limiter holds the state of all its buckets under one lock, so AllowAll
// can take from several at once.
type Limiter struct {
	mu        sync.Mutex
	now       func() time.Time
	levels    map[key]*level
	nextSweep time.Time
}

type key struct {
	bucket *Bucket
	id     string
}

// level is the units in one key's bucket, as of at.
type level struct {
	units float64
	at    time.Time
}

// New returns a limiter that tells the time with now: time.Now in
// production, whose monotonic reading a jump of the wall clock cannot move,
// so the jump neither fills nor drains a bucket.
func New(now func() time.Time) *Limiter {
	return &Limiter{now: now, levels: map[key]*level{}, nextSweep: now().Add(sweepEvery)}
}

// Bucket is one bucket per key, e.g. login_ip per client IP.
type Bucket struct {
	limiter  *Limiter
	name     string
	interval time.Duration // a unit comes back every interval
	burst    float64
}

// Bucket returns a bucket of l with rate r, named for logs, e.g. after its
// configuration key.
func (l *Limiter) Bucket(name string, r Rate) *Bucket {
	return &Bucket{
		limiter:  l,
		name:     name,
		interval: max(time.Minute/time.Duration(r.PerMinute), time.Nanosecond),
		burst:    float64(r.Burst),
	}
}

// Name is the bucket's name.
func (b *Bucket) Name() string { return b.name }

// Allow takes one unit from key's bucket. Without a unit it takes nothing
// and returns how long until one is back.
func (b *Bucket) Allow(key string) (retry time.Duration, ok bool) {
	if denied, retry := b.limiter.AllowAll(Check{Bucket: b, Key: key}); denied != nil {
		return retry, false
	}
	return 0, true
}

// Reserve takes one unit from key's bucket now; refund gives it back, at
// most once and never beyond the burst. Without a unit it takes nothing and
// returns how long until one is back.
func (b *Bucket) Reserve(key string) (refund func(), retry time.Duration, ok bool) {
	if retry, ok := b.Allow(key); !ok {
		return nil, retry, false
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			l := b.limiter
			l.mu.Lock()
			defer l.mu.Unlock()
			lv := l.level(Check{Bucket: b, Key: key}, l.now())
			lv.units = min(lv.units+1, b.burst)
		})
	}, 0, true
}

// Check is one key of one bucket.
type Check struct {
	Bucket *Bucket
	Key    string
}

// AllowAll takes one unit from the bucket of every check, or from none: when
// a bucket is empty it takes nothing and returns the empty bucket with the
// longest wait, and that wait, always positive (it becomes Retry-After).
// denied is nil when every bucket gave a unit. Every bucket must belong to l,
// and each check must be another bucket or key: one check twice would see
// the unit that is left twice and take two, below zero.
func (l *Limiter) AllowAll(checks ...Check) (denied *Bucket, retry time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	levels := make([]*level, len(checks))
	for i, c := range checks {
		if c.Bucket.limiter != l {
			panic("ratelimit: AllowAll with a bucket of another limiter")
		}
		if slices.Contains(checks[:i], c) {
			panic("ratelimit: AllowAll with the same bucket and key twice")
		}
		levels[i] = l.level(c, now)
		if units := levels[i].units; units < 1 {
			// At least 1ns: the missing fraction of a unit can round to 0.
			wait := max(time.Duration((1-units)*float64(c.Bucket.interval)), time.Nanosecond)
			if denied == nil || wait > retry {
				denied, retry = c.Bucket, wait
			}
		}
	}
	if denied != nil {
		return denied, retry
	}
	for _, lv := range levels {
		lv.units--
	}
	return nil, 0
}

// level returns c's level brought up to now; a new key starts full. The
// caller holds l.mu.
func (l *Limiter) level(c Check, now time.Time) *level {
	k := key{bucket: c.Bucket, id: c.Key}
	lv, ok := l.levels[k]
	if !ok {
		lv = &level{units: c.Bucket.burst, at: now}
		l.levels[k] = lv
	}
	c.Bucket.refill(lv, now)
	return lv
}

// refill adds the units that came back since lv.at, up to the burst.
func (b *Bucket) refill(lv *level, now time.Time) {
	if now.After(lv.at) {
		lv.units = min(b.burst, lv.units+float64(now.Sub(lv.at))/float64(b.interval))
		lv.at = now
	}
}

// sweep drops, at most once every sweepEvery, the keys whose bucket is full
// again: a full bucket is what a key without a level starts with. The caller
// holds l.mu.
func (l *Limiter) sweep(now time.Time) {
	if now.Before(l.nextSweep) {
		return
	}
	l.nextSweep = now.Add(sweepEvery)
	for k, lv := range l.levels {
		k.bucket.refill(lv, now)
		if lv.units >= k.bucket.burst {
			delete(l.levels, k)
		}
	}
}
