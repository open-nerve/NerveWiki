package bootstrap

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

func TestLintMatrixTargets(t *testing.T) {
	contract := apitest.Load(t)
	seed := newSeeded().in(t)
	rows := matrixRows()
	t.Run("actual-rows", func(t *testing.T) {
		var found []string
		for _, row := range rows {
			if row.op != "getLint" && row.op != "listLintItems" {
				continue
			}
			found = append(found, row.op)
			if !slices.Equal(row.callers(), notebookColumns()) || row.wiki || row.write {
				t.Fatalf("lint matrix row %s lost the existing ordinary notebook columns/read semantics", row.name())
			}
		}
		if !slices.Equal(found, []string{"getLint", "listLintItems"}) {
			t.Fatalf("actual matrix lint rows=%v, want exactly getLint/listLintItems", found)
		}
		if violations := matrixViolations(contract.Operations(), matrixExempt(), rows, seed); len(violations) != 0 {
			t.Fatalf("actual lint matrix has violations: %v", violations)
		}
	})
	t.Run("foreign-notebook-column", func(t *testing.T) {
		for _, op := range []string{"getLint", "listLintItems"} {
			t.Run(op, func(t *testing.T) {
				changed := slices.Clone(rows)
				at := slices.IndexFunc(changed, func(row matrixRow) bool { return row.op == op })
				if at < 0 {
					t.Fatalf("actual matrix omitted %s", op)
				}
				original := changed[at].request
				changed[at].request = func(c caller, s seeded) (string, string, string) {
					method, path, body := original(c, s)
					// wlab is a real, separately seeded workspace. The guard's
					// contract is workspace ownership, not equality within lab.
					return method, strings.Replace(path, s.notebook(notebookOf(c)).String(), s.wiki.notebook(notebookOf(c)).String(), 1), body
				}
				violations := matrixViolations(contract.Operations(), matrixExempt(), changed, seed)
				if len(violations) != len(notebookColumns()) {
					t.Fatalf("foreign notebook column for %s gave %d violations, want all %d ownership violations: %v", op, len(violations), len(notebookColumns()), violations)
				}
				for _, violation := range violations {
					if !strings.Contains(violation, "row "+op+",") || !strings.Contains(violation, "{notebook_id}") ||
						!strings.Contains(violation, "is no row seeded under its column's workspace lab") {
						t.Fatalf("foreign notebook column produced an unrelated violation: %s", violation)
					}
				}
			})
		}
	})
	t.Run("only-rule-after-lint", func(t *testing.T) {
		var pattern string
		for _, op := range contract.Operations() {
			if op.ID == "listLintItems" {
				pattern = op.Path
			}
		}
		if pattern != "/api/v0/notebooks/{notebook_id}/lint/{rule}" {
			t.Fatalf("actual listLintItems path=%q", pattern)
		}
		path := "/api/v0/notebooks/" + seed.notebook(notebookOf(callerNotebookAdmin)).String() + "/lint/wanted"
		if violation := targetViolation(pattern, path, "lab", seed); violation != "" {
			t.Fatalf("legal lint rule segment changed notebook target checking: %s", violation)
		}
		pattern, path = strings.Replace(pattern, "/lint/", "/checks/", 1), strings.Replace(path, "/lint/", "/checks/", 1)
		if violation := targetViolation(pattern, path, "lab", seed); !strings.Contains(violation, "{rule} is no target the matrix knows") {
			t.Fatalf("a rule outside lint was incorrectly exempted: %q", violation)
		}
	})
}
