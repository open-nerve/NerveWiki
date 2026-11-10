package bootstrap

import (
	"maps"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The upgrade's tools' rows of the matrix over MCP (M11/P2 design 3.15),
// in the eighth shard: each cell on a copy of lab's seeds of its own,
// which the row prepares by SQL before the call, as REST's upgrade rows
// do; TestThePermissionMatrixCoversEveryTool reads them with the others
// (coveredMCPRows).

// upgradeMCPRow is a tool's row of the upgrade's and how its cells' copy
// is prepared.
type upgradeMCPRow struct {
	mcpRow
	prepare func(t *testing.T, c caller, s seeded, pool *pgxpool.Pool)
}

// upgradeMCPRows are upgrade_verify of an upgrade upgrading, which its
// admins' agents check, its gates naming their paths; and upgrade_submit,
// which lab's notebooks do not pass (no AGENTS): wiki.upgrade_not_ready.
// Another column that sees the notebook is forbidden, one that does not
// notebook.not_found, as REST's adminsOnly.
func upgradeMCPRows() []upgradeMCPRow {
	admins := func(ok mcpCell) func(c caller) mcpCell {
		cells := adminsOnly(cellOK(), cell{http.StatusNotFound, "notebook.not_found"})
		return func(c caller) mcpCell {
			switch cells[c].status {
			case http.StatusOK:
				return ok
			case http.StatusForbidden:
				return mcpCell{code: "forbidden"}
			}
			return mcpCell{code: "notebook.not_found"}
		}
	}
	return []upgradeMCPRow{{
		mcpRow: mcpRow{tool: "upgrade_verify", entries: mcpserver.BothEntries, want: admins(mcpCell{}),
			check: func(t *testing.T, _ caller, _ seeded, a mcpAnswer) {
				t.Helper()
				got := answerOf[struct {
					Passed bool                      `json:"passed"`
					Gates  map[string]map[string]any `json:"gates"`
					Rules  []map[string]any          `json:"rules"`
				}](t, a)
				if len(got.Gates) != 8 || len(got.Rules) != 8 || got.Passed || got.Gates["agents"]["passed"] != false {
					t.Errorf("checked %s, want eight gates and rules, AGENTS failed", a.body)
				}
				for _, gate := range []string{"reserved_titles", "links", "preserved"} {
					if _, ok := got.Gates[gate]["paths"].([]any); !ok {
						t.Errorf("the gate %s = %v, want its paths", gate, got.Gates[gate])
					}
				}
			}},
		prepare: upgradedBy("upgrading"),
	}, {
		mcpRow: mcpRow{tool: "upgrade_submit", entries: mcpserver.BothEntries, write: true,
			want: admins(mcpCell{code: "wiki.upgrade_not_ready"})},
		prepare: upgradedBy("upgrading"),
	}}
}

// Each cell of the upgrade's tools, in parallel, as
// TestThePermissionMatrixOverMCP runs the others', on a copy of its own
// prepared by its row: in the notebook entry a column REST's listNodes
// does not let in is the edge's 404, in the account entry the tool's
// notebook.not_found.
func TestUpgradeToolsMatrix(t *testing.T) {
	takes := takesNotebook(readToolLists(t))
	d := prepareMatrix(t)
	pats := map[caller]string{}
	created := t.Run("tokens", func(t *testing.T) {
		contract := apitest.Load(t)
		base := startApp(t, d.config(t, d.url, nil), migrations.FS())
		for _, c := range notebookColumns() {
			pats[c] = createToken(t, contract, base, d.tokens[c])
		}
	})
	if !created || len(pats) != len(notebookColumns()) {
		t.Fatal("the tokens were not created")
	}
	gate := gateCells(t)
	apps := make(chan struct{}, matrixApps)
	for _, r := range upgradeMCPRows() {
		for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
			for _, c := range notebookColumns() {
				t.Run(r.tool+"/"+entry.String()+"/"+string(c), func(t *testing.T) {
					t.Parallel()
					apps <- struct{}{}
					t.Cleanup(func() { <-apps })
					want := r.want(c)
					if entry == mcpserver.NotebookEntry && gate[c].status == http.StatusNotFound {
						want = mcpCell{http: http.StatusNotFound, code: "notebook.not_found"}
					}
					url := pgtest.NewDatabaseFrom(t, d.url)
					s := d.seeded.in(t)
					r.prepare(t, c, s, connect(t, url))
					m := &mcpApp{base: startApp(t, d.config(t, url, nil), migrations.FS()), pat: pats[c]}
					notebook := s.notebook(notebookOf(c)).String()
					args := map[string]any{}
					if r.args != nil {
						maps.Copy(args, r.args(c, s))
					}
					path := atNotebook(notebook)
					if entry == mcpserver.AccountEntry {
						path = mcpserver.Pattern
						if takes[r.tool] {
							args["notebook"] = notebook
						}
					}
					a := m.call(t, path, r.tool, args)
					if got := mcpCellOf(t, a); got != want {
						t.Fatalf("%s at %s = %s (%d %.300s), want %s", r.tool, path, got, a.status, a.body, want)
					}
					if r.check != nil && want == (mcpCell{}) {
						r.check(t, c, s, a)
					}
				})
			}
		}
	}
}
