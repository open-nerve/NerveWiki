package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintExecNotebooks struct {
	workspace func(context.Context, uuid.UUID) (uuid.UUID, bool, error)
	state     func(context.Context, uuid.UUID) (NotebookState, bool, error)
}

func (n lintExecNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return n.workspace(ctx, id)
}
func (n lintExecNotebooks) StateOf(ctx context.Context, id uuid.UUID) (NotebookState, bool, error) {
	return n.state(ctx, id)
}

type lintExecAuth func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error)

func (f lintExecAuth) Authorize(ctx context.Context, a shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
	return f(ctx, a, action, target)
}

type lintExecSources func(context.Context, uuid.UUID, uuid.UUID) (SourceData, error)

func (f lintExecSources) LintData(ctx context.Context, notebook, root uuid.UUID) (SourceData, error) {
	return f(ctx, notebook, root)
}

type lintExecNodes func(context.Context, uuid.UUID) ([]CompactNode, error)

func (f lintExecNodes) Nodes(ctx context.Context, notebook uuid.UUID) ([]CompactNode, error) {
	return f(ctx, notebook)
}

type lintExecPaths func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error)

func (f lintExecPaths) Paths(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return f(ctx, notebook, ids)
}

type lintExecRules func(context.Context, ReadInput, RuleQuery) (PreparedRules, error)

func (f lintExecRules) PrepareRules(ctx context.Context, input ReadInput, query RuleQuery) (PreparedRules, error) {
	return f(ctx, input, query)
}

type lintExecSnapshots func(context.Context, func(context.Context) error) error

func (f lintExecSnapshots) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	return f(ctx, fn)
}

type lintExecClock func() time.Time

func (f lintExecClock) Now() time.Time { return f() }

type lintExecCursors struct {
	decode func(string) (DecodedCursor, error)
	encode func(LintScope, domain.LintRule, domain.LintPosition) (string, error)
}

func (f lintExecCursors) Decode(raw string) (DecodedCursor, error) { return f.decode(raw) }
func (f lintExecCursors) Encode(scope LintScope, rule domain.LintRule, pos domain.LintPosition) (string, error) {
	return f.encode(scope, rule, pos)
}

type lintExecDecoded func(LintScope, domain.LintRule) (domain.LintPosition, error)

func (f lintExecDecoded) PositionFor(scope LintScope, rule domain.LintRule) (domain.LintPosition, error) {
	return f(scope, rule)
}

type lintExecPrepared struct {
	stale   int64
	counts  map[domain.LintRule]domain.LintCount
	items   map[domain.LintRule][]domain.LintItem
	more    map[domain.LintRule]bool
	timings PreparationTimings
}

func (p *lintExecPrepared) StalePages() int64 { return p.stale }
func (p *lintExecPrepared) RuleCount(ctx context.Context, rule domain.LintRule) (domain.LintCount, error) {
	return p.counts[rule], ctx.Err()
}
func (p *lintExecPrepared) RuleItems(ctx context.Context, rule domain.LintRule) ([]domain.LintItem, bool, error) {
	return p.items[rule], p.more[rule], ctx.Err()
}
func (p *lintExecPrepared) Timings() PreparationTimings { return p.timings }

type lintExecTrace struct {
	mu    sync.Mutex
	calls []string
}

func (x *lintExecTrace) add(call string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.calls = append(x.calls, call)
}
func (x *lintExecTrace) copy() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.calls)
}

type lintExecSnapshotKey struct{}

func lintExecPtr[T any](value T) *T { return &value }

func lintExecID(n byte) uuid.UUID { var id uuid.UUID; id[15] = n; return id }

func lintExecActor(ctx context.Context, id byte) context.Context {
	return shared.WithActor(ctx, shared.Actor{UserID: lintExecID(id), SessionID: lintExecID(id + 100)})
}

func lintExecFixture(t *testing.T) (LintDeps, *lintExecTrace, *lintRunEvents, *lintRunLog) {
	t.Helper()
	trace := &lintExecTrace{}
	runs, events, log := lintRunFixture(t, 4, time.Second)
	deps := LintDeps{
		Notebooks: lintExecNotebooks{
			workspace: func(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
				trace.add("workspace")
				return lintExecID(2), true, nil
			},
			state: func(context.Context, uuid.UUID) (NotebookState, bool, error) {
				trace.add("state")
				return NotebookState{Mode: domain.ModeStandard}, true, nil
			},
		},
		Authorizer: lintExecAuth(func(ctx context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
			trace.add("authorize")
			if action != domain.ActionLintRead || target.WorkspaceID != lintExecID(2) || target.NotebookID != lintExecID(1) {
				return shared.Grant{}, errors.New("wrong authorization action or target")
			}
			return shared.Grant{}, ctx.Err()
		}),
		Sources: lintExecSources(func(context.Context, uuid.UUID, uuid.UUID) (SourceData, error) {
			trace.add("sources")
			return SourceData{}, nil
		}),
		Nodes: lintExecNodes(func(context.Context, uuid.UUID) ([]CompactNode, error) {
			trace.add("nodes")
			return nil, nil
		}),
		Rules: lintExecRules(func(context.Context, ReadInput, RuleQuery) (PreparedRules, error) {
			trace.add("prepare")
			return &lintExecPrepared{}, nil
		}),
		Paths: lintExecPaths(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error) {
			trace.add("paths")
			return nil, nil
		}),
		Snapshots: lintExecSnapshots(func(ctx context.Context, fn func(context.Context) error) error {
			trace.add("begin")
			err := fn(context.WithValue(ctx, lintExecSnapshotKey{}, true))
			trace.add("end")
			return err
		}),
		Clock: lintExecClock(func() time.Time {
			trace.add("clock")
			return time.Date(2026, 10, 30, 14, 0, 0, 123, time.FixedZone("fixture", 8*3600))
		}),
		Cursors: lintExecCursors{
			decode: func(raw string) (DecodedCursor, error) {
				trace.add("decode")
				if raw == "bad" {
					return nil, shared.InvalidCursor()
				}
				return lintExecDecoded(func(LintScope, domain.LintRule) (domain.LintPosition, error) {
					trace.add("bind")
					if raw == "foreign" {
						return domain.LintPosition{}, shared.InvalidCursor()
					}
					return domain.LintPosition{Begin: true}, nil
				}), nil
			},
			encode: func(_ LintScope, rule domain.LintRule, pos domain.LintPosition) (string, error) {
				trace.add("encode")
				if !pos.Begin {
					return "", errors.New("expected distinct start position")
				}
				return "begin:" + string(rule), nil
			},
		},
		Runs: runs,
	}
	return deps, trace, events, log
}

func TestLintJudgesInTurn(t *testing.T) {
	for _, tc := range []struct {
		name, rule, cursor                string
		summary, hidden, missing, noActor bool
		limit                             *int
		code                              string
		status                            int
		calls                             []string
	}{
		{name: "actor-first", limit: lintExecPtr(0), rule: "unknown", cursor: "bad", noActor: true, code: "unauthorized", status: 401},
		{name: "known-foreign-hidden", limit: lintExecPtr(0), rule: "wanted", cursor: "foreign", hidden: true, code: "bad_request", status: 400, calls: []string{"decode", "bind"}},
		{name: "known-foreign-visible", limit: lintExecPtr(0), rule: "orphans", cursor: "foreign", code: "bad_request", status: 400, calls: []string{"decode", "bind"}},
		{name: "known-valid-hidden", rule: "wanted", cursor: "valid", hidden: true, limit: lintExecPtr(0), code: "notebook.not_found", status: 404, calls: []string{"decode", "bind", "workspace", "authorize"}},
		{name: "unknown-bad-hidden", limit: lintExecPtr(0), rule: "secret-unknown", cursor: "bad", hidden: true, code: "bad_request", status: 400, calls: []string{"decode"}},
		{name: "unknown-valid-hidden", limit: lintExecPtr(0), rule: "secret-unknown", cursor: "foreign", hidden: true, code: "notebook.not_found", status: 404, calls: []string{"decode", "workspace", "authorize"}},
		{name: "unknown-valid-visible", limit: lintExecPtr(0), rule: "secret-unknown", cursor: "foreign", code: "wiki.lint_rule_not_found", status: 404, calls: []string{"decode", "workspace", "authorize"}},
		{name: "unknown-missing", limit: lintExecPtr(0), rule: "secret-unknown", missing: true, code: "notebook.not_found", status: 404, calls: []string{"workspace"}},
		{name: "summary-bad", summary: true, cursor: "bad", code: "bad_request", status: 400, calls: []string{"decode"}},
		{name: "summary-valid-hidden", summary: true, cursor: "foreign", hidden: true, code: "notebook.not_found", status: 404, calls: []string{"decode", "workspace", "authorize"}},
		{name: "summary-valid-visible", summary: true, cursor: "foreign", code: "validation_failed", status: 422, calls: []string{"decode", "workspace", "authorize"}},
		{name: "summary-limit", summary: true, limit: lintExecPtr(5), code: "validation_failed", status: 422, calls: []string{"workspace", "authorize"}},
		{name: "exclusive-before-limit", rule: "missing_type", limit: lintExecPtr(0), code: "notebook.not_llm_wiki", status: 409, calls: []string{"workspace", "authorize", "state"}},
		{name: "common-invalid-limit", rule: "wanted", limit: lintExecPtr(0), code: "validation_failed", status: 422, calls: []string{"workspace", "authorize"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, trace, events, _ := lintExecFixture(t)
			if tc.hidden {
				deps.Authorizer = lintExecAuth(func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
					trace.add("authorize")
					return shared.Grant{}, fmt.Errorf("wrapped: %w", shared.ErrNotVisible)
				})
			}
			if tc.missing {
				n := deps.Notebooks.(lintExecNotebooks)
				n.workspace = func(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
					trace.add("workspace")
					return uuid.UUID{}, false, nil
				}
				deps.Notebooks = n
			}
			query := LintQuery{Limit: tc.limit}
			if !tc.summary {
				query.Rule = &tc.rule
			}
			if tc.cursor != "" {
				query.Cursor = &tc.cursor
			}
			ctx := t.Context()
			if !tc.noActor {
				ctx = lintExecActor(ctx, 10)
			}
			got, err := NewLint(deps).Execute(ctx, LintScope{NotebookID: lintExecID(1)}, query)
			var problem *shared.Error
			if !errors.As(err, &problem) || problem.ProblemStatus() != tc.status || problem.Code != tc.code {
				t.Fatalf("judgment precedence: got %v; want %d/%s", err, tc.status, tc.code)
			}
			if !reflect.DeepEqual(got, domain.LintResult{}) {
				t.Fatalf("error exposed partial result: %+v", got)
			}
			if calls := trace.copy(); !slices.Equal(calls, tc.calls) {
				t.Fatalf("judgment calls=%v; want %v", calls, tc.calls)
			}
			select {
			case event := <-events.c:
				t.Fatalf("invalid request reached Runs: %+v", event)
			default:
			}
		})
	}
}

func TestLintCallersKeepTheirAuthority(t *testing.T) {
	deps, _, events, _ := lintExecFixture(t)
	contexts := map[uuid.UUID]context.Context{}
	for _, id := range []byte{10, 11, 12} {
		contexts[lintExecID(id)] = lintExecActor(t.Context(), id)
	}
	var mu sync.Mutex
	counts := map[uuid.UUID]int{}
	revoked := false
	deps.Authorizer = lintExecAuth(func(ctx context.Context, actor shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
		mu.Lock()
		defer mu.Unlock()
		counts[actor.UserID]++
		if ctx != contexts[actor.UserID] || ctx.Value(lintExecSnapshotKey{}) != nil || action != domain.ActionLintRead || target.NotebookID != lintExecID(1) {
			return shared.Grant{}, errors.New("authorization reused worker or another caller")
		}
		if actor.UserID == lintExecID(11) || (actor.UserID == lintExecID(12) && revoked) {
			return shared.Grant{}, shared.ErrNotVisible
		}
		return shared.Grant{}, nil
	})
	release := make(chan struct{})
	defer close(release)
	var preparations atomic.Int32
	deps.Rules = lintExecRules(func(context.Context, ReadInput, RuleQuery) (PreparedRules, error) {
		preparations.Add(1)
		<-release
		return &lintExecPrepared{}, nil
	})
	l := NewLint(deps)
	scope := LintScope{NotebookID: lintExecID(1)}
	a := lintExecAsync(l, contexts[lintExecID(10)], scope, LintQuery{})
	flight := events.take(t, RunAcquired, 0)
	b := lintExecAsync(l, contexts[lintExecID(11)], scope, LintQuery{})
	if answer := lintRunTake(t, b); !errors.Is(answer.err, domain.ErrNotebookNotFound) {
		t.Fatalf("invisible follower admitted: %+v", answer)
	}
	c := lintExecAsync(l, contexts[lintExecID(12)], scope, LintQuery{})
	events.take(t, RunJoined, flight.FlightID)
	mu.Lock()
	revoked = true
	mu.Unlock()
	release <- struct{}{}
	if answer := lintRunTake(t, a); answer.err != nil || answer.result.Mode != domain.ModeStandard {
		t.Fatalf("authorized leader=%+v", answer)
	}
	if answer := lintRunTake(t, c); !errors.Is(answer.err, domain.ErrNotebookNotFound) || !reflect.DeepEqual(answer.result, domain.LintResult{}) {
		t.Fatalf("revoked follower received result: %+v", answer)
	}
	mu.Lock()
	defer mu.Unlock()
	if counts[lintExecID(10)] != 2 || counts[lintExecID(11)] != 1 || counts[lintExecID(12)] != 2 || preparations.Load() != 1 {
		t.Fatalf("per-caller authority counts=%v, preparations=%d", counts, preparations.Load())
	}
}

func TestLintCancellationAtDelivery(t *testing.T) {
	deps, _, _, _ := lintExecFixture(t)
	ctx, cancel := context.WithCancel(lintExecActor(t.Context(), 10))
	defer cancel()
	var calls int
	deps.Authorizer = lintExecAuth(func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return shared.Grant{}, nil
	})
	result, err := NewLint(deps).Execute(ctx, LintScope{NotebookID: lintExecID(1)}, LintQuery{})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, domain.LintResult{}) || calls != 2 {
		t.Fatalf("canceled delivery=%+v/%v, calls=%d", result, err, calls)
	}
}

func TestLintScopesNeverShare(t *testing.T) {
	deps, _, events, _ := lintExecFixture(t)
	release := make(chan struct{})
	defer close(release)
	var preparations atomic.Int32
	deps.Rules = lintExecRules(func(_ context.Context, in ReadInput, q RuleQuery) (PreparedRules, error) {
		preparations.Add(1)
		<-release
		label := "implicit-standard"
		if in.Explicit {
			if in.EffectiveMode != domain.ModeLLMWiki || in.AreaRootID == nil {
				return nil, errors.New("wrong proposed mode")
			}
			switch *in.AreaRootID {
			case lintExecID(20):
				label = "proposed-A"
			case lintExecID(21):
				label = "proposed-B"
			default:
				return nil, errors.New("wrong proposed root")
			}
		}
		if q.Rule == nil || *q.Rule != domain.RuleWanted {
			return nil, errors.New("wrong rule")
		}
		return &lintExecPrepared{counts: map[domain.LintRule]domain.LintCount{domain.RuleWanted: {Value: 1}}, items: map[domain.LintRule][]domain.LintItem{
			domain.RuleWanted: {{Rule: domain.RuleWanted, ItemKey: "w:" + label, Cursor: "after:" + label, Target: label, Pages: 1}},
		}}, nil
	})
	l := NewLint(deps)
	ctx := lintExecActor(t.Context(), 10)
	query := LintQuery{Rule: lintExecPtr("wanted")}
	implicit := lintExecAsync(l, ctx, LintScope{NotebookID: lintExecID(1)}, query)
	f0 := events.take(t, RunCreated, 0)
	explicitA := lintExecAsync(l, ctx, LintScope{NotebookID: lintExecID(1), Contract: lintExecPtr(domain.ModeLLMWiki), AreaRootID: lintExecPtr(lintExecID(20))}, query)
	fA := events.take(t, RunCreated, 0)
	explicitB := lintExecAsync(l, ctx, LintScope{NotebookID: lintExecID(1), Contract: lintExecPtr(domain.ModeLLMWiki), AreaRootID: lintExecPtr(lintExecID(21))}, query)
	fB := events.take(t, RunCreated, 0)
	if f0.FlightID == fA.FlightID || fA.FlightID == fB.FlightID {
		t.Fatal("distinct scopes reused a flight")
	}
	followerA := lintExecAsync(l, lintExecActor(t.Context(), 12), LintScope{NotebookID: lintExecID(1), Contract: lintExecPtr(domain.ModeLLMWiki), AreaRootID: lintExecPtr(lintExecID(20))}, query)
	events.take(t, RunJoined, fA.FlightID)
	for range 3 {
		release <- struct{}{}
	}
	for _, tc := range []struct {
		done   <-chan lintRunAnswer
		mode   domain.Mode
		target string
	}{
		{implicit, domain.ModeStandard, "implicit-standard"}, {explicitA, domain.ModeLLMWiki, "proposed-A"},
		{explicitB, domain.ModeLLMWiki, "proposed-B"}, {followerA, domain.ModeLLMWiki, "proposed-A"},
	} {
		answer := lintRunTake(t, tc.done)
		if answer.err != nil || answer.result.Mode != tc.mode || len(answer.result.Rules) != 1 || len(answer.result.Rules[0].Items) != 1 || answer.result.Rules[0].Items[0].Target != tc.target {
			t.Fatalf("scope result=%+v; want %s/%s", answer, tc.mode, tc.target)
		}
	}
	if preparations.Load() != 3 {
		t.Fatalf("scope preparations=%d, want 3", preparations.Load())
	}
}

func TestLintOwnsQueuedInputs(t *testing.T) {
	deps, _, _, _ := lintExecFixture(t)
	runs, events, _ := lintRunFixture(t, 1, time.Second)
	deps.Runs = runs
	release := make(chan struct{})
	defer close(release)
	block := lintRunAsync(runs, t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) { <-release; return RunOutput{}, nil })
	blocking := events.take(t, RunAcquired, 0)
	events.take(t, RunCreated, blocking.FlightID)
	deps.Rules = lintExecRules(func(_ context.Context, in ReadInput, query RuleQuery) (PreparedRules, error) {
		if in.EffectiveMode != domain.ModeLLMWiki || !in.Explicit || in.AreaRootID == nil || *in.AreaRootID != lintExecID(20) ||
			query.Rule == nil || *query.Rule != domain.RuleWanted || query.Limit != 3 || query.After == nil || !query.After.Begin {
			return nil, errors.New("queued caller changed worker input")
		}
		return &lintExecPrepared{}, nil
	})
	mode, root, rule, limit, cursor := domain.ModeLLMWiki, lintExecID(20), "wanted", 3, "valid"
	done := lintExecAsync(NewLint(deps), lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1), Contract: &mode, AreaRootID: &root}, LintQuery{Rule: &rule, Limit: &limit, Cursor: &cursor})
	events.take(t, RunCreated, 0)
	mode, root, rule, limit, cursor = domain.ModeStandard, lintExecID(21), "orphans", 1, "bad"
	release <- struct{}{}
	if answer := lintRunTake(t, block); answer.err != nil {
		t.Fatal(answer.err)
	}
	if answer := lintRunTake(t, done); answer.err != nil || answer.result.Mode != domain.ModeLLMWiki {
		t.Fatalf("queued input ownership=%+v", answer)
	}
}

func TestLintScopeStructure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contract *domain.Mode
		root     *uuid.UUID
		valid    bool
		mode     domain.Mode
	}{
		{"implicit-root", nil, lintExecPtr(uuid.UUID{}), false, ""},
		{"standard-root", lintExecPtr(domain.ModeStandard), lintExecPtr(uuid.UUID{}), false, ""},
		{"llm-without-root", lintExecPtr(domain.ModeLLMWiki), nil, false, ""},
		{"unknown", lintExecPtr(domain.Mode("secret-mode")), nil, false, ""},
		{"explicit-standard", lintExecPtr(domain.ModeStandard), nil, true, domain.ModeStandard},
		{"explicit-llm-zero-root", lintExecPtr(domain.ModeLLMWiki), lintExecPtr(uuid.UUID{}), true, domain.ModeLLMWiki},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, trace, _, _ := lintExecFixture(t)
			result, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1), Contract: tc.contract, AreaRootID: tc.root}, LintQuery{})
			if tc.valid {
				if err != nil || result.Mode != tc.mode {
					t.Fatalf("valid proposed scope=%+v/%v", result, err)
				}
				return
			}
			var problem *shared.Error
			if err == nil || errors.As(err, &problem) || !reflect.DeepEqual(result, domain.LintResult{}) || len(trace.copy()) != 0 {
				t.Fatalf("invalid internal scope=%+v/%v, calls=%v", result, err, trace.copy())
			}
		})
	}
}

// Each subtest owns its logger, secrets and actual owner. These are app-port
// witnesses; real bootstrap authorization/SQL remain separate integration tests.
func TestLintLogsItsRunNotItsInputs(t *testing.T) {
	t.Run("successful-followers", func(t *testing.T) {
		deps, _, events, log := lintExecFixture(t)
		secrets := []string{"success-raw-target-secret", "success-canonical-secret", "success-path-secret", "success-json-secret", "success-cursor-secret"}
		n := deps.Notebooks.(lintExecNotebooks)
		n.state = func(context.Context, uuid.UUID) (NotebookState, bool, error) {
			return NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: lintExecID(4)}, true, nil
		}
		deps.Notebooks = n
		release := make(chan struct{})
		defer close(release)
		deps.Rules = lintExecRules(func(context.Context, ReadInput, RuleQuery) (PreparedRules, error) {
			<-release
			return &lintExecPrepared{stale: 7, counts: map[domain.LintRule]domain.LintCount{domain.RuleInvalidSources: {Value: 1}},
				items: map[domain.LintRule][]domain.LintItem{domain.RuleInvalidSources: {{Rule: domain.RuleInvalidSources, PageID: lintExecPtr(lintExecID(6)),
					ItemKey: "s:identity", Cursor: "after:identity", Target: secrets[0] + secrets[3], NotLink: true}}},
				timings: PreparationTimings{SharedSources: lintExecPtr(time.Nanosecond), Rules: []RuleTiming{{Rule: domain.RuleInvalidSources, Duration: time.Nanosecond}}}}, nil
		})
		deps.Paths = lintExecPaths(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error) {
			return map[uuid.UUID]string{lintExecID(6): secrets[2]}, nil
		})
		codec := deps.Cursors.(lintExecCursors)
		codec.encode = func(LintScope, domain.LintRule, domain.LintPosition) (string, error) { return secrets[1], nil }
		deps.Cursors = codec
		l := NewLint(deps)
		query := LintQuery{Rule: lintExecPtr("invalid_sources"), Cursor: &secrets[4]}
		a := lintExecAsync(l, lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, query)
		flight := events.take(t, RunAcquired, 0)
		b := lintExecAsync(l, lintExecActor(t.Context(), 12), LintScope{NotebookID: lintExecID(1)}, query)
		events.take(t, RunJoined, flight.FlightID)
		release <- struct{}{}
		for _, done := range []<-chan lintRunAnswer{a, b} {
			answer := lintRunTake(t, done)
			if answer.err != nil || len(answer.result.Rules) != 1 || len(answer.result.Rules[0].Items) != 1 || answer.result.Rules[0].Items[0].Path == nil ||
				*answer.result.Rules[0].Items[0].Path != secrets[2] || answer.result.Rules[0].Items[0].Target != secrets[0]+secrets[3] {
				t.Fatalf("secret fixture did not reach result: %+v", answer)
			}
		}
		events.take(t, RunFinished, flight.FlightID)
		entry := lintExecLogEntry(t, log, secrets)
		if entry["msg"] != "lint computed" || entry["mode"] != "llm_wiki" || entry["rule"] != "invalid_sources" || entry["stale_pages"] != float64(7) {
			t.Fatalf("observed run log=%v", entry)
		}
		for _, field := range []string{"prepare_ms", "shared_sources_ms", "wait_ms"} {
			if v, ok := entry[field].(float64); !ok || v < 0 {
				t.Fatalf("missing observed timing %s: %v", field, entry)
			}
		}
		rules, ok := entry["rules"].(map[string]any)
		if !ok || len(rules) != 1 {
			t.Fatalf("exclusive rule statistics=%v", entry)
		}
		stat, ok := rules["invalid_sources"].(map[string]any)
		if !ok || stat["count"] != float64(1) || stat["capped"] != false || stat["duration_ms"] != float64(0) {
			t.Fatalf("observed rule statistics=%v", entry)
		}
	})
	for _, phase := range []string{"prepare-error", "panic", "expiry"} {
		t.Run(phase, func(t *testing.T) {
			deps, _, events, log := lintExecFixture(t)
			secret := "only-" + phase + "-json-error-secret"
			cause := errors.New(secret)
			release := make(chan struct{})
			defer close(release)
			if phase == "expiry" {
				runs, e, l := lintRunFixture(t, 1, 30*time.Millisecond)
				deps.Runs, events, log = runs, e, l
			}
			deps.Rules = lintExecRules(func(ctx context.Context, _ ReadInput, _ RuleQuery) (PreparedRules, error) {
				switch phase {
				case "panic":
					panic(secret)
				case "expiry":
					<-ctx.Done()
					<-release
					return nil, ctx.Err()
				}
				return nil, cause
			})
			done := lintExecAsync(NewLint(deps), lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{})
			flight := events.take(t, RunCreated, 0)
			if phase == "expiry" {
				events.take(t, RunExpired, flight.FlightID)
			}
			answer := lintRunTake(t, done)
			if answer.err == nil || !reflect.DeepEqual(answer.result, domain.LintResult{}) {
				t.Fatalf("failure result=%+v", answer)
			}
			if phase == "prepare-error" && !errors.Is(answer.err, cause) {
				t.Fatalf("ordinary error reclassified: %v", answer.err)
			}
			if phase == "expiry" {
				lintRunBusy(t, answer.err)
				if log.String() != "" {
					t.Fatal("expiry logged before worker cleanup")
				}
				release <- struct{}{}
			}
			events.take(t, RunFinished, flight.FlightID)
			entry := lintExecLogEntry(t, log, []string{secret})
			if entry["msg"] != "lint stopped" {
				t.Fatalf("failure log=%v", entry)
			}
			for _, field := range []string{"stale_pages", "shared_sources_ms", "rules"} {
				if _, present := entry[field]; present {
					t.Fatalf("unavailable %s fabricated: %v", field, entry)
				}
			}
			if phase == "panic" {
				if _, present := entry["prepare_ms"]; present || entry["mode"] != "unknown" || entry["fault"] != "panic" {
					t.Fatalf("panic fabricated callback stats: %v", entry)
				}
			} else if _, present := entry["prepare_ms"]; !present || entry["mode"] != "standard" {
				t.Fatalf("observed failure statistics missing: %v", entry)
			}
		})
	}
	t.Run("unknown-rule", func(t *testing.T) {
		deps, _, _, log := lintExecFixture(t)
		secret := "only-unknown-rule-secret"
		_, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{Rule: &secret})
		if !errors.Is(err, domain.ErrLintRuleNotFound) || strings.Contains(err.Error(), secret) || log.String() != "" {
			t.Fatalf("unvalidated rule reached run log: %v / %s", err, log.String())
		}
	})
}

func lintExecLogEntry(t *testing.T, log *lintRunLog, secrets []string) map[string]any {
	t.Helper()
	raw := strings.TrimSpace(log.String())
	for _, secret := range secrets {
		if strings.Contains(raw, secret) {
			t.Fatalf("run log leaked this subtest's secret %q: %s", secret, raw)
		}
	}
	if raw == "" || strings.Count(raw, "\n") != 0 {
		t.Fatalf("want one final worker log, got %q", raw)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}
