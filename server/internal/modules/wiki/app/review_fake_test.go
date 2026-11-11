package app_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type reviewTxKey struct{}

// reviewFake is every port of an upgrade's approval, rejection and
// send-back (M11/P3 design 3.6–3.8): a transaction manager that runs fn,
// rolls back on its error (the rollback failing when told) and commits
// (the COMMIT failing when told); the notebook as state says, the
// module's row as row, which the writes change as the statements would,
// put back by a rollback; the gates' reads; the rebuild, answering hashed
// unless rebuild is set; the logger; the calls in order, each write's with
// what it wrote, the log's lines among them.
type reviewFake struct {
	unsubmitted
	mu    sync.Mutex
	calls []string
	lines []map[string]any

	ws                           uuid.UUID
	noWorkspace, noShare, noLock bool
	state                        app.UpgradeNotebook
	authErr                      error
	at                           time.Time
	row                          domain.Upgrade
	noRow                        bool
	roots                        domain.UpgradeRoots
	reserved                     []app.RootNode
	hashed                       int
	rebuild                      func(ctx context.Context) error
	rowsRefuse                   bool
	job                          uuid.UUID
	startErr                     error
	rollbackErr, commitErr       error
	published                    []app.ModeChange
}

func (f *reviewFake) note(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *reviewFake) noted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// in fails the call outside the transaction.
func (f *reviewFake) in(ctx context.Context, call string) error {
	if ctx.Value(reviewTxKey{}) != true {
		return fmt.Errorf("%s outside the transaction", call)
	}
	return nil
}

func (f *reviewFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.note("begin")
	before := f.row
	if err := fn(context.WithValue(ctx, reviewTxKey{}, true)); err != nil {
		f.note("rollback")
		f.row = before
		if f.rollbackErr != nil {
			return fmt.Errorf("roll back transaction after %v: %w", err, f.rollbackErr) //nolint:errorlint // as the platform's
		}
		return err
	}
	f.note("commit")
	if f.commitErr != nil {
		f.row = before
		return fmt.Errorf("commit transaction: %w", f.commitErr)
	}
	return nil
}

func (f *reviewFake) WithinSnapshot(context.Context, func(context.Context) error) error {
	return errors.New("no snapshot in a review")
}

func (f *reviewFake) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	f.note("workspace")
	return f.ws, !f.noWorkspace, nil
}

func (f *reviewFake) StateOf(context.Context, uuid.UUID) (app.NotebookState, bool, error) {
	return app.NotebookState{}, false, errors.New("read under the lock")
}

func (f *reviewFake) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.note("share")
	if id != f.ws {
		return false, errors.New("another workspace")
	}
	return !f.noShare, f.in(ctx, "share")
}

func (f *reviewFake) Authorize(_ context.Context, _ shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	f.note("authorize")
	if action != "notebook.change_mode" || target.WorkspaceID != f.ws {
		return shared.Grant{}, errors.New("another decision")
	}
	return shared.Grant{}, f.authErr
}

func (f *reviewFake) Now() time.Time {
	f.note("clock")
	return f.at
}

func (f *reviewFake) Current(ctx context.Context, _ uuid.UUID) (domain.Upgrade, bool, error) {
	f.note("current")
	return f.row, !f.noRow, f.in(ctx, "current")
}

func (f *reviewFake) Roots(ctx context.Context, _, rootID uuid.UUID) (domain.UpgradeRoots, error) {
	f.note("roots of %s", rootID)
	return f.roots, f.in(ctx, "roots")
}

func (f *reviewFake) Baseline(context.Context, uuid.UUID) ([]domain.BaselineNode, error) {
	return nil, errors.New("no baseline read")
}

func (f *reviewFake) Preserved(context.Context, uuid.UUID) (domain.Preserved, error) {
	return domain.Preserved{}, errors.New("no gate 7 read")
}

func (f *reviewFake) Changesets(context.Context, uuid.UUID) (int, error) {
	return 0, errors.New("no changesets read")
}

func (f *reviewFake) Capture(context.Context, uuid.UUID, uuid.UUID) (app.UpgradeBaseline, error) {
	return app.UpgradeBaseline{}, errors.New("no capture")
}

func (f *reviewFake) Protect(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return errors.New("no protect")
}

func (f *reviewFake) SetWanted(context.Context, uuid.UUID, map[uuid.UUID]int) error {
	return errors.New("no wanted")
}

func (f *reviewFake) Analyze(context.Context) error { return errors.New("no analyze") }

func (f *reviewFake) ClearExpiredSessions(context.Context, uuid.UUID, time.Time) error {
	return errors.New("no sessions cleared")
}

func (f *reviewFake) LiveSessions(context.Context, uuid.UUID, time.Time) ([]app.SessionHolder, int, error) {
	return nil, 0, errors.New("no sessions read")
}

func (f *reviewFake) RootNodes(ctx context.Context, _ uuid.UUID, keys []string) ([]app.RootNode, error) {
	f.note("root nodes %s", strings.Join(keys, " "))
	return f.reserved, f.in(ctx, "root nodes")
}

func (f *reviewFake) Left(context.Context, uuid.UUID, uuid.UUID) (domain.RollbackLeft, error) {
	return domain.RollbackLeft{}, errors.New("no rollback's end")
}

func (f *reviewFake) DeleteBaseline(ctx context.Context, upgradeID uuid.UUID) error {
	f.note("delete baseline of %s", upgradeID)
	return f.in(ctx, "delete baseline")
}

func (f *reviewFake) Rebuild(ctx context.Context, notebookID, rootID uuid.UUID) (int, error) {
	f.note("rebuild %s under %s", notebookID, rootID)
	if err := f.in(ctx, "rebuild"); err != nil {
		return 0, err
	}
	if f.rebuild != nil {
		return 0, f.rebuild(ctx)
	}
	return f.hashed, nil
}

func (f *reviewFake) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.note("names")
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = "name of " + id.String()
	}
	return out, nil
}

func (f *reviewFake) Insert(context.Context, domain.Upgrade, uuid.UUID, time.Time) error {
	return errors.New("no row inserted")
}

func (f *reviewFake) RollBack(ctx context.Context, id uuid.UUID, ending domain.UpgradeEnding, jobID, by uuid.UUID, at time.Time) (bool, error) {
	f.note("roll back %s %s with %s by %s at %s", id, ending, jobID, by, at.Format(time.RFC3339Nano))
	if f.rowsRefuse {
		return false, nil
	}
	f.row.State, f.row.Ending, f.row.RollbackJobID = domain.UpgradeRollingBack, ending, &jobID
	return true, f.in(ctx, "roll back")
}

func (f *reviewFake) ContinueRollback(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errors.New("no rollback continued")
}

func (f *reviewFake) EndRollback(context.Context, uuid.UUID, *domain.RollbackLeft, uuid.UUID, time.Time) (time.Time, bool, error) {
	return time.Time{}, false, errors.New("no rollback ended")
}

func (f *reviewFake) Approve(ctx context.Context, id, submissionID, by uuid.UUID, at time.Time) (bool, error) {
	f.note("approve row %s of %s by %s at %s", id, submissionID, by, at.Format(time.RFC3339Nano))
	if f.rowsRefuse {
		return false, nil
	}
	f.row.State = domain.UpgradeApproved
	return true, f.in(ctx, "approve row")
}

func (f *reviewFake) SendBack(ctx context.Context, id, by uuid.UUID, at, takenOver time.Time) (bool, error) {
	f.note("send back %s by %s at %s, taken over %s", id, by, at.Format(time.RFC3339Nano), takenOver.Format(time.RFC3339Nano))
	if f.rowsRefuse {
		return false, nil
	}
	f.row.State, f.row.SubmissionID, f.row.SubmittedBy, f.row.SubmittedAt, f.row.Report = domain.UpgradeUpgrading, nil, nil, nil, nil
	return true, f.in(ctx, "send back")
}

// reviewStates are the notebook's states over the fake: its Approve is
// UpgradeStates', the fake's UpgradeRows'.
type reviewStates struct{ f *reviewFake }

func (s reviewStates) Lock(ctx context.Context, _ uuid.UUID) (app.UpgradeNotebook, bool, error) {
	s.f.note("lock")
	return s.f.state, !s.f.noLock, s.f.in(ctx, "lock")
}

func (s reviewStates) Start(context.Context, uuid.UUID, app.NotebookUpgrade) error {
	return errors.New("no upgrade starts")
}

func (s reviewStates) SetState(ctx context.Context, _, upgradeID uuid.UUID, state domain.UpgradeState, by uuid.UUID, at time.Time) error {
	s.f.note("set state %s of %s by %s at %s", state, upgradeID, by, at.Format(time.RFC3339Nano))
	return s.f.in(ctx, "set state")
}

func (s reviewStates) Clear(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
	return errors.New("no upgrade cleared")
}

func (s reviewStates) Approve(ctx context.Context, id, upgradeID, rootID, by uuid.UUID, at time.Time) error {
	s.f.note("approve notebook %s of %s, its root %s, by %s at %s", id, upgradeID, rootID, by, at.Format(time.RFC3339Nano))
	return s.f.in(ctx, "approve notebook")
}

func (s reviewStates) Publish(ctx context.Context, c app.ModeChange) error {
	s.f.note("publish")
	s.f.published = append(s.f.published, c)
	return s.f.in(ctx, "publish")
}

// reviewRollbacks are the transfer module's starts of a rollback, over the
// fake's job, error and calls.
type reviewRollbacks struct{ f *reviewFake }

func (r reviewRollbacks) Start(ctx context.Context, _, upgradeID uuid.UUID, client string) (uuid.UUID, error) {
	r.f.note("start rollback of %s from %s", upgradeID, client)
	if err := r.f.in(ctx, "the rollback's start"); err != nil {
		return uuid.UUID{}, err
	}
	return r.f.job, r.f.startErr
}

// reviewLog is the use cases' logger: each line a call "log LEVEL msg" in
// the fake's order, and its attributes, the level and the message among
// them, kept whole.
type reviewLog struct{ f *reviewFake }

func (l reviewLog) Enabled(context.Context, slog.Level) bool { return true }

func (l reviewLog) Handle(_ context.Context, r slog.Record) error {
	line := map[string]any{"level": r.Level.String(), "msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		line[a.Key] = a.Value.Any()
		return true
	})
	l.f.note("log %s %s", r.Level, r.Message)
	l.f.mu.Lock()
	l.f.lines = append(l.f.lines, line)
	l.f.mu.Unlock()
	return nil
}

func (l reviewLog) WithAttrs([]slog.Attr) slog.Handler { return l }

func (l reviewLog) WithGroup(string) slog.Handler { return l }

// reviewFixture is the use cases over a fake: an admin, alice, reviews the
// upgrade up of notebook nb, reviewing with a report that passed, its
// source root root, started by starter, submitted by submitter as
// submission; the clock at at, nanoseconds and all; SubmitTimeout 17 s.
type reviewFixture struct {
	f                                            *reviewFake
	d                                            app.UpgradeDeps
	nb, up, root, alice, starter, submitter, sub uuid.UUID
	submittedAt                                  time.Time
}

func newReviewFixture(change func(x *reviewFixture)) reviewFixture {
	x := reviewFixture{nb: uuid.NewV7(), up: uuid.NewV7(), root: uuid.NewV7(), alice: uuid.NewV7(), starter: uuid.NewV7(),
		submitter: uuid.NewV7(), sub: uuid.NewV7()}
	agents := uuid.NewV7()
	x.f = &reviewFake{ws: uuid.NewV7(), at: time.Date(2026, 11, 12, 10, 20, 30, 123456789, time.UTC), job: uuid.NewV7(), hashed: 7,
		roots: domain.UpgradeRoots{AgentsID: &agents, AgentsBody: true, RootPage: true}}
	x.submittedAt = x.f.at.Add(-time.Hour).Truncate(time.Microsecond)
	x.f.state = app.UpgradeNotebook{Mode: domain.ModeStandard, Upgrade: app.NotebookUpgrade{ID: x.up, State: domain.UpgradeReviewing,
		By: x.starter}}
	x.f.row = domain.Upgrade{ID: x.up, NotebookID: x.nb, State: domain.UpgradeReviewing, SourceRootID: x.root, SubmissionID: &x.sub,
		SubmittedBy: &x.submitter, SubmittedAt: &x.submittedAt, Report: &domain.UpgradeReport{Passed: true}, CreatedBy: x.starter}
	if change != nil {
		change(&x)
	}
	x.d = app.UpgradeDeps{Tx: x.f, Snapshots: x.f, Notebooks: x.f, Workspaces: x.f, States: reviewStates{x.f}, Pages: x.f,
		Rollback: x.f, Rows: x.f, Rollbacks: reviewRollbacks{x.f}, Names: x.f, Authorizer: x.f, Action: "notebook.change_mode",
		Clock: x.f, Logger: slog.New(reviewLog{x.f}), Gates: x.f, ReservedKeys: []string{"index", "log", "index.md", "log.md"},
		SubmitTimeout: 17 * time.Second, SourceHashes: x.f, ApproveTimeout: time.Hour}
	return x
}

func (x reviewFixture) ctx() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: x.alice, SessionID: uuid.NewV7()})
}

// at is the fixture's clock as the RFC 3339 of the calls.
func (x reviewFixture) at() string { return x.f.at.Format(time.RFC3339Nano) }

// reviewRefusals are the lock's and the decision's refusals, which the
// three use cases share, each with the last call before it.
func reviewRefusals() []struct {
	name   string
	change func(x *reviewFixture)
	code   string
	last   string
} {
	return []struct {
		name   string
		change func(x *reviewFixture)
		code   string
		last   string
	}{
		{"no notebook", func(x *reviewFixture) { x.f.noWorkspace = true }, "notebook.not_found", "workspace"},
		{"no workspace's row", func(x *reviewFixture) { x.f.noShare = true }, "notebook.not_found", "share"},
		{"no notebook's row", func(x *reviewFixture) { x.f.noLock = true }, "notebook.not_found", "lock"},
		{"a notebook the caller cannot see", func(x *reviewFixture) { x.f.authErr = shared.ErrNotVisible }, "notebook.not_found", "authorize"},
		{"an editor", func(x *reviewFixture) { x.f.authErr = shared.Forbidden() }, "forbidden", "authorize"},
		{"no upgrade", func(x *reviewFixture) { x.f.state.Upgrade = app.NotebookUpgrade{} }, "wiki.upgrade_state", "clock"},
		{"upgrading", func(x *reviewFixture) {
			x.f.state.Upgrade.State, x.f.row.State, x.f.row.Report = domain.UpgradeUpgrading, domain.UpgradeUpgrading, nil
		}, "wiki.upgrade_state", "clock"},
		{"rolling back", func(x *reviewFixture) {
			x.f.state.Upgrade.State, x.f.row.State, x.f.row.Ending = domain.UpgradeRollingBack, domain.UpgradeRollingBack, domain.UpgradeAbandoned
		}, "wiki.upgrade_state", "clock"},
	}
}

// reviewWrites are the calls that write, by their prefixes.
func reviewWrites() []string {
	return []string{"set state", "approve notebook", "rebuild", "delete baseline", "approve row", "publish", "start rollback", "roll back",
		"send back"}
}

// wrote is the first call of calls that writes, "" for none.
func wrote(calls []string) string {
	for _, c := range calls {
		for _, w := range reviewWrites() {
			if strings.HasPrefix(c, w) {
				return c
			}
		}
	}
	return ""
}
