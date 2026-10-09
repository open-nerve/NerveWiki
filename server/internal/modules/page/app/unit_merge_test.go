package app_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// importUnit runs a unit of eng that changes the tree as an import does:
// of the import kind, merged into into when it is set.
func (f *fixture) importUnit(ctx context.Context, client domain.Client, into uuid.UUID, do func(ctx context.Context, u *app.Unit) error) (
	app.Outcome, error,
) {
	return f.writer().Run(ctx, app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true, Client: client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetImport, Changeset: into}, do)
}

// create creates a page named name at the root in u.
func create(name string) func(ctx context.Context, u *app.Unit) error {
	return func(ctx context.Context, u *app.Unit) error {
		_, err := u.CreatePage(ctx, app.PageDraft{Title: name})
		return err
	}
}

// A unit acts as an account with exactly one credential: an actor without
// one, or with more, is its caller's fault, refused before it reads
// anything, and so is it by Allowed.
func TestAUnitRefusesAnActorWithoutExactlyOneCredential(t *testing.T) {
	for _, actor := range []shared.Actor{
		{UserID: uuid.NewV7()},
		{UserID: uuid.NewV7(), SessionID: uuid.NewV7(), APITokenID: uuid.NewV7()},
		{UserID: uuid.NewV7(), SessionID: uuid.NewV7(), JobID: uuid.NewV7()},
	} {
		f := newFixture()
		f.grant(domain.ActionRename)
		ctx := shared.WithActor(context.Background(), actor)
		_, err := f.run(ctx, domain.ClientWeb, func(context.Context, *app.Unit) error { return nil })
		allowed := f.writer().Allowed(ctx, app.UnitSpec{NotebookID: f.eng, Action: domain.ActionRename, NotFound: domain.ErrNotFound})
		if err == nil || allowed == nil || len(f.rec.calls) != 0 {
			t.Errorf("a unit of %+v = %v, allowed %v, after %v; want errors before any call", actor, err, allowed, f.rec.calls)
		}
	}
}

// A job acting for its starter is an actor with one credential: its unit
// runs.
func TestAUnitRunsForAJob(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, JobID: uuid.NewV7()})
	if _, err := f.importUnit(ctx, domain.ClientAPI, uuid.UUID{}, create("Imported")); err != nil {
		t.Fatal(err)
	}
}

// A unit's changeset is of its kind: an edit unless the unit says
// otherwise.
func TestAUnitsChangesetIsOfItsKind(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate, domain.ActionRename)
	if _, err := f.run(f.asAlice(), domain.ClientWeb, create("Edited")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.importUnit(f.asAlice(), domain.ClientWeb, uuid.UUID{}, create("Imported")); err != nil {
		t.Fatal(err)
	}
	if len(f.store.changesets) != 2 || f.store.changesets[0].Kind != domain.ChangesetEdit || f.store.changesets[1].Kind != domain.ChangesetImport {
		t.Errorf("changesets = %+v, want an edit's, then an import's", f.store.changesets)
	}
}

// A unit merged into an earlier one's changeset writes in it: no
// changeset of its own, its items and the observers' event in that one,
// which it moves to its time; its outcome names it.
func TestAUnitMergesIntoAnEarlierUnitsChangeset(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	first, err := f.importUnit(f.asAlice(), domain.ClientWeb, uuid.UUID{}, create("One"))
	if err != nil {
		t.Fatal(err)
	}
	var two domain.Node
	second, err := f.importUnit(f.asAlice(), domain.ClientWeb, first.ChangesetID, func(ctx context.Context, u *app.Unit) error {
		two, err = u.CreatePage(ctx, app.PageDraft{Title: "Two"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.store.changesets) != 1 || second.ChangesetID != first.ChangesetID {
		t.Fatalf("changesets %+v, outcomes %s and %s; want one, both units'", f.store.changesets, first.ChangesetID, second.ChangesetID)
	}
	if at, ok := f.store.touched[first.ChangesetID]; !ok || !at.Equal(second.At) || !second.At.After(first.At) {
		t.Errorf("the changeset's time = %v, %v; want the second unit's, %v", at, ok, second.At)
	}
	if it := f.store.items[two.ID]; it.ChangesetID != first.ChangesetID {
		t.Errorf("the second unit's item is in %s, want %s", it.ChangesetID, first.ChangesetID)
	}
	if len(o.events) != 2 || o.events[1].ChangesetID != first.ChangesetID {
		t.Errorf("events = %+v, want the second in the first's changeset", o.events)
	}
}

// A unit merges only into a changeset of its notebook, of its kind, by
// its actor and from its client: any other, or none, fails it before it
// writes.
func TestAUnitMergesOnlyIntoItsOwnKindOfChangeset(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(f *fixture, c *app.Changeset)
	}{
		{"another notebook's", func(_ *fixture, c *app.Changeset) { c.NotebookID = uuid.NewV7() }},
		{"an edit's", func(_ *fixture, c *app.Changeset) { c.Kind = domain.ChangesetEdit }},
		{"another account's", func(_ *fixture, c *app.Changeset) { c.By = uuid.NewV7() }},
		{"the API's", func(_ *fixture, c *app.Changeset) { c.Client = domain.ClientAPI }},
		{"none", func(_ *fixture, c *app.Changeset) { c.ID = uuid.NewV7() }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionCreate)
			first, err := f.importUnit(f.asAlice(), domain.ClientWeb, uuid.UUID{}, create("One"))
			if err != nil {
				t.Fatal(err)
			}
			tt.edit(f, &f.store.changesets[0])
			f.rec.calls = nil
			_, err = f.importUnit(f.asAlice(), domain.ClientWeb, first.ChangesetID, create("Two"))
			if err == nil || f.called("CreateNode") || f.called("TouchChangeset") || f.called("CreateChangeset") {
				t.Errorf("a unit merged into %s = %v after %v; want an error before any write", tt.name, err, f.rec.calls)
			}
		})
	}
}

// An edit session's write never merges into another changeset: a unit
// that merges refuses one that resumes its session's changeset.
func TestAMergingUnitRefusesAnEditSessionsWrite(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	n := f.page("Notes", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Hour))
	s.ChangesetID, s.Revision = uuid.NewV7(), 1
	f.store.sessions[s.ID] = s
	first, err := f.importUnit(f.asAlice(), domain.ClientWeb, uuid.UUID{}, create("One"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.importUnit(f.asAlice(), domain.ClientWeb, first.ChangesetID, func(ctx context.Context, u *app.Unit) error {
		_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: n.ID, Content: "mine", Base: 1, EditSession: s.ID})
		return err
	})
	if err == nil || f.store.contents[n.ID].Revision != 1 {
		t.Errorf("an edit session's write in a merging unit = %v, revision %d; want an error, nothing written", err, f.store.contents[n.ID].Revision)
	}
}
