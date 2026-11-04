package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// IngestNodeBefore holds only facts provable at this write unit's beginning.
// SHARE writers cannot prove an unlocked target's old revision or byte size.
type IngestNodeBefore struct {
	ID                                   uuid.UUID
	Exists                               bool
	Kind                                 domain.Kind
	InSource, IsSourceRoot, IsRootAgents bool
	PageBytes                            int64
	PageBytesKnown                       bool
	Revision                             int
	RevisionKnown                        bool
}

type IngestBefore interface {
	NodesBefore(context.Context, []uuid.UUID) ([]IngestNodeBefore, error)
}

// IngestHead is current metadata, without acquiring another content lock.
type IngestHead struct {
	ID                                   uuid.UUID
	Exists, Live                         bool
	Kind                                 domain.Kind
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
type IngestNow interface {
	Now(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]IngestHead, error)
}
type IngestMetadata interface {
	IngestNow
	Heads(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, int) (IngestHeadBatch, error)
}
