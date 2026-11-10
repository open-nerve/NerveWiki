package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// An upgrade's check and its rules' items through the whole program
// (M11/P2 design 3.11, 7.1, 7.2); the wiki module's half with fakes is
// wiki/app's TestCheckUpgradeAnswersInOrder and its neighbours.

// upgradeCheckAnswer is an UpgradeCheck answer, as much as the tests check.
type upgradeCheckAnswer struct {
	ComputedAt time.Time                    `json:"computed_at"`
	StalePages int64                        `json:"stale_pages"`
	Passed     bool                         `json:"passed"`
	Gates      map[string]upgradeGateAnswer `json:"gates"`
	Rules      []struct {
		Rule  string `json:"rule"`
		Count int    `json:"count"`
	} `json:"rules"`
	Counts upgradeCountsAnswer `json:"counts"`
}

// upgradeGateAnswer is a gate of a check: each gate's fields, those it
// has.
type upgradeGateAnswer struct {
	Passed     bool     `json:"passed"`
	PagesWorse int      `json:"pages_worse"`
	Nodes      int      `json:"nodes"`
	NodesKept  int      `json:"nodes_kept"`
	Pages      int      `json:"pages"`
	PagesKept  int      `json:"pages_kept"`
	NodeIDs    []string `json:"node_ids"`
}

// upgradeCountsAnswer is a check's counts.
type upgradeCountsAnswer struct {
	Sources          int `json:"sources"`
	MovedIntoSources int `json:"moved_into_sources"`
	WikiPages        int `json:"wiki_pages"`
	NewPages         int `json:"new_pages"`
	Changesets       int `json:"changesets"`
	WantedBefore     struct {
		Count int `json:"count"`
	} `json:"wanted_before"`
	WantedNow struct {
		Count int `json:"count"`
	} `json:"wanted_now"`
}

// upgradeGateKeys are the gates in their order (M11 design 4.7).
func upgradeGateKeys() []string {
	return []string{"agents", "source_root", "reserved_titles", "frontmatter", "sources", "links", "preserved", "index"}
}

// failed are a's gates that did not pass, in their order.
func (a upgradeCheckAnswer) failed() []string {
	out := []string{}
	for _, key := range upgradeGateKeys() {
		if !a.Gates[key].Passed {
			out = append(out, key)
		}
	}
	return out
}

// checkUpgrade is by's GET of nb's upgrade's check, and of a rule's items
// when suffix names one, as the API answers it.
func (tm acmeTeam) checkUpgrade(t *testing.T, by, nb, suffix string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/check"+suffix, tm.tokens[by], "")
}

// checked is by's check of nb's upgrade, which must answer 200 with its
// eight gates and eight rules.
func (tm acmeTeam) checked(t *testing.T, by, nb string) upgradeCheckAnswer {
	t.Helper()
	status, answer := tm.checkUpgrade(t, by, nb, "")
	if status != http.StatusOK {
		t.Fatalf("%s's check of %s = %d %s, want 200", by, nb, status, answer)
	}
	var a upgradeCheckAnswer
	decodeAnswer(t, answer, &a)
	if len(a.Gates) != len(upgradeGateKeys()) || len(a.Rules) != 8 {
		t.Fatalf("the check %s, want eight gates and eight rules", answer)
	}
	return a
}

// checkPage is a LintPage answer, as much as the tests check.
type checkPage struct {
	Mode string `json:"mode"`
	Data []struct {
		PageID string `json:"page_id"`
	} `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

// listedCheck is by's read of nb's upgrade's items at suffix, which must
// answer 200.
func (tm acmeTeam) listedCheck(t *testing.T, by, nb, suffix string) checkPage {
	t.Helper()
	status, answer := tm.checkUpgrade(t, by, nb, suffix)
	if status != http.StatusOK {
		t.Fatalf("%s's read of %s's items %s = %d %s, want 200", by, nb, suffix, status, answer)
	}
	var p checkPage
	decodeAnswer(t, answer, &p)
	return p
}

// upgradeRuns is acme on an app whose lint's rules hold the slot of the
// next preparation, armed (heldRules), and whose runs' events the test
// reads: the app of M11/P2 design 7.2.
type upgradeRuns struct {
	tm     acmeTeam
	rules  *heldRules
	events chan wiki.RunEvent
	logs   *syncBuffer
}

// upgradeRunsOn is upgradeRuns on the configuration change alters, whose
// lint must have slots slots and runs of timeout, a submission's part
// apart from its request submit, its default for 0, lint's and ten
// seconds (the wiring asserts them: the design's arithmetic of 7.2 holds
// only for them).
func upgradeRunsOn(t *testing.T, change func(*config.Config), slots int, timeout, submit time.Duration) upgradeRuns {
	t.Helper()
	r := upgradeRuns{rules: &heldRules{held: make(chan struct{}, 1), free: make(chan struct{})},
		events: make(chan wiki.RunEvent, 4096), logs: &syncBuffer{}}
	want := submit
	if want == 0 {
		want = timeout + 10*time.Second
	}
	r.tm = newAcmeTeamOn(t, "member", "", change, &wiring{submitTimeout: submit, wiki: func(d *wiki.Deps) {
		if d.MaxLintRuns != slots || d.LintTimeout != timeout || d.SubmitTimeout != want {
			t.Fatalf("lint's runs = %d of %s, a submission's %s; want %d of %s, %s", d.MaxLintRuns, d.LintTimeout, d.SubmitTimeout, slots,
				timeout, want)
		}
		r.rules.Rules, d.Rules = d.Rules, r.rules
		d.RunObserver = func(e wiki.RunEvent) {
			select {
			case r.events <- e:
			default:
			}
		}
	}}, slog.New(slog.NewJSONHandler(r.logs, nil)))
	return r
}

// submitApp is acme on the separate app of M11/P2 design 7.2: requests of
// 20 s, written in 60 s, eight connections; so lint has two slots and
// runs of 10 s, a caller waits for one about 9.9 s, and a submission's
// part apart from its request has 20 s.
func submitApp(t *testing.T) upgradeRuns {
	t.Helper()
	return upgradeRunsOn(t, func(c *config.Config) {
		c.Server.RequestTimeout, c.Server.WriteTimeout, c.Database.MaxConns = 20*time.Second, 60*time.Second, 8
	}, 2, 10*time.Second, 0)
}

// held sends c with lint's rules armed and waits until its run holds a
// slot in its preparation, its snapshot taken; letGo lets it go and
// answers what c got.
func (r upgradeRuns) held(t *testing.T, c step) (letGo func() answer) {
	t.Helper()
	r.rules.armed.Store(true)
	send := r.tm.sender(t, c)
	done := make(chan answer, 1)
	go func() { done <- send() }()
	select {
	case <-r.rules.held:
	case <-time.After(interleavingWait):
		t.Fatalf("%s took no slot", c.name())
	}
	return func() answer {
		t.Helper()
		r.rules.free <- struct{}{}
		select {
		case a := <-done:
			if a.err == nil {
				r.tm.contract.CheckResponse(t, a.req, a.res)
			}
			return a
		case <-time.After(interleavingWait):
			t.Fatalf("%s did not answer once let go", c.name())
			return answer{}
		}
	}
}

// drain forgets the runs' events so far.
func (r upgradeRuns) drain() {
	for {
		select {
		case <-r.events:
		default:
			return
		}
	}
}

// event waits for the next run event of kind in notebook nb, of flight
// unless it is 0.
func (r upgradeRuns) event(t *testing.T, kind wiki.RunEventKind, nb string, flight uint64) wiki.RunEvent {
	t.Helper()
	timer := time.NewTimer(interleavingWait)
	defer timer.Stop()
	for {
		select {
		case e := <-r.events:
			if e.Kind == kind && e.NotebookID.String() == nb && (flight == 0 || e.FlightID == flight) {
				return e
			}
		case <-timer.C:
			t.Fatalf("no run event %d of %s's flight %d", kind, nb, flight)
			return wiki.RunEvent{}
		}
	}
}

// checkStep is by's GET of nb's upgrade's check, and of a rule's items when
// suffix names one, as a step.
func checkStep(by, nb, suffix string) step {
	return request(by, http.MethodGet, "/api/v0/notebooks/"+nb+"/upgrade/check"+suffix, "")
}

// reviewing sets nb's upgrade reviewing by SQL, as a submission leaves it
// (M11/P2/S2's submission comes a commit later).
func (tm acmeTeam) reviewing(t *testing.T, nb, up string) {
	t.Helper()
	tm.exec(t, `UPDATE notebook_upgrades SET state = 'reviewing', submission_id = gen_random_uuid(), submitted_by_id = created_by_id,
		submitted_at = now() WHERE id = $1`, up)
	tm.exec(t, "UPDATE notebooks SET upgrade_state = 'reviewing' WHERE id = $1", nb)
}

// The check's and the items' codes in M11/P2 design 3.11's order, a cell
// for each code ahead of the next: a cursor that does not decode before
// the notebook, the notebook and its admins before the rule, the rule
// before the upgrade, the upgrade before the cursor's binding (a lint's,
// which binds no proposed root), the binding before the limit; one
// lint slot, held, server_busy; the runs' lines name their kind and the
// upgrade's scope; an upgrade reviewing is checked too. A cursor binds the
// proposed source root, not the upgrade: the previous upgrade's, whose
// root its rollback deleted, is bad_request; one of a raw both upgrades
// propose is read. Last, interleaving #8 on the separate app (7.2).
func TestUpgradeCheck(t *testing.T) {
	r := upgradeRunsOn(t, nil, 1, 2*time.Second, 0)
	tm := r.tm
	orphans := func(t *testing.T, name string, pages ...string) string {
		t.Helper()
		nb := tm.openNotebook(t, "alice", name)
		for _, title := range pages {
			tm.createPageWith(t, "alice", nb, "", title, "---\ntype: note\nsummary: "+title+".\n---\n"+title+"\n")
		}
		return nb
	}
	t.Run("the codes in their order", func(t *testing.T) {
		up := orphans(t, "Codes", "One", "Two")
		plain := orphans(t, "Plain", "One")
		status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+up+"/lint/orphans?limit=1", tm.tokens["alice"], "")
		var lint checkPage
		decodeAnswer(t, answer, &lint)
		if status != http.StatusOK || lint.NextCursor == nil {
			t.Fatalf("the lint's page = %d %s, want a cursor", status, answer)
		}
		lintCursor := url.QueryEscape(*lint.NextCursor)
		tm.upgraded(t, "alice", up)
		none := uuid.NewV7().String()
		for _, tc := range []struct {
			what, by, nb, suffix string
			want                 cell
		}{
			{"a cursor that does not decode, of no notebook", "carol", none, "/nonesuch?limit=0&cursor=bad", cell{http.StatusBadRequest, "bad_request"}},
			{"no notebook", "alice", none, "/nonesuch?limit=0", cell{http.StatusNotFound, "notebook.not_found"}},
			{"no notebook, the check", "alice", none, "", cell{http.StatusNotFound, "notebook.not_found"}},
			{"a notebook carol does not see", "carol", up, "", cell{http.StatusNotFound, "notebook.not_found"}},
			{"an editor's, of an unknown rule", "bob", up, "/nonesuch?limit=0", cellForbidden()},
			{"an editor's", "bob", up, "", cellForbidden()},
			{"an unknown rule, of a notebook not upgraded", "alice", plain, "/nonesuch?limit=0", cell{http.StatusNotFound, "wiki.lint_rule_not_found"}},
			{"no upgrade", "alice", plain, "", cell{http.StatusConflict, "wiki.upgrade_state"}},
			{"no upgrade, a lint's cursor and no limit", "alice", plain, "/orphans?limit=0&cursor=" + lintCursor, cell{http.StatusConflict, "wiki.upgrade_state"}},
			{"a lint's cursor and no limit", "alice", up, "/orphans?limit=0&cursor=" + lintCursor, cell{http.StatusBadRequest, "bad_request"}},
			{"a limit of none", "alice", up, "/orphans?limit=0", cell{http.StatusUnprocessableEntity, "validation_failed"}},
			{"a limit past 100", "alice", up, "/orphans?limit=101", cell{http.StatusUnprocessableEntity, "validation_failed"}},
			{"the check", "alice", up, "", cellOK()},
			{"the items", "alice", up, "/orphans?limit=100", cellOK()},
		} {
			status, answer := tm.checkUpgrade(t, tc.by, tc.nb, tc.suffix)
			wantCode(t, tc.what, status, answer, tc.want)
		}
		page := tm.listedCheck(t, "alice", up, "/orphans?limit=1")
		if page.Mode != "llm_wiki" || len(page.Data) != 1 || page.NextCursor == nil {
			t.Fatalf("the first orphan = %+v, want the LLM Wiki's one and a cursor", page)
		}
		next := tm.listedCheck(t, "alice", up, "/orphans?cursor="+url.QueryEscape(*page.NextCursor))
		if len(next.Data) != 1 || next.Data[0].PageID == page.Data[0].PageID || next.NextCursor != nil {
			t.Errorf("the orphan after %s = %+v, want the other one and no cursor", page.Data[0].PageID, next)
		}
		checkPages(t, tm.pool)
	})

	t.Run("the cursors bind the proposed root", func(t *testing.T) {
		// first is a cursor after the first orphan of nb's upgrade, then
		// abandoned and its rollback done; the upgrade started again.
		again := func(t *testing.T, nb string) (string, upgradeAnswer, upgradeAnswer) {
			t.Helper()
			before := tm.upgraded(t, "alice", nb)
			page := tm.listedCheck(t, "alice", nb, "/orphans?limit=1")
			if page.NextCursor == nil {
				t.Fatalf("the first orphan of %s = %+v, want a cursor", nb, page)
			}
			job := tm.abandoned(t, "alice", nb)
			status, answer := tm.checkUpgrade(t, "alice", nb, "")
			wantCode(t, "the check rolling back", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
			if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" {
				t.Fatalf("the rollback of %s = %+v, want it done", nb, j)
			}
			status, answer = tm.checkUpgrade(t, "alice", nb, "/orphans")
			wantCode(t, "the items rolled back", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
			return url.QueryEscape(*page.NextCursor), before, tm.upgraded(t, "alice", nb)
		}
		seeded := orphans(t, "Seeded", "One", "Two")
		cursor, before, after := again(t, seeded)
		if len(before.SeededPageIDs) != 2 || after.SourceRootID == before.SourceRootID {
			t.Fatalf("the upgrades proposed %s, then %s, want two raw pages their starts wrote", before.SourceRootID, after.SourceRootID)
		}
		status, answer := tm.checkUpgrade(t, "alice", seeded, "/orphans?cursor="+cursor)
		wantCode(t, "the previous upgrade's cursor, its raw rolled back", status, answer, cell{http.StatusBadRequest, "bad_request"})

		kept := orphans(t, "Kept", "One", "Two")
		tm.createPageWith(t, "alice", kept, "", "raw", "Sources.\n")
		cursor, before, after = again(t, kept)
		if len(before.SeededPageIDs) != 1 || after.SourceRootID != before.SourceRootID {
			t.Fatalf("the upgrades proposed %s, then %s, want the raw the notebook held twice", before.SourceRootID, after.SourceRootID)
		}
		if page := tm.listedCheck(t, "alice", kept, "/orphans?cursor="+cursor); len(page.Data) != 1 || page.NextCursor != nil {
			t.Errorf("after the previous upgrade's cursor = %+v, want the second orphan", page)
		}
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
	})

	t.Run("reviewing", func(t *testing.T) {
		nb := orphans(t, "Reviewing", "One", "Two")
		up := tm.upgraded(t, "alice", nb)
		tm.reviewing(t, nb, up.ID)
		if a := tm.checked(t, "alice", nb); !a.Passed {
			t.Errorf("the check reviewing failed %v, want it passed", a.failed())
		}
		if page := tm.listedCheck(t, "alice", nb, "/orphans"); len(page.Data) != 2 {
			t.Errorf("the orphans reviewing = %+v, want two", page)
		}
	})

	t.Run("the runs' lines", func(t *testing.T) {
		nb := orphans(t, "Lines", "One")
		up := tm.upgraded(t, "alice", nb)
		before := len((&mcpApp{logs: r.logs}).logLines(t))
		tm.checked(t, "alice", nb)
		tm.listedCheck(t, "alice", nb, "/missing_type")
		var kinds []string
		for _, l := range upgradeLines(t, r.logs, before, "lint computed") {
			if l["notebook_id"] == nb {
				kinds = append(kinds, fmt.Sprint(l["query_kind"], " ", l["rule"], " ", l["mode"], " ", l["upgrade_scope"]))
			}
		}
		if want := []string{"upgrade_check summary llm_wiki " + up.ID, "upgrade_items missing_type llm_wiki " + up.ID}; !slices.Equal(kinds, want) {
			t.Errorf("the runs' lines = %q, want %q", kinds, want)
		}
	})

	// One slot, held by a rule's items: the check waits for it about 1.9 s
	// and answers server_busy, Retry-After: 1. The held read's own wait
	// ends about then too: it answers what it got, its page or
	// server_busy.
	t.Run("the slot taken", func(t *testing.T) {
		nb := orphans(t, "Busy", "One")
		tm.upgraded(t, "alice", nb)
		letGo := r.held(t, checkStep("alice", nb, "/missing_type"))
		req := newRequest(t, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/check", tm.tokens["alice"], nil)
		at := time.Now()
		res, body := sendRequest(t, req)
		elapsed := time.Since(at)
		held := letGo()
		tm.contract.CheckResponse(t, req, res)
		if res.StatusCode != http.StatusServiceUnavailable || problemCodeOf(string(body)) != "server_busy" ||
			res.Header.Get("Retry-After") != "1" || elapsed > 3*time.Second {
			t.Errorf("the check with the slot held = %d %s %v after %v, want 503 server_busy within its wait", res.StatusCode, body,
				res.Header, elapsed)
		}
		if !held.is(http.StatusOK, "") && !held.is(http.StatusServiceUnavailable, "server_busy") {
			t.Errorf("the held read = %d %s %v, want its page or server_busy", held.status, held.body, held.err)
		}
		tm.checked(t, "alice", nb)
	})

	// #8: both slots held by two rules' items, the check's flight made
	// and queued; alice abandons the upgrade; let go, the check's worker
	// takes a slot and reads the upgrade rolling back in its snapshot:
	// wiki.upgrade_state. The held reads' snapshots, taken before, answer
	// their pages.
	t.Run("8 the check queued and the abandonment", func(t *testing.T) {
		s := submitApp(t)
		nb := s.tm.openNotebook(t, "alice", "Eight")
		s.tm.createPage(t, "alice", nb, "", "Page")
		s.tm.upgraded(t, "alice", nb)
		types := s.held(t, checkStep("alice", nb, "/missing_type"))
		summaries := s.held(t, checkStep("alice", nb, "/missing_summary"))
		s.drain()
		check := s.tm.sender(t, checkStep("alice", nb, ""))
		checked := make(chan answer, 1)
		go func() { checked <- check() }()
		flight := s.event(t, wiki.RunCreated, nb, 0).FlightID
		job := s.tm.abandoned(t, "alice", nb)
		for _, a := range []answer{types(), summaries()} {
			wantAnswer(t, "a held read", a, cellOK())
		}
		s.event(t, wiki.RunAcquired, nb, flight)
		select {
		case a := <-checked:
			if a.err == nil {
				s.tm.contract.CheckResponse(t, a.req, a.res)
			}
			wantAnswer(t, "the check queued, the upgrade abandoned", a, cell{http.StatusConflict, "wiki.upgrade_state"})
		case <-time.After(interleavingWait):
			t.Fatal("the queued check did not answer")
		}
		if j := s.tm.endedBackground(t, "alice", job); j.State != "succeeded" {
			t.Errorf("the rollback = %+v, want it done", j)
		}
		checkPages(t, s.tm.pool)
		checkLinks(t, s.tm.pool)
		checkSearch(t, s.tm.pool)
	})
}

// gated is a notebook of alice's holding pages, written over REST in
// their order after before ran (nil: nothing), whose upgrade she started
// through the API, her agent in it.
func (tm acmeTeam) gated(t *testing.T, before func(t *testing.T, nb string), pages ...upgradedPage) upgradedNotebook {
	t.Helper()
	name := t.Name()[strings.LastIndex(t.Name(), "/")+1:]
	g := upgradedNotebook{tm: tm, id: tm.openNotebook(t, "alice", name), ids: map[string]string{}}
	if before != nil {
		before(t, g.id)
	}
	for _, p := range pages {
		g.ids[p.title] = tm.createPageWith(t, "alice", g.id, g.ids[p.parent], p.title, p.content)
	}
	g.upgrade = tm.upgraded(t, "alice", g.id).ID
	g.agent = tm.agent(t, "alice", g.id)
	return g
}

// typed is a page's content with a type and a summary, then body.
func typed(summary, body string) string {
	return "---\ntype: note\nsummary: " + summary + "\n---\n" + body
}

// Each gate of the check, passed and not, through the whole program
// (M11/P2 design 3.10, 7.1): each notebook upgraded through the API, then
// changed by alice's agent or by SQL. Gate 6 judges pages, not the
// notebook's net count: two pages offset fail it (decision 180); a page
// moved out of the start's raw keeps the count its second pass recorded
// (B2-I1), and one moved into raw, more links wanted since, is not judged
// (the check runs no second pass). Gate 7 is the ledger: only SQL past the
// units fails it (decision 191). Then the seven counts.
func TestUpgradeGates(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	plan := upgradedPage{title: "Plan", content: typed("The plan.", "Plan.\n")}
	var asset string
	for _, tc := range []struct {
		name   string
		before func(t *testing.T, nb string)
		pages  []upgradedPage
		change func(t *testing.T, g upgradedNotebook)
		failed []string
		check  func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer)
	}{{
		name: "every gate passed", pages: []upgradedPage{plan},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			// raw and AGENTS the start wrote; Plan the baseline's one node.
			if p := a.Gates["preserved"]; p.Nodes != 1 || p.NodesKept != 1 || p.Pages != 1 || p.PagesKept != 1 || len(p.NodeIDs) != 0 {
				t.Errorf("the preserved gate = %+v, want Plan kept", p)
			}
		},
	}, {
		name: "AGENTS only frontmatter", pages: []upgradedPage{plan},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "AGENTS's write", "write", map[string]any{"path": "AGENTS.md", "content": "---\ntype: guide\nsummary: The rules.\n---\n \n",
				"base_version": g.version(t, "AGENTS")})
		},
		failed: []string{"agents"},
	}, {
		name: "the source root named AGENTS", pages: []upgradedPage{plan},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "AGENTS's delete", "delete", map[string]any{"path": "AGENTS.md"})
			g.tm.exec(t, "UPDATE nodes SET name = 'AGENTS', name_key = 'agents' WHERE id = $1", g.idOf(t, "raw"))
		},
		failed: []string{"source_root"},
	}, {
		name: "an attachment index.md at the root", pages: []upgradedPage{plan},
		before: func(t *testing.T, nb string) {
			t.Helper()
			asset = tm.upload(t, "alice", nb, "", "pic.png", pngHead).ID
		},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.tm.exec(t, "UPDATE nodes SET name = 'index.md', name_key = 'index.md' WHERE id = $1", asset)
		},
		failed: []string{"reserved_titles"},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if got := a.Gates["reserved_titles"].NodeIDs; !slices.Equal(got, []string{asset}) {
				t.Errorf("the reserved titles' nodes = %v, want the attachment %s", got, asset)
			}
		},
	}, {
		name: "a page without a summary", pages: []upgradedPage{plan},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Draft's write", "write", map[string]any{"path": "Draft.md", "content": "---\ntype: note\n---\nA draft.\n"})
		},
		failed: []string{"frontmatter"},
	}, {
		name: "a source outside the source area", pages: []upgradedPage{plan},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Cites's write", "write", map[string]any{"path": "Cites.md",
				"content": "---\ntype: note\nsummary: Cites.\nsources: ['[[Plan]]']\n---\nCites.\n"})
		},
		failed: []string{"sources"},
	}, {
		name: "two pages offset",
		pages: []upgradedPage{{title: "Plan", content: typed("The plan.", "[[Gone]]\n")},
			{title: "Note", content: typed("A note.", "Note.\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Gone's write", "write", map[string]any{"path": "Gone.md", "content": typed("Gone.", "Gone.\n")})
			g.done(t, "Note's frontmatter", "write", map[string]any{"path": "Note.md",
				"content": "---\ntype: note\nsummary: A note.\nsee: '[[Nowhere]]'\n---\nNote.\n", "base_version": g.version(t, "Note")})
		},
		failed: []string{"links"},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if l := a.Gates["links"]; l.PagesWorse != 1 || !slices.Equal(l.NodeIDs, []string{g.idOf(t, "Note")}) ||
				a.Counts.WantedBefore.Count != 1 || a.Counts.WantedNow.Count != 1 {
				t.Errorf("the links gate = %+v, wanted %+v, want Note worse, one wanted before and now", l, a.Counts)
			}
		},
	}, {
		name:  "a baseline page's body past the units",
		pages: []upgradedPage{plan, {title: "Note", content: typed("A note.", "Note.\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.bypass(t, "Note", typed("A note.", "Changed.\n"))
		},
		failed: []string{"preserved"},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if p := a.Gates["preserved"]; p.Nodes != 2 || p.NodesKept != 2 || p.Pages != 2 || p.PagesKept != 1 ||
				!slices.Equal(p.NodeIDs, []string{g.idOf(t, "Note")}) {
				t.Errorf("the preserved gate = %+v, want Note not kept", p)
			}
		},
	}, {
		name: "a stale index", pages: []upgradedPage{plan},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.tm.exec(t, "UPDATE indexed_pages SET extractor = extractor - 1 WHERE node_id = $1", g.idOf(t, "Plan"))
		},
		failed: []string{"index"},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if a.StalePages != 1 {
				t.Errorf("%d stale pages, want Plan", a.StalePages)
			}
			g.tm.exec(t, "UPDATE indexed_pages SET extractor = extractor + 1 WHERE node_id = $1", g.idOf(t, "Plan"))
		},
	}, {
		name: "a page moved out of raw",
		pages: []upgradedPage{plan, {title: "raw", content: "Sources.\n"},
			{parent: "raw", title: "Source", content: typed("A source.", "[[Nowhere]]\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Source's move out of raw", "move", map[string]any{"from": "raw/Source.md", "to": "Source.md"})
		},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if n := count(t, g.tm.pool, "SELECT wanted FROM upgrade_nodes WHERE upgrade_id = $1 AND node_id = $2", g.upgrade,
				g.idOf(t, "Source")); n != 1 {
				t.Errorf("Source's baseline wanted %d, want the 1 the second pass recorded", n)
			}
		},
	}, {
		name:  "a page moved into raw",
		pages: []upgradedPage{plan, {title: "Out", content: typed("Out.", "[[Ghost]]\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Out's move into raw", "move", map[string]any{"from": "Out.md", "to": "raw/Out.md"})
			g.done(t, "Out's frontmatter", "write", map[string]any{"path": "raw/Out.md",
				"content": "---\ntype: note\nsummary: Out.\nsee: '[[Ghost 2]]'\n---\n[[Ghost]]\n", "base_version": g.version(t, "Out")})
		},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			if a.Counts.WantedBefore.Count != 1 || a.Counts.WantedNow.Count != 0 {
				t.Errorf("wanted %+v, want Out's one before and none of the source area's now", a.Counts)
			}
		},
	}, {
		// raw held two pages as the upgrade started; the agent moves a
		// third in, the only one moved; Plan is the wiki's page, AGENTS
		// the start's new one; the start and the move two changes; Plan's
		// link wanted before and now.
		name: "the counts of a raw the notebook held",
		pages: []upgradedPage{{title: "Plan", content: typed("The plan.", "[[Ghost]]\n")}, {title: "raw", content: "Sources.\n"},
			{parent: "raw", title: "A", content: "a\n"}, {parent: "raw", title: "B", content: "b\n"}, {title: "Moved", content: typed("Moved.", "m\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "Moved's move into raw", "move", map[string]any{"from": "Moved.md", "to": "raw/Moved.md"})
		},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			want := upgradeCountsAnswer{Sources: 3, MovedIntoSources: 1, WikiPages: 1, NewPages: 1, Changesets: 2}
			want.WantedBefore.Count, want.WantedNow.Count = 1, 1
			if a.Counts != want {
				t.Errorf("the counts = %+v, want %+v", a.Counts, want)
			}
		},
	}, {
		// The start wrote raw and AGENTS, a change; the agent moves two
		// pages in and writes Ghost, three; Ghost's write leaves one of
		// Plan's two links wanted.
		name: "the counts of a raw the start wrote",
		pages: []upgradedPage{{title: "Plan", content: typed("The plan.", "[[Ghost]] [[Other]]\n")},
			{title: "One", content: typed("One.", "1\n")}, {title: "Two", content: typed("Two.", "2\n")}},
		change: func(t *testing.T, g upgradedNotebook) {
			t.Helper()
			g.done(t, "One's move into raw", "move", map[string]any{"from": "One.md", "to": "raw/One.md"})
			g.done(t, "Two's move into raw", "move", map[string]any{"from": "Two.md", "to": "raw/Two.md"})
			g.done(t, "Ghost's write", "write", map[string]any{"path": "Ghost.md", "content": typed("Ghost.", "Ghost.\n")})
		},
		check: func(t *testing.T, g upgradedNotebook, a upgradeCheckAnswer) {
			t.Helper()
			want := upgradeCountsAnswer{Sources: 2, MovedIntoSources: 2, WikiPages: 2, NewPages: 3, Changesets: 4}
			want.WantedBefore.Count, want.WantedNow.Count = 2, 1
			if a.Counts != want {
				t.Errorf("the counts = %+v, want %+v", a.Counts, want)
			}
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			g := tm.gated(t, tc.before, tc.pages...)
			if tc.change != nil {
				tc.change(t, g)
			}
			a := tm.checked(t, "alice", g.id)
			failed := tc.failed
			if failed == nil {
				failed = []string{}
			}
			if got := a.failed(); !slices.Equal(got, failed) || a.Passed != (len(failed) == 0) {
				t.Errorf("the check failed %v, passed %t, want %v", got, a.Passed, failed)
			}
			if tc.check != nil {
				tc.check(t, g, a)
			}
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
}
