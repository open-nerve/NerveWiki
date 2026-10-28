package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A restored arrival was not a live source at the transaction's beginning.
// An original source deleted and restored within this unit still was. The
// folded write makes the later SetSourceHashes a no-op in both cases, so it
// must not be the only way to remember that difference.
func TestSourceRestorePreservesTheUnitsOriginalDuplicateTarget(t *testing.T) {
	for _, sameUnit := range []bool{false, true} {
		t.Run(map[bool]string{false: "arrives from trash", true: "deleted in this unit"}[sameUnit], func(t *testing.T) {
			f, raw, known := sourceDuplicateFixture()
			f.grant(domain.ActionRename, domain.ActionDelete, domain.ActionRestore)
			if !sameUnit {
				f.trashed(known.ID, now().Add(-time.Hour), f.alice)
			}
			restored := false
			_, err := f.run(f.asAlice(), domain.ClientAPI, func(ctx context.Context, u *app.Unit) error {
				if sameUnit {
					if _, err := u.Delete(ctx, known.ID); err != nil {
						return err
					}
				}
				root, err := f.store.FindTrashRoot(ctx, known.ID)
				if err != nil {
					return err
				}
				group, err := f.store.TrashGroup(ctx, f.eng, known.ID, root.DeletedAt)
				if err != nil {
					return err
				}
				if _, err := u.Restore(ctx, app.TrashRestore{Root: root, Group: group, Retention: retention}); err != nil {
					return err
				}
				restored = true
				_, err = u.CreatePage(ctx, app.PageDraft{Title: "Duplicate", ParentID: &raw.ID, Content: "same"})
				return err
			})
			var problem *shared.Error
			if !restored || !errors.As(err, &problem) || problem.Code != "page.source_duplicate" {
				t.Fatalf("restore then duplicate = %v (restored=%t), want duplicate after successful restore", err, restored)
			}
			if (problem.DuplicateOf != nil) != sameUnit || sameUnit && *problem.DuplicateOf != known.ID {
				t.Errorf("duplicate target = %v; want original committed source only when deleted in this unit=%t", problem.DuplicateOf, sameUnit)
			}
		})
	}
}
