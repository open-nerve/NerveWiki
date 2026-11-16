package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// recordedNames are the fixture's display names, each read recorded.
type recordedNames struct {
	*recorder
	names fakeNames
}

func (n recordedNames) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	n.record(ctx, fmt.Sprintf("DisplayNames %d", len(ids)))
	return n.names.DisplayNames(ctx, ids)
}

// oneSnapshot runs fn in a context the recorder sees as a transaction, as
// fakeSnapshots does, and refuses one within a transaction, as
// postgres.TxManager's WithinSnapshot does.
type oneSnapshot struct{ *recorder }

func (s oneSnapshot) WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error {
	s.record(ctx, "WithinSnapshot")
	if ctx.Value(txKey{}) != nil {
		return errors.New("a snapshot within a transaction")
	}
	return fn(context.WithValue(ctx, txKey{}, true))
}

// The log's page and a changeset's detail are read in one snapshot, from
// the first read of the answer to the paths of its nodes (M12 design
// 4.6.2, H5; P3 design 3.8.3, decision 370): each use case opens one
// snapshot; every port it calls before it, the notebook's decision and
// with a page id its node, which the answer does not hold, is called
// outside it; the call after it is the first read the answer holds
// (ListChangesets; the path's StateOf; FindChangeset), and every call
// from there on, of every port, the history, the names, the paths, the
// notebooks and the decision, is in it. A snapshot that began a read late,
// or ended one early, or two snapshots, fail.
func TestTheLogAndAChangesetReadInOneSnapshot(t *testing.T) {
	for _, tt := range []struct {
		name   string
		read   func(f *logFixture, r app.HistoryReads) error
		before []string // the calls outside the snapshot, in order
		in     []string // the calls the snapshot holds, by their start, among others
	}{
		{"the log", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewListChangesets(r).Execute(f.asAlice(), f.eng, app.LogQuery{})
			return err
		}, []string{"WorkspaceOf", "Authorize page.history"},
			[]string{"ListChangesets", "ChangesetFacts", "FirstChanges", "FirstIngestSources", "DisplayNames"}},
		{"the log of a page", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewListChangesets(r).Execute(f.asAlice(), f.eng, app.LogQuery{PageID: &f.a.ID})
			return err
		}, []string{"WorkspaceOf", "Authorize page.history", "NodesNow 1"},
			[]string{"ListChangesets page", "ChangesetFacts", "FirstChanges", "FirstIngestSources", "DisplayNames"}},
		{"the log by its path", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewListChangesets(r).AtPath(f.asAlice(), f.eng, app.PathLogQuery{Path: new("A/C.md")})
			return err
		}, []string{"WorkspaceOf", "Authorize page.history"},
			[]string{"StateOf", "RootByKey", "ChildByKey", "ListChangesets page", "ChangesetFacts", "FirstChanges", "FirstIngestSources",
				"DisplayNames", "PathsOf"}},
		{"the log by no path", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewListChangesets(r).AtPath(f.asAlice(), f.eng, app.PathLogQuery{})
			return err
		}, []string{"WorkspaceOf", "Authorize page.history"},
			[]string{"ListChangesets", "ChangesetFacts", "FirstChanges", "FirstIngestSources", "DisplayNames", "PathsOf"}},
		{"a changeset", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewGetChangeset(r).Execute(f.asAlice(), f.made.ID)
			return err
		}, nil,
			[]string{"FindChangeset", "WorkspaceOf", "Authorize page.history", "ChangesetFacts", "FirstChanges", "FirstIngestSources",
				"DisplayNames", "ChangesetMoves"}},
		{"a changeset in its notebook", func(f *logFixture, r app.HistoryReads) error {
			_, err := app.NewGetChangeset(r).InNotebook(f.asAlice(), f.eng, f.made.ID, nil, nil)
			return err
		}, []string{"WorkspaceOf", "Authorize page.history"},
			[]string{"FindChangeset", "ChangesetFacts", "FirstChanges", "FirstIngestSources", "DisplayNames", "ChangesetMoves",
				"ListChanges", "PathsOf"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newLogFixture()
			// C, under A, made by a changeset of an ingest source, A, alive:
			// the log's answer holds A's path, which PathsOf reads.
			c := f.page("C", &f.a.ID, 0)
			state := c.State()
			made := f.log.changeset(domain.ChangesetEdit, f.alice, f.madeAt.Add(time.Minute), []domain.ChangesetItem{{NodeID: c.ID, After: &state}})
			made.sources = []uuid.UUID{f.a.ID}
			r := app.HistoryReads{Notebooks: f.notebooks, History: f.log, Names: recordedNames{f.rec, f.names}, Auth: f.auth,
				Paths: f.store, Snapshots: oneSnapshot{f.rec}}
			f.rec.calls = nil
			if err := tt.read(f, r); err != nil {
				t.Fatal(err)
			}
			at := slices.Index(f.rec.calls, "WithinSnapshot")
			if at < 0 || slices.Index(f.rec.calls[at+1:], "WithinSnapshot") >= 0 {
				t.Fatalf("calls %q: want one snapshot", f.rec.calls)
			}
			if before := f.rec.calls[:at]; !slices.Equal(before, tt.before) {
				t.Errorf("calls before the snapshot %q, want %q", before, tt.before)
			}
			in := f.rec.calls[at+1:]
			if len(in) == 0 || !strings.HasPrefix(in[0], tt.in[0]+" ") && in[0] != tt.in[0]+" in tx" {
				t.Errorf("the snapshot's first call %q, want %s, the answer's first read", in, tt.in[0])
			}
			for _, call := range in {
				if !strings.HasSuffix(call, " in tx") {
					t.Errorf("%s after the snapshot began, out of it (calls %q)", call, f.rec.calls)
				}
			}
			for _, want := range tt.in {
				if !slices.ContainsFunc(in, func(call string) bool { return call == want+" in tx" || strings.HasPrefix(call, want+" ") }) {
					t.Errorf("no %s in the snapshot (calls %q)", want, f.rec.calls)
				}
			}
		})
	}
}
