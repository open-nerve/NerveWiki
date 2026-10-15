package app_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func ptr[T any](v T) *T { return &v }

// fakeReads are a search's tiers in memory, newest first: the title
// tier's rows, and the body tier's candidates, each passing its check or
// not, which SearchBodies walks as the statement does, their count
// SearchHitCount's; and the counts the count's statements answer.
type fakeReads struct {
	*recorder
	titles   []app.SearchTitleRow
	bodies   []fakeBody
	counts   fakeCounts
	building bool
	scopes   []app.SearchScope
}

type fakeCounts struct{ titles, titlePages, bodies int }

type fakeBody struct {
	row    app.SearchTitleRow
	passes bool
}

// below reports whether r is below the cursor's position.
func below(r app.SearchTitleRow, c domain.SearchCursor) bool {
	return r.SortAt.Before(c.At) || r.SortAt.Equal(c.At) && r.Node.ID.Compare(c.ID) < 0
}

func (f *fakeReads) SearchTitles(ctx context.Context, s app.SearchScope, limit int) ([]app.SearchTitleRow, error) {
	f.record(ctx, fmt.Sprintf("SearchTitles %d", limit))
	f.scopes = append(f.scopes, s)
	var out []app.SearchTitleRow
	for _, r := range f.titles {
		if below(r, s.Below) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReads) SearchHitCount(ctx context.Context, _ domain.SearchQuery) (int, error) {
	f.record(ctx, "SearchHitCount")
	return len(f.bodies), nil
}

func (f *fakeReads) SearchBodies(ctx context.Context, s app.SearchScope, walk, limit int) ([]app.SearchBodyRow, error) {
	f.record(ctx, fmt.Sprintf("SearchBodies %d %d of %d", walk, limit, s.Hits))
	f.scopes = append(f.scopes, s)
	var walked []fakeBody
	for _, b := range f.bodies {
		if below(b.row, s.Below) && len(walked) < walk+1 {
			walked = append(walked, b)
		}
	}
	var out []app.SearchBodyRow
	for i, b := range walked {
		row := app.SearchBodyRow{SearchTitleRow: b.row, Seq: i + 1, Walked: len(walked), Passed: b.passes && i < walk}
		if row.Passed || row.Seq == min(len(walked), walk) {
			out = append(out, row)
		}
	}
	passed := 0
	for i, r := range out {
		if r.Passed {
			passed++
		}
		if i+1 == limit || passed == limit {
			return out[:i+1], nil
		}
	}
	return out, nil
}

func (f *fakeReads) SearchTitleCount(ctx context.Context, _ app.SearchScope, limit int) (int, int, error) {
	f.record(ctx, fmt.Sprintf("SearchTitleCount %d", limit))
	return f.counts.titles, f.counts.titlePages, nil
}

func (f *fakeReads) SearchBodyCount(ctx context.Context, s app.SearchScope, limit int) (int, error) {
	f.record(ctx, fmt.Sprintf("SearchBodyCount %d of %d", limit, s.Hits))
	return f.counts.bodies, nil
}

func (f *fakeReads) SearchBuilding(ctx context.Context, _ []uuid.UUID) (bool, error) {
	f.record(ctx, "SearchBuilding")
	return f.building, nil
}

// fakeSnapshots runs fn in a context the recorder sees as a transaction.
type fakeSnapshots struct{ *recorder }

func (f fakeSnapshots) WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error {
	f.record(ctx, "WithinSnapshot")
	return fn(context.WithValue(ctx, txKey{}, true))
}

// fakeVisible sees the notebooks listed.
type fakeVisible struct {
	*recorder
	ids []uuid.UUID
}

func (f fakeVisible) VisibleIn(ctx context.Context, _, _ uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error) {
	f.record(ctx, "VisibleIn "+string(role))
	return f.ids, nil
}

// searchFixture is a search of acme, whose notebooks eng and ops alice
// sees, with reads and attachments of its own.
type searchFixture struct {
	*fixture
	reads   *fakeReads
	visible fakeVisible
	infos   fakeAttachments
	ops     uuid.UUID
	walk    int
	// countScan is SearchDeps.CountScan: 0, its default, unless a test
	// sets it.
	countScan int
}

func newSearchFixture() *searchFixture {
	f := newFixture()
	s := &searchFixture{fixture: f, reads: &fakeReads{recorder: f.rec}, ops: uuid.NewV7(), walk: 4,
		infos: fakeAttachments{recorder: f.rec, infos: map[uuid.UUID]app.AttachmentInfo{}}}
	s.visible = fakeVisible{recorder: f.rec, ids: []uuid.UUID{f.eng, s.ops}}
	f.workspaces.slugs["acme"] = f.acme
	f.grant(domain.ActionSearch)
	return s
}

func (s *searchFixture) search(r app.SearchRequest) (app.SearchPage, error) {
	return app.NewSearch(app.SearchDeps{Snapshots: fakeSnapshots{s.rec}, Workspaces: s.workspaces, Visible: s.visible, Auth: s.auth,
		Reads: s.reads, Trash: s.store, Attachments: s.infos, Walk: s.walk, CountScan: s.countScan}).Execute(s.asAlice(), "acme", r)
}

// row is a node of eng found, changed minutes ago.
func (s *searchFixture) row(name string, minutes int, kind domain.Kind) app.SearchTitleRow {
	return app.SearchTitleRow{Node: app.SearchNode{ID: uuid.NewV7(), NotebookID: s.eng, Kind: kind, Name: name},
		SortAt: now().Add(-time.Duration(minutes) * time.Minute)}
}

// The codes come in the design's order (M8/P5 design 3.6): the cursor, q,
// the workspace, the decision, the notebook, the limit.
func TestTheSearchAnswersItsCodesInOrder(t *testing.T) {
	cursor := func(c domain.SearchCursor) *string {
		s, err := shared.EncodeCursor(c)
		if err != nil {
			t.Fatal(err)
		}
		return &s
	}
	other := uuid.NewV7()
	for _, tt := range []struct {
		name  string
		setup func(s *searchFixture)
		r     app.SearchRequest
		want  string
	}{
		{"a cursor that does not decode, before q", nil, app.SearchRequest{Cursor: ptr("x"), Q: ptr("")}, "bad_request"},
		{"a cursor of no tier", nil, app.SearchRequest{Cursor: cursor(domain.SearchCursor{Tier: 2, At: now().UTC()}), Q: ptr("a")},
			"bad_request"},
		{"q, before the workspace", func(s *searchFixture) { delete(s.workspaces.slugs, "acme") }, app.SearchRequest{Q: ptr(" ")},
			"validation_failed q"},
		{"no such workspace", func(s *searchFixture) { delete(s.workspaces.slugs, "acme") }, app.SearchRequest{Q: ptr("a")},
			"workspace.not_found"},
		{"a workspace the caller is no member of", func(s *searchFixture) { s.auth.grants = map[shared.Action]bool{} },
			app.SearchRequest{Q: ptr("a")}, "workspace.not_found"},
		{"a notebook the caller does not see, before the limit", nil,
			app.SearchRequest{Q: ptr("a"), NotebookID: &other, Limit: ptr(0)}, "notebook.not_found"},
		{"a limit past the bound", nil, app.SearchRequest{Q: ptr("a"), Limit: ptr(domain.MaxSearchLimit + 1)}, "validation_failed limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSearchFixture()
			if tt.setup != nil {
				tt.setup(s)
			}
			_, err := s.search(tt.r)
			if got := codeOf(err); got != tt.want {
				t.Errorf("Execute() = %v, want %s", err, tt.want)
			}
			if s.called("WithinSnapshot") {
				t.Error("refused after it read the tiers")
			}
		})
	}
}

// A workspace's search reads the notebooks the caller sees there, by the
// role the decision gave; a notebook_id one of them alone. With none to
// see, the page is empty, exact, and nothing is read.
func TestTheSearchReadsTheNotebooksSeen(t *testing.T) {
	s := newSearchFixture()
	if _, err := s.search(app.SearchRequest{Q: ptr("a"), NotebookID: &s.ops}); err != nil {
		t.Fatal(err)
	}
	if len(s.reads.scopes) == 0 || !slices.Equal(s.reads.scopes[0].Notebooks, []uuid.UUID{s.ops}) ||
		!s.called("VisibleIn member") || !s.called("Authorize page.search") {
		t.Errorf("scopes %+v after %v: want ops alone, of the member's notebooks", s.reads.scopes, s.rec.calls)
	}
	if got := s.auth.targets; len(got) != 1 || got[0] != (shared.Target{WorkspaceID: s.acme}) {
		t.Errorf("decided on %+v, want the workspace alone", got)
	}
	s = newSearchFixture()
	s.visible.ids = nil
	page, err := s.search(app.SearchRequest{Q: ptr("a")})
	if err != nil || len(page.Hits) != 0 || page.Hits == nil || page.Count == nil || *page.Count != (domain.SearchCount{Kind: domain.CountExact}) ||
		page.NextCursor != "" || page.Building || s.called("WithinSnapshot") {
		t.Errorf("Execute() = %+v, %v after %v; want an empty page counted exact 0, nothing read", page, err, s.rec.calls)
	}
}

// The tiers (M8/P5 design 3.5): the title tier's limit + 1 first; the body
// tier from its top when they fall short. The next page goes on below the
// limit-th hit; below the walk's last row when the walk leaves more; none
// when the walk ends with the candidates. The first page alone is
// counted: exact when it holds every hit.
func TestTheSearchGoesThroughItsTiers(t *testing.T) {
	type want struct {
		names []string
		next  *domain.SearchCursor
		count *domain.SearchCount
		calls []string
		// sampled is where the count's sample went on from, if it took one.
		sampled *domain.SearchCursor
	}
	for _, tt := range []struct {
		name  string
		setup func(s *searchFixture) want
	}{
		{"the title tier fills the page", func(s *searchFixture) want {
			for i := range 4 {
				s.reads.titles = append(s.reads.titles, s.row(fmt.Sprintf("a %d", i), i, domain.KindPage))
			}
			s.reads.bodies = []fakeBody{{s.row("b", 10, domain.KindPage), true}}
			s.reads.counts = fakeCounts{titles: 4, titlePages: 4, bodies: 5}
			third := s.reads.titles[2]
			return want{names: []string{"a 0", "a 1", "a 2"},
				next: &domain.SearchCursor{Tier: domain.SearchTierTitle, At: third.SortAt, ID: third.Node.ID},
				// The body tier sampled from its top: 1 of 1 passed, 5
				// candidates less the 4 title pages.
				count: &domain.SearchCount{Value: 5, Kind: domain.CountAbout},
				calls: []string{"SearchTitles 4", fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), "SearchHitCount",
					fmt.Sprintf("SearchBodyCount %d of 1", domain.MaxSearchCountScan),
					fmt.Sprintf("SearchBodies %d %d of 1", domain.SearchCountSample, domain.SearchCountSample)}}
		}},
		{"the names past the count's bound", func(s *searchFixture) want {
			for i := range 4 {
				s.reads.titles = append(s.reads.titles, s.row(fmt.Sprintf("a %d", i), i, domain.KindPage))
			}
			s.reads.bodies = []fakeBody{{s.row("b", 10, domain.KindPage), true}}
			s.reads.counts = fakeCounts{titles: domain.MaxSearchCount + 1, titlePages: 4}
			third := s.reads.titles[2]
			// The count is settled: the body tier neither counted nor
			// sampled.
			return want{names: []string{"a 0", "a 1", "a 2"},
				next:  &domain.SearchCursor{Tier: domain.SearchTierTitle, At: third.SortAt, ID: third.Node.ID},
				count: &domain.SearchCount{Value: domain.MaxSearchCount, Kind: domain.CountAtLeast},
				calls: []string{"SearchTitles 4", fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1)}}
		}},
		{"the body tier fills it", func(s *searchFixture) want {
			s.reads.titles = []app.SearchTitleRow{s.row("a", 1, domain.KindAsset)}
			s.reads.bodies = []fakeBody{{s.row("b", 2, domain.KindPage), true}, {s.row("c", 3, domain.KindPage), false},
				{s.row("d", 4, domain.KindPage), true}, {s.row("e", 5, domain.KindPage), true}}
			s.reads.counts = fakeCounts{titles: 1, bodies: 4}
			d := s.reads.bodies[2].row
			return want{names: []string{"a", "b", "d"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: d.SortAt, ID: d.Node.ID},
				count: &domain.SearchCount{Value: 4, Kind: domain.CountAbout},
				// The candidates counted once, for both statements.
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 3 of 4",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 4", domain.MaxSearchCountScan)}}
		}},
		{"the walk leaves more to walk", func(s *searchFixture) want {
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), i == 1})
			}
			s.reads.counts = fakeCounts{bodies: 6}
			fourth := s.reads.bodies[3].row
			below := &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID}
			// The walk checked 4 candidates, fewer than the sample, and left
			// some: the sample goes on below it, to the 6, 1 passing in all.
			return want{names: []string{"1"}, next: below, count: &domain.SearchCount{Value: 1, Kind: domain.CountAbout},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 6", domain.MaxSearchCountScan),
					fmt.Sprintf("SearchBodies %d %d of 6", domain.SearchCountSample-4, domain.SearchCountSample-4)}, sampled: below}
		}},
		{"the walk finds none yet", func(s *searchFixture) want {
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), false})
			}
			s.reads.counts = fakeCounts{bodies: 6}
			fourth := s.reads.bodies[3].row
			return want{names: []string{}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 6", domain.MaxSearchCountScan),
					fmt.Sprintf("SearchBodies %d %d of 6", domain.SearchCountSample-4, domain.SearchCountSample-4)}}
		}},
		{"the candidates counted to the search's bound", func(s *searchFixture) want {
			s.countScan = 6
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), i == 1 || i == 2})
			}
			s.reads.counts = fakeCounts{bodies: 6}
			fourth := s.reads.bodies[3].row
			// The sample's rate, 2 of 6, of the 6 candidates counted, the
			// bound: at least.
			return want{names: []string{"1", "2"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				count: &domain.SearchCount{Value: 2, Kind: domain.CountAtLeast},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), "SearchBodyCount 6 of 6",
					fmt.Sprintf("SearchBodies %d %d of 6", domain.SearchCountSample-4, domain.SearchCountSample-4)}}
		}},
		{"the walk ends with the candidates", func(s *searchFixture) want {
			for i := range 4 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), i%2 == 0})
			}
			return want{names: []string{"0", "2"}, count: &domain.SearchCount{Value: 2, Kind: domain.CountExact},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 4"}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSearchFixture()
			w := tt.setup(s)
			page, err := s.search(app.SearchRequest{Q: ptr("a"), Limit: ptr(3)})
			if err != nil {
				t.Fatal(err)
			}
			names := []string{}
			for _, h := range page.Hits {
				names = append(names, h.Node.Name)
			}
			var next *domain.SearchCursor
			if page.NextCursor != "" {
				next = &domain.SearchCursor{}
				if err := shared.DecodeCursor(page.NextCursor, next); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(names, w.names) || fmt.Sprint(next) != fmt.Sprint(w.next) || fmt.Sprint(page.Count) != fmt.Sprint(w.count) {
				t.Errorf("Execute() = %v, next %v, count %v; want %v, %v, %v", names, next, page.Count, w.names, w.next, w.count)
			}
			var reads []string
			for _, c := range s.rec.calls {
				if read, ok := strings.CutSuffix(c, " in tx"); ok && strings.HasPrefix(c, "Search") && !strings.HasPrefix(c, "SearchBuilding") {
					reads = append(reads, read)
				} else if strings.HasPrefix(c, "Search") && !strings.HasPrefix(c, "SearchBuilding") {
					reads = append(reads, c+" outside the snapshot")
				}
			}
			if !slices.Equal(reads, w.calls) {
				t.Errorf("reads %v, want %v, each in the snapshot", reads, w.calls)
			}
			if sampled := s.reads.scopes[len(s.reads.scopes)-1].Below; w.sampled != nil && sampled != *w.sampled {
				t.Errorf("the sample went on below %+v, want below the walk's last row, %+v", sampled, *w.sampled)
			}
			if next == nil {
				return
			}
			// The next page goes on below the cursor, uncounted.
			cur := page.NextCursor
			later, err := s.search(app.SearchRequest{Q: ptr("a"), Limit: ptr(3), Cursor: &cur})
			if err != nil || later.Count != nil {
				t.Errorf("the next page = %+v, %v; want no count", later, err)
			}
			last := s.reads.scopes[len(s.reads.scopes)-1]
			switch {
			case next.Tier == domain.SearchTierBody && last.Below != *next:
				t.Errorf("the next page read below %+v, want %+v", last.Below, *next)
			case next.Tier == domain.SearchTierTitle && last.Below != domain.SearchFrom(domain.SearchTierBody):
				// The title tier ended on it: the body tier from its top.
				t.Errorf("the next page read the body tier below %+v, want from its top", last.Below)
			}
		})
	}
}

// The first page's count when the title tier nearly fills the page, the
// body tier's walk stopped at its first passes (M8/P5A review C-M1): the
// rate of SearchCountSample candidates, the sample going on below the
// walk, not of one or two; one of 2,000 candidates in 50 passing, the
// newest among them, the page of 20.
func TestTheCountOfAShortWalkRestsOnTheSample(t *testing.T) {
	for _, tt := range []struct {
		titles int
		sample string // the sample's read, if any
		want   domain.SearchCount
	}{
		{20, "SearchBodies 99 99 of 2000", domain.SearchCount{Value: 60, Kind: domain.CountAbout}}, // 1 + 1 of 1 + 99 pass: true 60
		{19, "SearchBodies 49 49 of 2000", domain.SearchCount{Value: 59, Kind: domain.CountAbout}}, // 2 + 0 of 51 + 49: true 59
		{18, "", domain.SearchCount{Value: 77, Kind: domain.CountAbout}},                           // the walk's 3 of 101: true 58
		{15, "", domain.SearchCount{Value: 62, Kind: domain.CountAbout}},                           // 6 of 251: true 55
		{10, "", domain.SearchCount{Value: 50, Kind: domain.CountAbout}},                           // 10 of 500: true 50
		{0, "", domain.SearchCount{Value: 40, Kind: domain.CountAbout}},                            // 10 of 500: true 40
	} {
		s := newSearchFixture()
		s.walk = domain.MaxSearchWalk
		for i := range tt.titles {
			s.reads.titles = append(s.reads.titles, s.row(fmt.Sprintf("a %d", i), i, domain.KindPage))
		}
		const candidates = 2000
		for i := range candidates {
			s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprintf("b %d", i), 100+i, domain.KindPage), i%50 == 0})
		}
		s.reads.counts = fakeCounts{titles: tt.titles, titlePages: tt.titles, bodies: candidates}
		page, err := s.search(app.SearchRequest{Q: ptr("a"), Limit: ptr(20)})
		if err != nil {
			t.Fatal(err)
		}
		var bodies []string // the walk's read, then the sample's
		for _, c := range s.rec.calls {
			if read, ok := strings.CutSuffix(c, " in tx"); ok && strings.HasPrefix(read, "SearchBodies ") {
				bodies = append(bodies, read)
			}
		}
		sample := ""
		if len(bodies) == 2 {
			sample = bodies[1]
		}
		if page.Count == nil || *page.Count != tt.want || len(bodies) > 2 || sample != tt.sample {
			t.Errorf("%d titles: count %v, the walk and the sample %q; want %v, the sample %q", tt.titles, page.Count, bodies, tt.want,
				tt.sample)
		}
	}
}

// A hit has its path, root first, its name's hits, a page's excerpt around
// its first hit in the content, when it last changed, and an attachment's
// type and size; the page tells whether the index is building.
func TestTheSearchAssemblesItsHits(t *testing.T) {
	s := newSearchFixture()
	root := s.page("Root", nil, 0)
	parent := s.page("Parent", &root.ID, 0)
	asset := s.row("Go 报告.pdf", 1, domain.KindAsset)
	s.store.nodes[asset.Node.ID] = domain.Node{ID: asset.Node.ID, NotebookID: s.eng, ParentID: &parent.ID, Kind: domain.KindAsset,
		Name: asset.Node.Name}
	s.infos.infos[asset.Node.ID] = app.AttachmentInfo{MIME: "application/pdf", Bytes: 42}
	page := s.row("Notes", 2, domain.KindPage)
	page.At, page.Excerpt, page.Chars = 3, "学Go语言", 5
	s.reads.titles = []app.SearchTitleRow{asset}
	s.reads.bodies = []fakeBody{{page, true}}
	s.reads.building = true
	got, err := s.search(app.SearchRequest{Q: ptr("go")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 2 || !got.Building {
		t.Fatalf("Execute() = %+v, want two hits, building", got)
	}
	a, p := got.Hits[0], got.Hits[1]
	if len(a.Ancestors) != 2 || a.Ancestors[0].ID != root.ID || a.Ancestors[1].ID != parent.ID || a.Attachment == nil ||
		*a.Attachment != (app.AttachmentInfo{MIME: "application/pdf", Bytes: 42}) || a.Excerpt != nil ||
		!slices.Equal(a.TitleHits, []domain.TextRange{{Start: 0, End: 2}}) || !a.ChangedAt.Equal(asset.SortAt) {
		t.Errorf("the attachment's hit = %+v", a)
	}
	if p.Excerpt == nil || p.Excerpt.Text != "学Go语言" || !slices.Equal(p.Excerpt.Hits, []domain.TextRange{{Start: 1, End: 3}}) ||
		p.Attachment != nil || len(p.TitleHits) != 0 {
		t.Errorf("the page's hit = %+v, excerpt %+v", p, p.Excerpt)
	}
}
