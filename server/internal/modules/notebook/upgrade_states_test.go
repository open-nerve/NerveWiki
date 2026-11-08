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
