package app

import (
	"context"
	"log/slog"
	"uuid"
)

// Ingestion mirrors only the metadata the page owner can prove. Linking owns
// these DTOs; the composition root explicitly converts its owner's values.
type IngestNotebook struct {
	Mode         string
	SourceRootID *uuid.UUID
}
type IngestNodeBefore struct {
	ID                                   uuid.UUID
	Exists                               bool
	Kind                                 string
	InSource, IsSourceRoot, IsRootAgents bool
	PageBytes                            int64
	PageBytesKnown                       bool
	Revision                             int
	RevisionKnown                        bool
}
type IngestBefore interface {
	NodesBefore(context.Context, []uuid.UUID) ([]IngestNodeBefore, error)
}
type IngestHead struct {
	ID                                   uuid.UUID
	Exists, Live                         bool
	Kind                                 string
	HasContent                           bool
	Revision                             int
	PageBytes                            int64
	InSource, IsSourceRoot, IsRootAgents bool
}
type IngestHeadBatch struct {
	Rows []IngestHead
	Last *uuid.UUID
	Done bool
}
type IngestMetadata interface {
	Now(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]IngestHead, error)
	Heads(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, int) (IngestHeadBatch, error)
}
type IngestRevision struct {
	ID       uuid.UUID
	Revision int
}
type IngestIndexedHead struct {
	ID                                       uuid.UUID
	Present                                  bool
	PageRevision, IndexedRevision, Extractor int
	FrontmatterValid                         bool
}
type IngestIndexHeads interface {
	IndexedHeads(context.Context, uuid.UUID, []IngestRevision) ([]IngestIndexedHead, error)
}
type OldFactState uint8

const (
	OldFactsUnknown OldFactState = iota
	OldFactsKnownAbsent
	OldFactsKnownIneligible
	OldFactsKnown
)

type OldUnknownExtent uint8

const (
	OldUnknownAll OldUnknownExtent = iota
	OldUnknownPossibleTargets
)

type OldUncertainty struct {
	Extent          OldUnknownExtent
	PossibleTargets []uuid.UUID
}
type ReferenceOrigin uint8

const (
	ReferenceBody ReferenceOrigin = iota
	FirstSourcesSlot
)

type QualifiedReference struct {
	TargetID      uuid.UUID
	TargetIsAsset bool
	Origin        ReferenceOrigin
	Start         int32
}
type OldQualifiedFacts struct {
	PageID     uuid.UUID
	State      OldFactState
	References []QualifiedReference
	Unknown    OldUncertainty
}
type OldSourceFacts struct {
	PageID   uuid.UUID
	State    OldFactState
	AssetIDs []uuid.UUID
	Unknown  OldUncertainty
}
type QualifiedBefore struct {
	Heads IngestIndexHeads
	Reads *LintReads
}
type IngestSource struct {
	ID       uuid.UUID
	Asset    bool
	Revision int
}
type IngestCandidateQuery struct {
	NotebookID        uuid.UUID
	Candidates        []IngestSource
	EligibleEmitters  []uuid.UUID
	CurrentSources    []IngestRevision
	UntouchedSources  []uuid.UUID
	PossibleOldDirect []uuid.UUID
	ParentIDs         []uuid.UUID
}
type IngestCandidateEvidence struct {
	Candidates []IngestCandidateFact
	Parents    []IngestParentFact
}
type IngestCandidateFact struct {
	ID                               uuid.UUID
	Asset, Direct, FollowedUntouched bool
}
type IngestParentFact struct {
	ID     uuid.UUID
	Direct bool
}
type IngestCandidateReads interface {
	NovelSources(context.Context, IngestCandidateQuery) (IngestCandidateEvidence, error)
}

// IngestOldBodyReadRows bounds one fully consumed private SQL result. The
// app still exposes at most LintReadRows per Next; no complete relation is capped.
const IngestOldBodyReadRows = 16384

// IngestOldBodyReads is only for the locked pre-Replace old-F capture. Its
// emitter relation has already passed the complete Before/current-head proof.
// It is not a public Capture or Lint reader; a transaction alone is insufficient.
// take is in [1, IngestOldBodyReadRows]. Each result reaches native EOF/Err and
// Close before return; even a short nonempty batch requires a terminal empty
// query. A native error anywhere in one physical batch precedes app processing
// of that batch. Errors are terminal and never trigger a fallback or retry.
type IngestOldBodyReads interface {
	OldBodyReferences(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error)
}
type IngestMarkResult struct{ Added, Count int }
type IngestMarks interface {
	Add(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) (IngestMarkResult, error)
}

// Ingest is the normal write observer's dependency bundle. Reindex has none:
// it updates the index directly and cannot create edit history.
type Ingest struct {
	Metadata   IngestMetadata
	Heads      IngestIndexHeads
	Reads      *LintReads
	OldBodies  IngestOldBodyReads
	Candidates IngestCandidateReads
	Marks      IngestMarks
	Logger     *slog.Logger
}
