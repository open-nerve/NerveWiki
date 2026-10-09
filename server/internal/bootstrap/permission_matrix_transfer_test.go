package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"uuid"
)

// The transfer module's rows (M7/P5 design 3.14), by the notebook columns:
// each aims at its notebook (notebookOf) and at two exports seeded in it,
// the column's own, succeeded, and someone else's, queued. Any role starts
// an export, lists its jobs, and reads its own and asks to cancel it (an
// export that ended: transfer.not_cancellable); another's job is its
// notebook's admins' alone and is not found by the rest; the columns
// without a role do not see the notebook. The download is public: its
// signature decides (matrixExempt).

// matrixJob is a seeded export: owner's, of notebook, in state.
type matrixJob struct {
	notebook string
	owner    caller
	state    string
}

// matrixJobs are the seeded exports: each notebook column's own in its
// notebook, succeeded, its archive not written; and in each notebook of
// the columns someone else's, queued, never enqueued. gone-nb's are
// deleted with it.
func matrixJobs() []matrixJob {
	var out []matrixJob
	for _, c := range notebookColumns() {
		out = append(out, matrixJob{notebookOf(c), c, "succeeded"})
	}
	for _, nb := range sessionNotebooks() {
		out = append(out, matrixJob{nb, someoneElse, "queued"})
	}
	return out
}

// transferJobAnswer is a TransferJob answer, as much as the rows check.
type transferJobAnswer struct {
	ID         string  `json:"id"`
	NotebookID string  `json:"notebook_id"`
	RootID     *string `json:"root_id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	State      string  `json:"state"`
	CreatedBy  struct {
		UserID string `json:"user_id"`
	} `json:"created_by"`
	Download *struct {
		URL string `json:"url"`
	} `json:"download"`
}

// adminsSee answers a notebook's admins, and does not show the rest
// another's job.
func adminsSee(answer, notFound cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range notebookColumns() {
		cells[c] = notFound
		if role, _ := roleIn(c); role == "admin" {
			cells[c] = answer
		}
	}
	return cells
}

func transferMatrixRows() []matrixRow {
	notebookPath := func(c caller, s seeded) string {
		return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String()
	}
	jobPath := func(id uuid.UUID) string { return "/api/v0/transfer-jobs/" + id.String() }
	notFound := cell{http.StatusNotFound, "transfer.not_found"}
	// checkJob checks that answer is the job of notebook owner's, in state.
	checkJob := func(t *testing.T, c caller, s seeded, answer string, owner caller, state string) transferJobAnswer {
		t.Helper()
		var j transferJobAnswer
		decodeAnswer(t, answer, &j)
		if want := s.job(notebookOf(c), owner); j.ID != want.String() || j.State != state || j.Kind != "export" ||
			j.CreatedBy.UserID != s.accounts[owner].String() {
			t.Errorf("job %+v, want %s's export %s, %s", j, owner, want, state)
		}
		return j
	}
	return []matrixRow{
		{
			op:      "startExport",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebookPath(c, s) + "/exports", `{"root_id": "` + s.page(pageOf(c)).String() + `"}`
			},
			cells: seenOrNot(cell{status: http.StatusAccepted}, cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var j transferJobAnswer
				decodeAnswer(t, answer, &j)
				if root := s.page(pageOf(c)).String(); j.Kind != "export" || j.State != "queued" || j.RootID == nil || *j.RootID != root ||
					j.Name != pageOf(c) || j.CreatedBy.UserID != s.accounts[c].String() {
					t.Errorf("started %+v, want %s's export of %s, queued", j, c, pageOf(c))
				}
			},
		},
		{
			op:      "listTransferJobs",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, notebookPath(c, s) + "/transfer-jobs", ""
			},
			cells: seenOrNot(cellOK(), cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var list struct{ Data []transferJobAnswer }
				decodeAnswer(t, answer, &list)
				// A notebook's admin lists its every job, the rest their own.
				var want []string
				role, _ := roleIn(c)
				for _, j := range matrixJobs() {
					if j.notebook == notebookOf(c) && (j.owner == c || role == "admin") {
						want = append(want, s.job(j.notebook, j.owner).String())
					}
				}
				got := make([]string, len(list.Data))
				for i, j := range list.Data {
					got[i] = j.ID
				}
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) {
					t.Errorf("listed %q, want %q", got, want)
				}
			},
		},
		{
			op:      "getTransferJob",
			variant: "its own",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, jobPath(s.job(notebookOf(c), c)), ""
			},
			cells: seenOrNot(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				j := checkJob(t, c, s, answer, c, "succeeded")
				if j.Download == nil || !strings.HasPrefix(j.Download.URL, jobPath(s.job(notebookOf(c), c))+"/download?e=") {
					t.Errorf("download %+v, want the archive's signed address", j.Download)
				}
			},
		},
		{
			op:      "getTransferJob",
			variant: "another's",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, jobPath(s.job(notebookOf(c), someoneElse)), ""
			},
			cells: adminsSee(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				if j := checkJob(t, c, s, answer, someoneElse, "queued"); j.Download != nil {
					t.Errorf("download %+v of a queued job, want none", j.Download)
				}
			},
		},
		{
			op:      "cancelTransferJob",
			variant: "its own, ended",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, jobPath(s.job(notebookOf(c), c)) + "/cancel", ""
			},
			cells: seenOrNot(cell{http.StatusConflict, "transfer.not_cancellable"}, notFound),
		},
		{
			op:      "cancelTransferJob",
			variant: "another's, queued",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, jobPath(s.job(notebookOf(c), someoneElse)) + "/cancel", ""
			},
			cells: adminsSee(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				checkJob(t, c, s, answer, someoneElse, "cancelled")
			},
		},
	}
}
