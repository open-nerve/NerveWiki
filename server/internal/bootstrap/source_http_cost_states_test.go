//go:build !race

package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Each operation prepares one complete, un-analyzed corpus. A stopped seed
// supplies independent owned clones with identical initial rows and files.
// Within each state, create/move/restore/revert retain their original three
// request/reset sequence and its history growth. Forget gets a new clone for
// every sample. Setup, login, resets, observations and invariant scans are
// outside the request timer; no request or commit deadline is increased.
func sourceHTTPCost(t *testing.T, shape string) {
	t.Helper()
	if testing.Short() {
		t.Skip("source request cost uses owned PostgreSQL and repeated timing")
	}
	seed := sourceHTTPPrepare(t, shape)
	for _, stage := range []string{"no_statistics", "analyzed", "force_generic_setting"} {
		t.Run(stage, func(t *testing.T) {
			if shape == "forget" {
				for sample := 0; sample < 3; sample++ {
					t.Run(fmt.Sprintf("sample%d", sample), func(t *testing.T) {
						a := sourceHTTPClone(t, seed, stage)
						defer a.stop()
						sourceHTTPSample(t, a, seed, stage, sample)
					})
				}
				return
			}
			a := sourceHTTPClone(t, seed, stage)
			defer a.stop()
			var samples []time.Duration
			for sample := 0; sample < 3; sample++ {
				samples = append(samples, sourceHTTPSample(t, a, seed, stage, sample))
			}
			if shape == "create" {
				sourceCostWithin(t, "create "+stage+" fastest of three", slices.Min(samples), sourceCreateTime)
			}
			sourceCostHealthy(t, a.team)
		})
	}
}

type sourceHTTPSeed struct {
	database, storage, shape, root string
	wiki, other                    guardedNotebook
	member                         uuid.UUID
	fingerprint                    sourceHTTPRows
	files                          sourceHTTPFileSummary
}

type sourceHTTPApp struct {
	team                          acmeTeam
	actualPool                    *pgxpool.Pool
	stop                          func()
	requestTimeout, commitTimeout time.Duration
}

// The URL startup parameter reaches every connection the real app creates.
// The page hook observes the already wired pool; it replaces no dependency.
// The inspection pool is configured separately so it cannot stand in for
// evidence from the measured app. force_generic_setting is only a setting:
// source ports still use Planned; actual PREPARE-generic proof is elsewhere.
func sourceHTTPStartApp(t *testing.T, database, mode, copyFrom string) sourceHTTPApp {
	t.Helper()
	cfg := testConfig(t, database, false)
	cfg.Database.URL = sourceHTTPModeURL(t, database, mode)
	if cfg.Server.RequestTimeout != 4*time.Second || cfg.Database.CommitTimeout != 2*time.Second {
		t.Fatal("HTTP source costs must keep normal request4s/commit2s deadlines")
	}
	if copyFrom != "" {
		sourceHTTPCopyFiles(t, copyFrom, cfg.Storage.Dir)
	}
	var actualPool *pgxpool.Pool
	logs := &syncBuffer{}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("source HTTP app logs:\n%s", logs.String())
		}
	})
	a, err := wireApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(logs, nil)), migrations.FS(), testWebUI(),
		wiring{page: func(d *page.Deps) { actualPool = d.Pool }})
	if err != nil {
		t.Fatal(err)
	}
	if actualPool == nil || actualPool != a.pool {
		a.close()
		t.Fatal("HTTP cost did not observe the actual application pool")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done, stopped := make(chan error, 1), make(chan error, 1)
	var once sync.Once
	var inspection *pgxpool.Pool
	stop := func() {
		once.Do(func() {
			cancel()
			if err := <-stopped; err != nil {
				t.Errorf("source HTTP app stop: %v", err)
			}
			a.close()
			if inspection != nil {
				inspection.Close()
			}
		})
	}
	t.Cleanup(stop)
	go func() {
		err := a.run(ctx)
		done <- err
		stopped <- err
	}()
	base := waitHealthy(t, cfg.Server.AddrFile, done)
	inspectionConfig := cfg.Database
	inspectionConfig.URL = sourceHTTPModeURL(t, database, mode)
	inspection, err = postgres.NewPool(t.Context(), inspectionConfig)
	if err != nil {
		t.Fatal(err)
	}
	return sourceHTTPApp{team: acmeTeam{url: database, base: base, storage: cfg.Storage.Dir, pool: inspection,
		contract: apitest.Load(t), tokens: map[string]string{}, members: map[string]uuid.UUID{}},
		actualPool: actualPool, stop: stop, requestTimeout: cfg.Server.RequestTimeout, commitTimeout: cfg.Database.CommitTimeout}
}

func sourceHTTPModeURL(t *testing.T, database, mode string) string {
	t.Helper()
	u, err := url.Parse(database)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("plan_cache_mode", mode)
	u.RawQuery = q.Encode()
	return u.String()
}

func sourceHTTPTables() []string {
	return []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"}
}

func sourceHTTPPrepare(t *testing.T, shape string) sourceHTTPSeed {
	t.Helper()
	seed := sourceHTTPSeed{database: pgtest.NewDatabase(t), shape: shape}
	// Disable automatic analysis before even the LLM Wiki template is seeded.
	// Do not erase statistics: a nonempty inherited pg_stats is a failure.
	setup := connect(t, seed.database)
	for _, table := range sourceHTTPTables() {
		if _, err := setup.Exec(t.Context(), "ALTER TABLE "+table+" SET (autovacuum_enabled=false)"); err != nil {
			t.Fatal(err)
		}
	}
	sourceHTTPCatalog(t, setup, "no_statistics")
	setup.Close()
	a := sourceHTTPStartApp(t, seed.database, "auto", "")
	tm := a.team
	for _, name := range []string{"alice", "bob", "carol", "dana"} {
		tm.tokens[name] = registerAccount(t, tm.contract, tm.base, name+"@example.com").AccessToken
	}
	if status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces", tm.tokens["alice"], `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("seed workspace=%d %s", status, body)
	}
	if err := tm.pool.QueryRow(t.Context(), "SELECT id FROM workspace_members").Scan(&seed.member); err != nil {
		t.Fatal(err)
	}
	tm.members["alice"] = seed.member
	seed.wiki = tm.llmWiki(t, "Source costs")
	sourceCostPages(t, tm, seed.wiki.raw, 100000, true)
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", seed.wiki.id); got != 100000 {
		t.Fatalf("source cost baseline=%d want100000", got)
	}
	switch shape {
	case "create":
	case "move", "restore", "revert":
		seed.root = sourceCostItem(t, tm, seed.wiki)
		if shape != "move" {
			tm.send(t, nodeMove("alice", seed.root, seed.wiki.raw), http.StatusOK)
		}
	case "forget":
		seed.other = tm.llmWiki(t, "Unrelated")
		tm.createPageWith(t, "alice", seed.other.id, seed.other.raw, "Kept", "other source")
	default:
		t.Fatalf("unknown source HTTP cost shape %q", shape)
	}
	sourceCostHealthy(t, tm)
	sourceIndexSettle(t, tm.pool)
	sourceHTTPBoundary(t, a, "no_statistics")
	seed.storage = tm.storage
	a.stop()
	// Fingerprint the completed, stopped seed. This temporary inspection pool
	// is also closed before verifying zero connections and using TEMPLATE.
	finished := connect(t, seed.database)
	sourceHTTPCatalog(t, finished, "no_statistics")
	seed.fingerprint = sourceHTTPFingerprint(t, finished)
	seed.files = sourceHTTPFiles(t, seed.storage)
	wantFiles := 0
	if seed.root != "" {
		wantFiles = 9900
	}
	if seed.files.Count != wantFiles || seed.files.Bytes != int64(wantFiles)*64 {
		t.Fatalf("seed real files=%+v want%d files of64 bytes", seed.files, wantFiles)
	}
	finished.Close()
	sourceIndexDisconnected(t, seed.database)
	sourceHTTPLog(t, "seed", struct {
		Shape string
		Rows  sourceHTTPRows
		Files sourceHTTPFileSummary
	}{shape, seed.fingerprint, seed.files})
	return seed
}

func sourceHTTPClone(t *testing.T, seed sourceHTTPSeed, stage string) sourceHTTPApp {
	t.Helper()
	sourceIndexDisconnected(t, seed.database)
	database := pgtest.NewDatabaseFrom(t, seed.database)
	mode := "auto"
	if stage == "force_generic_setting" {
		mode = "force_generic_plan"
	}
	a := sourceHTTPStartApp(t, database, mode, seed.storage)
	tm := a.team
	if files := sourceHTTPFiles(t, tm.storage); files != seed.files {
		t.Fatalf("clone files=%+v want stopped seed %+v", files, seed.files)
	}
	if rows := sourceHTTPFingerprint(t, tm.pool); rows.SHA256 != seed.fingerprint.SHA256 {
		t.Fatalf("clone initial row fingerprint=%+v want %+v", rows, seed.fingerprint)
	}
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/auth/login", "", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`)
	var tokens authTokens
	if status != http.StatusOK || json.Unmarshal([]byte(body), &tokens) != nil || tokens.AccessToken == "" {
		t.Fatalf("clone login=%d %s", status, body)
	}
	tm.tokens["alice"], tm.members["alice"] = tokens.AccessToken, seed.member
	sourceCostHealthy(t, tm)
	sourceIndexSettle(t, tm.pool)
	if stage != "no_statistics" {
		sourceCostAnalyze(t, tm)
	}
	sourceHTTPBoundary(t, a, stage)
	return a
}

type sourceHTTPTableState struct {
	Name               string
	Rows, Statistics   int
	AutovacuumDisabled bool
}

// Empty asset_blobs has no column statistics even after ANALYZE. Every
// nonempty measured table must have statistics in the analyzed states;
// no_statistics requires zero pg_stats rows on all six, including empties.
func sourceHTTPCatalog(t *testing.T, pool *pgxpool.Pool, stage string) []sourceHTTPTableState {
	t.Helper()
	var result []sourceHTTPTableState
	for _, table := range sourceHTTPTables() {
		s := sourceHTTPTableState{Name: table, Rows: count(t, pool, "SELECT count(*) FROM "+table)}
		err := pool.QueryRow(t.Context(), `SELECT
		 (SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename=$1),
		 COALESCE('autovacuum_enabled=false'=ANY(c.reloptions),false)
		 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		 WHERE n.nspname='public' AND c.relname=$1`, table).Scan(&s.Statistics, &s.AutovacuumDisabled)
		if err != nil {
			t.Fatal(err)
		}
		if !s.AutovacuumDisabled {
			t.Fatalf("source HTTP state %s table %s allows automatic analysis", stage, table)
		}
		if stage == "no_statistics" && s.Statistics != 0 || stage != "no_statistics" && s.Rows > 0 && s.Statistics == 0 {
			t.Fatalf("source HTTP state %s table %s has pg_stats=%d rows=%d", stage, table, s.Statistics, s.Rows)
		}
		result = append(result, s)
	}
	return result
}

type sourceHTTPPoolState struct {
	RuntimeMode string
	MaxConns    int32
	BackendPIDs []uint32
	Modes       []string
}

type sourceHTTPState struct {
	Tables                []sourceHTTPTableState
	Pool                  sourceHTTPPoolState
	InspectionRuntimeMode string
	InspectionMode        string
}

func sourceHTTPBoundary(t *testing.T, a sourceHTTPApp, stage string) sourceHTTPState {
	t.Helper()
	state := sourceHTTPState{Tables: sourceHTTPCatalog(t, a.team.pool, stage)}
	want := "auto"
	if stage == "force_generic_setting" {
		want = "force_generic_plan"
	}
	state.InspectionRuntimeMode = a.team.pool.Config().ConnConfig.RuntimeParams["plan_cache_mode"]
	if err := a.team.pool.QueryRow(t.Context(), "SHOW plan_cache_mode").Scan(&state.InspectionMode); err != nil {
		t.Fatal(err)
	}
	if state.InspectionRuntimeMode != want || state.InspectionMode != want {
		t.Fatalf("source HTTP state %s inspection pool mode=%+v want %s", stage, state, want)
	}
	if a.actualPool == a.team.pool {
		t.Fatal("source HTTP mode proof used the inspection pool")
	}
	cfg := a.actualPool.Config()
	state.Pool.RuntimeMode = cfg.ConnConfig.RuntimeParams["plan_cache_mode"]
	state.Pool.MaxConns = cfg.MaxConns
	if cfg.MaxConns != 4 {
		t.Fatalf("HTTP cost actual pool max_conns=%d want normal4", cfg.MaxConns)
	}
	// Hold all four connections simultaneously, so SHOW cannot repeatedly
	// inspect one good backend while a different app backend has wrong mode.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var held []*pgxpool.Conn
	defer func() {
		for _, conn := range held {
			conn.Release()
		}
	}()
	for range cfg.MaxConns {
		conn, err := a.actualPool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, conn)
		var mode string
		if err := conn.QueryRow(ctx, "SHOW plan_cache_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		state.Pool.BackendPIDs = append(state.Pool.BackendPIDs, conn.Conn().PgConn().PID())
		state.Pool.Modes = append(state.Pool.Modes, mode)
	}
	for _, mode := range state.Pool.Modes {
		if state.Pool.RuntimeMode != want || mode != want {
			t.Fatalf("source HTTP state %s actual application pool mode=%+v want %s", stage, state, want)
		}
	}
	if a.requestTimeout != 4*time.Second || a.commitTimeout != 2*time.Second {
		t.Fatal("HTTP source cost deadline changed")
	}
	return state
}

type sourceHTTPTableDigest struct {
	Name   string
	Rows   int
	SHA256 string
}

type sourceHTTPRows struct {
	Tables []sourceHTTPTableDigest
	SHA256 string
}

// All columns, including UUIDs, bodies, hashes and history timestamps, are
// fingerprinted. Authentication/session and River housekeeping rows are
// excluded because clone login/startup legitimately changes those rows.
func sourceHTTPFingerprint(t *testing.T, pool *pgxpool.Pool) sourceHTTPRows {
	t.Helper()
	var result sourceHTTPRows
	all := sha256.New()
	for _, table := range []struct{ name, order string }{
		{"notebooks", "id"}, {"nodes", "id"}, {"page_contents", "node_id"},
		{"page_revisions", "id"}, {"changesets", "id"},
		{"changeset_items", "id"}, {"asset_blobs", "id"}, {"page_links", "source_id,range_start"},
	} {
		rows, err := pool.Query(t.Context(), "SELECT row_to_json(r)::text FROM "+table.name+" r ORDER BY "+table.order)
		if err != nil {
			t.Fatal(err)
		}
		digest := sourceHTTPTableDigest{Name: table.name}
		hash := sha256.New()
		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			_, _ = fmt.Fprintf(hash, "%d:%s\n", len(row), row)
			digest.Rows++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		digest.SHA256 = hex.EncodeToString(hash.Sum(nil))
		_, _ = fmt.Fprintf(all, "%s:%d:%s\n", digest.Name, digest.Rows, digest.SHA256)
		result.Tables = append(result.Tables, digest)
	}
	result.SHA256 = hex.EncodeToString(all.Sum(nil))
	return result
}

type sourceHTTPFileSummary struct {
	Count  int
	Bytes  int64
	SHA256 string
}

func sourceHTTPFiles(t *testing.T, dir string) sourceHTTPFileSummary {
	t.Helper()
	var result sourceHTTPFileSummary
	hash := sha256.New()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected nonregular storage file %s", path)
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(hash, "%s:%d:%x\n", filepath.ToSlash(relative), len(body), sha256.Sum256(body))
		result.Count++
		result.Bytes += int64(len(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return result
}

func sourceHTTPCopyFiles(t *testing.T, from, to string) {
	t.Helper()
	if filepath.Clean(from) == filepath.Clean(to) {
		t.Fatal("clone storage must be independent of seed storage")
	}
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected nonregular seed file %s", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	if sourceHTTPFiles(t, from) != sourceHTTPFiles(t, to) {
		t.Fatal("clone storage file count/bytes/SHA changed before app startup")
	}
}

type sourceHTTPCounts struct {
	Nodes, Pages, Assets, LiveSourceHashes, DistinctSourceHashes int
	PayloadPages64, GroupNodes, PayloadLinks                     int
}

func sourceHTTPCountsOf(t *testing.T, tm acmeTeam, seed sourceHTTPSeed) sourceHTTPCounts {
	t.Helper()
	var counts sourceHTTPCounts
	var root any
	if seed.root != "" {
		root = seed.root
	}
	err := tm.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER (WHERE kind='page'),
	 count(*) FILTER (WHERE kind='asset'),count(*) FILTER (WHERE source_hash IS NOT NULL AND deleted_at IS NULL),
	 count(DISTINCT (kind,source_hash)) FILTER (WHERE source_hash IS NOT NULL AND deleted_at IS NULL),
	 count(*) FILTER (WHERE id=$2::uuid OR parent_id=$2::uuid)
	 FROM nodes WHERE notebook_id=$1`, seed.wiki.id, root).Scan(&counts.Nodes, &counts.Pages, &counts.Assets,
		&counts.LiveSourceHashes, &counts.DistinctSourceHashes, &counts.GroupNodes)
	if err != nil {
		t.Fatal(err)
	}
	counts.PayloadPages64 = count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes n ON n.id=c.node_id
	 WHERE (n.parent_id=$1 OR n.parent_id=$2::uuid) AND c.byte_size=64 AND octet_length(c.content)=64`, seed.wiki.raw, root)
	counts.PayloadLinks = count(t, tm.pool, `SELECT count(*) FROM page_links l JOIN nodes n ON n.id=l.source_id
	 WHERE n.parent_id=$1 OR n.parent_id=$2::uuid`, seed.wiki.raw, root)
	wantPayload, wantGroup, wantAssets := 100000, 0, 0
	if root != nil {
		wantPayload, wantGroup, wantAssets = 100099, 10000, 9900
	}
	if counts.PayloadPages64 != wantPayload || counts.GroupNodes != wantGroup || counts.Assets != wantAssets || counts.PayloadLinks != 0 {
		t.Fatalf("HTTP source corpus changed: %+v want payload64=%d group=%d assets=%d links0", counts, wantPayload, wantGroup, wantAssets)
	}
	if counts.LiveSourceHashes != counts.DistinctSourceHashes {
		t.Fatalf("cost corpus has duplicate live source hashes: %+v", counts)
	}
	return counts
}

type sourceHTTPRequestSample struct {
	Shape, Stage, InitialRowSHA256 string
	Sample                         int
	InitialFiles                   sourceHTTPFileSummary
	Before                         sourceHTTPCounts
	State                          sourceHTTPState
	ElapsedNS, BudgetNS            int64
	RequestTimeout, CommitTimeout  time.Duration
	SuccessfulStatus               int
	ResultID, ChangesetID          string
}

func sourceHTTPSample(t *testing.T, a sourceHTTPApp, seed sourceHTTPSeed, stage string, sample int) time.Duration {
	t.Helper()
	tm := a.team
	var command step
	var content string
	want, budget := http.StatusOK, sourceCreateTime
	switch seed.shape {
	case "create":
		content = fmt.Sprintf("new-source-cost-%d", sample)
		command = sourceCostCreate(t, tm, seed.wiki.id, seed.wiki.raw, fmt.Sprintf("New%d", sample), content)
		want = http.StatusCreated
	case "move":
		command, budget = nodeMove("alice", seed.root, seed.wiki.raw), sourceMoveTime
	case "restore":
		tm.send(t, nodeDeletion("alice", seed.root), http.StatusNoContent)
		command, budget = request("alice", http.MethodPost, "/api/v0/nodes/"+seed.root+"/restore", ""), sourceRestoreTime
	case "revert":
		tm.send(t, nodeDeletion("alice", seed.root), http.StatusNoContent)
		command, budget = changesetRevert("alice", tm.newest(t, seed.wiki.id).ID), sourceRevertTime
	case "forget":
		command, budget = request("alice", http.MethodPost, "/api/v0/notebooks/"+seed.wiki.id+"/downgrade", ""), sourceForgetTime
	default:
		t.Fatalf("unknown HTTP sample shape %q", seed.shape)
	}
	// State is checked after all fixture/reset HTTP work, immediately before
	// the measured request. Neither observation below is inside its timer.
	e := sourceHTTPRequestSample{Shape: seed.shape, Stage: stage, Sample: sample,
		InitialRowSHA256: seed.fingerprint.SHA256, InitialFiles: seed.files,
		Before: sourceHTTPCountsOf(t, tm, seed), State: sourceHTTPBoundary(t, a, stage),
		BudgetNS: budget.Nanoseconds(), RequestTimeout: a.requestTimeout, CommitTimeout: a.commitTimeout}
	label := fmt.Sprintf("%s/%s/sample%d", seed.shape, stage, sample)
	body, took := sourceCostTimed(t, tm, label, command, want)
	e.ElapsedNS, e.SuccessfulStatus = took.Nanoseconds(), want
	switch seed.shape {
	case "create":
		e.ResultID = idOf(t, answer{body: body})
		if count(t, tm.pool, "SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.id=$1 AND n.source_hash=sha256(convert_to($2,'UTF8')) AND c.content=$2", e.ResultID, content) != 1 {
			t.Fatal("create did not commit exact body/hash")
		}
	case "move", "restore", "revert":
		if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE (id=$1 OR parent_id=$1) AND source_hash IS NOT NULL AND deleted_at IS NULL", seed.root); got != 9999 {
			t.Fatalf("%s live group hashes=%d want9999", seed.shape, got)
		}
		if seed.shape == "revert" {
			var result revertAnswer
			decodeAnswer(t, body, &result)
			if !result.Changed || result.ChangesetID == nil {
				t.Fatalf("revert not changed: %s", body)
			}
			e.ChangesetID = *result.ChangesetID
		}
	case "forget":
		if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", seed.wiki.id) != 0 {
			t.Fatal("downgrade retained hashes")
		}
		if count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id=$1 AND mode='standard' AND source_root_id IS NULL", seed.wiki.id) != 1 {
			t.Fatal("downgrade mode/root not committed")
		}
		if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", seed.other.id) != 1 {
			t.Fatal("downgrade cleared unrelated source")
		}
	}
	sourceCostHealthy(t, tm)
	sourceHTTPLog(t, "sample", e)
	if seed.shape != "create" {
		sourceCostWithin(t, label, took, budget)
	}
	if seed.shape == "move" {
		tm.send(t, nodeMove("alice", seed.root, ""), http.StatusOK)
	}
	return took
}

func sourceHTTPLog(t *testing.T, kind string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("source HTTP %s %s", kind, encoded)
}
