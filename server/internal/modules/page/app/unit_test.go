package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// run runs a unit of eng that changes the tree, decided on as a rename.
func (f *fixture) run(ctx context.Context, client domain.Client, do func(ctx context.Context, u *app.Unit) error) (app.Outcome, error) {
	return f.writer().Run(ctx, app.UnitSpec{NotebookID: f.eng, Action: domain.ActionRename, Tree: true, Client: client,
		Options: app.Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}, do)
}

// A unit opens the outermost transaction: in one already, it refuses
// before it reads anything.
func TestAUnitRefusesToRunInATransaction(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	err := f.tx.WithinTx(f.asAlice(), func(ctx context.Context) error {
		_, err := f.run(ctx, domain.ClientWeb, func(context.Context, *app.Unit) error { return nil })
		return err
	})
	if err == nil || len(f.rec.calls) != 0 {
		t.Errorf("a unit in a transaction = %v after %v, want an error before any call", err, f.rec.calls)
	}
}

// A client the changesets table refuses is the adapter's defect: the unit
// refuses it before it reads anything.
func TestAUnitRefusesAnUnknownClient(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	for _, client := range []domain.Client{"", "mobile", "mcp:"} {
		if _, err := f.run(f.asAlice(), client, func(context.Context, *app.Unit) error { return nil }); err == nil {
			t.Errorf("a unit of client %q = nil error", client)
		}
	}
	if len(f.rec.calls) != 0 {
		t.Errorf("calls = %v, want none", f.rec.calls)
	}
}

// A unit of two operations has one changeset, with the merged item of its
// node, and tells the observers once: the change merged, the earliest
// before and the latest after.
func TestAUnitOfTwoOperationsTellsOnce(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	var created domain.Node
	outcome, err := f.run(f.asAlice(), domain.ClientAPI, func(ctx context.Context, u *app.Unit) error {
		var err error
		if created, err = u.CreatePage(ctx, app.PageDraft{Title: "Draft"}); err != nil {
			return err
		}
		_, err = u.Rename(ctx, created.ID, "Final")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.store.changesets) != 1 || outcome.ChangesetID != f.store.changesets[0].ID || f.store.changesets[0].Client != domain.ClientAPI {
		t.Fatalf("changesets %+v, outcome %+v; want one of the API, the outcome's", f.store.changesets, outcome)
	}
	if item := f.store.items[created.ID]; item.Change.Before != nil || item.Change.After.Name != "Final" {
		t.Errorf("item = %+v, want the page created as Final", item.Change)
	}
	if len(o.events) != 1 {
		t.Fatalf("events = %d, want 1", len(o.events))
	}
	e := o.events[0]
	if e.ChangesetID != outcome.ChangesetID || e.WorkspaceID != f.acme || e.By != f.alice || e.Client != domain.ClientAPI ||
		!e.Options.UpdateLinks || !e.At.Equal(now()) || len(e.Changes) != 1 || e.Changes[0].Before != nil ||
		e.Changes[0].After.Name != "Final" || e.Changes[0].Revision != 1 {
		t.Errorf("event = %+v, want the page created as Final at revision 1, in the changeset", e)
	}
	if i, j := slices.Index(f.rec.calls, "PagesChanged in tx"), slices.Index(f.rec.calls, "RenameNode in tx"); i < j {
		t.Errorf("calls = %v, want the observers after the operations", f.rec.calls)
	}
}

// A unit marks its changeset as renaming or moving a node from its merged
// changes (M9/P3 design 3.20.2), once, before the search queue, when it
// queues one, and the observers: a node alive before and after it of
// another name renames, under another parent moves, at the root or under
// a folder alike; a creation renamed, a reorder and a content's write
// neither, and mark nothing.
func TestAUnitMarksItsChangesetsRenamesAndMoves(t *testing.T) {
	for _, tt := range []struct {
		name string
		do   func(ctx context.Context, u *app.Unit, f *fixture) error
		want *domain.ChangesetMoves
	}{
		{"a rename", func(ctx context.Context, u *app.Unit, f *fixture) error {
			_, err := u.Rename(ctx, f.page("A", nil, 0).ID, "B")
			return err
		}, &domain.ChangesetMoves{Renames: true}},
		{"a move, then a rename back", func(ctx context.Context, u *app.Unit, f *fixture) error {
			a, p := f.page("A", nil, 0), f.page("P", nil, 1)
			if _, err := u.Rename(ctx, a.ID, "B"); err != nil {
				return err
			}
			if _, err := u.Move(ctx, a.ID, &p.ID, app.First()); err != nil {
				return err
			}
			_, err := u.Rename(ctx, a.ID, "A")
			return err
		}, &domain.ChangesetMoves{Moves: true}},
		{"a reorder at the root", func(ctx context.Context, u *app.Unit, f *fixture) error {
			a, b := f.page("A", nil, 0), f.page("B", nil, 1)
			_, err := u.Move(ctx, a.ID, nil, app.After(b.ID))
			return err
		}, nil},
		{"a rename under a folder", func(ctx context.Context, u *app.Unit, f *fixture) error {
			p := f.page("P", nil, 0)
			_, err := u.Rename(ctx, f.page("A", &p.ID, 0).ID, "B")
			return err
		}, &domain.ChangesetMoves{Renames: true}},
		{"a reorder under a folder", func(ctx context.Context, u *app.Unit, f *fixture) error {
			p := f.page("P", nil, 0)
			a, b := f.page("A", &p.ID, 0), f.page("B", &p.ID, 1)
			parent := p.ID // the request's own, as a call brings it
			_, err := u.Move(ctx, a.ID, &parent, app.After(b.ID))
			return err
		}, nil},
		{"a creation renamed", func(ctx context.Context, u *app.Unit, f *fixture) error {
			created, err := u.CreatePage(ctx, app.PageDraft{Title: "Draft"})
			if err != nil {
				return err
			}
			_, err = u.Rename(ctx, created.ID, "Final")
			return err
		}, nil},
		{"a content's write", func(ctx context.Context, u *app.Unit, f *fixture) error {
			p := f.page("P", nil, 0)
			f.content(p.ID, "one", 1)
			_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: "two", Facts: "two", Base: 1})
			return err
		}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			f.observers = []app.PageObserver{&observer{recorder: f.rec}}
			outcome, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error { return tt.do(ctx, u, f) })
			if err != nil {
				t.Fatal(err)
			}
			got, marked := f.store.marked[outcome.ChangesetID]
			queued, observed := slices.Index(f.rec.calls, "QueueSearch in tx"), slices.Index(f.rec.calls, "PagesChanged in tx")
			if marks := slices.Index(f.rec.calls, "MarkChangesetMoves in tx"); tt.want == nil && (marked || marks >= 0) ||
				tt.want != nil && (got != *tt.want || marks < 0 || queued >= 0 && marks > queued || marks > observed) {
				t.Errorf("marked %+v (%v), calls %v; want %+v, before the search queue and the observers", got, marked, f.rec.calls,
					tt.want)
			}
		})
	}
}

// An operation a participant adds runs through the guards, into the same
// changeset and the same event, and calls no participant.
func TestAParticipantAddsToTheUnit(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	other := f.page("Other", nil, 0)
	g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
	p := &participant{recorder: f.rec, rename: &other.ID, name: "Renamed"}
	f.guards, f.observers, f.partakers = []app.WriteGuard{g}, []app.PageObserver{o}, []app.Participant{p}
	var created domain.Node
	_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		var err error
		created, err = u.CreatePage(ctx, app.PageDraft{Title: "New"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.steps) != 1 || p.steps[0].Operation != domain.OpCreate || p.steps[0].Changes[0].NodeID != created.ID {
		t.Errorf("the participant followed %+v, want the creation alone", p.steps)
	}
	if len(g.steps) != 2 || g.steps[1].Operation != domain.OpRename || g.steps[1].Changes[0].NodeID != other.ID {
		t.Errorf("the guard saw %+v, want the creation, then the added rename", g.steps)
	}
	if len(f.store.changesets) != 1 || f.store.items[other.ID].ChangesetID != f.store.changesets[0].ID {
		t.Errorf("changesets %d, the rename's item %+v; want the rename in the one changeset", len(f.store.changesets), f.store.items[other.ID])
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 2 || o.events[0].Changes[1].After.Name != "Renamed" {
		t.Errorf("events = %+v, want one with both changes", o.events)
	}
}

// What a participant defers runs once the unit is over: after its
// observers, its transaction ended, committed or rolled back; once.
func TestWhatAParticipantDefersRunsOnceTheUnitIsOver(t *testing.T) {
	for _, fails := range []bool{false, true} {
		f := newFixture()
		f.grant(domain.ActionRename)
		var ran []string
		p := &participant{recorder: f.rec}
		p.deferred = func() {
			ran = append(ran, fmt.Sprintf("rolled back %t after %s", f.tx.rolledBack, f.rec.calls[len(f.rec.calls)-1]))
		}
		var err error
		if fails {
			err = errors.New("the observer failed")
		}
		f.partakers, f.observers = []app.Participant{p}, []app.PageObserver{&observer{recorder: f.rec, err: err}}
		if _, got := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
			_, err := u.CreatePage(ctx, app.PageDraft{Title: "New"})
			return err
		}); !errors.Is(got, err) {
			t.Fatalf("the unit: %v, want %v", got, err)
		}
		if want := []string{fmt.Sprintf("rolled back %t after PagesChanged in tx", fails)}; !slices.Equal(ran, want) {
			t.Errorf("deferred ran %q, want %q", ran, want)
		}
	}
}

// A guard's, a participant's or an observer's error rolls the unit back
// and is its answer.
func TestAnExtensionsErrorRollsTheUnitBack(t *testing.T) {
	failure := errors.New("the extension failed")
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture)
		want  error
	}{
		{"a guard", func(f *fixture) { f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}} }, refusal},
		{"a participant", func(f *fixture) {
			missing := uuid.NewV7()
			f.partakers = []app.Participant{&participant{recorder: f.rec, rename: &missing, name: "X"}}
		}, domain.ErrNotFound},
		{"an observer", func(f *fixture) { f.observers = []app.PageObserver{&observer{recorder: f.rec, err: failure}} }, failure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			tt.setup(f)
			_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
				_, err := u.CreatePage(ctx, app.PageDraft{Title: "New"})
				return err
			})
			if !errors.Is(err, tt.want) || !f.tx.rolledBack {
				t.Errorf("the unit = %v, rolled back %v; want %v, rolled back", err, f.tx.rolledBack, tt.want)
			}
		})
	}
}

// Several registrants run in the order bootstrap hands them (v0.1 design
// 13.1, item 21): the guards, then, after the write, the participants;
// the observers once the operations are done. The first guard's refusal
// stops the rest.
func TestTheRegistrantsRunInTheirOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	f.guards = []app.WriteGuard{&guard{recorder: f.rec, name: "first"}, &guard{recorder: f.rec, name: "second"}}
	f.partakers = []app.Participant{&participant{recorder: f.rec, label: "first"}, &participant{recorder: f.rec, label: "second"}}
	f.observers = []app.PageObserver{&observer{recorder: f.rec, name: "first"}, &observer{recorder: f.rec, name: "second"}}
	if _, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		_, err := u.CreatePage(ctx, app.PageDraft{Title: "New"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, call := range f.rec.calls {
		if strings.HasSuffix(call, " first in tx") || strings.HasSuffix(call, " second in tx") {
			got = append(got, call)
		}
	}
	want := []string{"GuardWrite create first in tx", "GuardWrite create second in tx", "Participate create first in tx",
		"Participate create second in tx", "PagesChanged first in tx", "PagesChanged second in tx"}
	if !slices.Equal(got, want) {
		t.Errorf("registrants ran %v, want %v", got, want)
	}

	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	f = newFixture()
	f.grant(domain.ActionRename)
	second := &guard{recorder: f.rec, name: "second"}
	f.guards = []app.WriteGuard{&guard{recorder: f.rec, name: "first", err: refusal}, second}
	_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		_, err := u.CreatePage(ctx, app.PageDraft{Title: "New"})
		return err
	})
	if !errors.Is(err, refusal) || len(second.steps) != 0 {
		t.Errorf("the unit = %v after the second guard saw %d steps; want the first's refusal, the second not asked", err, len(second.steps))
	}
}

// A guard's refusal comes out of the unit marked with the node it is about
// (M10/P1 design 3.22): a problem with the step's first node, the node
// moved before the nodes it carries; one the guard marked already keeps
// its node; any other error, the database's, is as it is. Through the
// mark, errors.Is sees the problem and the unit rolls back.
func TestAUnitMarksItsGuardsRefusals(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	elsewhere := uuid.NewV7()
	failure := errors.New("the database is gone")
	for _, tt := range []struct {
		name   string
		err    error
		marked bool
		node   bool // the moved node's; else elsewhere
	}{
		{"a problem", refusal, true, true},
		{"a problem marked", domain.Refuse(refusal, elsewhere), true, false},
		{"another error", failure, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			notes := f.page("Notes", nil, 1)
			f.page("Inner", &notes.ID, 1)
			other := f.page("Other", nil, 2)
			g := &guard{recorder: f.rec, err: tt.err}
			f.guards = []app.WriteGuard{g}
			_, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
				_, err := u.Move(ctx, notes.ID, &other.ID, app.First())
				return err
			})
			var r *domain.Refused
			switch {
			case len(g.steps) != 1 || len(g.steps[0].Changes) != 2 || g.steps[0].Changes[0].NodeID != notes.ID:
				t.Fatalf("the guard saw %+v, want the move of Notes and Inner under it", g.steps)
			case !errors.Is(err, tt.err) || !f.tx.rolledBack:
				t.Errorf("the unit = %v, rolled back %v; want %v, rolled back", err, f.tx.rolledBack, tt.err)
			case errors.As(err, &r) != tt.marked:
				t.Errorf("the unit = %v, marked %v; want marked %v", err, r != nil, tt.marked)
			case tt.marked && tt.node && r.NodeID != notes.ID:
				t.Errorf("the refusal is of %s, want the moved node's, %s", r.NodeID, notes.ID)
			case tt.marked && !tt.node && r.NodeID != elsewhere:
				t.Errorf("the refusal is of %s, want the guard's own, %s", r.NodeID, elsewhere)
			}
		})
	}
}

// An operation given a context without the unit's transaction refuses:
// it would write on the pool, outside the unit and its locks.
func TestAnOperationRefusesAContextOutsideTheUnit(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	outside := f.asAlice()
	_, err := f.run(outside, domain.ClientWeb, func(_ context.Context, u *app.Unit) error {
		_, err := u.CreatePage(outside, app.PageDraft{Title: "New"})
		return err
	})
	if err == nil || f.called("CreateNode") || f.called("CreateNode in tx") {
		t.Errorf("an operation outside the unit's transaction = %v, wrote %v; want an error and no write", err, f.rec.calls)
	}
}
