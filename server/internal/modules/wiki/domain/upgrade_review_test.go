package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// An upgrade's review (M11/P3 design 3.2, decision 283): its report first,
// then its submission's moment plus the timeout. The moment and the
// timeout are values of their own, 12:00:00.000001 and 17 s, so that
// neither reads as the other; the boundary is SubmitUpgrade's SQL's
// (submitted_at <= taken_over takes it over): a microsecond before is
// still checking, at it and after it stale. A report with the moment
// still ahead is reported. An upgrade not reviewing, or reviewing
// without a submission, is a defect.
func TestReviewOf(t *testing.T) {
	submitted := time.Date(2026, 11, 12, 12, 0, 0, 1000, time.UTC)
	const timeout = 17 * time.Second
	moment := submitted.Add(timeout)
	reviewing := func(report bool, at *time.Time) domain.Upgrade {
		up := domain.Upgrade{ID: checkID(1), State: domain.UpgradeReviewing, SubmittedAt: at}
		if report {
			up.Report = &domain.UpgradeReport{Passed: true}
		}
		return up
	}
	for _, tc := range []struct {
		name string
		up   domain.Upgrade
		at   time.Time
		want domain.Review
	}{
		{"a report, the moment still ahead", reviewing(true, &submitted), submitted.Add(time.Second), domain.ReviewReported},
		{"a report, the moment past", reviewing(true, &submitted), moment.Add(time.Hour), domain.ReviewReported},
		{"none, as submitted", reviewing(false, &submitted), submitted, domain.ReviewChecking},
		{"none, a microsecond before the moment", reviewing(false, &submitted), moment.Add(-time.Microsecond), domain.ReviewChecking},
		{"none, at the moment", reviewing(false, &submitted), moment, domain.ReviewStale},
		{"none, a microsecond after the moment", reviewing(false, &submitted), moment.Add(time.Microsecond), domain.ReviewStale},
	} {
		if got, err := domain.ReviewOf(tc.up, tc.at, timeout); got != tc.want || err != nil {
			t.Errorf("%s: ReviewOf = %d, %v; want %d", tc.name, got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		up   domain.Upgrade
	}{
		{"upgrading", domain.Upgrade{ID: checkID(1), State: domain.UpgradeUpgrading}},
		{"rolling back, its submission and report kept", domain.Upgrade{ID: checkID(1), State: domain.UpgradeRollingBack,
			SubmittedAt: &submitted, Report: &domain.UpgradeReport{}}},
		{"reviewing without a submission", reviewing(false, nil)},
	} {
		if got, err := domain.ReviewOf(tc.up, submitted, timeout); got != 0 || err == nil {
			t.Errorf("%s: ReviewOf = %d, %v; want a defect's error", tc.name, got, err)
		}
	}
}

// The two refusals that name the gates not passed (M11/P2 design 3.12,
// M11/P3 design 3.2, decision 282): wiki.upgrade_not_ready, 409, each its
// own sentence after the same keys, the submission's upgrading again, the
// approval's still in review; neither changes ErrUpgradeNotReady.
func TestTheNotReadyRefusalsNameTheirGates(t *testing.T) {
	failed := []domain.UpgradeGateKey{domain.GateAgents, domain.GateReservedTitles}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"the submission's", domain.UpgradeNotReady(failed),
			"These gates did not pass: agents, reserved_titles. The upgrade is upgrading again: check it, then submit it again."},
		{"the approval's", domain.ApprovalNotReady(failed),
			"These gates did not pass: agents, reserved_titles. The upgrade is still in review: send it back, or reject it."},
		{"the approval's of one gate", domain.ApprovalNotReady([]domain.UpgradeGateKey{domain.GateSourceRoot}),
			"These gates did not pass: source_root. The upgrade is still in review: send it back, or reject it."},
	} {
		var e *shared.Error
		if !errors.As(tc.err, &e) || !errors.Is(tc.err, domain.ErrUpgradeNotReady) || e.ProblemStatus() != 409 ||
			e.ProblemCode() != "wiki.upgrade_not_ready" || e.Detail != tc.want {
			t.Errorf("%s = %#v, want wiki.upgrade_not_ready, 409, %q", tc.name, tc.err, tc.want)
		}
	}
	if domain.ErrUpgradeNotReady.Detail != "The upgrade does not pass its checks: it is upgrading again. Check it, then submit it again." {
		t.Errorf("ErrUpgradeNotReady's detail changed: %q", domain.ErrUpgradeNotReady.Detail)
	}
}
