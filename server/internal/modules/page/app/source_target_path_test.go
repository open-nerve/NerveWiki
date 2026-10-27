package app_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceTargetPathRequiresCurrentVisibilityAndNotebook(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	p := f.page("Visible", nil, 0)
	read := app.NewSourceTargetPath(f.notebooks, f.store, f.auth)
	if path, err := read.Execute(f.asAlice(), f.eng, p.ID); err != nil || path != "Visible.md" {
		t.Errorf("visible target=%q/%v", path, err)
	}
	for _, id := range []uuid.UUID{uuid.NewV7(), uuid.Nil()} {
		if path, err := read.Execute(f.asAlice(), id, p.ID); err == nil || path != "" {
			t.Errorf("foreign notebook leaks %q/%v", path, err)
		}
	}
	delete(f.auth.grants, domain.ActionRead)
	f.auth.forbidden[domain.ActionRead] = true
	if path, err := read.Execute(f.asAlice(), f.eng, p.ID); err == nil || path != "" {
		t.Errorf("current read denial leaked %q/%v", path, err)
	}
}
