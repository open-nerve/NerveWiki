package ratelimit

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a clock the test moves by hand, to the nanosecond:
// clocktest.Fixed keeps microseconds, like the database.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// units reads what key holds in b now, without taking any.
func units(b *Bucket, key string) float64 {
	l := b.limiter
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level(Check{Bucket: b, Key: key}, l.now()).units
}

// A bucket lets its burst through at once, then one unit per interval.
func TestBucketAllowsItsBurstThenItsRate(t *testing.T) {
	clock := newClock()
	b := New(clock.now).Bucket("login_ip", Rate{PerMinute: 60, Burst: 3})

	for i := range 3 {
		if _, ok := b.Allow("192.0.2.1"); !ok {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}
	if retry, ok := b.Allow("192.0.2.1"); ok || retry != time.Second {
		t.Errorf("after the burst: Allow() = %v, %v; want refused, retry in 1s", retry, ok)
	}
	clock.advance(400 * time.Millisecond)
	if retry, ok := b.Allow("192.0.2.1"); ok || retry != 600*time.Millisecond {
		t.Errorf("0.4 s later: Allow() = %v, %v; want refused, retry in 600ms", retry, ok)
	}
	clock.advance(600 * time.Millisecond)
	if _, ok := b.Allow("192.0.2.1"); !ok {
		t.Error("a second later: Allow() refused, want the unit that came back")
	}
	if _, ok := b.Allow("192.0.2.1"); ok {
		t.Error("the unit that came back was taken twice")
	}
}

func TestBucketNeverHoldsMoreThanItsBurst(t *testing.T) {
	clock := newClock()
	b := New(clock.now).Bucket("anonymous", Rate{PerMinute: 600, Burst: 2})
	b.Allow("k")

	clock.advance(time.Hour)

	if got := units(b, "k"); got != 2 {
		t.Errorf("after an hour the bucket holds %v, want its burst 2", got)
	}
}

func TestKeysAndBucketsAreSeparate(t *testing.T) {
	l := New(newClock().now)
	a := l.Bucket("a", Rate{PerMinute: 1, Burst: 1})
	b := l.Bucket("b", Rate{PerMinute: 1, Burst: 1})
	a.Allow("k")

	if _, ok := a.Allow("other"); !ok {
		t.Error("another key of the same bucket was refused")
	}
	if _, ok := b.Allow("k"); !ok {
		t.Error("the same key of another bucket was refused")
	}
	if _, ok := a.Allow("k"); ok {
		t.Error("the emptied key was allowed")
	}
}

// When one bucket refuses, AllowAll takes nothing from the others (M1/P2
// design 3.2): a refused sign-in to one address must not spend the IP's.
func TestAllowAllTakesFromEveryBucketOrNone(t *testing.T) {
	l := New(newClock().now)
	ip := l.Bucket("login_ip", Rate{PerMinute: 30, Burst: 5})
	ipEmail := l.Bucket("login_ip_email", Rate{PerMinute: 10, Burst: 2})
	checks := func(email string) []Check {
		return []Check{{Bucket: ip, Key: "192.0.2.1"}, {Bucket: ipEmail, Key: "192.0.2.1 " + email}}
	}

	for range 2 {
		if denied, _ := l.AllowAll(checks("a@x")...); denied != nil {
			t.Fatalf("AllowAll() refused by %s within both bursts", denied.Name())
		}
	}
	for range 10 {
		if denied, retry := l.AllowAll(checks("a@x")...); denied != ipEmail || retry != 6*time.Second {
			t.Fatalf("AllowAll() = %v, %v; want refused by login_ip_email, retry in 6s", denied, retry)
		}
	}

	if got := units(ip, "192.0.2.1"); got != 3 {
		t.Errorf("login_ip holds %v after 2 allowed and 10 refused attempts, want 3", got)
	}
	if denied, _ := l.AllowAll(checks("b@x")...); denied != nil {
		t.Errorf("another address from the same IP was refused by %s", denied.Name())
	}
}

// Of several empty buckets, AllowAll names the one with the longest wait.
func TestAllowAllReportsTheLongestWait(t *testing.T) {
	l := New(newClock().now)
	fast := l.Bucket("fast", Rate{PerMinute: 60, Burst: 1})
	slow := l.Bucket("slow", Rate{PerMinute: 1, Burst: 1})
	both := []Check{{Bucket: fast, Key: "k"}, {Bucket: slow, Key: "k"}}
	l.AllowAll(both...)

	if denied, retry := l.AllowAll(both...); denied != slow || retry != time.Minute {
		t.Errorf("AllowAll() = %v, %v; want slow, 1m", denied, retry)
	}
}

// A refusal always has a positive wait, which becomes Retry-After: here the
// float arithmetic puts the missing part of a unit just under 1ns.
func TestRefusalWaitsAtLeastANanosecond(t *testing.T) {
	clock := newClock()
	b := New(clock.now).Bucket("login_ip", Rate{PerMinute: 48, Burst: 1}) // a unit every 1.25 s
	b.Allow("k")
	clock.advance(1250*time.Millisecond - time.Nanosecond)

	if retry, ok := b.Allow("k"); ok || retry != time.Nanosecond {
		t.Errorf("1ns before the unit is back: Allow() = %v, %v; want refused, retry in 1ns", retry, ok)
	}
}

func TestAllowAllRejectsABucketOfAnotherLimiter(t *testing.T) {
	l := New(newClock().now)
	other := New(newClock().now).Bucket("other", Rate{PerMinute: 1, Burst: 1})
	defer func() {
		if recover() == nil {
			t.Error("AllowAll() with another limiter's bucket did not panic")
		}
	}()
	l.AllowAll(Check{Bucket: other, Key: "k"})
}

// The same bucket and key twice would take two units where one is left: a
// programming error, like a bucket of another limiter.
func TestAllowAllRejectsTheSameCheckTwice(t *testing.T) {
	l := New(newClock().now)
	b := l.Bucket("login_ip", Rate{PerMinute: 1, Burst: 1})
	defer func() {
		if recover() == nil {
			t.Error("AllowAll() with the same bucket and key twice did not panic")
		}
	}()
	l.AllowAll(Check{Bucket: b, Key: "k"}, Check{Bucket: b, Key: "k"})
}

// Reserve takes a unit at once; its refund gives it back once, and never
// beyond the burst (M1/P2 design 3.3: the failure gate refunds unless the
// credential failed).
func TestReserveAndRefund(t *testing.T) {
	clock := newClock()
	b := New(clock.now).Bucket("auth_failure", Rate{PerMinute: 60, Burst: 2})

	refund, _, ok := b.Reserve("192.0.2.1")
	if !ok || units(b, "192.0.2.1") != 1 {
		t.Fatalf("Reserve() ok = %v, bucket holds %v; want a unit taken", ok, units(b, "192.0.2.1"))
	}
	b.Reserve("192.0.2.1") // empty the bucket, so a second refund would show
	refund()
	refund()
	if got := units(b, "192.0.2.1"); got != 1 {
		t.Errorf("after refunding twice the bucket holds %v, want 1: a refund counts once", got)
	}
	clock.advance(time.Second) // the bucket is full again

	refundA, _, _ := b.Reserve("192.0.2.1")
	b.Reserve("192.0.2.1")
	if _, retry, ok := b.Reserve("192.0.2.1"); ok || retry != time.Second {
		t.Errorf("Reserve() of an empty bucket = %v, %v; want refused, retry in 1s", retry, ok)
	}
	clock.advance(2 * time.Second) // the bucket is full again
	refundA()
	if got := units(b, "192.0.2.1"); got != 2 {
		t.Errorf("a late refund left %v units, want at most the burst 2", got)
	}
}

// Reserving is one step under the lock: with a burst of 3, 50 concurrent
// reservations get exactly 3 units.
func TestReserveUnderConcurrency(t *testing.T) {
	b := New(newClock().now).Bucket("auth_failure", Rate{PerMinute: 60, Burst: 3})
	var granted atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, _, ok := b.Reserve("192.0.2.1"); ok {
				granted.Add(1)
			}
		})
	}
	wg.Wait()

	if got := granted.Load(); got != 3 {
		t.Errorf("%d of 50 concurrent reservations were granted, want the burst 3", got)
	}
}

// Keys whose bucket has filled up again are dropped once a minute; a key
// still short of its burst stays.
func TestIdleKeysAreDropped(t *testing.T) {
	clock := newClock()
	l := New(clock.now)
	b := l.Bucket("anonymous", Rate{PerMinute: 60, Burst: 10})
	for i := range 100 {
		b.Allow("192.0.2." + strconv.Itoa(i))
	}
	clock.advance(59 * time.Second)
	for range 5 {
		b.Allow("busy") // not yet a minute: no sweep
	}
	if len(l.levels) != 101 {
		t.Fatalf("%d keys before a minute has passed, want 101", len(l.levels))
	}

	clock.advance(time.Second)
	b.Allow("busy") // a minute has passed: the sweep runs first

	if len(l.levels) != 1 || units(b, "busy") != 5 {
		t.Errorf("after a minute %d keys are left and busy holds %v; want only busy, with 5", len(l.levels), units(b, "busy"))
	}
}
