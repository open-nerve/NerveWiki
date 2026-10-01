package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Ownerless is since when a notebook has had no active admin, and who its
// last one was: its former owner, to whom it returns.
type Ownerless struct {
	Since       time.Time
	FormerOwner uuid.UUID
}

// Holding is an account's active membership of a notebook as the end of
// its workspace memberships reads it, under the notebook's lock: its role
// there, and the notebook's active admins and active explicit members, its
// own membership counted in both.
type Holding struct {
	NotebookID  uuid.UUID
	WorkspaceID uuid.UUID
	Role        shared.NotebookRole
	Admins      int
	Members     int
}

// SoleAdmin reports whether the account is the notebook's only active
// admin: the notebook becomes ownerless when the membership ends (M3
// design 4).
func (h Holding) SoleAdmin() bool { return h.Role == shared.NotebookAdmin && h.Admins == 1 }

// RuleTwo is rule two (M3 design 4) for an account that leaves its
// workspaces itself, by leaving one or by deactivating: the notebooks it
// is the only active admin of while another active explicit member is
// left, counted by workspace id; none when it may go. A notebook used by
// its workspace access alone does not count: it becomes ownerless, and a
// workspace admin takes it over.
func RuleTwo(holdings []Holding) map[uuid.UUID]int {
	blocked := map[uuid.UUID]int{}
	for _, h := range holdings {
		if h.SoleAdmin() && h.Members > 1 {
			blocked[h.WorkspaceID]++
		}
	}
	return blocked
}

// ErrSoleAdminOf is rule two's refusal: 409 notebook.sole_admin, naming the
// workspaces by slug and how many such notebooks each has, never the
// notebooks. The command line gives the reason to the server's
// administrator, who is not to read a private notebook's name, and names
// repeat.
func ErrSoleAdminOf(bySlug map[string]int) *shared.Error {
	slugs := slices.Sorted(maps.Keys(bySlug))
	counts := make([]string, len(slugs))
	for i, s := range slugs {
		counts[i] = fmt.Sprintf("%d in %s", bySlug[s], s)
	}
	return shared.NewError(shared.KindConflict, ErrSoleAdmin.Code, "The account is the only admin of notebooks with other members ("+
		strings.Join(counts, ", ")+"): another member must become their admin, or they must be deleted, first.")
}
