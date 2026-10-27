package app

import (
	"iter"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// checkRevertContract judges the whole schedule before any of its steps.
// The caller owns the memo and the timing of any extra authorization:
// unlocked for a precheck, or the unit's Grants under its lock.
func checkRevertContract(state domain.RevertState, plan domain.RevertPlan, notebook domain.NotebookState,
	ask func(shared.Action) error,
) error {
	return checkRevertFacts(domain.RevertFacts(state, plan, notebook.SourceRootID), notebook, ask)
}

// checkRevertFacts shares only the pure rule and authorization handling;
// synchronous and background callers provide their own actual fact order.
func checkRevertFacts(steps iter.Seq[domain.StepFacts], notebook domain.NotebookState, ask func(shared.Action) error) error {
	if notebook.Mode != domain.ModeLLMWiki {
		return nil
	}
	for facts := range steps {
		verdict := domain.CheckContract(notebook, facts)
		if verdict.Err != nil {
			return domain.Refuse(verdict.Err, verdict.NodeID)
		}
		if verdict.Needs != "" {
			err := ask(verdict.Needs)
			if isForbidden(err) {
				return domain.Refuse(domain.ErrSourceAdminOnly, verdict.NodeID)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
