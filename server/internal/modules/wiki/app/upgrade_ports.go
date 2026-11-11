package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// The ports of an upgrade's start and read (M11/P1 design 3.14), which the
// composition root fills from the workspace, notebook, page, transfer and
// identity modules, and the module's own table. Each but Analyze runs in
// the transaction ctx carries.

// UpgradeWorkspaces locks a notebook's workspace's row FOR SHARE, first
// of an admin's write's locks: false for none.
type UpgradeWorkspaces interface {
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// NotebookUpgrade is a notebook's upgrade under way as the notebook module
// keeps it on the notebook's row: the zero value for none.
type NotebookUpgrade struct {
	ID    uuid.UUID
	State domain.UpgradeState
	By    uuid.UUID
	At    time.Time
}

// UpgradeNotebook is a notebook's mode, source root and upgrade, read under
// its row's lock.
type UpgradeNotebook struct {
	Mode         domain.Mode
	SourceRootID uuid.UUID
	Upgrade      NotebookUpgrade
}

// ModeChange is a change the notebook's mode subscribers follow: an
// upgrade's start is one from standard to standard.
type ModeChange struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	From, To    domain.Mode
	By          uuid.UUID
	At          time.Time
}

// UpgradeStates are the notebook module's upgrade's columns (M11/P1 design
// 3.11, 3.20): Lock locks the notebook's row FOR NO KEY UPDATE and answers
// its state, false for none; Start sets its upgrade; SetState its next
// state and Clear ends it, each of the notebook's upgrade upgradeID by by
// at at; Publish tells the mode's subscribers.
type UpgradeStates interface {
	Lock(ctx context.Context, id uuid.UUID) (UpgradeNotebook, bool, error)
	Start(ctx context.Context, id uuid.UUID, up NotebookUpgrade) error
	SetState(ctx context.Context, id, upgradeID uuid.UUID, state domain.UpgradeState, by uuid.UUID, at time.Time) error
	Clear(ctx context.Context, id, upgradeID, by uuid.UUID, at time.Time) error
	Publish(ctx context.Context, c ModeChange) error
}

// RollbackPages are the page module's part in a rollback's end (M11/P1
// design 3.18, 3.20): what the rollback of the upgrade has left to do in
// the notebook, read now; the upgrade's baseline deleted as it ends.
type RollbackPages interface {
	Left(ctx context.Context, upgradeID, notebookID uuid.UUID) (domain.RollbackLeft, error)
	DeleteBaseline(ctx context.Context, upgradeID uuid.UUID) error
}

// UpgradeBaseline is how many nodes and pages a baseline holds.
type UpgradeBaseline struct{ Nodes, Pages int }

// SessionHolder is a page an edit session holds and who holds it.
type SessionHolder struct{ PageID, UserID uuid.UUID }

// RootNode is a node alive at a notebook's root: its id, its kind and its
// title key.
type RootNode struct {
	ID   uuid.UUID
	Kind domain.NodeKind
	Key  string
}

// UpgradePages are the page module's part in an upgrade's start (M11/P1
// design 3.12): the baseline, the seeded source root's protection, the
// baseline pages' wanted counts, the statistics after the commit, the
// notebook's edit sessions and its root's nodes by title key.
type UpgradePages interface {
	Capture(ctx context.Context, upgradeID, notebookID uuid.UUID) (UpgradeBaseline, error)
	Protect(ctx context.Context, upgradeID, notebookID, rootID uuid.UUID) error
	SetWanted(ctx context.Context, upgradeID uuid.UUID, wanted map[uuid.UUID]int) error
	Analyze(ctx context.Context) error
	ClearExpiredSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) error
	LiveSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) ([]SessionHolder, int, error)
	RootNodes(ctx context.Context, notebookID uuid.UUID, keys []string) ([]RootNode, error)
}

// WritingJobs tells whether a job that writes a notebook's tree is queued
// or running: the transfer module's writing key.
type WritingJobs interface {
	Writing(ctx context.Context, notebookID uuid.UUID) (bool, error)
}

// Names are accounts' display names, by id.
type Names interface {
	DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// UpgradeRows are the module's notebook_upgrades: Insert writes an
// upgrade's row by by at at; Current reads a notebook's upgrade under way,
// its submission and report too, false for none; RollBack sets the
// upgrade id upgrading or reviewing rolling back for ending, its
// rollback's job jobID, in one statement; ContinueRollback sets the job of
// the upgrade id rolling back; EndRollback ends the upgrade id rolling
// back, rolled back, complete for left nil, and answers when its row
// changed before. Each write is by by at at, false for no row in the
// state it wants.
type UpgradeRows interface {
	Insert(ctx context.Context, u domain.Upgrade, by uuid.UUID, at time.Time) error
	Current(ctx context.Context, notebookID uuid.UUID) (domain.Upgrade, bool, error)
	RollBack(ctx context.Context, id uuid.UUID, ending domain.UpgradeEnding, jobID, by uuid.UUID, at time.Time) (bool, error)
	ContinueRollback(ctx context.Context, id, jobID, by uuid.UUID, at time.Time) (bool, error)
	EndRollback(ctx context.Context, id uuid.UUID, left *domain.RollbackLeft, by uuid.UUID, at time.Time) (time.Time, bool, error)
	// Submit sets the upgrade id reviewing, submission submissionID by by
	// at at: from upgrading, or from reviewing without a report submitted
	// at takenOver or before (M11 design 4.8 step 1); false for no row so.
	Submit(ctx context.Context, id, submissionID, by uuid.UUID, at, takenOver time.Time) (bool, error)
	// KeepReport keeps report as the submission submissionID's, reviewing
	// without one; Unsubmit sets it upgrading again, its submission
	// cleared. False for no row so.
	KeepReport(ctx context.Context, id, submissionID uuid.UUID, report domain.UpgradeReport, by uuid.UUID, at time.Time) (bool, error)
	Unsubmit(ctx context.Context, id, submissionID, by uuid.UUID, at time.Time) (bool, error)
}

// Rollbacks start an upgrade's rollback's job, the transfer module's
// (M11/P1 design 3.19): Start writes the rollback of the notebook's
// upgrade upgradeID, queued, as its caller from client ("web" or "api")
// asks, in the caller's transaction, which holds the notebook's row and
// has set the upgrade rolling back, and answers the job's id: server_busy
// when the queue is full.
type Rollbacks interface {
	Start(ctx context.Context, notebookID, upgradeID uuid.UUID, client string) (uuid.UUID, error)
}

// UpgradeView is an upgrade as its admins read it: the row, and the
// display names of who started it and who submitted it, "" for none.
type UpgradeView struct {
	Upgrade         domain.Upgrade
	StartedByName   string
	SubmittedByName string
}

// UpgradeGatePages are the page module's reads of an upgrade's check (M11
// design 4.7), each in the snapshot ctx carries.
type UpgradeGatePages interface {
	// Roots are gates 1 and 2's: the page at the notebook's root keyed
	// agents, nil for none (an attachment is none), whether its body past
	// its frontmatter and a BOM, trimmed, is not empty; whether rootID is
	// a live page of the notebook.
	Roots(ctx context.Context, notebookID, rootID uuid.UUID) (domain.UpgradeRoots, error)
	// Baseline is the upgrade's baseline rows: each node's kind and parent
	// then, a page's wanted count then.
	Baseline(ctx context.Context, upgradeID uuid.UUID) ([]domain.BaselineNode, error)
	// Preserved is gate 7's (M11 design 4.7): the baseline's nodes and
	// pages, those alive and those whose content is the ledger's, and the
	// first five that are not, by id, in one pass.
	Preserved(ctx context.Context, upgradeID uuid.UUID) (domain.Preserved, error)
	// Changesets is how many changesets the upgrade wrote.
	Changesets(ctx context.Context, upgradeID uuid.UUID) (int, error)
}
