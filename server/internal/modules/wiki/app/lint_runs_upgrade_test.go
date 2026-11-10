package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// upgradeRunKey is a key of an upgrade's run in the explicit LLM contract
// whose source root is root (M11/P2 design 3.8): kind, scope, and a rule's
// page for the items.
func upgradeRunKey(kind domain.LintQueryKind, root, scope uuid.UUID) RunKey {
	key := RunKey{NotebookID: lintRunKey().NotebookID, Explicit: true, Contract: domain.ModeLLMWiki, HasRoot: true, RootID: root,
		Kind: kind, Scope: scope}
	if kind == domain.LintUpgradeItems {
		key.Rule, key.Limit = domain.RuleMissingType, 50
	}
	return key
}

// nextFlight is the flight a caller's Do made or joined: the next creation
// or join, other events skipped.
func nextFlight(t *testing.T, events *lintRunEvents) RunEvent {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case e := <-events.c:
			if e.Kind == RunCreated || e.Kind == RunJoined {
				return e
			}
		case <-timeout:
			t.Fatal("no flight made or joined")
			return RunEvent{}
		}
	}
}

// Checks of one upgrade join; two upgrades' checks, a check and the items
// of one upgrade, anything and a submission, never; nor the items of two
// upgrades that propose one source root, one after the other (M11 design
// 4.7, B-I1, B2-N2).
func TestTheUpgradeKeysJoinOnlyTheirOwn(t *testing.T) {
	r, events, _ := lintRunFixture(t, 8, 2*time.Second)
	release := make(chan struct{})
	work := func(context.Context) (RunOutput, error) {
		<-release
		return RunOutput{Result: domain.LintResult{Mode: domain.ModeLLMWiki}, Check: &domain.UpgradeCheck{Passed: true}}, nil
	}
	root := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	one, two := uuid.MustParse("44444444-4444-4444-8444-444444444444"), uuid.MustParse("55555555-5555-4555-8555-555555555555")
	var answers []<-chan lintRunAnswer
	call := func(what string, key RunKey, want RunEventKind, of uint64) uint64 {
		t.Helper()
		answers = append(answers, lintRunAsync(r, t.Context(), key, work))
		e := nextFlight(t, events)
		if e.Kind != want || want == RunJoined && e.FlightID != of {
			t.Fatalf("%s: event %d of flight %d, want %d of %d", what, e.Kind, e.FlightID, want, of)
		}
		return e.FlightID
	}
	check := call("one upgrade's check", upgradeRunKey(domain.LintUpgradeCheck, root, one), RunCreated, 0)
	call("the same upgrade's check again", upgradeRunKey(domain.LintUpgradeCheck, root, one), RunJoined, check)
	call("another upgrade's check", upgradeRunKey(domain.LintUpgradeCheck, root, two), RunCreated, 0)
	items := call("one upgrade's items", upgradeRunKey(domain.LintUpgradeItems, root, one), RunCreated, 0)
	call("the same items again", upgradeRunKey(domain.LintUpgradeItems, root, one), RunJoined, items)
	call("the items of the next upgrade of the same root", upgradeRunKey(domain.LintUpgradeItems, root, two), RunCreated, 0)
	call("a submission of the upgrade, keyed by its id", upgradeRunKey(domain.LintUpgradeSubmit, root, one), RunCreated, 0)
	submission := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	call("a submission", upgradeRunKey(domain.LintUpgradeSubmit, root, submission), RunCreated, 0)
	call("lint's explicit summary of the same scope", upgradeRunKey(domain.LintSummary, root, uuid.UUID{}), RunCreated, 0)
	close(release)
	for _, done := range answers {
		if answer := lintRunTake(t, done); answer.err != nil || answer.check == nil || !answer.check.Passed {
			t.Fatalf("an upgrade's run answered %+v", answer)
		}
	}
}

// Each caller of a run gets a check of its own (M11/P2 design 3.8): one
// that changes its slices and map changes nothing of another's.
func TestTheRunsCopyACheck(t *testing.T) {
	r, events, _ := lintRunFixture(t, 1, time.Second)
	release := make(chan struct{})
	defer close(release)
	id := uuid.MustParse("77777777-7777-4777-8777-777777777777")
	page, path := id, "Notes/Plan.md"
	work := func(context.Context) (RunOutput, error) {
		<-release
		return RunOutput{Check: &domain.UpgradeCheck{
			Gates: domain.UpgradeGates{ReservedTitles: domain.NodesGate{NodeIDs: []uuid.UUID{id}},
				Links: domain.LinksGate{PagesWorse: 1, NodeIDs: []uuid.UUID{id}}, Preserved: domain.PreservedGate{NodeIDs: []uuid.UUID{id}}},
			Rules: []domain.LintRuleResult{{Rule: domain.RuleOrphans, Items: []domain.LintItem{{PageID: &page, Path: &path}}}},
			Paths: map[uuid.UUID]string{id: path},
		}}, nil
	}
	key := upgradeRunKey(domain.LintUpgradeCheck, id, id)
	a := lintRunAsync(r, t.Context(), key, work)
	flight := events.take(t, RunAcquired, 0)
	b := lintRunAsync(r, t.Context(), key, work)
	events.take(t, RunJoined, flight.FlightID)
	release <- struct{}{}
	x, y := lintRunTake(t, a), lintRunTake(t, b)
	if x.err != nil || y.err != nil || x.check == nil || y.check == nil {
		t.Fatalf("callers answered %+v, %+v", x, y)
	}
	other := uuid.MustParse("88888888-8888-4888-8888-888888888888")
	x.check.Gates.ReservedTitles.NodeIDs[0] = other
	x.check.Gates.Links.NodeIDs[0] = other
	x.check.Gates.Preserved.NodeIDs[0] = other
	*x.check.Rules[0].Items[0].Path = "cut"
	*x.check.Rules[0].Items[0].PageID = other
	x.check.Paths[id] = "cut"
	x.check.Rules[0].Items = nil
	g := y.check.Gates
	if g.ReservedTitles.NodeIDs[0] != id || g.Links.NodeIDs[0] != id || g.Preserved.NodeIDs[0] != id || y.check.Paths[id] != path ||
		len(y.check.Rules[0].Items) != 1 || *y.check.Rules[0].Items[0].Path != path || *y.check.Rules[0].Items[0].PageID != id {
		t.Fatalf("one caller's change of its check changed another's: %+v", y.check)
	}
}

// The runs' line names an upgrade's three kinds, and their scope, the
// upgrade's or the submission's id: never unknown (M11/P2 design 3.8,
// 3.16; B2-N3).
func TestTheUpgradeKindsAreNamed(t *testing.T) {
	scope := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	for _, tc := range []struct {
		kind       domain.LintQueryKind
		name, rule string
	}{
		{domain.LintUpgradeCheck, "upgrade_check", "summary"},
		{domain.LintUpgradeItems, "upgrade_items", "missing_type"},
		{domain.LintUpgradeSubmit, "upgrade_submit", "summary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, events, log := lintRunFixture(t, 1, time.Second)
			if _, err := r.Do(t.Context(), upgradeRunKey(tc.kind, scope, scope), func(context.Context) (RunOutput, error) {
				return RunOutput{Result: domain.LintResult{Mode: domain.ModeLLMWiki}}, nil
			}); err != nil {
				t.Fatal(err)
			}
			events.take(t, RunFinished, 0)
			var line map[string]any
			for _, raw := range strings.Split(strings.TrimSpace(log.String()), "\n") {
				if err := json.Unmarshal([]byte(raw), &line); err != nil {
					t.Fatal(err)
				}
			}
			if line["msg"] != "lint computed" || line["query_kind"] != tc.name || line["rule"] != tc.rule ||
				line["upgrade_scope"] != scope.String() {
				t.Fatalf("the run's line = %v, want %s, %s and its scope", line, tc.name, tc.rule)
			}
		})
	}
	t.Run("lint's", func(t *testing.T) {
		r, events, log := lintRunFixture(t, 1, time.Second)
		if _, err := r.Do(t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) { return RunOutput{}, nil }); err != nil {
			t.Fatal(err)
		}
		events.take(t, RunFinished, 0)
		if strings.Contains(log.String(), "upgrade_scope") {
			t.Fatalf("lint's line has an upgrade's scope: %s", log.String())
		}
	})
}
