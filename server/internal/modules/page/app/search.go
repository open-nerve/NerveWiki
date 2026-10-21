package app

import (
	"cmp"
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// SearchRequest is a search's parameters (M8/P5 design 3.6): q, the
// notebook it is limited to, the cursor of a page after the first, and the
// page's size.
type SearchRequest struct {
	Q          *string
	NotebookID *uuid.UUID
	Cursor     *string
	Limit      *int
}

// SearchHit is a node a search found: where it is, root first, its name's
// hits, a page's content's excerpt around its first hit (nil when the
// content holds none), when the node or its content last changed, an
// attachment's type and size, and the tier that found it, which a page of
// hits cut short rebuilds its cursor from (M9 design 4.7).
type SearchHit struct {
	Node       SearchNode
	Ancestors  []domain.Ancestor
	TitleHits  []domain.TextRange
	Excerpt    *domain.SearchExcerpt
	ChangedAt  time.Time
	Attachment *AttachmentInfo
	Tier       int
}

// Cursor is the cursor of the page after h: below its tier, time and id,
// as Execute's next_cursor after the limit-th hit (M9/P4 design 3.8).
func (h SearchHit) Cursor() (string, error) {
	return shared.EncodeCursor(domain.SearchCursor{Tier: h.Tier, At: h.ChangedAt, ID: h.Node.ID})
}

// SearchPage is a page of a search's hits and the cursor of the next, ""
// after the last; the first page's count, nil on the later ones and when
// none passed yet; whether the notebooks hold nodes the job has not built
// yet, which no search finds.
type SearchPage struct {
	Hits       []SearchHit
	NextCursor string
	Count      *domain.SearchCount
	Building   bool
}

// SearchDeps are what a search reads with; Notebooks is InNotebook's way
// to a notebook's workspace. Walk and CountScan are domain.MaxSearchWalk
// and MaxSearchCountScan when 0, which production leaves them.
type SearchDeps struct {
	Snapshots   shared.Snapshots
	Workspaces  Workspaces
	Notebooks   Notebooks
	Visible     VisibleNotebooks
	Auth        shared.Authorizer
	Reads       SearchReads
	Trash       Trash
	Attachments AttachmentInfos
	Walk        int
	CountScan   int
}

// Search searches the nodes of a workspace's notebooks the caller sees:
// GET /api/v0/workspaces/{slug}/search (M8/P5 design 3.5, 3.6).
type Search struct {
	d SearchDeps
}

// NewSearch returns the use case.
func NewSearch(d SearchDeps) *Search {
	d.Walk = cmp.Or(d.Walk, domain.MaxSearchWalk)
	d.CountScan = cmp.Or(d.CountScan, domain.MaxSearchCountScan)
	return &Search{d: d}
}

// Execute returns the page of the search r of the workspace of slug: the
// cursor that does not decode 400 bad_request; q 422 (domain.ParseSearchQuery),
// before the workspace is looked for, as its text says nothing of it; no
// such workspace, or one the caller cannot see, workspace.not_found
// (page.search: any member); a notebook_id the caller does not see
// notebook.not_found; then the limit, 422 outside 1–MaxSearchLimit. With
// no notebook to see it answers an empty page, counted exact. The tiers,
// the first page's count, building and the hits' paths are read in one
// read-only snapshot, so that a rename cannot list a node twice in a page,
// after the terms' texts in contents when some term's differ (inContents);
// the hits' ranges and the attachments' infos after it.
func (s *Search) Execute(ctx context.Context, slug string, r SearchRequest) (SearchPage, error) {
	a, err := parsed(r)
	if err != nil {
		return SearchPage{}, err
	}
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return SearchPage{}, err
	}
	workspaceID, ok, err := s.d.Workspaces.FindBySlug(ctx, slug)
	switch {
	case err != nil:
		return SearchPage{}, err
	case !ok:
		return SearchPage{}, domain.ErrWorkspaceNotFound
	}
	grant, err := authorize(ctx, s.d.Auth, actor, domain.ActionSearch, shared.Target{WorkspaceID: workspaceID},
		domain.ErrWorkspaceNotFound)
	if err != nil {
		return SearchPage{}, err
	}
	notebooks, err := s.d.Visible.VisibleIn(ctx, workspaceID, actor.UserID, grant.WorkspaceRole)
	if err != nil {
		return SearchPage{}, err
	}
	if r.NotebookID != nil {
		if !slices.Contains(notebooks, *r.NotebookID) {
			return SearchPage{}, domain.ErrNotebookNotFound
		}
		notebooks = []uuid.UUID{*r.NotebookID}
	}
	limit, err := domain.SearchLimit(r.Limit)
	if err != nil {
		return SearchPage{}, err
	}
	return s.run(ctx, notebooks, a, limit)
}

// InNotebook returns the page of the search r of the notebook notebookID
// alone, as an MCP call asks for it (M9 design 4.13; M9/P4 design 3.8):
// the cursor, 400; q, 422; the limit, 422; then the notebook's workspace
// (Notebooks.WorkspaceOf), page.search there and the notebooks the caller
// sees in it, which must hold notebookID: no such notebook, a deleted
// one, one of a workspace the caller is not a member of and one there it
// does not see are all notebook.not_found, never workspace.not_found, the
// zero id without a read; then Execute's reads, in one read-only
// snapshot, its scope notebookID. r.NotebookID is not read.
func (s *Search) InNotebook(ctx context.Context, notebookID uuid.UUID, r SearchRequest) (SearchPage, error) {
	a, err := parsed(r)
	if err != nil {
		return SearchPage{}, err
	}
	limit, err := domain.SearchLimit(r.Limit)
	if err != nil {
		return SearchPage{}, err
	}
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return SearchPage{}, err
	}
	if notebookID == (uuid.UUID{}) {
		return SearchPage{}, domain.ErrNotebookNotFound
	}
	workspaceID, ok, err := s.d.Notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return SearchPage{}, err
	case !ok:
		return SearchPage{}, domain.ErrNotebookNotFound
	}
	grant, err := authorize(ctx, s.d.Auth, actor, domain.ActionSearch, shared.Target{WorkspaceID: workspaceID},
		domain.ErrNotebookNotFound)
	if err != nil {
		return SearchPage{}, err
	}
	notebooks, err := s.d.Visible.VisibleIn(ctx, workspaceID, actor.UserID, grant.WorkspaceRole)
	if err != nil {
		return SearchPage{}, err
	}
	if !slices.Contains(notebooks, notebookID) {
		return SearchPage{}, domain.ErrNotebookNotFound
	}
	return s.run(ctx, []uuid.UUID{notebookID}, a, limit)
}

// searchAsked is a search's request read: the position its page goes on
// below, whether it is the first page, and its query.
type searchAsked struct {
	below domain.SearchCursor
	first bool
	q     domain.SearchQuery
}

// parsed is r's cursor, 400 when it does not decode or names no tier, and
// its q, 422 (domain.ParseSearchQuery).
func parsed(r SearchRequest) (searchAsked, error) {
	a := searchAsked{below: domain.SearchFrom(domain.SearchTierTitle), first: r.Cursor == nil}
	if !a.first {
		if err := shared.DecodeCursor(*r.Cursor, &a.below); err != nil {
			return searchAsked{}, err
		}
		if !a.below.Valid() {
			return searchAsked{}, shared.InvalidCursor()
		}
	}
	var err error
	if a.q, err = domain.ParseSearchQuery(r.Q); err != nil {
		return searchAsked{}, err
	}
	return a, nil
}

// run reads the page of the search a in notebooks: with none, an empty
// page, counted exact on the first; else the tiers, the count, building
// and the hits' paths in one read-only snapshot, the terms' texts in
// contents first, then the hits' ranges and the attachments' infos.
func (s *Search) run(ctx context.Context, notebooks []uuid.UUID, a searchAsked, limit int) (SearchPage, error) {
	if len(notebooks) == 0 {
		page := SearchPage{Hits: []SearchHit{}}
		if a.first {
			page.Count = &domain.SearchCount{Kind: domain.CountExact}
		}
		return page, nil
	}
	q := a.q
	var found searchFound
	err := s.d.Snapshots.WithinSnapshot(ctx, func(ctx context.Context) error {
		var err error
		if q, err = s.inContents(ctx, q); err != nil {
			return err
		}
		found, err = s.read(ctx, SearchScope{Notebooks: notebooks, Query: q, Below: a.below}, limit, a.first)
		return err
	})
	if err != nil {
		return SearchPage{}, err
	}
	return s.page(ctx, q, found)
}

// inContents is q with the texts its terms are matched with in contents:
// PostgreSQL's NFC of those Go's NFC changed (domain.SearchQuery.Written),
// read in one statement; q itself, without one, for a query typed in NFC,
// most of them (M8 Codex review R2; fix check 1, C1-1).
func (s *Search) inContents(ctx context.Context, q domain.SearchQuery) (domain.SearchQuery, error) {
	written := q.Written()
	if len(written) == 0 {
		return q, nil
	}
	nfc, err := s.d.Reads.SearchNFC(ctx, written)
	if err != nil {
		return domain.SearchQuery{}, err
	}
	return q.InContents(nfc)
}

// searchFound is what a search read in its snapshot: the rows of its page,
// each with its tier, the next page's cursor, the count, building and the
// rows' lines.
type searchFound struct {
	rows     []tierRow
	next     *domain.SearchCursor
	count    *domain.SearchCount
	building bool
	lines    map[uuid.UUID][]domain.TrashAncestor
}

// tierRow is a row a tier found.
type tierRow struct {
	SearchTitleRow
	tier int
}

// read reads a page of the search s below its cursor, at most limit hits:
// the title tier's limit + 1, then, when they fall short, the body tier's
// walk from its top, on to limit + 1. The next page goes on below the
// limit-th hit when there is one more; else below the walk's last row when
// the walk left more to walk, which may leave this page short of limit,
// even empty; else there is none. The first page's count: exact when it
// is all there is, else estimated tier by tier (domain.EstimateCount),
// the body tier sampled when the page took none of it.
func (s *Search) read(ctx context.Context, scope SearchScope, limit int, first bool) (searchFound, error) {
	var out searchFound
	var tally domain.SearchTally
	counted := false
	// bodied is scope for the body tier's statements: its candidates by
	// their lexemes counted, once a read, when the first of them runs.
	bodied := func() (SearchScope, error) {
		if !counted {
			n, err := s.d.Reads.SearchHitCount(ctx, scope)
			if err != nil {
				return SearchScope{}, err
			}
			scope.Hits, counted = n, true
		}
		return scope, nil
	}
	if scope.Below.Tier == domain.SearchTierTitle {
		titles, err := s.d.Reads.SearchTitles(ctx, scope, limit+1)
		if err != nil {
			return searchFound{}, err
		}
		for _, t := range titles {
			out.rows = append(out.rows, tierRow{t, domain.SearchTierTitle})
		}
		scope.Below = domain.SearchFrom(domain.SearchTierBody)
	}
	var walkedTo *SearchBodyRow
	if len(out.rows) <= limit && scope.Query.Bodied() {
		b, err := bodied()
		if err != nil {
			return searchFound{}, err
		}
		want := limit + 1 - len(out.rows)
		bodies, err := s.d.Reads.SearchBodies(ctx, b, s.d.Walk, want)
		if err != nil {
			return searchFound{}, err
		}
		for i, b := range bodies {
			if b.Passed {
				out.rows = append(out.rows, tierRow{b.SearchTitleRow, domain.SearchTierBody})
			}
			walkedTo = &bodies[i]
		}
		tally.Checked, tally.Passed, tally.Ended = walkTally(bodies)
		tally.Full = tally.Passed == want
	}
	tally.Found = len(out.rows)
	switch {
	case len(out.rows) > limit:
		last := out.rows[limit-1]
		out.next = &domain.SearchCursor{Tier: last.tier, At: last.SortAt, ID: last.Node.ID}
		out.rows = out.rows[:limit]
	case walkedTo != nil && walkedTo.Walked > s.d.Walk:
		out.next = &domain.SearchCursor{Tier: domain.SearchTierBody, At: walkedTo.SortAt, ID: walkedTo.Node.ID}
	}
	if first {
		count, err := s.count(ctx, scope, bodied, tally, out)
		if err != nil {
			return searchFound{}, err
		}
		out.count = count
	}
	var err error
	if out.building, err = s.d.Reads.SearchBuilding(ctx, scope.Notebooks); err != nil {
		return searchFound{}, err
	}
	if len(out.rows) > 0 {
		ids := make([]uuid.UUID, len(out.rows))
		for i, r := range out.rows {
			ids[i] = r.Node.ID
		}
		if out.lines, err = s.d.Trash.TrashLines(ctx, ids); err != nil {
			return searchFound{}, err
		}
	}
	return out, nil
}

// count is the first page's count: exact without a next page; else the
// title tier counted, and, unless its count settles the estimate, the body
// tier's candidates, at the rate of the page's walk from its top, or, when
// the page walked none of them, the title tier filling it, of a sample
// from its top (bodied: the scope of its statements).
//
// A walk that ran, however short, is the rate's: no sample goes on after
// it. The sample is the body tier's statement again, which reads, joins
// and sorts every candidate before it checks one: as long as a whole walk
// however few it checks. At 300,000 pages that is 160 to 250 ms for
// 参考文献 in all notebooks, whose first page went from 258 ms to some
// 420 (M8/P5A fix check I-1), for the same at_least 1000. A walk the full
// page stopped is rated without its last pass instead (EstimateCount).
func (s *Search) count(ctx context.Context, scope SearchScope, bodied func() (SearchScope, error), tally domain.SearchTally,
	out searchFound) (*domain.SearchCount, error) {
	if out.next == nil {
		return &domain.SearchCount{Value: len(out.rows), Kind: domain.CountExact}, nil
	}
	tally.Bodied = scope.Query.Bodied()
	var err error
	if tally.Titles, tally.TitlePages, err = s.d.Reads.SearchTitleCount(ctx, scope, domain.MaxSearchCount+1); err != nil {
		return nil, err
	}
	if !tally.Bodied || tally.Titles > domain.MaxSearchCount {
		return domain.EstimateCount(tally), nil
	}
	if scope, err = bodied(); err != nil {
		return nil, err
	}
	if tally.Bodies, err = s.d.Reads.SearchBodyCount(ctx, scope, s.d.CountScan); err != nil {
		return nil, err
	}
	tally.BodiesCapped = tally.Bodies >= s.d.CountScan
	if tally.Checked == 0 && !tally.Ended { // the page's walk did not run: a walk of no candidate ended
		scope.Below = domain.SearchFrom(domain.SearchTierBody)
		sample, err := s.d.Reads.SearchBodies(ctx, scope, domain.SearchCountSample, domain.SearchCountSample)
		if err != nil {
			return nil, err
		}
		tally.Checked, tally.Passed, tally.Ended = walkTally(sample)
	}
	return domain.EstimateCount(tally), nil
}

// walkTally is what a walk of the body tier tells the count: how many of
// its candidates it checked, how many passed, and whether it checked them
// all, its last row the last of them; a walk of no row found none to
// check.
func walkTally(rows []SearchBodyRow) (checked, passed int, ended bool) {
	if len(rows) == 0 {
		return 0, 0, true
	}
	for _, r := range rows {
		if r.Passed {
			passed++
		}
	}
	last := rows[len(rows)-1]
	return last.Seq, passed, last.Seq == last.Walked
}

// page is the hits of what the search found, with their ranges, excerpts,
// lines and attachments' infos.
func (s *Search) page(ctx context.Context, q domain.SearchQuery, found searchFound) (SearchPage, error) {
	var assets []uuid.UUID
	for _, r := range found.rows {
		if r.Node.Kind == domain.KindAsset {
			assets = append(assets, r.Node.ID)
		}
	}
	infos := map[uuid.UUID]AttachmentInfo{}
	if len(assets) > 0 {
		var err error
		if infos, err = s.d.Attachments.Of(ctx, assets); err != nil {
			return SearchPage{}, err
		}
	}
	out := SearchPage{Hits: make([]SearchHit, len(found.rows)), Count: found.count, Building: found.building}
	for i, r := range found.rows {
		line := found.lines[r.Node.ID]
		h := SearchHit{Node: r.Node, Ancestors: make([]domain.Ancestor, len(line)), TitleHits: q.TitleHits(r.Node.Name),
			ChangedAt: r.SortAt, Tier: r.tier}
		for j, a := range line {
			h.Ancestors[j] = domain.Ancestor{ID: a.ID, Name: a.Name}
		}
		if r.At > 0 {
			e := q.Excerpt(r.Excerpt, r.At, r.Chars)
			h.Excerpt = &e
		}
		if info, ok := infos[r.Node.ID]; ok {
			h.Attachment = &info
		}
		out.Hits[i] = h
	}
	if found.next != nil {
		var err error
		if out.NextCursor, err = shared.EncodeCursor(*found.next); err != nil {
			return SearchPage{}, err
		}
	}
	return out, nil
}
