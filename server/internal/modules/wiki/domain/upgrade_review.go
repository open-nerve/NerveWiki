package domain

import (
	"fmt"
	"time"
)

// Review is what an upgrade in review holds as an admin acts on it (M11
// design 4.3, 4.8): its report, kept as its check passed; none, its
// submission still being checked; none, past the moment another
// submission or a send-back may take it over.
type Review int

// An upgrade's reviews.
const (
	ReviewReported Review = iota + 1
	ReviewChecking
	ReviewStale
)

// ReviewOf is up's review at at, timeout the bound of a submission's part
// apart from its request (SubmitTimeout): its report decides first; then
// at before submitted_at plus timeout is still checking, and at or after
// it is stale. up not reviewing, or reviewing without a submission, is a
// defect (00057's checks forbid the latter). The submission's first
// transaction, the approval and the send-back each branch on it (M11/P3
// design 3.2, decision 283); the SQL of each judges the same again under
// the notebook's lock.
func ReviewOf(up Upgrade, at time.Time, timeout time.Duration) (Review, error) {
	switch {
	case up.State != UpgradeReviewing:
		return 0, fmt.Errorf("wiki: the review of the upgrade %s, %s, a defect", up.ID, up.State)
	case up.Report != nil:
		return ReviewReported, nil
	case up.SubmittedAt == nil:
		return 0, fmt.Errorf("wiki: the upgrade %s reviewing has no submission, a defect", up.ID)
	case at.Before(up.SubmittedAt.Add(timeout)):
		return ReviewChecking, nil
	}
	return ReviewStale, nil
}
