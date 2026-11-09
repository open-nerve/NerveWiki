package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
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

// startLog is the start's log, safe for the runs' goroutines.
type startLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *startLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *startLog) lines(t *testing.T) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(l.b.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

type startTxKey struct{}

// startFake is every port of an upgrade's start (M11/P1 design 3.14): a
// transaction manager that runs fn and answers its error, rolled back late
// by rollbackLate, or commits, late by commitLate, answering commitErr; the
// notebook, its workspace, its pages and its lint as the fields say; the
// calls in order, and at four of them whether the start held the lint's
// only slot.
type startFake struct {
	mu    sync.Mutex
	calls []string
	runs  *app.Runs
	slots []string

	at                        time.Time
	ws                        uuid.UUID
	noWorkspace, noShare      bool
	noLock                    bool
	state                     app.UpgradeNotebook
	authErr                   error
	writing                   bool
	holders                   []app.SessionHolder
	total                     int
	roots                     []app.RootNode
	base                      app.UpgradeBaseline
	stale                     int64
	byPage                    map[uuid.UUID]int
	count                     domain.LintCount
	captureWaits              bool
	commitLate, rollbackLate  time.Duration
	commitErr                 error
	analyzeErr                error
	started                   app.NotebookUpgrade
	protected                 uuid.UUID
	wanted                    map[uuid.UUID]int
	inserted                  domain.Upgrade
	insertedBy                uuid.UUID
	published                 app.ModeChange
	query                     app.RuleQuery
	input                     app.ReadInput
	analyzeDeadline           bool
	analyzeErrAtCall          error
	stateOutsideTransaction   bool
	sourcesOutsideTransaction bool
}

func (f *startFake) note(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *startFake) trace() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// slot notes at call whether the lint's only slot was held: one taken here
// goes back at once.
func (f *startFake) slot(call string) {
	release, ok := f.runs.TryHold()
	if ok {
		release()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.slots = append(f.slots, fmt.Sprintf("%s held %t", call, !ok))
}

func (f *startFake) held() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.slots)
}

func inTx(ctx context.Context) bool { return ctx.Value(startTxKey{}) == true }

func (f *startFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.note("begin")
	if err := fn(context.WithValue(ctx, startTxKey{}, true)); err != nil {
		time.Sleep(f.rollbackLate)
		f.note("rollback")
		return err
	}
	time.Sleep(f.commitLate)
	f.slot("commit")
	if f.commitErr != nil {
		f.note("commit failed")
		return f.commitErr
	}
	f.note("commit")
	return nil
}

func (f *startFake) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	f.note("workspace")
	return f.ws, !f.noWorkspace, nil
}

func (f *startFake) StateOf(ctx context.Context, _ uuid.UUID) (app.NotebookState, bool, error) {
	f.note("state")
	if !inTx(ctx) {
		f.stateOutsideTransaction = true
	}
	return app.NotebookState{Mode: domain.ModeStandard}, true, nil
}

func (f *startFake) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	f.note("share")
	if !inTx(ctx) || id != f.ws {
		return false, errors.New("the workspace's lock outside the transaction")
	}
	return !f.noShare, nil
}

func (f *startFake) Lock(ctx context.Context, _ uuid.UUID) (app.UpgradeNotebook, bool, error) {
	f.note("lock")
	if !inTx(ctx) {
		return app.UpgradeNotebook{}, false, errors.New("the notebook's lock outside the transaction")
	}
	return f.state, !f.noLock, nil
}

func (f *startFake) Start(_ context.Context, _ uuid.UUID, up app.NotebookUpgrade) error {
	f.note("start")
	f.started = up
	return nil
}

func (f *startFake) Publish(_ context.Context, c app.ModeChange) error {
	f.note("publish")
	f.published = c
	return nil
}

func (f *startFake) Authorize(_ context.Context, _ shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	f.note("authorize")
	if action != "notebook.change_mode" || target.WorkspaceID != f.ws {
		return shared.Grant{}, errors.New("another decision")
	}
	return shared.Grant{}, f.authErr
}

func (f *startFake) Now() time.Time {
	f.note("clock")
	return f.at
}

func (f *startFake) Writing(context.Context, uuid.UUID) (bool, error) {
	f.note("writing")
	return f.writing, nil
}

func (f *startFake) Capture(ctx context.Context, _, _ uuid.UUID) (app.UpgradeBaseline, error) {
	f.note("capture")
	f.slot("capture")
	if f.captureWaits {
		<-ctx.Done()
		return app.UpgradeBaseline{}, ctx.Err()
	}
	return f.base, nil
}

func (f *startFake) Protect(_ context.Context, _, _, root uuid.UUID) error {
	f.note("protect")
	f.protected = root
	return nil
}

func (f *startFake) SetWanted(_ context.Context, _ uuid.UUID, wanted map[uuid.UUID]int) error {
	f.note("wanted")
	f.wanted = wanted
	return nil
}

func (f *startFake) Analyze(ctx context.Context) error {
	f.note("analyze")
	f.slot("analyze")
	_, f.analyzeDeadline = ctx.Deadline()
	f.analyzeErrAtCall = ctx.Err()
	return f.analyzeErr
}

func (f *startFake) ClearExpiredSessions(context.Context, uuid.UUID, time.Time) error {
	f.note("clear")
	return nil
}

func (f *startFake) LiveSessions(context.Context, uuid.UUID, time.Time) ([]app.SessionHolder, int, error) {
	f.note("live")
	return f.holders, f.total, nil
}

func (f *startFake) RootNodes(_ context.Context, _ uuid.UUID, keys []string) ([]app.RootNode, error) {
	f.note("roots")
	if !slices.Equal(keys, []string{"raw", "agents"}) {
		return nil, errors.New("other keys")
	}
	return f.roots, nil
}

func (f *startFake) Insert(_ context.Context, u domain.Upgrade, by uuid.UUID, _ time.Time) error {
	f.note("insert")
	f.inserted, f.insertedBy = u, by
	return nil
}

func (f *startFake) Current(context.Context, uuid.UUID) (domain.Upgrade, bool, error) {
	return domain.Upgrade{}, false, errors.New("the start reads no row")
}

func (f *startFake) DisplayNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.note("names")
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = "name of " + id.String()
	}
	return out, nil
}

func (f *startFake) LintData(ctx context.Context, _, root uuid.UUID) (app.SourceData, error) {
	f.note("sources")
	if !inTx(ctx) {
		f.sourcesOutsideTransaction = true
	}
	return app.SourceData{Nodes: []app.CompactNode{{ID: root, Kind: domain.NodePage, NameKey: "raw"}}}, nil
}

func (f *startFake) Nodes(context.Context, uuid.UUID) ([]app.CompactNode, error) {
	return nil, errors.New("a standard read")
}

func (f *startFake) PrepareRules(_ context.Context, input app.ReadInput, query app.RuleQuery) (app.PreparedRules, error) {
	f.note("prepare")
	f.slot("prepare")
	f.input, f.query = input, query
	return startPrepared{f}, nil
}

type startPrepared struct{ f *startFake }

func (p startPrepared) StalePages() int64 { return p.f.stale }
func (p startPrepared) RuleCount(_ context.Context, rule domain.LintRule) (domain.LintCount, error) {
	if rule != domain.RuleWanted {
		return domain.LintCount{}, errors.New("a rule not prepared")
	}
	return p.f.count, nil
}
func (p startPrepared) RuleItems(context.Context, domain.LintRule) ([]domain.LintItem, bool, error) {
	return nil, false, errors.New("the start projects no item")
}
func (p startPrepared) Timings() app.PreparationTimings { return app.PreparationTimings{} }
func (p startPrepared) WantedByPage() map[uuid.UUID]int { return maps.Clone(p.f.byPage) }

// tracedSeeds are the template's seeds, traced among the start's calls.
type tracedSeeds struct {
	*fakeSeeds
	f *startFake
}

func (s tracedSeeds) Seed(ctx context.Context, x app.Seed) ([]uuid.UUID, error) {
	s.f.note("seed")
	return s.fakeSeeds.Seed(ctx, x)
}

type startFixture struct {
	f     *startFake
	seeds *fakeSeeds
	start *app.StartUpgrade
	runs  *app.Runs
	log   *startLog
	actor uuid.UUID
	nb    uuid.UUID
}

func newStartFixture(t *testing.T, timeout time.Duration, change func(*startFake)) startFixture {
	t.Helper()
	f := &startFake{ws: uuid.NewV7(), at: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC), base: app.UpgradeBaseline{Nodes: 3, Pages: 2},
		count: domain.LintCount{Value: 4}}
	if change != nil {
		change(f)
	}
	log := &startLog{}
	logger := slog.New(slog.NewJSONHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	runs := app.NewRuns(1, time.Second, logger, nil)
	f.runs = runs
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := runs.Stop(ctx); err != nil {
			t.Errorf("a slot was never released: %v", err)
		}
	})
	seeds := &fakeSeeds{}
	template, err := app.NewTemplate(tracedSeeds{seeds, f})
	if err != nil {
		t.Fatal(err)
	}
	seeds.parsed = nil
	lint := app.NewLint(app.LintDeps{Notebooks: f, Sources: f, Nodes: f, Rules: f, Authorizer: f, Clock: f, Runs: runs})
	start := app.NewStartUpgrade(app.UpgradeDeps{Tx: f, Notebooks: f, Workspaces: f, States: f, Pages: f, Rows: f, Writing: f,
		Names: f, Authorizer: f, Action: "notebook.change_mode", Template: template, Clock: f, Logger: logger, Timeout: timeout}, lint, runs)
	return startFixture{f: f, seeds: seeds, start: start, runs: runs, log: log, actor: uuid.NewV7(), nb: uuid.NewV7()}
}

func (x startFixture) ctx() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: x.actor, SessionID: uuid.NewV7()})
}

// slotFree reports whether the start released its slot: the only one can
// be held again.
func (x startFixture) slotFree(t *testing.T) {
	t.Helper()
	release, ok := x.runs.TryHold()
	if !ok {
		t.Fatal("the start kept its lint slot")
	}
	release()
}

func startCode(err error) string {
	var problem *shared.Error
	if errors.As(err, &problem) {
		return problem.Code
	}
	return ""
}

// refusalInItsTurn checks a refused start's calls: none for an invalid
// body, or all of them up to last, then rolled back; its slot given back;
// the locks of wiki.upgrade_pages_locked named.
func refusalInItsTurn(t *testing.T, x startFixture, err error, code, last string, holders []app.SessionHolder) {
	t.Helper()
	calls := x.f.trace()
	if last == "" {
		if len(calls) != 0 {
			t.Errorf("read %v", calls)
		}
		return
	}
	inTx := slices.Contains(calls, "begin")
	if inTx && (len(calls) < 2 || calls[len(calls)-2] != last || calls[len(calls)-1] != "rollback") ||
		!inTx && calls[len(calls)-1] != last || slices.Contains(calls, "commit") {
		t.Errorf("calls %v, want them to end at %s", calls, last)
	}
	x.slotFree(t)
	if code == "wiki.upgrade_pages_locked" {
		var problem *shared.Error
		errors.As(err, &problem)
		if len(problem.Locks) != 100 || problem.Locks[0].PageID != holders[0].PageID || problem.Locks[0].DisplayName != "name of "+holders[0].UserID.String() ||
			!strings.HasPrefix(problem.Detail, "150 pages") {
			t.Errorf("the locks named: %d, %+v, %q", len(problem.Locks), problem.Locks[0], problem.Detail)
		}
	}
}

// An upgrade's start (M11/P1 design 3.14; M11 design 4.4), with fakes of
// every port: the order of its steps and their codes, the template's pages
// the notebook lacks written as the upgrade's own unit, only the wanted
// rule prepared in its transaction, a slot held now or not at all, after
// every refusal but the stale index, from the baseline through the commit;
// its own deadline server_busy with a warn line, a refusal rolled back past
// it its own code, a commit past it 200, a commit out of its own time not
// server_busy; the statistics refreshed on the request's ctx. The whole
// program's half is the seventh shard's TestUpgradeStart.
func TestUpgradeStart(t *testing.T) {
	t.Run("it-writes-what-the-notebook-lacks", func(t *testing.T) {
		raw, agents, page := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		x := newStartFixture(t, time.Second, func(f *startFake) { f.byPage = map[uuid.UUID]int{page: 2} })
		x.seeds.ids = []uuid.UUID{raw, agents}
		view, err := x.start.Execute(x.ctx(), x.nb, "zh-CN")
		if err != nil {
			t.Fatal(err)
		}
		f := x.f
		want := []string{"workspace", "begin", "share", "lock", "authorize", "clock", "writing", "clear", "live", "roots", "start",
			"capture", "seed", "protect", "state", "sources", "prepare", "wanted", "insert", "publish", "names", "commit", "analyze"}
		if got := f.trace(); !slices.Equal(got, want) {
			t.Fatalf("calls %v\nwant  %v", got, want)
		}
		up := view.Upgrade
		wantUp := domain.Upgrade{ID: up.ID, NotebookID: x.nb, State: domain.UpgradeUpgrading, SourceRootID: raw, SeededRoot: true,
			SeededAgentsID: &agents, Locale: domain.LocaleZhCN, BaselineNodes: 3, BaselinePages: 2, BaselineWanted: 4,
			CreatedBy: x.actor, CreatedAt: f.at}
		if up.ID == (uuid.UUID{}) || !reflect.DeepEqual(up, wantUp) || !reflect.DeepEqual(f.inserted, wantUp) || f.insertedBy != x.actor ||
			view.StartedByName != "name of "+x.actor.String() {
			t.Fatalf("upgrade %+v\ninserted %+v\nwant %+v", view, f.inserted, wantUp)
		}
		if f.started != (app.NotebookUpgrade{ID: up.ID, State: domain.UpgradeUpgrading, By: x.actor, At: f.at}) {
			t.Errorf("the notebook's upgrade %+v", f.started)
		}
		s := x.seeds.seeds
		if len(s) != 1 || s[0].Upgrade != up.ID || s[0].NotebookID != x.nb || s[0].Client != "web" || s[0].At != f.at ||
			len(s[0].Pages) != 2 || s[0].Pages[0].Title != "raw" || s[0].Pages[1].Title != "AGENTS" {
			t.Errorf("seeded %+v", s)
		}
		if f.protected != raw {
			t.Errorf("protected %s, want the seeded raw %s", f.protected, raw)
		}
		llm, wanted := domain.ModeLLMWiki, domain.RuleWanted
		if !reflect.DeepEqual(f.query, app.RuleQuery{Rule: &wanted, Limit: 1, WantedByPage: true}) ||
			f.input.EffectiveMode != llm || !f.input.Explicit || f.input.AreaRootID == nil || *f.input.AreaRootID != raw ||
			f.stateOutsideTransaction || f.sourcesOutsideTransaction {
			t.Errorf("prepared %+v of %+v (outside the transaction: %t %t)", f.query, f.input, f.stateOutsideTransaction, f.sourcesOutsideTransaction)
		}
		if !maps.Equal(f.wanted, map[uuid.UUID]int{page: 2}) {
			t.Errorf("wanted counts %v", f.wanted)
		}
		if f.published != (app.ModeChange{WorkspaceID: f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard, By: x.actor, At: f.at}) {
			t.Errorf("published %+v", f.published)
		}
		if f.analyzeDeadline || f.analyzeErrAtCall != nil {
			t.Error("the baseline was analyzed on the start's deadline, not the request's ctx")
		}
		if got, want := f.held(), []string{"capture held true", "prepare held true", "commit held true", "analyze held false"}; !slices.Equal(got, want) {
			t.Errorf("the lint slot %v\nwant %v: held while the start computes, to its commit, and given back before the statistics", got, want)
		}
		x.slotFree(t)
		lines := x.log.lines(t)
		if len(lines) != 1 {
			t.Fatalf("log %v", lines)
		}
		l := lines[0]
		if l["level"] != "INFO" || l["msg"] != "upgrade started" || l["workspace_id"] != f.ws.String() || l["notebook_id"] != x.nb.String() ||
			l["upgrade_id"] != up.ID.String() || l["user_id"] != x.actor.String() || l["baseline_nodes"] != 3.0 || l["baseline_pages"] != 2.0 ||
			l["seeded_root"] != true || l["seeded_agents"] != true || l["wanted"] != 4.0 || l["elapsed_ms"] == nil {
			t.Errorf("log line %v", l)
		}
	})
	t.Run("a-raw-page-is-the-source-root", func(t *testing.T) {
		raw, agents := uuid.NewV7(), uuid.NewV7()
		x := newStartFixture(t, time.Second, func(f *startFake) {
			f.roots = []app.RootNode{{ID: raw, Kind: domain.NodePage, Key: "raw"}, {ID: agents, Kind: domain.NodePage, Key: "agents"}}
			f.count = domain.LintCount{Value: 10000, Capped: true}
		})
		view, err := x.start.Execute(x.ctx(), x.nb, "")
		if err != nil {
			t.Fatal(err)
		}
		up := view.Upgrade
		if up.SourceRootID != raw || up.SeededRoot || up.SeededAgentsID != nil || up.Locale != domain.LocaleEn ||
			up.BaselineWanted != 10000 || !up.BaselineWantedCapped || *x.f.input.AreaRootID != raw {
			t.Errorf("upgrade %+v from %+v", up, x.f.input)
		}
		if got := x.f.trace(); slices.Contains(got, "seed") || slices.Contains(got, "protect") {
			t.Errorf("wrote or protected a page the notebook had: %v", got)
		}
	})
	t.Run("only-agents-is-written", func(t *testing.T) {
		raw, agents := uuid.NewV7(), uuid.NewV7()
		x := newStartFixture(t, time.Second, func(f *startFake) { f.roots = []app.RootNode{{ID: raw, Kind: domain.NodePage, Key: "raw"}} })
		x.seeds.ids = []uuid.UUID{agents}
		view, err := x.start.Execute(x.ctx(), x.nb, "en")
		if err != nil {
			t.Fatal(err)
		}
		up := view.Upgrade
		if up.SourceRootID != raw || up.SeededRoot || up.SeededAgentsID == nil || *up.SeededAgentsID != agents ||
			len(x.seeds.seeds) != 1 || len(x.seeds.seeds[0].Pages) != 1 || x.seeds.seeds[0].Pages[0].Title != "AGENTS" {
			t.Errorf("upgrade %+v, seeds %+v", up, x.seeds.seeds)
		}
		if slices.Contains(x.f.trace(), "protect") {
			t.Error("protected a raw the notebook had")
		}
	})
	// Each refusal in its turn, with the lint's only slot free and again
	// held by another: the slot is step 9, after every refusal but the
	// stale index, which a full slot's server_busy comes before.
	t.Run("each-refusal-in-its-turn", func(t *testing.T) {
		var holders []app.SessionHolder
		for range 100 {
			holders = append(holders, app.SessionHolder{PageID: uuid.NewV7(), UserID: uuid.NewV7()})
		}
		for _, tt := range []struct {
			name, locale, code string
			change             func(*startFake)
			last               string
		}{
			{"a-language", "fr", "validation_failed", nil, ""},
			{"no-notebook", "", "notebook.not_found", func(f *startFake) { f.noWorkspace = true }, "workspace"},
			{"no-workspace", "", "notebook.not_found", func(f *startFake) { f.noShare = true }, "share"},
			{"deleted-meanwhile", "", "notebook.not_found", func(f *startFake) { f.noLock = true }, "lock"},
			{"not-visible", "", "notebook.not_found", func(f *startFake) { f.authErr = shared.ErrNotVisible }, "authorize"},
			{"no-admin", "", "forbidden", func(f *startFake) { f.authErr = shared.Forbidden() }, "authorize"},
			{"an-llm-wiki", "", "notebook.already_llm_wiki", func(f *startFake) { f.state.Mode = domain.ModeLLMWiki }, "clock"},
			{"an-upgrade", "", "notebook.upgrading", func(f *startFake) {
				f.state.Upgrade = app.NotebookUpgrade{ID: uuid.NewV7(), State: domain.UpgradeUpgrading}
			}, "clock"},
			{"a-writing-job", "", "transfer.busy", func(f *startFake) { f.writing = true }, "writing"},
			{"edit-sessions", "", "wiki.upgrade_pages_locked", func(f *startFake) { f.holders, f.total = holders, 150 }, "names"},
			{"an-attachment-raw", "", "wiki.upgrade_title_taken", func(f *startFake) {
				f.roots = []app.RootNode{{ID: uuid.NewV7(), Kind: domain.NodePage, Key: "agents"}, {ID: uuid.NewV7(), Kind: domain.NodeAsset, Key: "raw"}}
			}, "roots"},
			{"a-stale-index", "", "wiki.upgrade_stale_index", func(f *startFake) { f.stale = 1 }, "prepare"},
		} {
			for _, full := range []bool{false, true} {
				name, code, last := tt.name, tt.code, tt.last
				if full {
					name += "-a-full-slot"
					if code == "wiki.upgrade_stale_index" {
						code, last = "server_busy", "roots"
					}
				}
				t.Run(name, func(t *testing.T) {
					x := newStartFixture(t, time.Second, tt.change)
					x.seeds.ids = []uuid.UUID{uuid.NewV7(), uuid.NewV7()}
					var other func()
					if full {
						var ok bool
						if other, ok = x.runs.TryHold(); !ok {
							t.Fatal("no slot")
						}
					}
					_, err := x.start.Execute(x.ctx(), x.nb, tt.locale)
					if other != nil {
						other()
					}
					if got := startCode(err); got != code {
						t.Fatalf("answered %v, want %s", err, code)
					}
					refusalInItsTurn(t, x, err, code, last, holders)
				})
			}
		}
	})
	t.Run("a-full-slot-is-503-at-once", func(t *testing.T) {
		x := newStartFixture(t, time.Minute, nil)
		release, ok := x.runs.TryHold()
		if !ok {
			t.Fatal("no slot")
		}
		started := time.Now()
		_, err := x.start.Execute(x.ctx(), x.nb, "")
		release()
		var problem *shared.Error
		if !errors.As(err, &problem) || problem.Code != "server_busy" || problem.RetryDelay != time.Second || time.Since(started) > 500*time.Millisecond {
			t.Fatalf("answered %v after %v", err, time.Since(started))
		}
		if calls := x.f.trace(); slices.Contains(calls, "start") || calls[len(calls)-1] != "rollback" {
			t.Errorf("calls %v", calls)
		}
	})
	t.Run("its-own-deadline", func(t *testing.T) {
		x := newStartFixture(t, 50*time.Millisecond, func(f *startFake) { f.captureWaits = true })
		_, err := x.start.Execute(x.ctx(), x.nb, "")
		var problem *shared.Error
		if !errors.As(err, &problem) || problem.Code != "server_busy" || problem.RetryDelay != time.Second {
			t.Fatalf("answered %v, want server_busy", err)
		}
		if calls := x.f.trace(); calls[len(calls)-1] != "rollback" || slices.Contains(calls, "insert") {
			t.Errorf("calls %v", calls)
		}
		x.slotFree(t)
		lines := x.log.lines(t)
		if len(lines) != 1 {
			t.Fatalf("log %v", lines)
		}
		l := lines[0]
		if l["level"] != "WARN" || l["msg"] != "upgrade start timed out" || l["workspace_id"] != x.f.ws.String() || l["notebook_id"] != x.nb.String() ||
			l["user_id"] != x.actor.String() || l["timeout_ms"] != 50.0 || l["elapsed_ms"] == nil {
			t.Errorf("log line %v", l)
		}
	})
	t.Run("the-requests-own-end", func(t *testing.T) {
		x := newStartFixture(t, time.Minute, func(f *startFake) { f.captureWaits = true })
		ctx, cancel := context.WithTimeout(x.ctx(), 50*time.Millisecond)
		defer cancel()
		_, err := x.start.Execute(ctx, x.nb, "")
		if !errors.Is(err, context.DeadlineExceeded) || startCode(err) != "" {
			t.Fatalf("answered %v, want the request's own end", err)
		}
		if lines := x.log.lines(t); len(lines) != 0 {
			t.Errorf("log %v", lines)
		}
	})
	t.Run("a-commit-past-its-deadline", func(t *testing.T) {
		x := newStartFixture(t, 50*time.Millisecond, func(f *startFake) { f.commitLate = 100 * time.Millisecond })
		view, err := x.start.Execute(x.ctx(), x.nb, "")
		if err != nil || view.Upgrade.ID == (uuid.UUID{}) {
			t.Fatalf("a committed start answered %+v, %v", view, err)
		}
		if x.f.analyzeDeadline || x.f.analyzeErrAtCall != nil {
			t.Error("the baseline was analyzed on the start's deadline")
		}
		if lines := x.log.lines(t); len(lines) != 1 || lines[0]["msg"] != "upgrade started" {
			t.Errorf("log %v", lines)
		}
	})
	// A domain's refusal answers its own code, though its rollback ran past
	// the start's deadline (decision 227).
	t.Run("a-refusal-rolled-back-past-its-deadline", func(t *testing.T) {
		x := newStartFixture(t, 50*time.Millisecond, func(f *startFake) { f.stale, f.rollbackLate = 1, 100*time.Millisecond })
		_, err := x.start.Execute(x.ctx(), x.nb, "")
		if got := startCode(err); got != "wiki.upgrade_stale_index" {
			t.Fatalf("answered %v, want wiki.upgrade_stale_index", err)
		}
		if lines := x.log.lines(t); len(lines) != 0 {
			t.Errorf("log %v", lines)
		}
		x.slotFree(t)
	})
	// A commit that ran out of its own time, the start's deadline not yet
	// up: an error of the transaction, not server_busy (M11 design 4.4).
	t.Run("a-commit-out-of-its-own-time", func(t *testing.T) {
		failed := fmt.Errorf("commit transaction: %w", context.DeadlineExceeded)
		x := newStartFixture(t, time.Minute, func(f *startFake) { f.commitErr = failed })
		_, err := x.start.Execute(x.ctx(), x.nb, "")
		if !errors.Is(err, failed) || startCode(err) != "" {
			t.Fatalf("answered %v, want the commit's error", err)
		}
		if lines := x.log.lines(t); len(lines) != 0 {
			t.Errorf("log %v", lines)
		}
		if slices.Contains(x.f.trace(), "analyze") {
			t.Error("analyzed a baseline not committed")
		}
		x.slotFree(t)
	})
	t.Run("the-statistics-fail", func(t *testing.T) {
		x := newStartFixture(t, time.Second, func(f *startFake) { f.analyzeErr = errors.New("analyze failed") })
		if _, err := x.start.Execute(x.ctx(), x.nb, ""); err != nil {
			t.Fatal(err)
		}
		lines := x.log.lines(t)
		if len(lines) != 2 || lines[0]["level"] != "WARN" || lines[0]["msg"] != "upgrade baseline not analyzed" ||
			lines[0]["error"] != "analyze failed" || lines[0]["upgrade_id"] == nil || lines[1]["msg"] != "upgrade started" {
			t.Errorf("log %v", lines)
		}
	})
}
