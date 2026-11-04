package bootstrap

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	linkingdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The original fixture, Writer, River job, full image and exact T oracle are
// shared with the explicit First/After fallback. Only the observed read route
// and its two retained, deliberately wrong ambiguity baselines differ.
func testIngestPreparedSourceBoundary(t *testing.T) { testIngestOldFBoundary(t, true) }

type ingestPreparedBoundary struct {
	s                                           *ingestNativeOldF
	base                                        pgx.Tx
	tx                                          *ingestPreparedBoundaryTx
	unit                                        *linkingapp.SourcePreparationUnit
	created, read, applied                      []uuid.UUID
	inputs                                      map[uuid.UUID][]linkingapp.Link
	tailNames                                   map[uuid.UUID]string
	inspect                                     []*ingestPreparedBoundaryRows
	residualRead                                *ingestPreparedBoundaryRows
	residualWant, residualSeen, residual, fresh []linkingapp.Link
	residualCalls, setCalls                     int
}

func (p *ingestPreparedBoundary) fail(message string) error {
	err := errors.New("PREPARED_BOUNDARY_FIXTURE: " + message)
	if p.s.life.fixtureErr == nil {
		p.s.life.fixtureErr = err
	}
	return err
}

// The original manager still owns begin/end. Every prepared handle sees one
// stable forwarding Tx identity, not a fresh wrapper on ReadBefore alone.
func (p *ingestPreparedBoundary) bind(ctx context.Context) (context.Context, error) {
	tx, ok := postgres.TxFrom(ctx)
	if !ok || tx.Conn().PgConn().PID() != p.s.life.pid {
		return nil, p.fail("missing original Writer Tx")
	}
	if p.tx == nil {
		p.base, p.tx = tx, &ingestPreparedBoundaryTx{Tx: tx, p: p}
	}
	if tx != p.base {
		return nil, p.fail("Writer Tx identity changed")
	}
	return ingestPreparedBoundaryContext{Context: ctx, tx: p.tx}, nil
}

type ingestPreparedBoundaryContext struct {
	context.Context
	tx *ingestPreparedBoundaryTx
}

func (c ingestPreparedBoundaryContext) Value(key any) any {
	v := c.Context.Value(key)
	if _, ok := v.(pgx.Tx); ok {
		return c.tx
	}
	return v
}

type ingestPreparedBoundaryFactory struct {
	linkingapp.SourcePreparations
	p *ingestPreparedBoundary
}

func (f ingestPreparedBoundaryFactory) NewSource(ctx context.Context, unit *linkingapp.SourcePreparationUnit, page linkingapp.Page, facts linkingdomain.Facts) linkingapp.PreparedSource {
	p := f.p
	if !c10DraftActive(ctx, p.s.life) {
		return f.SourcePreparations.NewSource(ctx, unit, page, facts)
	}
	bound, err := p.bind(ctx)
	if err != nil {
		return nil
	}
	if len(p.created) >= 2 || page.ID != p.s.ids[len(p.created)] || page.NotebookID != p.s.notebook || page.Revision != 2 || len(facts.Links) != p.s.sources[len(p.created)].rows {
		_ = p.fail("NewSource did not receive the complete ordered source facts")
		return nil
	}
	if p.unit == nil {
		p.unit = unit
	}
	if p.unit != unit {
		_ = p.fail("NewSource changed the preparation unit")
		return nil
	}
	p.s.life.mu.Lock()
	before := len(p.s.life.events)
	p.s.life.mu.Unlock()
	plan := f.SourcePreparations.NewSource(bound, unit, page, facts)
	p.s.life.mu.Lock()
	after := len(p.s.life.events)
	p.s.life.mu.Unlock()
	if plan == nil || before != after {
		_ = p.fail("real NewSource was absent or executed SQL")
		return nil
	}
	p.created = append(p.created, page.ID)
	return ingestPreparedBoundaryPlan{PreparedSource: plan, p: p, source: page.ID}
}

type ingestPreparedBoundaryPlan struct {
	linkingapp.PreparedSource
	p      *ingestPreparedBoundary
	source uuid.UUID
}

func (r ingestPreparedBoundaryPlan) ReadBefore(ctx context.Context, unit *linkingapp.SourcePreparationUnit) ([]uuid.UUID, error) {
	p, s := r.p, r.p.s
	bound, err := p.bind(ctx)
	if err != nil {
		return nil, err
	}
	if len(p.read) >= 2 || p.s.ids[len(p.read)] != r.source || unit != p.unit {
		return nil, p.fail("ReadBefore source/unit order changed")
	}
	p.read = append(p.read, r.source)
	s.calls++
	assets, err := r.PreparedSource.ReadBefore(bound, unit)
	if err != nil {
		if assets != nil {
			return nil, p.fail("failed ReadBefore returned partial F")
		}
		return nil, err
	}
	want := p.sourceRows(r.source, false)
	if len(assets) != len(want) {
		return nil, p.fail("ReadBefore truncated the full asset-flagged relation")
	}
	for i, id := range assets {
		if id != want[i].Resolution.ID {
			return nil, p.fail("ReadBefore asset order/value changed")
		}
	}
	b := s.batches[len(s.batches)-1]
	if !b.eof || !b.errRead || !b.closed || p.tx.Conn().PgConn().IsBusy() || s.stream.replaces != 0 {
		return nil, p.fail("ReadBefore returned before EOF/Err/Close or after Replace")
	}
	if len(p.read) == 1 {
		if err := s.pause(ctx, p.base); err != nil {
			return nil, err
		}
	}
	return assets, nil
}

func (r ingestPreparedBoundaryPlan) Apply(ctx context.Context, unit *linkingapp.SourcePreparationUnit) (linkingapp.Dropped, *linkingapp.SourceLinks, error) {
	p := r.p
	bound, err := p.bind(ctx)
	if err != nil {
		return linkingapp.Dropped{}, nil, err
	}
	if len(p.read) != 2 || !slices.Equal(p.s.seen, p.s.want) || slices.Contains(p.applied, r.source) || unit != p.unit {
		return linkingapp.Dropped{}, nil, p.fail("Apply preceded complete F or repeated a source")
	}
	old, supplied, err := r.PreparedSource.Apply(bound, unit)
	if err != nil {
		return old, supplied, err
	}
	if supplied == nil || supplied.SourceID != r.source || !slices.Equal(supplied.Links, p.sourceRows(r.source, true)) {
		return linkingapp.Dropped{}, nil, p.fail("Apply lost the complete changed/retained comparison baseline")
	}
	p.applied = append(p.applied, r.source)
	p.inputs[r.source] = slices.Clone(supplied.Links)
	return old, supplied, nil
}

func (f ingestPreparedBoundaryFactory) LinksExcept(ctx context.Context, notebook uuid.UUID, reach linkingdomain.Reach, excluded []uuid.UUID) ([]linkingapp.Link, error) {
	p := f.p
	if !c10DraftActive(ctx, p.s.life) {
		return f.SourcePreparations.LinksExcept(ctx, notebook, reach, excluded)
	}
	bound, err := p.bind(ctx)
	if err != nil {
		return nil, err
	}
	if len(p.applied) != 2 || !slices.Equal(excluded, p.s.ids) {
		return nil, p.fail("residual read preceded both Apply results or excluded the wrong sources")
	}
	before := linkingdomain.Reach{Sources: slices.Clone(reach.Sources), Targets: slices.Clone(reach.Targets), Keys: slices.Clone(reach.Keys)}
	p.residualCalls++
	got, err := f.SourcePreparations.LinksExcept(bound, notebook, reach, excluded)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(before.Sources, reach.Sources) || !slices.Equal(before.Targets, reach.Targets) || !slices.Equal(before.Keys, reach.Keys) {
		return nil, p.fail("residual mutated caller Reach")
	}
	p.residual = slices.Clone(got)
	return got, nil
}

type ingestPreparedBoundaryStore struct {
	linkingapp.Store
	p *ingestPreparedBoundary
}

func (r ingestPreparedBoundaryStore) SetResolutions(ctx context.Context, links []linkingapp.Link) error {
	p := r.p
	if c10DraftActive(ctx, p.s.life) {
		if _, err := p.bind(ctx); err != nil {
			return err
		}
		if p.residualCalls != 1 {
			return p.fail("Set preceded the actual residual SQL")
		}
		p.setCalls++
		for _, link := range links {
			if slices.Contains(p.s.ids, link.SourceID) {
				p.fresh = append(p.fresh, link)
			}
		}
	}
	return r.Store.SetResolutions(ctx, links)
}

type ingestPreparedBoundaryTx struct {
	pgx.Tx
	p *ingestPreparedBoundary
}

func (tx *ingestPreparedBoundaryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	p, name := tx.p, stableIndexSQLName(sql)
	if name != "InspectPageLinksForPreparation" && name != "LinksReachedExcept" {
		return tx.Tx.Query(ctx, sql, args...)
	}
	var source uuid.UUID
	residual := name == "LinksReachedExcept"
	if !residual {
		if len(p.inspect) >= 2 || len(args) != 3 || args[0] != p.s.ids[len(p.inspect)] || args[1] != p.s.notebook || args[2] != int32(2) {
			return nil, p.fail("native Inspect arguments/order changed")
		}
		source = args[0].(uuid.UUID)
	} else {
		if p.residualRead != nil || len(args) != 6 || args[0] != pgx.QueryExecModeDescribeExec || args[3] != p.s.notebook {
			return nil, p.fail("native residual mode/arguments changed")
		}
		ids, ok := args[5].([]uuid.UUID)
		excluded, exact := args[1].([]uuid.UUID)
		if !ok || !exact || !slices.Equal(excluded, p.s.ids) {
			return nil, p.fail("native residual omitted the supplied exclusion")
		}
		for _, id := range ids {
			if slices.Contains(p.s.ids, id) {
				return nil, p.fail("native residual source arm still requests supplied rows")
			}
		}
	}
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err != nil {
		return rows, err
	}
	b := &ingestOldFTailBatch{}
	r := &ingestPreparedBoundaryRows{Rows: rows, p: p, b: b, source: source, residual: residual, stable: true}
	if residual {
		p.residualRead = r
	} else {
		p.inspect = append(p.inspect, r)
		p.s.batches = append(p.s.batches, b)
	}
	return r, nil
}

type ingestPreparedBoundaryRows struct {
	pgx.Rows
	p                *ingestPreparedBoundary
	b                *ingestOldFTailBatch
	source           uuid.UUID
	residual, stable bool
	dest             []any
	closes           int
}

func (r *ingestPreparedBoundaryRows) Next() bool {
	ok := r.Rows.Next()
	if !ok {
		r.b.eof = true
	}
	return ok
}
func (r *ingestPreparedBoundaryRows) Close()     { r.Rows.Close(); r.closes++; r.b.closed = true }
func (r *ingestPreparedBoundaryRows) Err() error { r.b.errRead = true; return r.Rows.Err() }
func (r *ingestPreparedBoundaryRows) Scan(dest ...any) error {
	if r.dest == nil {
		r.dest = slices.Clone(dest)
	}
	r.stable = r.stable && slices.Equal(r.dest, dest)
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	if r.residual {
		return r.scanResidual(dest)
	}
	p, s := r.p, r.p.s
	if len(dest) != 15 {
		return p.fail("Inspect did not scan all 15 columns")
	}
	can, c := dest[0].(*bool)
	start, a := dest[1].(*pgtype.Int4)
	end, e := dest[2].(*int32)
	book, b := dest[3].(*uuid.UUID)
	kind, k := dest[4].(*string)
	property, q := dest[5].(*pgtype.Text)
	target, d := dest[6].(*string)
	anchor, h := dest[7].(*pgtype.Text)
	display, v := dest[8].(*pgtype.Text)
	key, y := dest[9].(*pgtype.Text)
	alt, z := dest[10].(*pgtype.Text)
	aliases, l := dest[11].(*bool)
	id, i := dest[12].(*pgtype.UUID)
	ambiguous, m := dest[13].(*bool)
	asset, f := dest[14].(*bool)
	if !c || !a || !e || !b || !k || !q || !d || !h || !v || !y || !z || !l || !i || !m || !f || !*can || !start.Valid || !id.Valid || *book != s.notebook || *kind != "link" || property.Valid || anchor.Valid || display.Valid || alt.Valid || *aliases || !*asset {
		return p.fail("15-column native values/destinations differ from the literal fixture")
	}
	oldTarget, tail := p.tailNames[uuid.UUID(id.Bytes)]
	if !tail {
		oldTarget = "a.txt"
	}
	if *target != oldTarget || !key.Valid || key.String != oldTarget || *end != start.Int32+int32(len(oldTarget)) || *ambiguous != tail {
		return p.fail("native old facts or wrong-derived tail baseline changed")
	}
	row := linkingapp.BodyReference{EdgePos: linkingapp.EdgePos{SourceID: r.source, Start: start.Int32}, ResolvedID: uuid.UUID(id.Bytes), ResolvedAsset: *asset}
	if at := len(s.seen); at >= len(s.want) || row != s.want[at] {
		return p.fail("native F omitted/reordered a literal row")
	}
	s.seen = append(s.seen, row)
	r.b.rows++
	if len(s.seen) == 16385 && s.mode != "healthy" {
		// A real second Scan on this final native row produces pgx's own error.
		// The rows_err leg returns the first success, then native Next/Err report
		// that retained error. No fake EOF, replacement Err or cancellation.
		s.cause = r.Rows.Scan(dest[:14]...)
		if s.cause == nil || s.cause.Error() != "number of field descriptions must equal number of destinations, got 15 and 14" {
			return p.fail("native Scan count fault did not produce its pinned pgx cause")
		}
		s.injected, s.life.coverageErr = true, s.cause
		if s.mode == "scan" {
			return s.cause
		}
	}
	return nil
}

func (r *ingestPreparedBoundaryRows) scanResidual(dest []any) error {
	if len(dest) != 7 {
		return r.p.fail("residual did not scan seven native columns")
	}
	source, a := dest[0].(*uuid.UUID)
	start, b := dest[1].(*int32)
	target, c := dest[2].(*string)
	id, d := dest[3].(**uuid.UUID)
	ambiguous, e := dest[4].(*bool)
	asset, f := dest[5].(*bool)
	aliases, g := dest[6].(*bool)
	if !a || !b || !c || !d || !e || !f || !g || slices.Contains(r.p.s.ids, *source) {
		return r.p.fail("native residual returned a supplied source or changed destinations")
	}
	row := linkingapp.Link{SourceID: *source, Start: int(*start), Target: *target, Aliases: *aliases}
	if *id != nil {
		row.Resolution = linkingdomain.Resolution{ID: **id, Ambiguous: *ambiguous, Asset: *asset}
	}
	r.p.residualSeen = append(r.p.residualSeen, row)
	r.b.rows++
	return nil
}

// Literal new targets follow a.txt -> b.txt, preserving every byte range. A
// supplied baseline has zero Resolutions for changed rows and wrong ambiguity
// on the retained tails; full fresh output must instead contain the true target and false ambiguity.
func (p *ingestPreparedBoundary) sourceRows(source uuid.UUID, baseline bool) []linkingapp.Link {
	var out []linkingapp.Link
	for _, old := range p.s.want {
		if old.SourceID != source {
			continue
		}
		target, tail := p.tailNames[old.ResolvedID]
		if !tail {
			target = "b.txt"
		}
		row := linkingapp.Link{SourceID: source, Start: int(old.Start), Target: target}
		if !baseline || tail {
			row.Resolution = linkingdomain.Resolution{ID: old.ResolvedID, Ambiguous: baseline && tail, Asset: true}
		}
		out = append(out, row)
	}
	return out
}

func (p *ingestPreparedBoundary) seed(t *testing.T, f ingestFixture, one, two string) {
	t.Helper()
	p.tailNames = map[uuid.UUID]string{uuid.MustParse(one): "tail-one.txt", uuid.MustParse(two): "tail-two.txt"}
	for _, source := range p.s.sources {
		ingestExec(t, f.tm, "UPDATE page_links SET ambiguous=true WHERE source_id=$1 AND range_start=$2", source.id, int32(19+10*(source.rows-1)))
	}
	p.residualWant = append(p.residualWant,
		linkingapp.Link{SourceID: uuid.MustParse(f.p), Start: 6, Target: "raw/tail-one.txt"},
		linkingapp.Link{SourceID: uuid.MustParse(f.p), Start: 30, Target: "raw/tail-two.txt"},
		linkingapp.Link{SourceID: uuid.MustParse(f.p), Start: 50, Target: "raw/T"})
	slices.SortFunc(p.residualWant, func(a, b linkingapp.Link) int {
		if c := a.SourceID.Compare(b.SourceID); c != 0 {
			return c
		}
		return a.Start - b.Start
	})
}

func (p *ingestPreparedBoundary) addQ(id, target, name string) {
	p.residualWant = append(p.residualWant, linkingapp.Link{SourceID: uuid.MustParse(id), Start: 2, Target: name,
		Resolution: linkingdomain.Resolution{ID: uuid.MustParse(target)}})
}

func (p *ingestPreparedBoundary) checkPaused(t *testing.T) {
	t.Helper()
	if !slices.Equal(p.created, p.s.ids) || len(p.read) != 1 || len(p.inspect) != 1 || len(p.applied) != 0 || p.residualCalls != 0 || p.setCalls != 0 {
		t.Fatal("PREPARED_GATE: creation/read/apply advanced during the real River gate")
	}
}

func (p *ingestPreparedBoundary) check(t *testing.T, f ingestFixture) {
	t.Helper()
	s := p.s
	if !slices.Equal(p.created, s.ids) || !slices.Equal(p.read, s.ids) || len(p.inspect) != 2 || s.calls != 2 || len(s.batches) != 2 {
		t.Fatal("PREPARED_ONCE: not exactly two complete actual source inspections")
	}
	for i, r := range p.inspect {
		ended := s.mode != "scan" || i == 0
		if r.source != s.sources[i].id || r.b.rows != s.sources[i].rows || !r.stable || r.closes != 1 || !r.b.closed || r.b.eof != ended || r.b.errRead != ended {
			t.Fatalf("PREPARED_NATIVE: source%d EOF/Err/Close/15 stable destinations differ: %+v", i, r.b)
		}
		seen := map[any]bool{}
		for _, dst := range r.dest {
			seen[dst] = true
		}
		if len(seen) != 15 {
			t.Fatal("PREPARED_NATIVE: Scan fields share addresses")
		}
	}
	if s.mode != "healthy" {
		if len(p.applied) != 0 || len(p.inputs) != 0 || p.residualRead != nil || p.residualCalls != 0 || p.setCalls != 0 {
			t.Fatal("PREPARED_FAILURE: successor work followed native error")
		}
		return
	}
	if len(p.applied) != 2 || len(p.inputs) != 2 || p.residualCalls != 1 || p.setCalls != 1 || p.residualRead == nil {
		t.Fatal("PREPARED_HEALTHY: missing actual supply/residual/fresh Set")
	}
	var wantFresh []linkingapp.Link
	supplied := 0
	for _, source := range s.sources {
		supplied += len(p.inputs[source.id])
		wantFresh = append(wantFresh, p.sourceRows(source.id, false)...)
	}
	r := p.residualRead
	seen := map[any]bool{}
	for _, dst := range r.dest {
		seen[dst] = true
	}
	if len(seen) != 7 {
		t.Fatal("PREPARED_RESIDUAL: native Scan fields share addresses")
	}
	if supplied != 16385 || !slices.Equal(p.fresh, wantFresh) || !slices.Equal(p.residual, p.residualWant) || !slices.Equal(p.residualSeen, p.residualWant) || r.b.rows != 5 || !r.b.eof || !r.b.errRead || !r.b.closed || r.closes != 1 || !r.stable {
		t.Fatalf("PREPARED_FRESH: supplied=%d source Set=%d residual=%d; full rows/wrong-derived tails/residual EOF differ", supplied, len(p.fresh), len(p.residual))
	}
	if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=ANY($1::uuid[]) AND resolved_id IS NOT NULL AND resolved_asset AND NOT ambiguous", s.ids); n != 16385 {
		t.Fatalf("PREPARED_PERSISTED: corrected live source rows=%d, want16385", n)
	}
}
