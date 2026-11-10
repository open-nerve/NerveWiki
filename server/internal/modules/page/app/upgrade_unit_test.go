package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The write unit's part in an upgrade's guardrails (M11/P2 design 3.3,
// 3.5): the body's hash of each content step the guard compares, the hash
// it writes over, and the ledger.

// A content step carries the body's hash of what it writes while its
// notebook is upgrading, as the guard compares it (UpgradeBodyHash, the
// frontmatter cut where the unit's Markdown tells), but for the system's
// rewrite, which the guard lets by unread; none while reviewing, rolling
// back, or with no upgrade. It carries the hash of the content it writes
// over, as the unit locked it, whenever the notebook has an upgrade, the
// ledger's before; none with no upgrade.
func TestTheUnitHashesTheBodyWhileUpgrading(t *testing.T) {
	const content = "---\na: 2\n---\nText"
	current := uuid.NewV7()
	for _, tt := range []struct {
		name         string
		state        domain.UpgradeState // none: no upgrade
		rewrite      bool
		body, before bool
	}{
		{"upgrading", domain.UpgradeUpgrading, false, true, true},
		{"upgrading, the system's rewrite", domain.UpgradeUpgrading, true, false, true},
		{"reviewing", domain.UpgradeReviewing, false, false, true},
		{"rolling back", domain.UpgradeRollingBack, false, false, true},
		{"no upgrade", "", false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			if tt.state != "" {
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(tt.state, current)}
			}
			p := f.page("Plan", nil, 0)
			f.content(p.ID, "---\na: 1\n---\nText", 2)
			locked := f.store.contents[p.ID].Hash
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{g}
			if _, err := f.run(f.asAlice(), agent, func(ctx context.Context, u *app.Unit) error {
				_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: p.ID, Content: content, Base: 2, Rewrite: tt.rewrite})
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if len(g.steps) != 1 || g.steps[0].Content == nil {
				t.Fatalf("the guard saw %d steps, want one content step", len(g.steps))
			}
			var body, before []byte
			if tt.body {
				body = domain.UpgradeBodyHash(content, len("---\na: 2\n---\n"))
			}
			if tt.before {
				before = locked
			}
			c := g.steps[0].Content
			if !sameBytes(c.BodyHash, body) || !sameBytes(c.BeforeHash, before) {
				t.Errorf("the step's body hash %x, before %x; want %x, %x", c.BodyHash, c.BeforeHash, body, before)
			}
		})
	}
}

// sameBytes tells a and b equal, nil none but nil.
func sameBytes(a, b []byte) bool { return (a == nil) == (b == nil) && bytes.Equal(a, b) }

// Each content step of a notebook with an upgrade writes the upgrade's
// ledger once, the page's hash after it (M11/P2 design 3.5): upgrading,
// over the hash the unit locked, the participants' rewrites too; rolling
// back, over none; a page's creation, though it writes a content, and a
// rename write none, nor does any step of a notebook with no upgrade. The
// ledger's error is the unit's, nothing written.
func TestTheUnitKeepsTheLedger(t *testing.T) {
	current := uuid.NewV7()
	type written struct {
		node          string
		before, after string
	}
	for _, tt := range []struct {
		name  string
		state domain.UpgradeState // none: no upgrade
		want  []written           // the pages' names, their hashes' contents, "" for none
	}{
		{"upgrading", domain.UpgradeUpgrading, []written{{"Plan", "plan", "plan 2"}, {"Notes", "[[Map]]", "[[Moved]]"}}},
		{"rolling back", domain.UpgradeRollingBack, []written{{"Plan", "", "plan 2"}, {"Notes", "", "[[Moved]]"}}},
		{"no upgrade", "", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			own := uuid.UUID{}
			if tt.state != "" {
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(tt.state, current)}
			}
			if tt.state == domain.UpgradeRollingBack {
				own = current // a rollback's unit is the upgrade's own
			}
			plan, notes, target := f.page("Plan", nil, 0), f.page("Notes", nil, 1), f.page("Map", nil, 2)
			f.content(plan.ID, "plan", 2)
			f.content(notes.ID, "[[Map]]", 3)
			p := &participant{recorder: f.rec}
			f.partakers = []app.Participant{p}
			names := map[uuid.UUID]string{plan.ID: "Plan", notes.ID: "Notes"}
			_, err := f.writer().Run(f.asAlice(), app.UnitSpec{NotebookID: f.eng, Action: domain.ActionRename, Tree: true, Client: agent,
				Options: app.Options{UpdateLinks: true}, NotFound: domain.ErrNotFound, UpgradeOwn: own},
				func(ctx context.Context, u *app.Unit) error {
					if _, err := u.CreatePage(ctx, app.PageDraft{Title: "New", Content: "new"}); err != nil {
						return err
					}
					if _, err := u.WriteContent(ctx, app.ContentWrite{NodeID: plan.ID, Content: "plan 2", Base: 2}); err != nil {
						return err
					}
					p.write = &app.ContentWrite{NodeID: notes.ID, Content: "[[Moved]]", Base: 3} // the rename's rewrite of Notes
					_, err := u.Rename(ctx, target.ID, "Moved")
					return err
				})
			if err != nil {
				t.Fatal(err)
			}
			var got []written
			for _, w := range f.ledger.writes {
				if w.upgrade != current {
					t.Errorf("a write of the ledger of the upgrade %s, want %s", w.upgrade, current)
				}
				got = append(got, written{names[w.node], fmt.Sprintf("%x", w.before), fmt.Sprintf("%x", w.after)})
			}
			var want []written
			for _, w := range tt.want {
				before := ""
				if w.before != "" {
					before = fmt.Sprintf("%x", hashOf(w.before))
				}
				want = append(want, written{w.node, before, fmt.Sprintf("%x", hashOf(w.after))})
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("the ledger took %v, want %v", got, want)
			}
		})
	}

	f := newFixture()
	f.grant(domain.ActionRename)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(domain.UpgradeUpgrading, current)}
	plan := f.page("Plan", nil, 0)
	f.content(plan.ID, "plan", 2)
	failure := errors.New("the database is gone")
	f.ledger.err = failure
	_, err := f.run(f.asAlice(), agent, func(ctx context.Context, u *app.Unit) error {
		_, err := u.WriteContent(ctx, app.ContentWrite{NodeID: plan.ID, Content: "plan 2", Base: 2})
		return err
	})
	if !errors.Is(err, failure) || !f.tx.rolledBack {
		t.Errorf("a unit whose ledger fails = %v, rolled back %v; want its error, rolled back", err, f.tx.rolledBack)
	}
}
