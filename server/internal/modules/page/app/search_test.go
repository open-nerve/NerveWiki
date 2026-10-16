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
	// nfc is SearchNFC's text of each it is given; the text itself for
	// one it does not have.
	nfc map[string]string
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

func (f *fakeReads) SearchNFC(ctx context.Context, texts []string) ([]string, error) {
	f.record(ctx, fmt.Sprintf("SearchNFC %d", len(texts)))
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = t
		if nfc, ok := f.nfc[t]; ok {
			out[i] = nfc
		}
	}
	return out, nil
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

func (f *fakeReads) SearchHitCount(ctx context.Context, _ app.SearchScope) (int, error) {
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
			s.reads.counts = fakeCounts{titles: 1, bodies: 40}
			d := s.reads.bodies[2].row
			// The walk stopped at its third pass, which filled the page: 2
			// of the 3 before it pass, of 40 candidates (M8/P5A fix check
			// I-1), not 3 of 4, which would give 31.
			return want{names: []string{"a", "b", "d"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: d.SortAt, ID: d.Node.ID},
				count: &domain.SearchCount{Value: 28, Kind: domain.CountAbout},
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
			// The walk checked 4 candidates and left some, one passing: at
			// least the hit seen. No sample goes on after a walk that ran
			// (M8/P5A fix check I-1).
			return want{names: []string{"1"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				count: &domain.SearchCount{Value: 1, Kind: domain.CountAtLeast},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 6", domain.MaxSearchCountScan)}}
		}},
		{"the walk finds none yet", func(s *searchFixture) want {
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), false})
			}
			s.reads.counts = fakeCounts{bodies: 6}
			fourth := s.reads.bodies[3].row
			return want{names: []string{}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 6", domain.MaxSearchCountScan)}}
		}},
		{"the candidates counted to the search's bound", func(s *searchFixture) want {
			s.countScan = 6
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), i == 1 || i == 2})
			}
			s.reads.counts = fakeCounts{bodies: 6}
			fourth := s.reads.bodies[3].row
			// The walk's rate, 2 of 4, of the 6 candidates counted, the
			// bound: at least.
			return want{names: []string{"1", "2"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				count: &domain.SearchCount{Value: 3, Kind: domain.CountAtLeast},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), "SearchBodyCount 6 of 6"}}
		}},
		{"the walk's bound on a pass", func(s *searchFixture) want {
			for i := range 6 {
				s.reads.bodies = append(s.reads.bodies, fakeBody{s.row(fmt.Sprint(i), i, domain.KindPage), i == 1 || i == 3})
			}
			s.reads.counts = fakeCounts{bodies: 60}
			fourth := s.reads.bodies[3].row
			// The walk's bound stopped it on its second pass, the page not
			// full: every pass rated, 2 of 4, of 60 candidates; its last
			// pass left out, 1 of 3, it would give 20.
			return want{names: []string{"1", "3"}, next: &domain.SearchCursor{Tier: domain.SearchTierBody, At: fourth.SortAt, ID: fourth.Node.ID},
				count: &domain.SearchCount{Value: 30, Kind: domain.CountAbout},
				calls: []string{"SearchTitles 4", "SearchHitCount", "SearchBodies 4 4 of 6",
					fmt.Sprintf("SearchTitleCount %d", domain.MaxSearchCount+1), fmt.Sprintf("SearchBodyCount %d of 6", domain.MaxSearchCountScan)}}
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
// body tier's walk stopped at its first passes (M8/P5A review C-M1, fix
// check I-1): the walk's rate, its last pass, the one that filled the
// page, left out, and no sample after it, a rate of fewer than two passes
// the hits seen at least (fix check F-1); a sample from the body tier's
// top only when the title tier filled the page, the walk not run. One of
// 2,000 candidates in 50 passing, the newest among them, the page of 20.
func TestTheCountOfAShortWalkLeavesItsLastPassOut(t *testing.T) {
	for _, tt := range []struct {
		titles  int
		sampled bool // the walk not run, the count sampled from the top
		want    domain.SearchCount
	}{
		{21, true, domain.SearchCount{Value: 61, Kind: domain.CountAbout}},    // the sample's 2 of 100: true 61
		{20, false, domain.SearchCount{Value: 21, Kind: domain.CountAtLeast}}, // one pass, of 1: true 60
		{19, false, domain.SearchCount{Value: 21, Kind: domain.CountAtLeast}}, // the 2nd pass, its rate's one: true 59
		{18, false, domain.SearchCount{Value: 58, Kind: domain.CountAbout}},   // 2 of 100: true 58
		{15, false, domain.SearchCount{Value: 55, Kind: domain.CountAbout}},   // 5 of 250: true 55
		{10, false, domain.SearchCount{Value: 50, Kind: domain.CountAbout}},   // its bound: 10 of 500, true 50
		{0, false, domain.SearchCount{Value: 40, Kind: domain.CountAbout}},    // its bound: 10 of 500, true 40
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
		var bodies []string // the walk's read, or the sample's
		for _, c := range s.rec.calls {
			if read, ok := strings.CutSuffix(c, " in tx"); ok && strings.HasPrefix(read, "SearchBodies ") {
				bodies = append(bodies, read)
			}
		}
		wantReads := []string{fmt.Sprintf("SearchBodies %d %d of 2000", domain.MaxSearchWalk, 21-tt.titles)}
		if tt.sampled {
			wantReads = []string{fmt.Sprintf("SearchBodies %[1]d %[1]d of 2000", domain.SearchCountSample)}
		}
		if page.Count == nil || *page.Count != tt.want || !slices.Equal(bodies, wantReads) {
			t.Errorf("%d titles: count %v, the body tier's reads %q; want %v, %q", tt.titles, page.Count, bodies, tt.want, wantReads)
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

// A query of a term Go's NFC changes (M8 Codex review R2; fix check 1,
// C1-1) reads PostgreSQL's NFC of it first in the snapshot, once: the
// tiers match it in contents by that NFC, and a hit's excerpt marks it; in
// names by Go's. A query of no such term, typed in NFC, reads none; one of
// a decomposed letter reads it.
func TestTheSearchReadsTheNFCOfATermGoWritesOtherwiseFirst(t *testing.T) {
	acute := string(rune(0x0301))
	raw31 := "a" + strings.Repeat(acute, 31)
	pg31 := string(rune(0x00E1)) + strings.Repeat(acute, 30)
	s := newSearchFixture()
	s.reads.nfc = map[string]string{raw31: pg31}
	row := s.row("Notes", 2, domain.KindPage)
	row.At, row.Excerpt, row.Chars = 3, "x "+pg31, 33
	s.reads.titles = []app.SearchTitleRow{row}
	got, err := s.search(app.SearchRequest{Q: ptr("zz " + raw31)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := slices.Index(s.rec.calls, "WithinSnapshot")
	if snapshot < 0 || snapshot+1 >= len(s.rec.calls) || s.rec.calls[snapshot+1] != "SearchNFC 1 in tx" ||
		slices.Index(s.rec.calls[snapshot+2:], "SearchNFC 1 in tx") >= 0 {
		t.Errorf("calls %q: want SearchNFC of the one term first in the snapshot, once", s.rec.calls)
	}
	for _, sc := range s.reads.scopes {
		in, names := sc.Query.ContentPatterns(), sc.Query.Patterns()
		if len(in) != 2 || in[0] != names[0] || in[1] == names[1] || strings.Count(in[1], "0301") != 30 || strings.Contains(in[1], "034F") {
			t.Errorf("the scope's patterns of contents %q, of names %q: want the term's in contents of its NFC there", in, names)
		}
	}
	if len(got.Hits) != 1 || got.Hits[0].Excerpt == nil || !slices.Equal(got.Hits[0].Excerpt.Hits, []domain.TextRange{{Start: 2, End: 33}}) {
		t.Errorf("Execute() = %+v: want the excerpt's hit of the term", got)
	}
	for q, want := range map[string]int{string(rune(0x00E1)) + strings.Repeat(acute, 29): 0, "data": 0, "cafe" + acute: 1} {
		s = newSearchFixture()
		_, err := s.search(app.SearchRequest{Q: ptr(q)})
		if n := len(slices.DeleteFunc(slices.Clone(s.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "SearchNFC") })); err != nil ||
			n != want {
			t.Errorf("%U: %v after %q, want %d SearchNFC", []rune(q), err, s.rec.calls, want)
		}
	}
}
