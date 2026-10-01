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
		switch r.Level {
		case LevelWorkspace:
			checkRoles(t, action, r.Workspace, shared.WorkspaceRoles(), len(r.Notebook))
		case LevelNotebook:
			checkRoles(t, action, r.Notebook, shared.NotebookRoles(), len(r.Workspace))
		default:
			t.Errorf("%s: level %d", action, r.Level)
		}
	}
}

// checkRoles checks a rule's roles of its level: some, each a role of the
// level; and none of the other level's (others).
func checkRoles[R comparable](t *testing.T, action shared.Action, roles, known []R, others int) {
	t.Helper()
	if len(roles) == 0 {
		t.Errorf("%s allows no role", action)
	}
	for _, role := range roles {
		if !slices.Contains(known, role) {
			t.Errorf("%s allows %v, which is no role of its level", action, role)
		}
	}
	if others != 0 {
		t.Errorf("%s lists roles of the other level", action)
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
