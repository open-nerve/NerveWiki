package app_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// memoKey is a key of a test's own, as a module keeps its own.
type memoKey struct{}

// memoParticipant stores the step's node under memoKey in the unit's memo
// and keeps the memo it got.
type memoParticipant struct{ memos []*app.UnitMemo }

func (p *memoParticipant) Participate(_ context.Context, s app.Step, _ app.Appender) error {
	p.memos = append(p.memos, s.Memo)
	s.Memo.Store(memoKey{}, s.Changes[0].NodeID)
	return nil
}

// memoObserver keeps the memo it got and takes memoKey from it, before and
// at the unit's end.
type memoObserver struct {
	memos []*app.UnitMemo
	taken []any
	found []bool
}

func (o *memoObserver) PagesChanged(_ context.Context, e app.Event) error {
	o.memos = append(o.memos, e.Memo)
	v, ok := e.Memo.Take(memoKey{})
	o.taken, o.found = append(o.taken, v), append(o.found, ok)
	return nil
}

// Each unit has one memo of its own (M12/P4/S7 design 4.3 item 7): its
// participants and its observers get the same one, which the next unit
// does not; what a participant stores its observer takes once. A nil memo
// holds nothing and does nothing.
func TestEachUnitHasItsOwnMemo(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	a, b := f.page("A", nil, 0), f.page("B", nil, 1)
	p, o := &memoParticipant{}, &memoObserver{}
	f.partakers, f.observers = []app.Participant{p}, []app.PageObserver{o}
	for i, n := range []domain.Node{a, b} {
		if _, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
			_, err := u.Rename(ctx, n.ID, "Renamed "+n.Name)
			return err
		}); err != nil {
			t.Fatalf("unit %d: %v", i, err)
		}
	}
	if len(p.memos) != 2 || len(o.memos) != 2 {
		t.Fatalf("participated %d times, observed %d; want 2 each", len(p.memos), len(o.memos))
	}
	for i := range 2 {
		if p.memos[i] == nil || p.memos[i] != o.memos[i] {
			t.Errorf("unit %d: the participant's memo %p, the observer's %p; want one", i, p.memos[i], o.memos[i])
		}
	}
	if p.memos[0] == p.memos[1] {
		t.Error("two units shared a memo")
	}
	if o.taken[0] != a.ID || o.taken[1] != b.ID || !o.found[0] || !o.found[1] {
		t.Errorf("the observers took %v (%v); want each its unit's node", o.taken, o.found)
	}
	if v, ok := o.memos[1].Take(memoKey{}); ok || v != nil {
		t.Errorf("a memo still held %v after its observer took it", v)
	}
	var none *app.UnitMemo
	none.Store(memoKey{}, 1)
	none.Drop(memoKey{})
	if v, ok := none.Take(memoKey{}); ok || v != nil {
		t.Errorf("a nil memo held %v", v)
	}
	m := &app.UnitMemo{}
	m.Store(memoKey{}, 1)
	m.Drop(memoKey{})
	if _, ok := m.Take(memoKey{}); ok {
		t.Error("a memo held what was dropped")
	}
}
