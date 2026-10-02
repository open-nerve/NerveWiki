package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's problems (M4 design 5; M4/P1 design 3.5).
var (
	// ErrNotFound: no node has the id, it is deleted, or the caller has no
	// role in its notebook.
	ErrNotFound = shared.NewError(shared.KindNotFound, "page.not_found", "No such page.")
	// ErrNotebookNotFound: the notebook module's notebook.not_found, for
	// the operations that name a notebook: no notebook has the id, it is
	// deleted, or the caller has no role in it.
	ErrNotebookNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
	// ErrTitleTaken: a node under the same parent has a title of the same
	// key.
	ErrTitleTaken = shared.NewError(shared.KindConflict, "page.title_taken",
		"A page or attachment under the same parent has this title.")
	// ErrTooDeep: the page would be deeper than MaxDepth.
	ErrTooDeep = shared.NewError(shared.KindConflict, "page.too_deep", "Pages nest at most 10 levels deep.")
)

// NotAllowed is 422 on field: a parent or a sibling the write names is no
// node of the notebook, or not where the write says.
func NotAllowed(field, message string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldNotAllowed, Message: message})
}
