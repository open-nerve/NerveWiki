//go:build !race

package bootstrap

import (
	"context"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	pagepostgres "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// grep's guards in the composition root (M9/P4 design 6.1, 3.6.4; 13.4
// item 8): MCP's grep at its bounds, over pages seeded by SQL, each within
// three times its time here (S1's report), run alone by make
// test-go-guards with -v; -short skips them. Their app's request timeout
// is production's, 15 s, so that the soft deadline, 12 s in, is no stop.
const (
	// grepWorstLineBound is a call over a line of 5 MiB: 0.72-0.77 s here
	// in three runs.
	grepWorstLineBound = 2300 * time.Millisecond
	// grepWorkBound is a call that stops at MaxGrepWork over lines of
	// 4 KiB: 1.89-1.96 s here.
	grepWorkBound = 5900 * time.Millisecond
	// grepCandidatesBound is GrepCandidates of 100,000 candidates, the
	// median of three, in each state of the statistics: 74-89 ms here.
	grepCandidatesBound = 270 * time.Millisecond
	// grepCandidatesCallBound is a call over them, the tree's 100,000
	// pages walked to the 500th candidate. It was first three times the
	// one call first timed, cold, 143-150 ms here: 450 ms. The median of
	// three (S2 fix round 4) judges a warm call, about 130 ms here: the
	// tree's read about 52 ms, GrepCandidates about 68, the rest 10. On
	// slow CI runners this call ran at 4.0-4.6 times here, 608.6 ms alone
	// (run 39868181582) and a median of 585.7 ms (run 39900615286), while
	// GrepCandidates in the same run stayed near 2.3 times. Past
	// GrepCandidates' statement the call's work is mostly in Go: the
	// tree's rows, 191 MB allocated a call, half the call's CPU here the
	// collector's, which this host's spare cores hide, likely where a
	// runner's few fell behind. 1 s is about
	// 7.5 times the median here and 1.7 times the slowest CI median; it is
	// to catch per-candidate work, which a query or an allocation per
	// candidate turns into seconds.
	grepCandidatesCallBound = time.Second
)

// grepAlternation is 3.6.4's pattern: 36 branches of the class
// [\pL\pN\pM\pS\pP] repeated 70 down to 35 times before QZ, no two alike
// at their starts, so that the parser lifts nothing out of them: 867
// bytes, 1,999 instructions, the most a pattern compiles to.
func grepAlternation() string {
	alts := make([]string, 0, 36)
	for n := 70; n >= 35; n-- {
		alts = append(alts, `[\pL\pN\pM\pS\pP]{`+strconv.Itoa(n)+`}QZ`)
	}
	return "(?:" + strings.Join(alts, "|") + ")"
}

// startGrepGuardApp is a readsApp of production's request timeout.
func startGrepGuardApp(t *testing.T) readsApp {
	t.Helper()
	db := pgtest.NewDatabase(t)
	logs := &syncBuffer{}
	cfg := testConfig(t, db, false)
	cfg.Server.RequestTimeout, cfg.Server.WriteTimeout = 15*time.Second, 25*time.Second
	m := runMCPApp(t, wireMCPAppWith(t, cfg, logs, wiring{}), db, logs)
	return readsApp{mcpApp: m, storage: cfg.Storage.Dir, pool: connect(t, db)}
}

// seedGrepPages inserts n pages at the root of r's notebook through SQL,
// alice's, named prefix and their number of six digits, each holding the
// content content's SQL makes of their number k, as a write leaves a page
// but for the search's queue: a pattern of no keys reads no row of it.
func seedGrepPages(t *testing.T, r readsApp, n int, prefix, content string) {
	t.Helper()
	ctx := context.Background()
	for i, s := range []string{
		`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			SELECT uuidv7(), $1, NULL, 'page', $2 || lpad(k::text, 6, '0'), $2 || lpad(k::text, 6, '0'), k,
				u.id, u.id, now(), now()
			FROM generate_series(1, $3::integer) k, users u WHERE u.email = 'alice@example.com'`,
		`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT c.id, c.content, 1, sha256(convert_to(c.content, 'UTF8')), octet_length(c.content), c.created_by_id, now()
			FROM (SELECT n.id, n.created_by_id, ` + content + ` AS content
				FROM nodes n, LATERAL (SELECT substr(n.name, length($2) + 1)::integer AS k) x
				WHERE n.notebook_id = $1 AND n.name LIKE $2 || '%' AND n.kind = 'page') c`,
	} {
		args := []any{r.notebook, prefix, n}
		if i > 0 {
			args = args[:2]
		}
		if _, err := r.pool.Exec(ctx, s, args...); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// grepWalkOf is the log's last "grep walked" line.
func grepWalkOf(t *testing.T, logs *syncBuffer) grepWalked {
	t.Helper()
	walks := grepWalkLines(t, logs)
	if len(walks) == 0 {
		t.Fatal(`no "grep walked" line`)
	}
	return walks[len(walks)-1]
}

// The worst line (M9/P4 design 6.1, 3.6.4, review A3-m1): a page of one
// line of 5 MiB, dense ASCII without Q, and the 36 branches' pattern; the
// call matches the line's first GrepLineCut bytes, MaxGrepLineWork's
// work, and answers it in partial_lines, within its bound.
func TestGrepOfTheWorstLineTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-guards")
	}
	r := startGrepGuardApp(t)
	seedGrepPages(t, r, 1, "line", `substr(repeat(md5(k::text), 163840), 1, `+strconv.Itoa(domain.MaxContentBytes-1)+`) || E'\n'`)
	start := time.Now()
	a := r.grep(t, map[string]any{"pattern": grepAlternation()})
	took := time.Since(start)
	walk := grepWalkOf(t, r.logs)
	insts := int64(1999)
	t.Logf("a line of %d bytes: %v, work %d of %d", domain.MaxContentBytes-1, took, walk.Work, domain.MaxGrepLineWork)
	if len(a.Hits) != 0 || len(a.PartialLines) != 1 || a.PartialLines[0] != (mcpGrepLine{"line000001.md", 1}) || a.Incomplete ||
		walk.Work > domain.MaxGrepLineWork || walk.Work <= domain.MaxGrepLineWork-3*insts || took > grepWorstLineBound {
		t.Errorf("took %v of %v: %+v, the walk %+v; want line 1 partial, MaxGrepLineWork's work", took, grepWorstLineBound, a, walk)
	}
}

// The call's bound (M9/P4 design 6.1, 3.6.4, reviews A3-m1, C3-M2): 500
// pages of 64 KiB in lines of 4 KiB, dense ASCII without Q, and the 36
// branches' pattern, within MaxGrepPatternBytes and taken, which compiles
// to 1,999 instructions and matches 35 letters and QZ, so that it is the
// work it is counted as: the call stops at MaxGrepWork, within a line of
// it, within its bound.
func TestGrepAtItsWorkBoundTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-guards")
	}
	pattern := grepAlternation()
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := syntax.Compile(re.Simplify())
	if err != nil {
		t.Fatal(err)
	}
	if len(pattern) > domain.MaxGrepPatternBytes || len(prog.Inst) != 1999 ||
		!regexp.MustCompile(pattern).MatchString(strings.Repeat("a", 35)+"QZ") {
		t.Fatalf("the pattern: %d bytes, %d instructions; want within %d, 1,999, matching 35 letters and QZ", len(pattern), len(prog.Inst),
			domain.MaxGrepPatternBytes)
	}
	r := startGrepGuardApp(t)
	seedGrepPages(t, r, 500, "work",
		`(SELECT string_agg(substr(repeat(md5(k::text || '-' || j::text), 128), 1, 4095), E'\n') || E'\n' FROM generate_series(1, 16) j)`)
	start := time.Now()
	a := r.grep(t, map[string]any{"pattern": pattern})
	took := time.Since(start)
	walk := grepWalkOf(t, r.logs)
	t.Logf("%d pages of 64 KiB read, %d lines: %v, work %d of %d", walk.Pages, walk.Lines, took, walk.Work, domain.MaxGrepWork)
	if len(a.Hits) != 0 || !a.Incomplete || a.NextCursor == nil || walk.Stop != "work" ||
		walk.Work <= domain.MaxGrepWork-domain.GrepWork(4095, 1999) || walk.Work > domain.MaxGrepWork || took > grepWorkBound {
		t.Errorf("took %v of %v: %d hits, incomplete %v, the walk %+v; want none, incomplete at MaxGrepWork", took, grepWorkBound,
			len(a.Hits), a.Incomplete, walk)
	}
}

// A hundred thousand candidates (M9/P4 design 6.1, reviews A-n4, C-M5):
// the search's rows of 100,000 pages written through SQL, each holding the
// 32 keys of a literal after the notebook's prefix; GrepCandidates of them
// without statistics, analyzed, and planned generic, each within its
// bound, and a call of the literal, which walks the tree to its 500th
// candidate, within its own: the median of three calls, as GrepCandidates'
// (one call took 608.6 ms against 450 ms on a slow CI runner: S2 fix round
// 4; a median of 585.7 ms on another, CI run 39900615286, made the bound
// 1 s), each call's walk checked.
func TestGrepOfAHundredThousandCandidatesTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-guards")
	}
	ctx := context.Background()
	const literal = "abcdefghijklmnopqrstuvwxyz01234567"
	re, err := syntax.Parse(literal, syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	keys := domain.GrepKeys(re)
	if len(keys) != domain.MaxGrepKeys {
		t.Fatalf("%d keys, want %d", len(keys), domain.MaxGrepKeys)
	}
	r := startGrepGuardApp(t)
	for _, table := range []string{"nodes", "page_contents", "page_search"} {
		if _, err := r.pool.Exec(ctx, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)"); err != nil {
			t.Fatal(err)
		}
	}
	seedGrepPages(t, r, 100_000, "cand", `E'x\n'`)
	// The notebook's prefix, as the job's first run gives it (TagSearchNotebooks).
	if _, err := r.pool.Exec(ctx, `INSERT INTO page_search_tags (notebook_id, prefix)
		VALUES ($1, page_search_prefix(nextval('page_search_tags_seq'))) ON CONFLICT (notebook_id) DO NOTHING`, r.notebook); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body, partial)
		SELECT n.id, 1, n.name, $2, now(), array_to_tsvector(ARRAY[]::text[]),
			array_to_tsvector(ARRAY(SELECT t.prefix || k FROM unnest($3::text[]) k)), false
		FROM nodes n JOIN page_search_tags t ON t.notebook_id = n.notebook_id WHERE n.notebook_id = $1`,
		r.notebook, domain.SearchTokenizer, keys); err != nil {
		t.Fatal(err)
	}
	generic := func() *pgxpool.Pool {
		cfg, err := pgxpool.ParseConfig(r.url)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_generic_plan"
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	notebook, err := uuid.Parse(r.notebook)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"without statistics", "analyzed", "generic"} {
		pool := r.pool
		switch stage {
		case "analyzed":
			if _, err := r.pool.Exec(ctx, "ANALYZE nodes, page_contents, page_search, page_search_queue"); err != nil {
				t.Fatal(err)
			}
		case "generic":
			pool = generic()
		}
		store := pagepostgres.New(pool)
		var times []time.Duration
		for range 3 {
			start := time.Now()
			ids, err := store.GrepCandidates(ctx, notebook, keys)
			times = append(times, time.Since(start))
			if err != nil || len(ids) != 100_000 {
				t.Fatalf("%s: %d candidates, %v", stage, len(ids), err)
			}
		}
		slices.Sort(times)
		t.Logf("GrepCandidates of 100,000, %s: %v", stage, times)
		if times[1] > grepCandidatesBound {
			t.Errorf("GrepCandidates of 100,000, %s: a median of %v, over %v", stage, times[1], grepCandidatesBound)
		}
	}
	var times []time.Duration
	for range 3 {
		start := time.Now()
		a := r.grep(t, map[string]any{"pattern": literal})
		times = append(times, time.Since(start))
		walk := grepWalkOf(t, r.logs)
		if len(a.Hits) != 0 || !a.Incomplete || walk.Stop != "pages" || walk.Candidates != 100_000 {
			t.Errorf("a call over them: %+v, the walk %+v; want no hit, the 500 pages' stop of 100,000 candidates", a, walk)
		}
	}
	slices.Sort(times)
	t.Logf("a call over them: %v", times)
	if times[1] > grepCandidatesCallBound {
		t.Errorf("a call over them: a median of %v, over %v", times[1], grepCandidatesCallBound)
	}
}
