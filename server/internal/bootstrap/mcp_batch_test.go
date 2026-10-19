package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// The MCP tool batch over the whole program (M9/P3 design 3.14.2–3.14.6):
// a batch refused is refused whole, at its bounds too; the largest batch
// is far within the values' bound; and the tool reaches its use case in
// both entries, its pages found by the search at once. Its merges on the
// database are in mcp_batch_merge_test.go, its links in
// mcp_batch_links_test.go, its interleavings in interleavings_batch_test.go.

// mcpBatched is batch's answer, as these tests read it.
type mcpBatched struct {
	ChangesetID  *string       `json:"changeset_id"`
	At           *time.Time    `json:"at"`
	Changed      bool          `json:"changed"`
	Results      []mcpOpResult `json:"results"`
	ResultsCount int           `json:"results_count"`
}

// mcpOpResult is an op's result in batch's answer.
type mcpOpResult struct {
	Op      string  `json:"op"`
	Path    string  `json:"path"`
	NodeID  string  `json:"node_id"`
	Changed bool    `json:"changed"`
	Version *string `json:"version"`
	Created []struct {
		Path   string `json:"path"`
		NodeID string `json:"node_id"`
	} `json:"created"`
	CreatedCount *int `json:"created_count"`
	Replacements *int `json:"replacements"`
	Pages        *int `json:"pages"`
	Attachments  *int `json:"attachments"`
	MergedInto   *int `json:"merged_into"`
}

// The ops of batch's arguments: a write of content at path; one on the
// version base; an edit of old to with; a move; a delete, and one with the
// page's children.

func opWrite(path, content string) map[string]any {
	return map[string]any{"op": "write", "path": path, "content": content}
}

func opWriteOn(path, content, base string) map[string]any {
	return opWith(opWrite(path, content), "base_version", base)
}

func opEdit(path, old, with string) map[string]any {
	return map[string]any{"op": "edit", "path": path, "old_string": old, "new_string": with}
}

func opMove(from, to string) map[string]any {
	return map[string]any{"op": "move", "from": from, "to": to}
}

func opDelete(path string) map[string]any { return map[string]any{"op": "delete", "path": path} }

func opDeleteAll(path string) map[string]any { return opWith(opDelete(path), "recursive", true) }

// opWith is op with its member key v.
func opWith(op map[string]any, key string, v any) map[string]any {
	out := maps.Clone(op)
	out[key] = v
	return out
}

// batch is m's batch of ops, with a message, in its notebook.
func (m *mcpApp) batch(t testing.TB, ops ...map[string]any) mcpAnswer {
	t.Helper()
	return m.use(t, "batch", batched(ops...))
}

// batchDone is m's batch of ops, failing t unless it is done.
func (m *mcpApp) batchDone(t *testing.T, ops ...map[string]any) mcpBatched {
	t.Helper()
	return answerOf[mcpBatched](t, m.batch(t, ops...))
}

// separately makes each op of ops by its tool alone, in their order,
// failing t unless each is done.
func (m *mcpApp) separately(t *testing.T, ops ...map[string]any) {
	t.Helper()
	for i, op := range ops {
		args := maps.Clone(op)
		tool, _ := args["op"].(string)
		delete(args, "op")
		if a := m.use(t, tool, args); a.result(t)["isError"] == true {
			t.Fatalf("ops[%d], a %s alone, answered %.400s", i, tool, a.body)
		}
	}
}

// in is m in the notebook nb, its token the same.
func (m *mcpApp) in(nb string) *mcpApp {
	c := *m
	c.notebook = nb
	return &c
}

// refusedAt is the op of a tool's error, -1 for none.
func refusedAt(r mcpRefusal) int {
	if r.Op == nil {
		return -1
	}
	return *r.Op
}

// newAcmeTeamLogged is newAcmeTeam, bob a member, on an app that logs
// JSON lines from info up to the buffer it answers.
func newAcmeTeamLogged(t *testing.T) (acmeTeam, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	return newAcmeTeamLogging(t, "member", "", nil, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil))), logs
}

// noErrorLogged fails t when logs hold a line at ERROR.
func noErrorLogged(t *testing.T, logs *syncBuffer) {
	t.Helper()
	if lines := (&mcpApp{logs: logs}).loggedAt(t, "ERROR"); len(lines) != 0 {
		t.Errorf("the app logged %d lines at ERROR, want none: %v", len(lines), lines)
	}
}

// restPage is a page a test makes through REST: its path from the root
// without .md, its parent made before it, and its content.
type restPage struct{ path, content string }

// seed makes pages in the notebook nb through REST as alice, in their
// order, and answers their ids by path.
func (tm acmeTeam) seed(t *testing.T, nb string, pages ...restPage) map[string]string {
	t.Helper()
	ids := map[string]string{}
	for _, p := range pages {
		body := map[string]any{"parent_id": nil, "title": p.path, "content": p.content}
		if i := strings.LastIndexByte(p.path, '/'); i >= 0 {
			body["parent_id"], body["title"] = ids[p.path[:i]], p.path[i+1:]
		}
		status, got := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/pages", tm.tokens["alice"], jsonBody(body))
		if status != http.StatusCreated {
			t.Fatalf("create %s = %d %s", p.path, status, got)
		}
		ids[p.path] = idOf(t, answer{body: got})
	}
	return ids
}

// changesetsIn counts the notebook nb's changesets.
func (tm acmeTeam) changesetsIn(t *testing.T, nb string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", nb)
}

// reindex makes the notebook nb's index its rebuild, as the command
// reindex does: the pages seedSQL makes have their rows without links.
func (tm acmeTeam) reindex(t *testing.T, nb string) {
	t.Helper()
	admin, err := reindexAdmin(tm.pool, testConfig(t, tm.url, false), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), uuid.MustParse(nb)); err != nil {
		t.Fatal(err)
	}
}

// sqlPage is a page seedSQL makes: its parent's id ("" for the root), its
// name and its content.
type sqlPage struct{ parent, name, content string }

// seedSQL makes pages in the notebook nb through SQL (seedPagesSQL).
func (tm acmeTeam) seedSQL(t *testing.T, nb string, pages []sqlPage) {
	t.Helper()
	seedPagesSQL(t, tm.pool, nb, pages)
}

// seedPagesSQL makes pages in the notebook nb through SQL, as a creation
// of its creator's makes them, in one changeset, after their siblings in
// their order: each page's node, content, version and item, its row of the
// index, without links, and its place in the search's queue.
func seedPagesSQL(tb testing.TB, pool *pgxpool.Pool, nb string, pages []sqlPage) {
	tb.Helper()
	ctx := context.Background()
	ids, parents, names, contents := make([]string, len(pages)), make([]string, len(pages)), make([]string, len(pages)),
		make([]string, len(pages))
	for i, p := range pages {
		ids[i], parents[i], names[i], contents[i] = uuid.NewV7().String(), p.parent, p.name, p.content
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	c := uuid.NewV7()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
			SELECT $1, id, 'edit', 'web', created_by_id, now(), now() FROM notebooks WHERE id = $2`, []any{c, nb}},
		{`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			SELECT u.id::uuid, b.id, nullif(u.parent, '')::uuid, 'page', u.name, lower(u.name), 1000 + u.n, b.created_by_id, b.created_by_id,
				now(), now()
			FROM unnest($1::text[], $2::text[], $3::text[]) WITH ORDINALITY AS u(id, parent, name, n), notebooks b WHERE b.id = $4`,
			[]any{ids, parents, names, nb}},
		{`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT u.id::uuid, u.content, 1, sha256(convert_to(u.content, 'UTF8')), octet_length(u.content), b.created_by_id, now()
			FROM unnest($1::text[], $2::text[]) AS u(id, content), notebooks b WHERE b.id = $3`, []any{ids, contents, nb}},
		{`INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
			SELECT gen_random_uuid(), $1, node_id, 1, content, content_hash, byte_size, now(), now() FROM page_contents
			WHERE node_id = ANY($2::uuid[])`, []any{c, ids}},
		{`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, revision, created_at, updated_at)
			SELECT gen_random_uuid(), $1, id, parent_id, name, sort_order, 1, now(), now() FROM nodes WHERE id = ANY($2::uuid[])`,
			[]any{c, ids}},
		{`INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
			SELECT id, notebook_id, 1, $2, true FROM nodes WHERE id = ANY($1::uuid[])`, []any{ids, linking.Extractor}},
		{`INSERT INTO page_search_queue (node_id, notebook_id, bulk) SELECT id, notebook_id, false FROM nodes WHERE id = ANY($1::uuid[])`,
			[]any{ids}},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.args...); err != nil {
			tb.Fatalf("%v: %s", err, q.sql)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		tb.Fatal(err)
	}
}

// A batch refused writes nothing (M9/P3 design 3.14.5): after each
// refusal the notebook's nodes, contents, index and changesets are as they
// were, the answer's code and op as 3.9.3 orders them: an op not found
// after a creation and a move; an edit session's page.locked at the guard
// of the op that writes the page; a move's linking.pages_locked; B1 and
// B3; a version of a page the batch creates. And at the bounds (3.8): 101
// ops; strings past 6 MiB, each op valid; 10,001 nodes deleted; pages read
// for edits past 14 MiB, where 14 MiB is done; the plan's parses past 10
// MiB, which stop it at once; and past 100 pages. The app's deadlines are
// production's (3.8): the 8 MiB the plan parses before it stops take the
// race detector seconds, past the tests' 4 s on a slower machine or beside
// the other shards.
func TestTheMCPBatchIsRefusedWhole(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "member", "", func(c *config.Config) {
		c.Server.WriteTimeout, c.Server.RequestTimeout = 60*time.Second, 15*time.Second
	}, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil)))
	agent := tm.agent(t, "alice", "")
	notebooks := 0
	each := func(name string, pages []restPage, test func(t *testing.T, nb string, ids map[string]string, m *mcpApp)) {
		t.Run(name, func(t *testing.T) {
			notebooks++
			nb := tm.openNotebook(t, "alice", fmt.Sprint("Eng ", notebooks))
			test(t, nb, tm.seed(t, nb, pages...), agent.in(nb))
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
	// call is m's batch of ops, refused with code at the op at (-1: none),
	// the notebook nb as it was; and how long it took.
	call := func(t *testing.T, nb string, m *mcpApp, code string, at int, ops ...map[string]any) (mcpRefusal, time.Duration) {
		t.Helper()
		was, sets := tm.notebookState(t, nb), tm.changesetsIn(t, nb)
		a := m.batch(t, ops...)
		r := refusalOf(t, a)
		if r.Code != code || refusedAt(r) != at {
			t.Errorf("the batch = %s at op %d: %.300s; want %s at op %d", r.Code, refusedAt(r), r.Message, code, at)
		}
		if tm.notebookState(t, nb) != was || tm.changesetsIn(t, nb) != sets {
			t.Errorf("the batch refused changed the notebook")
		}
		return r, a.took
	}
	refused := func(t *testing.T, nb string, m *mcpApp, code string, at int, ops ...map[string]any) mcpRefusal {
		t.Helper()
		r, _ := call(t, nb, m, code, at, ops...)
		return r
	}

	each("an op not found after a creation and a move", []restPage{{"A", "a"}}, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		refused(t, nb, m, "page.not_found", 3, opWrite("N.md", "n"), opMove("A.md", "B.md"), opEdit("B.md", "a", "b"),
			opEdit("Missing.md", "x", "y"))
	})
	each("an op's page being edited", []restPage{{"A", ""}, {"L", "l"}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		tm.openSession(t, "bob", ids["L"])
		r := refused(t, nb, m, "page.locked", 2, opWrite("N.md", "n"), opMove("A.md", "B.md"), opEdit("L.md", "l", "m"))
		if r.Lock == nil || r.Lock.DisplayName != "bob" || !strings.HasPrefix(r.Message, "page.locked: ops[2] (edit): ") {
			t.Errorf("the refusal = %+v, want bob's lock at ops[2]", r)
		}
	})
	each("a move whose links are being edited", []restPage{{"A", ""}, {"Q", "[[A]]"}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		tm.openSession(t, "bob", ids["Q"])
		if r := refused(t, nb, m, "linking.pages_locked", 0, opMove("A.md", "B.md")); len(r.Locks) != 1 || r.Locks[0].DisplayName != "bob" {
			t.Errorf("the refusal = %+v, want bob's lock of Q", r)
		}
	})
	each("a page created, then deleted (B1)", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		refused(t, nb, m, "validation_failed", 1, opWrite("N.md", "n"), opDelete("N.md"))
	})
	each("a page renamed, then deleted (B3)", []restPage{{"F", ""}}, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		refused(t, nb, m, "validation_failed", 1, opMove("F.md", "G.md"), opDelete("G.md"))
	})
	each("a version of a page the batch creates", []restPage{{"A", ""}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		refused(t, nb, m, "validation_failed", 1, opWrite("N.md", "n"),
			opWith(opEdit("N.md", "n", "m"), "base_version", versionOf(ids["A"], 1)))
	})

	each("101 ops", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		refused(t, nb, m, "page.batch_too_large", -1, slices.Repeat([]map[string]any{opDelete("Missing.md")}, domain.MaxBatchOps+1)...)
	})
	each("strings past 6 MiB, each op valid", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		// Contents of 5,200,000 bytes, within 5 MiB, and old_strings of
		// 1,200,000, each within 1 MiB: 6,400,000 bytes and more.
		const content, old = 2_600_000, 600_000
		r := refused(t, nb, m, "page.batch_too_large", -1,
			opWrite("W1.md", strings.Repeat("a", content)), opWrite("W2.md", strings.Repeat("b", content)),
			opWith(opEdit("W1.md", strings.Repeat("a", old), ""), "replace_all", true),
			opWith(opEdit("W2.md", strings.Repeat("b", old), ""), "replace_all", true))
		if !strings.Contains(r.Message, "bytes of strings") {
			t.Errorf("the refusal = %q, want the strings' bytes", r.Message)
		}
	})
	each("10,001 nodes deleted", []restPage{{"R", ""}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		var under []sqlPage
		for i := range domain.MaxUnitNodes {
			under = append(under, sqlPage{ids["R"], fmt.Sprint("s", i), ""})
		}
		tm.seedSQL(t, nb, under)
		refused(t, nb, m, "page.batch_too_large", 1, opWrite("N.md", "n"), opDeleteAll("R.md"))
	})
	// Three pages of a title line and a unit of 1 KiB repeated, 14 MiB in
	// all, and a byte more: each edit takes every unit away, read for it.
	const unit = 1 << 10
	pagesOf := func(t *testing.T, nb string, more int) []string {
		t.Helper()
		var pages []sqlPage
		var contents []string
		for i, n := range []int{4778, 4778, 4779} {
			header := 341 + i/2 + more*(i/2) // 341, 341 and 342 bytes, the last one more
			content := fmt.Sprintf("# P%d%s\n", i, strings.Repeat(" ", header-5)) + strings.Repeat(strings.Repeat("x", unit-1)+"\n", n)
			pages, contents = append(pages, sqlPage{"", fmt.Sprint("P", i), content}), append(contents, content)
		}
		tm.seedSQL(t, nb, pages)
		return contents
	}
	edits := []map[string]any{}
	for i := range 3 {
		edits = append(edits, opWith(opEdit(fmt.Sprintf("P%d.md", i), strings.Repeat("x", unit-1)+"\n", ""), "replace_all", true))
	}
	each("pages read past 14 MiB", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		contents := pagesOf(t, nb, 1)
		if total := len(contents[0]) + len(contents[1]) + len(contents[2]); total != 14<<20+1 {
			t.Fatalf("the pages hold %d bytes, want 14 MiB and one", total)
		}
		refused(t, nb, m, "page.batch_too_large", 2, edits...)
	})
	each("pages read up to 14 MiB", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		contents := pagesOf(t, nb, 0)
		if total := len(contents[0]) + len(contents[1]) + len(contents[2]); total != 14<<20 {
			t.Fatalf("the pages hold %d bytes, want 14 MiB", total)
		}
		m.batchDone(t, edits...)
		sizes := queryStrings(t, tm.pool, `SELECT n.name || ' ' || c.byte_size FROM nodes n JOIN page_contents c ON c.node_id = n.id
			AND c.deleted_at IS NULL WHERE n.notebook_id = $1 ORDER BY n.name`, nb)
		if want := []string{"P0 341", "P1 341", "P2 342"}; !slices.Equal(sizes, want) {
			t.Errorf("the pages edited are of %q, want their title lines alone, %q", sizes, want)
		}
	})
	each("the plan's parses past 10 MiB", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		// Index, of 4 MiB, links to A0 … A49 and holds the marks m00 … m49:
		// a move of Ai, then an edit of Index's mark, 50 times over. The
		// first edit parses Index and its rewrite, 8 MiB; the second would
		// parse it again, past 10 MiB: the plan stops at ops[3], at once.
		// The pages are made through SQL and indexed by a rebuild: a batch
		// making them parsed Index under the request's deadline, and ran past
		// it under the race detector beside the other shards.
		var b strings.Builder
		var pages []sqlPage
		for i := range 50 {
			fmt.Fprintf(&b, "[[A%d]] m%02d\n", i, i)
			pages = append(pages, sqlPage{"", fmt.Sprint("A", i), ""})
		}
		b.WriteString(strings.Repeat("filler line\n", (4<<20-b.Len())/len("filler line\n")))
		tm.seedSQL(t, nb, append(pages, sqlPage{"", "Index", b.String()}))
		tm.reindex(t, nb)
		var ops []map[string]any
		for i := range 50 {
			ops = append(ops, opMove(fmt.Sprintf("A%d.md", i), fmt.Sprintf("B%d.md", i)), opEdit("Index.md", fmt.Sprintf("m%02d", i),
				fmt.Sprintf("M%02d", i)))
		}
		limit := 2 * time.Second
		if markdowntest.Race {
			limit *= 6 // the race detector parses ten times slower: alone here 0.37 s, 3.8 s with it
		}
		_, took := call(t, nb, m, "page.batch_too_large", 3, ops...)
		t.Logf("the batch was refused in %s", took)
		if took > limit {
			t.Errorf("the batch took %s to be refused, want %s at most", took, limit)
		}
	})
	each("the plan's parses past 100 pages", nil, func(t *testing.T, nb string, _ map[string]string, m *mcpApp) {
		// P0 … P59 link to A: a move of A, then an edit of each, whose
		// content the plan rewrites, parsing it and its rewrite.
		setup := []map[string]any{opWrite("A.md", "")}
		ops := []map[string]any{opMove("A.md", "B.md")}
		for i := range 60 {
			setup = append(setup, opWrite(fmt.Sprintf("P%d.md", i), fmt.Sprintf("[[A]] p%d", i)))
			ops = append(ops, opEdit(fmt.Sprintf("P%d.md", i), fmt.Sprintf("p%d", i), fmt.Sprintf("q%d", i)))
		}
		m.batchDone(t, setup...)
		refused(t, nb, m, "page.batch_too_large", 51, ops...)
	})
}

// The largest batch is far within the values' bound (M9/P3 design 3.8.1):
// 100 edits, each with its five arguments, are done; 100 ops of all eleven
// members, each one its tool does not take, are refused as not_allowed;
// neither is the edge's 400. The bodies are the check: bodyshape exports
// no count of their values.
func TestTheLargestBatchIsFarWithinTheValuesBound(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	var b strings.Builder
	for i := range domain.MaxBatchOps {
		fmt.Fprintf(&b, "x%d. ", i)
	}
	p := tm.createPageWith(t, "alice", nb, "", "P", b.String())
	m := tm.agent(t, "alice", nb)
	var largest, every []map[string]any
	for i := range domain.MaxBatchOps {
		edit := opWith(opWith(opEdit("P.md", fmt.Sprintf("x%d.", i), fmt.Sprintf("y%d.", i)), "replace_all", true),
			"base_version", versionOf(p, 1))
		largest = append(largest, edit)
		every = append(every, opWith(opWith(opWith(opWith(opWith(edit, "content", "c"), "from", "P.md"), "to", "Q.md"), "update_links", true),
			"recursive", false))
	}
	if done := m.batchDone(t, largest...); done.ResultsCount != domain.MaxBatchOps || !done.Changed {
		t.Errorf("the largest batch = %d results, changed %t; want 100, changed", done.ResultsCount, done.Changed)
	}
	if !strings.HasPrefix(tm.content(t, "alice", p).Content, "y0. y1. ") {
		t.Errorf("P after the largest batch = %.40q, want its marks edited", tm.content(t, "alice", p).Content)
	}
	a := m.batch(t, every...)
	member := a.toolError(t)
	fields, _ := member["fields"].([]any)
	first, _ := fields[0].(map[string]any)
	if member["code"] != "validation_failed" || member["op"] != 0.0 || first["code"] != "not_allowed" {
		t.Errorf("the batch of every member = %.400s, want validation_failed at ops[0], a member not allowed", a.body)
	}
}

// The batch reaches its use case in both entries (M9/P3 design 3.14.6): an
// ingest's batch, three pages created, one edited, one renamed, one
// deleted, with a message, answers each op; it is one changeset, an edit,
// of its message and the agent's client, which log and changeset read;
// and the search finds the pages it created at once, queued exact (3.10.1).
func TestTheMCPBatchReachesItsUseCase(t *testing.T) {
	for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
		t.Run(entry.String(), func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			ids := tm.seed(t, nb, restPage{"Notes", "draft\n"}, restPage{"Old", "old\n"}, restPage{"Gone", ""})
			m := tm.agent(t, "alice", nb)
			m.handshakeAs(t, "whole-program")
			args := map[string]any{"message": "Ingest the meeting", "ops": []map[string]any{
				opWrite("Inbox/Meeting.md", "zebrafish minutes\n"), opWrite("Actions.md", "zebrafish actions\n"),
				opWrite("Decisions.md", "zebrafish decisions\n"), opEdit("Notes.md", "draft", "final"), opMove("Old.md", "Older.md"),
				opDelete("Gone.md"),
			}}
			var a mcpAnswer
			if entry == mcpserver.AccountEntry {
				args["notebook"] = nb
				a = m.call(t, mcpserver.Pattern, "batch", args)
			} else {
				a = m.use(t, "batch", args)
			}
			b := answerOf[mcpBatched](t, a)
			if b.ChangesetID == nil || b.At == nil || !b.Changed || b.ResultsCount != 6 || len(b.Results) != 6 {
				t.Fatalf("the batch = %+v, want a changeset and six results", b)
			}
			r := b.Results
			if meeting := r[0]; meeting.Op != "write" || meeting.Path != "Inbox/Meeting.md" || meeting.CreatedCount == nil ||
				*meeting.CreatedCount != 2 || len(meeting.Created) != 2 || meeting.Created[0].Path != "Inbox.md" ||
				meeting.Created[1].NodeID != meeting.NodeID || *meeting.Version != versionOf(meeting.NodeID, 1) || !meeting.Changed {
				t.Errorf("ops[0] = %+v, want Meeting and Inbox created, at version 1", meeting)
			}
			if notes := r[3]; notes.Op != "edit" || notes.NodeID != ids["Notes"] || *notes.Replacements != 1 ||
				*notes.Version != versionOf(ids["Notes"], 2) {
				t.Errorf("ops[3] = %+v, want Notes edited once, at version 2", notes)
			}
			if old := r[4]; old.Op != "move" || old.Path != "Older.md" || old.NodeID != ids["Old"] || !old.Changed {
				t.Errorf("ops[4] = %+v, want Old renamed Older", old)
			}
			if gone := r[5]; gone.Op != "delete" || gone.NodeID != ids["Gone"] || *gone.Pages != 1 || *gone.Attachments != 0 {
				t.Errorf("ops[5] = %+v, want Gone deleted", gone)
			}
			if got := queryStrings(t, tm.pool, `SELECT c.kind || ' ' || c.client || ' ' || coalesce(c.message, '-') || ' ' || u.email
				FROM changesets c JOIN users u ON u.id = c.created_by_id WHERE c.notebook_id = $1 AND c.client <> 'web'`, nb); !slices.Equal(got,
				[]string{"edit mcp:whole-program Ingest the meeting alice@example.com"}) {
				t.Errorf("the agent's changesets are %q, want one edit of the batch's message", got)
			}
			if rows, _ := structured(t, m.use(t, "log", map[string]any{"limit": 1}))["changesets"].([]any); len(rows) != 1 ||
				rows[0].(map[string]any)["id"] != *b.ChangesetID || rows[0].(map[string]any)["message"] != "Ingest the meeting" {
				t.Errorf("log's newest = %v, want the batch's changeset", rows)
			}
			header, _ := structured(t, m.use(t, "changeset", map[string]any{"id": *b.ChangesetID}))["changeset"].(map[string]any)
			if header["kind"] != "edit" || header["client"] != "mcp:whole-program" || header["renames"] != true {
				t.Errorf("changeset's header = %v, want the batch's edit, its rename", header)
			}
			tm.finds(t, "alice", "zebrafish", "Meeting", "Actions", "Decisions")
			checkPages(t, tm.pool)
		})
	}
}
