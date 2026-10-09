package domain

import "unicode/utf8"

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

// ProblemCode is what befell a node, a code the web words.
type ProblemCode string

// The problems of an export.
const (
	// ProblemRenamed is a node exported under another path: To is where.
	ProblemRenamed ProblemCode = "renamed"
	// ProblemFileMissing is an attachment whose file is not in the store.
	ProblemFileMissing ProblemCode = "file_missing"
)

// Problem is what befell a node, at its path in the archive's vault.
type Problem struct {
	Path string
	Code ProblemCode
	// To is the path a renamed node is exported at.
	To string
}

// Counts are a report's numbers: an export's pages and attachments
// written, the nodes renamed and the attachments whose files were missing;
// an import's skipped entries (P6).
type Counts struct {
	Pages, Attachments, Renamed, Missing, Skipped int64
}

// Report is what a job tells as it ends (M7 design 4.11): its failure, its
// counts and its first MaxProblems problems.
type Report struct {
	Failure  Failure
	Counts   Counts
	Problems []Problem
	// Truncated tells problems were left out past MaxProblems.
	Truncated bool
}

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
