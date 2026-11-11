package app_test

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type checkSnapshotKey struct{}

// checkFake is every port of an upgrade's check (M11/P2 design 3.11): the
// notebook, its grants, its upgrade's row (another, in the snapshot, when
// snapUp is set), its page reads, lint's reads (extra nodes at the root
// besides) and rules (counts set, besides missing_type's), a runner that
// runs the worker at once or answers runErr; each call traced, "@" marking
// those in a snapshot, which it counts.
type checkFake struct {
	unsubmitted
	mu        sync.Mutex
	calls     []string
	snapshots int
	keys      []app.RunKey
	runCtx    []context.Context

	nb, ws, root, agents, page uuid.UUID
	at                         time.Time
	noNotebook                 bool
	authErrs                   []error // the decisions' answers in turn, nil after
	authCalls                  int
	up                         domain.Upgrade
	noUp                       bool
	snapUp                     *domain.Upgrade
	roots                      domain.UpgradeRoots
	preserved                  domain.Preserved
	runErr                     error
	paths                      map[uuid.UUID]string
	stale                      int64 // the preparation's stale pages
	clean                      bool  // no page misses its type
	extra                      []app.CompactNode
	counts                     map[domain.LintRule]int64
	moreWanted                 int // the page's wanted targets now beyond its baseline's one
}

func newCheckFake() *checkFake {
	nb, root, agents, page := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f := &checkFake{nb: nb, ws: uuid.NewV7(), root: root, agents: agents, page: page, at: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC),
		up: domain.Upgrade{ID: uuid.NewV7(), NotebookID: nb, State: domain.UpgradeUpgrading, SourceRootID: root, BaselineNodes: 3,
			BaselinePages: 3},
		roots:     domain.UpgradeRoots{AgentsID: &agents, AgentsBody: true, RootPage: true},
		preserved: domain.Preserved{Nodes: 3, NodesKept: 3, Pages: 3, PagesKept: 2, NotKept: []uuid.UUID{page}},
	}
	f.paths = map[uuid.UUID]string{page: "Page.md"}
	return f
}

func (f *checkFake) note(ctx context.Context, call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ctx.Value(checkSnapshotKey{}) == true {
		call += "@"
	}
	f.calls = append(f.calls, call)
}

func (f *checkFake) trace() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *checkFake) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	f.mu.Lock()
	f.snapshots++
	f.mu.Unlock()
	return fn(context.WithValue(ctx, checkSnapshotKey{}, true))
}

func (f *checkFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (f *checkFake) WorkspaceOf(ctx context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	f.note(ctx, "workspace")
	return f.ws, !f.noNotebook, nil
}

func (f *checkFake) StateOf(ctx context.Context, _ uuid.UUID) (app.NotebookState, bool, error) {
	f.note(ctx, "state")
	return app.NotebookState{Mode: domain.ModeStandard}, true, nil
}

func (f *checkFake) Authorize(ctx context.Context, _ shared.Actor, action shared.Action, _ shared.Target) (shared.Grant, error) {
	f.note(ctx, "authorize")
	f.mu.Lock()
	defer f.mu.Unlock()
	if action != "notebook.change_mode" {
		return shared.Grant{}, errors.New("another action")
	}
	f.authCalls++
	if f.authCalls <= len(f.authErrs) {
		return shared.Grant{}, f.authErrs[f.authCalls-1]
	}
	return shared.Grant{}, nil
}

func (f *checkFake) Now() time.Time { return f.at }

func (f *checkFake) Current(ctx context.Context, _ uuid.UUID) (domain.Upgrade, bool, error) {
	f.note(ctx, "Current")
	if ctx.Value(checkSnapshotKey{}) == true && f.snapUp != nil {
		return *f.snapUp, true, nil
	}
	return f.up, !f.noUp, nil
}

func (f *checkFake) Insert(context.Context, domain.Upgrade, uuid.UUID, time.Time) error {
	return errors.New("no insert")
}
func (f *checkFake) RollBack(context.Context, uuid.UUID, domain.UpgradeEnding, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errors.New("no rollback")
}
func (f *checkFake) ContinueRollback(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, errors.New("no rollback")
}
func (f *checkFake) EndRollback(context.Context, uuid.UUID, *domain.RollbackLeft, uuid.UUID, time.Time) (time.Time, bool, error) {
	return time.Time{}, false, errors.New("no rollback")
}

func (f *checkFake) Roots(ctx context.Context, nb, root uuid.UUID) (domain.UpgradeRoots, error) {
	f.note(ctx, "Roots")
	if nb != f.nb || root != f.root {
		return domain.UpgradeRoots{}, errors.New("roots of another notebook or root")
	}
	return f.roots, nil
}

func (f *checkFake) Baseline(ctx context.Context, id uuid.UUID) ([]domain.BaselineNode, error) {
	f.note(ctx, "Baseline")
	if id != f.up.ID {
		return nil, errors.New("another upgrade's baseline")
	}
	return []domain.BaselineNode{{ID: f.root, Kind: domain.NodePage}, {ID: f.page, Kind: domain.NodePage, Wanted: 1},
		{ID: f.agents, Kind: domain.NodePage}}, nil
}

func (f *checkFake) Preserved(ctx context.Context, id uuid.UUID) (domain.Preserved, error) {
	f.note(ctx, "Preserved")
	if id != f.up.ID {
		return domain.Preserved{}, errors.New("another upgrade's rows")
	}
	return f.preserved, nil
}

func (f *checkFake) Changesets(ctx context.Context, id uuid.UUID) (int, error) {
	f.note(ctx, "Changesets")
	return 4, nil
}

func (f *checkFake) LintData(ctx context.Context, _ uuid.UUID, root uuid.UUID) (app.SourceData, error) {
	f.note(ctx, "nodes")
	if root != f.root {
		return app.SourceData{}, errors.New("another root")
	}
	return app.SourceData{Nodes: append([]app.CompactNode{{ID: f.root, Kind: domain.NodePage, NameKey: "raw"},
		{ID: f.agents, Kind: domain.NodePage, NameKey: "agents"}, {ID: f.page, Kind: domain.NodePage, NameKey: "page"}}, f.extra...)}, nil
}

func (f *checkFake) Nodes(ctx context.Context, _ uuid.UUID) ([]app.CompactNode, error) {
	return nil, errors.New("an LLM contract reads its sources' data")
}

func (f *checkFake) PrepareRules(ctx context.Context, in app.ReadInput, q app.RuleQuery) (app.PreparedRules, error) {
	f.note(ctx, "prepare")
	if in.EffectiveMode != domain.ModeLLMWiki || !in.Explicit || in.AreaRootID == nil || *in.AreaRootID != f.root {
		return nil, errors.New("not the upgrade's scope")
	}
	if q.Rule == nil && (!q.WantedByPage || q.AreaWantedByPage) {
		return nil, errors.New("a check asks the first pass's counts only")
	}
	return checkPrepared{f}, nil
}

type checkPrepared struct{ f *checkFake }

func (p checkPrepared) StalePages() int64 { return p.f.stale }
func (p checkPrepared) RuleCount(_ context.Context, rule domain.LintRule) (domain.LintCount, error) {
	if n, ok := p.f.counts[rule]; ok {
		return domain.LintCount{Value: n}, nil
	}
	if rule == domain.RuleMissingType && !p.f.clean {
		return domain.LintCount{Value: 1}, nil
	}
	return domain.LintCount{}, nil
}
func (p checkPrepared) RuleItems(_ context.Context, rule domain.LintRule) ([]domain.LintItem, bool, error) {
	if rule != domain.RuleMissingType || p.f.clean {
		return nil, false, nil
	}
	return []domain.LintItem{{Rule: rule, ItemKey: "p:" + p.f.page.String(), Cursor: "c", PageID: &p.f.page}}, false, nil
}
func (p checkPrepared) Timings() app.PreparationTimings { return app.PreparationTimings{} }
func (p checkPrepared) WantedByPage() map[uuid.UUID]int {
	return map[uuid.UUID]int{p.f.page: 1 + p.f.moreWanted}
}
func (f *checkFake) Decode(raw string) (app.DecodedCursor, error) {
	if raw == "bad" {
		return nil, shared.InvalidCursor()
	}
	root, err := uuid.Parse(strings.TrimPrefix(raw, "bound:"))
	if err != nil {
		return nil, shared.InvalidCursor()
	}
	return checkCursor{root: root}, nil
}

func (f *checkFake) Encode(scope app.LintScope, rule domain.LintRule, _ domain.LintPosition) (string, error) {
	f.note(context.Background(), "encode")
	return "bound:" + scope.AreaRootID.String() + ":" + string(rule), nil
}

type checkCursor struct{ root uuid.UUID }

func (c checkCursor) PositionFor(scope app.LintScope, _ domain.LintRule) (domain.LintPosition, error) {
	if scope.Contract == nil || *scope.Contract != domain.ModeLLMWiki || scope.AreaRootID == nil || *scope.AreaRootID != c.root {
		return domain.LintPosition{}, shared.InvalidCursor()
	}
	id := uuid.UUID{15: 1}
	return domain.LintPosition{ID: &id}, nil
}

func (f *checkFake) Paths(ctx context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.note(ctx, "paths")
	return maps.Clone(f.paths), nil
}

// Do is the runner: the key recorded, the worker run on ctx, or runErr.
func (f *checkFake) Do(ctx context.Context, key app.RunKey, run func(context.Context) (app.RunOutput, error)) (app.RunResult, error) {
	f.mu.Lock()
	f.keys = append(f.keys, key)
	f.runCtx = append(f.runCtx, ctx)
	err := f.runErr
	f.mu.Unlock()
	f.note(ctx, "run")
	if err != nil {
		return app.RunResult{}, err
	}
	out, err := run(ctx)
	if err != nil {
		return app.RunResult{}, err
	}
	return app.RunResult{Lint: domain.CloneLintResult(out.Result), Check: domain.CloneUpgradeCheck(out.Check)}, nil
}

func (f *checkFake) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}

// newCheck is the use case over f, logging to log, and the caller's ctx.
func newCheck(f *checkFake, log *startLog) (*app.CheckUpgrade, context.Context) {
	logger := slog.New(slog.NewJSONHandler(log, nil))
	lint := app.NewLint(app.LintDeps{Notebooks: f, Sources: f, Nodes: f, Rules: f, Cursors: f, Paths: f, Snapshots: f, Authorizer: f,
		Clock: f, Runs: f})
	d := app.UpgradeDeps{Tx: f, Snapshots: f, Notebooks: f, Rows: f, Names: f, Authorizer: f, Action: "notebook.change_mode",
		Clock: f, Logger: logger, Gates: f, Cursors: f, Paths: f, ReservedKeys: []string{"index", "log", "index.md", "log.md"}}
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	return app.NewCheckUpgrade(d, lint, f), ctx
}

func strp(s string) *string { return &s }
func intp(n int) *int       { return &n }

// The check answers its codes in the order of M11/P2 design 3.11, each
// before the next: a cursor that does not decode; the notebook and its
// grant; a rule's absence or name; the upgrade's state; a cursor bound to
// another root; the limit; the runs' budget. Each pair of neighbours
// answers the first.
func TestCheckUpgradeAnswersInOrder(t *testing.T) {
	type tweak func(f *checkFake, q *app.LintQuery)
	var (
		badCursor   tweak = func(_ *checkFake, q *app.LintQuery) { q.Cursor = strp("bad") }
		noNotebook  tweak = func(f *checkFake, _ *app.LintQuery) { f.noNotebook = true }
		invisible   tweak = func(f *checkFake, _ *app.LintQuery) { f.authErrs = []error{shared.ErrNotVisible} }
		notAdmin    tweak = func(f *checkFake, _ *app.LintQuery) { f.authErrs = []error{shared.Forbidden()} }
		limitAlone  tweak = func(_ *checkFake, q *app.LintQuery) { q.Rule, q.Limit = nil, intp(5) }
		unknownRule tweak = func(_ *checkFake, q *app.LintQuery) { q.Rule = strp("broken") }
		noUpgrade   tweak = func(f *checkFake, _ *app.LintQuery) { f.noUp = true }
		rolling     tweak = func(f *checkFake, _ *app.LintQuery) { f.up.State = domain.UpgradeRollingBack }
		otherRoot   tweak = func(_ *checkFake, q *app.LintQuery) { q.Cursor = strp("bound:" + uuid.NewV7().String()) }
		bigLimit    tweak = func(_ *checkFake, q *app.LintQuery) { q.Limit = intp(101) }
		busy        tweak = func(f *checkFake, _ *app.LintQuery) { f.runErr = shared.ServerBusy(time.Second) }
	)
	items := func(f *checkFake, q *app.LintQuery) { q.Rule = strp("missing_type") }
	for _, tc := range []struct {
		name   string
		tweaks []tweak
		code   string
	}{
		{"a cursor that does not decode", []tweak{badCursor}, "bad_request"},
		{"no notebook", []tweak{noNotebook}, "notebook.not_found"},
		{"a notebook not visible", []tweak{invisible}, "notebook.not_found"},
		{"no admin", []tweak{notAdmin}, "forbidden"},
		{"a limit without a rule", []tweak{limitAlone}, "validation_failed"},
		{"an unknown rule", []tweak{unknownRule}, "wiki.lint_rule_not_found"},
		{"no upgrade", []tweak{noUpgrade}, "wiki.upgrade_state"},
		{"an upgrade rolling back", []tweak{rolling}, "wiki.upgrade_state"},
		{"a cursor bound to another root", []tweak{otherRoot}, "bad_request"},
		{"a limit past 100", []tweak{bigLimit}, "validation_failed"},
		{"no slot", []tweak{busy}, "server_busy"},
		{"a bad cursor before a notebook not visible", []tweak{badCursor, invisible}, "bad_request"},
		{"a notebook not visible before an unknown rule", []tweak{invisible, unknownRule}, "notebook.not_found"},
		{"an unknown rule before an upgrade rolling back", []tweak{unknownRule, rolling}, "wiki.lint_rule_not_found"},
		{"an upgrade rolling back before a cursor bound elsewhere", []tweak{rolling, otherRoot}, "wiki.upgrade_state"},
		{"a cursor bound elsewhere before a limit past 100", []tweak{otherRoot, bigLimit}, "bad_request"},
		{"a limit past 100 before no slot", []tweak{bigLimit, busy}, "validation_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCheckFake()
			q := app.LintQuery{}
			items(f, &q)
			for _, tw := range tc.tweaks {
				tw(f, &q)
			}
			check, ctx := newCheck(f, &startLog{})
			_, err := check.Execute(ctx, f.nb, q)
			if got := codeOf(err); got != tc.code {
				t.Fatalf("answered %v (%s), want %s; calls %v", err, got, tc.code, f.trace())
			}
		})
	}
}

// One check reads every gate, the summary and the counts in one snapshot,
// in turn (M11/P2 design 3.11; S3A-M2): the upgrade again, the roots,
// lint's state, nodes, rules, start cursor and item paths, the baseline,
// what was kept, the changesets, the gates' paths. Its key is the
// upgrade's check's, scoped by the upgrade; the items' by it too.
func TestTheCheckReadsInOneSnapshot(t *testing.T) {
	f := newCheckFake()
	check, ctx := newCheck(f, &startLog{})
	got, err := check.Execute(ctx, f.nb, app.LintQuery{})
	if err != nil || got.Check == nil || got.Items != nil {
		t.Fatalf("check = %+v, %v", got, err)
	}
	// encode takes no ctx: it is traced where it falls, between the
	// snapshot's reads.
	var snap []string
	for _, call := range f.trace() {
		if strings.HasSuffix(call, "@") || call == "encode" {
			snap = append(snap, call)
		}
	}
	want := []string{"Current@", "Roots@", "state@", "nodes@", "prepare@", "encode", "paths@", "Baseline@", "Preserved@", "Changesets@",
		"paths@"}
	if f.snapshots != 1 || !slices.Equal(snap, want) {
		t.Fatalf("read in %d snapshots: %v, want one: %v; all %v", f.snapshots, snap, want, f.trace())
	}
	c := got.Check
	if !c.ComputedAt.Equal(f.at) || c.Passed || !slices.Equal(c.Gates.Failed(), []domain.UpgradeGateKey{domain.GateFrontmatter,
		domain.GatePreserved}) || len(c.Rules) != 8 || c.Paths[f.page] != "Page.md" || c.Counts.Changesets != 4 {
		t.Fatalf("check = %+v", c)
	}
	key := f.keys[0]
	if key.Kind != domain.LintUpgradeCheck || key.Scope != f.up.ID || !key.Explicit || key.Contract != domain.ModeLLMWiki ||
		!key.HasRoot || key.RootID != f.root || key.NotebookID != f.nb {
		t.Fatalf("the check's key = %+v", key)
	}

	f = newCheckFake()
	check, ctx = newCheck(f, &startLog{})
	got, err = check.Execute(ctx, f.nb, app.LintQuery{Rule: strp("missing_type"), Limit: intp(7), Cursor: strp("bound:" + f.root.String())})
	if err != nil || got.Items == nil || got.Check != nil || len(got.Items.Rules) != 1 || got.Items.Kind != domain.LintItems {
		t.Fatalf("items = %+v, %v", got, err)
	}
	key = f.keys[0]
	if key.Kind != domain.LintUpgradeItems || key.Scope != f.up.ID || key.Rule != domain.RuleMissingType || key.Limit != 7 ||
		key.CursorCanonical != "bound:"+f.root.String()+":missing_type" || key.RootID != f.root {
		t.Fatalf("the items' key = %+v", key)
	}
}

// A check answers nothing to a caller who is no longer the notebook's
// admin once it is computed (M11 design 4.7): the decision again before
// the answer.
func TestTheCheckAuthorizesBeforeItsAnswer(t *testing.T) {
	for _, q := range []app.LintQuery{{}, {Rule: strp("wanted")}} {
		f := newCheckFake()
		f.authErrs = []error{nil, shared.Forbidden()}
		check, ctx := newCheck(f, &startLog{})
		got, err := check.Execute(ctx, f.nb, q)
		if codeOf(err) != "forbidden" || got.Check != nil || got.Items != nil {
			t.Fatalf("%+v: answered %+v, %v; want forbidden, nothing", q, got, err)
		}
		if trace := f.trace(); !slices.Contains(trace, "run") || trace[len(trace)-1] != "authorize" {
			t.Fatalf("%+v: calls %v, want the decision after the run", q, trace)
		}
	}
}

// The snapshot's upgrade must be the one the check judged (B-M3): another
// one, or the same one rolling back by the time its run had a slot, is
// wiki.upgrade_state, read nothing more.
func TestTheCheckRefusesAnotherUpgradesSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*domain.Upgrade)
	}{
		{"another upgrade", func(u *domain.Upgrade) { u.ID = uuid.NewV7() }},
		{"rolling back", func(u *domain.Upgrade) { u.State = domain.UpgradeRollingBack }},
	} {
		for _, q := range []app.LintQuery{{}, {Rule: strp("wanted")}} {
			f := newCheckFake()
			now := f.up
			tc.change(&now)
			f.snapUp = &now
			check, ctx := newCheck(f, &startLog{})
			if _, err := check.Execute(ctx, f.nb, q); codeOf(err) != "wiki.upgrade_state" {
				t.Fatalf("%s, %+v: answered %v", tc.name, q, err)
			}
			if trace := f.trace(); slices.Contains(trace, "Roots@") || slices.Contains(trace, "prepare@") {
				t.Fatalf("%s: read on after the snapshot's upgrade: %v", tc.name, trace)
			}
		}
	}
	// Reviewing is a state a check reads.
	f := newCheckFake()
	f.up.State = domain.UpgradeReviewing
	check, ctx := newCheck(f, &startLog{})
	if got, err := check.Execute(ctx, f.nb, app.LintQuery{}); err != nil || got.Check == nil {
		t.Fatalf("reviewing: %+v, %v", got, err)
	}
}

// The check's defects (M11/P2 design 3.16): a proposed root that is no
// live page answers an error, a line at error; baseline rows that are not
// the upgrade's counts, a line at error, and the gate fails.
func TestTheChecksDefectsAreLogged(t *testing.T) {
	f := newCheckFake()
	f.roots.RootPage = false
	log := &startLog{}
	check, ctx := newCheck(f, log)
	var coded *shared.Error
	if _, err := check.Execute(ctx, f.nb, app.LintQuery{}); err == nil || errors.As(err, &coded) {
		t.Fatalf("a dead source root answered %v, want a defect's error", err)
	}
	lines := log.lines(t)
	if len(lines) != 1 || lines[0]["msg"] != "upgrade source root is no live page" || lines[0]["level"] != "ERROR" ||
		lines[0]["upgrade_id"] != f.up.ID.String() || lines[0]["notebook_id"] != f.nb.String() {
		t.Fatalf("logged %v", lines)
	}

	f = newCheckFake()
	f.preserved = domain.Preserved{Nodes: 2, NodesKept: 2, Pages: 2, PagesKept: 2, NotKept: []uuid.UUID{}}
	log = &startLog{}
	check, ctx = newCheck(f, log)
	got, err := check.Execute(ctx, f.nb, app.LintQuery{})
	if err != nil || got.Check.Gates.Preserved.Passed {
		t.Fatalf("a baseline row short = %+v, %v", got, err)
	}
	lines = log.lines(t)
	if len(lines) != 1 || lines[0]["msg"] != "upgrade baseline rows not its counts" || lines[0]["level"] != "ERROR" ||
		lines[0]["nodes"] != float64(2) || lines[0]["baseline_nodes"] != float64(3) || lines[0]["pages"] != float64(2) ||
		lines[0]["baseline_pages"] != float64(3) {
		t.Fatalf("logged %v", lines)
	}
}
