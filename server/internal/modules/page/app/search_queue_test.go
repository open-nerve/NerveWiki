package app_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A unit queues for the search's job the nodes it creates, deletes, renames
// or writes the content of, once each, in ascending id order, after its
// writes and before the observers; exact, but an import's unit, through
// ImportWrites, in bulk. A move and a reorder queue nothing (M8/P5 design
// 3.4).
func TestTheUnitQueuesWhatItChanges(t *testing.T) {
	for _, tt := range []struct {
		name string
		do   func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error
		// want are the nodes queued, of a, b and the node the unit created.
		want    func(a, b, created domain.Node) []uuid.UUID
		bulk    bool
		import_ bool
	}{
		{"a creation", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return create("New")
		}, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{created.ID} }, false, false},
		{"renames, in id order however made", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return func(ctx context.Context, u *app.Unit) error {
				if _, err := u.Rename(ctx, b.ID, "B2"); err != nil {
					return err
				}
				_, err := u.Rename(ctx, a.ID, "A2")
				return err
			}
		}, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{a.ID, b.ID} }, false, false},
		{"a content's write", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return func(ctx context.Context, u *app.Unit) error {
				_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: b.ID, Content: "new", Base: 1})
				return err
			}
		}, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{b.ID} }, false, false},
		{"a content's write and a rename of one node, once", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return func(ctx context.Context, u *app.Unit) error {
				if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: a.ID, Content: "new", Base: 1}); err != nil {
					return err
				}
				_, err := u.Rename(ctx, a.ID, "A2")
				return err
			}
		}, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{a.ID} }, false, false},
		{"a deletion of a subtree", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return func(ctx context.Context, u *app.Unit) error {
				_, err := u.Delete(ctx, a.ID)
				return err
			}
		}, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{a.ID, b.ID} }, false, false},
		{"a move and a reorder", func(f *fixture, a, b domain.Node) func(ctx context.Context, u *app.Unit) error {
			return func(ctx context.Context, u *app.Unit) error {
				if _, err := u.Move(ctx, b.ID, nil, app.First()); err != nil {
					return err
				}
				_, err := u.Move(ctx, a.ID, nil, app.After(b.ID))
				return err
			}
		}, func(a, b, created domain.Node) []uuid.UUID { return nil }, false, false},
		{"an import's creation, in bulk", nil, func(a, b, created domain.Node) []uuid.UUID { return []uuid.UUID{created.ID} }, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename, domain.ActionCreate)
			a := f.page("A", nil, 0)
			b := f.page("B", &a.ID, 0)
			before := map[uuid.UUID]bool{a.ID: true, b.ID: true}
			var err error
			if tt.import_ {
				_, err = f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
					_, err := u.CreatePage(ctx, app.ImportedPage{Name: "Imported"})
					return err
				})
			} else {
				_, err = f.run(f.asAlice(), domain.ClientWeb, tt.do(f, a, b))
			}
			if err != nil {
				t.Fatal(err)
			}
			var created domain.Node
			for id, n := range f.store.nodes {
				if !before[id] {
					created = n
				}
			}
			want := tt.want(a, b, created)
			var got []uuid.UUID
			for _, q := range f.store.queued {
				if q.notebook != f.eng || q.bulk != tt.bulk {
					t.Errorf("queued %+v, want of eng, bulk %v", q, tt.bulk)
				}
				got = append(got, q.ids...)
			}
			if len(f.store.queued) > 1 || !slices.Equal(got, want) {
				t.Errorf("queued %+v, want one queueing of %v", f.store.queued, want)
			}
			if want != nil {
				i, j := slices.Index(f.rec.calls, "QueueSearch in tx"), slices.IndexFunc(f.rec.calls, func(c string) bool {
					return c == "CreateChangeset in tx" || c == "TouchChangeset in tx"
				})
				if i < 0 || i < j {
					t.Errorf("calls %v: want QueueSearch in the unit's transaction, after its writes", f.rec.calls)
				}
			}
		})
	}
}
