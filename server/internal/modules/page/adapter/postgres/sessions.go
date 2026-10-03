package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// sessionOf is an edit_sessions row: a session that has not written has
// neither changeset nor revision, one not ended no end (the table's checks
// keep each pair or trio together).
func sessionOf(r gen.EditSession) app.EditSession {
	s := app.EditSession{
		ID: r.ID, NodeID: r.NodeID, NotebookID: r.NotebookID, UserID: r.UserID, Client: domain.Client(r.Client),
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt,
	}
	if r.ChangesetID != nil && r.Revision != nil {
		s.ChangesetID, s.Revision = *r.ChangesetID, int(*r.Revision)
	}
	if r.EndedReason != nil && r.EndedByID != nil && r.EndedAt != nil {
		s.EndedReason, s.EndedByID, s.EndedAt = domain.EndReason(*r.EndedReason), *r.EndedByID, *r.EndedAt
	}
	return s
}

func sessionsOf(rows []gen.EditSession) []app.EditSession {
	out := make([]app.EditSession, len(rows))
	for i, r := range rows {
		out[i] = sessionOf(r)
	}
	return out
}

// CreateSession implements app.SessionWriter.
func (s *Store) CreateSession(ctx context.Context, e app.EditSession) error {
	if err := s.queries(ctx).CreateSession(ctx, gen.CreateSessionParams{
		ID: e.ID, NodeID: e.NodeID, NotebookID: e.NotebookID, UserID: e.UserID, Client: string(e.Client),
		Now: e.CreatedAt, ExpiresAt: e.ExpiresAt,
	}); err != nil {
		return fmt.Errorf("create edit session: %w", err)
	}
	return nil
}

// LockSession implements app.SessionWriter.
func (s *Store) LockSession(ctx context.Context, id uuid.UUID) (app.EditSession, error) {
	row, err := s.queries(ctx).LockSession(ctx, id)
	if err != nil {
		return app.EditSession{}, notFound("lock edit session", err)
	}
	return sessionOf(row), nil
}

// SetSessionWrite implements app.SessionWriter. The unit holds the row
// FOR UPDATE, so it is there: one not updated fails the write rather than
// leave the session resuming an older changeset (M4/P4 review T1).
func (s *Store) SetSessionWrite(ctx context.Context, id, changesetID uuid.UUID, revision int) error {
	r := int32(revision)
	n, err := s.queries(ctx).SetSessionWrite(ctx, gen.SetSessionWriteParams{ID: id, ChangesetID: &changesetID, Revision: &r})
	if err != nil {
		return fmt.Errorf("set edit session's write: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("set edit session's write: session %s updated %d rows, not 1", id, n)
	}
	return nil
}

// DeleteNodeSessions implements app.SessionWriter.
func (s *Store) DeleteNodeSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	rows, err := s.queries(ctx).DeleteNodeSessions(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delete the pages' edit sessions: %w", err)
	}
	return sessionsOf(rows), nil
}

// DeleteExpiredSessionsOf implements app.SessionWriter.
func (s *Store) DeleteExpiredSessionsOf(ctx context.Context, id uuid.UUID, now time.Time) error {
	if _, err := s.queries(ctx).DeleteExpiredSessionsOf(ctx, gen.DeleteExpiredSessionsOfParams{NodeID: id, Now: now}); err != nil {
		return fmt.Errorf("delete the page's expired edit sessions: %w", err)
	}
	return nil
}

// EndAliveSessions implements app.SessionWriter.
func (s *Store) EndAliveSessions(ctx context.Context, e app.SessionsEnd) ([]app.EditSession, error) {
	rows, err := s.queries(ctx).EndAliveSessions(ctx, gen.EndAliveSessionsParams{
		Reason: string(e.Reason), ByID: e.By, At: e.At, Until: e.Until, NodeID: e.NodeID, UserID: e.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("end the page's edit sessions: %w", err)
	}
	return sessionsOf(rows), nil
}

// AliveSessionsOf implements app.AliveSessions.
func (s *Store) AliveSessionsOf(ctx context.Context, ids []uuid.UUID, now time.Time) ([]app.EditSession, error) {
	rows, err := s.queries(ctx).AliveSessionsOf(ctx, gen.AliveSessionsOfParams{NodeIds: ids, Now: now})
	if err != nil {
		return nil, fmt.Errorf("read the pages' edit sessions: %w", err)
	}
	return sessionsOf(rows), nil
}

// DeleteNotebookSessions implements app.NotebookPages.
func (s *Store) DeleteNotebookSessions(ctx context.Context, ids []uuid.UUID) ([]app.EditSession, error) {
	rows, err := s.queries(ctx).DeleteNotebookSessions(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delete the notebooks' edit sessions: %w", err)
	}
	return sessionsOf(rows), nil
}

// FindLiveSession implements app.Sessions.
func (s *Store) FindLiveSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	row, err := s.queries(ctx).FindLiveSession(ctx, gen.FindLiveSessionParams{ID: id, UserID: userID, Now: now})
	if err != nil {
		return app.EditSession{}, notFound("find edit session", err)
	}
	return sessionOf(row), nil
}

// HeartbeatSession implements app.Sessions.
func (s *Store) HeartbeatSession(ctx context.Context, id, userID uuid.UUID, now, until time.Time) (app.EditSession, error) {
	row, err := s.queries(ctx).HeartbeatSession(ctx, gen.HeartbeatSessionParams{ID: id, UserID: userID, Now: now, Until: until})
	if err != nil {
		return app.EditSession{}, notFound("heartbeat edit session", err)
	}
	return sessionOf(row), nil
}

// EndSession implements app.Sessions.
func (s *Store) EndSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (app.EditSession, error) {
	row, err := s.queries(ctx).EndSession(ctx, gen.EndSessionParams{ID: id, UserID: userID, Now: now})
	if err != nil {
		return app.EditSession{}, notFound("end edit session", err)
	}
	return sessionOf(row), nil
}

// FindEndedSession implements app.Sessions.
func (s *Store) FindEndedSession(ctx context.Context, id, userID uuid.UUID) (app.EditSession, error) {
	row, err := s.queries(ctx).FindEndedSession(ctx, gen.FindEndedSessionParams{ID: id, UserID: userID})
	if err != nil {
		return app.EditSession{}, notFound("find ended edit session", err)
	}
	return sessionOf(row), nil
}

// DeleteExpiredSessions implements app.ExpiredSessions.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).DeleteExpiredSessions(ctx, gen.DeleteExpiredSessionsParams{Now: now, Batch: int32(batch)})
	if err != nil {
		return int(n), fmt.Errorf("delete expired edit sessions: %w", err)
	}
	return int(n), nil
}
