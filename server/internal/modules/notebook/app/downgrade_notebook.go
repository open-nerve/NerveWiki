package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DowngradeNotebookDeps are what DowngradeNotebook needs: the manager's,
// the writing jobs, source hashes and mode subscribers.
type DowngradeNotebookDeps struct {
	Workspaces   Workspaces
	Finder       NotebookFinder
	Notebooks    NotebookWriter
	Jobs         WritingJobs
	SourceHashes SourceHashes
	Subscribers  []ModeSubscriber
	Auth         shared.Authorizer
	Tx           shared.TxManager
	Clock        Clock
	Logger       *slog.Logger
}

// DowngradeNotebook makes an LLM Wiki a standard notebook:
// POST /api/v0/notebooks/{notebook_id}/downgrade (M10/P1 design 3.10).
type DowngradeNotebook struct {
	d DowngradeNotebookDeps
	m manager
}

// NewDowngradeNotebook returns the use case.
func NewDowngradeNotebook(d DowngradeNotebookDeps) *DowngradeNotebook {
	return &DowngradeNotebook{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute makes the LLM Wiki id a standard notebook (M10 design 4.5), in
// the order of a notebook's admin write (v0.1 design 13.1 item 5):
//
//  1. the notebook, unlocked: notebook.not_found for none, a deleted one,
//     or the zero id;
//  2. in one transaction: its workspace's row FOR SHARE, its row FOR NO KEY
//     UPDATE, the decision of notebook.change_mode (404 for a caller who
//     cannot see it, 403 for one who is no admin);
//  3. notebook.not_llm_wiki (409) for a standard notebook, another admin's
//     downgrade first among them;
//  4. transfer.busy (409) when a job that writes its tree, an import, a
//     trash restore or a revert, is queued or running: one read;
//  5. the clock, once; the row standard, its source root NULL, by the
//     caller at that time;
//  6. forget the source hashes, then the mode subscribers, in order, in
//     the transaction;
//  7. the member count, for the answer; the commit; a log line.
func (d *DowngradeNotebook) Execute(ctx context.Context, id uuid.UUID) (View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return View{}, err
	}
	// The mode read here is not looked at: another admin's downgrade may
	// commit before the lock.
	n, err := find(ctx, d.d.Finder, id)
	if err != nil {
		return View{}, err
	}
	var out View
	err = d.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, grant, err := d.m.lock(ctx, actor, domain.ActionChangeMode, n, domain.ErrNotFound)
		if err != nil {
			return err
		}
		if locked.Mode != domain.ModeLLMWiki {
			return domain.ErrNotLLMWiki
		}
		writing, err := d.d.Jobs.Writing(ctx, locked.ID)
		if err != nil {
			return err
		}
		if writing {
			return domain.ErrJobsWriting
		}
		standard, err := locked.Downgraded(d.d.Clock.Now())
		if err != nil {
			return err
		}
		if err := d.d.Notebooks.SetMode(ctx, standard, actor.UserID); err != nil {
			return err
		}
		if err := d.d.SourceHashes.Forget(ctx, standard.ID); err != nil {
			return err
		}
		change := ModeChange{
			WorkspaceID: standard.WorkspaceID, NotebookID: standard.ID, From: locked.Mode, To: standard.Mode,
			By: actor.UserID, At: standard.UpdatedAt,
		}
		if err := publishMode(ctx, d.d.Subscribers, change); err != nil {
			return err
		}
		count, err := d.d.Notebooks.CountMembers(ctx, standard.ID)
		if err != nil {
			return err
		}
		out = View{Notebook: standard, Role: grant.NotebookRole, MemberCount: count}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	d.d.Logger.InfoContext(ctx, "notebook downgraded", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
