package app

import (
	"context"
	"log/slog"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A single rename or move reads and resolves the links it reaches once
// (M12/P4/S7 design 3.2): the rewrite, the unit's participant, leaves in
// the unit's memo the links it read of the reach of the unit's changes and
// where they resolve after them; the index, its observer, takes them
// instead of reading and resolving them again. The participant leaves them
// only for a tree unit (its notebook's row held from its start: no other
// unit writes the rows either reads in between), of no batch (whose
// deferred participant's way to the observers this does not follow), that
// relocates a node (else the observer reaches nothing), and drops them
// before it writes a page; the observer takes them, whether or not it uses
// them, before it spreads its reach, and uses them only when its changes
// are the participant's, one by one. Whoever changes spread, Affected or
// resolutions, or how the two use them; adds a write between the
// participant and the observer, or a writer of page_links, page_aliases or
// nodes; lets a tree unit not hold its notebook's row; or lets a batch use
// them: looks at these conditions again (design 3.2, "以后的人改这几处时").

// unitReachKey is the memo's key of a unit's reach: no other module has it.
type unitReachKey struct{}

// unitReach is what the participant read of the reach of changes: links,
// as read, and where each resolves after changes, in links' order.
type unitReach struct {
	changes []domain.Change
	links   []Link
	after   []domain.Resolution
}

// keepReach leaves in m's memo, when m is a single rename or move of a
// tree unit that relocates a node, the links the participant read of its
// changes' reach and where they resolve after them, for the observer.
func keepReach(m Moved, relocates bool, links []Link, after []domain.Resolution) {
	if m.Memo == nil || !m.Tree || m.Batch || !relocates {
		return
	}
	m.Memo.Store(unitReachKey{}, unitReach{changes: slices.Clone(m.Changes), links: links, after: after})
}

// dropReach drops from m's memo what keepReach left: the participant
// writes a page, so the observer's reach is no longer what it read, and
// the links it read need not be kept to the unit's end.
func dropReach(m Moved) {
	if m.Memo != nil {
		m.Memo.Drop(unitReachKey{})
	}
}

// takenReach takes from e's memo what the participant left of a unit
// whose changes are e's, one by one: then the links it reached and those
// that resolve otherwise after them. ok is false, nothing taken but
// dropped, when it left none or e's changes are others: the observer reads
// them as it does any unit's.
func (x Index) takenReach(ctx context.Context, e PagesChanged) (links, changed []Link, ok bool) {
	if e.Memo == nil {
		return nil, nil, false
	}
	v, found := e.Memo.Take(unitReachKey{})
	r, isReach := v.(unitReach)
	if !found || !isReach || !slices.EqualFunc(e.Changes, r.changes, sameChange) {
		return nil, nil, false
	}
	for i, l := range r.links {
		if r.after[i] != l.Resolution {
			l.Resolution = r.after[i]
			changed = append(changed, l)
		}
	}
	if x.Logger != nil {
		x.Logger.LogAttrs(ctx, slog.LevelDebug, unitReachMessage,
			slog.String("notebook_id", e.NotebookID.String()), slog.Int("links", len(r.links)), slog.Int("changed", len(changed)))
	}
	return r.links, changed, true
}

// unitReachMessage is the debug line of an observer that took the links
// its unit's participant read.
const unitReachMessage = "the index takes the links its unit's rewrite read"

// sameChange reports whether a and b change the same node from the same
// place to the same place, with the same version.
func sameChange(a, b domain.Change) bool {
	return a.NodeID == b.NodeID && a.Revision == b.Revision && samePlace(a.Before, b.Before) && samePlace(a.After, b.After)
}

func samePlace(a, b *domain.Place) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Name == b.Name && sameParentID(a.ParentID, b.ParentID)
}

func sameParentID(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
