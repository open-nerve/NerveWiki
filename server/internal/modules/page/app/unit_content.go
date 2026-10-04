package app

import (
	"bytes"
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Writing a page's content in a unit (M4/P4 design 3.4).

// ContentWrite is a write of a page's content: the new content, checked
// and its facts taken before the transaction, the revision the writer read
// the page at, and the edit session it is made in (zero: none).
type ContentWrite struct {
	NodeID      uuid.UUID
	Content     string
	Facts       Facts
	Base        int
	EditSession uuid.UUID
}

// WriteContent writes w under the page's gate, its content row FOR NO KEY
// UPDATE: 404 for a page that is not in the notebook; 409
// page.edit_session_ended for a session that is not the writer's, from
// the unit's client, of the page, alive at the unit's time, but 409
// page.edit_session_taken_over or page.edit_session_unlocked for the
// writer's own session taken over or unlocked (M5/P1); nothing
// written when the page holds the content already, whatever the base: the
// write is done; 409 page.revision_mismatch for a base that is not the
// page's revision. It returns the page's revision as the write leaves it.
//
// An edit session's writes go to one changeset, its page's one version
// there, while the page stays at the revision the session last wrote: a
// write by anyone else in between has the session's next write start a
// new changeset, so that no version holds another's change between its
// base and its revision. A unit that writes several pages' contents under
// the notebook's FOR SHARE locks them in their ids' order (M4 design 4);
// M4's units write one.
func (u *Unit) WriteContent(ctx context.Context, w ContentWrite) (int, error) {
	return u.writeContent(ctx, w, true)
}

func (u *Unit) writeContent(ctx context.Context, w ContentWrite, participate bool) (int, error) {
	current, err := u.w.d.Nodes.LockContent(ctx, u.write.NotebookID, w.NodeID)
	if err != nil {
		return 0, found(err, domain.ErrNotFound)
	}
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, w.NodeID)
	if err != nil {
		return 0, found(err, domain.ErrNotFound)
	}
	var session EditSession
	if w.EditSession != (uuid.UUID{}) {
		if session, err = u.writersSession(ctx, w); err != nil {
			return 0, err
		}
	}
	c := u.content(n.ID, w.Content, current.Revision+1)
	switch {
	case c.ByteSize == current.ByteSize && bytes.Equal(c.Hash, current.Hash):
		return current.Revision, nil
	case w.Base != current.Revision:
		return 0, domain.ErrRevisionMismatch
	}
	resumed := session.ChangesetID != (uuid.UUID{}) && session.Revision == current.Revision
	if resumed {
		if err := u.changesetOf(session.ChangesetID); err != nil {
			return 0, err
		}
	}
	state := n.State()
	step := u.step(domain.OpContent, domain.Change{NodeID: n.ID, Before: &state, After: &state, Revision: c.Revision, Facts: w.Facts})
	step.EditSessionID = session.ID
	base := current.Revision
	err = u.apply(ctx, step, participate, func(ctx context.Context) error {
		if resumed {
			if err := u.w.d.Changesets.TouchChangeset(ctx, u.changeset, u.write.At); err != nil {
				return err
			}
		}
		if err := u.w.d.NodeWriter.WriteContent(ctx, c); err != nil {
			return err
		}
		if err := u.recordRevision(ctx, c, &base); err != nil {
			return err
		}
		if session.ID == (uuid.UUID{}) {
			return nil
		}
		return u.w.d.SessionWriter.SetSessionWrite(ctx, session.ID, u.changeset, c.Revision)
	})
	return c.Revision, err
}

// writersSession is w's edit session, locked FOR UPDATE after the page's
// content row: page.edit_session_ended unless it is the writer's, from the
// unit's client, of the page; then a tombstone says why it ended (M5
// design 4.3); then page.edit_session_ended unless it is alive at the
// unit's time. A write from another client than the one that opened the
// session would join a changeset recorded as that client's (M4/P4 review
// D1); one code for any session not the writer's tells nothing of whose
// it is.
func (u *Unit) writersSession(ctx context.Context, w ContentWrite) (EditSession, error) {
	s, err := u.w.d.SessionWriter.LockSession(ctx, w.EditSession)
	switch {
	case errors.Is(err, ErrNotFound):
		return EditSession{}, domain.ErrEditSessionEnded
	case err != nil:
		return EditSession{}, err
	case s.UserID != u.write.By || s.Client != u.write.Client || s.NodeID != w.NodeID:
		return EditSession{}, domain.ErrEditSessionEnded
	case s.EndedReason != "":
		return EditSession{}, endedError(ctx, u.w.d.Names, s)
	case !s.Alive(u.write.At):
		return EditSession{}, domain.ErrEditSessionEnded
	}
	return s, nil
}
