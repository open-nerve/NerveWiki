//go:build !race

package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type sourcePrecheckRead struct {
	Method                        string
	Notebook                      uuid.UUID
	IDs                           []uuid.UUID
	Heads                         []app.SourceValue
	Probes                        []app.SourceProbe
	Matches                       []app.SourceMatch
	Backend                       int
	Snapshot, Isolation, ReadOnly string
	SQL                           string
	Arguments                     []any
	AdapterElapsed                time.Duration
	Work                          map[string]sourceRelationWork
}

// This decorator never supplies a match or a classification. Every metadata
// and probe result comes from Store using Check's actual read-only transaction.
// The per-query counters are backend-local; heads and probes stay separate.
type sourcePrecheckMeter struct {
	app.SourceReads
	t     *testing.T
	reads []sourcePrecheckRead
}

func (m *sourcePrecheckMeter) observe(ctx context.Context, r *sourcePrecheckRead) pgx.Tx {
	m.t.Helper()
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		m.t.Fatal("source precheck read escaped its transaction")
	}
	err := tx.QueryRow(ctx, `SELECT pg_backend_pid(),pg_current_snapshot()::text,
	 current_setting('transaction_isolation'),current_setting('transaction_read_only')`).Scan(
		&r.Backend, &r.Snapshot, &r.Isolation, &r.ReadOnly)
	if err != nil {
		m.t.Fatal(err)
	}
	return tx
}

func (m *sourcePrecheckMeter) SourceHeads(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.SourceValue, error) {
	r := sourcePrecheckRead{Method: "SourceHeads", Notebook: nb, IDs: slices.Clone(ids)}
	tx := m.observe(ctx, &r)
	s := statementOf(m.t, func(q *gen.Queries) error {
		_, err := q.SourceHeads(ctx, gen.SourceHeadsParams{NotebookID: nb, NodeIds: ids})
		return err
	})
	r.SQL, r.Arguments = s.sql, s.args
	before := sourceWork(m.t, tx)
	started := time.Now()
	got, err := m.SourceReads.SourceHeads(ctx, nb, ids)
	r.AdapterElapsed = time.Since(started)
	r.Work = sourceWorkDelta(before, sourceWork(m.t, tx))
	for _, id := range ids {
		if h, ok := got[id]; ok {
			r.Heads = append(r.Heads, h)
		}
	}
	m.reads = append(m.reads, r)
	return got, err
}

func sourceCostProbeParams(nb uuid.UUID, probes []app.SourceProbe) gen.ProbeSourcesParams {
	p := gen.ProbeSourcesParams{NotebookID: nb}
	for _, probe := range probes {
		cursor := uuid.Nil()
		if probe.After != nil {
			cursor = *probe.After
		}
		p.Kinds = append(p.Kinds, string(probe.Key.Kind))
		p.Hashes = append(p.Hashes, probe.Key.Hash[:])
		p.AfterIds = append(p.AfterIds, cursor)
		p.HasAfters = append(p.HasAfters, probe.After != nil)
		p.Takes = append(p.Takes, int32(probe.Take))
	}
	return p
}

func (m *sourcePrecheckMeter) ProbeSources(ctx context.Context, nb uuid.UUID, probes []app.SourceProbe) ([]app.SourceMatch, error) {
	r := sourcePrecheckRead{Method: "ProbeSources", Notebook: nb, Probes: slices.Clone(probes)}
	for i := range r.Probes {
		if after := r.Probes[i].After; after != nil {
			copied := *after
			r.Probes[i].After = &copied
		}
	}
	tx := m.observe(ctx, &r)
	s := statementOf(m.t, func(q *gen.Queries) error {
		_, err := q.ProbeSources(ctx, sourceCostProbeParams(nb, probes))
		return err
	})
	r.SQL, r.Arguments = s.sql, s.args
	before := sourceWork(m.t, tx)
	started := time.Now()
	got, err := m.SourceReads.ProbeSources(ctx, nb, probes)
	r.AdapterElapsed = time.Since(started)
	r.Work = sourceWorkDelta(before, sourceWork(m.t, tx))
	r.Matches = slices.Clone(got)
	m.reads = append(m.reads, r)
	return got, err
}

type sourceCostNoAttachments struct{ calls int }

func (a *sourceCostNoAttachments) Of(context.Context, []uuid.UUID) (map[uuid.UUID]app.AttachmentInfo, error) {
	a.calls++
	return nil, errors.New("page-only source precheck read attachment metadata")
}

type sourcePrecheckScene struct {
	f              fixture
	shape          string
	raw, leaf      uuid.UUID
	ids            []uuid.UUID
	hash           []byte
	witness        *uuid.UUID
	facts          []domain.StepFacts
	keyCardinality int
}

func newSourcePrecheckScene(t *testing.T, x int, shape string) sourcePrecheckScene {
	t.Helper()
	f := newFixture(t)
	f.exec(t, "ALTER TABLE page_contents SET (autovacuum_enabled=false)")
	f.sourceProbeCorpus(t)
	s := sourcePrecheckScene{f: f, shape: shape, hash: sourceSum("converged bytes"), keyCardinality: 100000}
	if err := f.pool.QueryRow(t.Context(), "SELECT source_root_id FROM notebooks WHERE id=$1", f.eng).Scan(&s.raw); err != nil {
		t.Fatal(err)
	}
	start := 1
	switch shape {
	case "first":
		start = 2
		id := sourcePlanID(1, 0, 1)
		s.witness = &id
	case "last_in_window":
		id := sourcePlanID(1, 0, x+1)
		s.witness = &id
	case "absent":
		// X cannot exhaust a 100k key when X is 3 or 60k. Preserve both
		// original H groups and add a distinct K with exactly X own pages.
		start, s.keyCardinality = 100001, x
		const body = "isolated key bytes"
		s.hash = sourceSum(body)
		for first := start; first < start+x; first += domain.MaxUnitNodes {
			f.exec(t, `INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,
			 created_by_id,updated_by_id,created_at,updated_at)
			 SELECT ('70000000-0000-7000-8000-000100'||lpad(to_hex(k),6,'0'))::uuid,
			 $1,$2,'page','Isolated'||k,'isolated'||k,k,$3,$4,$4,$7,$7
			 FROM generate_series($5::int,$6::int) k`, f.eng, s.raw, s.hash, f.alice, first, min(start+x-1, first+domain.MaxUnitNodes-1), now())
		}
		f.exec(t, `INSERT INTO page_contents (node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at)
		 SELECT id,1,$3,$2,$4,$5,$6 FROM nodes
		 WHERE notebook_id=$1 AND kind='page' AND source_hash=$2`, f.eng, s.hash, body, len(body), f.alice, now())
		// Foreign and different-kind K candidates make an absent result
		// depend on the query's actual namespace, not an empty database.
		var foreignRaw uuid.UUID
		if err := f.pool.QueryRow(t.Context(), "SELECT source_root_id FROM notebooks WHERE id=$1", f.ops).Scan(&foreignRaw); err != nil {
			t.Fatal(err)
		}
		foreign := f.page(t, f.ops, &foreignRaw, "IsolatedForeign", -4)
		f.exec(t, "UPDATE nodes SET source_hash=$2 WHERE id=$1", foreign.ID, s.hash)
		f.exec(t, "UPDATE page_contents SET content=$2,content_hash=$3,byte_size=$4 WHERE node_id=$1", foreign.ID, body, s.hash, len(body))
		asset := f.asset(t, f.eng, &s.raw, "isolated.bin", -4)
		f.exec(t, "UPDATE nodes SET source_hash=$2 WHERE id=$1", asset.ID, s.hash)
		f.exec(t, `INSERT INTO asset_blobs (id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
		 VALUES ($1,$2,$3,'application/octet-stream',$4,$5,$6,$7)`, uuid.NewV7(), asset.ID, f.eng, len(body), s.hash, f.alice, now())
	default:
		t.Fatalf("unknown source precheck shape %q", shape)
	}
	for ordinal := start; ordinal < start+x; ordinal++ {
		s.ids = append(s.ids, sourcePlanID(1, 0, ordinal))
	}
	root := s.ids[0]
	s.leaf = s.ids[len(s.ids)-1]
	// Make a real root-first subtree. The facts below are read from this
	// graph, rather than treating the corpus's original siblings as children.
	for chunk := range slices.Chunk(s.ids[1:], domain.MaxUnitNodes) {
		f.exec(t, "UPDATE nodes SET parent_id=$2 WHERE notebook_id=$1 AND id=ANY($3::uuid[])", f.eng, root, chunk)
	}
	var out domain.StepFacts
	out.Operation = domain.OpMove
	for chunk := range slices.Chunk(s.ids, domain.MaxUnitNodes) {
		rows, err := f.pool.Query(t.Context(), "SELECT id,parent_id,kind,name_key FROM nodes WHERE notebook_id=$1 AND id=ANY($2::uuid[]) ORDER BY id", f.eng, chunk)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var n domain.NodeFacts
			var before domain.NodePlace
			if err := rows.Scan(&n.ID, &before.ParentID, &n.Kind, &before.Key); err != nil {
				t.Fatal(err)
			}
			want := root
			if n.ID == root {
				want = s.raw
			}
			if before.ParentID == nil || *before.ParentID != want || n.Kind != domain.KindPage {
				t.Fatalf("invalid source subtree node %+v at %+v", n, before)
			}
			after := before
			if n.ID == root {
				after.ParentID = nil
			}
			n.Before, n.After = &before, &after
			out.Nodes = append(out.Nodes, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if len(out.Nodes) != x || f.count(t, "SELECT count(*) FROM nodes WHERE parent_id=$1 AND deleted_at IS NULL", root) != x-1 {
		t.Fatal("source fixture does not contain its complete X-node subtree")
	}
	out = domain.SourceFacts(out, s.raw, []domain.Ancestor{{ID: s.raw}}, nil, nil)
	leaf := out.Nodes[len(out.Nodes)-1]
	back := domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: leaf.ID, Kind: leaf.Kind,
		Before: &domain.NodePlace{ParentID: &root, Key: leaf.Before.Key},
		After:  &domain.NodePlace{ParentID: &s.raw, Key: leaf.Before.Key}}}}
	back = domain.SourceFacts(back, s.raw, []domain.Ancestor{{ID: root}}, []domain.Ancestor{{ID: s.raw}}, nil)
	for _, n := range out.Nodes {
		if !n.Before.InArea || n.After.InArea {
			t.Fatalf("invalid source-out membership for %s", n.ID)
		}
	}
	if back.Nodes[0].Before.InArea || !back.Nodes[0].After.InArea {
		t.Fatal("returning leaf must enter from the now-outside subtree")
	}
	s.facts = []domain.StepFacts{out, back}
	for _, step := range s.facts {
		if verdict := domain.CheckContract(domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: s.raw}, step); verdict.Err != nil {
			t.Fatalf("unreachable source cost facts: %v", verdict.Err)
		}
	}
	for _, item := range []struct {
		nb   uuid.UUID
		want int
	}{{f.eng, 100000}, {f.ops, 200000}} {
		if n := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND kind='page' AND source_hash=$2 AND deleted_at IS NULL", item.nb, sourceSum("converged bytes")); n != item.want {
			t.Fatalf("preserved converged H pages=%d want%d", n, item.want)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND kind='page' AND source_hash=$2 AND deleted_at IS NULL", f.eng, s.hash); n != s.keyCardinality {
		t.Fatalf("probed key cardinality=%d want%d", n, s.keyCardinality)
	}
	// Parent updates create obsolete index entries. Remove those setup
	// versions before measuring accesses; this is not a cold-cache claim.
	// VACUUM supplies relation sizes but must not create column statistics.
	f.exec(t, "VACUUM (ANALYZE false) nodes,page_contents")
	if n := f.count(t, "SELECT count(*) FROM pg_stats WHERE tablename IN ('nodes','page_contents')"); n != 0 {
		t.Fatalf("no-statistics fixture has %d analyzed columns", n)
	}
	return s
}

func (s sourcePrecheckScene) sequence() iter.Seq[domain.StepFacts] {
	return slices.Values(s.facts)
}

func (s sourcePrecheckScene) checkReadWork(t *testing.T, reads []sourcePrecheckRead) (int, int) {
	t.Helper()
	seen := make(map[uuid.UUID]bool, len(s.ids))
	var previous *uuid.UUID
	probes, returned, takes, heads := 0, 0, 0, 0
	for _, r := range reads {
		if r.Backend != reads[0].Backend || r.Snapshot != reads[0].Snapshot || r.Isolation != "repeatable read" || r.ReadOnly != "on" || r.Notebook != s.f.eng {
			t.Fatalf("source reads did not share the same read-only snapshot: %+v", r)
		}
		switch r.Method {
		case "SourceHeads":
			heads++
			if len(r.IDs) > domain.MaxUnitNodes || len(r.Heads) != len(r.IDs) {
				t.Fatalf("metadata request/result count=%d/%d", len(r.IDs), len(r.Heads))
			}
			for i, h := range r.Heads {
				if h.ID != r.IDs[i] || seen[h.ID] || h.Kind != domain.KindPage || h.Bytes <= 0 || !bytes.Equal(h.BeforeSourceHash, s.hash) || !bytes.Equal(h.Hash, s.hash) {
					t.Fatalf("invalid or repeated initial eligible metadata %+v", h)
				}
				seen[h.ID] = true
			}
		case "ProbeSources":
			probes++
			if len(r.Probes) != 1 {
				t.Fatalf("single admitted key produced %d probes", len(r.Probes))
			}
			p := r.Probes[0]
			if p.Key.Kind != domain.KindPage || p.Key.Hash != [32]byte(s.hash) || p.Take < 1 || p.Take > domain.MaxUnitNodes || len(r.Matches) > p.Take {
				t.Fatalf("invalid bounded probe %+v with %d matches", p, len(r.Matches))
			}
			if (p.After == nil) != (previous == nil) || p.After != nil && *p.After != *previous {
				t.Fatal("probe restarted or did not use the preceding actual last ID")
			}
			wantTake := 1
			if probes > 1 {
				wantTake = min(len(s.ids)-returned+1, domain.MaxUnitNodes)
			}
			if p.Take != wantTake {
				t.Errorf("probe %d take=%d want remaining E+1 bound %d", probes, p.Take, wantTake)
			}
			assertSourceProbeWork(t, fmt.Sprintf("%s/probe%d", s.shape, probes), r.Work, len(r.Matches), p.After != nil)
			for _, match := range r.Matches {
				if match.Probe != 0 || previous != nil && match.ID.Compare(*previous) <= 0 {
					t.Fatalf("invalid ordinal or cursor result %+v", match)
				}
				ordinal := returned + 1
				if s.shape == "absent" {
					ordinal += 100000
				}
				if match.ID != sourcePlanID(1, 0, ordinal) {
					t.Fatalf("source result %d=%s want ordinal %d", returned, match.ID, ordinal)
				}
				id := match.ID
				previous = &id
				returned++
			}
			takes += p.Take
		default:
			t.Fatalf("unrecognized source read %s", r.Method)
		}
	}
	if heads != (len(s.ids)+domain.MaxUnitNodes-1)/domain.MaxUnitNodes || len(seen) != len(s.ids) {
		t.Fatalf("whole-X metadata coverage=%d calls/%d IDs want%d IDs", heads, len(seen), len(s.ids))
	}
	for _, id := range s.ids {
		if !seen[id] {
			t.Fatalf("whole-X metadata missed %s", id)
		}
	}
	wantRows, wantProbes := len(s.ids)+1, 1+(len(s.ids)+domain.MaxUnitNodes-1)/domain.MaxUnitNodes
	switch s.shape {
	case "first":
		wantRows, wantProbes = 1, 1
	case "absent":
		wantRows--
	}
	if returned != wantRows || probes != wantProbes || returned > len(s.ids)+1 || takes > len(s.ids)+1 {
		t.Fatalf("actual probes=%d rows=%d takes=%d; want%d/%d and E+1=%d", probes, returned, takes, wantProbes, wantRows, len(s.ids)+1)
	}
	return probes, returned
}

type sourcePrecheckCostEvidence struct {
	Shape, Stage, PlanCacheMode, Execution string
	Outcome, ProblemCode                   string
	Sample, X, E, KeyCardinality           int
	ProbeCalls, Returned, AdminAsks        int
	Elapsed                                time.Duration
	AllocatedBytes                         uint64
	Witness, ActualDuplicateOf             *uuid.UUID
	Reads                                  []sourcePrecheckRead
}

// Explicit evidence captures preserve every raw ID, head and match. Normal
// CI output is a compact projection: all assertions below still use the full
// results, while each full read is bound by its SHA256 outside the timer.
func sourcePrecheckEvidence(t *testing.T, e sourcePrecheckCostEvidence) {
	t.Helper()
	if os.Getenv("NERVEWIKI_SOURCE_RANGE_EVIDENCE") != "" {
		sourceEvidence(t, "source-precheck-cost", e)
		return
	}
	type projectedRead struct {
		Read                sourcePrecheckRead
		IDs, Heads, Matches int
		FirstID, LastID     *uuid.UUID
		FullReadSHA256      string
	}
	var projected []projectedRead
	for _, r := range e.Reads {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		v := projectedRead{Read: r, IDs: len(r.IDs), Heads: len(r.Heads), Matches: len(r.Matches), FullReadSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
		if len(r.IDs) > 0 {
			v.FirstID, v.LastID = &r.IDs[0], &r.IDs[len(r.IDs)-1]
		} else if len(r.Matches) > 0 {
			v.FirstID, v.LastID = &r.Matches[0].ID, &r.Matches[len(r.Matches)-1].ID
		}
		v.Read.IDs, v.Read.Heads, v.Read.Matches, v.Read.Arguments = nil, nil, nil, nil
		projected = append(projected, v)
	}
	e.Reads = nil
	sourceEvidence(t, "source-precheck-cost-summary", struct {
		Measurement sourcePrecheckCostEvidence
		Reads       []projectedRead
	}{e, projected})
}

// This is the complete SourcePrecheck.Check, not a port query starting at
// cursor 60000. E is genuinely X: move the entire X-node subtree out, then
// bring one of its leaves back. Only the latter admits a key. Last means
// external at X+1 in the required window; absent uses a distinct exhaustive
// K while preserving the converged 100k/foreign200k H corpus.
//
// Each of 6 shapes has 3 settings and 3 measured calls (54 calls total).
// Instrumented Check elapsed includes counter collection; per-adapter times
// are also recorded. Measured full Check medians were at most2.906ms/327.599ms for X3/60000.
// The bounds round up three times these medians, with the existing40ms
// database-read floor for the small case. Adapter times remain separate.
func TestSourcePrecheckConvergedPlansCostTheirActualExclusions(t *testing.T) {
	for _, x := range []int{3, 60000} {
		for _, shape := range []string{"first", "last_in_window", "absent"} {
			t.Run(fmt.Sprintf("x%d/%s", x, shape), func(t *testing.T) {
				s := newSourcePrecheckScene(t, x, shape)
				conn, err := s.f.pool.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Release()
				ctx := postgres.OnConn(t.Context(), conn)
				for _, stage := range []string{"no_statistics", "analyzed", "force_generic_setting"} {
					if stage == "analyzed" {
						s.f.exec(t, "ANALYZE nodes,page_contents")
					}
					stats := s.f.count(t, "SELECT count(*) FROM pg_stats WHERE tablename IN ('nodes','page_contents')")
					if (stage == "no_statistics") != (stats == 0) {
						t.Fatalf("%s actual statistics columns=%d", stage, stats)
					}
					mode := "auto"
					if stage == "force_generic_setting" {
						mode = "force_generic_plan"
					}
					if _, err := conn.Exec(ctx, "SELECT set_config('plan_cache_mode',$1,false)", mode); err != nil {
						t.Fatal(err)
					}
					var actualMode string
					if err := conn.QueryRow(ctx, "SHOW plan_cache_mode").Scan(&actualMode); err != nil || actualMode != mode {
						t.Fatalf("actual plan_cache_mode=%s want%s: %v", actualMode, mode, err)
					}
					t.Run(stage, func(t *testing.T) {
						var firstReads []sourcePrecheckRead
						var samples []time.Duration
						for sample := 1; sample <= 3; sample++ {
							meter := &sourcePrecheckMeter{SourceReads: s.f.s, t: t}
							attachments := &sourceCostNoAttachments{}
							check := app.NewSourcePrecheck(app.SourcePrecheckDeps{
								Snapshots: postgres.NewTxManager(s.f.pool, time.Second), Sources: meter, Attachments: attachments})
							asks := 0
							var before, after runtime.MemStats
							runtime.ReadMemStats(&before)
							started := time.Now()
							err := check.Check(ctx, s.f.eng, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: s.raw}, s.sequence(), func(sctx context.Context, action shared.Action) error {
								asks++
								if !postgres.InTx(sctx) || action != domain.ActionManageSource {
									return fmt.Errorf("source permission outside snapshot or unexpected action %s", action)
								}
								return nil
							})
							elapsed := time.Since(started)
							runtime.ReadMemStats(&after)
							probes, rows := 0, 0
							for _, r := range meter.reads {
								if r.Method == "ProbeSources" {
									probes++
									rows += len(r.Matches)
								}
							}
							e := sourcePrecheckCostEvidence{Shape: shape, Stage: stage, PlanCacheMode: actualMode, Execution: "production Planned; generic setting is not prepared-generic proof", Outcome: "allow",
								Sample: sample, X: x, E: x, KeyCardinality: s.keyCardinality, ProbeCalls: probes, Returned: rows, AdminAsks: asks,
								Elapsed: elapsed, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Witness: s.witness, Reads: meter.reads}
							if err != nil {
								e.Outcome = err.Error()
							}
							var actualProblem *shared.Error
							if errors.As(err, &actualProblem) {
								e.ProblemCode, e.ActualDuplicateOf = actualProblem.Code, actualProblem.DuplicateOf
							}
							// Keep the raw actual call even when an oracle below fails.
							sourcePrecheckEvidence(t, e)
							if s.witness == nil {
								if err != nil {
									t.Fatalf("exhausted K should allow the returning leaf: %v", err)
								}
							} else {
								var problem *shared.Error
								var refusal *domain.Refused
								if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" || problem.DuplicateOf == nil || *problem.DuplicateOf != *s.witness || !errors.As(err, &refusal) || refusal.NodeID != s.leaf {
									t.Fatalf("duplicate=%v target=%v want target%s on returning leaf%s", err, problem, *s.witness, s.leaf)
								}
							}
							if asks != 1 || attachments.calls != 0 {
								t.Fatalf("source-out admin asks=%d assets=%d want1/0", asks, attachments.calls)
							}
							s.checkReadWork(t, meter.reads)
							samples = append(samples, elapsed)
							if sample == 1 {
								firstReads = meter.reads
							}
						}
						// These independent executions use the exact observed probe
						// parameters, outside the Check timings/counters above. The
						// existing helper verifies actual PREPARE generic counts,
						// full EXPLAIN JSON and physical accesses, without changing
						// Planned in the real application path.
						for i, r := range firstReads {
							if r.Method != "ProbeSources" {
								continue
							}
							want := make([]uuid.UUID, len(r.Matches))
							for j, match := range r.Matches {
								want[j] = match.ID
							}
							s.f.sourceProbeRange(t, fmt.Sprintf("whole-X%d/%s/%s/read%d", x, shape, stage, i), sourceCostProbeParams(s.f.eng, r.Probes), stage == "force_generic_setting", want)
						}
						ordered := slices.Clone(samples)
						slices.Sort(ordered)
						bound := 40 * time.Millisecond
						if x == 60000 {
							bound = time.Second
						}
						t.Logf("whole-X=%d shape=%s stage=%s instrumented Check samples=%v median=%s bound=%s", x, shape, stage, samples, ordered[1], bound)
						if ordered[1] > bound {
							t.Errorf("whole-X%d %s/%s median %s exceeds %s", x, shape, stage, ordered[1], bound)
						}
					})
				}
			})
		}
	}
}
