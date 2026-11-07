package markdown

import "sync"

// Reservation owns optional credit from the same global budget as Take.
// An open reservation retains its whole grant, including the credit its
// children return after KeepFacts. Close returns only unused credit; live
// children then return their own shares directly to the original budget.
// It stores neither facts nor children. The lock order is Hold -> this ->
// the semaphore; this type never takes a Hold's lock.
type Reservation struct {
	budget      *Budget
	mu          sync.Mutex
	total, free int
	closed      bool
}

// TryReserve takes credits now, or answers false without waiting, logging,
// or producing a parse error. credits is already Budget.Takes(upperBytes).
// Existing semaphore waiters retain their priority for positive takes.
func (b *Budget) TryReserve(credits int) (*Reservation, bool) {
	if credits < 0 || credits > b.size || credits > 0 && !b.bytes.TryAcquire(int64(credits)) {
		return nil, false
	}
	return &Reservation{budget: b, total: credits, free: credits}, true
}

// TryGrow tries only the difference from the grant already owned. A failed
// attempt changes nothing. A smaller request never shrinks a live grant.
func (r *Reservation) TryGrow(credits int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || credits < 0 || credits > r.budget.size {
		return false
	}
	delta := credits - r.total
	if delta <= 0 {
		return true
	}
	if !r.budget.bytes.TryAcquire(int64(delta)) {
		return false
	}
	r.total += delta
	r.free += delta
	return true
}

// Take lends a child's actual content take from the grant. It does not
// acquire the global semaphore again. The child's KeepFacts therefore uses
// actualBytes, never the upper bound used to reserve this grant.
func (r *Reservation) Take(actualBytes int) (*Hold, bool) {
	h := r.budget.hold(actualBytes)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || h.n > r.free {
		return nil, false
	}
	r.free -= h.n
	h.parent = r
	return h, true
}

// give is called under a child's lock. After Close, that child is the only
// owner of its remaining share; returning it must not reopen the grant.
func (r *Reservation) give(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		r.total -= n
		r.budget.bytes.Release(int64(n))
	} else {
		r.free += n
	}
}

// Close gives back the unused part once. A parsed result handed to a Unit
// keeps its child Hold and releases that share at the original Unit end.
// A future old result must retain its open grant until ordered promotion
// or disposal; Close is not permission to discard a running task.
func (r *Reservation) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.total -= r.free
	r.budget.bytes.Release(int64(r.free))
	r.free = 0
}
