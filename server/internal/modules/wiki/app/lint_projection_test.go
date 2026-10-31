package app

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func lintExecOrdinal(n uint64) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], n)
	return id
}

func lintExecEight() []domain.LintRule {
	return []domain.LintRule{domain.RuleWanted, domain.RuleOrphans, domain.RuleAmbiguous, domain.RuleInvalidFrontmatter,
		domain.RuleMissingType, domain.RuleMissingSummary, domain.RuleUndigestedSources, domain.RuleInvalidSources}
}

func lintExecNonempty() *lintExecPrepared {
	p := &lintExecPrepared{stale: 4, counts: make(map[domain.LintRule]domain.LintCount),
		items: make(map[domain.LintRule][]domain.LintItem), more: make(map[domain.LintRule]bool)}
	for i, rule := range lintExecEight() {
		item := domain.LintItem{Rule: rule, ItemKey: "identity:" + string(rule), Cursor: "after:" + string(rule)}
		switch rule {
		case domain.RuleWanted:
			item.Target, item.Pages = "target-secret", 2
		case domain.RuleUndigestedSources:
			item.NodeID, item.Kind, item.Bytes = lintExecPtr(lintExecID(3)), domain.NodeAsset, 19
			item.CreatedAt = time.Date(2026, 10, 1, 1, 2, 3, 4, time.UTC)
		default:
			item.PageID = lintExecPtr(uuid.UUID{})
		}
		p.counts[rule] = domain.LintCount{Value: 1}
		p.items[rule] = []domain.LintItem{item}
		p.timings.Rules = append(p.timings.Rules, RuleTiming{Rule: rule, Duration: time.Duration(i + 11)})
	}
	p.timings.SharedSources = lintExecPtr(7 * time.Nanosecond)
	return p
}

func TestLintSnapshotOrder(t *testing.T) {
	t.Run("complete-source-and-final-paths", func(t *testing.T) {
		deps, trace, _, _ := lintExecFixture(t)
		n := deps.Notebooks.(lintExecNotebooks)
		n.state = func(ctx context.Context, id uuid.UUID) (NotebookState, bool, error) {
			trace.add("state")
			if ctx.Value(lintExecSnapshotKey{}) != true || id != lintExecID(1) {
				return NotebookState{}, false, errors.New("state outside snapshot")
			}
			return NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: uuid.UUID{}}, true, nil
		}
		deps.Notebooks = n
		sources := make([]SourceNode, 10002)
		for i := range sources {
			sources[i] = SourceNode{ID: lintExecOrdinal(uint64(i + 100)), Kind: domain.NodePage, PageBytes: 1}
		}
		sources[0].PageBytes, sources[1].Kind = 0, domain.NodeAsset
		parent := lintExecID(17)
		nodes := []CompactNode{{ID: lintExecID(3), ParentID: &parent, Kind: domain.NodeAsset, NameKey: "metadata", Linkable: true}}
		deps.Sources = lintExecSources(func(ctx context.Context, id, root uuid.UUID) (SourceData, error) {
			trace.add("sources")
			if ctx.Value(lintExecSnapshotKey{}) != true || id != lintExecID(1) || root != (uuid.UUID{}) {
				return SourceData{}, errors.New("wrong source context or zero root")
			}
			return SourceData{Sources: sources, Nodes: nodes}, nil
		})
		deps.Nodes = lintExecNodes(func(context.Context, uuid.UUID) ([]CompactNode, error) {
			return nil, errors.New("LLM reread metadata outside its source bundle")
		})
		prepared := lintExecNonempty()
		deps.Rules = lintExecRules(func(ctx context.Context, in ReadInput, q RuleQuery) (PreparedRules, error) {
			trace.add("prepare")
			if ctx.Value(lintExecSnapshotKey{}) != true || in.EffectiveMode != domain.ModeLLMWiki || in.Explicit || in.AreaRootID == nil || *in.AreaRootID != (uuid.UUID{}) ||
				in.NotebookID != lintExecID(1) || len(in.SourceIDs) != 10002 || q.Rule != nil || q.After != nil || q.Limit != 0 || !reflect.DeepEqual(in.Nodes, nodes) {
				return nil, errors.New("snapshot preparation omitted scope or metadata")
			}
			for i, id := range in.SourceIDs {
				if id != sources[i].ID {
					return nil, errors.New("complete sources changed")
				}
			}
			// The port does not own the original node pointers.
			*in.Nodes[0].ParentID = lintExecID(18)
			if parent != lintExecID(17) {
				return nil, errors.New("metadata pointer was borrowed")
			}
			in.Nodes[0].NameKey = "consumer changed"
			in.SourceIDs[0] = lintExecID(19)
			if nodes[0].NameKey != "metadata" || sources[0].ID != lintExecOrdinal(100) {
				return nil, errors.New("metadata slice was borrowed")
			}
			return prepared, nil
		})
		deps.Paths = lintExecPaths(func(ctx context.Context, id uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
			trace.add("paths")
			if ctx.Value(lintExecSnapshotKey{}) != true || id != lintExecID(1) || !slices.Equal(ids, []uuid.UUID{{}, lintExecID(3)}) {
				return nil, errors.New("final paths not deduplicated")
			}
			return map[uuid.UUID]string{{}: "page-secret", lintExecID(3): "asset-secret"}, nil
		})
		result, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Mode != domain.ModeLLMWiki || result.Kind != domain.LintSummary || result.StalePages != 4 ||
			result.ComputedAt != time.Date(2026, 10, 30, 6, 0, 0, 123, time.UTC) || len(result.Rules) != 8 {
			t.Fatalf("snapshot result=%+v", result)
		}
		for i, rule := range lintExecEight() {
			got := result.Rules[i]
			if got.Rule != rule || got.Count != (domain.LintCount{Value: 1}) || len(got.Items) != 1 || got.Items[0].ItemKey != "identity:"+string(rule) ||
				got.StartCursor == nil || *got.StartCursor != "begin:"+string(rule) || got.NextCursor != nil {
				t.Fatalf("rule projection=%+v", got)
			}
			wantPath := "page-secret"
			if rule == domain.RuleWanted {
				if got.Items[0].Path != nil || got.Items[0].Target != "target-secret" || got.Items[0].Pages != 2 {
					t.Fatalf("wanted projection=%+v", got)
				}
				continue
			}
			if rule == domain.RuleUndigestedSources {
				wantPath = "asset-secret"
			}
			if got.Items[0].Path == nil || *got.Items[0].Path != wantPath {
				t.Fatalf("path projection=%+v", got)
			}
		}
		wantCalls := []string{"workspace", "authorize", "begin", "state", "clock", "sources", "prepare"}
		for range 8 {
			wantCalls = append(wantCalls, "encode")
		}
		wantCalls = append(wantCalls, "paths", "end", "workspace", "authorize")
		if got := trace.copy(); !slices.Equal(got, wantCalls) {
			t.Fatalf("snapshot order=%v; want %v", got, wantCalls)
		}
		*result.Rules[1].Items[0].PageID = lintExecID(77)
		result.Rules[1].Items[0].Cursor = "changed"
		if *prepared.items[domain.RuleOrphans][0].PageID != (uuid.UUID{}) || prepared.items[domain.RuleOrphans][0].Cursor != "after:orphans" {
			t.Fatal("result retained prepared item pointers")
		}
	})
	t.Run("standard-empty-does-not-read-sources-or-paths", func(t *testing.T) {
		deps, trace, _, _ := lintExecFixture(t)
		result, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{})
		if err != nil {
			t.Fatal(err)
		}
		want := []domain.LintRule{domain.RuleWanted, domain.RuleOrphans, domain.RuleAmbiguous, domain.RuleInvalidFrontmatter}
		if len(result.Rules) != 4 {
			t.Fatalf("standard rules=%+v", result.Rules)
		}
		for i, rule := range want {
			if got := result.Rules[i]; got.Rule != rule || got.Items == nil || len(got.Items) != 0 || got.StartCursor != nil || got.NextCursor != nil {
				t.Fatalf("empty projection=%+v", got)
			}
		}
		if got := trace.copy(); !slices.Equal(got, []string{"workspace", "authorize", "begin", "state", "clock", "nodes", "prepare", "end", "workspace", "authorize"}) {
			t.Fatalf("empty snapshot order=%v", got)
		}
	})
	t.Run("queued-mode-is-rechecked", func(t *testing.T) {
		deps, trace, _, _ := lintExecFixture(t)
		runs, events, _ := lintRunFixture(t, 1, time.Second)
		deps.Runs = runs
		release := make(chan struct{})
		defer close(release)
		block := lintRunAsync(runs, t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) { <-release; return RunOutput{}, nil })
		blockFlight := events.take(t, RunAcquired, 0)
		events.take(t, RunCreated, blockFlight.FlightID)
		n := deps.Notebooks.(lintExecNotebooks)
		n.state = func(ctx context.Context, _ uuid.UUID) (NotebookState, bool, error) {
			trace.add("state")
			if ctx.Value(lintExecSnapshotKey{}) == true {
				return NotebookState{Mode: domain.ModeStandard}, true, nil
			}
			return NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: lintExecID(4)}, true, nil
		}
		deps.Notebooks = n
		done := lintExecAsync(NewLint(deps), lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{Rule: lintExecPtr("missing_type")})
		events.take(t, RunCreated, 0)
		release <- struct{}{}
		if a := lintRunTake(t, block); a.err != nil {
			t.Fatal(a.err)
		}
		answer := lintRunTake(t, done)
		if !errors.Is(answer.err, domain.ErrNotLLMWiki) || !reflect.DeepEqual(answer.result, domain.LintResult{}) {
			t.Fatalf("queued mode recheck=%+v", answer)
		}
		if calls := trace.copy(); slices.Contains(calls, "nodes") || slices.Contains(calls, "prepare") {
			t.Fatalf("invalid mode read rule data: %v", calls)
		}
	})
}

func TestLintProjectionRejectsPartialResults(t *testing.T) {
	for _, failure := range []string{"negative-stale", "invalid-count", "wrong-rule", "missing-identity", "missing-path", "too-many-items", "empty-more", "commit"} {
		t.Run(failure, func(t *testing.T) {
			deps, trace, _, _ := lintExecFixture(t)
			prepared := lintExecNonempty()
			rule := domain.RuleOrphans
			prepared.timings = PreparationTimings{}
			deps.Rules = lintExecRules(func(context.Context, ReadInput, RuleQuery) (PreparedRules, error) { return prepared, nil })
			deps.Paths = lintExecPaths(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error) {
				if failure == "missing-path" {
					return nil, nil
				}
				return map[uuid.UUID]string{{}: "page"}, nil
			})
			switch failure {
			case "negative-stale":
				prepared.stale = -1
			case "invalid-count":
				prepared.counts[rule] = domain.LintCount{Value: 10000}
			case "wrong-rule":
				prepared.items[rule][0].Rule = domain.RuleWanted
			case "missing-identity":
				prepared.items[rule][0].PageID = nil
			case "too-many-items":
				prepared.items[rule] = append(prepared.items[rule], prepared.items[rule][0])
				prepared.counts[rule] = domain.LintCount{Value: 2}
			case "empty-more":
				prepared.items[rule] = nil
				prepared.more[rule] = true
			case "commit":
				deps.Snapshots = lintExecSnapshots(func(ctx context.Context, fn func(context.Context) error) error {
					if err := fn(ctx); err != nil {
						return err
					}
					return errors.New("commit failure")
				})
			}
			got, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{Rule: lintExecPtr("orphans"), Limit: lintExecPtr(1)})
			if err == nil || !reflect.DeepEqual(got, domain.LintResult{}) {
				t.Fatalf("invalid projection exposed a result: %+v / %v", got, err)
			}
			authorizations := 0
			for _, call := range trace.copy() {
				if call == "authorize" {
					authorizations++
				}
			}
			if authorizations != 1 {
				t.Fatalf("failed result was delivered: %v", trace.copy())
			}
		})
	}
}

func TestLintListUsesActualLastCursor(t *testing.T) {
	deps, _, _, _ := lintExecFixture(t)
	prepared := &lintExecPrepared{counts: map[domain.LintRule]domain.LintCount{domain.RuleOrphans: {Value: 10000, Capped: true}},
		items: map[domain.LintRule][]domain.LintItem{domain.RuleOrphans: {
			{Rule: domain.RuleOrphans, PageID: lintExecPtr(uuid.UUID{}), ItemKey: "p:zero", Cursor: "after:zero"},
			{Rule: domain.RuleOrphans, PageID: lintExecPtr(lintExecID(6)), ItemKey: "p:six", Cursor: "after:six"},
		}}, more: map[domain.LintRule]bool{domain.RuleOrphans: true}}
	deps.Rules = lintExecRules(func(_ context.Context, _ ReadInput, q RuleQuery) (PreparedRules, error) {
		if q.Limit != 2 || q.Rule == nil || *q.Rule != domain.RuleOrphans || q.After == nil || !q.After.Begin {
			return nil, errors.New("list query lost position or limit")
		}
		return prepared, nil
	})
	deps.Paths = lintExecPaths(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error) {
		return map[uuid.UUID]string{{}: "zero", lintExecID(6): "six"}, nil
	})
	got, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{Rule: lintExecPtr("orphans"), Limit: lintExecPtr(2), Cursor: lintExecPtr("valid")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != domain.LintItems || len(got.Rules) != 1 || got.Rules[0].NextCursor == nil || *got.Rules[0].NextCursor != "after:six" ||
		got.Rules[0].StartCursor == nil || *got.Rules[0].StartCursor != "begin:orphans" || len(got.Rules[0].Items) != 2 || !got.Rules[0].Count.Capped {
		t.Fatalf("list continuation=%+v", got)
	}
}

func lintExecAsync(l *Lint, ctx context.Context, scope LintScope, query LintQuery) <-chan lintRunAnswer {
	done := make(chan lintRunAnswer, 1)
	go func() { result, err := l.Execute(ctx, scope, query); done <- lintRunAnswer{result, err} }()
	return done
}
