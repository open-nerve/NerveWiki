package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SourceValue is the bounded metadata used by a source write. BeforeSourceHash
// is the nullable derived hash before the write, for pages and attachments.
type SourceValue struct {
	ID                     uuid.UUID
	Kind                   domain.Kind
	Hash, BeforeSourceHash []byte
	Bytes                  int64
}

// SourceKey keeps page and attachment hashes in separate namespaces.
type SourceKey struct {
	Kind domain.Kind
	Hash [32]byte
}

// SourceProbe reads at most Take matches strictly after After, or from the
// first UUID when After is nil. A request's total Take is at most MaxUnitNodes.
type SourceProbe struct {
	Key   SourceKey
	After *uuid.UUID
	Take  int
}

// SourceMatch names its zero-based input probe and the matching source.
type SourceMatch struct {
	Probe int
	ID    uuid.UUID
}

// SourceReads reads metadata without bodies, and bounded indexed matches.
type SourceReads interface {
	SourceHeads(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]SourceValue, error)
	ProbeSources(context.Context, uuid.UUID, []SourceProbe) ([]SourceMatch, error)
}

// SourceWriter maintains the hashes of a unit's changed nodes in its transaction.
type SourceWriter interface {
	SourceHeads(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]SourceValue, error)
	SetSourceHashes(context.Context, uuid.UUID, []SourceValue) (map[uuid.UUID]SourceValue, error)
	SourceBaselines(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]SourceValue, error)
}

// SourceStore is the source area's repository, including transaction-scoped
// serialization and the notebook downgrade's cleanup.
type SourceStore interface {
	SourceReads
	SourceWriter
	LockSources(context.Context, uuid.UUID) error
	ForgetSources(context.Context, uuid.UUID) error
}
