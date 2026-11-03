package app_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The use cases must pass true only, choose 3/20 once, retain null positions,
// and read paths only for the MCP entry points after authorization.
func TestIngestHistory(t *testing.T) {
	fixture := func() (*logFixture, []*uuid.UUID) {
		f := newLogFixture()
		want := make([]*uuid.UUID, 25)
		for i := range 25 {
			node := f.page(fmt.Sprintf("Source-%02d", i), nil, float64(i+10))
			f.made.sources = append(f.made.sources, node.ID)
			want[i] = new(node.ID)
		}
		f.trashed(f.made.sources[0], now(), f.alice)
		delete(f.store.nodes, f.made.sources[2])
		want[0], want[2] = nil, nil
		f.made.IngestCount = 25
		return f, want
	}
	t.Run("preview_counts", func(t *testing.T) {
		f, want := fixture()
		l, err := app.NewListChangesets(f.historyReads()).Execute(f.asAlice(), f.eng, app.LogQuery{})
		if err != nil || !slices.Equal(ids(l.Changesets), []uuid.UUID{f.reverted.ID, f.written.ID, f.renamed.ID, f.made.ID}) {
			t.Fatalf("history=%v,%v; want all four", ids(l.Changesets), err)
		}
		if !reflect.DeepEqual(l.Changesets[3].IngestSources, want[:3]) {
			t.Errorf("list preview=%v, want %v", l.Changesets[3].IngestSources, want[:3])
		}
		for _, call := range f.rec.calls {
			if strings.HasPrefix(call, "PathsOf ") {
				t.Error("REST history read paths")
			}
		}
		f.rec.calls = nil
		d, err := app.NewGetChangeset(f.historyReads()).Execute(f.asAlice(), f.made.ID)
		if err != nil || !reflect.DeepEqual(d.IngestSources, want[:20]) {
			t.Errorf("detail preview=%v,%v, want first20", d.IngestSources, err)
		}
		got := []string{}
		for _, call := range f.rec.calls {
			if strings.HasPrefix(call, "FirstIngestSources ") {
				got = append(got, call)
			}
		}
		if !slices.Equal(got, []string{"FirstIngestSources 1 20"}) {
			t.Errorf("detail preview calls=%v, want one batch20", got)
		}
		for _, ingest := range []*bool{nil, new(false)} {
			list, err := app.NewListChangesets(f.historyReads()).Execute(f.asAlice(), f.eng, app.LogQuery{Ingest: ingest})
			if err != nil || !slices.Equal(ids(list.Changesets), []uuid.UUID{f.reverted.ID, f.written.ID, f.renamed.ID, f.made.ID}) {
				t.Errorf("false/omitted history=%v,%v", ids(list.Changesets), err)
			}
		}
	})
	t.Run("mcp_paths", func(t *testing.T) {
		f, want := fixture()
		l, err := app.NewListChangesets(f.pathHistoryReads()).AtPath(f.asAlice(), f.eng, app.PathLogQuery{})
		if err != nil || !slices.Equal(ids(l.Changesets), []uuid.UUID{f.reverted.ID, f.written.ID, f.renamed.ID, f.made.ID}) {
			t.Fatalf("path log=%v,%v", ids(l.Changesets), err)
		}
		if len(l.Paths) != 1 || l.Paths[*want[1]] != "Source-01.md" {
			t.Errorf("log source paths=%v", l.Paths)
		}
		f.rec.calls = nil
		p, err := app.NewGetChangeset(f.pathHistoryReads()).InNotebook(f.asAlice(), f.eng, f.made.ID, nil, nil)
		if err != nil || p.Detail == nil || !reflect.DeepEqual(p.Detail.IngestSources, want[:20]) {
			t.Fatalf("MCP detail=%+v,%v", p.Detail, err)
		}
		if p.Paths[*want[19]] != "Source-19.md" {
			t.Errorf("detail missing last source path: %v", p.Paths)
		}
		calls := 0
		for _, call := range f.rec.calls {
			if strings.HasPrefix(call, "PathsOf ") {
				calls++
			}
		}
		if calls != 1 {
			t.Errorf("detail read paths %d times, want union once", calls)
		}
		boom := errors.New("path read failed")
		f.store.errs = map[string]error{"PathsOf": boom}
		if _, err := app.NewListChangesets(f.pathHistoryReads()).AtPath(f.asAlice(), f.eng, app.PathLogQuery{Ingest: new(true)}); !errors.Is(err, boom) {
			t.Errorf("log swallowed path failure: %v", err)
		}
	})
	t.Run("authorization_and_failure", func(t *testing.T) {
		f, _ := fixture()
		delete(f.auth.grants, domain.ActionHistory)
		f.rec.calls = nil
		_, err := app.NewGetChangeset(f.pathHistoryReads()).InNotebook(f.asAlice(), f.eng, f.made.ID, nil, nil)
		if !errors.Is(err, domain.ErrNotebookNotFound) {
			t.Errorf("hidden notebook=%v", err)
		}
		for _, call := range f.rec.calls {
			if strings.HasPrefix(call, "FirstIngestSources ") || strings.HasPrefix(call, "PathsOf ") {
				t.Errorf("hidden notebook read %s", call)
			}
		}
		f2, _ := fixture()
		boom := errors.New("preview read failed")
		f2.store.errs = map[string]error{"FirstIngestSources": boom}
		_, err = app.NewGetChangeset(f2.pathHistoryReads()).InNotebook(f2.asAlice(), f2.eng, f2.made.ID, nil, nil)
		if !errors.Is(err, boom) {
			t.Errorf("preview failure=%v", err)
		}
		for _, call := range f2.rec.calls {
			if call == "ChangesetMoves" || call == "ListChanges" || strings.HasPrefix(call, "PathsOf ") {
				t.Errorf("work after preview error: %s", call)
			}
		}
	})
}
