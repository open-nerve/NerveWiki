package app_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A seed for an upgrade is the upgrade's own unit (M11/P1 design 3.12):
// in a notebook being upgraded, the freeze its first guard, Seed with
// Upgrade the notebook's writes its pages, at the root's end, and their
// changeset records the upgrade; without it, notebook.upgrading, nothing
// more written.
func TestASeedForAnUpgradeIsItsOwn(t *testing.T) {
	current := uuid.NewV7()
	f := newFixture()
	f.grant(domain.ActionCreate)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: f.upgrading(domain.UpgradeUpgrading, current)}
	f.guards = []app.WriteGuard{app.NewFreezeGuard(f.auth, changeMode, &upgradeNodes{})}
	seeds := app.NewSeedWrites(f.writer(), f.md)
	seed := func(upgrade uuid.UUID, titles ...string) ([]uuid.UUID, error) {
		pages := make([]app.PageDraft, len(titles))
		for i, title := range titles {
			pages[i] = app.PageDraft{Title: title, Content: "# " + title}
		}
		var ids []uuid.UUID
		err := f.tx.WithinTx(f.asAlice(), func(ctx context.Context) error {
			var err error
			ids, err = seeds.Seed(ctx, app.SeedSpec{NotebookID: f.eng, At: now(), Client: domain.ClientWeb, Pages: pages, Upgrade: upgrade})
			return err
		})
		return ids, err
	}
	ids, err := seed(current, "raw", "AGENTS")
	if err != nil || len(ids) != 2 || len(f.store.nodes) != 2 || len(f.store.changesets) != 1 ||
		f.store.changesets[0].UpgradeID != current {
		t.Fatalf("Seed for the upgrade = %v, %v, %d nodes, changesets %+v; want raw and AGENTS, one changeset of the upgrade",
			ids, err, len(f.store.nodes), f.store.changesets)
	}
	if _, err := seed(uuid.UUID{}, "Other"); codeOf(err) != "notebook.upgrading" || len(f.store.nodes) != 2 {
		t.Errorf("Seed for no upgrade = %v, %d nodes; want notebook.upgrading, nothing more", err, len(f.store.nodes))
	}
}
