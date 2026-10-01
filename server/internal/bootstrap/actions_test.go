package bootstrap

import (
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// The access module's rule table has a rule for each module's action and
// for nothing else (M2/P1 design 3.4): an action without a rule is refused
// at run time, and a rule without an action is a decision no use case asks
// for. access cannot import the modules; bootstrap sees both.
func TestTheRuleTableIsTheModulesActions(t *testing.T) {
	actions := slices.Concat(workspace.Actions())
	slices.Sort(actions)
	if rules := access.RuleKeys(); len(actions) == 0 || !slices.Equal(actions, rules) {
		t.Errorf("the modules' actions = %q, want the rule table's %q", actions, rules)
	}
}
