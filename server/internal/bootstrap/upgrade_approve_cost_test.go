//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The approval guard's notebook (M11/P3 design 7.1, decision 280): in raw
// approveCostFolders folders with a body, each holding approveCostPages
// pages with a body, approveCostEmpty empty pages and approveCostFiles
// attachments; at the root AGENTS, which the start writes, and Notes,
// holding approveCostNotes notes with clean frontmatter.
const (
	approveCostFolders = 500
	approveCostPages   = 90
	approveCostEmpty   = 5
	approveCostFiles   = 4
	approveCostNotes   = 10000
)

// approveCostSources are the hashes the approval writes: the folders',
// their pages' and their attachments', 47,500.
const approveCostSources = approveCostFolders * (1 + approveCostPages + approveCostFiles)

// The other notebooks beside it: LLM Wikis of approveCostOtherPages pages,
// approveCostOtherSources of them sources with their hashes, and
// approveCostOtherFiles attachments in their sources, each with one
// upgrade's baseline of its nodes, so that a read that loses its
// notebook's key reads theirs too (decision 240, review P3B-I1). Nine
// beside the timed approvals' notebook, a tenth of nodes; four beside the
// one whose plans alone are checked, about 22% (decision 292).
const (
	approveCostOthers       = 9
	approveCostPlanOthers   = 4
	approveCostOtherPages   = 50000
	approveCostOtherSources = 45000
	approveCostOtherFiles   = 2000
)

// approveCostRequest is production's request, approveCostDeadline the
// approval's three quarters of it (decision 279).
const (
	approveCostRequest  = 15 * time.Second
	approveCostDeadline = approveCostRequest * 3 / 4
)

// approveCostBound is the median of three approvals the guard allows in
// each state and plan: three times the largest of the medians of two local
// rounds, rounded up to 100 ms, well within approveCostDeadline. The
// rounds' medians, custom then generic, each never analyzed, as the start
// left them, and written after them, were 1.15, 1.17, 1.24, 1.29, 1.11
// and 1.11 s, and 1.14, 1.15, 1.24, 1.14, 1.12 and 1.12 s; three times
// 1.29 s is 3.86 s. Of an approval, in both rounds, SetSourceHashes took
// 0.74-0.94 s in its five chunks, SourceRebuildHeads 0.11-0.18 s,
// LintNodeMetadata 0.10-0.15 s, DeleteUpgradeNodes 15-32 ms. Each template
// took 10-29 s to seed, the whole guard 121 s. The bound stays (decision
// 298): if a run of costs-1 on main, on an EPYC 7763 or a slower runner,
// shows any cell's median past 3.25 s (a margin under 1.2), it goes to
// 4.5 s, as the owner said on 2026-11-06; not another order, and no
// optimization. The branch's run on a Xeon 8370C showed 2.25-2.51 s.
const approveCostBound = 3900 * time.Millisecond

// The approval's cost guard (M11/P3 design 7.1, decisions 280, 292): the
// notebook above, its upgrade started and submitted through the API, its
// report kept, beside approveCostOthers notebooks, its approval under
// production's 15 s request. Three states of the statistics: never
// analyzed (cleared on the copy), as the start left them (the seed
// analyzed, then the start analyzed upgrade_nodes), and this notebook's
// pages written after them, as a large notebook just imported; each in
// custom and in generic plans, set for the copy's database, which the
// app's connections show. Each approval on a fresh copy of its template
// on an app of its own, its jobs at start done first. Plans first: in the
// first approval of each, each statement approveCostPlanned names, just
// before its first run, on its connection, inside the transaction, its
// plans for its arguments and generic read nodes only by its key or by an
// index whose first column is notebook_id, each with its key in the
// condition, page_contents, asset_blobs and upgrade_nodes only by their
// keys, never a Seq Scan, a scan repeated on a Nested Loop's inner side by
// the outer row's key, SetSourceHashes the notebook's nodes once at most,
// SourceRebuildHeads its nodes and contents only by their keys. Time
// second: each approval answers 200, its 47,500 hashes set, the median of
// three of the server's elapsed_ms within approveCostBound. Then the
// same, the plans of the rebuild's statements alone, beside
// approveCostPlanOthers notebooks, untimed. -v prints each approval's
// parts.
func TestUpgradeApprovalTakesItsTimeAtAnyStatistics(t *testing.T) {
	if testing.Short() {
		t.Skip("the approval's cost guard runs alone, without race")
	}
	contract, keyFile := apitest.Load(t), signingKeyFile(t)
	type template struct{ analyzed, late approveCostTemplate }
	timed := template{approveCostSeed(t, contract, keyFile, approveCostOthers, false),
		approveCostSeed(t, contract, keyFile, approveCostOthers, true)}
	planned := template{approveCostSeed(t, contract, keyFile, approveCostPlanOthers, false),
		approveCostSeed(t, contract, keyFile, approveCostPlanOthers, true)}
	of := func(tm template, statistics string) approveCostTemplate {
		if statistics == "written after them" {
			return tm.late
		}
		return tm.analyzed
	}
	var medians []float64
	for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
		for _, statistics := range approveCostStates() {
			t.Run(mode+"/"+statistics, func(t *testing.T) {
				var took []float64
				for sample := range 3 {
					a := approveCostApprove(t, contract, keyFile, of(timed, statistics), mode, statistics, approveCostPlanned(),
						sample == 0)
					took = append(took, a.elapsed)
					approveCostLog(t, "sample", map[string]any{"mode": mode, "statistics": statistics, "sample": sample,
						"elapsed_ms": a.elapsed, "explain_ms": ms(a.parts.Explain), "statements_ms": a.parts.Took,
						"plans": a.parts.Plans})
				}
				slices.Sort(took)
				medians = append(medians, took[1])
				if bound := float64(approveCostBound.Milliseconds()); took[1] > bound {
					t.Errorf("the median of three approvals took %.1f ms, past %.0f ms: %v", took[1], bound, took)
				}
			})
		}
	}
	approveCostLog(t, "medians", medians)
	for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
		for _, statistics := range approveCostStates() {
			t.Run("plans alone/"+mode+"/"+statistics, func(t *testing.T) {
				a := approveCostApprove(t, contract, keyFile, of(planned, statistics), mode, statistics, approveCostRebuild(), true)
				approveCostLog(t, "plans alone", map[string]any{"mode": mode, "statistics": statistics, "elapsed_ms": a.elapsed,
					"statements_ms": a.parts.Took, "plans": a.parts.Plans})
			})
		}
	}
}

// approveCostStates are the guard's three states of the statistics.
func approveCostStates() []string {
	return []string{"never analyzed", "as the start left them", "written after them"}
}

// approveCostStatistics are the tables whose statistics the guard clears
// and autovacuum would not set behind it: those the approval reads by
// this notebook's key, the one it deletes from, and its two rows'.
func approveCostStatistics() []string {
	return []string{"nodes", "page_contents", "asset_blobs", "upgrade_nodes", "notebook_upgrades"}
}

// approveCostRebuild are the rebuild's statements, in its order.
func approveCostRebuild() []string {
	return []string{"ForgetSources", "LintNodeMetadata", "SourceRebuildHeads", "AttachmentInfos", "SetSourceHashes"}
}

// approveCostPlanned are the statements whose plans the timed approvals
// check: the gates' reads of the root, the rebuild's, and the baseline's
// deletion, after which they are checked. The point reads of notebooks,
// notebook_upgrades, workspaces, users and the members' tables are not:
// a few rows each here, a Seq Scan is the right plan (M11/P3 design 7.1).
func approveCostPlanned() []string {
	return append([]string{"RootNodesByKey", "FindNodeIn", "PageContent"}, append(approveCostRebuild(), "DeleteUpgradeNodes")...)
}

// approveCostLog logs v in JSON after the guard's own prefix.
func approveCostLog(t testing.TB, label string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("upgrade approval cost %s %s", label, b)
}

// approveCostTemplate is a database the guard copies for each approval:
// alice's token and the notebook, its upgrade in review with its report.
type approveCostTemplate struct{ url, token, nb string }

// approveCostSeed makes a template beside others notebooks: the
// notebook, raw and Notes through the API, the rest by SQL, analyzed
// after all or, late, before this notebook's pages; then the upgrade
// started and submitted through the API. Its app and pools are closed as
// it returns, so that it can be copied.
func approveCostSeed(t *testing.T, contract *apitest.Contract, keyFile string, others int, late bool) approveCostTemplate {
	t.Helper()
	tm := approveCostTemplate{url: pgtest.NewDatabase(t)}
	t.Run(fmt.Sprintf("seed beside %d, late %v", others, late), func(t *testing.T) {
		started := time.Now()
		pool := connect(t, tm.url)
		exec := func(sql string, args ...any) { t.Helper(); lintCostExec(t, pool, sql, args...) }
		for _, table := range approveCostStatistics() {
			exec("ALTER TABLE " + table + " SET (autovacuum_enabled = false)")
		}
		base := runApp(t, approveCostApp(t, tm.url, keyFile, nil, slog.New(slog.DiscardHandler)))
		tm.token = registerAccount(t, contract, base, "alice@example.com").AccessToken
		post := func(path, body string) string {
			t.Helper()
			status, got := ask(t, contract, http.MethodPost, base+path, tm.token, body)
			if status != http.StatusCreated {
				t.Fatalf("POST %s = %d %s", path, status, got)
			}
			return idOf(t, answer{body: got})
		}
		if status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/workspaces", tm.token, `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
			t.Fatalf("create acme = %d %s", status, answer)
		}
		tm.nb = post("/api/v0/workspaces/acme/notebooks", `{"name":"Fifty thousand sources"}`)
		raw := post("/api/v0/notebooks/"+tm.nb+"/pages", `{"parent_id":null,"title":"raw"}`)
		notes := post("/api/v0/notebooks/"+tm.nb+"/pages", `{"parent_id":null,"title":"Notes","content":"`+
			strings.ReplaceAll(cleanNote, "\n", `\n`)+`"}`)
		approveCostOthersSeed(t, exec, tm.nb, others)
		if late {
			exec("ANALYZE")
		}
		approveCostNotebookSeed(t, exec, tm.nb, raw, notes)
		if !late {
			exec("ANALYZE")
		}
		seeded := time.Since(started)
		status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/notebooks/"+tm.nb+"/upgrade", tm.token, `{}`)
		var u upgradeAnswer
		if status == http.StatusOK {
			decodeAnswer(t, answer, &u)
		}
		if status != http.StatusOK || u.SourceRootID != raw || len(u.SeededPageIDs) != 1 {
			t.Fatalf("the start = %d %s, want raw its source root, AGENTS written", status, answer)
		}
		status, answer = ask(t, contract, http.MethodPost, base+"/api/v0/notebooks/"+tm.nb+"/upgrade/submit", tm.token, "")
		if status != http.StatusOK || !strings.Contains(answer, `"state":"reviewing"`) || !strings.Contains(answer, `"passed":true`) {
			t.Fatalf("the submission = %d %.2000s, want it reviewing, passed", status, answer)
		}
		var nodes, ours int
		if err := pool.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM nodes), (SELECT count(*) FROM nodes WHERE notebook_id = $1)",
			tm.nb).Scan(&nodes, &ours); err != nil {
			t.Fatal(err)
		}
		approveCostLog(t, "seed", map[string]any{"others": others, "late": late, "seed_ms": ms(seeded),
			"elapsed_ms": ms(time.Since(started)), "nodes": nodes, "notebook_nodes": ours})
	})
	if t.Failed() {
		t.FailNow()
	}
	return tm
}

// approveCostOthersSeed writes the other notebooks beside nb: LLM Wikis
// whose source root raw holds approveCostOtherSources pages with their
// hashes and approveCostOtherFiles attachments with theirs, the rest of
// their pages at the root, every page with a content; a baseline of each
// one's nodes.
func approveCostOthersSeed(t *testing.T, exec func(string, ...any), nb string, others int) {
	t.Helper()
	const ofOthers = `notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`
	exec(`INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), b.workspace_id, 'Other ' || g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $2::integer) g WHERE b.id = $1`, nb, others)
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), o.id, 'page', NULL, 'raw', 'raw', 0, o.created_by_id, o.created_by_id, now(), now()
		FROM notebooks o WHERE o.name LIKE 'Other %'`)
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), r.notebook_id, 'page', CASE WHEN k <= $1 THEN r.id END, 'o' || k, 'o' || k, k, r.created_by_id,
			r.created_by_id, now(), now()
		FROM nodes r, generate_series(1, $2::integer - 1) k WHERE r.`+ofOthers+` AND r.parent_id IS NULL`,
		approveCostOtherSources, approveCostOtherPages)
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), r.notebook_id, 'asset', r.id, 'a' || k || '.bin', 'a' || k || '.bin', 100000 + k, r.created_by_id,
			r.created_by_id, now(), now()
		FROM nodes r, generate_series(1, $1::integer) k WHERE r.`+ofOthers+` AND r.parent_id IS NULL AND r.name_key = 'raw'`,
		approveCostOtherFiles)
	exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE ` + ofOthers + ` AND kind = 'page'`)
	exec(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		SELECT gen_random_uuid(), id, notebook_id, 'application/octet-stream', 1, sha256(convert_to(id::text, 'UTF8')), created_by_id, now()
		FROM nodes WHERE ` + ofOthers + ` AND kind = 'asset'`)
	exec(`UPDATE nodes n SET source_hash = sha256('x') FROM nodes r
		WHERE n.` + ofOthers + ` AND n.kind = 'page' AND r.id = n.parent_id AND r.parent_id IS NULL AND r.name_key = 'raw'`)
	exec(`UPDATE nodes n SET source_hash = b.sha256 FROM asset_blobs b WHERE b.node_id = n.id AND n.` + ofOthers)
	exec(`UPDATE notebooks o SET mode = 'llm_wiki', source_root_id = r.id
		FROM nodes r WHERE r.notebook_id = o.id AND r.parent_id IS NULL AND r.name_key = 'raw' AND o.name LIKE 'Other %'`)
	exec(`INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT md5(n.notebook_id::text)::uuid, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
			c.content_hash, c.byte_size, CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id WHERE n.` + ofOthers)
}

// approveCostNotebookSeed writes the notebook nb's nodes under raw and
// notes by SQL as the API would leave them: each page with its content,
// indexed by the current extractor, the notes with their frontmatter's
// properties; each attachment with its file's row.
func approveCostNotebookSeed(t *testing.T, exec func(string, ...any), nb, raw, notes string) {
	t.Helper()
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), b.id, 'page', $2::uuid, 'f' || g, 'f' || g, g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $3::integer) g WHERE b.id = $1`, nb, raw, approveCostFolders)
	pages, empty := approveCostPages, approveCostPages+approveCostEmpty
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), f.notebook_id, s.kind, f.id, s.name, s.name, k, f.created_by_id, f.created_by_id, now(), now()
		FROM nodes f, generate_series(1, $3::integer) k,
			LATERAL (SELECT CASE WHEN k > $4 THEN 'asset' ELSE 'page' END AS kind,
				'c' || k || CASE WHEN k > $4 THEN '.bin' ELSE '' END AS name) s
		WHERE f.notebook_id = $1 AND f.parent_id = $2`, nb, raw, empty+approveCostFiles, empty)
	exec(`INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), b.id, 'page', $2::uuid, 'n' || g, 'n' || g, g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $3::integer) g WHERE b.id = $1`, nb, notes, approveCostNotes)
	exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT n.id, s.content, 1, sha256(convert_to(s.content, 'UTF8')), octet_length(s.content), n.created_by_id, now()
		FROM nodes n LEFT JOIN page_contents x ON x.node_id = n.id,
			LATERAL (SELECT CASE WHEN n.parent_id = $2 THEN $3::text
				WHEN CASE WHEN n.name_key ~ '^c[0-9]+$' THEN substr(n.name_key, 2)::integer ELSE 0 END > $4 THEN ''
				ELSE 'x' || n.id::text END AS content) s
		WHERE n.notebook_id = $1 AND n.kind = 'page' AND x.node_id IS NULL`, nb, notes, cleanNote, pages)
	exec(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		SELECT gen_random_uuid(), id, notebook_id, 'application/octet-stream', 1, sha256(convert_to(id::text, 'UTF8')), created_by_id, now()
		FROM nodes WHERE notebook_id = $1 AND kind = 'asset'`, nb)
	exec(`INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
		SELECT n.id, n.notebook_id, 1, $2, true FROM nodes n LEFT JOIN indexed_pages i ON i.node_id = n.id
		WHERE n.notebook_id = $1 AND n.kind = 'page' AND i.node_id IS NULL`, nb, linking.Extractor)
	exec(`INSERT INTO page_properties (source_id, position, notebook_id, key, value)
		SELECT n.id, p.position, n.notebook_id, p.key, p.value
		FROM nodes n, (VALUES (0, 'type', '"note"'::jsonb), (1, 'summary', '"A note."'::jsonb)) p(position, key, value)
		WHERE n.notebook_id = $1 AND n.parent_id = $2`, nb, notes)
}

// approveCostApp is an app on url under production's request, its tokens
// alive an hour; with trace, its statements traced, their plans judged as
// trace says, and its baseline's deletion wrapped (approveCostRollback).
func approveCostApp(t *testing.T, url, keyFile string, trace *approveCostTrace, logger *slog.Logger) *app {
	t.Helper()
	cfg := testConfig(t, url, false)
	cfg.Auth.JWT.PrivateKeyFile, cfg.Auth.AccessTokenTTL = keyFile, time.Hour
	cfg.Server.RequestTimeout, cfg.Server.WriteTimeout = approveCostRequest, 45*time.Second
	w := wiring{wiki: func(d *wiki.Deps) {
		if d.ApproveTimeout != approveCostDeadline {
			t.Errorf("the approval's deadline is %v, want %v under a %v request", d.ApproveTimeout, approveCostDeadline, approveCostRequest)
		}
		if trace != nil {
			d.RollbackPages = approveCostRollback{d.RollbackPages, trace}
		}
	}}
	if trace != nil {
		w.queryTracer = trace
	}
	a, err := wireApp(context.Background(), cfg, logger, migrations.FS(), testWebUI(), w)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	return a
}

// approveCostApproval is an approval as the guard read it: the server's
// elapsed_ms less the plans' EXPLAINs, and its statements.
type approveCostApproval struct {
	elapsed float64
	parts   approveCostParts
}

// approveCostApprove copies tm, in statistics and mode, starts an app on
// the copy, waits for its jobs at start, and approves the upgrade: 200,
// the hashes set, the plans of planned checked when plans says.
func approveCostApprove(t *testing.T, contract *apitest.Contract, keyFile string, tm approveCostTemplate, mode, statistics string,
	planned []string, plans bool,
) approveCostApproval {
	t.Helper()
	var out approveCostApproval
	t.Run("approval", func(t *testing.T) {
		copied := pgtest.NewDatabaseFrom(t, tm.url)
		pool := connect(t, copied)
		var database string
		if err := pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database); err != nil {
			t.Fatal(err)
		}
		// The database's own setting: a copy does not take its template's.
		lintCostExec(t, pool, "ALTER DATABASE "+pgx.Identifier{database}.Sanitize()+" SET plan_cache_mode = "+mode)
		if statistics == "never analyzed" {
			for _, table := range approveCostStatistics() {
				lintCostExec(t, pool, "SELECT pg_clear_relation_stats('public', '"+table+"')")
				lintCostExec(t, pool, `SELECT pg_clear_attribute_stats('public', '`+table+`', attname, false)
					FROM pg_attribute WHERE attrelid = '`+table+`'::regclass AND attnum > 0 AND NOT attisdropped`)
			}
		}
		trace, logs := &approveCostTrace{planned: planned}, &syncBuffer{}
		base := runApp(t, approveCostApp(t, copied, keyFile, trace, slog.New(slog.NewJSONHandler(logs, nil))))
		approveCostSettle(t, pool)
		trace.begin(plans)
		status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/notebooks/"+tm.nb+"/upgrade/approve", tm.token, "")
		out.parts = trace.end()
		// The plans' error first: an EXPLAIN that fails leaves the
		// transaction failed, and the approval answers 500.
		if plans && out.parts.PlanErr != nil {
			t.Fatalf("the approval's plans (the approval answered %d): %v", status, out.parts.PlanErr)
		}
		if status != http.StatusOK {
			t.Fatalf("the approval = %d %s", status, answer)
		}
		if out.parts.Mode != mode {
			t.Errorf("the approval's connection planned by %q, want %q", out.parts.Mode, mode)
		}
		lines := upgradeLines(t, logs, 0, "upgrade approved")
		if len(lines) != 1 || lines[0]["sources_hashed"] != float64(approveCostSources) {
			t.Fatalf("logged %v, want one line of %d sources hashed", lines, approveCostSources)
		}
		elapsed, _ := lines[0]["elapsed_ms"].(float64)
		out.elapsed = elapsed - ms(out.parts.Explain)
		if n := count(t, pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", tm.nb); n != approveCostSources {
			t.Errorf("%d sources hashed, want %d", n, approveCostSources)
		}
	})
	if t.Failed() {
		t.FailNow()
	}
	return out
}

// approveCostSettle waits for the app's jobs at start to end, the purge
// first, so that none runs beside the approval.
func approveCostSettle(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	awaitJob(t, pool, jobs.PurgeKind)
	for deadline := time.Now().Add(15 * time.Second); count(t, pool,
		"SELECT count(*) FROM river_job WHERE state IN ('available', 'running', 'retryable') AND scheduled_at <= now()") != 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the jobs at start did not end: %v", queryStrings(t, pool, "SELECT kind || ' ' || state FROM river_job"))
		}
	}
}

// approveCostTrace keeps, while armed, each statement's time by its sqlc
// name, and, when plans, judges the plans of each planned one just before
// its first run, on its connection (decision 296 as review P3S1B-M2
// revised it): the rebuild's are chunked, their first chunk full. The
// statement's time starts after its EXPLAINs.
type approveCostTrace struct {
	mu      sync.Mutex
	armed   bool
	plans   bool
	planned []string
	parts   approveCostParts
}

// approveCostParts are an approval's statements' times by name, the plan
// mode its connection showed, and its plans' check: the plans by name,
// the EXPLAINs' time, and the first error, a plan the rules refuse, an
// EXPLAIN that failed, or a planned statement that did not run.
type approveCostParts struct {
	Took    map[string]float64
	Mode    string
	Explain time.Duration
	PlanErr error
	Plans   map[string][]string // custom, generic
}

func (s *approveCostTrace) begin(plans bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed, s.plans = true, plans
	s.parts = approveCostParts{Took: map[string]float64{}, Plans: map[string][]string{}}
}

func (s *approveCostTrace) end() approveCostParts {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = false
	for _, name := range s.planned {
		if _, judged := s.parts.Plans[name]; s.plans && !judged && s.parts.PlanErr == nil {
			s.parts.PlanErr = fmt.Errorf("the approval ran no %s", name)
		}
	}
	return s.parts
}

// judges reports whether the trace judges the plans of name's run now, its
// first while armed, and marks it judged. After an error it judges none:
// an EXPLAIN that failed leaves the transaction failed.
func (s *approveCostTrace) judges(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, judged := s.parts.Plans[name]; !s.armed || !s.plans || judged || s.parts.PlanErr != nil || !slices.Contains(s.planned, name) {
		return false
	}
	s.parts.Plans[name] = nil
	return true
}

func (s *approveCostTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	rest, ok := strings.CutPrefix(data.SQL, "-- name: ")
	if !ok {
		return ctx // an EXPLAIN of the trace's own among them
	}
	name, _, _ := strings.Cut(rest, " ")
	if s.judges(name) {
		started := time.Now()
		shapes, err := approveCostPlans(ctx, conn, name, data.SQL, data.Args)
		s.mu.Lock()
		s.parts.Explain += time.Since(started)
		s.parts.Plans[name] = shapes
		if err != nil && s.parts.PlanErr == nil {
			s.parts.PlanErr = err
		}
		s.mu.Unlock()
	}
	return context.WithValue(ctx, startCostKey{}, startCostQuery{name, time.Now(), data.SQL, data.Args})
}

func (s *approveCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	q, ok := ctx.Value(startCostKey{}).(startCostQuery)
	if !ok || data.Err != nil {
		return
	}
	took := time.Since(q.at)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.armed {
		return
	}
	s.parts.Took[q.name] += ms(took)
}

// approveCostRollback shows, after the baseline's deletion, inside the
// approval's transaction, the plan mode of its connection.
type approveCostRollback struct {
	wiki.RollbackPages
	s *approveCostTrace
}

func (p approveCostRollback) DeleteBaseline(ctx context.Context, upgradeID uuid.UUID) error {
	if err := p.RollbackPages.DeleteBaseline(ctx, upgradeID); err != nil {
		return err
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return errors.New("the baseline deleted outside a transaction")
	}
	var mode string
	if err := tx.QueryRow(ctx, "SHOW plan_cache_mode").Scan(&mode); err != nil {
		return err
	}
	p.s.mu.Lock()
	p.s.parts.Mode = mode
	p.s.mu.Unlock()
	return nil
}

// approveCostPlans returns the shapes of the plans of the statement name,
// sql, on conn, for args and generic, and an error, which the test
// reports, unless each reads as approveCostScans says. conn is the
// statement's own, idle: pgx traces a query's start before it uses the
// connection.
func approveCostPlans(ctx context.Context, conn *pgx.Conn, name, sql string, args []any) ([]string, error) {
	var shapes []string
	for _, generic := range []bool{false, true} {
		var raw []byte
		if generic {
			// A generic plan's parameters have no values: the simple protocol.
			results, err := conn.PgConn().Exec(ctx, "EXPLAIN (GENERIC_PLAN, FORMAT JSON) "+sql).ReadAll()
			if err != nil {
				return shapes, fmt.Errorf("%s's generic plan: %w", name, err)
			}
			if len(results) != 1 || len(results[0].Rows) != 1 {
				return shapes, fmt.Errorf("%s's generic plan: %d results", name, len(results))
			}
			raw = results[0].Rows[0][0]
		} else if err := conn.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
			return shapes, fmt.Errorf("%s's plan: %w", name, err)
		}
		var plans []struct {
			Plan startCostNode `json:"Plan"`
		}
		if err := json.Unmarshal(raw, &plans); err != nil {
			return shapes, fmt.Errorf("%s's plan, generic %v: %s: %w", name, generic, raw, err)
		}
		if len(plans) != 1 {
			return shapes, fmt.Errorf("%s's plan, generic %v: %d plans: %s", name, generic, len(plans), raw)
		}
		shapes = append(shapes, plans[0].Plan.shape())
		var scans []startCostScan
		plans[0].Plan.scans(false, &scans)
		if err := approveCostScans(name, scans); err != nil {
			return shapes, fmt.Errorf("%s's plan, generic %v: %w: %s", name, generic, err, raw)
		}
	}
	return shapes, nil
}

// approveCostScans fails unless each scan of nodes is by nodes_pkey with
// id in its condition or by an index whose first column is notebook_id
// with it in its condition (review P3S1B-M1),
// each of page_contents by page_contents_pkey, of asset_blobs by
// asset_blobs_node_id_key, of upgrade_nodes by upgrade_nodes_pkey with
// upgrade_id in its condition; a scan repeated on a Nested Loop's inner
// side reads by the outer row's key, nodes with id in its condition
// (nodes_pkey, or nodes_notebook_id_id_key with both), upgrade_nodes with
// node_id in its condition, not the notebook's rows again for each, nor a
// set of nodes by = ANY again for each; SetSourceHashes reads the
// notebook's nodes once at most; and
// SourceRebuildHeads reads its nodes and their contents only by their keys,
// each read keyed as on an inner side, the notebook's nodes not even once
// (decision 292).
func approveCostScans(name string, scans []startCostScan) error {
	once := 0
	for _, s := range scans {
		switch s.Relation {
		case "nodes", "page_contents", "asset_blobs", "upgrade_nodes":
		default:
			continue
		}
		if len(s.indexes) == 0 {
			return fmt.Errorf("%s read by a %s", s.Relation, s.Type)
		}
		for _, ix := range s.indexes {
			var ok, keyed bool
			switch s.Relation {
			case "nodes":
				pkey := ix.Index == "nodes_pkey" && strings.Contains(ix.Cond, "(id = ")
				ok = pkey || strings.HasPrefix(ix.Index, "nodes_notebook_id_") && strings.Contains(ix.Cond, "notebook_id = ")
				keyed = pkey || strings.Contains(ix.Cond, "(id = ")
			case "page_contents":
				ok = ix.Index == "page_contents_pkey" && strings.Contains(ix.Cond, "node_id = ")
				keyed = ok
			case "asset_blobs":
				ok = ix.Index == "asset_blobs_node_id_key" && strings.Contains(ix.Cond, "node_id = ")
				keyed = ok
			case "upgrade_nodes":
				ok = ix.Index == "upgrade_nodes_pkey" && strings.Contains(ix.Cond, "upgrade_id = ")
				keyed = strings.Contains(ix.Cond, "node_id = ")
			}
			if !ok {
				return fmt.Errorf("%s read by a %s of %s on %q", s.Relation, s.Type, ix.Index, ix.Cond)
			}
			if s.repeated && !keyed {
				return fmt.Errorf("%s read again for each outer row by a %s of %s on %q", s.Relation, s.Type, ix.Index, ix.Cond)
			}
			if s.repeated && s.Relation == "nodes" && strings.Contains(ix.Cond, "= ANY") {
				return fmt.Errorf("%s read again for each outer row by a %s of %s on %q, a set each time", s.Relation, s.Type, ix.Index, ix.Cond)
			}
			if name == "SourceRebuildHeads" && !keyed {
				return fmt.Errorf("%s read by a %s of %s on %q, not by its key", s.Relation, s.Type, ix.Index, ix.Cond)
			}
		}
		if s.Relation == "nodes" && !s.repeated {
			once++
		}
	}
	if name == "SetSourceHashes" && once > 1 {
		return fmt.Errorf("the notebook's nodes read %d times", once)
	}
	return nil
}

// The guard's check of the approval's reads (M11/P3 design 7.1) on plans
// written by hand, without a database, so that it runs in the short line,
// which builds this file without race: a key read for each row on a
// Nested Loop's inner side, by nodes_pkey or by the notebook and the id,
// the notebook's nodes read once, and a set of nodes by their key once are
// accepted; the notebook's nodes read again for each row, a Seq Scan, a
// read by another index, SetSourceHashes reading the notebook twice, and
// SourceRebuildHeads reading it even once are refused, and so is each
// read whose condition lacks the key the rules ask of it, once and for
// each row, and a set of nodes read again for each row: a cell for each
// of the rules (review P3S1B-M1).
func TestTheApprovalGuardJudgesItsReads(t *testing.T) {
	scan := func(relation, index, cond, parent string) string {
		return `{"Node Type":"Index Scan","Relation Name":"` + relation + `","Index Name":"` + index + `","Index Cond":"` + cond +
			`","Parent Relationship":"` + parent + `"}`
	}
	loop := func(outer, inner string) string {
		return `{"Node Type":"Nested Loop","Plans":[` + outer + `,` + inner + `]}`
	}
	values := `{"Node Type":"Function Scan","Parent Relationship":"Outer"}`
	notebook := scan("nodes", "nodes_notebook_id_parent_id_idx", "(notebook_id = $1)", "Outer")
	for _, tt := range []struct {
		name, statement, plan string
		ok                    bool
	}{
		{"nodes by their key for each id", "SourceRebuildHeads", loop(values, scan("nodes", "nodes_pkey", "(id = i.id)", "Inner")), true},
		{"nodes by the notebook and the id for each", "SetSourceHashes",
			loop(values, scan("nodes", "nodes_notebook_id_id_key", "((notebook_id = $1) AND (id = v.id))", "Inner")), true},
		{"the notebook's nodes once, their contents by key", "LintNodeMetadata",
			loop(notebook, scan("page_contents", "page_contents_pkey", "(node_id = n.id)", "Inner")), true},
		{"the baseline by its key's prefix", "DeleteUpgradeNodes", scan("upgrade_nodes", "upgrade_nodes_pkey", "(upgrade_id = $1)", ""), true},
		{"the heads joined to the notebook's nodes read once", "SourceRebuildHeads",
			`{"Node Type":"Nested Loop","Plans":[{"Node Type":"Hash Join","Parent Relationship":"Outer","Plans":[` + notebook +
				`,{"Node Type":"Hash","Parent Relationship":"Inner","Plans":[` + values + `]}]},` +
				scan("page_contents", "page_contents_pkey", "(node_id = n.id)", "Inner") + `]}`, false},
		{"the notebook's nodes again for each id", "SourceRebuildHeads",
			loop(values, scan("nodes", "nodes_notebook_id_parent_id_idx", "(notebook_id = $1)", "Inner")), false},
		{"a Seq Scan of page_contents", "LintNodeMetadata",
			`{"Node Type":"Hash Join","Plans":[` + notebook + `,{"Node Type":"Hash","Parent Relationship":"Inner","Plans":[` +
				`{"Node Type":"Seq Scan","Relation Name":"page_contents","Parent Relationship":"Outer"}]}]}`, false},
		{"asset_blobs by the notebook's index", "AttachmentInfos",
			scan("asset_blobs", "asset_blobs_notebook_id_idx", "(notebook_id = $1)", ""), false},
		{"the notebook's nodes twice", "SetSourceHashes",
			`{"Node Type":"Hash Join","Plans":[` + notebook + `,{"Node Type":"Hash","Parent Relationship":"Inner","Plans":[` +
				scan("nodes", "nodes_notebook_id_parent_id_idx", "(notebook_id = $1)", "Outer") + `]}]}`, false},
		{"a set of nodes by their key once", "SetSourceHashes", scan("nodes", "nodes_pkey", "(id = ANY ($1))", ""), true},
		{"the whole of nodes by their key once", "LintNodeMetadata",
			loop(scan("nodes", "nodes_pkey", "", "Outer"), scan("page_contents", "page_contents_pkey", "(node_id = n.id)", "Inner")), false},
		{"the whole of nodes by their key for each id", "SetSourceHashes", loop(values, scan("nodes", "nodes_pkey", "", "Inner")), false},
		{"nodes by the notebook's index with the id alone", "SetSourceHashes",
			loop(values, scan("nodes", "nodes_notebook_id_id_key", "(id = v.id)", "Inner")), false},
		{"a set of nodes by their key again for each id", "SetSourceHashes",
			loop(values, scan("nodes", "nodes_pkey", "(id = ANY ($1))", "Inner")), false},
		{"the notebook's nodes again for each row of another statement", "LintNodeMetadata",
			loop(values, scan("nodes", "nodes_notebook_id_parent_id_idx", "(notebook_id = $1)", "Inner")), false},
		{"the whole of page_contents by its key for each node", "LintNodeMetadata",
			loop(notebook, scan("page_contents", "page_contents_pkey", "", "Inner")), false},
		{"asset_blobs by its node's key without a node", "AttachmentInfos", scan("asset_blobs", "asset_blobs_node_id_key", "", ""), false},
		{"the baseline by the notebook's index", "DeleteUpgradeNodes",
			scan("upgrade_nodes", "upgrade_nodes_notebook_id_idx", "(notebook_id = $1)", ""), false},
		{"the baseline by its key's prefix again for each node", "DeleteUpgradeNodes",
			loop(values, scan("upgrade_nodes", "upgrade_nodes_pkey", "(upgrade_id = $1)", "Inner")), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var plan startCostNode
			if err := json.Unmarshal([]byte(tt.plan), &plan); err != nil {
				t.Fatal(err)
			}
			var scans []startCostScan
			plan.scans(false, &scans)
			if err := approveCostScans(tt.statement, scans); (err == nil) != tt.ok {
				t.Errorf("%s: the check answered %v, want accepted %v", plan.shape(), err, tt.ok)
			}
		})
	}
}
