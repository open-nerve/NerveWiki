package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The upgrade's rows of the matrix (M11/P1 design 3.22), in the seventh
// shard: each cell on a copy of lab's seeds of its own, which the row
// prepares by SQL before the request; TestThePermissionMatrixCoversEveryOperation
// reads them with the others. The start's notebook has its edit sessions
// cleared, the seeds' being alive; getUpgrade's notebook is being upgraded,
// upgrading or reviewing, by its creator, as the start would leave it; the
// rollback's rows' are rollbackMatrixRows'.

// upgradeMatrixRow is a row of the upgrade's and how its cells' copy is
// prepared; left, when set, reads what each cell, answered or refused,
// left there.
type upgradeMatrixRow struct {
	matrixRow
	prepare func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool)
	left    func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool)
}

func upgradeMatrixRows() []upgradeMatrixRow {
	notebook := func(c caller, s seeded) string {
		return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() + "/upgrade"
	}
	notFound := cell{http.StatusNotFound, "notebook.not_found"}
	upgradedBy := func(state string) func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool) {
		return func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool) {
			t.Helper()
			nb := s.notebook(notebookOf(c)).String()
			if count(t, pool, "SELECT count(*) FROM notebooks WHERE id = $1 AND deleted_at IS NULL", nb) == 0 {
				return // a deleted notebook has no upgrade to read
			}
			by := queryStrings(t, pool, "SELECT created_by_id::text FROM notebooks WHERE id = $1", nb)[0]
			if _, err := upgradeIn(context.Background(), pool, nb, by, state); err != nil {
				t.Fatal(err)
			}
		}
	}
	read := func(state string) func(t *testing.T, c caller, s seeded, answer string) {
		return func(t *testing.T, c caller, s seeded, answer string) {
			t.Helper()
			var u upgradeAnswer
			decodeAnswer(t, answer, &u)
			if u.NotebookID != s.notebook(notebookOf(c)).String() || u.State != state {
				t.Errorf("read %+v, want %s's upgrade, %s", u, notebookOf(c), state)
			}
		}
	}
	rows := []upgradeMatrixRow{{
		matrixRow: matrixRow{
			op: "startUpgrade", variant: "no upgrade", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) { return http.MethodPost, notebook(c, s), `{}` },
			cells:   adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var u upgradeAnswer
				decodeAnswer(t, answer, &u)
				if u.NotebookID != s.notebook(notebookOf(c)).String() || u.State != "upgrading" || len(u.SeededPageIDs) != 2 {
					t.Errorf("started %+v, want %s upgrading, raw and AGENTS written", u, notebookOf(c))
				}
			},
			after: func(t *testing.T, c caller, s seeded, get func(path string) (int, string)) {
				t.Helper()
				if status, answer := get(notebook(c, s)); status != http.StatusOK || !strings.Contains(answer, `"state":"upgrading"`) {
					t.Errorf("GET the upgrade started = %d %s", status, answer)
				}
			},
		},
		prepare: func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool) {
			t.Helper()
			if _, err := pool.Exec(context.Background(), "DELETE FROM edit_sessions WHERE notebook_id = $1", s.notebook(notebookOf(c))); err != nil {
				t.Fatal(err)
			}
		},
	}}
	for _, state := range []string{"upgrading", "reviewing"} {
		rows = append(rows, upgradeMatrixRow{
			matrixRow: matrixRow{
				op: "getUpgrade", variant: state, columns: notebookColumns(), write: true,
				request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, notebook(c, s), "" },
				cells:   adminsOnly(cellOK(), notFound),
				check:   read(state),
			},
			prepare: upgradedBy(state),
		})
	}
	rows = append(rows, checkMatrixRows(notebook, upgradedBy)...)
	return append(rows, rollbackMatrixRows(notebook, upgradedBy)...)
}

// checkMatrixRows are M11/P2/S2's (design 3.15): the check of an upgrade
// upgrading and of one reviewing, its submission, and a rule's items of
// one upgrading, its admins' alone. The seeds' notebooks hold no AGENTS:
// the check answers its gates, not passed, 200; the submission
// wiki.upgrade_not_ready (2.1 item 12).
func checkMatrixRows(notebook func(c caller, s seeded) string,
	upgradedBy func(state string) func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool),
) []upgradeMatrixRow {
	notFound := cell{http.StatusNotFound, "notebook.not_found"}
	var rows []upgradeMatrixRow
	for _, state := range []string{"upgrading", "reviewing"} {
		rows = append(rows, upgradeMatrixRow{
			matrixRow: matrixRow{
				op: "checkUpgrade", variant: state, columns: notebookColumns(), write: true,
				request: func(c caller, s seeded) (string, string, string) {
					return http.MethodGet, notebook(c, s) + "/check", ""
				},
				cells: adminsOnly(cellOK(), notFound),
				check: func(t *testing.T, c caller, s seeded, answer string) {
					t.Helper()
					var a upgradeCheckAnswer
					decodeAnswer(t, answer, &a)
					if len(a.Gates) != 8 || len(a.Rules) != 8 || a.Gates["agents"].Passed {
						t.Errorf("checked %s, want eight gates and eight rules, AGENTS failed", answer)
					}
				},
			},
			prepare: upgradedBy(state),
		})
	}
	rows = append(rows, upgradeMatrixRow{
		matrixRow: matrixRow{
			op: "submitUpgrade", variant: "upgrading", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebook(c, s) + "/submit", ""
			},
			cells: adminsOnly(cell{http.StatusConflict, "wiki.upgrade_not_ready"}, notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				t.Errorf("submitted %s, want a problem: the seeds pass no gate of AGENTS", answer)
			},
		},
		prepare: upgradedBy("upgrading"),
		// Refused or not ready, the upgrade is upgrading, no submission
		// left (M11/P2 design 3.15).
		left: func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool) {
			t.Helper()
			for _, row := range queryStrings(t, pool, `SELECT u.state || ' ' || n.upgrade_state || ' ' || (u.submission_id IS NULL)::text || ' ' ||
					(u.submitted_by_id IS NULL)::text || ' ' || (u.submitted_at IS NULL)::text || ' ' || (u.report IS NULL)::text
				FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id WHERE u.notebook_id = $1`, s.notebook(notebookOf(c))) {
				if row != "upgrading upgrading true true true true" {
					t.Errorf("the upgrade left %s, want it upgrading, no submission", row)
				}
			}
		},
	})
	return append(rows, upgradeMatrixRow{
		matrixRow: matrixRow{
			op: "listUpgradeCheckItems", variant: "upgrading", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, notebook(c, s) + "/check/wanted", ""
			},
			cells: adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var page struct {
					Mode string `json:"mode"`
				}
				decodeAnswer(t, answer, &page)
				if page.Mode != "llm_wiki" {
					t.Errorf("listed %s, want the LLM Wiki's", answer)
				}
			},
		},
		prepare: upgradedBy("upgrading"),
	})
}

// rollbackJobVariants are the cancel rows' variants whose jobs seeded
// names.
func rollbackJobVariants() []string { return []string{"own", "another's"} }

// rollbackMatrixRows are S3's (M11/P1 design 3.22), in the copies of lab
// as S2's are (S2's deviation 5): the abandonment of an upgrade
// upgrading; the end of a rollback whose job stopped; the cancel of the
// column's own rollback, ended, which its starter, no admin now, is
// forbidden; the cancel of another's, queued, the notebook rolling back.
// Each cancel's job is the column's seeded id, the row written by SQL.
func rollbackMatrixRows(notebook func(c caller, s seeded) string,
	upgradedBy func(state string) func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool),
) []upgradeMatrixRow {
	notFound, jobNotFound := cell{http.StatusNotFound, "notebook.not_found"}, cell{http.StatusNotFound, "transfer.not_found"}
	jobPath := func(id uuid.UUID) string { return "/api/v0/transfer-jobs/" + id.String() + "/cancel" }
	rollingBack := func(variant, state string, by func(c caller, s seeded) uuid.UUID) func(t *testing.T, c caller, s seeded,
		pool *pgxpool.Pool) {
		return func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool) {
			t.Helper()
			job := uuid.NewV7()
			if variant != "" {
				job = s.rollbackJob(variant, c)
			}
			if err := rollingBackIn(context.Background(), pool, s.notebook(notebookOf(c)).String(), by(c, s).String(), job.String(),
				state); err != nil {
				t.Fatal(err)
			}
		}
	}
	creator := func(c caller, s seeded) uuid.UUID { return s.accounts[someoneElse] }
	itself := func(c caller, s seeded) uuid.UUID { return s.accounts[c] }
	return []upgradeMatrixRow{{
		matrixRow: matrixRow{
			op: "abandonUpgrade", variant: "upgrading", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebook(c, s) + "/abandon", ""
			},
			cells: adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var a rollbackAnswer
				decodeAnswer(t, answer, &a)
				if a.State != "rolling_back" || a.Ending == nil || *a.Ending != "abandoned" || a.RollbackJobID == nil {
					t.Errorf("abandoned %s, want it rolling back, abandoned, with its job", answer)
				}
			},
		},
		prepare: upgradedBy("upgrading"),
	}, {
		matrixRow: matrixRow{
			op: "endUpgradeRollback", variant: "rolling_back, its job stopped", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebook(c, s) + "/end-rollback", ""
			},
			cells: adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var a rollbackAnswer
				decodeAnswer(t, answer, &a)
				if a.State != "rolled_back" || a.RollbackComplete == nil || !*a.RollbackComplete {
					t.Errorf("ended %s, want it rolled back, complete", answer)
				}
			},
		},
		prepare: rollingBack("", "failed", creator),
	}, {
		matrixRow: matrixRow{
			op: "cancelTransferJob", variant: "its own rollback, ended", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, jobPath(s.rollbackJob("own", c)), ""
			},
			cells: adminsOnly(cell{http.StatusConflict, "transfer.not_cancellable"}, jobNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				t.Errorf("cancelled %s, want a problem", answer)
			},
		},
		prepare: rollingBack("own", "failed", itself),
	}, {
		matrixRow: matrixRow{
			op: "cancelTransferJob", variant: "another's rollback, queued", columns: notebookColumns(), write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, jobPath(s.rollbackJob("another's", c)), ""
			},
			cells: adminsSee(cellOK(), jobNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var j transferJobAnswer
				decodeAnswer(t, answer, &j)
				if j.ID != s.rollbackJob("another's", c).String() || j.Kind != "rollback" || j.State != "cancelled" ||
					j.CreatedBy.UserID != s.accounts[someoneElse].String() {
					t.Errorf("cancelled %s, want the rollback of %s, cancelled", answer, someoneElse)
				}
			},
		},
		prepare: rollingBack("another's", "queued", creator),
	}}
}

// upgradeAnswer is an Upgrade answer, as much as the tests check.
type upgradeAnswer struct {
	ID            string   `json:"id"`
	NotebookID    string   `json:"notebook_id"`
	State         string   `json:"state"`
	SourceRootID  string   `json:"source_root_id"`
	SeededPageIDs []string `json:"seeded_page_ids"`
	StartedBy     struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	} `json:"started_by"`
	Baseline struct {
		Nodes  int `json:"nodes"`
		Pages  int `json:"pages"`
		Wanted int `json:"wanted"`
	} `json:"baseline"`
}

// The upgrade's rows (M11/P1 design 3.22): each cell on its own copy,
// prepared, its answer checked and read back; the copy then breaks none
// of the pages' rules, the upgrade's among them.
func TestUpgradePermissionMatrix(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	apps := make(chan struct{}, matrixApps)
	for _, r := range upgradeMatrixRows() {
		for _, c := range r.callers() {
			want, ok := r.cells[c]
			if !ok {
				continue // TestThePermissionMatrixCoversEveryOperation reports it
			}
			t.Run(r.name()+"/"+string(c), func(t *testing.T) {
				t.Parallel()
				apps <- struct{}{}
				t.Cleanup(func() { <-apps })
				url := pgtest.NewDatabaseFrom(t, d.url)
				pool := connect(t, url)
				r.prepare(t, c, d.seeded.in(t), pool)
				base := startApp(t, d.config(t, url, nil), migrations.FS())
				method, path, body := r.request(c, d.seeded.in(t))
				status, answer := askTyped(t, contract, method, base+path, d.tokens[c], "", body)
				got := cell{status: status}
				if status >= http.StatusBadRequest {
					got.code = problemCode(t, answer)
				}
				if got != want {
					t.Fatalf("%s %s = %d %s, want %s", method, path, status, strings.TrimSpace(answer), want)
				}
				if got.code == "" {
					r.check(t, c, d.seeded.in(t), answer)
					if r.after != nil {
						r.after(t, c, d.seeded.in(t), func(path string) (int, string) {
							return ask(t, contract, http.MethodGet, base+path, d.tokens[c], "")
						})
					}
				}
				if r.left != nil {
					r.left(t, c, d.seeded.in(t), pool)
				}
				for _, v := range pageViolations(t, pool) {
					if strings.Contains(v, "upgrade") {
						t.Error(v)
					}
				}
			})
		}
	}
}
