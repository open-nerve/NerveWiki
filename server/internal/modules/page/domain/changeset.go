package domain

// ChangesetKind is what kind of write a changeset records (v0.1 design
// 3.8): an edit, or from M7 an import, whose units all write in its first
// unit's changeset (M7/P6 design 3.2); from M8 a version restored, a trash
// item restored, a changeset reverted (M8 design 4.2).
type ChangesetKind string

// The kinds of changeset.
const (
	ChangesetEdit           ChangesetKind = "edit"
	ChangesetImport         ChangesetKind = "import"
	ChangesetVersionRestore ChangesetKind = "version_restore"
	ChangesetTrashRestore   ChangesetKind = "trash_restore"
	// ChangesetRevert reverts another changeset, which it names.
	ChangesetRevert ChangesetKind = "revert"
)
