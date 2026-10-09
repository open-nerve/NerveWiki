package app

import (
	"context"
	"uuid"
)

// Contributor adds files of its own to an export (M7 design 4.10; M7/P5
// design 3.10): the transfer module's ExportContributor, which M10
// registers. It runs in the export's snapshot.
type Contributor interface {
	Contribute(ctx context.Context, scope Scope, sink Sink) error
}

// Scope is what an export holds: its notebook, the page exported with its
// subtree (nil for the whole notebook), and its nodes with their paths in
// the vault.
type Scope struct {
	NotebookID uuid.UUID
	RootID     *uuid.UUID
	Nodes      []ScopeNode
}

// ScopeNode is a node of an export: its path in the vault is a page's
// file, or its folder ending in "/"; an attachment's file.
type ScopeNode struct {
	ID    uuid.UUID
	Asset bool
	Path  string
}

// Sink takes a contributor's files into the archive.
type Sink interface {
	// Add adds content at path, relative to the vault: names of a node's
	// rules joined by "/", where nothing of the export is; otherwise it
	// returns domain.ErrContributorConflict, and the export fails.
	Add(path string, content []byte) error
}
