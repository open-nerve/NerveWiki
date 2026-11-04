package app_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

type ingestObserver func(context.Context, app.Event) error

func (f ingestObserver) PagesChanged(ctx context.Context, e app.Event) error { return f(ctx, e) }
func ingestFixtureBefore() (*fixture, domain.Node) {
	f := newFixture()
	f.grant(domain.ActionRename)
	raw := f.page("raw", nil, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
	return f, raw
}
func TestIngestBefore(t *testing.T) {
	t.Run("first_memo", func(t *testing.T) {
		f, raw := ingestFixtureBefore()
		folder := f.page("Folder", &raw.ID, 0)
		p := f.page("P", &folder.ID, 0)
		calls := 0
		f.observers = []app.PageObserver{ingestObserver(func(ctx context.Context, e app.Event) error {
			calls++
			rows, err := e.BeforeIngest.NodesBefore(ctx, []uuid.UUID{p.ID})
			if err != nil {
				return err
			}
			if len(rows) != 1 || !rows[0].Exists || !rows[0].InSource || rows[0].Revision != 1 || !rows[0].RevisionKnown || rows[0].PageBytes != 0 || !rows[0].PageBytesKnown {
				t.Fatalf("first child facts=%+v, want initial raw descendant at empty r1", rows)
			}
			return nil
		})}
		_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
			if _, err := u.Rename(ctx, folder.ID, "Temporary"); err != nil {
				return err
			}
			if _, err := u.Move(ctx, folder.ID, nil, app.Position{}); err != nil {
				return err
			}
			if _, err := u.Rename(ctx, folder.ID, "Final"); err != nil {
				return err
			}
			_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "now outside", Base: 1})
			return err
		})
		if err != nil || calls != 1 {
			t.Fatalf("unit=%v observations=%d", err, calls)
		}
		if f.store.nodes[folder.ID].ParentID != nil {
			t.Fatal("fixture did not actually move ancestor out")
		}
	})
	t.Run("before_revision", func(t *testing.T) {
		f, _ := ingestFixtureBefore()
		p := f.page("P", nil, 1)
		f.observers = []app.PageObserver{ingestObserver(func(ctx context.Context, e app.Event) error {
			rows, err := e.BeforeIngest.NodesBefore(ctx, []uuid.UUID{p.ID})
			if err != nil {
				return err
			}
			if len(rows) != 1 || rows[0].Revision != 1 || rows[0].PageBytes != 0 || !rows[0].RevisionKnown || !rows[0].PageBytesKnown {
				t.Fatalf("first locked content=%+v", rows)
			}
			return nil
		})}
		_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
			if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "first", Base: 1}); err != nil {
				return err
			}
			_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "second", Base: 2})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		writes := 0
		for _, call := range f.rec.calls {
			if call == "WriteContent in tx" {
				writes++
			}
		}
		if f.store.contents[p.ID].Revision != 3 || writes != 2 || len(f.store.revisions) != 1 || f.store.revisions[0].Base == nil || *f.store.revisions[0].Base != 1 {
			t.Fatalf("not two actual writes: %+v", f.store.revisions)
		}
	})
	t.Run("share_target", func(t *testing.T) {
		f, raw := ingestFixtureBefore()
		p := f.page("P", nil, 1)
		s := f.page("S", &raw.ID, 0)
		c := f.store.contents[s.ID]
		c.Content = "late evidence"
		c.ByteSize = len(c.Content)
		c.Revision = 7
		f.store.contents[s.ID] = c
		f.observers = []app.PageObserver{ingestObserver(func(ctx context.Context, e app.Event) error {
			ids := []uuid.UUID{p.ID, s.ID}
			slices.SortFunc(ids, uuid.UUID.Compare)
			rows, err := e.BeforeIngest.NodesBefore(ctx, ids)
			if err != nil {
				return err
			}
			for _, r := range rows {
				if r.ID == p.ID && (!r.RevisionKnown || !r.PageBytesKnown || r.Revision != 1) {
					t.Fatalf("own locked facts=%+v", r)
				}
				if r.ID == s.ID && (!r.InSource || r.RevisionKnown || r.PageBytesKnown) {
					t.Fatalf("unlocked late target became known: %+v", r)
				}
			}
			return nil
		})}
		_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionRename, Client: domain.ClientWeb, NotFound: domain.ErrNotFound}, func(ctx context.Context, u *app.Unit) error {
			_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "[[raw/S]]", Base: 1})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("delete_all_nodes", func(t *testing.T) {
		f, _ := ingestFixtureBefore()
		root := f.page("Delete", nil, 1)
		ids := []uuid.UUID{root.ID}
		for i := range 513 {
			n := f.page(fmt.Sprintf("Child%03d", i), &root.ID, float64(i))
			ids = append(ids, n.ID)
		}
		slices.SortFunc(ids, uuid.UUID.Compare)
		f.observers = []app.PageObserver{ingestObserver(func(ctx context.Context, e app.Event) error {
			for start := 0; start < len(ids); start += 512 {
				want := ids[start:min(start+512, len(ids))]
				rows, err := e.BeforeIngest.NodesBefore(ctx, want)
				if err != nil {
					return err
				}
				if len(rows) != len(want) {
					t.Fatal("deletion memo lost rows")
				}
				for i, row := range rows {
					if row.ID != want[i] || !row.Exists || row.Kind != domain.KindPage || !row.RevisionKnown || row.Revision != 1 {
						t.Fatalf("deleted old row=%+v", row)
					}
				}
			}
			return nil
		})}
		_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error { _, err := u.Delete(ctx, root.ID); return err })
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			if _, ok := f.store.nodes[id]; ok {
				t.Fatal("delete fixture left node alive")
			}
		}
	})
}
