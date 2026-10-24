package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A unit's memo asks each action once (M10/P1 design 3.19): the first
// Ask calls ask, the next answers what it kept, an action apart asks
// apart; a refusal, ask's forbidden or not visible, is kept as forbidden;
// a database's error is answered as it is, kept not, and asked again. The
// memo keeps any action alike: these are the module's own.
func TestAGrantIsAskedOnceAUnit(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("the database is gone")
	for _, tt := range []struct {
		name   string
		answer error
		want   error // as errors.Is tells it; nil for allowed
		asks   int   // the calls of ask by two Asks
	}{
		{"allowed", nil, nil, 1},
		{"forbidden", shared.Forbidden(), shared.Forbidden(), 1},
		{"not visible", shared.ErrNotVisible, shared.Forbidden(), 1},
		{"the database's error", failure, failure, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var g app.Grants
			asks := 0
			ask := func(context.Context) error {
				asks++
				return tt.answer
			}
			for i := range 2 {
				err := g.Ask(ctx, domain.ActionMove, ask)
				if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
					t.Errorf("Ask #%d = %v, want %v", i+1, err, tt.want)
				}
				if errors.Is(err, shared.ErrNotVisible) {
					t.Errorf("Ask #%d = %v, want no 404: the unit sees its notebook", i+1, err)
				}
			}
			if asks != tt.asks {
				t.Errorf("two Asks called ask %d times, want %d", asks, tt.asks)
			}
		})
	}

	t.Run("two actions", func(t *testing.T) {
		var g app.Grants
		asked := map[shared.Action]int{}
		for _, action := range []shared.Action{domain.ActionMove, domain.ActionRename, domain.ActionMove, domain.ActionRename} {
			if err := g.Ask(ctx, action, func(context.Context) error { asked[action]++; return nil }); err != nil {
				t.Errorf("Ask(%s) = %v, want allowed", action, err)
			}
		}
		if asked[domain.ActionMove] != 1 || asked[domain.ActionRename] != 1 {
			t.Errorf("asked %v, want each action once", asked)
		}
	})
}
