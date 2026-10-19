package app_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// batchUnit runs do in a batch's unit: one that changes the tree, its
// participants deferred.
func (f *fixture) batchUnit(do func(ctx context.Context, u *app.Unit) error) (app.Outcome, error) {
	f.grant(domain.ActionBatch)
	return f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionBatch, Tree: true, Client: domain.MCPClient("agent"),
		Options: app.Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound, DeferParticipants: true}, do)
}

// A batch's unit calls no participant as it goes (M9/P3 design 3.7.1):
// they follow it once, at its end, through Unit.Participate, its renames
// and moves merged with no revision, the nodes a move carried as they are,
// but none of a node a move kept the links of, and none deleted since; the
// batch's rewrite on the step; the pages their appends wrote counted.
func TestABatchsParticipantsFollowItOnceAtItsEnd(t *testing.T) {
	f := newFixture()
	a, h := f.page("A", nil, 1), f.page("H", nil, 2)
	folder, g := f.page("F", nil, 3), f.page("G", nil, 4)
	x, y := f.page("X", &folder.ID, 1), f.page("Y", &folder.ID, 2)
	p := &participant{recorder: f.rec, write: &app.ContentWrite{NodeID: a.ID, Base: 1, Content: "a again", Facts: "a again"}}
	f.partakers = []app.Participant{p}
	var written int
	b := app.BatchRewrite{MaxPages: 7, FirstMove: 1}
	_, err := f.batchUnit(func(ctx context.Context, u *app.Unit) error {
		for _, step := range []struct {
			id uuid.UUID
			p  app.Placement
		}{
			{a.ID, app.Placement{Name: "A2"}},
			{folder.ID, app.Placement{ParentID: &g.ID, Name: "F"}},
			{h.ID, app.Placement{Name: "H2", KeepLinks: true}},
		} {
			if _, err := u.Relocate(ctx, step.id, step.p); err != nil {
				return err
			}
		}
		sub, err := u.Subtree(ctx, y.ID)
		if err != nil {
			return err
		}
		if err := u.DeleteSubtree(ctx, sub); err != nil {
			return err
		}
		if len(p.steps) != 0 {
			t.Errorf("the participants followed %d ops as they went", len(p.steps))
		}
		written, err = u.Participate(ctx, b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.steps) != 1 || p.steps[0].Batch == nil || p.steps[0].Batch.MaxPages != 7 || p.steps[0].Batch.FirstMove != 1 {
		t.Fatalf("the participants followed %+v, want one step with the batch's rewrite", p.steps)
	}
	var ids []uuid.UUID
	for _, c := range p.steps[0].Changes {
		ids = append(ids, c.NodeID)
		if c.Revision != 0 || c.Facts != nil || c.Before == nil || c.After == nil {
			t.Errorf("the change of %s = %+v, want a move's, of no revision", c.NodeID, c)
		}
	}
	if want := []uuid.UUID{a.ID, folder.ID, x.ID}; !slices.Equal(ids, want) {
		t.Errorf("the changes followed are of %v, want A, F and X it carried, not H, kept, nor Y, deleted: %v", ids, want)
	}
	if written != 1 {
		t.Errorf("Participate counted %d pages written, want 1", written)
	}
}

// A batch's unit drops what it changed back (M9/P3 design 3.5.4): a page
// renamed and renamed back is no item and no change, and the changeset
// with nothing left is none; a page whose content it wrote keeps its
// change, which the observers index again; a node a move carried keeps its
// change while the move is one.
func TestABatchDropsWhatItChangedBack(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 1)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	out, err := f.batchUnit(func(ctx context.Context, u *app.Unit) error {
		for _, name := range []string{"B", "A"} {
			if _, err := u.Relocate(ctx, a.ID, app.Placement{Name: name}); err != nil {
				return err
			}
		}
		return u.DropUnchanged(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ChangesetID != (uuid.UUID{}) || len(f.store.changesets) != 0 || len(f.store.recorded) != 0 || len(o.events) != 0 {
		t.Errorf("renamed and back: changeset %v, %d changesets, %d items, %d events; want none", out.ChangesetID,
			len(f.store.changesets), len(f.store.recorded), len(o.events))
	}

	f = newFixture()
	a = f.page("A", nil, 1)
	folder, g := f.page("F", nil, 2), f.page("G", nil, 3)
	x := f.page("X", &folder.ID, 1)
	o = &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	out, err = f.batchUnit(func(ctx context.Context, u *app.Unit) error {
		if _, err := u.Relocate(ctx, a.ID, app.Placement{Name: "B"}); err != nil {
			return err
		}
		if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: a.ID, Base: 1, Content: "a again", Facts: "a again"}); err != nil {
			return err
		}
		if _, err := u.Relocate(ctx, a.ID, app.Placement{Name: "A"}); err != nil {
			return err
		}
		if _, err := u.Relocate(ctx, folder.ID, app.Placement{ParentID: &g.ID, Name: "F"}); err != nil {
			return err
		}
		return u.DropUnchanged(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ChangesetID == (uuid.UUID{}) || len(o.events) != 1 {
		t.Fatalf("changeset %v, %d events; want the changeset and its event", out.ChangesetID, len(o.events))
	}
	var changed []uuid.UUID
	for _, c := range o.events[0].Changes {
		changed = append(changed, c.NodeID)
	}
	if want := []uuid.UUID{a.ID, folder.ID, x.ID}; !slices.Equal(changed, want) {
		t.Errorf("the event's changes are of %v, want A, its content written, F and X it carried: %v", changed, want)
	}
	for _, it := range f.store.recorded {
		if it.Change.NodeID == a.ID {
			t.Errorf("A's item is left: %+v", it)
		}
	}
}
