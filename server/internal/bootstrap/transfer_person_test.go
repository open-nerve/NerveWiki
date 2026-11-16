package bootstrap

import (
	"fmt"
	"net/http"
	"testing"
	"uuid"
)

// A person's jobs queued or running, over every notebook, are at most five
// (M12 design 4.6.6, T2; P3 design 3.13): alice's sixth export, in a
// notebook of no job of hers, is 409 transfer.too_many_jobs, with its
// words and no Retry-After, while bob's starts there; once she cancels
// one of hers, hers starts. Her five are seeded through SQL, each in a
// notebook of its own, with the River job that holds it an hour on, so
// that the start's lost leaves it queued (P3 design 3.14.3).
func TestAPersonsSixthJobIsRefusedAndAnothersStarts(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	kind, queue := riverJobOf("export")
	var hers []string
	for i := range 5 {
		nb, job := tm.openNotebook(t, "alice", fmt.Sprintf("Queued %d", i)), uuid.NewV7()
		if _, err := tm.pool.Exec(t.Context(), `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
			SELECT $2, $1, 'export', 'queued', 'Queued', id, 'web', now() FROM users WHERE email = 'alice@example.com'`, nb, job); err != nil {
			t.Fatal(err)
		}
		if _, err := tm.pool.Exec(t.Context(), riverHoldsSQL, kind, job.String(), queue); err != nil {
			t.Fatal(err)
		}
		hers = append(hers, job.String())
	}
	sixth := tm.openNotebook(t, "alice", "Sixth")

	req := newRequest(t, http.MethodPost, tm.base+"/api/v0/notebooks/"+sixth+"/exports", tm.tokens["alice"], []byte(`{}`))
	res, answer := sendRequest(t, req)
	tm.contract.CheckResponse(t, req, res)
	var p struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	decodeAnswer(t, string(answer), &p)
	if res.StatusCode != http.StatusConflict || p.Code != "transfer.too_many_jobs" ||
		p.Detail != "You have 5 jobs queued or running: wait for one to end, or cancel one." || res.Header.Get("Retry-After") != "" {
		t.Errorf("alice's sixth export = %d %s, Retry-After %q; want 409 transfer.too_many_jobs with its words, none",
			res.StatusCode, answer, res.Header.Get("Retry-After"))
	}

	if j := tm.startExport(t, "bob", sixth, ""); j.State != "queued" {
		t.Errorf("bob's export beside alice's five = %+v, want queued", j)
	}
	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/transfer-jobs/"+hers[0]+"/cancel", tm.tokens["alice"], ""); status != http.StatusOK {
		t.Fatalf("alice's cancel of one of hers = %d %s", status, answer)
	}
	if j := tm.startExport(t, "alice", sixth, ""); j.State != "queued" {
		t.Errorf("alice's export once she cancelled one = %+v, want queued", j)
	}
}
