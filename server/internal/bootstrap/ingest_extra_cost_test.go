//go:build !race

package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	pagedomain "github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// Separate top-level processes keep each additional data shape reviewable.
// The original two formal tops and their 4M/100k/100-op samples stay unchanged.
func TestIngestOfAMaximumSourcePageTakesItsTime(t *testing.T) {
	ingestAdditionalRewriteCost(t, "maximum_source_page", []int{page.MaxContentBytes}, 10000, true, false, ingestCostBatchBudget, 11*time.Second)
}
func TestIngestOfMixedSourcePagesTakesItsTime(t *testing.T) {
	ingestAdditionalRewriteCost(t, "mixed_maximum_bytes", heaviestLast(pagedomain.MaxBatchRewriteBytes), 10000, true, false, ingestCostDenseRequestBudget, ingestCostDenseUnitBudget)
}
func TestIngestOfUnreferencedSourcesTakesItsTime(t *testing.T) {
	sizes := make([]int, 400)
	for i := range sizes {
		sizes[i] = 5 + 8*300
	}
	ingestAdditionalRewriteCost(t, "unreferenced_old_f", sizes, 300, false, false, ingestCostBatchBudget, 11*time.Second)
}

// The single REST rename was judged by the test's 4s request deadline, as
// M10/P4 designed it ("verified with the existing request and parser
// bounds, raising no limit", 04-P4-virtual-ingest.md:215); the 11s passed
// below went unasserted for it from 6d1549fc, as a REST rename logs no
// unit. On main at 5154e0de it answered 500 at 4.0s. From M12 it runs under
// production's 15s and 11s judges it (00-M12, the side branch
// m12-ingest-request-deadline, as decision 359).
func TestIngestOfMoreThanBatchSourcesTakesItsTime(t *testing.T) {
	sizes := make([]int, 401)
	for i := range sizes {
		sizes[i] = 5 + 8*300
	}
	ingestAdditionalRewriteCost(t, "single_rename_401_sources", sizes, 300, true, true, ingestCostBatchBudget, 11*time.Second)
}

// ingestAdditionalTeam starts the app under request, and answers the
// request deadline it started under.
func ingestAdditionalTeam(t *testing.T, trace *ingestCostTrace, logs *syncBuffer, request time.Duration) (acmeTeam, time.Duration) {
	var started time.Duration
	tm := newAcmeTeamOn(t, "", "", func(c *config.Config) {
		if c.Server.RequestTimeout != 4*time.Second || c.Database.CommitTimeout != 2*time.Second {
			t.Fatal("original budgets changed")
		}
		if request != ingestCostOrdinaryBudget {
			ingestCostServerBudget(c, request)
		}
		started = c.Server.RequestTimeout
	}, &wiring{queryTracer: trace}, slog.New(slog.NewJSONHandler(logs, nil)))
	return tm, started
}

// ingestAdditionalDeadline is the request deadline a shape's app must run
// under, worked out from its bound alone: the smallest of the test's 4s,
// production's 15s and the dense shapes' 30s (00-M10 decision 150) above
// the bound, so that the bound, not the deadline, judges the shape (00-M12
// decision 359). It fails unless the shape's request, which starts the
// app, is the same: either alone could give a shape a deadline its bound
// does not ask for.
func ingestAdditionalDeadline(t *testing.T, name string, request, bound time.Duration) time.Duration {
	t.Helper()
	want := ingestCostDenseRequestBudget
	for _, d := range []time.Duration{ingestCostOrdinaryBudget, ingestCostBatchBudget} {
		if bound < d {
			want = d
			break
		}
	}
	if bound >= want || request != want {
		t.Fatalf("%s: bound %s; request deadline %s, want %s, the smallest above the bound", name, bound, request, want)
	}
	return want
}
func ingestAdditionalAssets(t *testing.T, tm acmeTeam, nb, root string, n int) []string {
	names := ingestCostAssetNames(n)
	ingestCostSettle(t, tm, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,created_by_id,updated_by_id,created_at,updated_at)
 SELECT gen_random_uuid(),b.id,$2,'asset',n,n,ord,sha256(convert_to(n,'UTF8')),b.created_by_id,b.created_by_id,now(),now()
 FROM notebooks b,unnest($3::text[]) WITH ORDINALITY a(n,ord) WHERE b.id=$1`, nb, root, names)
	ingestCostSettle(t, tm, `INSERT INTO asset_blobs(id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
 SELECT gen_random_uuid(),id,notebook_id,'application/octet-stream',4,source_hash,created_by_id,created_at FROM nodes WHERE notebook_id=$1 AND kind='asset'`, nb)
	ingestCostSettle(t, tm, `INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE notebook_id=$1 AND kind='asset'`, nb)
	return names
}
func ingestAdditionalRewriteCost(t *testing.T, name string, sizes []int, assets int, referenced, single bool, requestBudget, unitBudget time.Duration) {
	trace, logs := &ingestCostTrace{}, &syncBuffer{}
	deadline := ingestAdditionalDeadline(t, name, requestBudget, unitBudget)
	tm, started := ingestAdditionalTeam(t, trace, logs, requestBudget)
	if started != deadline {
		t.Fatalf("%s: the app started under a %s request deadline, want %s", name, started, deadline)
	}
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Additional ingestion shape","mode":"llm_wiki"}`)
	names := ingestAdditionalAssets(t, tm, nb.ID, *nb.SourceRootID, assets)
	var pages []sqlPage
	pairs, totalBytes := 0, 0
	for i, size := range sizes {
		prefix := fmt.Sprintf("S%03d\n", i)
		if len(prefix) != 5 || size < len(prefix)+8*2 || size > page.MaxContentBytes {
			t.Fatalf("source size%d/prefix%d", size, len(prefix))
		}
		n := min(assets, (size-len(prefix))/8)
		var body strings.Builder
		body.WriteString(prefix)
		for _, target := range names[:n] {
			body.WriteString("[](" + target + ")")
		}
		body.WriteString(strings.Repeat("x", size-body.Len()))
		if body.Len() != size {
			t.Fatal("body size lost")
		}
		pages = append(pages, sqlPage{*nb.SourceRootID, fmt.Sprintf("S%03d", i), body.String()})
		if referenced {
			pages = append(pages, sqlPage{"", fmt.Sprintf("Q%03d", i), fmt.Sprintf("[[raw/S%03d]]", i)})
		}
		pairs += n
		totalBytes += size
	}
	if name == "mixed_maximum_bytes" && (totalBytes != pagedomain.MaxBatchRewriteBytes || len(sizes) != 400 || sizes[len(sizes)-1] != page.MaxContentBytes) {
		t.Fatalf("mixed shape bytes%d/pages%d/last%d", totalBytes, len(sizes), sizes[len(sizes)-1])
	}
	seedPagesSQL(t, tm.pool, nb.ID, pages)
	ingestCostSettle(t, tm, `UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c WHERE c.node_id=n.id AND n.notebook_id=$1 AND n.parent_id=$2 AND c.byte_size>0`, nb.ID, *nb.SourceRootID)
	tm.reindex(t, nb.ID)
	// Rewrite.followed processes UUID-sorted source IDs. Prove the actual
	// seeded order, including the heaviest source last for the mixed bound.
	actualOrder := queryStrings(t, tm.pool, "SELECT n.name||':'||c.byte_size::text FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.notebook_id=$1 AND n.parent_id=$2 AND n.name LIKE 'S%' ORDER BY n.id", nb.ID, *nb.SourceRootID)
	expectedOrder := make([]string, len(sizes))
	for i, size := range sizes {
		expectedOrder[i] = fmt.Sprintf("S%03d:%d", i, size)
	}
	if !slices.Equal(actualOrder, expectedOrder) {
		t.Fatalf("actual rewrite UUID/byte order%v want%v", actualOrder, expectedOrder)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM page_links l JOIN nodes n ON n.id=l.source_id JOIN page_contents c ON c.node_id=n.id JOIN indexed_pages ip ON ip.node_id=n.id
 WHERE n.notebook_id=$1 AND n.name LIKE 'S%' AND n.parent_id=$2 AND ip.extractor=2 AND ip.revision=c.revision AND l.property_key IS NULL AND l.resolved_asset AND l.resolved_id IS NOT NULL`, nb.ID, *nb.SourceRootID); n != pairs {
		t.Fatalf("real complete old F%d want%d", n, pairs)
	}
	if count(t, tm.pool, "SELECT count(DISTINCT source_hash) FROM nodes WHERE notebook_id=$1 AND name LIKE 'S%'", nb.ID) != len(sizes) {
		t.Fatal("distinct source hashes missing")
	}
	var ts, ps []string
	for sample := range 3 {
		ts = append(ts, tm.createPageWith(t, "alice", nb.ID, *nb.SourceRootID, fmt.Sprintf("T%d", sample), fmt.Sprintf("independent T%d", sample)))
		ps = append(ps, tm.createPage(t, "alice", nb.ID, "", fmt.Sprintf("P%d", sample)))
	}
	m := tm.agent(t, "alice", nb.ID)
	m.handshakeAs(t, "ingest-extra-cost")
	asset := ingestCostID(t, tm, nb.ID, names[0])
	candidateNames, candidateIDs := make([]string, 3), make([]string, 3)
	for sample := range 3 {
		candidateNames[sample] = names[1+sample]
		candidateIDs[sample] = ingestCostID(t, tm, nb.ID, candidateNames[sample])
	}
	beforeName := names[0]
	for sample := range 3 {
		next := []string{"zz.x", "zz.y", "zz.z"}[sample]
		text := fmt.Sprintf("[A](raw/%s) [[raw/T%d]]", candidateNames[sample], sample)
		var answer mcpAnswer
		var status int
		var body string
		budget := requestBudget
		var request step
		if single {
			request = nodeRename("alice", asset, next)
		}
		// client() gives up after 5s; under the 15s deadline it would judge
		// the rename in place of its bound. Contract checked as ask does.
		patient := &http.Client{Timeout: 2 * budget}
		trace.start()
		metric, err := virtualCostMeasure(t, func() error {
			if single {
				req := newRequest(t, request.method, tm.base+request.path, tm.tokens[request.by], []byte(request.body))
				res, raw := sendRequestBy(t, patient, req)
				tm.contract.CheckResponse(t, req, res)
				status, body = res.StatusCode, string(raw)
			} else {
				answer = m.batch(t, opMove("raw/"+beforeName, "raw/"+next), opWriteOn(fmt.Sprintf("P%d.md", sample), text, versionOf(ps[sample], 1)))
				status, body = answer.status, string(answer.body)
			}
			if status != http.StatusOK {
				return fmt.Errorf("HTTP%d %s", status, body)
			}
			return nil
		})
		queries, statements, indexNS := trace.stop()
		ingestCostEvent(t, fmt.Sprintf("%s/%d", name, sample), metric, queries, indexNS, map[string]any{"source_pages": len(sizes), "source_bytes": totalBytes, "old_f_pairs": pairs, "request_budget_ns": int64(budget), "single_rest_rename": single})
		t.Logf("INGESTRESPONSE case=%s/%d status=%d body=%s", name, sample, status, body)
		took := time.Duration(metric.WallNS)
		if err != nil || took >= budget || single && took >= unitBudget || !single && answer.result(t)["isError"] == true {
			ingestCostPlans(t, tm, statements, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		if took >= budget {
			t.Fatalf("additional source shape exceeded its %s request budget", budget)
		}
		// A REST rename logs no write unit, so its bound judges the whole
		// measured request, which holds the unit.
		if single && took >= unitBudget {
			t.Fatalf("single REST rename took %s, at or above its %s bound (request deadline %s)", took, unitBudget, budget)
		}
		if queries["InspectPageLinksForPreparation"].Calls != int64(len(sizes)) || queries["InspectPageLinksForPreparation"].Rows != int64(pairs) || queries["IngestOldBodyReferencesFirst"].Calls != 0 || queries["IngestOldBodyReferencesAfter"].Calls != 0 {
			t.Fatal("additional source preparation did not completely read old sources")
		}
		residualRows := 0
		if referenced {
			residualRows = len(sizes)
		}
		if !single {
			residualRows += 2
		}
		if queries["LinksReachedExcept"].Calls != 1 || queries["LinksReachedExcept"].Rows != int64(residualRows) {
			t.Fatal("additional source residual relation differs")
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE n.notebook_id=$1 AND n.parent_id=$2 AND n.name LIKE 'S%' AND c.revision=$3`, nb.ID, *nb.SourceRootID, sample+2); n != len(sizes) {
			t.Fatalf("actual system rewrite pages%d want%d", n, len(sizes))
		}
		var cs string
		if single {
			// The single REST rename has only source writes, so it has no new wiki
			// candidate. A separate real ordinary edit proves fresh T afterward;
			// this is explicitly a separate transaction and is outside rename timing.
			if queries["IngestHeadsFirst"].Calls != 0 || queries["IngestNovelSources"].Calls != 0 {
				t.Fatal("source-only rename unexpectedly scanned full coverage")
			}
			f := ingestFixture{tm: tm, nb: nb.ID, raw: *nb.SourceRootID}
			cs = f.write(t, ps[sample], text)
		} else {
			result := answerOf[mcpBatched](t, answer)
			if !result.Changed || result.ChangesetID == nil || result.ResultsCount != 2 {
				t.Fatalf("actual additional batch=%+v", result)
			}
			cs = *result.ChangesetID
			line := (&mcpApp{logs: logs}).loggedLast(t, "batch applied")
			if line == nil || line["rewritten"] != float64(len(sizes)) {
				t.Fatalf("actual rewritten line=%v", line)
			}
			if ms, ok := line["unit_ms"].(float64); !ok || ms >= float64(unitBudget.Milliseconds()) {
				t.Fatalf("additional batch exceeded %s unit guard: %v", unitBudget, line)
			}
			if queries["IngestNovelSources"].Calls != 1 {
				t.Fatal("additional batch did not complete candidate evidence")
			}
		}
		want := []string{ts[sample]}
		if !referenced {
			want = append(want, candidateIDs[sample])
		}
		ingestCostMark(t, tm, cs, want...)
		ingestCostPlans(t, tm, statements, sample == 0)
		beforeName = next
	}
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
}

// Whole Q/S groups are deleted by two real operations, preserving the
// 100-op bound. All old D is captured before all old F, allowing immediate
// folding; each sample has its own groups and a fresh nonempty T.
func TestIngestOfDeletedSourceGroupsTakesItsTime(t *testing.T) {
	trace, logs := &ingestCostTrace{}, &syncBuffer{}
	request, bound := ingestCostBatchBudget, 11*time.Second
	deadline := ingestAdditionalDeadline(t, "delete_q_s", request, bound)
	tm, started := ingestAdditionalTeam(t, trace, logs, request)
	if started != deadline {
		t.Fatalf("delete_q_s: the app started under a %s request deadline, want %s", started, deadline)
	}
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest deleted groups","mode":"llm_wiki"}`)
	names := ingestAdditionalAssets(t, tm, nb.ID, *nb.SourceRootID, 300)
	for sample := range 3 {
		sourceName, qName := fmt.Sprintf("G%d", sample), fmt.Sprintf("Q%d", sample)
		sourceGroup := tm.createPage(t, "alice", nb.ID, *nb.SourceRootID, sourceName)
		qGroup := tm.createPage(t, "alice", nb.ID, "", qName)
		var pages []sqlPage
		for i := range 400 {
			var b strings.Builder
			fmt.Fprintf(&b, "sample%d S%03d\n", sample, i)
			for _, name := range names {
				b.WriteString("[](../" + name + ")")
			}
			pages = append(pages, sqlPage{sourceGroup, fmt.Sprintf("S%03d", i), b.String()}, sqlPage{qGroup, fmt.Sprintf("Q%03d", i), fmt.Sprintf("[[raw/%s/S%03d]]", sourceName, i)})
		}
		seedPagesSQL(t, tm.pool, nb.ID, pages)
		ingestCostSettle(t, tm, `UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c WHERE c.node_id=n.id AND n.notebook_id=$1 AND n.parent_id=$2 AND c.byte_size>0`, nb.ID, sourceGroup)
		tm.reindex(t, nb.ID)
		if count(t, tm.pool, `SELECT count(*) FROM page_links l JOIN nodes n ON n.id=l.source_id WHERE n.parent_id=$1 AND l.property_key IS NULL AND l.resolved_asset AND l.resolved_id IS NOT NULL`, sourceGroup) != 120000 {
			t.Fatal("deleted old F fixture not actually resolved")
		}
		target := tm.createPageWith(t, "alice", nb.ID, *nb.SourceRootID, fmt.Sprintf("T%d", sample), fmt.Sprintf("fresh delete control%d", sample))
		p := tm.createPage(t, "alice", nb.ID, "", fmt.Sprintf("P%d", sample))
		m := tm.agent(t, "alice", nb.ID)
		m.handshakeAs(t, "ingest-deleted-cost")
		trace.start()
		var answer mcpAnswer
		metric, err := virtualCostMeasure(t, func() error {
			answer = m.batch(t, opDeleteAll("raw/"+sourceName+".md"), opDeleteAll(qName+".md"), opWriteOn(fmt.Sprintf("P%d.md", sample), fmt.Sprintf("[A](raw/%s) [[raw/T%d]]", names[1+sample], sample), versionOf(p, 1)))
			return nil
		})
		queries, statements, indexNS := trace.stop()
		ingestCostEvent(t, fmt.Sprintf("delete_q_s/%d", sample), metric, queries, indexNS, map[string]any{"old_f_pairs": 120000, "old_direct_sources": 400, "ops": 3})
		t.Logf("INGESTRESPONSE case=delete_q_s/%d status=%d body=%s", sample, answer.status, answer.body)
		if err != nil || time.Duration(metric.WallNS) >= request || answer.result(t)["isError"] == true {
			ingestCostPlans(t, tm, statements, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		if time.Duration(metric.WallNS) >= request {
			t.Fatalf("deleted-group batch exceeded its %s request deadline", request)
		}
		result := answerOf[mcpBatched](t, answer)
		if result.ChangesetID == nil || !result.Changed || result.ResultsCount != 3 {
			t.Fatalf("real delete result=%+v", result)
		}
		line := (&mcpApp{logs: logs}).loggedLast(t, "batch applied")
		if ms, ok := line["unit_ms"].(float64); !ok || ms >= float64(bound.Milliseconds()) {
			t.Fatalf("deleted group exceeded its %s unit bound: %v", bound, line)
		}
		ingestCostMark(t, tm, *result.ChangesetID, target)
		if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE parent_id IN($1,$2) AND deleted_at IS NOT NULL", sourceGroup, qGroup) != 800 {
			t.Fatal("source/wiki groups did not both delete400 pages")
		}
		if queries["IngestOldBodyReferencesFirst"].Rows+queries["IngestOldBodyReferencesAfter"].Rows != 120000 || queries["LintBodyReferencesFirst"].Rows+queries["LintBodyReferencesAfter"].Rows < 400 || queries["InspectPageLinksForPreparation"].Calls != 0 || queries["IngestNovelSources"].Calls != 1 {
			t.Fatal("deleted D/F prefixes did not finish real evidence")
		}
		ingestCostPlans(t, tm, statements, sample == 0)
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
	}
}
