package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"

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
// upgrading or reviewing, by its creator, as the start would leave it.

// upgradeMatrixRow is a row of the upgrade's and how its cells' copy is
// prepared.
type upgradeMatrixRow struct {
	matrixRow
	prepare func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool)
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
	return rows
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
				for _, v := range pageViolations(t, pool) {
					if strings.Contains(v, "upgrade") {
						t.Error(v)
					}
				}
			})
		}
	}
}
