package mcpadapter

import (
	"errors"
	"strconv"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lockMember is a lock as a tool's error gives it (M9/P2 design 3.10.4):
// the page and its path, null when it could not be read, its holder, and
// whether the holder is the caller.
type lockMember struct {
	PageID      uuid.UUID `json:"page_id"`
	Path        *string   `json:"path"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	You         bool      `json:"you"`
}

// holderMember is a lock without its path: lock beside locks, whose first
// item has it, or lock when the answer has no room for its path.
type holderMember struct {
	PageID      uuid.UUID `json:"page_id"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	You         bool      `json:"you"`
}

// maxNamedHolders is how many holders a lock's text names, the others
// counted (M9 design 4.7).
const maxNamedHolders = 3

// lockErr is l, a refusal of edit locks, as a tool answers it (M9/P2
// design 3.10.4, 3.11.5): page.locked of write or edit, the one page path
// names, its lock with its path while the answer holds it; page.locked of
// a deletion of path, every page held in its subtree; linking.pages_locked
// of a move from path, the pages whose links it would rewrite. A list
// gives as many of its first mcpserver.MaxErrorLocks locks, each with its
// path, as the answer holds, and their number. The text names the
// holders, c's caller as "you, in a browser", and never their pages'
// paths but path.
func lockErr(err error, l *domain.LockPaths, c mcpserver.Call, path string) error {
	return lockErrOf(err, l, c, path, "Moving "+domain.Quoted(path))
}

// lockErrOf is lockErr, its linking.pages_locked saying what would rewrite
// the links: mover, a move's "Moving <path>", or a batch's renames and
// moves, which the first op that moves need not be (M9/P3 design 3.9.2).
func lockErrOf(err error, l *domain.LockPaths, c mcpserver.Call, path, mover string) error {
	var e *shared.Error
	if !errors.As(l.Err, &e) || e.Lock == nil && len(e.Locks) == 0 {
		return err
	}
	member := func(h shared.LockHolder) lockMember {
		m := lockMember{PageID: h.PageID, UserID: h.UserID, DisplayName: h.DisplayName, You: h.UserID == c.UserID}
		if p, ok := l.Paths[h.PageID]; ok {
			m.Path = &p
		}
		return m
	}
	holder := func(h shared.LockHolder) holderMember {
		return holderMember{PageID: h.PageID, UserID: h.UserID, DisplayName: h.DisplayName, You: h.UserID == c.UserID}
	}
	at := domain.Quoted(path)
	if len(e.Locks) == 0 {
		lock := *e.Lock
		text := "page.locked: " + at + " is being edited by " + nameOf(lock) + " in the web editor; try again after they leave " +
			"edit mode. The agent cannot take the lock over."
		if lock.UserID == c.UserID {
			text = "page.locked: " + at + " is being edited by you, in a browser; try again after you leave edit mode. " +
				"The agent cannot take the lock over."
		}
		return &mcpserver.ToolError{Err: err, Fit: &mcpserver.ErrorFit{N: 1, Build: func(k int) (string, map[string]any) {
			if k == 0 {
				return text, map[string]any{"lock": holder(lock)}
			}
			return text, map[string]any{"lock": member(lock)}
		}}}
	}
	pages, names := domain.PagesAndAttachments(len(e.Locks), 0), holdersText(e.Locks, c.UserID)
	text := "linking.pages_locked: " + mover + " would rewrite links in " + pages + " being edited in the web editor, " +
		names + ". Try again after they leave edit mode, or move with update_links false to leave their links as they are; " +
		"the agent cannot take the locks over."
	if errors.Is(err, domain.ErrLocked) {
		text = "page.locked: " + at + " has " + pages + " being edited in the web editor, " + names +
			": deleting it would end their edits. Try again after they leave edit mode; the agent cannot take the locks over."
	}
	return &mcpserver.ToolError{Err: err, Fit: &mcpserver.ErrorFit{N: min(len(e.Locks), mcpserver.MaxErrorLocks),
		Build: func(k int) (string, map[string]any) {
			locks := make([]lockMember, k)
			for i, h := range e.Locks[:k] {
				locks[i] = member(h)
			}
			extra := map[string]any{"locks": locks, "locks_count": len(e.Locks)}
			if e.Lock != nil {
				extra["lock"] = holder(*e.Lock)
			}
			return text, extra
		}}}
}

// holdersText names the holders of locks, each once, in the locks'
// order: at most maxNamedHolders names, the rest counted, the caller me
// last as "you, in a browser" ("by Alice, Bob and by you, in a browser").
func holdersText(locks []shared.LockHolder, me uuid.UUID) string {
	var others []string
	seen, you := map[uuid.UUID]bool{}, false
	for _, h := range locks {
		switch {
		case seen[h.UserID]:
		case h.UserID == me:
			you = true
		default:
			others = append(others, nameOf(h))
		}
		seen[h.UserID] = true
	}
	room := maxNamedHolders
	if you {
		room--
	}
	named := others[:min(len(others), room)]
	items := append([]string(nil), named...)
	switch rest := len(others) - len(named); {
	case rest == 1:
		items = append(items, "1 other")
	case rest > 1:
		items = append(items, strconv.Itoa(rest)+" others")
	}
	if you {
		items = append(items, "by you, in a browser")
	}
	text := listed(items)
	if len(others) > 0 {
		text = "by " + text
	}
	return text
}

// nameOf is a holder's display name; "someone" for an account that has
// none to give, its name read empty.
func nameOf(h shared.LockHolder) string {
	if h.DisplayName == "" {
		return "someone"
	}
	return h.DisplayName
}

// listed is items as a list in words: "a", "a and b", "a, b and c".
func listed(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
