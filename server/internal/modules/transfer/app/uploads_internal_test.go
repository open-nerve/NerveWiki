package app

import (
	"testing"
	"uuid"
)

// An upload counts for the others once admitted, with what it has yet to
// store; its row, committing, counts in its stead for a start under the
// queue's lock, not yet for a Check; written, for both.
func TestTheUploadsCountAsTheirRowsAreWritten(t *testing.T) {
	u := NewUploads()
	eng, ops := uuid.NewV7(), uuid.NewV7()
	count := func(locked bool) [2]int64 {
		n, bytes := u.others(ops, locked)
		return [2]int64{int64(n), bytes}
	}
	steps := []struct {
		name           string
		do             func()
		check, creator [2]int64
	}{
		{"claimed", func() { u.claim(eng, 100) }, [2]int64{}, [2]int64{}},
		{"admitted", func() { u.admit(eng) }, [2]int64{1, 100}, [2]int64{1, 100}},
		{"30 bytes stored", func() { u.store(eng, 30) }, [2]int64{1, 70}, [2]int64{1, 70}},
		{"more stored than declared", func() { u.store(eng, 90) }, [2]int64{1, 0}, [2]int64{1, 0}},
		{"its row committing", func() { u.rowCommitting(eng) }, [2]int64{1, 0}, [2]int64{}},
		{"its row written", func() { u.rowWritten(eng) }, [2]int64{}, [2]int64{}},
	}
	for _, s := range steps {
		s.do()
		if got := count(false); got != s.check {
			t.Errorf("%s: a Check counts %v, want %v", s.name, got, s.check)
		}
		if got := count(true); got != s.creator {
			t.Errorf("%s: a start under the queue's lock counts %v, want %v", s.name, got, s.creator)
		}
	}
	if n, _ := u.others(eng, false); n != 0 {
		t.Errorf("eng's own upload counted for it: %d", n)
	}
	if u.claim(eng, 1) {
		t.Error("a second claim of eng taken")
	}
	u.release(eng)
	if !u.claim(eng, 1) {
		t.Error("eng's claim not taken once released")
	}
}
