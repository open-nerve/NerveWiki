package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The full-text search's rows and queue (M8/P5 design 3.4, 3.5).

// statementError is a search statement's error as it goes up: its SQLSTATE
// alone, or, without one (an argument pgx refused to encode), its type.
// PostgreSQL's message and pgx's would carry a pattern, a tsquery or a
// row's lexemes, the user's text, into the logs (M8/P5 design 3.6).
type statementError struct {
	what  string
	state string
	cause error // a context's end, which the caller tells apart
}

func (e *statementError) Error() string { return e.what + ": " + e.state }

func (e *statementError) Unwrap() error { return e.cause }

// stateOnly is err of the statement what as a statementError; nil for nil.
func stateOnly(what string, err error) error {
	if err == nil {
		return nil
	}
	e := &statementError{what: what}
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr):
		e.state = "SQLSTATE " + pgErr.Code
	case errors.Is(err, context.Canceled):
		e.state, e.cause = "canceled", context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		e.state, e.cause = "deadline exceeded", context.DeadlineExceeded
	default:
		e.state = fmt.Sprintf("%T", errors.Unwrap(err))
		if errors.Unwrap(err) == nil {
			e.state = fmt.Sprintf("%T", err)
		}
	}
	return e
}

// QueueSearch implements app.SearchQueue.
func (s *Store) QueueSearch(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, bulk bool) error {
	err := s.queries(ctx).QueueSearch(ctx, gen.QueueSearchParams{Ids: ids, NotebookID: notebookID, Bulk: bulk})
	if err != nil {
		return fmt.Errorf("queue nodes for the search: %w", err)
	}
	return nil
}

// QueueSearchOfNotebooks implements app.NotebookPages.
func (s *Store) QueueSearchOfNotebooks(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	err := s.queries(ctx).QueueSearchOfNotebooks(ctx, gen.QueueSearchOfNotebooksParams{NotebookIds: ids, DeletedAt: at})
	if err != nil {
		return fmt.Errorf("queue the deleted notebooks' nodes for the search: %w", err)
	}
	return nil
}

// searchReleaseTimeout bounds the release of the job's lock, and then the
// close of its connection: a connection that no longer answers (a
// half-open TCP) must not hold the queue's one worker past its run (M8/P5A
// review B-N3).
const searchReleaseTimeout = 5 * time.Second

// LockSearch implements app.SearchIndex: the lock is the session's, of a
// connection taken from the pool for the run, which the run's statements
// run on (postgres.OnConn): the lock is the run's own, and the run takes
// one connection (M8/P5A review B-M4, B-S3). release gives it back, or,
// when the unlock fails, closes the connection, which ends the session's
// lock, each within searchReleaseTimeout.
func (s *Store) LockSearch(ctx context.Context) (context.Context, func(), bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("take a connection for the search's job: %w", err)
	}
	ok, err := gen.New(conn).TrySearchLock(ctx)
	if err != nil || !ok {
		conn.Release()
		if err != nil {
			return nil, nil, false, fmt.Errorf("take the search's job's lock: %w", err)
		}
		return nil, nil, false, nil
	}
	release := func() {
		unlock, cancel := context.WithTimeout(context.WithoutCancel(ctx), searchReleaseTimeout)
		defer cancel()
		if err := gen.New(conn).ReleaseSearchLock(unlock); err != nil {
			closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), searchReleaseTimeout)
			defer cancel()
			_ = conn.Hijack().Close(closing)
			return
		}
		conn.Release()
	}
	return postgres.OnConn(ctx, conn), release, true, nil
}

// QueuedSearch implements app.SearchIndex.
func (s *Store) QueuedSearch(ctx context.Context, bulk bool, after app.QueuedNode, limit int) ([]app.QueuedNode, error) {
	var out []app.QueuedNode
	if bulk {
		rows, err := s.queries(ctx).QueuedSearchBulk(ctx, gen.QueuedSearchBulkParams{AfterNotebook: after.NotebookID, After: after.ID,
			MaxRows: int32(limit)})
		if err != nil {
			return nil, fmt.Errorf("read the search's queue: %w", err)
		}
		for _, r := range rows {
			out = append(out, app.QueuedNode{ID: r.NodeID, NotebookID: r.NotebookID, Version: r.Version})
		}
		return out, nil
	}
	rows, err := s.queries(ctx).QueuedSearch(ctx, gen.QueuedSearchParams{After: after.ID, MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("read the search's queue: %w", err)
	}
	for _, r := range rows {
		out = append(out, app.QueuedNode{ID: r.NodeID, NotebookID: r.NotebookID, Version: r.Version})
	}
	return out, nil
}

// SearchSources implements app.SearchIndex.
func (s *Store) SearchSources(ctx context.Context, ids []uuid.UUID) ([]app.SearchSource, error) {
	rows, err := s.queries(ctx).SearchSources(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("read the search's sources: %w", err)
	}
	out := make([]app.SearchSource, len(rows))
	for i, r := range rows {
		src := app.SearchSource{ID: r.ID, Gone: r.Gone}
		if r.Kind != nil {
			src.Kind, src.Name = domain.Kind(*r.Kind), *r.Name
		}
		if r.Revision != nil {
			src.Revision = int(*r.Revision)
		}
		if r.ByteSize != nil {
			src.Bytes = int(*r.ByteSize)
		}
		if r.BuiltName != nil {
			src.Built = &app.BuiltSearchRow{Name: *r.BuiltName, Tokenizer: *r.BuiltTokenizer, At: *r.BuiltAt}
			if r.BuiltRevision != nil {
				src.Built.Revision = int(*r.BuiltRevision)
			}
		}
		out[i] = src
	}
	return out, nil
}

// SearchContents implements app.SearchIndex.
func (s *Store) SearchContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.PageText, error) {
	out := map[uuid.UUID]app.PageText{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.queries(ctx).SearchContents(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("read the contents the search builds: %w", err)
	}
	for _, r := range rows {
		out[r.NodeID] = app.PageText{Content: r.Content, Revision: int(r.Revision)}
	}
	return out, nil
}

// WriteSearchRows implements app.SearchIndex: in ctx's transaction, the
// rows' notebooks' prefixes given first, the rows as a batch of upserts by
// node id, then the deletions. The GIN indexes keep no pending list
// (migration 00041): a row's keys go into them as it is written, and no
// setting of the server's speeds that up (M8 Codex review R1).
func (s *Store) WriteSearchRows(ctx context.Context, rows []app.SearchRow, deleted []uuid.UUID) error {
	if !postgres.InTx(ctx) {
		return errors.New("write the search's rows: outside a transaction, a row could be written without its notebook's prefix")
	}
	q := s.queries(ctx)
	if len(rows) > 0 {
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.NodeID
		}
		if err := q.TagSearchNotebooks(ctx, ids); err != nil {
			return stateOnly("tag the search's notebooks", err)
		}
		params := make([]gen.WriteSearchRowParams, len(rows))
		for i, r := range rows {
			params[i] = gen.WriteSearchRowParams{NodeID: r.NodeID, Name: r.Name, Tokenizer: r.Tokenizer, BuiltAt: r.BuiltAt,
				Title: r.Title, Body: r.Body}
			if r.Revision > 0 {
				rev := int32(r.Revision)
				params[i].Revision = &rev
			}
		}
		var first error
		q.WriteSearchRow(ctx, params).Exec(func(_ int, err error) {
			if first == nil {
				first = err
			}
		})
		if first != nil {
			return stateOnly("write the search's rows", first)
		}
	}
	if len(deleted) > 0 {
		if err := q.DeleteSearchRows(ctx, deleted); err != nil {
			return stateOnly("delete the search's rows", err)
		}
	}
	return nil
}

// DequeueSearch implements app.SearchIndex.
func (s *Store) DequeueSearch(ctx context.Context, nodes []app.QueuedNode) error {
	ids, versions := make([]uuid.UUID, len(nodes)), make([]int64, len(nodes))
	for i, n := range nodes {
		ids[i], versions[i] = n.ID, n.Version
	}
	if err := s.queries(ctx).DequeueSearch(ctx, gen.DequeueSearchParams{Ids: ids, Versions: versions}); err != nil {
		return fmt.Errorf("take nodes off the search's queue: %w", err)
	}
	return nil
}

// AnalyzeSearch implements app.SearchIndex.
func (s *Store) AnalyzeSearch(ctx context.Context) error {
	if err := s.queries(ctx).AnalyzeSearch(ctx); err != nil {
		return fmt.Errorf("analyze the search's rows: %w", err)
	}
	return nil
}

// StaleSearchNodes are the live nodes of a notebook after the node after,
// by id, at most limit, whose row is missing or of another tokenizer than
// tokenizer: nervewiki reindex's (M8/P5 design 3.4).
func (s *Store) StaleSearchNodes(ctx context.Context, notebookID, after uuid.UUID, tokenizer string, limit int) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).StaleSearchNodes(ctx, gen.StaleSearchNodesParams{NotebookID: notebookID, After: after,
		Tokenizer: tokenizer, MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("read the nodes the search's rows are stale of: %w", err)
	}
	return ids, nil
}

// searchArgs are a scope's arguments of the search's statements: the
// notebooks, the patterns of names and of contents, and the keys.
type searchArgs struct {
	notebooks                    []uuid.UUID
	terms, titled, bodied        []string
	inContents, bodiedInContents []string
	keys                         []string
	bodyKeys, titledKeys         []string
	hits                         int64
	belowAt                      time.Time
	belowID                      uuid.UUID
}

func argsOf(sc app.SearchScope) searchArgs {
	q := sc.Query
	return searchArgs{notebooks: sc.Notebooks, terms: q.Patterns(), titled: q.TitledPatterns(), bodied: q.BodiedPatterns(),
		inContents: q.ContentPatterns(), bodiedInContents: q.BodiedContentPatterns(), keys: q.TitleKeys(),
		bodyKeys: q.BodyKeys(), titledKeys: q.TitledKeys(), hits: int64(sc.Hits), belowAt: sc.Below.At, belowID: sc.Below.ID}
}

// SearchNFC implements app.SearchReads.
func (s *Store) SearchNFC(ctx context.Context, texts []string) ([]string, error) {
	nfc, err := s.queries(ctx).SearchNFC(ctx, texts)
	if err != nil {
		return nil, stateOnly("put the search's terms in NFC", err)
	}
	return nfc, nil
}

// SearchTitles implements app.SearchReads.
func (s *Store) SearchTitles(ctx context.Context, sc app.SearchScope, limit int) ([]app.SearchTitleRow, error) {
	a := argsOf(sc)
	rows, err := s.planned(ctx).SearchTitles(ctx, gen.SearchTitlesParams{Keys: a.keys, Notebooks: a.notebooks, BelowAt: a.belowAt,
		BelowID: a.belowID, Terms: a.terms, MaxRows: int32(limit), InContents: a.inContents})
	if err != nil {
		return nil, stateOnly("search the titles", err)
	}
	out := make([]app.SearchTitleRow, len(rows))
	for i, r := range rows {
		out[i] = app.SearchTitleRow{
			Node:   app.SearchNode{ID: r.ID, NotebookID: r.NotebookID, Kind: domain.Kind(r.Kind), Name: r.Name},
			SortAt: r.SortAt, At: int(r.At), Excerpt: r.Excerpt, Chars: int(r.Chars),
		}
	}
	return out, nil
}

// SearchHitCount implements app.SearchReads.
func (s *Store) SearchHitCount(ctx context.Context, sc app.SearchScope) (int, error) {
	q := sc.Query
	n, err := s.planned(ctx).SearchHitCount(ctx, gen.SearchHitCountParams{Notebooks: sc.Notebooks, BodyKeys: q.BodyKeys(),
		TitledKeys: q.TitledKeys()})
	if err != nil {
		return 0, stateOnly("count the search's candidates", err)
	}
	return int(n), nil
}

// SearchBodies implements app.SearchReads.
func (s *Store) SearchBodies(ctx context.Context, sc app.SearchScope, walk, limit int) ([]app.SearchBodyRow, error) {
	a := argsOf(sc)
	rows, err := s.planned(ctx).SearchBodies(ctx, gen.SearchBodiesParams{BodyKeys: a.bodyKeys, TitledKeys: a.titledKeys,
		Hits: a.hits, Notebooks: a.notebooks, BelowAt: a.belowAt, BelowID: a.belowID, Terms: a.terms, Titled: a.titled, Walk: int32(walk),
		Bodied: a.bodied, BodiedInContents: a.bodiedInContents, InContents: a.inContents, MaxRows: int32(limit)})
	if err != nil {
		return nil, stateOnly("search the contents", err)
	}
	out := make([]app.SearchBodyRow, len(rows))
	for i, r := range rows {
		out[i] = app.SearchBodyRow{
			SearchTitleRow: app.SearchTitleRow{
				Node:   app.SearchNode{ID: r.ID, NotebookID: r.NotebookID, Kind: domain.KindPage, Name: r.Name},
				SortAt: r.SortAt, At: int(r.At), Excerpt: r.Excerpt, Chars: int(r.Chars),
			},
			Seq: int(r.Seq), Walked: int(r.Walked), Passed: r.Passed,
		}
	}
	return out, nil
}

// SearchTitleCount implements app.SearchReads.
func (s *Store) SearchTitleCount(ctx context.Context, sc app.SearchScope, limit int) (int, int, error) {
	a := argsOf(sc)
	r, err := s.planned(ctx).SearchTitleCount(ctx, gen.SearchTitleCountParams{Keys: a.keys, Notebooks: a.notebooks,
		Terms: a.terms, Cap: int32(limit)})
	if err != nil {
		return 0, 0, stateOnly("count the search's titles", err)
	}
	return int(r.Titles), int(r.TitlePages), nil
}

// SearchBodyCount implements app.SearchReads.
func (s *Store) SearchBodyCount(ctx context.Context, sc app.SearchScope, limit int) (int, error) {
	a := argsOf(sc)
	n, err := s.planned(ctx).SearchBodyCount(ctx, gen.SearchBodyCountParams{BodyKeys: a.bodyKeys, TitledKeys: a.titledKeys,
		Hits: a.hits, Notebooks: a.notebooks, Cap: int32(limit)})
	if err != nil {
		return 0, stateOnly("count the search's contents", err)
	}
	return int(n), nil
}

// SearchBuilding implements app.SearchReads.
func (s *Store) SearchBuilding(ctx context.Context, notebooks []uuid.UUID) (bool, error) {
	building, err := s.planned(ctx).SearchBuilding(ctx, notebooks)
	if err != nil {
		return false, stateOnly("read whether the search is building", err)
	}
	return building, nil
}
