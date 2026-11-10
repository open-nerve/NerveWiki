package domain

import (
	"time"
	"uuid"
)

// UpgradeState is where a notebook's upgrade to an LLM Wiki is (M11 design
// 4.3): under way while upgrading, reviewing or rolling back; approved or
// rolled back once it ended.
type UpgradeState string

// The upgrade's states, each notebook_upgrades.state's value.
const (
	UpgradeUpgrading   UpgradeState = "upgrading"
	UpgradeReviewing   UpgradeState = "reviewing"
	UpgradeRollingBack UpgradeState = "rolling_back"
	UpgradeApproved    UpgradeState = "approved"
	UpgradeRolledBack  UpgradeState = "rolled_back"
)

// UpgradeEnding is why an upgrade rolls back: abandoned by an admin, or
// rejected at its review.
type UpgradeEnding string

// The endings, each notebook_upgrades.ending's value.
const (
	UpgradeAbandoned UpgradeEnding = "abandoned"
	UpgradeRejected  UpgradeEnding = "rejected"
)

// Upgrade is a notebook's upgrade as the wiki module keeps it (M11 design
// 4.2): who started it and when, the source root it proposes, the pages
// its start wrote (SeededRoot: the source root is one; SeededAgentsID: the
// AGENTS page, nil for none), its template's language, its baseline's
// counts, its submission and the report it kept (M11/P2 design 3.12), its
// rollback's job and how the rollback ended. Ending is "" for none; the
// pointers are nil until their step.
type Upgrade struct {
	ID                   uuid.UUID
	NotebookID           uuid.UUID
	State                UpgradeState
	Ending               UpgradeEnding
	SourceRootID         uuid.UUID
	SeededRoot           bool
	SeededAgentsID       *uuid.UUID
	Locale               Locale
	BaselineNodes        int
	BaselinePages        int
	BaselineWanted       int
	BaselineWantedCapped bool
	SubmissionID         *uuid.UUID
	SubmittedBy          *uuid.UUID
	SubmittedAt          *time.Time
	Report               *UpgradeReport
	RollbackJobID        *uuid.UUID
	RollbackComplete     *bool
	RollbackLeft         *RollbackLeft
	CreatedBy            uuid.UUID
	CreatedAt            time.Time
}

// RollbackLeft is where a rollback ended short (M11 design 4.10): the
// baseline nodes not back in place, the pages not written back and the
// new nodes still there.
type RollbackLeft struct {
	Nodes   int
	Pages   int
	Created int
}
