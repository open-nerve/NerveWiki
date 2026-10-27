package app

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SourceCheckedJobFacts is a test-only observation of the actual history
// reads and checked compact job. Tests separately call PlanRevert and
// RunBatch; this hook neither supplies their plan nor writes any state.
func (w *JobWrites) SourceCheckedJobFacts(ctx context.Context, spec JobSpec, id, root uuid.UUID, batches JobBatches) ([]domain.StepFacts, error) {
	unit := revertUnit(spec, id)
	if err := w.writer.Allowed(ctx, unit); err != nil {
		return nil, err
	}
	c, err := w.history.FindChangeset(ctx, id)
	if err != nil {
		return nil, err
	}
	checked, err := w.checks().check(ctx, c, unit, revertBounds{Items: w.maxItems, Locks: true, BackgroundBatches: batches})
	if err != nil {
		return nil, err
	}
	if checked.job == nil {
		return nil, errors.New("test observation requires an LLM Wiki background plan")
	}
	return slices.Collect(jobRevertFacts(checked, *checked.job, checked.cuts, root)), nil
}
