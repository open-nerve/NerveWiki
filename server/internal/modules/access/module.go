// Package access decides who may do what (v0.1 design 13.1, item 3): the
// rule table and the decision, behind shared.Authorizer. It has no tables:
// the facts come through its ports, which the modules that own them
// implement and bootstrap wires.
package access

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/access/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/access/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// WorkspaceMemberships reads the facts of the workspace level.
type WorkspaceMemberships = app.WorkspaceMemberships

// NotebookFacts reads the facts of the notebook level, and NotebookFact is
// what it reads.
type (
	NotebookFacts = app.NotebookFacts
	NotebookFact  = app.NotebookFact
)

// Deps are the facts the decisions read.
type Deps struct {
	Memberships WorkspaceMemberships
	Notebooks   NotebookFacts
}

// New returns the Authorizer every module's use cases call.
func New(d Deps) shared.Authorizer {
	return app.NewAuthorizer(d.Memberships, d.Notebooks)
}

// RuleKeys returns the actions the rule table has a rule for: bootstrap
// checks they are the modules' actions.
func RuleKeys() []shared.Action {
	return domain.RuleKeys()
}
