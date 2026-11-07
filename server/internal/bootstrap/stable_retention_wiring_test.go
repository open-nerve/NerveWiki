package bootstrap

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	linkingdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type stableWiringRow struct {
	Source, Notebook                    uuid.UUID
	Start, End                          int32
	Kind, Target                        string
	Property, Anchor, Display, Key, Alt *string
	Aliases                             bool
	Resolved                            *uuid.UUID
	Ambiguous, Asset                    bool
}

func stableWiringRows(ctx context.Context, q postgres.Querier, id uuid.UUID) ([]stableWiringRow, error) {
	rows, err := q.Query(ctx, `SELECT source_id,notebook_id,range_start,range_end,kind,property_key,target,anchor,display,target_key,target_alt_key,aliases,resolved_id,ambiguous,resolved_asset FROM page_links WHERE source_id=$1 ORDER BY range_start`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stableWiringRow
	for rows.Next() {
		var r stableWiringRow
		if err := rows.Scan(&r.Source, &r.Notebook, &r.Start, &r.End, &r.Kind, &r.Property, &r.Target, &r.Anchor, &r.Display, &r.Key, &r.Alt, &r.Aliases, &r.Resolved, &r.Ambiguous, &r.Asset); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func stableWiringText(s string) *string { return &s }
func stableWiringExpected(p, nb, a, b uuid.UUID) []stableWiringRow {
	return []stableWiringRow{
		{Source: p, Notebook: nb, Start: 2, End: 7, Kind: "wikilink", Target: "raw/S", Key: stableWiringText("s"), Resolved: &a},
		{Source: p, Notebook: nb, Start: 16, End: 21, Kind: "wikilink", Target: "raw/T", Key: stableWiringText("t"), Resolved: &b},
	}
}

type stableWiringStage struct {
	Name  string
	Order int
	PID   uint32
}
type stableWiringSQL struct {
	Name              string
	Order             int
	PID               uint32
	Done              bool
	Err               error
	Rows              int64
	Mode              pgx.QueryExecMode
	Sources, Targets  []uuid.UUID
	Starts            []int32
	Ambiguous, Assets []bool
	BadArgs           string
}
type stableWiringUnitKey struct{}
type stableWiringQueryKey struct{}
type stableWiringQueryAt struct {
	state *stableWiringState
	index int
}
type stableWiringState struct {
	mu                                  sync.Mutex
	armed                               bool
	page                                uuid.UUID
	clock, units                        int
	stages                              []stableWiringStage
	sql                                 []stableWiringSQL
	replaced, afterSet                  []stableWiringRow
	probeErrors                         []error
	dropped                             linkingapp.Dropped
	links, changed                      []linkingapp.Link
	reach                               linkingdomain.Reach
	keys                                []string
	candidates, pathInputs, pathOutputs []uuid.UUID
}

func stableWiringPID(ctx context.Context) uint32 {
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return 0
	}
	return tx.Conn().PgConn().PID()
}
func (s *stableWiringState) marked(ctx context.Context) bool {
	return ctx.Value(stableWiringUnitKey{}) == s
}
func (s *stableWiringState) stageLocked(ctx context.Context, name string) {
	s.clock++
	s.stages = append(s.stages, stableWiringStage{name, s.clock, stableWiringPID(ctx)})
}
func (s *stableWiringState) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if !s.marked(ctx) || !strings.HasPrefix(d.SQL, "-- name: ") {
		return ctx
	}
	name := strings.Fields(d.SQL)[2]
	call := stableWiringSQL{Name: name, PID: c.PgConn().PID()}
	args := d.Args
	if len(args) > 0 {
		if mode, ok := args[0].(pgx.QueryExecMode); ok {
			call.Mode = mode
			args = args[1:]
		}
	}
	if name == "SetResolutions" {
		if len(args) != 5 {
			call.BadArgs = fmt.Sprintf("actual Set parameter count%d", len(args))
		} else {
			sources, sok := args[0].([]uuid.UUID)
			starts, pok := args[1].([]int32)
			targets, tok := args[2].([]uuid.UUID)
			ambiguous, aok := args[3].([]bool)
			assets, kok := args[4].([]bool)
			if !sok || !pok || !tok || !aok || !kok {
				call.BadArgs = fmt.Sprintf("actual Set types %T %T %T %T %T", args[0], args[1], args[2], args[3], args[4])
			} else {
				call.Sources = slices.Clone(sources)
				call.Starts = slices.Clone(starts)
				call.Targets = slices.Clone(targets)
				call.Ambiguous = slices.Clone(ambiguous)
				call.Assets = slices.Clone(assets)
			}
		}
	}
	s.mu.Lock()
	s.clock++
	call.Order = s.clock
	at := len(s.sql)
	s.sql = append(s.sql, call)
	s.mu.Unlock()
	return context.WithValue(ctx, stableWiringQueryKey{}, stableWiringQueryAt{s, at})
}
func (s *stableWiringState) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	at, ok := ctx.Value(stableWiringQueryKey{}).(stableWiringQueryAt)
	if !ok || at.state != s {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q := &s.sql[at.index]
	q.Done, q.Err, q.Rows = true, d.Err, d.CommandTag.RowsAffected()
}

type stableWiringObserver struct {
	real  linkIndex
	state *stableWiringState
}

func (o stableWiringObserver) PagesChanged(ctx context.Context, e page.Event) error {
	o.state.mu.Lock()
	armed, p := o.state.armed, o.state.page
	o.state.mu.Unlock()
	match := false
	for _, c := range e.Changes {
		match = match || armed && c.NodeID == p && c.Revision == 3
	}
	if match {
		ctx = context.WithValue(ctx, stableWiringUnitKey{}, o.state)
		o.state.mu.Lock()
		o.state.units++
		o.state.stageLocked(ctx, "observer")
		o.state.mu.Unlock()
	}
	return o.real.PagesChanged(ctx, e)
}

type stableWiringStore struct {
	linkingapp.Store
	state *stableWiringState
}

func (s stableWiringStore) ReplacePage(ctx context.Context, p linkingapp.Page, f linkingdomain.Facts) (linkingapp.Dropped, error) {
	dropped, err := s.Store.ReplacePage(ctx, p, f)
	if err == nil && s.state.marked(ctx) {
		tx, ok := postgres.TxFrom(ctx)
		var rows []stableWiringRow
		var probe error
		if !ok {
			probe = fmt.Errorf("Replace observer has no original Tx")
		} else {
			rows, probe = stableWiringRows(ctx, tx, p.ID)
		}
		s.state.mu.Lock()
		s.state.stageLocked(ctx, "replaced")
		s.state.replaced = rows
		s.state.probeErrors = append(s.state.probeErrors, probe)
		s.state.dropped = linkingapp.Dropped{Targets: slices.Clone(dropped.Targets), AliasKeys: slices.Clone(dropped.AliasKeys)}
		s.state.mu.Unlock()
	}
	return dropped, err
}
func (s stableWiringStore) Links(ctx context.Context, nb uuid.UUID, r linkingdomain.Reach) ([]linkingapp.Link, error) {
	got, err := s.Store.Links(ctx, nb, r)
	if s.state.marked(ctx) {
		s.state.mu.Lock()
		s.state.stageLocked(ctx, "links")
		s.state.links = slices.Clone(got)
		s.state.reach = linkingdomain.Reach{Sources: slices.Clone(r.Sources), Targets: slices.Clone(r.Targets), Keys: slices.Clone(r.Keys)}
		s.state.mu.Unlock()
	}
	return got, err
}
func (s stableWiringStore) SetResolutions(ctx context.Context, links []linkingapp.Link) error {
	var pageID uuid.UUID
	if s.state.marked(ctx) {
		s.state.mu.Lock()
		s.state.stageLocked(ctx, "set")
		s.state.changed = slices.Clone(links)
		pageID = s.state.page
		s.state.mu.Unlock()
	}
	err := s.Store.SetResolutions(ctx, links)
	if err == nil && s.state.marked(ctx) {
		tx, ok := postgres.TxFrom(ctx)
		var rows []stableWiringRow
		var probe error
		if !ok {
			probe = fmt.Errorf("Set observer has no original Tx")
		} else {
			rows, probe = stableWiringRows(ctx, tx, pageID)
		}
		s.state.mu.Lock()
		s.state.stageLocked(ctx, "set_finished")
		s.state.afterSet = rows
		s.state.probeErrors = append(s.state.probeErrors, probe)
		s.state.mu.Unlock()
	}
	return err
}

type stableWiringPages struct {
	linkingapp.Pages
	state *stableWiringState
}

func (p stableWiringPages) ByKeys(ctx context.Context, nb uuid.UUID, keys []string) ([]linkingdomain.Node, error) {
	got, err := p.Pages.ByKeys(ctx, nb, keys)
	if p.state.marked(ctx) {
		p.state.mu.Lock()
		p.state.stageLocked(ctx, "bykeys")
		p.state.keys = slices.Clone(keys)
		for _, n := range got {
			p.state.candidates = append(p.state.candidates, n.ID)
		}
		p.state.mu.Unlock()
	}
	return got, err
}
func (p stableWiringPages) Paths(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) ([]linkingdomain.Node, error) {
	got, err := p.Pages.Paths(ctx, nb, ids)
	if p.state.marked(ctx) {
		p.state.mu.Lock()
		p.state.stageLocked(ctx, "paths")
		p.state.pathInputs = slices.Clone(ids)
		for _, n := range got {
			p.state.pathOutputs = append(p.state.pathOutputs, n.ID)
		}
		p.state.mu.Unlock()
	}
	return got, err
}

// A held nonzero A baseline and a real newly written B distinguish the actual
// retained branch from legacy even though their final business state agrees.
func TestStableLinkRetentionUsesTheRealIndex(t *testing.T) {
	state := &stableWiringState{}
	w := wiring{queryTracer: state, page: func(d *page.Deps) {
		found := 0
		for i, observer := range d.Observers {
			real, ok := observer.(linkIndex)
			if !ok {
				continue
			}
			real.index.Store = stableWiringStore{real.index.Store, state}
			real.index.Pages = stableWiringPages{real.index.Pages, state}
			d.Observers[i] = stableWiringObserver{real, state}
			found++
		}
		if found != 1 {
			t.Fatalf("stable fixture has%d real index observers", found)
		}
	}}
	f := newIngestFixture(t, &w)
	p, nb, a, b := uuid.MustParse(f.p), uuid.MustParse(f.nb), uuid.MustParse(f.s), uuid.MustParse(f.t)
	if a == b || a == uuid.Nil() || b == uuid.Nil() {
		t.Fatal("two source IDs must be distinct nonzero")
	}
	marker := f.tm.createPage(t, "alice", f.nb, "", "Stable marker")
	f.write(t, f.p, "[[raw/S]] old\n")
	expected := stableWiringExpected(p, nb, a, b)
	initial, err := stableWiringRows(t.Context(), f.tm.pool, p)
	if err != nil || !reflect.DeepEqual(initial, expected[:1]) {
		t.Fatalf("first real write has no exact nonzero A baseline: got%+v err%v", initial, err)
	}
	f.tm.settle(t, f.nb, marker)
	stream := openStream(t, f.tm.base, "alice", f.tm.tokens["alice"])
	state.mu.Lock()
	state.page = p
	state.armed = true
	state.mu.Unlock()
	cs := f.write(t, f.p, "[[raw/S]] new\n[[raw/T]]\n")
	state.mu.Lock()
	state.armed = false
	state.mu.Unlock()
	// The real request completed; subsequent fixture writes are unmarked.
	// Snapshot under mutex; every observation below is checked on this goroutine.
	state.mu.Lock()
	units := state.units
	stages := slices.Clone(state.stages)
	sql := slices.Clone(state.sql)
	replaced := slices.Clone(state.replaced)
	afterSet := slices.Clone(state.afterSet)
	probes := slices.Clone(state.probeErrors)
	dropped := state.dropped
	links := slices.Clone(state.links)
	changed := slices.Clone(state.changed)
	reach := state.reach
	keys := slices.Clone(state.keys)
	candidates := slices.Clone(state.candidates)
	pathIn := slices.Clone(state.pathInputs)
	pathOut := slices.Clone(state.pathOutputs)
	state.mu.Unlock()
	if units != 1 || len(probes) != 2 || len(replaced) != 2 {
		t.Fatalf("stable observation was not the real one complete Index unit: units%d probes%v rows%+v", units, probes, replaced)
	}
	for _, err := range probes {
		if err != nil {
			t.Fatalf("stable same-Tx probe failed: %v", err)
		}
	}
	orders := map[string]int{}
	pid := uint32(0)
	for _, s := range stages {
		if s.PID == 0 || pid != 0 && s.PID != pid {
			t.Fatalf("stable observer lost original Tx: %+v", stages)
		}
		pid = s.PID
		if orders[s.Name] != 0 {
			t.Fatalf("stable observer repeated stage %s", s.Name)
		}
		orders[s.Name] = s.Order
	}
	if orders["replaced"] == 0 {
		t.Fatal("stable Replace observation never ran")
	}
	// This MUST be the first optimization failure for always-legacy. Do not
	// Fatal or affect the real callback: its fresh resolve/Set already finished.
	if replaced[0].Resolved == nil || *replaced[0].Resolved == uuid.Nil() {
		t.Errorf("stable wiring: retained A baseline is zero after real ReplacePage: page=%s start=2 expected=%s observed=%v pid=%d", p, a, replaced[0].Resolved, pid)
	}
	wantReplace := stableWiringExpected(p, nb, a, b)
	wantReplace[1].Resolved = nil
	if !reflect.DeepEqual(replaced, wantReplace) {
		t.Errorf("stable complete literal Replace facts=%+v want%+v", replaced, wantReplace)
	}
	if !slices.Equal(dropped.Targets, []uuid.UUID{a}) || len(dropped.AliasKeys) != 0 {
		t.Errorf("stable all old D lost retained A: %+v", dropped)
	}
	counts := map[string]int{}
	first := map[string]int{}
	freshCounts := map[string]int{}
	freshFirst := map[string]int{}
	var setSQL []stableWiringSQL
	for _, q := range sql {
		counts[q.Name]++
		if first[q.Name] == 0 {
			first[q.Name] = q.Order
		}
		if !q.Done || q.Err != nil || q.PID != pid {
			t.Errorf("stable real SQL completion/PID: %+v", q)
		}
		if q.Name == "SetResolutions" {
			setSQL = append(setSQL, q)
		}
		// Capture reads targets before Replace; only this interval is fresh resolve.
		if q.Order > orders["links"] && q.Order < orders["set"] {
			freshCounts[q.Name]++
			if freshFirst[q.Name] == 0 {
				freshFirst[q.Name] = q.Order
			}
		}
	}
	for _, name := range []string{"InspectPageLinksForRetention", "DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered"} {
		if counts[name] != 1 {
			t.Errorf("stable actual branch SQL %s count=%d want1", name, counts[name])
		}
	}
	for _, name := range []string{"DeleteLinksOf", "InsertLinks", "WritePageLinksRetainingEqual"} {
		if counts[name] != 0 {
			t.Errorf("stable wrong branch SQL %s count=%d", name, counts[name])
		}
	}
	for _, name := range []string{"LinkTargetsByKeys", "LinkTargetsByIDs"} {
		if freshCounts[name] != 1 {
			t.Errorf("stable real fresh candidate/path SQL %s count=%d want1", name, freshCounts[name])
		}
	}
	sequence := []int{orders["observer"], first["LockNotebook"], first["InspectPageLinksForRetention"], first["DeleteMissingPageLinks"], first["WritePageLinksRetainingEqualPrefiltered"], orders["replaced"], orders["links"], freshFirst["LinkTargetsByKeys"], orders["bykeys"], freshFirst["LinkTargetsByIDs"], orders["paths"], orders["set"], first["SetResolutions"], orders["set_finished"]}
	for i, n := range sequence {
		if n == 0 || i > 0 && sequence[i-1] >= n {
			t.Errorf("stable maintenance/fresh order=%v", sequence)
			break
		}
	}
	wantLinks := []linkingapp.Link{{SourceID: p, Start: 2, Target: "raw/S", Resolution: linkingdomain.Resolution{ID: a}}, {SourceID: p, Start: 16, Target: "raw/T"}}
	if !slices.Equal(links, wantLinks) || !slices.Contains(reach.Sources, p) {
		t.Errorf("stable full A/B fresh input: links=%+v reach=%+v", links, reach)
	}
	if !slices.Equal(keys, []string{"s", "t"}) || !slices.Contains(candidates, a) || !slices.Contains(candidates, b) || !slices.Contains(pathIn, p) || !slices.Contains(pathOut, p) {
		t.Errorf("stable full candidate/path reads: keys=%v candidates=%v pathsIn=%v pathsOut=%v", keys, candidates, pathIn, pathOut)
	}
	wantChanged := []linkingapp.Link{{SourceID: p, Start: 16, Target: "raw/T", Resolution: linkingdomain.Resolution{ID: b}}}
	if !slices.Equal(changed, wantChanged) {
		t.Errorf("stable actual Set input=%+v want only fresh B=%+v", changed, wantChanged)
	}
	if len(setSQL) != 1 {
		t.Errorf("stable real Set SQL calls%d want1", len(setSQL))
	} else {
		q := setSQL[0]
		if q.Mode != pgx.QueryExecModeDescribeExec || q.BadArgs != "" || !slices.Equal(q.Sources, []uuid.UUID{p}) || !slices.Equal(q.Starts, []int32{16}) || !slices.Equal(q.Targets, []uuid.UUID{b}) || !slices.Equal(q.Ambiguous, []bool{false}) || !slices.Equal(q.Assets, []bool{false}) || q.Rows != 1 {
			t.Errorf("stable actual B-only SQL parameters/update: %+v", q)
		}
	}
	if !reflect.DeepEqual(afterSet, expected) {
		t.Errorf("stable same-Tx after real Set=%+v want%+v", afterSet, expected)
	}
	committed, err := stableWiringRows(t.Context(), f.tm.pool, p)
	if err != nil || !reflect.DeepEqual(committed, expected) {
		t.Errorf("stable committed facts/resolutions=%+v err=%v want%+v", committed, err, expected)
	}
	if got := f.tm.content(t, "alice", f.p); got.Revision != 3 {
		t.Errorf("stable actual revision%d want3", got.Revision)
	}
	event := stream.pages(t, f.nb)
	if event.Tree || !reflect.DeepEqual(event.written(), map[string]int{f.p: 3}) {
		t.Errorf("stable real pages event=%+v", event)
	}
	stream.links(t, f.nb, []string{}, []string{f.s, f.t})
	f.tm.quiet(t, f.nb, marker, stream)
	assertIngestMarks(t, f.tm, cs, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	t.Logf("STABLE_WIRING actual sameTxPID=%d initialA=%s fullFactRows=%d changedRows=%d SQL=%+v stages=%+v", pid, a, len(replaced), len(changed), sql, stages)
}
