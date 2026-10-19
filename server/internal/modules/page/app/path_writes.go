package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What MCP's write tools' use cases share (M9/P2 design 3.8): the paths
// of a lock's refusal, and a page's version now.

// withLockPaths is err with the paths of the pages it names when it is a
// refusal of edit locks, page.locked or linking.pages_locked, a problem
// with a lock or locks: the first domain.MaxNamedLocks of its pages, by
// id, read now, unlocked, after the unit rolled back (M9/P2 design 3.8). A
// page gone since has none. When the read fails, the context ended or
// the pool failing, none has one and a warn line says so: the refusal is
// answered all the same. Any other err is as it is.
func withLockPaths(ctx context.Context, paths NodePaths, notebookID uuid.UUID, logger *slog.Logger, err error) error {
	var e *shared.Error
	if !errors.As(err, &e) || e.Lock == nil && len(e.Locks) == 0 {
		return err
	}
	holders := e.Locks
	if len(holders) == 0 {
		holders = []shared.LockHolder{*e.Lock}
	}
	ids := make([]uuid.UUID, 0, min(len(holders), domain.MaxNamedLocks))
	for _, h := range holders[:min(len(holders), domain.MaxNamedLocks)] {
		ids = append(ids, h.PageID)
	}
	read, rerr := paths.Paths(ctx, notebookID, ids)
	if rerr != nil {
		logger.WarnContext(ctx, "lock paths not read", slog.String("error", rerr.Error()), slog.Int("pages", len(ids)))
		read = nil
	}
	return &domain.LockPaths{Err: err, Paths: read}
}

// currentVersion is the version of the page id now, read in the
// transaction ctx carries, unlocked outside one: page.not_found when its
// content is gone.
func currentVersion(ctx context.Context, states PageStates, id uuid.UUID) (domain.VersionToken, error) {
	s, err := states.PageStates(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.VersionToken{}, err
	}
	state, ok := s[id]
	if !ok {
		return domain.VersionToken{}, domain.ErrNotFound
	}
	return domain.VersionToken{Node: id, Revision: state.Revision}, nil
}
