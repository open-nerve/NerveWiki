package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The reads' answers when what they name is not visible (M6/P5 design 2):
// the page and notebook modules' codes, which hide whether it exists.
var (
	// ErrPageNotFound: no page has the id, it or its notebook is deleted,
	// or the caller has no role in its notebook.
	ErrPageNotFound = shared.NewError(shared.KindNotFound, "page.not_found", "No such page.")
	// ErrNotebookNotFound: no notebook has the id, it is deleted, or the
	// caller has no role in it.
	ErrNotebookNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
)

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
