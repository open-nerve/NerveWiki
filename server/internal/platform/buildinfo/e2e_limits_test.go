package buildinfo

import (
	"strings"
	"testing"
)

// The e2e job's limits (00-M12 3 item 11, the side branch m12-e2e-browsers-install): E2E is judged by a limit of its
// own, the one item 11 names, so that E2E itself growing slower fails there and then, whatever the steps before it
// took; installing the browsers has a limit of its own, as an apt mirror can take nearly 15 minutes and more over their
// system packages; and the job's limit comes after both and the other steps, so that it cuts neither short. A
// continue-on-error on the step or the job would let a run over the limit pass green.
func TestTheE2EStepHasALimitOfItsOwn(t *testing.T) {
	// item 11's limit: the job's former 20 minutes less the steps before E2E as they usually go (31 s and 54 s, the
	// medians of 2026-11-07 to 18), about twice the slowest green E2E step of 2026-11-06 to 18 (8:52)
	const e2eLimit = 1
	// the other steps at most: checkout to pnpm install took 50 s at most on 2026-11-07 to 18; the cache of the
	// browsers' system packages and the upload of the report besides
	const others = 5
	job := jobsOf(t, "ci.yml", readWorkflow(t, "ci.yml"))["e2e"]
	if job == nil {
		t.Fatal("ci.yml: no e2e job")
	}
	var e2e, install []map[string]any
	for i, step := range stepsOf(job) {
		run, _ := step["run"].(string)
		if run == "make e2e" {
			e2e = append(e2e, step)
		}
		if strings.Contains(run, "playwright install") {
			install = append(install, step)
		}
		if step["continue-on-error"] != nil {
			t.Errorf("ci.yml: the e2e job's step %d has continue-on-error", i+1)
		}
	}
	if field(job, "continue-on-error") != nil {
		t.Errorf("ci.yml: the e2e job has continue-on-error")
	}
	if len(e2e) != 1 || len(install) != 1 {
		t.Fatalf("ci.yml: the e2e job's steps that run make e2e are %v and that install the browsers %v, want one each",
			e2e, install)
	}
	if limit := e2e[0]["timeout-minutes"]; limit != e2eLimit {
		t.Errorf("ci.yml: the E2E step's timeout-minutes is %v, want %d, the limit 00-M12 3 item 11 names", limit, e2eLimit)
	}
	installLimit, ok := install[0]["timeout-minutes"].(int)
	if !ok || installLimit <= 0 {
		t.Errorf("ci.yml: the step that installs the browsers has timeout-minutes %v, want a number of its own",
			install[0]["timeout-minutes"])
	}
	if limit, ok := field(job, "timeout-minutes").(int); !ok || limit < installLimit+e2eLimit+others {
		t.Errorf("ci.yml: the e2e job's timeout-minutes is %v, want at least %d: the browsers' %d, E2E's %d and %d for "+
			"the other steps, so that each of the two steps reaches its own limit first",
			field(job, "timeout-minutes"), installLimit+e2eLimit+others, installLimit, e2eLimit, others)
	}
}
