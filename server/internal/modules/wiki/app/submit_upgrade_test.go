package app_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// unsubmitted are UpgradeRows' writes of a submission, which the fakes of
// the use cases that never submit have: each a defect.
type unsubmitted struct{}

var errUnsubmitted = errors.New("a use case that does not submit wrote a submission")

func (unsubmitted) Submit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time, time.Time) (bool, error) {
	return false, errUnsubmitted
}

func (unsubmitted) KeepReport(context.Context, uuid.UUID, uuid.UUID, domain.UpgradeReport, uuid.UUID, time.Time) (bool, error) {
	return false, errUnsubmitted
}

func (unsubmitted) Unsubmit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errUnsubmitted
}

type submitTxKey struct{}

// submitFake is every port of an upgrade's submission (M11/P2 design
// 3.12): the check's (checkFake's), the workspace's and the notebook's
// locks and states, and the wiki row, which the writes change as the SQL
// would (Submit only from upgrading or a submission taken over, the other
// two only of the row's submission, reviewing, without a report), a
// transaction's writes undone when it fails. Each write is traced with
// its transaction's number. Hooks: before the second transaction, in the
// run, at a transaction's COMMIT, and a second transaction whose first
// lock waits for its deadline.
type submitFake struct {
	*checkFake
	row         domain.Upgrade
	noRow       bool
	lockGone    bool // the second transaction locks no notebook
	blockSecond bool // the second transaction's first lock waits for its deadline
	writes      []string
	tx          int
	takenOver   time.Time

	beforeSecond func(f *submitFake)
	duringRun    func(ctx context.Context)
	commit       func(ctx context.Context, n int) error
}

func newSubmitFake() *submitFake {
	c := newCheckFake()
	c.at = time.Now().UTC()
	return &submitFake{checkFake: c, row: c.up}
}

// passing has f's check pass: no page misses its type, every node kept.
func (f *submitFake) passing() *submitFake {
	f.clean = true
	f.preserved = domain.Preserved{Nodes: 3, NodesKept: 3, Pages: 3, PagesKept: 3}
	return f
}

// at is the submission's moment, f's clock to the microsecond.
func (f *submitFake) at() time.Time { return f.checkFake.at.Truncate(time.Microsecond) }

func (f *submitFake) write(ctx context.Context, what string) {
	n, _ := ctx.Value(submitTxKey{}).(int)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, fmt.Sprint(n, " ", what))
}

func (f *submitFake) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.writes)
}

func (f *submitFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.mu.Lock()
	f.tx++
	n, before := f.tx, f.row
	f.mu.Unlock()
	if n == 2 && f.beforeSecond != nil {
		f.beforeSecond(f)
		before = f.row
	}
	err := fn(context.WithValue(ctx, submitTxKey{}, n))
	if err == nil && f.commit != nil {
		err = f.commit(ctx, n)
	}
	if err != nil {
		f.mu.Lock()
		f.row = before
		f.mu.Unlock()
	}
	return err
}

func (f *submitFake) ShareByID(ctx context.Context, _ uuid.UUID) (bool, error) {
	if n, _ := ctx.Value(submitTxKey{}).(int); n == 2 && f.blockSecond {
		<-ctx.Done()
		return false, fmt.Errorf("lock the workspace: %w", ctx.Err())
	}
	return true, nil
}

func (f *submitFake) Lock(ctx context.Context, _ uuid.UUID) (app.UpgradeNotebook, bool, error) {
	if n, _ := ctx.Value(submitTxKey{}).(int); n == 2 && f.lockGone {
		return app.UpgradeNotebook{}, false, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	nb := app.UpgradeNotebook{Mode: domain.ModeStandard}
	if !f.noRow {
		nb.Upgrade = app.NotebookUpgrade{ID: f.row.ID, State: f.row.State}
	}
	return nb, true, nil
}

func (f *submitFake) Start(context.Context, uuid.UUID, app.NotebookUpgrade) error {
	return errors.New("no start")
}

func (f *submitFake) SetState(ctx context.Context, _, _ uuid.UUID, state domain.UpgradeState, _ uuid.UUID, _ time.Time) error {
	f.write(ctx, "SetState "+string(state))
	return nil
}

func (f *submitFake) Clear(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
	return errors.New("no clear")
}

func (f *submitFake) Publish(ctx context.Context, c app.ModeChange) error {
	if c.From != domain.ModeStandard || c.To != domain.ModeStandard {
		return errors.New("a submission changes no mode")
	}
	f.write(ctx, "Publish")
	return nil
}

// Authorize is the check's decisions, an expired ctx's an error: a
// decision after the COMMIT, past the deadline, fails.
func (f *submitFake) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	if err := ctx.Err(); err != nil {
		return shared.Grant{}, err
	}
	return f.checkFake.Authorize(ctx, actor, action, t)
}

func (f *submitFake) Current(ctx context.Context, _ uuid.UUID) (domain.Upgrade, bool, error) {
	f.note(ctx, "Current")
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.row, !f.noRow, nil
}

func (f *submitFake) Submit(ctx context.Context, id, submissionID, by uuid.UUID, at, takenOver time.Time) (bool, error) {
	f.mu.Lock()
	r := &f.row
	ok := id == r.ID && (r.State == domain.UpgradeUpgrading || r.State == domain.UpgradeReviewing && r.Report == nil &&
		r.SubmittedAt != nil && !r.SubmittedAt.After(takenOver))
	if ok {
		r.State, r.SubmissionID, r.SubmittedBy, r.SubmittedAt = domain.UpgradeReviewing, &submissionID, &by, &at
		f.takenOver = takenOver
	}
	f.mu.Unlock()
	f.write(ctx, "Submit")
	return ok, nil
}

func (f *submitFake) mine(id, submissionID uuid.UUID) bool {
	r := f.row
	return id == r.ID && r.State == domain.UpgradeReviewing && r.SubmissionID != nil && *r.SubmissionID == submissionID &&
		r.Report == nil
}

func (f *submitFake) KeepReport(ctx context.Context, id, submissionID uuid.UUID, report domain.UpgradeReport, _ uuid.UUID, _ time.Time,
) (bool, error) {
	f.mu.Lock()
	ok := f.mine(id, submissionID)
	if ok {
		f.row.Report = &report
	}
	f.mu.Unlock()
	f.write(ctx, "KeepReport")
	return ok, nil
}

func (f *submitFake) Unsubmit(ctx context.Context, id, submissionID, _ uuid.UUID, _ time.Time) (bool, error) {
	f.mu.Lock()
	ok := f.mine(id, submissionID)
	if ok {
		f.row.State, f.row.SubmissionID, f.row.SubmittedBy, f.row.SubmittedAt = domain.UpgradeUpgrading, nil, nil, nil
	}
	f.mu.Unlock()
	f.write(ctx, "Unsubmit")
	return ok, nil
}

func (f *submitFake) Do(ctx context.Context, key app.RunKey, run func(context.Context) (app.RunOutput, error)) (app.RunResult, error) {
	if f.duringRun != nil {
		f.duringRun(ctx)
	}
	return f.checkFake.Do(ctx, key, run)
}

// newSubmit is the use case over f, of SubmitTimeout timeout, logging to
// log, and the caller's ctx.
func newSubmit(f *submitFake, timeout time.Duration, log *startLog) (*app.SubmitUpgrade, context.Context) {
	logger := slog.New(slog.NewJSONHandler(log, nil))
	lint := app.NewLint(app.LintDeps{Notebooks: f, Sources: f, Nodes: f, Rules: f, Cursors: f, Paths: f, Snapshots: f, Authorizer: f,
		Clock: f, Runs: f})
	d := app.UpgradeDeps{Tx: f, Snapshots: f, Notebooks: f, Workspaces: f, States: f, Rows: f, Names: f, Authorizer: f,
		Action: "notebook.change_mode", Clock: f, Logger: logger, Gates: f, Cursors: f, Paths: f,
		ReservedKeys: []string{"index", "log", "index.md", "log.md"}, SubmitTimeout: timeout}
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	return app.NewSubmitUpgrade(d, app.NewCheckUpgrade(d, lint, f), f), ctx
}

// submitLines are log's lines of msg.
func submitLines(t *testing.T, log *startLog, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range log.lines(t) {
		if l["msg"] == msg {
			out = append(out, l)
		}
	}
	return out
}

// The writes of a submission's first transaction.
func firstWrites() []string { return []string{"1 SetState reviewing", "1 Submit", "1 Publish"} }

// The submission answers its codes in the order of M11/P2 design 3.12,
// each cell's writes and mode publications as it answers: the notebook
// and its grant; no upgrade, one rolling back, one reviewing with a
// report; one reviewing without a report submitted less than
// SubmitTimeout ago, a microsecond less, "still being checked", taken
// over at SubmitTimeout exactly; then the check's own error, a stale
// index, gates not passed, AGENTS's alone too, each setting the upgrade
// upgrading again; a passed one keeps its report. Neighbours answer the first.
// Then each gate but index's alone not passed: wiki.upgrade_not_ready
// naming it, the upgrade upgrading again (review A-M1; index's is the
// stale index's cell, which the check answers first).
func TestSubmitUpgradeAnswersInOrder(t *testing.T) {
	const timeout = time.Minute
	type tweak func(f *submitFake)
	var (
		noNotebook tweak = func(f *submitFake) { f.noNotebook = true }
		invisible  tweak = func(f *submitFake) { f.authErrs = []error{shared.ErrNotVisible} }
		notAdmin   tweak = func(f *submitFake) { f.authErrs = []error{shared.Forbidden()} }
		noUpgrade  tweak = func(f *submitFake) { f.noRow = true }
		rolling    tweak = func(f *submitFake) { f.row.State = domain.UpgradeRollingBack }
		reviewed         = func(ago time.Duration, report bool) tweak {
			return func(f *submitFake) {
				sub, by, at := uuid.NewV7(), uuid.NewV7(), f.at().Add(-ago)
				f.row.State, f.row.SubmissionID, f.row.SubmittedBy, f.row.SubmittedAt = domain.UpgradeReviewing, &sub, &by, &at
				if report {
					f.row.Report = &domain.UpgradeReport{Passed: true}
				}
			}
		}
		busy     tweak = func(f *submitFake) { f.runErr = shared.ServerBusy(time.Second) }
		stale    tweak = func(f *submitFake) { f.stale = 1 }
		passing  tweak = func(f *submitFake) { f.passing() }
		bodiless tweak = func(f *submitFake) { f.roots.AgentsBody = false }
	)
	upgrading := append(firstWrites(), "2 SetState upgrading", "2 Unsubmit", "2 Publish")
	stored := append(firstWrites(), "2 KeepReport", "2 Publish")
	submitting := domain.ErrUpgradeSubmitting.Detail
	for _, tc := range []struct {
		name   string
		tweaks []tweak
		code   string
		detail string
		writes []string
	}{
		{"no notebook", []tweak{noNotebook}, "notebook.not_found", "", nil},
		{"a notebook not visible", []tweak{invisible}, "notebook.not_found", "", nil},
		{"no admin", []tweak{notAdmin}, "forbidden", "", nil},
		{"no upgrade", []tweak{noUpgrade}, "wiki.upgrade_state", "", nil},
		{"an upgrade rolling back", []tweak{rolling}, "wiki.upgrade_state", "", nil},
		{"reviewing with a report", []tweak{reviewed(time.Hour, true)}, "wiki.upgrade_state", domain.ErrUpgradeState.Detail, nil},
		{"reviewing, submitted a microsecond less than SubmitTimeout ago", []tweak{reviewed(timeout-time.Microsecond, false)},
			"wiki.upgrade_state", submitting, nil},
		{"reviewing, submitted SubmitTimeout ago: taken over", []tweak{reviewed(timeout, false), passing}, "", "", stored},
		{"no slot", []tweak{busy}, "server_busy", "", upgrading},
		{"a stale index", []tweak{stale}, "wiki.upgrade_stale_index", "", upgrading},
		{"gates not passed", nil, "wiki.upgrade_not_ready",
			"These gates did not pass: frontmatter, preserved. The upgrade is upgrading again: check it, then submit it again.", upgrading},
		{"AGENTS alone not passed", []tweak{passing, bodiless}, "wiki.upgrade_not_ready",
			"These gates did not pass: agents. The upgrade is upgrading again: check it, then submit it again.", upgrading},
		{"passed", []tweak{passing}, "", "", stored},
		{"a notebook not visible without an upgrade", []tweak{noUpgrade, invisible}, "notebook.not_found", "", nil},
		{"no admin of an upgrade rolling back", []tweak{rolling, notAdmin}, "forbidden", "", nil},
		{"a report before a submission still checked", []tweak{reviewed(time.Microsecond, true)}, "wiki.upgrade_state",
			domain.ErrUpgradeState.Detail, nil},
		{"still checked, with a stale index", []tweak{reviewed(0, false), stale}, "wiki.upgrade_state", submitting, nil},
		{"no slot with a stale index", []tweak{busy, stale}, "server_busy", "", upgrading},
		{"a stale index with gates not passed", []tweak{stale}, "wiki.upgrade_stale_index", "", upgrading},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubmitFake()
			for _, tw := range tc.tweaks {
				tw(f)
			}
			submit, ctx := newSubmit(f, timeout, &startLog{})
			view, err := submit.Execute(ctx, f.nb)
			got := ""
			if err != nil {
				got = codeOf(err)
			}
			if got != tc.code {
				t.Fatalf("answered %v (%s), want %q; writes %v", err, got, tc.code, f.written())
			}
			var e *shared.Error
			if tc.detail != "" && (!errors.As(err, &e) || e.Detail != tc.detail) {
				t.Errorf("detail %q, want %q", e.Detail, tc.detail)
			}
			if w := f.written(); !slices.Equal(w, tc.writes) {
				t.Errorf("wrote %v, want %v", w, tc.writes)
			}
			if tc.code == "" && (view.Upgrade.State != domain.UpgradeReviewing || view.Upgrade.Report == nil ||
				*view.Upgrade.SubmissionID != *f.row.SubmissionID) {
				t.Errorf("answered %+v, want it reviewing with its report", view.Upgrade)
			}
		})
	}
	for _, tc := range []struct {
		gate  domain.UpgradeGateKey
		tweak tweak
	}{
		{domain.GateAgents, bodiless},
		{domain.GateSourceRoot, func(f *submitFake) { f.roots.AgentsID = &f.root }},
		{domain.GateReservedTitles, func(f *submitFake) {
			f.extra = []app.CompactNode{{ID: uuid.NewV7(), Kind: domain.NodePage, NameKey: "index"}}
		}},
		{domain.GateFrontmatter, func(f *submitFake) { f.clean = false }},
		{domain.GateSources, func(f *submitFake) { f.counts = map[domain.LintRule]int64{domain.RuleInvalidSources: 1} }},
		{domain.GateLinks, func(f *submitFake) { f.moreWanted = 1 }},
		{domain.GatePreserved, func(f *submitFake) {
			f.preserved = domain.Preserved{Nodes: 3, NodesKept: 3, Pages: 3, PagesKept: 2, NotKept: []uuid.UUID{f.page}}
		}},
	} {
		t.Run(string(tc.gate)+" alone not passed", func(t *testing.T) {
			f := newSubmitFake().passing()
			tc.tweak(f)
			submit, ctx := newSubmit(f, timeout, &startLog{})
			_, err := submit.Execute(ctx, f.nb)
			want := "These gates did not pass: " + string(tc.gate) + ". The upgrade is upgrading again: check it, then submit it again."
			if e := (*shared.Error)(nil); codeOf(err) != "wiki.upgrade_not_ready" || !errors.As(err, &e) || e.Detail != want {
				t.Fatalf("answered %v, want wiki.upgrade_not_ready naming %s alone", err, tc.gate)
			}
			if w := f.written(); !slices.Equal(w, upgrading) {
				t.Errorf("wrote %v, want %v", w, upgrading)
			}
		})
	}
	// The takeover's bound is the one the first transaction judged.
	f := newSubmitFake().passing()
	reviewed(timeout, false)(f)
	submit, ctx := newSubmit(f, timeout, &startLog{})
	if _, err := submit.Execute(ctx, f.nb); err != nil || !f.takenOver.Equal(f.at().Add(-timeout)) {
		t.Fatalf("taken over %v by %s, want at %s", err, f.takenOver, f.at().Add(-timeout))
	}
}

// The submission's check reads the upgrade again in its snapshot, which
// must still be this submission's, reviewing without a report (M11/P2
// design 3.12 step 4; review A-M3): one rolling back, another submission's
// or one reported while its run queued reads no gate and is
// wiki.upgrade_state; the second transaction writes nothing, superseded.
func TestTheSubmissionsSnapshotIsItsOwn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(up *domain.Upgrade)
	}{
		{"rolling back", func(up *domain.Upgrade) { up.State = domain.UpgradeRollingBack }},
		{"another submission", func(up *domain.Upgrade) { other := uuid.NewV7(); up.SubmissionID = &other }},
		{"a report kept", func(up *domain.Upgrade) { up.Report = &domain.UpgradeReport{Passed: true} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubmitFake().passing()
			f.duringRun = func(context.Context) {
				f.mu.Lock()
				defer f.mu.Unlock()
				tc.change(&f.row)
			}
			log := &startLog{}
			submit, ctx := newSubmit(f, time.Hour, log)
			if _, err := submit.Execute(ctx, f.nb); codeOf(err) != "wiki.upgrade_state" {
				t.Fatalf("answered %v, want wiki.upgrade_state", err)
			}
			if w := f.written(); !slices.Equal(w, firstWrites()) {
				t.Errorf("wrote %v, want the first transaction's alone", w)
			}
			if lines := submitLines(t, log, "upgrade submitted"); len(lines) != 1 || lines[0]["outcome"] != "superseded" {
				t.Errorf("logged %v, want one line, superseded", lines)
			}
			for _, call := range f.trace() {
				if call == "Roots@" || call == "prepare@" || call == "Preserved@" {
					t.Errorf("the check read %s on an upgrade not this submission's: %v", call, f.trace())
				}
			}
		})
	}
}

// The second part runs apart from the request (M11/P2 design 3.12 step
// 3): a request cancelled while its check runs still keeps the report.
// Its deadline is the earlier of the submission's moment, to the
// microsecond, plus SubmitTimeout and the request's.
func TestTheSubmissionDetaches(t *testing.T) {
	f := newSubmitFake().passing()
	submit, ctx := newSubmit(f, time.Hour, &startLog{})
	ctx, cancel := context.WithCancel(ctx)
	f.duringRun = func(context.Context) { cancel() }
	view, err := submit.Execute(ctx, f.nb)
	if err != nil || view.Upgrade.Report == nil || !slices.Equal(f.written(), append(firstWrites(), "2 KeepReport", "2 Publish")) {
		t.Fatalf("a request cancelled in the check = %+v, %v; wrote %v", view, err, f.written())
	}

	for _, tc := range []struct {
		name    string
		request time.Duration
	}{{"the request's later", 2 * time.Hour}, {"the request's earlier", 30 * time.Minute}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubmitFake().passing()
			f.checkFake.at = f.checkFake.at.Add(123 * time.Nanosecond)
			submit, ctx := newSubmit(f, time.Hour, &startLog{})
			ctx, cancel := context.WithTimeout(ctx, tc.request)
			defer cancel()
			if _, err := submit.Execute(ctx, f.nb); err != nil {
				t.Fatal(err)
			}
			want, _ := ctx.Deadline()
			if tc.request > time.Hour {
				want = f.at().Add(time.Hour)
			}
			got, ok := f.runCtx[0].Deadline()
			if !ok || !got.Equal(want) || !f.row.SubmittedAt.Equal(f.at()) {
				t.Fatalf("the check's deadline %s %t, want %s; submitted at %s", got, ok, want, f.row.SubmittedAt)
			}
		})
	}
}

// The second transaction writes only its own submission's end (M11/P2
// design 3.12 step 5): a submission taken over, an upgrade rolling back,
// a report another kept, each read under the lock, write nothing and
// answer wiki.upgrade_state; a notebook no longer there writes nothing
// and answers notebook.not_found. Each is superseded.
func TestTheSecondTransactionWritesOnlyItsOwn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(f *submitFake)
		code   string
	}{
		{"another submission", func(f *submitFake) { other := uuid.NewV7(); f.row.SubmissionID = &other }, "wiki.upgrade_state"},
		{"rolling back", func(f *submitFake) { f.row.State = domain.UpgradeRollingBack }, "wiki.upgrade_state"},
		{"a report kept", func(f *submitFake) { f.row.Report = &domain.UpgradeReport{} }, "wiki.upgrade_state"},
		{"another upgrade", func(f *submitFake) { f.row.ID = uuid.NewV7() }, "wiki.upgrade_state"},
		{"no notebook", func(f *submitFake) { f.lockGone = true }, "notebook.not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubmitFake().passing()
			f.beforeSecond = tc.change
			log := &startLog{}
			submit, ctx := newSubmit(f, time.Hour, log)
			if _, err := submit.Execute(ctx, f.nb); codeOf(err) != tc.code {
				t.Fatalf("answered %v, want %s", err, tc.code)
			}
			if w := f.written(); !slices.Equal(w, firstWrites()) {
				t.Fatalf("wrote %v, want the first transaction's only", w)
			}
			if lines := submitLines(t, log, "upgrade submitted"); len(lines) != 1 || lines[0]["outcome"] != "superseded" {
				t.Fatalf("logged %v", lines)
			}
		})
	}
}

// Past its deadline (M11/P2 design 3.12 step 6): a second transaction
// that waits on a lock past it answers server_busy and a warn line, writes
// nothing, whether the request is alive (SubmitTimeout first) or expired
// with it (the request's deadline first). A COMMIT that returns past it
// answers as its transaction decided: the decision came before it, a
// domain's conflict too.
func TestTheSubmissionsDeadline(t *testing.T) {
	// Its Retry-After is the whole seconds from the clock as it answers to
	// the submission's moment plus SubmitTimeout, at least one (decision
	// 274): the clock moved on as the check runs.
	for _, tc := range []struct {
		name                      string
		timeout, request, elapsed time.Duration
		retry                     time.Duration
	}{
		{"the request alive", 50 * time.Millisecond, 0, 0, time.Second},
		{"the request expired", time.Hour, 50 * time.Millisecond, 0, time.Hour},
		{"the request expired 7.5 s in", 10 * time.Second, 50 * time.Millisecond, 7500 * time.Millisecond, 3 * time.Second},
		{"the request expired as it may be taken over", 10 * time.Second, 50 * time.Millisecond, 10 * time.Second, time.Second},
		{"the request expired past that", 10 * time.Second, 50 * time.Millisecond, 12 * time.Second, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubmitFake().passing()
			f.blockSecond = true
			f.duringRun = func(context.Context) { f.checkFake.at = f.at().Add(tc.elapsed) }
			log := &startLog{}
			submit, ctx := newSubmit(f, tc.timeout, log)
			if tc.request > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.request)
				defer cancel()
			}
			_, err := submit.Execute(ctx, f.nb)
			if e := (*shared.Error)(nil); codeOf(err) != "server_busy" || !errors.As(err, &e) || e.RetryAfter() != tc.retry {
				t.Fatalf("answered %v, want server_busy, Retry-After %s", err, tc.retry)
			}
			if w := f.written(); !slices.Equal(w, firstWrites()) || f.row.State != domain.UpgradeReviewing || f.row.Report != nil {
				t.Fatalf("wrote %v, the row %+v; want it reviewing without a report", w, f.row)
			}
			warn := submitLines(t, log, "upgrade submission timed out")
			if len(warn) != 1 || warn[0]["level"] != "WARN" || warn[0]["timeout_ms"].(float64) > 50 ||
				warn[0]["submission_id"] != f.row.SubmissionID.String() || len(submitLines(t, log, "upgrade submitted")) != 0 {
				t.Fatalf("logged %v", log.lines(t))
			}
		})
	}
	for _, tc := range []struct {
		name string
		pass bool
		code string
	}{{"a report kept", true, ""}, {"gates not passed", false, "wiki.upgrade_not_ready"}} {
		t.Run("a COMMIT past the deadline, "+tc.name, func(t *testing.T) {
			f := newSubmitFake()
			if tc.pass {
				f.passing()
			}
			f.commit = func(ctx context.Context, n int) error {
				if n == 2 {
					<-ctx.Done()
				}
				return nil
			}
			submit, ctx := newSubmit(f, 50*time.Millisecond, &startLog{})
			_, err := submit.Execute(ctx, f.nb)
			if got := map[bool]string{true: codeOf(err), false: ""}[err != nil]; got != tc.code {
				t.Fatalf("answered %v, want %q", err, tc.code)
			}
		})
	}
}

// The decision before the answer is the second transaction's, before its
// COMMIT, and decides only the answer (M11/P2 design 3.12 step 5, 3.16):
// a submitter no longer an admin gets forbidden, one who no longer sees
// the notebook notebook.not_found, the report kept all the same, an info
// line stored.
func TestTheSubmitterIsDecidedBeforeTheAnswer(t *testing.T) {
	for _, tc := range []struct {
		decision error
		code     string
	}{{shared.Forbidden(), "forbidden"}, {shared.ErrNotVisible, "notebook.not_found"}} {
		f := newSubmitFake().passing()
		f.authErrs = []error{nil, tc.decision}
		log := &startLog{}
		submit, ctx := newSubmit(f, time.Hour, log)
		if _, err := submit.Execute(ctx, f.nb); codeOf(err) != tc.code {
			t.Fatalf("answered %v, want %s", err, tc.code)
		}
		if f.row.Report == nil || !slices.Equal(f.written(), append(firstWrites(), "2 KeepReport", "2 Publish")) {
			t.Fatalf("wrote %v, the row %+v; want the report kept", f.written(), f.row)
		}
		lines := submitLines(t, log, "upgrade submitted")
		if len(lines) != 1 || lines[0]["level"] != "INFO" || lines[0]["outcome"] != "stored" || lines[0]["passed"] != true ||
			lines[0]["upgrade_id"] != f.up.ID.String() || lines[0]["submission_id"] != f.row.SubmissionID.String() ||
			lines[0]["notebook_id"] != f.nb.String() || lines[0]["workspace_id"] != f.ws.String() ||
			fmt.Sprint(lines[0]["failed_gates"]) != "[]" || lines[0]["stale_pages"] != float64(0) {
			t.Fatalf("logged %v", lines)
		}
	}
	// A gate not passed names its keys in the line.
	f := newSubmitFake()
	log := &startLog{}
	submit, ctx := newSubmit(f, time.Hour, log)
	if _, err := submit.Execute(ctx, f.nb); codeOf(err) != "wiki.upgrade_not_ready" {
		t.Fatalf("answered %v", err)
	}
	if lines := submitLines(t, log, "upgrade submitted"); len(lines) != 1 || lines[0]["outcome"] != "not_ready" ||
		fmt.Sprint(lines[0]["failed_gates"]) != "[frontmatter preserved]" || lines[0]["passed"] != false {
		t.Fatalf("logged %v", lines)
	}
}
