package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// ErrPagesLocked: a rename or move would write again the links of pages
// being edited, and is refused as a whole (M6 design 4.6, the owner's
// decision 11.1). PagesLocked returns it with the locks member.
var ErrPagesLocked = shared.NewError(shared.KindConflict, "linking.pages_locked",
	"Pages whose links the change would write again are being edited.")

// PagesLocked is ErrPagesLocked naming the locks, one a page: each page
// and who edits it (M6/P4 design 4.1).
func PagesLocked(locks []shared.LockHolder) error {
	e := *ErrPagesLocked
	e.Locks = locks
	return &e
}
