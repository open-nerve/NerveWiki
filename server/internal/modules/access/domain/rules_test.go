package domain

import (
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestEveryRuleIsOfALevelAndAllowsKnownRoles(t *testing.T) {
	for _, action := range RuleKeys() {
		r, ok := RuleFor(action)
		if !ok {
			t.Errorf("RuleFor(%q) has no rule, but RuleKeys lists it", action)
			continue
		}
		if r.Level != LevelWorkspace {
			t.Errorf("%s: level %d", action, r.Level)
		}
		if len(r.Workspace) == 0 {
			t.Errorf("%s allows no role", action)
		}
		for _, role := range r.Workspace {
			if !slices.Contains(shared.WorkspaceRoles(), role) {
				t.Errorf("%s allows %q, which is no workspace role", action, role)
			}
		}
	}
}

func TestRuleForAnUnknownAction(t *testing.T) {
	if r, ok := RuleFor("no.such.action"); ok {
		t.Errorf("RuleFor(no.such.action) = %+v, true", r)
	}
	if !slices.IsSorted(RuleKeys()) {
		t.Errorf("RuleKeys() = %q, not sorted", RuleKeys())
	}
}
