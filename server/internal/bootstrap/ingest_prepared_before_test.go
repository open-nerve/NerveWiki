package bootstrap

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	linkingdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Observe B18's real explicitly wired preparation. The extra public Capture
// keeps its original ports and cannot produce any of these observations.
type ingestBeforePrepared struct {
	trace                               *ingestBeforeProductionTrace
	real                                linkingapp.PreparedSource
	source, notebook                    uuid.UUID
	tx                                  pgx.Tx
	pid                                 int32
	ready, replaced, admitted           bool
	newQueries, queries, reads, applies int
	scans, eof, errs, closes            int
	rows                                []linkingapp.BodyReference
	assets                              []uuid.UUID
	supplied                            []linkingapp.Link
	hasSupply                           bool
	err                                 error
}

type ingestBeforePreparations struct {
	linkingapp.SourcePreparations
	trace *ingestBeforeProductionTrace
}

func (f ingestBeforePreparations) NewSource(ctx context.Context, unit *linkingapp.SourcePreparationUnit, page linkingapp.Page, facts linkingdomain.Facts) linkingapp.PreparedSource {
	f.trace.mu.Lock()
	armed := f.trace.armed
	f.trace.mu.Unlock()
	if !armed {
		return f.SourcePreparations.NewSource(ctx, unit, page, facts)
	}
	p := &ingestBeforePrepared{trace: f.trace, source: page.ID, notebook: page.NotebookID}
	p.tx, p.ready = postgres.TxFrom(ctx)
	if p.ready {
		p.pid = int32(p.tx.Conn().PgConn().PID())
	}
	f.trace.mu.Lock()
	p.ready = p.ready && f.trace.locked && f.trace.tx == p.tx && len(f.trace.heads) > 0
	p.replaced = f.trace.replaced
	f.trace.prepared = append(f.trace.prepared, p)
	f.trace.mu.Unlock()
	p.real = f.SourcePreparations.NewSource(ingestBeforePreparedContext{ctx, p}, unit, page, facts)
	f.trace.mu.Lock()
	p.newQueries, p.admitted = p.queries, p.real != nil
	f.trace.mu.Unlock()
	if p.real == nil {
		return nil
	}
	return p
}

func (f ingestBeforePreparations) LinksExcept(ctx context.Context, notebook uuid.UUID, reach linkingdomain.Reach, excluded []uuid.UUID) ([]linkingapp.Link, error) {
	rows, err := f.SourcePreparations.LinksExcept(ctx, notebook, reach, excluded)
	f.trace.mu.Lock()
	defer f.trace.mu.Unlock()
	if f.trace.armed {
		f.trace.residuals++
		if !slices.Equal(excluded, []uuid.UUID{f.trace.target}) {
			return nil, fmt.Errorf("B18 prepared residual excluded=%v", excluded)
		}
		for _, row := range rows {
			if row.SourceID == f.trace.target {
				return nil, fmt.Errorf("B18 prepared source reentered residual SQL")
			}
		}
	}
	return rows, err
}

func (p *ingestBeforePrepared) ReadBefore(ctx context.Context, unit *linkingapp.SourcePreparationUnit) ([]uuid.UUID, error) {
	tx, ok := postgres.TxFrom(ctx)
	p.trace.mu.Lock()
	p.reads++
	p.ready = p.ready && ok && tx == p.tx
	p.replaced = p.replaced || p.trace.replaced
	p.trace.mu.Unlock()
	assets, err := p.real.ReadBefore(ingestBeforePreparedContext{ctx, p}, unit)
	p.trace.mu.Lock()
	p.assets, p.err = slices.Clone(assets), err
	p.replaced = p.replaced || p.trace.replaced
	p.trace.mu.Unlock()
	return assets, err
}

func (p *ingestBeforePrepared) Apply(ctx context.Context, unit *linkingapp.SourcePreparationUnit) (linkingapp.Dropped, *linkingapp.SourceLinks, error) {
	tx, ok := postgres.TxFrom(ctx)
	p.trace.mu.Lock()
	ready := ok && tx == p.tx && p.reads == 1 && p.eof == 1 && p.errs == 1 && p.closes == 1 && p.err == nil
	p.applies++
	p.trace.replaced = true
	stable := p.trace.stable
	p.trace.mu.Unlock()
	if !ready {
		return linkingapp.Dropped{}, nil, fmt.Errorf("B18 prepared Apply escaped complete original read")
	}
	if stable != nil {
		stable.mu.Lock()
		stable.captureReading = false
		stable.stageLocked(ctx, "replace_enter")
		stable.mu.Unlock()
	}
	old, supplied, err := p.real.Apply(ingestBeforePreparedContext{ctx, p}, unit)
	p.trace.mu.Lock()
	p.err = err
	if supplied != nil {
		p.hasSupply = supplied.SourceID == p.source
		p.supplied = slices.Clone(supplied.Links)
	}
	p.trace.mu.Unlock()
	return old, supplied, err
}

type ingestBeforePreparedContext struct {
	context.Context
	p *ingestBeforePrepared
}

func (c ingestBeforePreparedContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return ingestBeforePreparedTx{tx, c.p}
	}
	return v
}

type ingestBeforePreparedTx struct {
	pgx.Tx
	p *ingestBeforePrepared
}

func (tx ingestBeforePreparedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if stableIndexSQLName(sql) != "InspectPageLinksForPreparation" {
		return rows, err
	}
	tx.p.trace.mu.Lock()
	tx.p.queries++
	valid := len(args) == 3 && args[0] == tx.p.source && args[1] == tx.p.notebook && args[2] == int32(2) && int32(tx.Conn().PgConn().PID()) == tx.p.pid
	tx.p.trace.mu.Unlock()
	if !valid {
		if rows != nil {
			rows.Close()
		}
		return nil, fmt.Errorf("B18 actual preparation SQL scope/args changed")
	}
	if err != nil {
		return rows, err
	}
	return ingestBeforePreparedRows{rows, tx.p}, nil
}

type ingestBeforePreparedRows struct {
	pgx.Rows
	p *ingestBeforePrepared
}

func (r ingestBeforePreparedRows) Next() bool {
	ok := r.Rows.Next()
	if !ok {
		r.p.trace.mu.Lock()
		r.p.eof++
		r.p.trace.mu.Unlock()
	}
	return ok
}
func (r ingestBeforePreparedRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	if len(dest) != 15 {
		return fmt.Errorf("B18 prepared columns=%d", len(dest))
	}
	start, a := dest[1].(*pgtype.Int4)
	notebook, b := dest[3].(*uuid.UUID)
	property, c := dest[5].(*pgtype.Text)
	target, d := dest[12].(*pgtype.UUID)
	asset, e := dest[14].(*bool)
	if !a || !b || !c || !d || !e {
		return fmt.Errorf("B18 prepared native Scan destinations differ")
	}
	r.p.trace.mu.Lock()
	defer r.p.trace.mu.Unlock()
	r.p.scans++
	if start.Valid && *notebook == r.p.notebook && !property.Valid && target.Valid {
		r.p.rows = append(r.p.rows, linkingapp.BodyReference{EdgePos: linkingapp.EdgePos{SourceID: r.p.source, Start: start.Int32}, ResolvedID: uuid.UUID(target.Bytes), ResolvedAsset: *asset})
	}
	return nil
}
func (r ingestBeforePreparedRows) Err() error {
	err := r.Rows.Err()
	r.p.trace.mu.Lock()
	r.p.errs++
	r.p.trace.mu.Unlock()
	return err
}
func (r ingestBeforePreparedRows) Close() {
	r.Rows.Close()
	r.p.trace.mu.Lock()
	r.p.closes++
	r.p.trace.mu.Unlock()
}
