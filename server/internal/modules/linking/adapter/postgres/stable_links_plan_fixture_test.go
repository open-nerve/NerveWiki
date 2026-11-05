//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This fixture is an adapter observation, not bootstrap preparation capacity.
// Expected facts come from literal token positions, never from retained SQL.
type stablePlanRow struct {
	Source, Notebook                    uuid.UUID
	Start, End                          int32
	Kind, Target                        string
	Property, Anchor, Display, Key, Alt pgtype.Text
	Resolved                            pgtype.UUID
	Ambiguous, Aliases, Asset           bool
}

type stablePlanSpec struct {
	ID                            uuid.UUID
	Name, Shape, OldText, NewText string
	OldFacts, NewFacts            domain.Facts
	Old, Deleted, Written, Fresh  []stablePlanRow
	Removed, ChangedOld, ToWrite  []stablePlanRow
	N, M, S, C, I, Z              int
}

func stablePlanID(n uint64) uuid.UUID     { id := lintID(n); id[0] = 0x40; return id }
func stablePlanText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func stablePlanTarget(s string) pgtype.UUID {
	for i, key := range []string{"a", "b", "c", "d"} {
		if s == key {
			return pgtype.UUID{Bytes: stablePlanID(uint64(100 + i)), Valid: true}
		}
	}
	return pgtype.UUID{}
}

func stablePlanLong(salt string) string {
	var out strings.Builder
	for i := 0; i < 128; i++ {
		fmt.Fprintf(&out, "%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%d", salt, i))))
	}
	return out.String() // 8192 incompressible ASCII bytes; only two link positions use it.
}

func stablePlanHeader(name string, many, toast bool) (string, domain.Facts) {
	f := domain.Facts{FrontmatterValid: true}
	n := 2
	if many {
		n = 1000
	}
	var tags, aliases []string
	for i := 0; i < n; i++ {
		tag, alias := fmt.Sprintf("%st%04d", name, i), fmt.Sprintf("%sal%04d", name, i)
		tags, aliases = append(tags, tag), append(aliases, alias)
		f.Tags = append(f.Tags, domain.Tag{Key: tag, Name: tag, Count: 1})
		f.Aliases = append(f.Aliases, domain.Alias{Key: alias, Name: alias})
	}
	var out strings.Builder
	out.WriteString("---\n")
	for _, p := range []struct {
		key    string
		values []string
	}{{"tags", tags}, {"aliases", aliases}} {
		value, _ := json.Marshal(p.values)
		fmt.Fprintf(&out, "%s: %s\n", p.key, value)
		f.Properties = append(f.Properties, domain.Property{Key: p.key, Value: value})
	}
	properties := 2
	if many {
		properties = 998
	} // Exactly 1000 properties including tags and aliases.
	for i := 0; i < properties; i++ {
		key, value := fmt.Sprintf("p%04d", i), fmt.Sprintf("value%04d", i)
		if toast && i == 0 {
			value = stablePlanLong(name + "property")
		}
		encoded, _ := json.Marshal(value)
		fmt.Fprintf(&out, "%s: %s\n", key, encoded)
		f.Properties = append(f.Properties, domain.Property{Key: key, Value: encoded})
	}
	out.WriteString("---\n")
	return out.String(), f
}

func stablePlanSpecOf(name, shape string, id uuid.UUID, n int, toast bool) stablePlanSpec {
	s := stablePlanSpec{ID: id, Name: name, Shape: shape, N: n}
	header, facts := stablePlanHeader(name, shape == "noise", toast)
	var old, next strings.Builder
	old.WriteString(header)
	next.WriteString(header)
	s.OldFacts, s.NewFacts = facts, facts
	appendLink := func(b *strings.Builder, target, display string) domain.Link {
		start := b.Len() + 2
		fmt.Fprintf(b, "[[%s", target)
		if display != "" {
			fmt.Fprintf(b, "|%s", display)
		}
		b.WriteString("]]\n")
		return domain.Link{Kind: "wikilink", Target: target, Display: display, Start: start, End: start + len(target)}
	}
	row := func(l domain.Link) stablePlanRow {
		return stablePlanRow{Source: id, Notebook: stablePlanID(900001), Start: int32(l.Start), End: int32(l.End), Kind: "wikilink", Target: l.Target, Display: stablePlanText(l.Display), Key: stablePlanText(l.Target)}
	}
	for i := 0; i < n; i++ {
		target := []string{"a", "b", "c"}[i%3]
		if shape == "mixed" {
			target = "a"
			if i == 2 {
				target = "u"
			}
			if i == n-3 {
				target = "b"
			}
			if i >= n-2 {
				target = "c"
			}
		}
		if shape == "noise" {
			target = "noise"
		}
		display := ""
		if toast && (i == 0 || i == n-2) {
			display = stablePlanLong(fmt.Sprintf("%s/%d", name, i))
		}
		at := old.Len()
		l := appendLink(&old, target, display)
		s.OldFacts.Links = append(s.OldFacts.Links, l)
		r := row(l)
		r.Resolved = stablePlanTarget(target)
		if shape == "distinct_d" {
			r.Resolved = pgtype.UUID{Bytes: stablePlanID(uint64(1000000 + i)), Valid: true}
		}
		s.Old = append(s.Old, r)
		remove := shape == "one_to_empty" || (shape == "mixed" && i >= n-2)
		if remove {
			s.Removed = append(s.Removed, r)
			next.WriteString(strings.Repeat(" ", old.Len()-at-1) + "\n")
			s.Z++
			continue
		}
		s.Deleted = append(s.Deleted, r)
		changed := shape == "changed" || (shape == "mixed" && i == n-3)
		newTarget := target
		if changed {
			s.ChangedOld = append(s.ChangedOld, r)
			newTarget = "d"
			s.C++
		} else {
			s.S++
		}
		nl := appendLink(&next, newTarget, display)
		s.NewFacts.Links = append(s.NewFacts.Links, nl)
		nr := row(nl)
		if !changed {
			nr.Resolved, nr.Ambiguous, nr.Asset = r.Resolved, r.Ambiguous, r.Asset
		}
		s.Written = append(s.Written, nr)
		if changed {
			s.ToWrite = append(s.ToWrite, nr)
		}
		fr := nr
		fr.Resolved = stablePlanTarget(newTarget)
		s.Fresh = append(s.Fresh, fr)
	}
	newCount := 0
	if shape == "mixed" {
		newCount = 2
	}
	if shape == "empty_to_one" {
		newCount = 1
	}
	for i := 0; i < newCount; i++ {
		l := appendLink(&next, "d", "")
		s.NewFacts.Links = append(s.NewFacts.Links, l)
		r := row(l)
		s.Written = append(s.Written, r)
		s.ToWrite = append(s.ToWrite, r)
		r.Resolved = stablePlanTarget("d")
		s.Fresh = append(s.Fresh, r)
		s.I++
	}
	s.M = len(s.Written)
	s.OldText, s.NewText = old.String(), next.String()
	return s
}

// This small cross-module fixture keeps the real parser, Index, Store and Tx.
// Notebook metadata/path inputs are fixed flat pages, not production hooks.
type stablePlanInputs struct {
	notebook        uuid.UUID
	specs           []stablePlanSpec
	nodes           map[uuid.UUID]domain.Node
	byKey           map[string][]domain.Node
	locked, rekeyed int
	events          []app.LinksChanged
}

func (p *stablePlanInputs) PageIDs(_ context.Context, nb uuid.UUID) ([]uuid.UUID, error) {
	if nb != p.notebook {
		return nil, fmt.Errorf("fixture notebook %s", nb)
	}
	out := make([]uuid.UUID, len(p.specs))
	for i, s := range p.specs {
		out[i] = s.ID
	}
	return out, nil
}
func (p *stablePlanInputs) Content(_ context.Context, id uuid.UUID) (string, int, bool, error) {
	for _, s := range p.specs {
		if s.ID == id {
			return s.OldText, 1, true, nil
		}
	}
	return "", 0, false, nil
}
func (p *stablePlanInputs) Rekey(_ context.Context, nb uuid.UUID) ([]app.Clash, error) {
	if nb != p.notebook {
		return nil, fmt.Errorf("fixture rekey notebook %s", nb)
	}
	p.rekeyed++
	return nil, nil
}
func (p *stablePlanInputs) WorkspaceOf(_ context.Context, nb uuid.UUID) (uuid.UUID, bool, error) {
	return stablePlanID(900003), nb == p.notebook, nil
}
func (p *stablePlanInputs) LockByID(_ context.Context, nb uuid.UUID) (bool, error) {
	if nb != p.notebook {
		return false, nil
	}
	p.locked++
	return true, nil
}
func (p *stablePlanInputs) ByKeys(_ context.Context, nb uuid.UUID, keys []string) ([]domain.Node, error) {
	if nb != p.notebook {
		return nil, fmt.Errorf("fixture keys notebook %s", nb)
	}
	var out []domain.Node
	for _, key := range keys {
		out = append(out, p.byKey[key]...)
	}
	return out, nil
}
func (p *stablePlanInputs) Paths(_ context.Context, nb uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	if nb != p.notebook {
		return nil, fmt.Errorf("fixture paths notebook %s", nb)
	}
	var out []domain.Node
	for _, id := range ids {
		if n, ok := p.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}
func (p *stablePlanInputs) SourcePath(_ context.Context, nb, id uuid.UUID) (domain.Node, bool, error) {
	n, ok := p.nodes[id]
	return n, ok && nb == p.notebook, nil
}
func (p *stablePlanInputs) Subtree(_ context.Context, nb, id uuid.UUID) ([]domain.Step, error) {
	if nb != p.notebook {
		return nil, fmt.Errorf("fixture subtree notebook %s", nb)
	}
	if n, ok := p.nodes[id]; ok {
		return slices.Clone(n.Path), nil
	}
	return nil, nil
}
func (p *stablePlanInputs) LinksChanged(_ context.Context, e app.LinksChanged) error {
	e.Pages, e.Targets = slices.Clone(e.Pages), slices.Clone(e.Targets)
	p.events = append(p.events, e)
	return nil
}

type stablePlanFixture struct {
	fixture
	specs      []stablePlanSpec
	inputs     *stablePlanInputs
	trace      *stablePlanTrace
	statistics map[string]int
	rebuilt    app.Rebuilt
	state      string
	before     map[string][]stablePlanStored
}

func stablePlanParsed(t *testing.T, parser app.Parser, text string, want domain.Facts) domain.Facts {
	t.Helper()
	got, err := parser.Facts(t.Context(), text)
	if err != nil || got.FrontmatterValid != want.FrontmatterValid || !slices.Equal(got.Links, want.Links) || !slices.Equal(got.Tags, want.Tags) || !slices.Equal(got.Aliases, want.Aliases) || !reflect.DeepEqual(got.Properties, want.Properties) {
		t.Fatalf("stable literal fixture/parser mismatch: links=%d/%d tags=%d/%d aliases=%d/%d properties=%d/%d err=%v", len(got.Links), len(want.Links), len(got.Tags), len(want.Tags), len(got.Aliases), len(want.Aliases), len(got.Properties), len(want.Properties), err)
	}
	return got
}

func newStablePlanFixture(t *testing.T, state string, specs []stablePlanSpec, noisy bool) *stablePlanFixture {
	t.Helper()
	f := newFixture(t)
	f.eng, f.ops = stablePlanID(900001), stablePlanID(900002)
	ingestPlanSetup(t, f)
	for _, table := range []string{"page_tags", "page_aliases"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parser := markdownadapter.NewParser(md, markdown.NewBudget(8<<20, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))))
	p := &stablePlanInputs{notebook: f.eng, specs: slices.Clone(specs), nodes: map[uuid.UUID]domain.Node{}, byKey: map[string][]domain.Node{}}
	if noisy {
		p.specs = append(p.specs, stablePlanSpecOf("same_noise", "noise", stablePlanID(50), 10000, false))
	}
	for i, name := range []string{"a", "b", "c", "d"} {
		p.specs = append(p.specs, stablePlanSpecOf(name, "target", stablePlanID(uint64(100+i)), 0, false))
	}
	for i := range p.specs {
		s := &p.specs[i]
		if len(s.Removed) != s.Z || len(s.ChangedOld) != s.C || len(s.ToWrite) != s.C+s.I || s.N != s.S+s.C+s.Z || s.M != s.S+s.C+s.I {
			t.Fatalf("stable literal classification %s: N/M/S/C/I/Z=%d/%d/%d/%d/%d/%d removed/changed/write=%d/%d/%d", s.Name, s.N, s.M, s.S, s.C, s.I, s.Z, len(s.Removed), len(s.ChangedOld), len(s.ToWrite))
		}
		n := domain.Node{ID: s.ID, Path: []domain.Step{{ID: s.ID, Key: s.Name, Name: s.Name}}}
		p.nodes[s.ID], p.byKey[s.Name] = n, []domain.Node{n}
		s.OldFacts = stablePlanParsed(t, parser, s.OldText, s.OldFacts)
		s.NewFacts = stablePlanParsed(t, parser, s.NewText, s.NewFacts)
		if err := f.s.AddPage(t.Context(), app.Page{ID: s.ID, NotebookID: f.eng, Revision: 1}, s.OldFacts); err != nil {
			t.Fatal(err)
		}
		var baseline []app.Link
		for _, r := range s.Old {
			if r.Resolved.Valid {
				baseline = append(baseline, app.Link{SourceID: r.Source, Start: int(r.Start), Target: r.Target, Resolution: domain.Resolution{ID: uuid.UUID(r.Resolved.Bytes), Ambiguous: r.Ambiguous, Asset: r.Asset}})
			}
		}
		if err := f.s.SetResolutions(t.Context(), baseline); err != nil {
			t.Fatal(err)
		}
	}
	if noisy {
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
SELECT ('60000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,200000) k`, f.ops)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous)
SELECT node_id,0,1,notebook_id,'wikilink','foreign','foreign',false FROM indexed_pages WHERE notebook_id=$1`, f.ops)
		foreign := stablePlanSpecOf("foreign_side", "noise", stablePlanID(800), 0, false)
		if err := f.s.AddPage(t.Context(), app.Page{ID: foreign.ID, NotebookID: f.ops, Revision: 1}, foreign.OldFacts); err != nil {
			t.Fatal(err)
		}
	}
	// Establish an explicit pristine seed without creating column statistics.
	for _, table := range stablePlanTables() {
		lintExec(t, f, "VACUUM "+table)
	}
	tr := &stablePlanTrace{}
	pool := lintPool(t, f, tr, 1)
	f.pool, f.s, f.tx = pool, postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second)
	out := &stablePlanFixture{fixture: f, specs: slices.Clone(p.specs[:len(specs)]), inputs: p, trace: tr, state: state}
	switch state {
	case "no_statistics":
	case "analyzed", "app_rebuild_after_analyzed", bufferedRebuildState:
		if err := f.s.Analyze(t.Context()); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown stable fixture state %q", state)
	}
	if state == "app_rebuild_after_analyzed" || state == bufferedRebuildState {
		tr.begin()
		// Original three-version state stays streaming; the added state uses
		// the default buffered protocol and must prove two-version history.
		limit := uint64(1)
		if state == bufferedRebuildState {
			limit = 0
		}
		rebuild := app.Rebuild{Tx: f.tx, Index: app.Index{Store: f.s, Pages: p, Publisher: p}, Contents: p, Notebooks: p, Parser: parser, Writes: f.s, StagingLimit: limit}
		out.rebuilt, err = rebuild.Notebook(t.Context(), f.eng)
		history := tr.end()
		links, unresolved := 0, 0
		for _, s := range p.specs {
			links += s.N
			for _, r := range s.Old {
				if !r.Resolved.Valid {
					unresolved++
				}
			}
		}
		if err != nil || out.rebuilt.Pages != len(p.specs) || out.rebuilt.Links != links || out.rebuilt.Unresolved != unresolved || p.locked != 1 || p.rekeyed != 1 || len(p.events) != 1 {
			t.Fatalf("real adapter rebuild: result=%+v err=%v locks=%d rekeys=%d events=%d", out.rebuilt, err, p.locked, p.rekeyed, len(p.events))
		}
		if state == bufferedRebuildState {
			pageRows := make(map[uuid.UUID]int, len(p.specs))
			for _, s := range p.specs {
				pageRows[s.ID] = s.N
			}
			bufferedRebuildHistory(t, tr, history, pageRows)
		} else {
			written := map[string]int64{}
			for _, call := range history {
				if call.Error != "" {
					t.Fatalf("real rebuild query %s: %s", call.Name, call.Error)
				}
				written[call.Name] += call.RowsAffected
			}
			if written["DeleteNotebooksLinks"] != int64(links) || written["InsertLinks"] != int64(links) || written["SetResolutions"]+written["SetOrderedSourceResolutions"] != int64(links-unresolved) || !slices.Equal(tr.terminal(), []string{"commit"}) {
				t.Fatalf("real rebuild version history: commands=%v links=%d resolved=%d terminal=%v", written, links, links-unresolved, tr.terminal())
			}
			lintEvidence(t, "stable-rebuild-history", history)
		}
		// No post-Rebuild VACUUM/ANALYZE, no manual resolution repair.
	}
	out.statistics = stablePlanStatistics(t, f, state != "no_statistics")
	stablePlanCatalog(t, out)
	for _, s := range p.specs {
		stablePlanAssertSource(t, t.Context(), out, s, s.Old, 1)
	}
	stablePlanAssertNoise(t, out, noisy)
	out.before = stablePlanSnapshot(t, t.Context(), out)
	return out
}

func stablePlanTables() []string {
	return []string{"indexed_pages", "page_links", "page_tags", "page_properties", "page_aliases"}
}

func stablePlanStatistics(t *testing.T, f fixture, analyzed bool) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range stablePlanTables() {
		var disabled bool
		var count int
		err := f.pool.QueryRow(t.Context(), `SELECT coalesce(reloptions,'{}') @> ARRAY['autovacuum_enabled=false'] FROM pg_class WHERE oid=$1::regclass`, table).Scan(&disabled)
		if err != nil || !disabled {
			t.Fatalf("stable automatic maintenance %s disabled=%t err=%v", table, disabled, err)
		}
		err = f.pool.QueryRow(t.Context(), `SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename=$1`, table).Scan(&count)
		if err != nil || (count > 0) != analyzed {
			t.Fatalf("stable statistics %s=%d analyzed=%t err=%v", table, count, analyzed, err)
		}
		out[table] = count
	}
	return out
}

func stablePlanCatalog(t *testing.T, f *stablePlanFixture) {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), `SELECT c.relname,c.relpages,c.reltuples,c.relfrozenxid::text,
 coalesce((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.attname) FROM pg_stats s WHERE s.schemaname=ns.nspname AND s.tablename=c.relname),'[]'::jsonb)::text,
 coalesce((SELECT jsonb_agg(jsonb_build_object('name',ic.relname,'definition',pg_get_indexdef(i.indexrelid),'access_method',am.amname,'btree_level',CASE WHEN am.amname='btree' THEN (bt_metap(i.indexrelid::regclass::text)).level ELSE NULL END) ORDER BY ic.relname) FROM pg_index i JOIN pg_class ic ON ic.oid=i.indexrelid JOIN pg_am am ON am.oid=ic.relam WHERE i.indrelid=c.oid),'[]'::jsonb)::text,
 CASE WHEN c.reltoastrelid=0 THEN NULL ELSE c.reltoastrelid::regclass::text END
 FROM pg_class c JOIN pg_namespace ns ON ns.oid=c.relnamespace
 WHERE c.oid=ANY(ARRAY['indexed_pages'::regclass,'page_links'::regclass,'page_tags'::regclass,'page_properties'::regclass,'page_aliases'::regclass]) ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name, xid, statistics, indexes string
		var pages int64
		var tuples float64
		var toast pgtype.Text
		if err := rows.Scan(&name, &pages, &tuples, &xid, &statistics, &indexes, &toast); err != nil {
			t.Fatal(err)
		}
		lintEvidence(t, "stable-fixture-catalog", map[string]any{"state": f.state, "table": name, "relpages": pages, "reltuples": tuples, "relfrozenxid": xid, "column_statistics": json.RawMessage(statistics), "indexes": json.RawMessage(indexes), "toast_relation": toast, "fixture_is_adapter_not_bootstrap_capacity": true})
		count++
	}
	if err := rows.Err(); err != nil || count != 5 {
		t.Fatalf("stable complete five-table catalog count=%d err=%v", count, err)
	}
}

func stablePlanAssertLinks(t *testing.T, ctx context.Context, f *stablePlanFixture, id uuid.UUID, want []stablePlanRow) {
	t.Helper()
	rows, err := postgres.DB(ctx, f.pool).Query(ctx, `SELECT source_id,notebook_id,range_start,range_end,kind,target,property_key,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,aliases,resolved_asset FROM page_links WHERE source_id=$1 ORDER BY range_start`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var r stablePlanRow
		if err := rows.Scan(&r.Source, &r.Notebook, &r.Start, &r.End, &r.Kind, &r.Target, &r.Property, &r.Anchor, &r.Display, &r.Key, &r.Alt, &r.Resolved, &r.Ambiguous, &r.Aliases, &r.Asset); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) || r != want[i] {
			t.Fatalf("stable complete row %s[%d]=%+v; expected row exists=%t", id, i, r, i < len(want))
		}
		i++
	}
	if err := rows.Err(); err != nil || i != len(want) {
		t.Fatalf("stable complete source %s rows=%d/%d err=%v", id, i, len(want), err)
	}
}

func stablePlanAssertSource(t *testing.T, ctx context.Context, f *stablePlanFixture, s stablePlanSpec, want []stablePlanRow, revision int) {
	t.Helper()
	stablePlanAssertLinks(t, ctx, f, s.ID, want)
	q := postgres.DB(ctx, f.pool)
	var nb uuid.UUID
	var rev, extractor int
	var valid bool
	err := q.QueryRow(ctx, `SELECT notebook_id,revision,extractor,frontmatter_valid FROM indexed_pages WHERE node_id=$1`, s.ID).Scan(&nb, &rev, &extractor, &valid)
	if err != nil || nb != f.eng || rev != revision || extractor != domain.Extractor || !valid {
		t.Fatalf("stable head %s nb=%s rev=%d extractor=%d valid=%t err=%v", s.ID, nb, rev, extractor, valid, err)
	}
	facts := s.OldFacts
	if revision == 2 {
		facts = s.NewFacts
	}
	for _, relation := range []string{"page_tags", "page_aliases", "page_properties"} {
		var sql string
		switch relation {
		case "page_tags":
			sql = `SELECT notebook_id,tag_key,tag,count FROM page_tags WHERE source_id=$1 ORDER BY tag_key`
		case "page_aliases":
			sql = `SELECT notebook_id,alias_key,alias FROM page_aliases WHERE source_id=$1 ORDER BY alias_key`
		case "page_properties":
			sql = `SELECT notebook_id,position,key,value FROM page_properties WHERE source_id=$1 ORDER BY position`
		}
		rows, err := q.Query(ctx, sql, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		for rows.Next() {
			var nb uuid.UUID
			var key, value string
			var n int
			switch relation {
			case "page_tags":
				err = rows.Scan(&nb, &key, &value, &n)
				if err == nil && (i >= len(facts.Tags) || key != facts.Tags[i].Key || value != facts.Tags[i].Name || n != facts.Tags[i].Count) {
					err = fmt.Errorf("tag row %d differs", i)
				}
			case "page_aliases":
				err = rows.Scan(&nb, &key, &value)
				if err == nil && (i >= len(facts.Aliases) || key != facts.Aliases[i].Key || value != facts.Aliases[i].Name) {
					err = fmt.Errorf("alias row %d differs", i)
				}
			case "page_properties":
				var raw []byte
				err = rows.Scan(&nb, &n, &key, &raw)
				var got, expected any
				if err == nil {
					err = json.Unmarshal(raw, &got)
				}
				if err == nil && i < len(facts.Properties) {
					err = json.Unmarshal(facts.Properties[i].Value, &expected)
				}
				if err == nil && (i >= len(facts.Properties) || n != i || key != facts.Properties[i].Key || !reflect.DeepEqual(got, expected)) {
					err = fmt.Errorf("property row %d differs", i)
				}
			}
			if err != nil || nb != f.eng {
				rows.Close()
				t.Fatalf("stable complete %s %s[%d]: %v nb=%s", relation, s.ID, i, err, nb)
			}
			i++
		}
		err = rows.Err()
		rows.Close()
		wantN := len(facts.Properties)
		if relation == "page_tags" {
			wantN = len(facts.Tags)
		}
		if relation == "page_aliases" {
			wantN = len(facts.Aliases)
		}
		if err != nil || i != wantN {
			t.Fatalf("stable complete %s %d/%d: %v", relation, i, wantN, err)
		}
	}
}

// All rows, including every noise fact, are compared after rollback. JSONB text
// here preserves SQL NULL and every stored column; it is not a hash oracle.
type stablePlanStored struct {
	Source uuid.UUID
	Row    string
}

func stablePlanSnapshot(t *testing.T, ctx context.Context, f *stablePlanFixture) map[string][]stablePlanStored {
	t.Helper()
	out := map[string][]stablePlanStored{}
	for _, table := range stablePlanTables() {
		order := "source_id"
		switch table {
		case "indexed_pages":
			order = "node_id"
		case "page_links":
			order += ",range_start"
		case "page_tags":
			order += ",tag_key"
		case "page_aliases":
			order += ",alias_key"
		case "page_properties":
			order += ",position"
		}
		id := "source_id"
		if table == "indexed_pages" {
			id = "node_id"
		}
		rows, err := postgres.DB(ctx, f.pool).Query(ctx, "SELECT "+id+",to_jsonb(t)::text FROM "+table+" t ORDER BY "+order)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var raw stablePlanStored
			if err := rows.Scan(&raw.Source, &raw.Row); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			out[table] = append(out[table], raw)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func stablePlanAssertNoise(t *testing.T, f *stablePlanFixture, noisy bool) {
	t.Helper()
	if !noisy {
		return
	}
	// The foreign seed's whole row formula is independent of the writer/query.
	var total, bad int
	err := f.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER(WHERE range_start<>0 OR range_end<>1 OR kind<>'wikilink' OR target<>'foreign' OR target_key IS DISTINCT FROM 'foreign' OR target_alt_key IS NOT NULL OR property_key IS NOT NULL OR anchor IS NOT NULL OR display IS NOT NULL OR resolved_id IS NOT NULL OR ambiguous OR aliases OR resolved_asset) FROM page_links WHERE notebook_id=$1`, f.ops).Scan(&total, &bad)
	if err != nil || total != 200000 || bad != 0 {
		t.Fatalf("stable foreign complete row seed: total=%d bad=%d err=%v", total, bad, err)
	}
	// Source IDs must be exactly the deterministic 200000-member seed too.
	err = f.pool.QueryRow(t.Context(), `WITH expected AS (SELECT ('60000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid AS id FROM generate_series(1,200000) k), actual AS (SELECT source_id AS id FROM page_links WHERE notebook_id=$1) SELECT count(*) FROM ((SELECT id FROM expected EXCEPT ALL SELECT id FROM actual) UNION ALL (SELECT id FROM actual EXCEPT ALL SELECT id FROM expected)) d`, f.ops).Scan(&bad)
	if err != nil || bad != 0 {
		t.Fatalf("stable foreign source set differs: %d %v", bad, err)
	}
	err = f.pool.QueryRow(t.Context(), `WITH expected AS (SELECT ('60000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid AS id FROM generate_series(1,200000) k), actual AS (SELECT node_id AS id FROM indexed_pages WHERE notebook_id=$1 AND node_id<>$2) SELECT count(*) FROM ((SELECT id FROM expected EXCEPT ALL SELECT id FROM actual) UNION ALL (SELECT id FROM actual EXCEPT ALL SELECT id FROM expected)) d`, f.ops, stablePlanID(800)).Scan(&bad)
	if err != nil || bad != 0 {
		t.Fatalf("stable foreign head set differs: %d %v", bad, err)
	}
	err = f.pool.QueryRow(t.Context(), `SELECT count(*) FROM indexed_pages WHERE notebook_id=$1 AND (revision<>1 OR extractor<>2 OR NOT frontmatter_valid)`, f.ops).Scan(&bad)
	if err != nil || bad != 0 {
		t.Fatalf("stable foreign head values differ: %d %v", bad, err)
	}
	foreign := stablePlanSpecOf("foreign_side", "noise", stablePlanID(800), 0, false)
	copy := *f
	copy.eng = f.ops
	stablePlanAssertSource(t, t.Context(), &copy, foreign, nil, 1)
}

func stablePlanAssertSnapshot(t *testing.T, ctx context.Context, f *stablePlanFixture, except *uuid.UUID) {
	t.Helper()
	got := stablePlanSnapshot(t, ctx, f)
	for _, table := range stablePlanTables() {
		wantRows, gotRows := f.before[table], got[table]
		i, j := 0, 0
		for {
			for i < len(wantRows) && except != nil && wantRows[i].Source == *except {
				i++
			}
			for j < len(gotRows) && except != nil && gotRows[j].Source == *except {
				j++
			}
			if i == len(wantRows) || j == len(gotRows) {
				break
			}
			if wantRows[i] != gotRows[j] {
				t.Fatalf("stable complete %s snapshot differs: got=%+v want=%+v", table, gotRows[j], wantRows[i])
			}
			i++
			j++
		}
		if i != len(wantRows) || j != len(gotRows) {
			t.Fatalf("stable complete %s snapshot length differs at %d/%d expected %d/%d", table, i, j, len(wantRows), len(gotRows))
		}
	}
}
