package domain

import (
	"unicode/utf8"
	"uuid"
)

// Failure is why a job failed, a code the web words (M7 design 4.8).
type Failure string

// The failures of an export (M7/P5 design 3.6).
const (
	// FailureInterrupted is a job the server stopped, or was restarted
	// under.
	FailureInterrupted Failure = "interrupted"
	// FailureTimeout is a job that ran longer than transfer.job_timeout.
	FailureTimeout Failure = "timeout"
	// FailureForbidden is a job whose account can no longer read the
	// notebook as it runs.
	FailureForbidden Failure = "forbidden"
	// FailureRootNotFound is an export whose page is gone as it runs.
	FailureRootNotFound Failure = "root_not_found"
	// FailureStorageFull is an archive the store had no room for.
	FailureStorageFull Failure = "storage_full"
	// FailureContributorConflict is a file a contributor added where a
	// node, or another of its files, is.
	FailureContributorConflict Failure = "contributor_conflict"
	// FailureInternal is any other failure, logged.
	FailureInternal Failure = "internal"
)

// The failures of an import's archive as a whole (M7/P6 design 3.8): it
// writes nothing then.
const (
	// FailureNotZip is an archive that is no zip, or whose end or
	// directory is broken.
	FailureNotZip Failure = "not_zip"
	// FailureTooManyEntries is an archive of more entries than
	// transfer.import_max_entries, or with a larger directory than
	// MaxDirectory.
	FailureTooManyEntries Failure = "too_many_entries"
	// FailureUnpackedTooLarge is an archive whose entries unpack to more
	// than transfer.import_max_unpacked_bytes.
	FailureUnpackedTooLarge Failure = "unpacked_too_large"
	// FailureTooManyNodes is an archive whose plan, its entries and the
	// folders their paths imply, makes more nodes than a background revert
	// reverts, the page module's MaxJobItems (M8/P4 design 2.2 (a)): an
	// import made can be undone.
	FailureTooManyNodes Failure = "too_many_nodes"
	// FailureTreeChanged is an import a page of which was deleted as it
	// ran: a later unit could not write under it.
	FailureTreeChanged Failure = "tree_changed"
	// FailureChangesetReverted is an import whose changeset was reverted
	// as it ran (M8/P3 design 3.7): its later units were not written.
	FailureChangesetReverted Failure = "changeset_reverted"
)

// The failures of a background restore and revert (M8/P4 design 3.6):
// what they wrote kept, the rest not written. A restore's tree_changed is
// a part it brought back deleted, or moved too deep, as it ran; its
// forbidden and changeset_reverted are the import's.
const (
	// FailureTrashChanged is a restore whose trash item changed after it
	// started: restored, or wrapped in another deletion.
	FailureTrashChanged Failure = "trash_changed"
	// FailureRestoreExpired is a restore whose item reached the time it can
	// be restored in as it ran.
	FailureRestoreExpired Failure = "restore_expired"
	// FailureRevertConflict is a revert a node of whose batch changed since
	// it started, or cannot go back as it was: the report's conflicts.
	FailureRevertConflict Failure = "revert_conflict"
	// FailureChangesetTrimmed is a revert of a changeset the purge took part
	// of as it ran.
	FailureChangesetTrimmed Failure = "changeset_trimmed"
	// FailurePageLocked is a revert a page of whose batch an edit session
	// holds: the report's lock.
	FailurePageLocked Failure = "page_locked"
	// FailureRevertTooLarge is a revert one of whose segments, steps no
	// batch splits, holds more pages, attachments, history or links than
	// one batch writes (M9/P3 design 3.23): what it wrote before kept.
	FailureRevertTooLarge Failure = "revert_too_large"
	// FailureRefused is a write the notebook contract refused: the report names its code and node.
	FailureRefused Failure = "refused"
)

// ProblemCode is what befell a node, a code the web words.
type ProblemCode string

// The problems of an export.
const (
	// ProblemRenamed is a node exported under another path: To is where.
	ProblemRenamed ProblemCode = "renamed"
	// ProblemFileMissing is an attachment whose file is not in the store.
	ProblemFileMissing ProblemCode = "file_missing"
)

// The problems of an import (M7/P6 design 3.8): an entry or a node
// skipped, and why. ProblemRenamed is an import's node named otherwise in
// the notebook, To its path there.
const (
	// ProblemUnsafePath is an entry whose path leaves the archive's root:
	// "..", an absolute path, a drive.
	ProblemUnsafePath ProblemCode = "unsafe_path"
	// ProblemSpecialFile is a symbolic link, or another file that is no
	// regular one.
	ProblemSpecialFile ProblemCode = "special_file"
	// ProblemEncrypted is an encrypted entry.
	ProblemEncrypted ProblemCode = "encrypted"
	// ProblemUnsupportedMethod is an entry compressed otherwise than with
	// Store or Deflate.
	ProblemUnsupportedMethod ProblemCode = "unsupported_method"
	// ProblemTooCompressed is an entry that unpacks to more than
	// MaxRatio times its packed bytes.
	ProblemTooCompressed ProblemCode = "too_compressed"
	// ProblemNameNotUTF8 is an entry whose name is not UTF-8.
	ProblemNameNotUTF8 ProblemCode = "name_not_utf8"
	// ProblemInvalidContent is a page's file that is not UTF-8, or holds
	// NUL.
	ProblemInvalidContent ProblemCode = "invalid_content"
	// ProblemTooLarge is a page's file past 5 MiB, or an attachment past
	// asset.max_bytes.
	ProblemTooLarge ProblemCode = "too_large"
	// ProblemTooDeep is a node more than MaxDepth levels deep in the
	// notebook, given where the import goes.
	ProblemTooDeep ProblemCode = "too_deep"
	// ProblemDuplicate is an entry of a path an earlier one has.
	ProblemDuplicate ProblemCode = "duplicate"
	// ProblemDuplicateSource is an import's source with bytes already present; To names that source.
	ProblemDuplicateSource ProblemCode = "duplicate_source"
	// ProblemUnreadable is an entry that does not unpack: its data broken,
	// its checksum or its size not its header's.
	ProblemUnreadable ProblemCode = "unreadable"
)

// Problem is what befell a node, at its path in the archive's vault.
type Problem struct {
	Path string
	Code ProblemCode
	// To is the path a renamed node is exported at.
	To string
}

// Counts are a report's numbers: an export's pages and attachments
// written, the nodes renamed and the attachments whose files were
// missing; an import's pages and attachments created, the nodes renamed,
// and the entries and nodes skipped; a restore's pages and attachments
// brought back, the nodes renamed, and those skipped, restored by another
// first; a revert's nodes restored, deleted and put back, its pages
// written back, and those skipped, put back by another first (M8/P4
// design 3.6).
type Counts struct {
	Pages, Attachments, Renamed, Missing, Skipped int64
	Restored, Deleted, PutBack, Written           int64
}

// Placement is where a restore put its trash item's root: under ParentID,
// nil for the notebook's root, and why, as the page module's reasons go.
type Placement struct {
	ParentID *uuid.UUID
	Reason   string
}

// Conflict is a node that kept a revert's batch from going on: why, as the
// page module's reasons go, its name, and the node holding the name of a
// title_taken.
type Conflict struct {
	NodeID   uuid.UUID
	Name     string
	Reason   string
	HolderID *uuid.UUID
}

// Lock is the edit lock a revert's batch met: the page, and who holds it.
type Lock struct {
	PageID, UserID uuid.UUID
}

// Refusal is the notebook contract's reason for refusing a job's write,
// and the page or attachment it refused. Codes from later milestones
// remain readable in reports written by another server version. DuplicateOf
// is separate from the refused NodeID and is nil for an operation-local duplicate
// or a report saved before that optional target existed.
type Refusal struct {
	Code        string
	NodeID      uuid.UUID
	DuplicateOf *uuid.UUID
}

// Report is what a job tells as it ends (M7 design 4.11): its failure, its
// counts and its first MaxProblems problems; a restore's placement, a
// revert's conflicts, the first MaxConflicts of ConflictCount, and the
// lock it met (M8/P4 design 3.6); a contract refusal's code and node
// (M10/P1 design 3.26).
type Report struct {
	Failure  Failure
	Counts   Counts
	Problems []Problem
	// Truncated tells problems were left out past MaxProblems.
	Truncated     bool
	Placed        *Placement
	Conflicts     []Conflict
	ConflictCount int
	Lock          *Lock
	Refusal       *Refusal
}

// MaxConflicts is how many of a revert's conflicts its report keeps, as
// the page module's problem names.
const MaxConflicts = 100

// MaxProblems is how many problems a report keeps, and MaxProblemPath the
// longest path of one, in bytes (M7 design 4.11).
const (
	MaxProblems    = 1000
	MaxProblemPath = 1024
)

// Add adds p, its paths cut to MaxProblemPath bytes at a character's
// boundary; past MaxProblems it only marks the report truncated.
func (r *Report) Add(p Problem) {
	if len(r.Problems) == MaxProblems {
		r.Truncated = true
		return
	}
	p.Path, p.To = cut(p.Path), cut(p.To)
	r.Problems = append(r.Problems, p)
}

// cut is s cut to MaxProblemPath bytes at a character's boundary.
func cut(s string) string {
	if len(s) <= MaxProblemPath {
		return s
	}
	n := MaxProblemPath
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
