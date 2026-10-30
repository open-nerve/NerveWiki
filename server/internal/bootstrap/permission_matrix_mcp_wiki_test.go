package bootstrap

import (
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Both rows run in the existing standard-notebook matrix. The LLM variant
// below uses its real, separately prepared notebooks and the same 13 callers.
func wikiMCPRows() []mcpRow {
	return []mcpRow{
		{tool: "lint", entries: mcpserver.BothEntries, want: wikiMCPReader,
			check: func(t *testing.T, _ caller, _ seeded, a mcpAnswer) {
				got := answerOf[map[string]any](t, a)
				lintServeEnvelope(t, mcpLintWithoutPaths(got), "standard")
				lintServeSummary(t, got, "standard")
			}},
		{tool: "pending_sources", entries: mcpserver.BothEntries,
			want: func(c caller) mcpCell {
				if denied := wikiMCPReader(c); denied != (mcpCell{}) {
					return denied
				}
				return mcpCell{code: "notebook.not_llm_wiki"}
			}},
	}
}

// This permission oracle is explicit: a workspace admin with no notebook
// membership cannot read the private notebook. It does not reuse the authorizer.
func wikiMCPReader(c caller) mcpCell {
	switch c {
	case callerNotebookAdmin, callerNotebookEditor, callerNotebookReader,
		callerDefaultEditor, callerDefaultReader, callerGuestReaderOfOpen, callerOwnerlessMember:
		return mcpCell{}
	default:
		return mcpCell{code: "notebook.not_found"}
	}
}

func TestTheWikiToolsReadLLMWikiMatrix(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	base := startApp(t, d.config(t, d.url, productionDeadlines), migrations.FS())
	pats := map[caller]string{}
	t.Run("tokens", func(t *testing.T) {
		for _, c := range notebookColumns() {
			pats[c] = createToken(t, contract, base, d.tokens[c])
		}
	})
	if len(pats) != 13 {
		t.Fatal("LLM matrix needs all 13 real PATs; select prepare and tokens")
	}
	checked := 0
	for _, entry := range []string{"notebook", "account"} {
		for _, tool := range []string{"lint", "pending_sources"} {
			for _, c := range notebookColumns() {
				t.Run(tool+"/"+entry+"/"+string(c), func(t *testing.T) {
					s := d.seeded.in(t)
					name := notebookOf(c)
					m := &mcpApp{base: base, pat: pats[c], notebook: s.wiki.notebook(name).String()}
					a := sourceMCPCall(t, m, entry, tool, map[string]any{})
					want := wikiMCPReader(c)
					if entry == "notebook" && want.code != "" {
						want.http = http.StatusNotFound
					}
					if got := mcpCellOf(t, a); got != want {
						t.Fatalf("LLM %s/%s/%s=%s, want %s; %.500s", tool, entry, c, got, want, a.body)
					}
					checked++
					if want != (mcpCell{}) {
						return
					}
					got := answerOf[map[string]any](t, a)
					lintServeEnvelope(t, mcpLintWithoutPaths(got), "llm_wiki")
					items := got["data"]
					if tool == "lint" {
						lintServeSummary(t, got, "llm_wiki")
						pending := lintServeRule(t, got, "undigested_sources")
						if pending["count"] != float64(2) || pending["count_capped"] != false || pending["next_cursor"] != nil {
							t.Fatalf("LLM matrix pending summary=%v, want the two actual source nodes", pending)
						}
						items = pending["items"]
					} else if got["next_cursor"] != nil {
						t.Fatalf("two-source page unexpectedly continues: %v", got)
					}
					nodes := lintServeObjects(t, items)
					if len(nodes) != 2 {
						t.Fatalf("LLM matrix pending=%v, want actual source page and attachment", nodes)
					}
					expected := map[string]struct {
						kind, path string
						bytes      float64
					}{
						s.wiki.source(name, "page").String():  {"page", "raw/Source.md", float64(len("source original\n"))},
						s.wiki.source(name, "asset").String(): {"asset", "raw/source.txt", 3},
					}
					for _, item := range nodes {
						id := lintServeString(t, item, "node_id")
						exp, ok := expected[id]
						if !ok || item["rule"] != "undigested_sources" || item["item_key"] != "n:"+id || item["kind"] != exp.kind || item["path"] != exp.path || item["bytes"] != exp.bytes {
							t.Fatalf("LLM matrix source=%v, want independent stored source %s=%+v", item, id, exp)
						}
						if stamp, err := time.Parse(time.RFC3339Nano, lintServeString(t, item, "created_at")); err != nil || stamp.IsZero() || stamp.Location() != time.UTC {
							t.Fatalf("source creation time=%v/%v", item, err)
						}
						lintServeString(t, item, "cursor")
						delete(expected, id)
					}
					if len(expected) != 0 {
						t.Fatalf("missing actual source nodes: %v", expected)
					}
				})
			}
		}
	}
	if checked != 52 {
		t.Fatalf("checked %d real LLM matrix answers, want 13 columns × two entries × two tools", checked)
	}
}
