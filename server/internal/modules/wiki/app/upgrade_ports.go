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
// 3.11): Lock locks the notebook's row FOR NO KEY UPDATE and answers its
// state, false for none; Start sets its upgrade; Publish tells the mode's
// subscribers.
type UpgradeStates interface {
	Lock(ctx context.Context, id uuid.UUID) (UpgradeNotebook, bool, error)
	Start(ctx context.Context, id uuid.UUID, up NotebookUpgrade) error
	Publish(ctx context.Context, c ModeChange) error
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
// false for none.
type UpgradeRows interface {
	Insert(ctx context.Context, u domain.Upgrade, by uuid.UUID, at time.Time) error
	Current(ctx context.Context, notebookID uuid.UUID) (domain.Upgrade, bool, error)
}

// UpgradeView is an upgrade as its admins read it: the row, and the
// display names of who started it and who submitted it, "" for none.
type UpgradeView struct {
	Upgrade         domain.Upgrade
	StartedByName   string
	SubmittedByName string
}
