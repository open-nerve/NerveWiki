package bootstrap

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func (tm acmeTeam) originExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := tm.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// originLegacy is a literal historical fixture: version 1 lost only the
// empty property path. It is never a means of upgrading stored facts.
func (tm acmeTeam) originLegacy(t *testing.T, ids ...string) {
	t.Helper()
	tm.originExec(t, `UPDATE indexed_pages SET extractor=1 WHERE node_id=ANY($1::uuid[])`, ids)
	tm.originExec(t, `UPDATE page_links SET property_key=NULL WHERE source_id=ANY($1::uuid[]) AND property_key=''`, ids)
	for _, id := range ids {
		tm.originVersion(t, id, 1)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM page_links WHERE source_id=ANY($1::uuid[]) AND property_key=''`, ids); n != 0 {
		t.Fatalf("legacy fixture retained %d empty paths", n)
	}
}

func (tm acmeTeam) originVersion(t *testing.T, id string, version int) {
	t.Helper()
	var got int
	if err := tm.pool.QueryRow(t.Context(), `SELECT extractor FROM indexed_pages WHERE node_id=$1`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != version {
		t.Errorf("page %s extractor = %d, want literal %d", id, got, version)
	}
}

// originIndex includes NULLs as JSON null, not as an empty-string alias.
func (tm acmeTeam) originIndex(t *testing.T, nb string) []string {
	t.Helper()
	out := []string{}
	for _, table := range []string{"indexed_pages", "page_links", "page_properties", "page_tags", "page_aliases"} {
		out = append(out, queryStrings(t, tm.pool, `SELECT to_jsonb(r)::text FROM `+table+` r WHERE notebook_id=$1 ORDER BY to_jsonb(r)::text`, nb)...)
	}
	return out
}

func TestTheLintOriginCompatibility(t *testing.T) {
	t.Run("rules-after-S1-2", testLintOriginRules)
	t.Run("literal-v1", func(t *testing.T) {
		t.Run("views-and-backlinks", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			target := tm.createPage(t, "alice", nb, "", "Target")
			other := tm.createPage(t, "alice", nb, "", "Other")
			legacy := tm.createPageWith(t, "alice", nb, "", "Legacy", "---\n\"\": '[[Target]]'\n---\nold [[Target]]\n")
			current := tm.createPageWith(t, "alice", nb, "", "Current", "current [[Target]]\n")
			tm.originVersion(t, current, 2)
			tm.originLegacy(t, legacy)
			before := tm.originIndex(t, nb)
			var backlinks backlinkPage
			tm.get(t, "bob", "/api/v0/pages/"+target+"/backlinks", &backlinks)
			if len(backlinks.Data) != 2 || backlinks.Data[0].ID != legacy || backlinks.Data[0].Count != 2 || len(backlinks.Data[0].Contexts) != 0 ||
				backlinks.Data[1].ID != current || backlinks.Data[1].Count != 1 || !slices.Equal(backlinks.Data[1].Contexts, []string{"current [[Target]]"}) {
				t.Fatalf("literal-v1 backlinks retain counts but not contexts: %+v", backlinks)
			}
			var properties struct {
				Links []originPropertyLink `json:"links"`
			}
			tm.get(t, "bob", "/api/v0/pages/"+legacy+"/properties", &properties)
			if len(properties.Links) != 0 {
				t.Errorf("v1 empty path is still absent: %+v", properties)
			}
			var view struct {
				HTML string `json:"html"`
			}
			tm.get(t, "bob", "/api/v0/pages/"+legacy+"/view", &view)
			if strings.Count(view.HTML, `data-nw-node="`+target+`"`) != 2 {
				t.Errorf("old view did not resolve both links: %s", view.HTML)
			}
			if got := tm.originIndex(t, nb); !reflect.DeepEqual(got, before) {
				t.Errorf("read repaired old facts: %v -> %v", before, got)
			}

			// A deliberately misdirected resolution is a separate boundary
			// witness: current Views reads it, literal-v1 Views reparses instead.
			tm.originExec(t, `UPDATE page_links SET resolved_id=$1 WHERE source_id=$2`, other, current)
			tm.get(t, "bob", "/api/v0/pages/"+current+"/view", &view)
			if !strings.Contains(view.HTML, `data-nw-node="`+other+`"`) {
				t.Fatal("current view bypassed the index witness")
			}
			tm.originLegacy(t, current)
			sentinel := tm.originIndex(t, nb)
			tm.get(t, "bob", "/api/v0/pages/"+current+"/view", &view)
			if !strings.Contains(view.HTML, `data-nw-node="`+target+`"`) || strings.Contains(view.HTML, `data-nw-node="`+other+`"`) {
				t.Errorf("literal-v1 view used stale resolution: %s", view.HTML)
			}
			if got := tm.originIndex(t, nb); !reflect.DeepEqual(got, sentinel) {
				t.Error("fallback rewrote the index")
			}
		})
		t.Run("rewrite", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			target := tm.createPage(t, "alice", nb, "", "A")
			text := "---\n\"\": '[[A]]'\n---\n[[A]]\n"
			legacy := tm.createPageWith(t, "alice", nb, "", "Legacy", text)
			current := tm.createPageWith(t, "alice", nb, "", "Current", text)
			tm.originLegacy(t, legacy)
			tm.send(t, nodeRename("alice", target, "B"), http.StatusOK)
			tm.wrote(t, legacy, text, 1)
			tm.originVersion(t, legacy, 1)
			tm.wrote(t, current, strings.ReplaceAll(text, "[[A]]", "[[B]]"), 2)
			tm.originVersion(t, current, 2)
		})
		t.Run("only-real-content-writes-upgrade", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			text := "---\n\"\": '[[Missing]]'\n---\nbody\n"
			legacy := tm.createPageWith(t, "alice", nb, "", "Legacy", text)
			untouched := tm.createPageWith(t, "alice", nb, "", "Untouched", text)
			restored := tm.createPageWith(t, "alice", nb, "", "Restored", text)
			tm.send(t, contentWrite("alice", restored, text+"second\n", 1, ""), http.StatusOK)
			tm.originLegacy(t, legacy, untouched, restored)
			tm.send(t, nodeRename("alice", legacy, "Tree only"), http.StatusOK)
			tm.wrote(t, legacy, text, 1)
			tm.originVersion(t, legacy, 1)
			tm.send(t, contentWrite("alice", legacy, text, 1, ""), http.StatusOK)
			tm.wrote(t, legacy, text, 1)
			tm.originVersion(t, legacy, 1)
			tm.send(t, contentWrite("alice", legacy, text+"changed\n", 1, ""), http.StatusOK)
			tm.wrote(t, legacy, text+"changed\n", 2)
			tm.originVersion(t, legacy, 2)
			tm.send(t, versionRestore("alice", restored, 1, 2), http.StatusOK)
			tm.wrote(t, restored, text, 3)
			tm.originVersion(t, restored, 2)
			tm.wrote(t, untouched, text, 1)
			tm.originVersion(t, untouched, 1)
			if n := count(t, tm.pool, `SELECT count(*) FROM page_links WHERE source_id=ANY($1::uuid[]) AND property_key=''`, []string{legacy, restored}); n != 2 {
				t.Errorf("changed/restored empty origins = %d, want 2", n)
			}
		})
		for _, path := range []string{"Reached", "Indexed"} {
			t.Run("batch/"+path, func(t *testing.T) {
				for _, format := range []string{"literal-v1", "revision-mismatch", "current"} {
					t.Run(format, func(t *testing.T) {
						tm := newAcmeTeam(t, "member", "")
						nb := tm.openNotebook(t, "alice", "Eng")
						a := tm.createPage(t, "alice", nb, "", "A")
						p := tm.createPageWith(t, "alice", nb, "", "P", "[[A]] p")
						untouched := tm.createPageWith(t, "alice", nb, "", "Untouched", "[[A]] u")
						agent := tm.agent(t, "alice", nb)
						if format == "literal-v1" {
							tm.originLegacy(t, p, untouched)
						}
						if format == "revision-mismatch" {
							tm.originExec(t, `UPDATE indexed_pages SET revision=2 WHERE node_id=ANY($1::uuid[])`, []string{p, untouched})
						}
						if path == "Reached" {
							answer := agent.batch(t, opMove("A.md", "B.md"), opWrite("P.md", "[[A]] p"))
							if format == "current" {
								if r := refusalOf(t, answer); r.Code != "page.title_taken" || refusedAt(r) != 1 {
									t.Fatalf("current Reached must compare after rewriting: %+v", r)
								}
								b := agent.batchDone(t, opMove("A.md", "B.md"), opWrite("P.md", "[[B]] p"))
								if len(b.Results) != 2 || !b.Results[0].Changed || b.Results[1].Changed || b.Results[0].NodeID != a || b.Results[1].NodeID != p {
									t.Errorf("current move then no-op: %+v", b)
								}
								tm.wrote(t, p, "[[B]] p", 2)
								tm.originVersion(t, p, 2)
							} else {
								b := answerOf[mcpBatched](t, answer)
								if len(b.Results) != 2 || !b.Results[0].Changed || b.Results[1].Changed || b.Results[0].NodeID != a || b.Results[1].NodeID != p {
									t.Errorf("stale Reached must preserve a true no-op: %+v", b)
								}
								tm.wrote(t, p, "[[A]] p", 1)
								if format == "literal-v1" {
									tm.originVersion(t, p, 1)
								}
							}
						} else {
							b := agent.batchDone(t, opMove("A.md", "B.md"), opEdit("P.md", " p", " q"))
							if len(b.Results) != 2 || !b.Results[0].Changed || !b.Results[1].Changed || b.Results[0].NodeID != a || b.Results[1].NodeID != p {
								t.Errorf("move then edit: %+v", b)
							}
							want := "[[A]] q"
							if format == "current" {
								want = "[[B]] q"
							}
							tm.wrote(t, p, want, 2)
							tm.originVersion(t, p, 2)
						}
						if format == "literal-v1" {
							tm.wrote(t, untouched, "[[A]] u", 1)
							tm.originVersion(t, untouched, 1)
						}
					})
				}
			})
		}
	})
}
