package notebook_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// UpgradeStates writes under its caller's transaction (M11/P1 design
// 3.11): outside one each method fails and writes nothing; Lock answers
// the notebook's state with its four upgrade columns, none for no
// notebook; Start writes the four columns once, and they go with the
// caller's rollback; Start refuses a zero id, a state not upgrading, an
// LLM Wiki and a notebook with an upgrade, each a defect its caller judges
// before it.
func TestTheUpgradeStatesWriteUnderTheirCaller(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	states := notebook.NewUpgradeStates(f.pool, nil)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	up := notebook.Upgrade{ID: uuid.NewV7(), State: notebook.UpgradeUpgrading, By: f.bob, At: testNow()}
	// upgradeOf is eng's upgrade's four columns, read outside.
	upgradeOf := func(id uuid.UUID) notebook.Upgrade {
		t.Helper()
		s, ok, err := notebook.NewNotebooks(f.pool).StateOf(ctx, id)
		if err != nil || !ok {
			t.Fatalf("StateOf(%v) = %v, %v", id, ok, err)
		}
		return s.Upgrade
	}

	if _, _, err := states.Lock(ctx, f.eng); err == nil {
		t.Error("Lock() outside a transaction = nil, want an error")
	}
	if err := states.Start(ctx, f.eng, up); err == nil {
		t.Error("Start() outside a transaction = nil, want an error")
	}
	if err := states.Publish(ctx, notebook.ModeChange{NotebookID: f.eng}); err == nil {
		t.Error("Publish() outside a transaction = nil, want an error")
	}
	if got := upgradeOf(f.eng); got != (notebook.Upgrade{}) {
		t.Fatalf("eng's upgrade after the calls outside = %+v, want none", got)
	}

	rolledBack := errors.New("the caller rolls back")
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		s, ok, err := states.Lock(ctx, f.eng)
		if err != nil || !ok || s != (notebook.State{Mode: notebook.ModeStandard}) {
			t.Errorf("Lock(eng) = %+v, %v, %v; want standard with none", s, ok, err)
		}
		if _, ok, err := states.Lock(ctx, uuid.NewV7()); ok || err != nil {
			t.Errorf("Lock(no notebook) = %v, %v; want none", ok, err)
		}
		if err := states.Start(ctx, f.eng, up); err != nil {
			t.Fatal(err)
		}
		if s, _, _ := states.Lock(ctx, f.eng); s.Upgrade.ID != up.ID || s.Upgrade.State != up.State || s.Upgrade.By != up.By ||
			!s.Upgrade.At.Equal(up.At) {
			t.Errorf("Lock(eng) after Start = %+v, want the upgrade %+v", s, up)
		}
		return rolledBack
	})
	if !errors.Is(err, rolledBack) {
		t.Fatal(err)
	}
	if got := upgradeOf(f.eng); got != (notebook.Upgrade{}) {
		t.Errorf("eng's upgrade after its caller rolled back = %+v, want none", got)
	}

	f.exec(t, "UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", f.ops, uuid.NewV7())
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		up       notebook.Upgrade
	}{
		{"a zero id", f.eng, notebook.Upgrade{State: notebook.UpgradeUpgrading, By: f.bob, At: testNow()}},
		{"reviewing", f.eng, notebook.Upgrade{ID: uuid.NewV7(), State: notebook.UpgradeReviewing, By: f.bob, At: testNow()}},
		{"an LLM Wiki", f.ops, up},
		{"no notebook", uuid.NewV7(), up},
	} {
		if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Start(ctx, tt.notebook, tt.up) }); err == nil {
			t.Errorf("Start(%s) = nil, want a defect's error", tt.name)
		}
	}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Start(ctx, f.eng, up) }); err != nil {
		t.Fatal(err)
	}
	if got := upgradeOf(f.eng); got.ID != up.ID || got.State != up.State || got.By != up.By || !got.At.Equal(up.At) {
		t.Errorf("eng's upgrade after a committed Start = %+v, want %+v", got, up)
	}
	var by uuid.UUID
	var at time.Time
	if err := f.pool.QueryRow(ctx, "SELECT updated_by_id, updated_at FROM notebooks WHERE id = $1", f.eng).Scan(&by, &at); err != nil ||
		by != f.bob || !at.Equal(testNow()) {
		t.Errorf("eng updated by %v at %v, %v; want the initiator at the start", by, at, err)
	}
	another := notebook.Upgrade{ID: uuid.NewV7(), State: notebook.UpgradeUpgrading, By: f.alice, At: testNow()}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Start(ctx, f.eng, another) }); err == nil {
		t.Error("Start(a notebook with an upgrade) = nil, want a defect's error")
	}
	if got := upgradeOf(f.eng); got.ID != up.ID {
		t.Errorf("eng's upgrade after a second Start = %+v, want the first", got)
	}
}

// SetState and Clear write the notebook's upgrade when it is theirs
// (M11/P1 design 3.20): outside a transaction each fails; another
// upgrade's id, or none, changes no row and is a defect's error; SetState
// sets the state alone, the start's who and when kept; Clear clears the
// four columns together; each records who and when on the row; both go
// with their caller's rollback.
func TestTheUpgradeStatesSetAndClearTheirUpgradeOnly(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	states := notebook.NewUpgradeStates(f.pool, nil)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	up := notebook.Upgrade{ID: uuid.NewV7(), State: notebook.UpgradeUpgrading, By: f.bob, At: testNow()}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Start(ctx, f.eng, up) }); err != nil {
		t.Fatal(err)
	}
	// row is eng's upgrade's four columns and who changed the row when.
	type row struct {
		upgrade, by *uuid.UUID
		state       *string
		at          *time.Time
		updatedBy   uuid.UUID
		updatedAt   time.Time
	}
	read := func() row {
		t.Helper()
		var r row
		if err := f.pool.QueryRow(ctx, `SELECT upgrade_id, upgrade_state, upgrade_by_id, upgrade_at, updated_by_id, updated_at
			FROM notebooks WHERE id = $1`, f.eng).Scan(&r.upgrade, &r.state, &r.by, &r.at, &r.updatedBy, &r.updatedAt); err != nil {
			t.Fatal(err)
		}
		return r
	}
	later, latest := testNow().Add(time.Minute), testNow().Add(2*time.Minute)
	if err := states.SetState(ctx, f.eng, up.ID, notebook.UpgradeRollingBack, f.alice, later); err == nil {
		t.Error("SetState() outside a transaction = nil, want an error")
	}
	if err := states.Clear(ctx, f.eng, up.ID, f.alice, later); err == nil {
		t.Error("Clear() outside a transaction = nil, want an error")
	}
	other := uuid.NewV7()
	for name, write := range map[string]func(ctx context.Context) error{
		"SetState of another upgrade": func(ctx context.Context) error {
			return states.SetState(ctx, f.eng, other, notebook.UpgradeRollingBack, f.alice, later)
		},
		"Clear of another upgrade": func(ctx context.Context) error { return states.Clear(ctx, f.eng, other, f.alice, later) },
		"SetState of a notebook of none": func(ctx context.Context) error {
			return states.SetState(ctx, f.ops, up.ID, notebook.UpgradeRollingBack, f.alice, later)
		},
	} {
		if err := tx.WithinTx(ctx, write); err == nil {
			t.Errorf("%s = nil, want a defect's error", name)
		}
	}
	if r := read(); r.state == nil || *r.state != string(notebook.UpgradeUpgrading) || r.updatedBy != f.bob {
		t.Errorf("eng after the writes of another upgrade = %+v, want as it started", r)
	}

	rolledBack := errors.New("the caller rolls back")
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := states.SetState(ctx, f.eng, up.ID, notebook.UpgradeRollingBack, f.alice, later); err != nil {
			return err
		}
		return rolledBack
	}); !errors.Is(err, rolledBack) {
		t.Fatal(err)
	}
	if r := read(); *r.state != string(notebook.UpgradeUpgrading) {
		t.Errorf("eng's state after its caller rolled back = %s, want upgrading", *r.state)
	}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		return states.SetState(ctx, f.eng, up.ID, notebook.UpgradeRollingBack, f.alice, later)
	}); err != nil {
		t.Fatal(err)
	}
	if r := read(); r.upgrade == nil || *r.upgrade != up.ID || *r.state != string(notebook.UpgradeRollingBack) || *r.by != f.bob ||
		!r.at.Equal(testNow()) || r.updatedBy != f.alice || !r.updatedAt.Equal(later) {
		t.Errorf("eng after SetState = %+v, want rolling back, the start's who and when kept, alice's change", r)
	}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Clear(ctx, f.eng, up.ID, f.alice, latest) }); err != nil {
		t.Fatal(err)
	}
	if r := read(); r.upgrade != nil || r.state != nil || r.by != nil || r.at != nil || r.updatedBy != f.alice || !r.updatedAt.Equal(latest) {
		t.Errorf("eng after Clear = %+v, want no upgrade, alice's change", r)
	}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Clear(ctx, f.eng, up.ID, f.alice, latest) }); err == nil {
		t.Error("Clear of an upgrade ended = nil, want a defect's error")
	}
}

// modeRecorder records the mode changes it is told of, in a shared list,
// and answers err.
type modeRecorder struct {
	name string
	seen *[]string
	got  []notebook.ModeChange
	err  error
}

func (r *modeRecorder) ModeChanged(_ context.Context, m notebook.ModeChange) error {
	*r.seen = append(*r.seen, r.name)
	r.got = append(r.got, m)
	return r.err
}

// Publish tells the subscribers in their order (M11/P1 design 3.11), the
// change as it is; the first error stops it and is its answer.
func TestTheUpgradeStatesPublishInOrder(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	var seen []string
	first, second := &modeRecorder{name: "first", seen: &seen}, &modeRecorder{name: "second", seen: &seen}
	states := notebook.NewUpgradeStates(f.pool, []notebook.ModeSubscriber{first, second})
	change := notebook.ModeChange{WorkspaceID: f.acme, NotebookID: f.eng, From: notebook.ModeStandard, To: notebook.ModeStandard,
		By: f.alice, At: testNow()}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Publish(ctx, change) }); err != nil {
		t.Fatal(err)
	}
	if want := []notebook.ModeChange{change}; !reflect.DeepEqual(seen, []string{"first", "second"}) ||
		!reflect.DeepEqual(first.got, want) || !reflect.DeepEqual(second.got, want) {
		t.Errorf("told %v: %+v, %+v; want first then second, %+v", seen, first.got, second.got, change)
	}
	seen, first.err = nil, errors.New("the stream is gone")
	if err := tx.WithinTx(ctx, func(ctx context.Context) error { return states.Publish(ctx, change) }); !errors.Is(err, first.err) ||
		!reflect.DeepEqual(seen, []string{"first"}) {
		t.Errorf("Publish with the first failing = %v after %v; want its error, the second not told", err, seen)
	}
}
