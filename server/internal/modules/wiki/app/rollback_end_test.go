package app_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// endFake is every port of a rollback's end: the notebook's state as
// state says, none when noLock; what is left; whether the module's row
// still rolls back; the calls in order and what each wrote.
type endFake struct {
	unsubmitted
	calls     []string
	ws        uuid.UUID
	at, since time.Time
	state     app.UpgradeNotebook
	noLock    bool
	left      domain.RollbackLeft
	noRow     bool
	cleared   [3]uuid.UUID // the notebook, the upgrade, by
	deleted   uuid.UUID
	ended     uuid.UUID
	endedBy   uuid.UUID
	endedLeft *domain.RollbackLeft
	published []app.ModeChange
}

func (f *endFake) note(call string) { f.calls = append(f.calls, call) }

func (f *endFake) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	f.note("workspace")
	return f.ws, true, nil
}

func (f *endFake) StateOf(context.Context, uuid.UUID) (app.NotebookState, bool, error) {
	return app.NotebookState{}, false, errors.New("the end reads its state under the lock")
}

func (f *endFake) Lock(context.Context, uuid.UUID) (app.UpgradeNotebook, bool, error) {
	f.note("lock")
	return f.state, !f.noLock, nil
}

func (f *endFake) Start(context.Context, uuid.UUID, app.NotebookUpgrade) error {
	return errors.New("the end starts no upgrade")
}

func (f *endFake) SetState(context.Context, uuid.UUID, uuid.UUID, domain.UpgradeState, uuid.UUID, time.Time) error {
	return errors.New("the end sets no state")
}

func (f *endFake) Clear(_ context.Context, id, upgradeID, by uuid.UUID, at time.Time) error {
	f.note("clear")
	if !at.Equal(f.at) {
		return errors.New("cleared at another time")
	}
	f.cleared = [3]uuid.UUID{id, upgradeID, by}
	return nil
}

func (f *endFake) Publish(_ context.Context, c app.ModeChange) error {
	f.note("publish")
	f.published = append(f.published, c)
	return nil
}

func (f *endFake) Left(context.Context, uuid.UUID, uuid.UUID) (domain.RollbackLeft, error) {
	f.note("left")
	return f.left, nil
}

func (f *endFake) DeleteBaseline(_ context.Context, upgradeID uuid.UUID) error {
	f.note("delete baseline")
	f.deleted = upgradeID
	return nil
}

func (f *endFake) Insert(context.Context, domain.Upgrade, uuid.UUID, time.Time) error {
	return errors.New("the end inserts no row")
}

func (f *endFake) Current(context.Context, uuid.UUID) (domain.Upgrade, bool, error) {
	return domain.Upgrade{}, false, errors.New("the end reads no row")
}

func (f *endFake) RollBack(context.Context, uuid.UUID, domain.UpgradeEnding, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errors.New("the end rolls no upgrade back")
}

func (f *endFake) ContinueRollback(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errors.New("the end continues no rollback")
}

func (f *endFake) EndRollback(_ context.Context, id uuid.UUID, left *domain.RollbackLeft, by uuid.UUID, at time.Time) (time.Time, bool, error) {
	f.note("end row")
	if !at.Equal(f.at) {
		return time.Time{}, false, errors.New("ended at another time")
	}
	f.ended, f.endedLeft, f.endedBy = id, left, by
	return f.since, !f.noRow, nil
}

func (f *endFake) Now() time.Time {
	f.note("clock")
	return f.at
}

// An upgrade's rollback's end (M11/P1 design 3.20), with fakes of its
// ports, as the job's starter: End answers false and writes nothing when
// the notebook is gone, its upgrade another or not rolling back; true and
// writes nothing when something is left; else, in order, the clock, the
// notebook's upgrade cleared, the baseline deleted, the row rolled back,
// complete, the mode's change from standard to standard told, by the
// starter at the clock's time, and an info line measuring from the row's
// last change; a row no longer rolling back is a defect. Stopped tells the
// change when the upgrade is still it, rolling back, alone. Without an
// actor each fails.
func TestTheRollbackEnd(t *testing.T) {
	nb, up, starter := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC)
	rolling := app.UpgradeNotebook{Mode: domain.ModeStandard, Upgrade: app.NotebookUpgrade{ID: up, State: domain.UpgradeRollingBack,
		By: uuid.NewV7()}}
	fixture := func(change func(*endFake)) (*endFake, *app.RollbackEnd, *startLog) {
		f := &endFake{ws: uuid.NewV7(), at: at, since: at.Add(-90 * time.Second), state: rolling}
		if change != nil {
			change(f)
		}
		log := &startLog{}
		end := app.NewRollbackEnd(app.RollbackEndDeps{Notebooks: f, States: unapproved{f}, Pages: f, Rows: f, Clock: f,
			Logger: slog.New(slog.NewJSONHandler(log, nil))})
		return f, end, log
	}
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: starter, JobID: uuid.NewV7()})

	for _, tt := range []struct {
		name   string
		change func(*endFake)
		left   bool
		calls  string
	}{
		{"the notebook gone", func(f *endFake) { f.noLock = true }, false, "lock"},
		{"another upgrade", func(f *endFake) { f.state.Upgrade.ID = uuid.NewV7() }, false, "lock"},
		{"upgrading", func(f *endFake) { f.state.Upgrade.State = domain.UpgradeUpgrading }, false, "lock"},
		{"none", func(f *endFake) { f.state.Upgrade = app.NotebookUpgrade{} }, false, "lock"},
		{"a page left", func(f *endFake) { f.left = domain.RollbackLeft{Pages: 1} }, true, "lock, left"},
		{"a node new left", func(f *endFake) { f.left = domain.RollbackLeft{Created: 1} }, true, "lock, left"},
		{"a node out of place left", func(f *endFake) { f.left = domain.RollbackLeft{Nodes: 1} }, true, "lock, left"},
	} {
		f, end, log := fixture(tt.change)
		left, committed, err := end.End(ctx, nb, up)
		if err != nil || left != tt.left || committed != nil || strings.Join(f.calls, ", ") != tt.calls || len(log.lines(t)) != 0 {
			t.Errorf("%s: End() = %v, committed %v, %v after %v, logged %v; want %v, none, after %s, nothing written", tt.name, left,
				committed != nil, err, f.calls, log.lines(t), tt.left, tt.calls)
		}
	}

	f, end, log := fixture(nil)
	left, committed, err := end.End(ctx, nb, up)
	if want := "lock, left, workspace, clock, clear, delete baseline, end row, publish"; err != nil || left || committed == nil ||
		strings.Join(f.calls, ", ") != want {
		t.Fatalf("End() of nothing left = %v, committed %v, %v after %v; want false and committed after %s", left, committed != nil, err,
			f.calls, want)
	}
	// The line is its caller's to log once the transaction committed
	// (M11/P1 S3 review A-N1).
	if lines := log.lines(t); len(lines) != 0 {
		t.Errorf("logged %v before committed, want nothing", lines)
	}
	committed()
	if f.cleared != [3]uuid.UUID{nb, up, starter} || f.deleted != up || f.ended != up || f.endedLeft != nil || f.endedBy != starter {
		t.Errorf("cleared %v, deleted %v, ended %v by %v left %v; want the upgrade's, by the starter, complete", f.cleared, f.deleted,
			f.ended, f.endedBy, f.endedLeft)
	}
	if want := []app.ModeChange{{WorkspaceID: f.ws, NotebookID: nb, From: domain.ModeStandard, To: domain.ModeStandard, By: starter,
		At: at}}; !slices.Equal(f.published, want) {
		t.Errorf("published %+v, want %+v", f.published, want)
	}
	lines := log.lines(t)
	if len(lines) != 1 || lines[0]["msg"] != "upgrade rolled back" || lines[0]["level"] != "INFO" || lines[0]["upgrade_id"] != up.String() ||
		lines[0]["notebook_id"] != nb.String() || lines[0]["workspace_id"] != f.ws.String() || lines[0]["user_id"] != starter.String() ||
		lines[0]["elapsed_ms"] != float64(90000) {
		t.Errorf("logged %v, want one info line upgrade rolled back of 90,000 ms", lines)
	}

	f, end, _ = fixture(func(f *endFake) { f.noRow = true })
	if _, committed, err := end.End(ctx, nb, up); err == nil || committed != nil || slices.Contains(f.calls, "publish") {
		t.Errorf("End() of a row no longer rolling back = %v after %v, want a defect's error, nothing told", err, f.calls)
	}

	for _, tt := range []struct {
		name   string
		change func(*endFake)
		told   bool
	}{
		{"rolling back", nil, true},
		{"another upgrade", func(f *endFake) { f.state.Upgrade.ID = uuid.NewV7() }, false},
		{"reviewing", func(f *endFake) { f.state.Upgrade.State = domain.UpgradeReviewing }, false},
		{"the notebook gone", func(f *endFake) { f.noLock = true }, false},
	} {
		f, end, _ := fixture(tt.change)
		if err := end.Stopped(ctx, nb, up); err != nil || (len(f.published) == 1) != tt.told ||
			tt.told && f.published[0] != (app.ModeChange{WorkspaceID: f.ws, NotebookID: nb, From: domain.ModeStandard,
				To: domain.ModeStandard, By: starter, At: at}) {
			t.Errorf("%s: Stopped() = %v, told %+v; want told %v", tt.name, err, f.published, tt.told)
		}
	}

	_, end, _ = fixture(nil)
	if _, _, err := end.End(context.Background(), nb, up); err == nil {
		t.Error("End() without an actor = nil, want an error")
	}
	if err := end.Stopped(context.Background(), nb, up); err == nil {
		t.Error("Stopped() without an actor = nil, want an error")
	}
}
