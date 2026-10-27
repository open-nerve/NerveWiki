package app

import (
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// checkedBatches accepts a complete partition of the compact plan, with no
// boundary inside an atomic segment. A callback is an internal dependency.
func checkedBatches(steps []domain.JobStep, batches JobBatches) ([][2]int, error) {
	if batches == nil {
		return nil, errors.New("page: missing background batch callback")
	}
	cuts := batches(steps)
	from := 0
	for _, cut := range cuts {
		if cut[0] != from || cut[1] <= from || cut[1] > len(steps) || cut[1] < len(steps) && steps[cut[1]-1].Joined {
			return nil, errors.New("page: invalid background batch cuts")
		}
		from = cut[1]
	}
	if from != len(steps) {
		return nil, errors.New("page: incomplete background batch cuts")
	}
	return cuts, nil
}
