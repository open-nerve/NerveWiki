package app_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lineKeys are line's keys, sorted.
func lineKeys(line map[string]any) []string {
	return slices.Sorted(maps.Keys(line))
}

// An upgrade's approval (M11/P3 design 3.6), with fakes of its ports. Each
// refusal in its turn, after its last call, rolled back, nothing written,
// nothing logged: the lock's and the decision's; no upgrade in review;
// reviewing without a report, its submission still checked, then past
// its moment, wiki.upgrade_state (not "still being checked"); a report
// that did not pass, a defect. Each of gates 1 to 3 failing alone:
// wiki.upgrade_not_ready naming it, still in review, after the rollback
// one error line of the notebook, the upgrade and that gate (decision
// 293), no other line; the rollback failing then answers its failure, no
// line. Approved, in order: the gates read, the row named, the notebook an
// LLM Wiki, the hashes rebuilt, the baseline deleted, the row approved,
// the mode's change from standard to llm_wiki told, each by alice at the
// clock; after the COMMIT one info line; the answer the row approved. The
// row not approved is a defect; a COMMIT that fails answers its failure,
// no line. Its own deadline, in the rebuild: server_busy, one warn line;
// the request's deadline first: neither.
func TestApproveUpgrade(t *testing.T) {
	for _, tt := range append(reviewRefusals(), []struct {
		name   string
		change func(x *reviewFixture)
		code   string
		last   string
	}{
		{"reviewing, its submission still checked", func(x *reviewFixture) {
			x.f.row.Report, *x.f.row.SubmittedAt = nil, x.f.at.Add(-16*time.Second)
		}, "wiki.upgrade_state", "current"},
		{"reviewing, past its submission's moment", func(x *reviewFixture) { x.f.row.Report = nil }, "wiki.upgrade_state", "current"},
		{"a report that did not pass", func(x *reviewFixture) { x.f.row.Report = &domain.UpgradeReport{} }, "a defect", "current"},
		{"a row of another upgrade", func(x *reviewFixture) { x.f.row.ID = uuid.NewV7() }, "a defect", "current"},
	}...) {
		x := newReviewFixture(tt.change)
		_, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb)
		code := codeOf(err)
		var e *shared.Error
		if !errors.As(err, &e) {
			code = "a defect"
		}
		calls := x.f.noted()
		if code != tt.code || !refusedAfter(calls, tt.last) || wrote(calls) != "" || len(x.f.lines) != 0 {
			t.Errorf("%s: %v after %v, logged %v; want %s after %s, rolled back, nothing written or logged", tt.name, err, calls, x.f.lines,
				tt.code, tt.last)
		}
		if tt.code == "wiki.upgrade_state" && strings.Contains(err.Error(), "still being checked") {
			t.Errorf("%s: %v, want no submission still checked: an approval does not wait for it", tt.name, err)
		}
	}

	for _, tc := range []struct {
		name   string
		change func(x *reviewFixture)
		gate   string
	}{
		{"AGENTS without a body", func(x *reviewFixture) { x.f.roots.AgentsBody = false }, "agents"},
		{"the source root no live page", func(x *reviewFixture) { x.f.roots.RootPage = false }, "source_root"},
		// The approval judges the upgrade's own source root (review P3S1A-N1).
		{"the source root the root's AGENTS", func(x *reviewFixture) { x.f.roots.AgentsID = &x.root }, "source_root"},
		{"a page keyed index at the root", func(x *reviewFixture) {
			x.f.reserved = []app.RootNode{{ID: uuid.NewV7(), Kind: domain.NodePage, Key: "index"}}
		}, "reserved_titles"},
	} {
		x := newReviewFixture(tc.change)
		_, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb)
		var e *shared.Error
		want := "These gates did not pass: " + tc.gate + ". The upgrade is still in review: send it back, or reject it."
		if !errors.As(err, &e) || e.Code != "wiki.upgrade_not_ready" || e.ProblemStatus() != 409 || e.Detail != want {
			t.Errorf("%s: %v, want wiki.upgrade_not_ready, %q", tc.name, err, want)
		}
		calls := x.f.noted()
		if !slices.Equal(calls[len(calls)-3:], []string{"root nodes index log index.md log.md", "rollback",
			"log ERROR upgrade gates failed at approval"}) || wrote(calls) != "" {
			t.Errorf("%s: calls %v, want the gates read, rolled back, then the error line, nothing written", tc.name, calls)
		}
		wantLine := map[string]any{"level": "ERROR", "msg": "upgrade gates failed at approval", "notebook_id": x.nb.String(),
			"upgrade_id": x.up.String(), "failed_gates": []string{tc.gate}}
		if len(x.f.lines) != 1 || !reflect.DeepEqual(x.f.lines[0], wantLine) {
			t.Errorf("%s: logged %v, want %v alone", tc.name, x.f.lines, wantLine)
		}
	}
	// The rollback after the gates failed fails: its failure is the answer.
	lost := errors.New("the connection is gone")
	x := newReviewFixture(func(x *reviewFixture) { x.f.roots.AgentsBody, x.f.rollbackErr = false, lost })
	if _, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb); !errors.Is(err, lost) || errors.Is(err, domain.ErrUpgradeNotReady) ||
		len(x.f.lines) != 0 {
		t.Errorf("the gates failed, the rollback too: %v, logged %v; want the rollback's failure, no line", err, x.f.lines)
	}

	x = newReviewFixture(nil)
	view, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb)
	if err != nil {
		t.Fatal(err)
	}
	at := x.at()
	if want := []string{"workspace", "begin", "share", "lock", "authorize", "clock", "current", "roots of " + x.root.String(),
		"root nodes index log index.md log.md", "names",
		fmt.Sprintf("approve notebook %s of %s, its root %s, by %s at %s", x.nb, x.up, x.root, x.alice, at),
		fmt.Sprintf("rebuild %s under %s", x.nb, x.root), "delete baseline of " + x.up.String(),
		fmt.Sprintf("approve row %s of %s by %s at %s", x.up, x.sub, x.alice, at), "publish", "commit",
		"log INFO upgrade approved"}; !slices.Equal(x.f.noted(), want) {
		t.Errorf("calls %v\nwant %v", x.f.noted(), want)
	}
	if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeLLMWiki, By: x.alice,
		At: x.f.at}}; !reflect.DeepEqual(x.f.published, want) {
		t.Errorf("told %+v, want %+v", x.f.published, want)
	}
	approved := x.f.row
	approved.State = domain.UpgradeApproved
	if want := (app.UpgradeView{Upgrade: approved, StartedByName: "name of " + x.starter.String(),
		SubmittedByName: "name of " + x.submitter.String()}); !reflect.DeepEqual(view, want) {
		t.Errorf("answered %+v, want %+v", view, want)
	}
	if len(x.f.lines) != 1 || !slices.Equal(lineKeys(x.f.lines[0]), []string{"elapsed_ms", "level", "msg", "notebook_id",
		"sources_hashed", "submission_id", "upgrade_id", "user_id", "workspace_id"}) {
		t.Fatalf("logged %v, want one info line", x.f.lines)
	}
	if l := x.f.lines[0]; l["level"] != "INFO" || l["msg"] != "upgrade approved" || l["workspace_id"] != x.f.ws.String() ||
		l["notebook_id"] != x.nb.String() || l["upgrade_id"] != x.up.String() || l["submission_id"] != x.sub.String() ||
		l["user_id"] != x.alice.String() || l["sources_hashed"] != int64(7) {
		t.Errorf("the info line %v", l)
	}

	x = newReviewFixture(func(x *reviewFixture) { x.f.rowsRefuse = true })
	if _, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb); err == nil || codeOf(err) == "wiki.upgrade_state" ||
		!slices.Contains(x.f.noted(), "rollback") || len(x.f.lines) != 0 {
		t.Errorf("the row not approved: %v after %v; want a defect, rolled back, no line", err, x.f.noted())
	}

	full := errors.New("the disk is full")
	x = newReviewFixture(func(x *reviewFixture) { x.f.commitErr = full })
	if _, err := app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb); !errors.Is(err, full) || len(x.f.lines) != 0 ||
		x.f.noted()[len(x.f.noted())-1] != "commit" {
		t.Errorf("a COMMIT that failed: %v after %v, logged %v; want its failure, no line after the commit", err, x.f.noted(), x.f.lines)
	}

	x = newReviewFixture(func(x *reviewFixture) {
		x.f.rebuild = func(ctx context.Context) error {
			<-ctx.Done()
			return fmt.Errorf("rebuild the source hashes: %w", ctx.Err())
		}
	})
	x.d.ApproveTimeout = 50 * time.Millisecond
	_, err = app.NewApproveUpgrade(x.d).Execute(x.ctx(), x.nb)
	var busy *shared.Error
	if !errors.As(err, &busy) || busy.Code != "server_busy" || busy.RetryDelay != time.Second {
		t.Errorf("past its own deadline: %v, want server_busy, Retry-After 1", err)
	}
	if len(x.f.lines) != 1 || !slices.Equal(lineKeys(x.f.lines[0]), []string{"elapsed_ms", "level", "msg", "notebook_id", "timeout_ms",
		"user_id", "workspace_id"}) {
		t.Fatalf("past its own deadline logged %v, want one warn line", x.f.lines)
	}
	if l := x.f.lines[0]; l["level"] != "WARN" || l["msg"] != "upgrade approval timed out" || l["workspace_id"] != x.f.ws.String() ||
		l["notebook_id"] != x.nb.String() || l["user_id"] != x.alice.String() || l["timeout_ms"] != int64(50) ||
		l["elapsed_ms"].(int64) < 50 {
		t.Errorf("the warn line %v", l)
	}

	// The request's own deadline first, the approval's an hour: its
	// error as it is, no 503, no line.
	x = newReviewFixture(func(x *reviewFixture) {
		x.f.rebuild = func(ctx context.Context) error {
			<-ctx.Done()
			return fmt.Errorf("rebuild the source hashes: %w", ctx.Err())
		}
	})
	ctx, cancel := context.WithTimeout(x.ctx(), 50*time.Millisecond)
	defer cancel()
	if _, err := app.NewApproveUpgrade(x.d).Execute(ctx, x.nb); !errors.Is(err, context.DeadlineExceeded) || codeOf(err) == "server_busy" ||
		len(x.f.lines) != 0 {
		t.Errorf("the request's deadline first: %v, logged %v; want its error, no line", err, x.f.lines)
	}
}

// An upgrade's rejection (M11/P3 design 3.7), with fakes of its ports.
// Each refusal in its turn, nothing written or logged, upgrading among
// them (a rejection is a review's); a full queue's server_busy, rolled
// back. Reviewing, with its report or without one still checked, in
// order: the notebook's state rolling back, the mode's change told, the
// rollback's job started from the caller's client, the row rolling back,
// rejected, with the job; the row read as written; after the COMMIT one
// info line, its submission kept. A COMMIT that fails answers its failure,
// no line.
func TestRejectUpgrade(t *testing.T) {
	for _, tt := range reviewRefusals() {
		x := newReviewFixture(tt.change)
		_, err := app.NewRejectUpgrade(x.d).Execute(x.ctx(), x.nb)
		calls := x.f.noted()
		if codeOf(err) != tt.code || !refusedAfter(calls, tt.last) || wrote(calls) != "" || len(x.f.lines) != 0 {
			t.Errorf("%s: %v after %v; want %s after %s, rolled back, nothing written or logged", tt.name, err, calls, tt.code, tt.last)
		}
	}
	x := newReviewFixture(func(x *reviewFixture) { x.f.startErr = shared.ServerBusy(time.Second) })
	if _, err := app.NewRejectUpgrade(x.d).Execute(x.ctx(), x.nb); codeOf(err) != "server_busy" ||
		x.f.noted()[len(x.f.noted())-1] != "rollback" || len(x.f.lines) != 0 {
		t.Errorf("the queue full: %v after %v; want server_busy, rolled back", err, x.f.noted())
	}
	for _, reported := range []bool{true, false} {
		x := newReviewFixture(func(x *reviewFixture) {
			if !reported {
				x.f.row.Report, *x.f.row.SubmittedAt = nil, x.f.at.Truncate(time.Microsecond)
			}
		})
		view, err := app.NewRejectUpgrade(x.d).Execute(x.ctx(), x.nb)
		if err != nil {
			t.Fatalf("reported %v: %v", reported, err)
		}
		at := x.at()
		if want := []string{"workspace", "begin", "share", "lock", "authorize", "clock",
			fmt.Sprintf("set state rolling_back of %s by %s at %s", x.up, x.alice, at), "publish",
			fmt.Sprintf("start rollback of %s from web", x.up),
			fmt.Sprintf("roll back %s rejected with %s by %s at %s", x.up, x.f.job, x.alice, at), "current", "names", "commit",
			"log INFO upgrade rejected"}; !slices.Equal(x.f.noted(), want) {
			t.Errorf("reported %v: calls %v\nwant %v", reported, x.f.noted(), want)
		}
		if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard,
			By: x.alice, At: x.f.at}}; !reflect.DeepEqual(x.f.published, want) {
			t.Errorf("reported %v: told %+v, want %+v", reported, x.f.published, want)
		}
		up := view.Upgrade
		if up.State != domain.UpgradeRollingBack || up.Ending != domain.UpgradeRejected || up.RollbackJobID == nil ||
			*up.RollbackJobID != x.f.job || up.SubmissionID == nil || *up.SubmissionID != x.sub || (up.Report != nil) != reported ||
			view.SubmittedByName != "name of "+x.submitter.String() {
			t.Errorf("reported %v: answered %+v", reported, view)
		}
		wantLine := map[string]any{"level": "INFO", "msg": "upgrade rejected", "workspace_id": x.f.ws.String(), "notebook_id": x.nb.String(),
			"upgrade_id": x.up.String(), "submission_id": x.sub.String(), "user_id": x.alice.String(), "job_id": x.f.job.String()}
		if len(x.f.lines) != 1 || !reflect.DeepEqual(x.f.lines[0], wantLine) {
			t.Errorf("reported %v: logged %v, want %v", reported, x.f.lines, wantLine)
		}
	}
	full := errors.New("the disk is full")
	x = newReviewFixture(func(x *reviewFixture) { x.f.commitErr = full })
	if _, err := app.NewRejectUpgrade(x.d).Execute(x.ctx(), x.nb); !errors.Is(err, full) || len(x.f.lines) != 0 ||
		x.f.noted()[len(x.f.noted())-1] != "commit" {
		t.Errorf("a COMMIT that failed: %v after %v, logged %v; want its failure, no line", err, x.f.noted(), x.f.lines)
	}
}

// An upgrade's send-back (M11/P3 design 3.8), with fakes of its ports.
// Each refusal in its turn, nothing written or logged; without a report,
// its submission still checked: wiki.upgrade_state, "still being
// checked". With its report, and past its submission's moment without
// one, in order: the notebook upgrading again, the row upgrading, its
// submission and report cleared, the mode's change from standard to
// standard told, at the clock truncated to the microsecond, taken over
// at that moment less SubmitTimeout; the row read as written, upgrading,
// no submission; after the COMMIT one info line of the submission sent
// back and whether it had its report. The row not sent back is a defect;
// a COMMIT that fails answers its failure, no line.
func TestSendBackUpgrade(t *testing.T) {
	for _, tt := range append(reviewRefusals(), []struct {
		name   string
		change func(x *reviewFixture)
		code   string
		last   string
	}{
		{"reviewing, its submission still checked", func(x *reviewFixture) {
			x.f.row.Report, *x.f.row.SubmittedAt = nil, x.f.at.Add(-16*time.Second).Truncate(time.Microsecond)
		}, "wiki.upgrade_state", "current"},
		// As the approval's (review P3S1A-N2).
		{"a row of another upgrade", func(x *reviewFixture) { x.f.row.ID = uuid.NewV7() }, "a defect", "current"},
	}...) {
		x := newReviewFixture(tt.change)
		_, err := app.NewSendBackUpgrade(x.d).Execute(x.ctx(), x.nb)
		code := codeOf(err)
		var e *shared.Error
		if !errors.As(err, &e) {
			code = "a defect"
		}
		calls := x.f.noted()
		if code != tt.code || !refusedAfter(calls, tt.last) || wrote(calls) != "" || len(x.f.lines) != 0 {
			t.Errorf("%s: %v after %v; want %s after %s, rolled back, nothing written or logged", tt.name, err, calls, tt.code, tt.last)
		}
		checked := tt.code == "wiki.upgrade_state" && tt.last == "current"
		if checking := strings.Contains(fmt.Sprint(err), "still being checked"); checking != checked {
			t.Errorf("%s: %v, want still being checked %v", tt.name, err, checked)
		}
	}
	for _, reported := range []bool{true, false} {
		x := newReviewFixture(func(x *reviewFixture) {
			if !reported {
				// Past its moment: submitted 17 s before the clock, truncated.
				x.f.row.Report, *x.f.row.SubmittedAt = nil, x.f.at.Truncate(time.Microsecond).Add(-17*time.Second)
			}
		})
		view, err := app.NewSendBackUpgrade(x.d).Execute(x.ctx(), x.nb)
		if err != nil {
			t.Fatalf("reported %v: %v", reported, err)
		}
		at := "2026-11-12T10:20:30.123456Z"
		if want := []string{"workspace", "begin", "share", "lock", "authorize", "clock", "current",
			fmt.Sprintf("set state upgrading of %s by %s at %s", x.up, x.alice, at),
			fmt.Sprintf("send back %s by %s at %s, taken over 2026-11-12T10:20:13.123456Z", x.up, x.alice, at), "publish", "current",
			"names", "commit", "log INFO upgrade sent back"}; !slices.Equal(x.f.noted(), want) {
			t.Errorf("reported %v: calls %v\nwant %v", reported, x.f.noted(), want)
		}
		if want := []app.ModeChange{{WorkspaceID: x.f.ws, NotebookID: x.nb, From: domain.ModeStandard, To: domain.ModeStandard,
			By: x.alice, At: x.f.at.Truncate(time.Microsecond)}}; !reflect.DeepEqual(x.f.published, want) {
			t.Errorf("reported %v: told %+v, want %+v", reported, x.f.published, want)
		}
		if want := (app.UpgradeView{Upgrade: domain.Upgrade{ID: x.up, NotebookID: x.nb, State: domain.UpgradeUpgrading,
			SourceRootID: x.root, CreatedBy: x.starter}, StartedByName: "name of " + x.starter.String()}); !reflect.DeepEqual(view, want) {
			t.Errorf("reported %v: answered %+v, want %+v", reported, view, want)
		}
		wantLine := map[string]any{"level": "INFO", "msg": "upgrade sent back", "workspace_id": x.f.ws.String(), "notebook_id": x.nb.String(),
			"upgrade_id": x.up.String(), "submission_id": x.sub.String(), "user_id": x.alice.String(), "reported": reported}
		if len(x.f.lines) != 1 || !reflect.DeepEqual(x.f.lines[0], wantLine) {
			t.Errorf("reported %v: logged %v, want %v", reported, x.f.lines, wantLine)
		}
	}
	x := newReviewFixture(func(x *reviewFixture) { x.f.rowsRefuse = true })
	if _, err := app.NewSendBackUpgrade(x.d).Execute(x.ctx(), x.nb); err == nil || codeOf(err) == "wiki.upgrade_state" ||
		!slices.Contains(x.f.noted(), "rollback") || len(x.f.lines) != 0 {
		t.Errorf("the row not sent back: %v after %v; want a defect, rolled back, no line", err, x.f.noted())
	}
	full := errors.New("the disk is full")
	x = newReviewFixture(func(x *reviewFixture) { x.f.commitErr = full })
	if _, err := app.NewSendBackUpgrade(x.d).Execute(x.ctx(), x.nb); !errors.Is(err, full) || len(x.f.lines) != 0 ||
		x.f.noted()[len(x.f.noted())-1] != "commit" {
		t.Errorf("a COMMIT that failed: %v after %v, logged %v; want its failure, no line", err, x.f.noted(), x.f.lines)
	}
}
