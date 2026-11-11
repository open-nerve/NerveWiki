package app_test

import (
	"context"
	"errors"
	"fmt"
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

type abandonTxKey struct{}

// abandonFake is every port of an upgrade's abandonment, its rollback's
// end and its read (M11/P1 design 3.20): a transaction manager that runs
// fn and commits, or rolls back on its error; the notebook as state says,
// the module's row as row, which the writes change as the statements
// would; whether a job writes the tree; what the rollback left; the
// calls in order, each write's with what it wrote.
type abandonFake struct {
	unsubmitted
	calls []string

	at                   time.Time
	ws                   uuid.UUID
	noWorkspace, noShare bool
	noLock               bool
	state                app.UpgradeNotebook
	authErr              error
	writing              bool
	startErr             error
	job                  uuid.UUID
	row                  domain.Upgrade
	noRow                bool
	left                 domain.RollbackLeft
	published            []app.ModeChange
}

func (f *abandonFake) note(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *abandonFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.note("begin")
	if err := fn(context.WithValue(ctx, abandonTxKey{}, true)); err != nil {
		f.note("rollback")
		return err
	}
	f.note("commit")
	return nil
}

// WithinSnapshot runs fn, its calls between "snapshot" and "end snapshot".
func (f *abandonFake) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	f.note("snapshot")
	defer f.note("end snapshot")
	return fn(ctx)
}

// in fails the call outside the transaction.
func (f *abandonFake) in(ctx context.Context, call string) error {
	if ctx.Value(abandonTxKey{}) != true {
		return fmt.Errorf("%s outside the transaction", call)
	}
	return nil
}

func (f *abandonFake) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	f.note("workspace")
	return f.ws, !f.noWorkspace, nil
}

func (f *abandonFake) StateOf(context.Context, uuid.UUID) (app.NotebookState, bool, error) {
	return app.NotebookState{}, false, errors.New("read under the lock")
}

func (f *abandonFake) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.note("share")
	if id != f.ws {
		return false, errors.New("another workspace")
	}
	return !f.noShare, f.in(ctx, "share")
}

func (f *abandonFake) Lock(ctx context.Context, _ uuid.UUID) (app.UpgradeNotebook, bool, error) {
	f.note("lock")
	return f.state, !f.noLock, f.in(ctx, "lock")
}

func (f *abandonFake) Start(context.Context, uuid.UUID, app.NotebookUpgrade) error {
	return errors.New("no upgrade starts")
}

func (f *abandonFake) SetState(ctx context.Context, id, upgradeID uuid.UUID, state domain.UpgradeState, by uuid.UUID, at time.Time) error {
	f.note("set state %s of %s by %s at %s", state, upgradeID, by, at.Format(time.TimeOnly))
	return f.in(ctx, "set state")
}

func (f *abandonFake) Clear(ctx context.Context, id, upgradeID, by uuid.UUID, at time.Time) error {
	f.note("clear %s by %s at %s", upgradeID, by, at.Format(time.TimeOnly))
	return f.in(ctx, "clear")
}

func (f *abandonFake) Publish(ctx context.Context, c app.ModeChange) error {
	f.note("publish")
	f.published = append(f.published, c)
	return f.in(ctx, "publish")
}

func (f *abandonFake) Authorize(_ context.Context, _ shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	f.note("authorize")
	if action != "notebook.change_mode" || target.WorkspaceID != f.ws {
		return shared.Grant{}, errors.New("another decision")
	}
	return shared.Grant{}, f.authErr
}

func (f *abandonFake) Now() time.Time {
	f.note("clock")
	return f.at
}

func (f *abandonFake) Writing(context.Context, uuid.UUID) (bool, error) {
	f.note("writing")
	return f.writing, nil
}

func (f *abandonFake) Left(_ context.Context, upgradeID, _ uuid.UUID) (domain.RollbackLeft, error) {
	f.note("left of %s", upgradeID)
	return f.left, nil
}

func (f *abandonFake) DeleteBaseline(ctx context.Context, upgradeID uuid.UUID) error {
	f.note("delete baseline of %s", upgradeID)
	return f.in(ctx, "delete baseline")
}

func (f *abandonFake) Insert(context.Context, domain.Upgrade, uuid.UUID, time.Time) error {
	return errors.New("no row inserted")
}

func (f *abandonFake) Current(context.Context, uuid.UUID) (domain.Upgrade, bool, error) {
	f.note("current")
	return f.row, f.row.ID != uuid.UUID{}, nil
}

func (f *abandonFake) RollBack(ctx context.Context, id uuid.UUID, ending domain.UpgradeEnding, jobID, by uuid.UUID, at time.Time) (bool, error) {
	f.note("roll back %s %s with %s by %s at %s", id, ending, jobID, by, at.Format(time.TimeOnly))
	if f.noRow {
		return false, nil
	}
	f.row.State, f.row.Ending, f.row.RollbackJobID = domain.UpgradeRollingBack, ending, &jobID
	return true, f.in(ctx, "roll back")
}

func (f *abandonFake) ContinueRollback(ctx context.Context, id, jobID, by uuid.UUID, at time.Time) (bool, error) {
	f.note("continue %s with %s by %s at %s", id, jobID, by, at.Format(time.TimeOnly))
	if f.noRow {
		return false, nil
	}
	f.row.RollbackJobID = &jobID
	return true, f.in(ctx, "continue")
}

func (f *abandonFake) EndRollback(ctx context.Context, id uuid.UUID, left *domain.RollbackLeft, by uuid.UUID, at time.Time) (time.Time,
	bool, error,
) {
	f.note("end row %s left %v by %s at %s", id, left, by, at.Format(time.TimeOnly))
	if f.noRow {
		return time.Time{}, false, nil
	}
	return f.at.Add(-time.Minute), true, f.in(ctx, "end row")
}

func (f *abandonFake) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.note("names")
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = "name of " + id.String()
	}
	return out, nil
}

// abandonRollbacks are the transfer module's starts of a rollback, over
// the fake's job, error and calls.
type abandonRollbacks struct{ f *abandonFake }

func (r abandonRollbacks) Start(ctx context.Context, notebookID, upgradeID uuid.UUID, client string) (uuid.UUID, error) {
	r.f.note("start rollback of %s from %s", upgradeID, client)
	if err := r.f.in(ctx, "the rollback's start"); err != nil {
		return uuid.UUID{}, err
	}
	return r.f.job, r.f.startErr
}

// abandonFixture is the use cases over a fake: an admin, alice, abandons
// the upgrade up of notebook nb, upgrading, started by starter.
type abandonFixture struct {
	f                      *abandonFake
	log                    *startLog
	d                      app.UpgradeDeps
	nb, up, alice, starter uuid.UUID
}

func newAbandonFixture(change func(*abandonFake)) abandonFixture {
	x := abandonFixture{nb: uuid.NewV7(), up: uuid.NewV7(), alice: uuid.NewV7(), starter: uuid.NewV7(), log: &startLog{}}
	x.f = &abandonFake{ws: uuid.NewV7(), at: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC), job: uuid.NewV7()}
	x.f.state = app.UpgradeNotebook{Mode: domain.ModeStandard, Upgrade: app.NotebookUpgrade{ID: x.up, State: domain.UpgradeUpgrading,
		By: x.starter}}
	x.f.row = domain.Upgrade{ID: x.up, NotebookID: x.nb, State: domain.UpgradeUpgrading, CreatedBy: x.starter}
	if change != nil {
		change(x.f)
	}
	x.d = app.UpgradeDeps{Tx: x.f, Snapshots: x.f, Notebooks: x.f, Workspaces: x.f, States: unapproved{x.f}, Rollback: x.f, Rows: x.f, Writing: x.f,
		Rollbacks: abandonRollbacks{x.f}, Names: x.f, Authorizer: x.f, Action: "notebook.change_mode", Clock: x.f,
		Logger: slog.New(slog.NewJSONHandler(x.log, nil))}
	return x
}

func (x abandonFixture) ctx() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: x.alice})
}

// rollingBack sets the fixture's upgrade rolling back, abandoned, its job
// old.
func (x abandonFixture) rollingBack(old uuid.UUID) {
	x.f.state.Upgrade.State, x.f.row.State, x.f.row.Ending, x.f.row.RollbackJobID = domain.UpgradeRollingBack,
		domain.UpgradeRollingBack, domain.UpgradeAbandoned, &old
}

// refusals are the codes an abandonment and a rollback's end answer
// before they write, in their order, each with the last call before it.
type refusal struct {
	name   string
	change func(*abandonFake)
	code   string
	last   string
}

// refusedAfter tells whether calls end with last, then the transaction's
// rollback, or with last alone for the workspace's read before it.
func refusedAfter(calls []string, last string) bool {
	i := slices.Index(calls, last)
	switch {
	case i < 0:
		return false
	case last == "workspace":
		return i == len(calls)-1
	}
	return slices.Equal(calls[i+1:], []string{"rollback"})
}

func codeOf(err error) string {
	var e *shared.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return fmt.Sprint(err)
}

// lockRefusals are the refusals of the lock and the decision, which both
// use cases share.
func lockRefusals() []refusal {
	return []refusal{
		{"no notebook", func(f *abandonFake) { f.noWorkspace = true }, "notebook.not_found", "workspace"},
		{"no workspace's row", func(f *abandonFake) { f.noShare = true }, "notebook.not_found", "share"},
		{"no notebook's row", func(f *abandonFake) { f.noLock = true }, "notebook.not_found", "lock"},
		{"a notebook the caller cannot see", func(f *abandonFake) { f.authErr = shared.ErrNotVisible }, "notebook.not_found", "authorize"},
		{"an editor", func(f *abandonFake) { f.authErr = shared.Forbidden() }, "forbidden", "authorize"},
	}
}

// An upgrade's abandonment (M11/P1 design 3.20), with fakes of its ports:
// each code in its turn, the transaction rolled back, nothing logged, the
// full queue's server_busy, the job's start, the last; each refusal before
// it answers its own code with the queue full too (the S2A-M1 kind).
// Upgrading or reviewing, in order: the notebook's state rolling back, the
// mode's change told, the rollback's job started from the caller's client,
// the module's row rolling back, abandoned, with the job, in one write; the
// answer the row as written, named; an info line "upgrade abandoned". A
// rollback stopped is continued: no job queued or running, a new job, only
// the row's job changed, the change told from standard to standard, "upgrade
// rollback continued". A row not under way is a defect.
func TestAbandonUpgrade(t *testing.T) {
	refusals := append(lockRefusals(),
		refusal{"no upgrade", func(f *abandonFake) { f.state.Upgrade = app.NotebookUpgrade{} }, "wiki.upgrade_state", "clock"},
		refusal{"a rollback's job queued or running", func(f *abandonFake) {
			f.state.Upgrade.State, f.row.State, f.writing = domain.UpgradeRollingBack, domain.UpgradeRollingBack, true
		}, "transfer.busy", "writing"},
	)
	for _, tt := range refusals {
		for _, full := range []bool{false, true} {
			x := newAbandonFixture(func(f *abandonFake) {
				tt.change(f)
				if full {
					f.startErr = shared.ServerBusy(time.Second)
				}
			})
			_, err := app.NewAbandonUpgrade(x.d).Execute(x.ctx(), x.nb)
			if codeOf(err) != tt.code || !refusedAfter(x.f.calls, tt.last) || len(x.log.lines(t)) != 0 {
				t.Errorf("%s, the queue full %v: %v after %v; want %s after %s, rolled back, nothing logged", tt.name, full, err, x.f.calls,
					tt.code, tt.last)
			}
		}
	}
	for _, state := range []domain.UpgradeState{domain.UpgradeUpgrading, domain.UpgradeRollingBack} {
		x := newAbandonFixture(func(f *abandonFake) { f.startErr = shared.ServerBusy(time.Second) })
		if state == domain.UpgradeRollingBack {
			x.rollingBack(uuid.NewV7())
		}
		_, err := app.NewAbandonUpgrade(x.d).Execute(x.ctx(), x.nb)
		if codeOf(err) != "server_busy" || x.f.calls[len(x.f.calls)-1] != "rollback" || len(x.log.lines(t)) != 0 {
			t.Errorf("%s with the queue full: %v after %v; want server_busy, rolled back", state, err, x.f.calls)
		}
	}

	for _, state := range []domain.UpgradeState{domain.UpgradeUpgrading, domain.UpgradeReviewing} {
		x := newAbandonFixture(func(f *abandonFake) { f.state.Upgrade.State, f.row.State = state, state })
		view, err := app.NewAbandonUpgrade(x.d).Execute(x.ctx(), x.nb)
		if err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		at := x.f.at.Format(time.TimeOnly)
		want := []string{"workspace", "begin", "share", "lock", "authorize", "clock",
			fmt.Sprintf("set state rolling_back of %s by %s at %s", x.up, x.alice, at), "publish",
			fmt.Sprintf("start rollback of %s from web", x.up),
			fmt.Sprintf("roll back %s abandoned with %s by %s at %s", x.up, x.f.job, x.alice, at), "current", "names", "commit"}
		if !slices.Equal(x.f.calls, want) {
			t.Errorf("%s: calls\n%v\nwant\n%v", state, x.f.calls, want)
		}
		if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard,
			By: x.alice, At: x.f.at}}; !slices.Equal(x.f.published, want) {
			t.Errorf("%s: published %+v, want %+v", state, x.f.published, want)
		}
		up := view.Upgrade
		if up.State != domain.UpgradeRollingBack || up.Ending != domain.UpgradeAbandoned || up.RollbackJobID == nil ||
			*up.RollbackJobID != x.f.job || view.StartedByName != "name of "+x.starter.String() {
			t.Errorf("%s: answered %+v, want the row rolling back, abandoned, with the job, named", state, view)
		}
		lines := x.log.lines(t)
		if len(lines) != 1 || lines[0]["msg"] != "upgrade abandoned" || lines[0]["level"] != "INFO" ||
			lines[0]["workspace_id"] != x.f.ws.String() || lines[0]["notebook_id"] != x.nb.String() ||
			lines[0]["upgrade_id"] != x.up.String() || lines[0]["user_id"] != x.alice.String() || lines[0]["job_id"] != x.f.job.String() {
			t.Errorf("%s: logged %v, want one info line upgrade abandoned", state, lines)
		}
	}

	x := newAbandonFixture(nil)
	x.rollingBack(uuid.NewV7())
	api := shared.WithActor(context.Background(), shared.Actor{UserID: x.alice, APITokenID: uuid.NewV7()})
	view, err := app.NewAbandonUpgrade(x.d).Execute(api, x.nb)
	if err != nil {
		t.Fatal(err)
	}
	at := x.f.at.Format(time.TimeOnly)
	want := []string{"workspace", "begin", "share", "lock", "authorize", "clock", "writing", fmt.Sprintf("start rollback of %s from api", x.up),
		fmt.Sprintf("continue %s with %s by %s at %s", x.up, x.f.job, x.alice, at), "publish", "current", "names", "commit"}
	if !slices.Equal(x.f.calls, want) {
		t.Errorf("a continuation's calls\n%v\nwant\n%v", x.f.calls, want)
	}
	if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard,
		By: x.alice, At: x.f.at}}; !slices.Equal(x.f.published, want) {
		t.Errorf("a continuation published %+v, want %+v", x.f.published, want)
	}
	if up := view.Upgrade; up.State != domain.UpgradeRollingBack || up.Ending != domain.UpgradeAbandoned || *up.RollbackJobID != x.f.job {
		t.Errorf("a continuation answered %+v, want the row rolling back with the new job", up)
	}
	if lines := x.log.lines(t); len(lines) != 1 || lines[0]["msg"] != "upgrade rollback continued" ||
		lines[0]["job_id"] != x.f.job.String() || lines[0]["upgrade_id"] != x.up.String() {
		t.Errorf("a continuation logged %v, want one info line upgrade rollback continued", lines)
	}

	for _, state := range []domain.UpgradeState{domain.UpgradeUpgrading, domain.UpgradeRollingBack} {
		x := newAbandonFixture(func(f *abandonFake) { f.noRow = true })
		if state == domain.UpgradeRollingBack {
			x.rollingBack(uuid.NewV7())
		}
		_, err := app.NewAbandonUpgrade(x.d).Execute(x.ctx(), x.nb)
		var coded *shared.Error
		if err == nil || errors.As(err, &coded) || x.f.calls[len(x.f.calls)-1] != "rollback" {
			t.Errorf("%s without its row: %v after %v; want a defect's error, rolled back", state, err, x.f.calls)
		}
	}
	if _, err := app.NewAbandonUpgrade(x.d).Execute(context.Background(), x.nb); err == nil {
		t.Error("an abandonment without an actor = nil, want an error")
	}
}

// A rollback's end (M11/P1 design 3.20), with fakes of its ports: each
// code in its turn, rolled back, nothing logged; then, in order, the row
// read, what is left read now, the notebook's upgrade cleared, its
// baseline deleted, the row rolled back, complete when nothing was left,
// else what was left (decision 230), the change told; the answer the row
// rolled back; an info line "upgrade rollback ended" with the three counts
// and whether it was complete. A row no longer rolling back is a defect.
func TestEndUpgradeRollback(t *testing.T) {
	rolling := func(f *abandonFake) {
		old := uuid.NewV7()
		f.state.Upgrade.State, f.row.State, f.row.Ending, f.row.RollbackJobID = domain.UpgradeRollingBack, domain.UpgradeRollingBack,
			domain.UpgradeAbandoned, &old
	}
	refusals := append(lockRefusals(),
		refusal{"no upgrade", func(f *abandonFake) { f.state.Upgrade = app.NotebookUpgrade{} }, "wiki.upgrade_state", "clock"},
		refusal{"upgrading", func(*abandonFake) {}, "wiki.upgrade_state", "clock"},
		refusal{"reviewing", func(f *abandonFake) { f.state.Upgrade.State = domain.UpgradeReviewing }, "wiki.upgrade_state", "clock"},
		refusal{"its job queued or running", func(f *abandonFake) {
			rolling(f)
			f.writing = true
		}, "transfer.busy", "writing"},
	)
	for _, tt := range refusals {
		x := newAbandonFixture(tt.change)
		_, err := app.NewEndUpgradeRollback(x.d).Execute(x.ctx(), x.nb)
		if codeOf(err) != tt.code || !refusedAfter(x.f.calls, tt.last) || len(x.log.lines(t)) != 0 {
			t.Errorf("%s: %v after %v; want %s after %s, rolled back, nothing logged", tt.name, err, x.f.calls, tt.code, tt.last)
		}
	}

	for _, left := range []domain.RollbackLeft{{}, {Nodes: 1, Pages: 2, Created: 3}} {
		x := newAbandonFixture(func(f *abandonFake) {
			rolling(f)
			f.left = left
		})
		view, err := app.NewEndUpgradeRollback(x.d).Execute(x.ctx(), x.nb)
		if err != nil {
			t.Fatalf("%+v left: %v", left, err)
		}
		complete := left == (domain.RollbackLeft{})
		short := "<nil>"
		if !complete {
			short = "&{1 2 3}"
		}
		at := x.f.at.Format(time.TimeOnly)
		want := []string{"workspace", "begin", "share", "lock", "authorize", "clock", "writing", "current", "names",
			fmt.Sprintf("left of %s", x.up), fmt.Sprintf("clear %s by %s at %s", x.up, x.alice, at), fmt.Sprintf("delete baseline of %s", x.up),
			fmt.Sprintf("end row %s left %s by %s at %s", x.up, short, x.alice, at), "publish", "commit"}
		if !slices.Equal(x.f.calls, want) {
			t.Errorf("%+v left: calls\n%v\nwant\n%v", left, x.f.calls, want)
		}
		if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard,
			By: x.alice, At: x.f.at}}; !slices.Equal(x.f.published, want) {
			t.Errorf("%+v left: published %+v, want %+v", left, x.f.published, want)
		}
		up := view.Upgrade
		if up.State != domain.UpgradeRolledBack || up.RollbackComplete == nil || *up.RollbackComplete != complete ||
			complete != (up.RollbackLeft == nil) || !complete && *up.RollbackLeft != left || up.Ending != domain.UpgradeAbandoned {
			t.Errorf("%+v left: answered %+v, want rolled back, complete %v", left, up, complete)
		}
		lines := x.log.lines(t)
		if len(lines) != 1 || lines[0]["msg"] != "upgrade rollback ended" || lines[0]["level"] != "INFO" ||
			lines[0]["upgrade_id"] != x.up.String() || lines[0]["notebook_id"] != x.nb.String() ||
			lines[0]["workspace_id"] != x.f.ws.String() || lines[0]["user_id"] != x.alice.String() ||
			lines[0]["nodes"] != float64(left.Nodes) || lines[0]["pages"] != float64(left.Pages) ||
			lines[0]["created"] != float64(left.Created) || lines[0]["complete"] != complete {
			t.Errorf("%+v left: logged %v, want one info line upgrade rollback ended", left, lines)
		}
	}

	x := newAbandonFixture(func(f *abandonFake) {
		rolling(f)
		f.noRow = true
	})
	_, err := app.NewEndUpgradeRollback(x.d).Execute(x.ctx(), x.nb)
	var coded *shared.Error
	if err == nil || errors.As(err, &coded) || slices.Contains(x.f.calls, "publish") || x.f.calls[len(x.f.calls)-1] != "rollback" {
		t.Errorf("an end without its row rolling back: %v after %v; want a defect's error, rolled back, nothing told", err, x.f.calls)
	}
	if _, err := app.NewEndUpgradeRollback(x.d).Execute(context.Background(), x.nb); err == nil {
		t.Error("an end without an actor = nil, want an error")
	}
}

// The read of an upgrade (M11/P1 design 3.20): rolling back with no job
// queued or running, it tells what the rollback has left, read now; with
// one, or upgrading, it reads neither; the row, the jobs and what is left
// in one snapshot (M11/P1 S3 review A-M2); none, wiki.upgrade_not_found.
func TestGetUpgradeTellsWhatAStoppedRollbackLeft(t *testing.T) {
	left := domain.RollbackLeft{Nodes: 4, Pages: 5, Created: 6}
	for _, tt := range []struct {
		name    string
		state   domain.UpgradeState
		writing bool
		want    *domain.RollbackLeft
		calls   string
	}{
		{"stopped", domain.UpgradeRollingBack, false, &left, "workspace, authorize, snapshot, current, writing, left, end snapshot, names"},
		{"its job running", domain.UpgradeRollingBack, true, nil, "workspace, authorize, snapshot, current, writing, end snapshot, names"},
		{"upgrading", domain.UpgradeUpgrading, false, nil, "workspace, authorize, snapshot, current, end snapshot, names"},
	} {
		x := newAbandonFixture(func(f *abandonFake) {
			f.row.State, f.writing, f.left = tt.state, tt.writing, left
		})
		view, err := app.NewGetUpgrade(x.d).Execute(x.ctx(), x.nb)
		calls := strings.ReplaceAll(strings.Join(x.f.calls, ", "), "left of "+x.up.String(), "left")
		if err != nil || calls != tt.calls || (view.Upgrade.RollbackLeft == nil) != (tt.want == nil) ||
			tt.want != nil && *view.Upgrade.RollbackLeft != *tt.want {
			t.Errorf("%s: %+v, %v after %s; want left %+v after %s", tt.name, view.Upgrade.RollbackLeft, err, calls, tt.want, tt.calls)
		}
	}
	x := newAbandonFixture(func(f *abandonFake) { f.row = domain.Upgrade{} })
	if _, err := app.NewGetUpgrade(x.d).Execute(x.ctx(), x.nb); !errors.Is(err, domain.ErrUpgradeNotFound) ||
		strings.Join(x.f.calls, ", ") != "workspace, authorize, snapshot, current, end snapshot" {
		t.Errorf("none under way: %v after %v; want wiki.upgrade_not_found after the snapshot's read", err, x.f.calls)
	}
}
