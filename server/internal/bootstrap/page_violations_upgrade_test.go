package bootstrap

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// startUpgrade is by's start of the notebook nb's upgrade with body, as
// the API answers it.
func (tm acmeTeam) startUpgrade(t *testing.T, by, nb, body string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade", tm.tokens[by], body)
}

// upgraded is by's start of nb's upgrade, which must answer 200.
func (tm acmeTeam) upgraded(t *testing.T, by, nb string) upgradeAnswer {
	t.Helper()
	status, answer := tm.startUpgrade(t, by, nb, `{}`)
	if status != http.StatusOK {
		t.Fatalf("%s's start of %s = %d %s, want 200", by, nb, status, answer)
	}
	var u upgradeAnswer
	decodeAnswer(t, answer, &u)
	return u
}

// hasViolation reports whether violations hold rule's sentence.
func hasViolation(violations []string, rule string) bool {
	return slices.ContainsFunc(violations, func(v string) bool { return strings.HasSuffix(v, " "+rule+", want none") })
}

// pageViolations finds each gap of an upgrade under way and no other
// (M11/P1 design 3.15), as TestPageViolationsCatchEachLLMWikiGap does an
// LLM Wiki's: an upgrade started through the API breaks no rule; each row
// changes it through SQL, looks for its rule's sentence and puts it back;
// the LLM Wiki's row takes 00055's check out while it runs. Last, the
// notebook's deletion breaks none: the rules read live notebooks.
func TestPageViolationsCatchEachUpgradeGap(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	page := tm.createPage(t, "alice", nb, "", "Page")
	other := tm.openNotebook(t, "alice", "Other")
	otherPage := tm.createPage(t, "alice", other, "", "Elsewhere")
	u := tm.upgraded(t, "alice", nb)
	if len(u.SeededPageIDs) != 2 {
		t.Fatalf("the start wrote %v, want raw and AGENTS", u.SeededPageIDs)
	}
	agents := u.SeededPageIDs[1]
	if v := pageViolations(t, tm.pool); len(v) != 0 {
		t.Fatalf("an upgrade as started breaks %q, want nothing", v)
	}
	bob := tm.userOf(t, "bob")
	constraint := queryStrings(t, tm.pool, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'notebooks_upgrade_check'")[0]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	for _, tt := range []struct {
		name, rule   string
		change, undo func()
	}{
		{"its row another's", upgradeRowGap,
			func() { exec("UPDATE notebook_upgrades SET created_by_id = $2 WHERE id = $1", u.ID, bob) },
			func() {
				exec("UPDATE notebook_upgrades SET created_by_id = $2 WHERE id = $1", u.ID, u.StartedBy.UserID)
			}},
		{"its row deleted", upgradeRowGap,
			func() { exec("UPDATE notebook_upgrades SET deleted_at = now() WHERE id = $1", u.ID) },
			func() { exec("UPDATE notebook_upgrades SET deleted_at = NULL WHERE id = $1", u.ID) }},
		{"a row under way of a notebook not being upgraded", upgradeRowExtra,
			func() {
				exec(`INSERT INTO notebook_upgrades (id, notebook_id, state, source_root_id, seeded_root, locale, baseline_nodes,
					baseline_pages, baseline_wanted, baseline_wanted_capped, created_by_id, updated_by_id, created_at, updated_at)
					SELECT gen_random_uuid(), $1, 'upgrading', $2, false, 'en', 0, 0, 0, false, created_by_id, created_by_id, now(), now()
					FROM notebooks WHERE id = $1`, other, otherPage)
			},
			func() { exec("DELETE FROM notebook_upgrades WHERE notebook_id = $1", other) }},
		{"a baseline row more than counted", upgradeBaselineGap,
			func() { exec("UPDATE notebook_upgrades SET baseline_nodes = baseline_nodes + 1 WHERE id = $1", u.ID) },
			func() { exec("UPDATE notebook_upgrades SET baseline_nodes = baseline_nodes - 1 WHERE id = $1", u.ID) }},
		{"a protected row of no upgrade", upgradeRowsOfNone,
			func() {
				exec("INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind) VALUES (gen_random_uuid(), $2, $1, false, 'page')",
					other, otherPage)
			},
			func() { exec("DELETE FROM upgrade_nodes WHERE notebook_id = $1", other) }},
		{"the source root another notebook's", upgradeRootGap,
			func() { exec("UPDATE notebook_upgrades SET source_root_id = $2 WHERE id = $1", u.ID, otherPage) },
			func() { exec("UPDATE notebook_upgrades SET source_root_id = $2 WHERE id = $1", u.ID, u.SourceRootID) }},
		{"the source root deleted", upgradeRootGap,
			// Page stands for the source root: the raw the start wrote keeps
			// its guardrail's row, which a deletion by SQL would leave.
			func() {
				exec("UPDATE notebook_upgrades SET source_root_id = $2 WHERE id = $1", u.ID, page)
				exec("UPDATE nodes SET deleted_at = now() WHERE id = $1", page)
			},
			func() {
				exec("UPDATE nodes SET deleted_at = NULL WHERE id = $1", page)
				exec("UPDATE notebook_upgrades SET source_root_id = $2 WHERE id = $1", u.ID, u.SourceRootID)
			}},
		{"AGENTS another notebook's", upgradeAgentsGap,
			func() { exec("UPDATE notebook_upgrades SET seeded_agents_id = $2 WHERE id = $1", u.ID, otherPage) },
			func() { exec("UPDATE notebook_upgrades SET seeded_agents_id = $2 WHERE id = $1", u.ID, agents) }},
		{"an LLM Wiki being upgraded", upgradeOfLLMWiki,
			func() {
				exec("ALTER TABLE notebooks DROP CONSTRAINT notebooks_upgrade_check")
				exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", nb, u.SourceRootID)
			},
			func() {
				exec("UPDATE notebooks SET mode = 'standard', source_root_id = NULL WHERE id = $1", nb)
				exec("ALTER TABLE notebooks ADD CONSTRAINT notebooks_upgrade_check " + constraint)
			}},
	} {
		tt.change()
		if got := pageViolations(t, tm.pool); !hasViolation(got, tt.rule) {
			t.Errorf("%s: the violations %q hold no %q", tt.name, got, tt.rule)
		}
		tt.undo()
		if v := pageViolations(t, tm.pool); len(v) != 0 {
			t.Fatalf("%s put back breaks %q, want nothing", tt.name, v)
		}
	}
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/notebooks/"+nb, tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE the notebook being upgraded = %d %s, want 204", status, answer)
	}
	if v := pageViolations(t, tm.pool); len(v) != 0 {
		t.Errorf("the notebook being upgraded, deleted, breaks %q, want nothing", v)
	}
}
