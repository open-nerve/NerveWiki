package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The full-text search through serve (M8/P5 design 3.10; v0.1 design 13.1,
// item 21): each write a user makes through a request queues what it
// changed in its unit, and the search finds it at once, before the job has
// built its row; a background job's nodes, an import's, a restore's or a
// revert's, queued in bulk, wait for the job, the answer saying building
// meanwhile. The job runs on River in its own queue, again after each run;
// nervewiki reindex queues the rows of another tokenizer. The bootstrap
// tests register no job (testConfig's page.search_index_interval is 0):
// they run it themselves (drainSearch), so that what the queue alone
// answers is seen.

// searchRange is a TextRange answer.
type searchRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// searchHit is a SearchHit answer, as much as the tests read.
type searchHit struct {
	Node struct {
		ID         string `json:"id"`
		NotebookID string `json:"notebook_id"`
		Kind       string `json:"kind"`
		Name       string `json:"name"`
	} `json:"node"`
	Ancestors []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"ancestors"`
	TitleHits []searchRange `json:"title_hits"`
	Excerpt   *struct {
		Text string        `json:"text"`
		Hits []searchRange `json:"hits"`
	} `json:"excerpt"`
	ChangedAt  time.Time `json:"changed_at"`
	Attachment *struct {
		Mime     string `json:"mime"`
		ByteSize int64  `json:"byte_size"`
	} `json:"attachment"`
}

// searchAnswer is a SearchPage answer.
type searchAnswer struct {
	Data       []searchHit `json:"data"`
	NextCursor *string     `json:"next_cursor"`
	Count      *struct {
		Value int    `json:"value"`
		Kind  string `json:"kind"`
	} `json:"count"`
	Building bool `json:"building"`
}

// ids are the hits' node ids, in the answer's order.
func (a searchAnswer) ids() []string {
	out := make([]string, len(a.Data))
	for i, h := range a.Data {
		out[i] = h.Node.ID
	}
	return out
}

// names are the hits' names, in the answer's order.
func (a searchAnswer) names() []string {
	out := make([]string, len(a.Data))
	for i, h := range a.Data {
		out[i] = h.Node.Name
	}
	return out
}

// searchPath is the search of the workspace slug for q, with the further
// parameters of more when it is not nil.
func searchPath(slug, q string, more url.Values) string {
	v := url.Values{"q": {q}}
	for k, vs := range more {
		v[k] = vs
	}
	return "/api/v0/workspaces/" + slug + "/search?" + v.Encode()
}

// search is by's search of acme for q, which must answer 200.
func (tm acmeTeam) search(t *testing.T, by, q string) searchAnswer {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+searchPath("acme", q, nil), tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("search %q as %s = %d %s, want 200", q, by, status, body)
	}
	var a searchAnswer
	decodeAnswer(t, body, &a)
	return a
}

// finds fails t unless by's search of acme for q answers exactly the nodes
// of names, in any order, the index not being built.
func (tm acmeTeam) finds(t *testing.T, by, q string, names ...string) searchAnswer {
	t.Helper()
	a := tm.search(t, by, q)
	got := a.names()
	slices.Sort(got)
	want := slices.Sorted(slices.Values(names))
	if !slices.Equal(got, want) || a.Building || a.NextCursor != nil {
		t.Errorf("search %q = %q, building %t, next %v; want %q, not building, no next page", q, got, a.Building,
			a.NextCursor != nil, want)
	}
	return a
}

// building fails t unless by's search of acme for q answers no node, the
// index being built: a background job's writes wait for the search's job.
func (tm acmeTeam) building(t *testing.T, by, q string) {
	t.Helper()
	if a := tm.search(t, by, q); len(a.Data) != 0 || !a.Building {
		t.Errorf("search %q = %q, building %t; want none, building", q, a.names(), a.Building)
	}
}

// searchIndex is the search's index outside serve's job, on pool.
func searchIndex(pool *pgxpool.Pool) page.SearchIndex {
	return page.NewSearchIndex(pool, interleavingWait)
}

// drainSearch runs the search's job once, as serve's runs it, and fails t
// unless it ran, the lock free, and failed no node.
func drainSearch(t *testing.T, pool *pgxpool.Pool) page.SearchRun {
	t.Helper()
	run, err := searchIndex(pool).Drain(context.Background())
	if err != nil || run.Locked || run.Failed != 0 {
		t.Fatalf("Drain() = %+v, %v; want a run of no failed node", run, err)
	}
	return run
}

// queueSeeded queues for the search every node a test seeded through SQL,
// exact, deleted or not (M8/P5 design 3.8), as the units that made them
// would have: the reads find them through the queue, and a deleted one is
// a candidate the reads' filter must turn away. A node already queued
// stays as it is.
func queueSeeded(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO page_search_queue (node_id, notebook_id, bulk)
		SELECT id, notebook_id, false FROM nodes ORDER BY id ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
}

// checkSearch fails t when the search's index breaks its invariant
// (M8/P5 design 3.10) as it holds whether the job has run or not: each live
// node has a row of its name and its content's revision, or is queued,
// under its own notebook, so that a search finds it as it is now. A row of
// another tokenizer answers as before until nervewiki reindex queues it,
// and is not counted.
func checkSearch(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for what, query := range map[string]string{
		"live nodes neither indexed as they are nor queued": `SELECT count(*) FROM nodes n
			LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
			WHERE n.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM page_search_queue q WHERE q.node_id = n.id)
				AND NOT EXISTS (SELECT 1 FROM page_search s WHERE s.node_id = n.id AND s.name = n.name
					AND s.revision IS NOT DISTINCT FROM c.revision)`,
		"nodes queued under another notebook": `SELECT count(*) FROM page_search_queue q JOIN nodes n ON n.id = q.node_id
			WHERE q.notebook_id <> n.notebook_id`,
	} {
		if n := count(t, pool, query); n != 0 {
			t.Errorf("%d %s, want none", n, what)
		}
	}
}

// checkSearchBuilt runs the job on pool and fails t unless it emptied the
// queue and left the index its nodes' (M8/P5 design 3.10): a row for each
// live node and for no other, at its name and content's revision, by the
// current tokenizer, its lexemes those SearchTokens makes of them now,
// each behind the node's notebook's prefix, and no other (M8 Codex review
// R1).
func checkSearchBuilt(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	drainSearch(t, pool)
	for what, query := range map[string]string{
		"nodes still queued": "SELECT count(*) FROM page_search_queue",
		"live nodes without a row": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM page_search s WHERE s.node_id = n.id)`,
		"rows of no live node": `SELECT count(*) FROM page_search s WHERE NOT EXISTS (
			SELECT 1 FROM nodes n WHERE n.id = s.node_id AND n.deleted_at IS NULL)`,
		"rows of a notebook without a prefix": `SELECT count(*) FROM page_search s JOIN nodes n ON n.id = s.node_id
			WHERE NOT EXISTS (SELECT 1 FROM page_search_tags t WHERE t.notebook_id = n.notebook_id)`,
	} {
		if n := count(t, pool, query); n != 0 {
			t.Errorf("%d %s, want none", n, what)
		}
	}
	rows, err := pool.Query(context.Background(), `SELECT s.node_id::text, n.name, s.name, s.tokenizer, c.revision, s.revision, c.content,
			t.prefix, tsvector_to_array(s.title), tsvector_to_array(s.body)
		FROM page_search s JOIN nodes n ON n.id = s.node_id JOIN page_search_tags t ON t.notebook_id = n.notebook_id
			LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		ORDER BY s.node_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, builtName, tokenizer, prefix string
		var revision, builtRevision *int
		var content *string
		var title, body []string
		if err := rows.Scan(&id, &name, &builtName, &tokenizer, &revision, &builtRevision, &content, &prefix, &title, &body); err != nil {
			t.Fatal(err)
		}
		wantTitle, wantBody, _ := domain.SearchTokens(name, content)
		for _, want := range [][]string{wantTitle, wantBody} {
			for i := range want {
				want[i] = prefix + want[i]
			}
		}
		switch {
		case builtName != name || tokenizer != domain.SearchTokenizer || !equalInts(builtRevision, revision):
			t.Errorf("the row of %s is of %q at %v by %s, want %q at %v by %s", id, builtName, deref(itoa(builtRevision)), tokenizer, name,
				deref(itoa(revision)), domain.SearchTokenizer)
		case !sameLexemes(title, wantTitle) || !sameLexemes(body, wantBody) || (body == nil) != (wantBody == nil):
			t.Errorf("the row of %s holds the title %q and the body %q, want %q and %q", id, title, body, wantTitle, wantBody)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// equalInts reports whether a and b are both nil or both the same.
func equalInts(a, b *int) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// itoa is n as text, nil for none.
func itoa(n *int) *string {
	if n == nil {
		return nil
	}
	s := fmt.Sprint(*n)
	return &s
}

// sameLexemes reports whether a tsvector's lexemes are the tokens', which
// may repeat and come in any order.
func sameLexemes(lexemes, tokens []string) bool {
	return slices.Equal(slices.Compact(slices.Sorted(slices.Values(lexemes))), slices.Compact(slices.Sorted(slices.Values(tokens))))
}

// Through the API, each kind of write a request makes is found by the next
// search at once, through the queue: a page and an attachment created, a
// content written, a rename and its revert, a move and its rewrite of a
// link to the page moved, a task toggled, a deletion, a restore from the
// trash, a version restored. The job runs before each write, so that the
// index holds the node as it was and the queue nothing: only the write's
// own queueing makes the search find it as it is now, a write queued in
// bulk answering building (M8 closeout A-3). A background job's writes, a
// restore's, a revert's and an import's, are not found until the job has
// built them, the answer building meanwhile (M8/P5A review C-I1). The job
// then leaves the index its nodes'.
func TestTheSearchFindsEachWriteAtOnce(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPageWith(t, "alice", nb, "", "Quokka notes", "the wombat sleeps")
	if a := tm.finds(t, "bob", "quokka", "Quokka notes"); len(a.Data) == 1 && (len(a.Data[0].TitleHits) != 1 ||
		a.Data[0].TitleHits[0] != (searchRange{0, 6}) || a.Data[0].Node.Kind != "page" || a.Data[0].Node.NotebookID != nb) {
		t.Errorf("the hit = %+v, want the page, its title's first word", a.Data[0])
	}
	if a := tm.finds(t, "bob", "wombat", "Quokka notes"); len(a.Data) == 1 && (a.Data[0].Excerpt == nil ||
		a.Data[0].Excerpt.Text != "the wombat sleeps" || !slices.Equal(a.Data[0].Excerpt.Hits, []searchRange{{4, 10}})) {
		t.Errorf("the hit = %+v, want the content's wombat", a.Data[0])
	}

	drainSearch(t, tm.pool)
	photo := tm.upload(t, "alice", nb, id, "numbat.png", sevenByFive)
	if a := tm.finds(t, "bob", "numbat", "numbat.png"); len(a.Data) == 1 && (a.Data[0].Node.Kind != "asset" ||
		a.Data[0].Attachment == nil || a.Data[0].Attachment.Mime != "image/png" || a.Data[0].Attachment.ByteSize != int64(len(sevenByFive)) ||
		len(a.Data[0].Ancestors) != 1 || a.Data[0].Ancestors[0].Name != "Quokka notes" || a.Data[0].Excerpt != nil) {
		t.Errorf("the hit = %+v, want the attachment under its page, its type and size", a.Data[0])
	}

	drainSearch(t, tm.pool)
	tm.send(t, contentWrite("alice", id, "the dingo runs", 1, ""), http.StatusOK)
	tm.finds(t, "bob", "dingo", "Quokka notes")
	tm.finds(t, "bob", "wombat")

	drainSearch(t, tm.pool)
	tm.send(t, request("alice", http.MethodPatch, "/api/v0/nodes/"+id, `{"name":"Bilby notes"}`), http.StatusOK)
	renamed := tm.newest(t, nb).ID
	tm.finds(t, "bob", "bilby", "Bilby notes")
	tm.finds(t, "bob", "quokka")
	drainSearch(t, tm.pool)
	tm.reverted(t, "alice", renamed)
	tm.finds(t, "bob", "quokka", "Quokka notes")
	tm.finds(t, "bob", "bilby")

	shelf := tm.createPage(t, "alice", nb, "", "Shelf")
	burrow := tm.createPage(t, "alice", nb, "", "Burrow")
	echidna := tm.createPage(t, "alice", nb, "", "Echidna")
	pointer := tm.createPageWith(t, "alice", nb, "", "Pointer", "[e](/Echidna.md)\n")
	drainSearch(t, tm.pool)
	tm.send(t, request("alice", http.MethodPost, "/api/v0/nodes/"+id+"/move", `{"parent_id":"`+shelf+`"}`), http.StatusOK)
	if a := tm.finds(t, "bob", "numbat", "numbat.png"); len(a.Data) == 1 && len(a.Data[0].Ancestors) != 2 {
		t.Errorf("the hit's ancestors = %+v, want Shelf and Quokka notes", a.Data[0].Ancestors)
	}
	// The move's rewrite of Pointer's link, in its unit: found by its new
	// path at once.
	tm.send(t, nodeMove("alice", echidna, burrow), http.StatusOK)
	tm.wrote(t, pointer, "[e](/Burrow/Echidna.md)\n", 2)
	tm.finds(t, "bob", "burrow", "Burrow", "Pointer")
	checkSearch(t, tm.pool)

	// A task toggled: its page queued exact, in its unit. A toggle changes
	// no word, so the search finds the page all the same; checkSearch
	// fails it if the toggle queued nothing.
	chores := tm.createPageWith(t, "alice", nb, "", "Chores", "- [ ] feed the platypus\n")
	drainSearch(t, tm.pool)
	tm.send(t, taskToggle("bob", chores, 1, 3, true), http.StatusOK)
	tm.finds(t, "bob", "platypus", "Chores")
	checkSearch(t, tm.pool)

	drainSearch(t, tm.pool)
	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	tm.finds(t, "bob", "quokka")
	tm.finds(t, "bob", "numbat")
	drainSearch(t, tm.pool) // the rows of the page and its attachment deleted
	tm.send(t, nodeRestore("alice", id), http.StatusOK)
	tm.finds(t, "bob", "quokka", "Quokka notes")
	tm.finds(t, "bob", "numbat", "numbat.png")

	drainSearch(t, tm.pool)
	tm.send(t, versionRestore("alice", id, 1, tm.content(t, "alice", id).Revision), http.StatusOK)
	tm.finds(t, "bob", "wombat", "Quokka notes")
	tm.finds(t, "bob", "dingo")

	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	drainSearch(t, tm.pool)
	tm.finds(t, "bob", "numbat")
	restored := tm.background(t, trashRestoreStart("bob", nb, id))
	if restored.State != "succeeded" {
		t.Fatalf("the background restore = %s, want it succeeded", restored)
	}
	tm.building(t, "bob", "wombat")
	tm.building(t, "bob", "numbat")
	drainSearch(t, tm.pool)
	tm.finds(t, "bob", "wombat", "Quokka notes")
	tm.finds(t, "bob", "numbat", "numbat.png")
	tm.send(t, contentWrite("alice", id, "the emu walks", tm.content(t, "alice", id).Revision, ""), http.StatusOK)
	written := tm.newest(t, nb).ID
	drainSearch(t, tm.pool)
	if reverted := tm.background(t, revertStart("bob", nb, written, "")); reverted.State != "succeeded" ||
		reverted.counts()["written"] != 1 || reverted.counts()["put_back"] != 0 {
		t.Fatalf("the background revert = %s, want it succeeded, one page written back", reverted)
	}
	// The row is of the emu's content still, which the check refuses.
	tm.building(t, "bob", "wombat")
	tm.building(t, "bob", "emu")
	drainSearch(t, tm.pool)
	tm.finds(t, "bob", "wombat", "Quokka notes")
	tm.finds(t, "bob", "emu")
	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	tm.finds(t, "bob", "wombat")
	tm.finds(t, "bob", "numbat")
	checkSearch(t, tm.pool)

	j := tm.started(t, importStep(t, "bob", nb, "", vaultZip(t, vaultFile{"Kiwi.md", "the kakapo calls"}, vaultFile{"takahe.png", pngHead})))
	if ended := tm.endedJob(t, "bob", j.ID); ended.State != "succeeded" {
		t.Fatalf("the import = %+v, want it succeeded", ended)
	}
	tm.building(t, "bob", "kakapo")
	checkSearch(t, tm.pool)
	drainSearch(t, tm.pool)
	tm.finds(t, "bob", "kakapo", "Kiwi")
	tm.finds(t, "bob", "takahe", "takahe.png")
	tm.finds(t, "bob", "numbat")
	checkSearchBuilt(t, tm.pool)
	checkPages(t, tm.pool)
	if n := count(t, tm.pool, "SELECT count(*) FROM page_search WHERE node_id = $1", photo.ID); n != 0 {
		t.Errorf("%d rows of the attachment deleted, want none", n)
	}
}

// A background restore of more pages than a search walks (M8/P5A review
// C-I1, at the size of its probe): its nodes, the newest of the notebook,
// are queued in bulk, no candidates until the job has built them, the
// answer saying building. The first page of a search of older pages holds
// their hits at once, counted exact; walked first, the restored pages,
// none a hit, would have left the hits for a later page, behind pages of
// none with a cursor to search on.
func TestABackgroundRestoreLeavesASearchItsHits(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	var old []string
	for i := range 5 {
		old = append(old, fmt.Sprintf("Old %d", i))
		tm.createPageWith(t, "alice", nb, "", old[i], "the quillwort grows")
	}
	if j := tm.imported(t, "alice", nb, "", folderImport(t, domain.MaxSearchWalk+100)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	folder := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'Folder'", nb)[0]
	drainSearch(t, tm.pool)
	tm.send(t, nodeDeletion("alice", folder), http.StatusNoContent)
	drainSearch(t, tm.pool)
	if j := tm.background(t, trashRestoreStart("bob", nb, folder)); j.State != "succeeded" ||
		j.counts()["pages"] != domain.MaxSearchWalk+101 {
		t.Fatalf("the background restore = %s, want the folder's %d pages back", j, domain.MaxSearchWalk+101)
	}
	a := tm.search(t, "bob", "quillwort")
	if got := slices.Sorted(slices.Values(a.names())); !slices.Equal(got, old) || !a.Building || a.NextCursor != nil || a.Count == nil ||
		a.Count.Value != 5 || a.Count.Kind != "exact" {
		t.Errorf("search quillwort = %q, building %t, next %v, count %+v; want the five old pages, exact, building", got, a.Building,
			a.NextCursor != nil, a.Count)
	}
	drainSearch(t, tm.pool)
	tm.finds(t, "bob", "quillwort", old...)
	tm.finds(t, "bob", "p007", "p006", "p007") // p006 links to p007
	checkSearchBuilt(t, tm.pool)
}

// The search's job runs on River in the page_search queue, on serve's
// start, then every page.search_index_interval: once a run has completed,
// the next is inserted, the job unique but for the completed ones (M8/P5
// design 3.4). An import's pages are found once it has run, without a test
// running it.
func TestTheSearchJobRunsAgainOnItsQueue(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(cfg *config.Config) { cfg.Page.SearchIndexInterval = time.Second })
	const completed = "SELECT count(*) FROM river_job WHERE kind = 'page.index_search' AND queue = 'page_search' AND state = 'completed'"
	for deadline := time.Now().Add(15 * time.Second); count(t, tm.pool, completed) < 2; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%d runs completed on page_search within 15s, want 2: %v", count(t, tm.pool, completed), queryStrings(t, tm.pool,
				"SELECT queue || ' ' || state FROM river_job WHERE kind = 'page.index_search'"))
		}
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind = 'page.index_search' AND queue <> 'page_search'"); n != 0 {
		t.Errorf("%d runs in another queue, want none", n)
	}
	nb := tm.openNotebook(t, "alice", "Eng")
	j := tm.started(t, importStep(t, "bob", nb, "", vaultZip(t, vaultFile{"Kiwi.md", "the kakapo calls"})))
	if ended := tm.endedJob(t, "bob", j.ID); ended.State != "succeeded" {
		t.Fatalf("the import = %+v, want it succeeded", ended)
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		a := tm.search(t, "bob", "kakapo")
		if slices.Equal(a.names(), []string{"Kiwi"}) && !a.Building {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("search kakapo = %q, building %t, 15s after the import; want Kiwi, built", a.names(), a.Building)
		}
	}
	checkPages(t, tm.pool)
}

// A run of the search's job that a stopped server left running, its row as
// River holds it, holds no later run back: the next start discards it,
// before River enqueues the job, unique while it runs, and the job runs at
// once rather than after River's rescue, hours later (M8/P5A review B-I1).
func TestASearchRunLeftRunningByAStoppedServerIsDiscardedAtTheStart(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	configured := func() config.Config {
		cfg := testConfig(t, url, false)
		cfg.Page.SearchIndexInterval = time.Second
		return cfg
	}
	const completed = "SELECT count(*) FROM river_job WHERE kind = 'page.index_search' AND state = 'completed'"
	first := buildApp(t, configured(), migrations.FS())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- first.run(ctx) }()
	waitHealthy(t, first.cfg.Server.AddrFile, done)
	for deadline := time.Now().Add(15 * time.Second); count(t, pool, completed) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no run completed within 15s of the first start")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run() = %v, want nil after cancel", err)
	}
	// What a server killed mid-run leaves: the run's row running.
	var stale int64
	if err := pool.QueryRow(context.Background(), `UPDATE river_job SET state = 'running', finalized_at = NULL, attempted_at = now(),
		attempt = greatest(attempt, 1) WHERE id = (SELECT max(id) FROM river_job WHERE kind = 'page.index_search')
		RETURNING id`).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	before := count(t, pool, completed)
	runApp(t, buildApp(t, configured(), migrations.FS()))
	for deadline := time.Now().Add(15 * time.Second); count(t, pool, completed) <= before; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no run completed within 15s of the start; the runs %q", queryStrings(t, pool,
				"SELECT id || ' ' || state FROM river_job WHERE kind = 'page.index_search' ORDER BY id"))
		}
	}
	if got := queryStrings(t, pool, "SELECT state::text FROM river_job WHERE id = $1", stale); !slices.Equal(got, []string{"discarded"}) {
		t.Errorf("the run left running is %q, want discarded", got)
	}
}

// nervewiki reindex queues a notebook's nodes whose rows are missing or of
// another tokenizer, in bulk, and leaves a current row as it is: after a
// change of the tokenizer, the search says building until the job has
// built the rows again (M8/P5 design 3.4).
func TestReindexQueuesTheRowsOfAnotherTokenizer(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	first := tm.createPageWith(t, "alice", nb, "", "Alpha", "the wombat sleeps")
	tm.createPageWith(t, "alice", nb, first, "Beta", "the dingo runs")
	tm.upload(t, "alice", nb, first, "numbat.png", sevenByFive)
	drainSearch(t, tm.pool)
	cfg := testConfig(t, tm.url, false)
	reindex := func(want string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if err := Reindex(context.Background(), cfg, &stderr, &stdout, uuid.MustParse(nb)); err != nil ||
			!strings.HasSuffix(stdout.String(), "; "+want+" queued for the search\n") {
			t.Fatalf("reindex = %q, %v: %s; want %s queued", stdout.String(), err, stderr.String(), want)
		}
	}
	reindex("0 nodes")
	if n := count(t, tm.pool, "SELECT count(*) FROM page_search_queue"); n != 0 {
		t.Errorf("%d nodes queued with their rows current, want none", n)
	}

	if _, err := tm.pool.Exec(context.Background(), "UPDATE page_search SET tokenizer = '0/0.0.0'"); err != nil {
		t.Fatal(err)
	}
	tm.finds(t, "bob", "wombat", "Alpha")
	reindex("3 nodes")
	if got := queryStrings(t, tm.pool, "SELECT count(*) || ' ' || bool_and(bulk) FROM page_search_queue"); !slices.Equal(got, []string{"3 true"}) {
		t.Errorf("the queue = %q, want the three nodes in bulk", got)
	}
	if a := tm.search(t, "bob", "wombat"); !a.Building || !slices.Equal(a.names(), []string{"Alpha"}) {
		t.Errorf("search wombat = %q, building %t; want Alpha by its row as it was, building", a.names(), a.Building)
	}
	checkSearch(t, tm.pool)
	checkSearchBuilt(t, tm.pool)
	tm.finds(t, "bob", "dingo", "Beta")
	checkPages(t, tm.pool)
}
