//go:build !race

package postgresadapter_test

import (
	"fmt"
	"strings"
	"testing"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A no-source result must avoid page metadata and unrelated source rows even
// when another notebook owns a dense partial index before or after these IDs.
func TestLintPlansSourcePresence(t *testing.T) {
	if testing.Short() {
		t.Skip("source presence physical plans")
	}
	lintHiddenFixtures(t, "wanted", func(t *testing.T, f fixture, compare func(*testing.T, string, lintPlanEvidence)) {
		for _, stats := range []string{"no-statistics", "analyzed"} {
			if stats == "analyzed" {
				lintAnalyze(t, f)
			}
			t.Run(stats, func(t *testing.T) {
				for _, positive := range []bool{false, true} {
					ids := []uuid.UUID{lintReaderID(1), lintReaderID(2)}
					if positive {
						ids = append(ids, lintReaderID(3))
					}
					has, err := postgresadapter.NewLintReads(f.pool, 5<<20).HasSourceProperties(t.Context(), f.eng, ids)
					if err != nil || has != positive {
						t.Fatalf("actual presence=%t want=%t err=%v", has, positive, err)
					}
					for _, generic := range []bool{false, true} {
						t.Run(fmt.Sprintf("positive=%t/generic=%t", positive, generic), func(t *testing.T) {
							s := statementOf(t, func(q *gen.Queries) error {
								_, err := q.LintHasSourceProperties(t.Context(), gen.LintHasSourcePropertiesParams{NotebookID: f.eng, Extractor: domain.Extractor, PageIds: ids})
								return err
							})
							plan, ev := lintExecuted(t, f, s, generic)
							matched := int64(0)
							if positive {
								matched = 1
							}
							work := ev.ExecutionWork["page_properties_sources_idx"]
							if ev.Rows != 1 || work.Scans != int64(len(ids)) || work.Returned != matched || work.Fetched > matched {
								t.Fatalf("presence lost bounded property probes: scalar rows=%d work=%+v", ev.Rows, work)
							}
							for name, work := range ev.ExecutionWork {
								if strings.HasPrefix(name, "indexed_pages") && (work.Scans > matched || work.Returned > matched || work.Fetched > matched) {
									t.Fatalf("presence read page metadata without a property: %s %+v", name, work)
								}
								if strings.HasPrefix(name, "page_properties") && (work.Returned > matched || work.Fetched > matched) {
									t.Fatalf("presence visited unrelated properties: %s %+v", name, work)
								}
							}
							if len(plan.unlimited("page_properties")) != 0 {
								t.Fatal("source existence lacks a bounded per-page property probe")
							}
							compare(t, fmt.Sprintf("%s/%t/%t", stats, positive, generic), ev)
						})
					}
				}
			})
		}
	})
}
