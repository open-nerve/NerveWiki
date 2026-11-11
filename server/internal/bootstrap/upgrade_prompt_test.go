package bootstrap

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The prompts by the upgrade's states (M11/P3 design 3.18, 00-M11
// decision 222), in the seventh shard: the matrix of the four prompts in
// four states, and the upgrade's prompt through the notebook entry.

// upgradePromptStates are the states the matrix reads each prompt in: an
// LLM Wiki; a standard notebook with no upgrade; one upgrading; one in
// review.
func upgradePromptStates() []string {
	return []string{"llm_wiki", "standard", "upgrading", "reviewing"}
}

// upgradePromptAnswers are what a prompt answers each reader in each
// state: "" its text, else its refusal's code. The LLM Wiki's three
// answer an LLM Wiki alone; the upgrade's a notebook upgrading alone (M11
// design 4.13).
func upgradePromptAnswers() map[string]map[string]string {
	const notLLMWiki, state = "notebook.not_llm_wiki", "wiki.upgrade_state"
	llmWiki := func() map[string]string {
		return map[string]string{"llm_wiki": "", "standard": notLLMWiki, "upgrading": notLLMWiki, "reviewing": notLLMWiki}
	}
	return map[string]map[string]string{
		"ingest": llmWiki(), "lint": llmWiki(), "query": llmWiki(),
		"upgrade": {"llm_wiki": state, "standard": state, "upgrading": "", "reviewing": state},
	}
}

// promptAnswerOf is a prompt's get as a cell: its HTTP refusal; the code
// of its JSON-RPC error, -32602 whose message starts with that code; or
// none for an answer whose one message is the user's, text whole, and
// whose description is description.
func promptAnswerOf(t *testing.T, a mcpAnswer, text, description string) mcpCell {
	t.Helper()
	if a.status != http.StatusOK {
		return mcpCellOf(t, a)
	}
	if _, ok := a.decoded(t)["error"]; ok {
		code, message, data := promptRefusal(t, a)
		if code != -32602 || data == "" || !strings.HasPrefix(message, data+": ") {
			t.Fatalf("a refusal %v %q of %q, want -32602 of a code", code, message, data)
		}
		return mcpCell{code: data}
	}
	r := a.result(t)
	messages, _ := r["messages"].([]any)
	var message, content map[string]any
	if len(messages) == 1 {
		message, _ = messages[0].(map[string]any)
		content, _ = message["content"].(map[string]any)
	}
	if message["role"] != "user" || content["type"] != "text" || content["text"] != text || r["description"] != description {
		t.Fatalf("the answer %.300s, want one user message of the text %.60q…, the description %q", a.body, text, description)
	}
	return mcpCell{}
}

// promptDescriptions are the notebook entry's prompts' descriptions in
// api/mcp/prompts.json, by name.
func promptDescriptions(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range readPromptLists(t)["notebook"] {
		object, _ := p.(map[string]any)
		name, _ := object["name"].(string)
		out[name], _ = object["description"].(string)
	}
	return out
}

// Each prompt of the contract's file in each of the upgrade's states, by
// the 13 notebook columns (M11/P3 design 3.18, M11 design 4.14): the LLM
// Wiki's three answer an LLM Wiki alone, notebook.not_llm_wiki for a
// standard notebook upgrading, in review or not; the upgrade's answers a
// notebook upgrading alone, wiki.upgrade_state otherwise; one the caller
// cannot see stops at the HTTP edge. An answer's text is compared whole.
// The LLM Wiki and the standard notebook are the prepared database's,
// as TestThePromptReadsLLMWikiMatrix reads them; each of the upgrade's
// states is on a copy of its own, its notebooks being upgraded as
// TestUpgradePermissionMatrix's are, the 4 prompts × 13 columns read on
// it: a get writes nothing. A prompt added to the file without its row
// here fails.
func TestUpgradePromptMatrix(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	// The copies first: copying wants no one connected to the prepared
	// database, which the LLM Wiki's and the standard notebook's app is.
	urls := map[string]string{"llm_wiki": d.url, "standard": d.url}
	for _, state := range []string{"upgrading", "reviewing"} {
		url := pgtest.NewDatabaseFrom(t, d.url)
		pool := connect(t, url)
		upgraded := map[string]bool{}
		for _, c := range notebookColumns() {
			if name := notebookOf(c); !upgraded[name] {
				upgraded[name] = true
				upgradedBy(state)(t, c, d.seeded.in(t), pool)
			}
		}
		if got := count(t, pool, "SELECT count(*) FROM notebook_upgrades WHERE state = $1", state); got != 4 {
			t.Fatalf("the %s copy has %d upgrades %s, want the four live notebooks of the columns'", state, got, state)
		}
		pool.Close()
		urls[state] = url
	}
	// An app a database, which lives as long as the test: the LLM Wiki's
	// and the standard notebook's, and each copy's.
	bases, pats := map[string]string{}, map[string]map[caller]string{}
	for _, state := range upgradePromptStates() {
		if url := urls[state]; bases[url] == "" {
			bases[url] = startApp(t, d.config(t, url, productionDeadlines), migrations.FS())
		}
	}
	t.Run("tokens", func(t *testing.T) {
		for url, base := range bases {
			pats[url] = map[caller]string{}
			for _, c := range notebookColumns() {
				pats[url][c] = createToken(t, contract, base, d.tokens[c])
			}
		}
	})
	if len(bases) != 3 || len(pats) != 3 {
		t.Fatal("the matrix needs its three apps' 13 real PATs each; select prepare and tokens")
	}
	why := "Why?"
	texts := map[string]string{
		"ingest":  wiki.PlaybookText(wiki.PlaybookIngest, nil),
		"lint":    wiki.PlaybookText(wiki.PlaybookLint, nil),
		"query":   wiki.PlaybookText(wiki.PlaybookQuery, &why),
		"upgrade": wiki.UpgradePlaybookText(),
	}
	args := map[string]string{"question": why} // what query requires; the others declare no question
	descriptions := promptDescriptions(t)
	answers := upgradePromptAnswers()
	checked := 0
	for _, p := range readPromptLists(t)["notebook"] {
		prompt, _ := p.(map[string]any)["name"].(string)
		states, ok := answers[prompt]
		if !ok || texts[prompt] == "" {
			t.Fatalf("no row of the prompt %s", prompt)
		}
		for _, state := range upgradePromptStates() {
			for _, c := range notebookColumns() {
				t.Run(prompt+"/"+state+"/"+string(c), func(t *testing.T) {
					s := d.seeded.in(t)
					notebook := s.notebook(notebookOf(c))
					if state == "llm_wiki" {
						notebook = s.wiki.notebook(notebookOf(c))
					}
					want := wikiMCPReader(c)
					if want.code == "notebook.not_found" {
						want.http = http.StatusNotFound
					} else {
						want.code = states[state]
					}
					url := urls[state]
					m := &mcpApp{base: bases[url], pat: pats[url][c]}
					a := m.getPrompt(t, atNotebook(notebook.String()), m.pat, prompt, args)
					if got := promptAnswerOf(t, a, texts[prompt], descriptions[prompt]); got != want {
						t.Fatalf("%s/%s/%s = %s, want %s", prompt, state, c, got, want)
					}
					checked++
				})
			}
		}
	}
	if checked != 4*4*13 {
		t.Fatalf("checked %d answers, want the four prompts in four states by 13 columns", checked)
	}
}

// The upgrade's prompt through the notebook entry (M11/P3 design 3.18): a
// standard notebook without an upgrade is refused, wiki.upgrade_state as
// -32602; once its admin starts the upgrade, she and a reader who is no
// admin take it (lint.read), its one user message the playbook, its
// description the contract's; one who cannot see the notebook stops at
// the HTTP edge; the notebook entry lists it upgrading, standard and in
// an LLM Wiki. A get logs one line, the argument it does not declare not
// in it. Submitted, in review, it is refused; rolling back too; an LLM
// Wiki has no upgrade to answer it for.
func TestUpgradePrompt(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "member", "member", nil, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil)))
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Notes","workspace_access":"none"}`).ID
	tm.createPageWith(t, "alice", nb, "", "Note", cleanNote)
	tm.addNotebookMember(t, "alice", nb, "bob", "reader")
	plain := tm.openNotebook(t, "alice", "Plain")
	llmWiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`).ID
	alice, bob, carol := tm.agent(t, "alice", nb), tm.agent(t, "bob", nb), tm.agent(t, "carol", nb)
	text, description := wiki.UpgradePlaybookText(), promptDescriptions(t)["upgrade"]
	if description == "" {
		t.Fatal("api/mcp/prompts.json has no upgrade prompt")
	}
	get := func(t *testing.T, m *mcpApp, notebook string) mcpAnswer {
		t.Helper()
		return m.getPrompt(t, atNotebook(notebook), m.pat, "upgrade", map[string]string{"source": "raw/never-logged.md"})
	}
	refused := mcpCell{code: "wiki.upgrade_state"}
	// lines are the log's prompt lines after before, and how many lines
	// there are now.
	lines := func(t *testing.T, before int) ([]map[string]any, int) {
		t.Helper()
		all := (&mcpApp{logs: logs}).logLines(t)
		return slices.DeleteFunc(slices.Clone(all[before:]), func(l map[string]any) bool { return l["msg"] != "MCP prompt call" }), len(all)
	}
	// line checks a get's line: its keys, no more, and its values.
	line := func(t *testing.T, l map[string]any, by, notebook, outcome string) {
		t.Helper()
		keys := []string{"client", "duration_ms", "entry", "level", "msg", "notebook_id", "outcome", "prompt", "request_id", "time", "token_id", "user_id"}
		if got := lineKeysOf(l); !slices.Equal(got, keys) {
			t.Errorf("the line's keys = %v, want %v", got, keys)
		}
		if l["level"] != "INFO" || l["prompt"] != "upgrade" || l["entry"] != "notebook" || l["notebook_id"] != notebook ||
			l["user_id"] != tm.userOf(t, by) || l["outcome"] != outcome {
			t.Errorf("the line = %v, want %s's get of %s, %s", l, by, notebook, outcome)
		}
	}

	t.Run("no upgrade", func(t *testing.T) {
		_, before := lines(t, 0)
		a := get(t, alice, nb)
		if code, message, data := promptRefusal(t, a); code != -32602 || data != "wiki.upgrade_state" ||
			message != "wiki.upgrade_state: The upgrade is not in a state that allows this." {
			t.Errorf("a get without an upgrade = %v %q %q", code, message, data)
		}
		got, _ := lines(t, before)
		if len(got) != 1 {
			t.Fatalf("%d prompt lines, want the get's one", len(got))
		}
		line(t, got[0], "alice", nb, "wiki.upgrade_state")
	})

	tm.upgraded(t, "alice", nb)
	t.Run("upgrading", func(t *testing.T) {
		_, before := lines(t, 0)
		for name, m := range map[string]*mcpApp{"alice": alice, "bob": bob} {
			if got := promptAnswerOf(t, get(t, m, nb), text, description); got != (mcpCell{}) {
				t.Errorf("%s's get = %s, want the playbook", name, got)
			}
		}
		got, _ := lines(t, before)
		if len(got) != 2 {
			t.Fatalf("%d prompt lines, want the two gets'", len(got))
		}
		by := map[string]map[string]any{}
		for _, l := range got {
			by[l["user_id"].(string)] = l
		}
		line(t, by[tm.userOf(t, "alice")], "alice", nb, "ok")
		line(t, by[tm.userOf(t, "bob")], "bob", nb, "ok")
		for _, l := range (&mcpApp{logs: logs}).logLines(t) {
			if b, err := json.Marshal(l); err != nil || strings.Contains(string(b), "never-logged") {
				t.Errorf("a line holds the argument: %s, %v", b, err)
			}
		}
	})
	t.Run("hidden", func(t *testing.T) {
		if got := promptAnswerOf(t, get(t, carol, nb), text, description); got != (mcpCell{http: http.StatusNotFound, code: "notebook.not_found"}) {
			t.Errorf("a get by one who cannot see the notebook = %s", got)
		}
	})
	t.Run("listed", func(t *testing.T) {
		want := readPromptLists(t)
		for name, notebook := range map[string]string{"upgrading": nb, "standard": plain, "an LLM Wiki": llmWiki} {
			for _, d := range toolListDiff(alice.listedPrompts(t, notebook), want) {
				t.Errorf("%s: %s", name, d)
			}
		}
	})

	tm.submitted(t, "alice", nb)
	t.Run("in review", func(t *testing.T) {
		if got := promptAnswerOf(t, get(t, alice, nb), text, description); got != refused {
			t.Errorf("a get in review = %s, want %s", got, refused)
		}
	})
	tm.exec(t, `UPDATE notebooks SET upgrade_state = 'rolling_back' WHERE id = $1`, nb)
	tm.exec(t, `UPDATE notebook_upgrades SET state = 'rolling_back', ending = 'rejected', rollback_job_id = gen_random_uuid()
		WHERE notebook_id = $1 AND state = 'reviewing'`, nb)
	if got := tm.submission(t, nb); got != "rolling_back rolling_back true false true" {
		t.Fatalf("the upgrade set rolling back = %q", got)
	}
	t.Run("rolling back", func(t *testing.T) {
		if got := promptAnswerOf(t, get(t, alice, nb), text, description); got != refused {
			t.Errorf("a get rolling back = %s, want %s", got, refused)
		}
	})
	t.Run("an LLM Wiki", func(t *testing.T) {
		if got := promptAnswerOf(t, get(t, alice, llmWiki), text, description); got != refused {
			t.Errorf("an LLM Wiki's get = %s, want %s", got, refused)
		}
	})
}
