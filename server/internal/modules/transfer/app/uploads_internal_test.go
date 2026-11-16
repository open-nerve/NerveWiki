package app

import (
	"testing"
	"uuid"
)

// An upload counts for the others once admitted, with what it has yet to
// store, among its person's too (M12 design 4.6.6, T2); its row,
// committing, counts in its stead for a start under the queue's lock, not
// yet for a Check; written, for both.
func TestTheUploadsCountAsTheirRowsAreWritten(t *testing.T) {
	u := NewUploads()
	eng, ops := uuid.NewV7(), uuid.NewV7()
	bob, carol := uuid.NewV7(), uuid.NewV7()
	count := func(person uuid.UUID, locked bool) [3]int64 {
		all, mine, bytes := u.others(ops, person, locked)
		return [3]int64{int64(all), int64(mine), bytes}
	}
	steps := []struct {
		name           string
		do             func()
		check, creator [3]int64
	}{
		{"claimed", func() { u.claim(eng, bob, 100) }, [3]int64{}, [3]int64{}},
		{"admitted", func() { u.admit(eng) }, [3]int64{1, 1, 100}, [3]int64{1, 1, 100}},
		{"30 bytes stored", func() { u.store(eng, 30) }, [3]int64{1, 1, 70}, [3]int64{1, 1, 70}},
		{"more stored than declared", func() { u.store(eng, 90) }, [3]int64{1, 1, 0}, [3]int64{1, 1, 0}},
		{"its row committing", func() { u.rowCommitting(eng) }, [3]int64{1, 1, 0}, [3]int64{}},
		{"its row written", func() { u.rowWritten(eng) }, [3]int64{}, [3]int64{}},
	}
	for _, s := range steps {
		s.do()
		if got := count(bob, false); got != s.check {
			t.Errorf("%s: a Check of bob's counts %v, want %v", s.name, got, s.check)
		}
		if got := count(bob, true); got != s.creator {
			t.Errorf("%s: a start of bob's under the queue's lock counts %v, want %v", s.name, got, s.creator)
		}
		mine := s.check
		mine[1] = 0
		if got := count(carol, false); got != mine {
			t.Errorf("%s: a Check of carol's counts %v, want %v: bob's upload not hers", s.name, got, mine)
		}
	}
	if all, mine, _ := u.others(eng, bob, false); all != 0 || mine != 0 {
		t.Errorf("eng's own upload counted for it: %d, %d", all, mine)
	}
	if u.claim(eng, carol, 1) {
		t.Error("a second claim of eng taken")
	}
	u.release(eng)
	if !u.claim(eng, carol, 1) {
		t.Error("eng's claim not taken once released")
	}
}
