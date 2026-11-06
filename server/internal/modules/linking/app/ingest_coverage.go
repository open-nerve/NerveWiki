package app

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type CoverageState uint8

const (
	CoverageIncomplete CoverageState = iota
	CoverageUnknown
	CoverageComplete
)

type CoverageReason uint8

const (
	CoverageNoGap CoverageReason = iota
	MissingIndex
	StaleExtractor
	RevisionMismatch
	WorkBudget
)

type ingestScope struct {
	identity           *ingestUnit
	notebookID, rootID uuid.UUID
	extractor          int
}

func (r CoverageReason) String() string {
	switch r {
	case MissingIndex:
		return "missing_index"
	case StaleExtractor:
		return "stale_extractor"
	case RevisionMismatch:
		return "revision_mismatch"
	case WorkBudget:
		return "work_budget"
	default:
		return "no_gap"
	}
}

type coverageResult struct {
	state                             CoverageState
	reason                            CoverageReason
	visited                           int64
	scope                             ingestScope
	excludedW, eligibleEmitters       []uuid.UUID
	currentSources                    []IngestRevision
	untouchedSources, unknownFParents []uuid.UUID
}

// heads counts the current heads the proof read before it ended.
func (r coverageResult) heads() int { return len(r.eligibleEmitters) + len(r.currentSources) }

type IngestCoverage struct {
	Metadata IngestMetadata
	Heads    IngestIndexHeads
}

func validIngestNode(h IngestHead) bool {
	return h.PageBytes >= 0 && h.Revision >= 0 && (!h.Exists || h.Kind == "page" || h.Kind == "asset") &&
		(!h.Live || h.Exists && (h.Kind != "page" || h.HasContent && h.Revision > 0))
}

// check visits raw nodes, including trash, and derives its own terminal proof.
// A full 512-row batch never certifies EOF; zero UUID is an ordinary first ID.
func (c IngestCoverage) check(ctx context.Context, scope ingestScope, w []uuid.UUID) (coverageResult, error) {
	out := coverageResult{scope: scope, excludedW: slices.Clone(w)}
	if scope.identity == nil || scope.extractor != domain.Extractor || !orderedIngestIDs(w) {
		return out, errors.New("ingest coverage: invalid private scope")
	}
	var after *uuid.UUID
	unknown, rootSeen := false, false
	for {
		started := time.Now()
		batch, err := c.Metadata.Heads(ctx, scope.notebookID, scope.rootID, after, LintReadRows)
		if err != nil {
			return out, err
		}
		if len(batch.Rows) > LintReadRows || batch.Done != (len(batch.Rows) < LintReadRows) {
			return out, errors.New("ingest coverage: invalid raw terminal state")
		}
		var revisions []IngestRevision
		for i, h := range batch.Rows {
			if !h.Exists || !validIngestNode(h) || (i > 0 && batch.Rows[i-1].ID.Compare(h.ID) >= 0) ||
				(after != nil && after.Compare(h.ID) >= 0) {
				return out, errors.New("ingest coverage: malformed raw metadata")
			}
			out.visited++
			if h.ID == scope.rootID {
				rootSeen = h.Live && h.Kind == "page" && h.IsSourceRoot && !h.InSource
			}
			if !h.Live || h.Kind != "page" || h.IsSourceRoot || h.IsRootAgents {
				continue
			}
			_, changed := slices.BinarySearchFunc(w, h.ID, uuid.UUID.Compare)
			if h.InSource {
				if h.PageBytes > 0 {
					revisions = append(revisions, IngestRevision{ID: h.ID, Revision: h.Revision})
				}
			} else if !changed {
				revisions = append(revisions, IngestRevision{ID: h.ID, Revision: h.Revision})
			}
		}
		heads, err := c.Heads.IndexedHeads(ctx, scope.notebookID, revisions)
		if err != nil {
			return out, err
		}
		if len(heads) != len(revisions) {
			return out, errors.New("ingest coverage: incomplete indexed heads")
		}
		at := 0
		for i, h := range heads {
			if h.ID != revisions[i].ID || h.PageRevision != revisions[i].Revision {
				return out, errors.New("ingest coverage: malformed indexed head")
			}
			for batch.Rows[at].ID != h.ID {
				at++
			}
			n := batch.Rows[at]
			_, changed := slices.BinarySearchFunc(w, h.ID, uuid.UUID.Compare)
			if !currentIngestHead(h, n.Revision) {
				if n.InSource {
					if !changed {
						out.unknownFParents = append(out.unknownFParents, h.ID)
					}
					continue
				}
				unknown = true
				switch {
				case !h.Present:
					out.reason = MissingIndex
				case h.Extractor != domain.Extractor:
					out.reason = StaleExtractor
				default:
					out.reason = RevisionMismatch
				}
				continue
			}
			if n.InSource {
				out.currentSources = append(out.currentSources, IngestRevision{ID: h.ID, Revision: n.Revision})
				if !changed {
					out.untouchedSources = append(out.untouchedSources, h.ID)
				}
			} else {
				out.eligibleEmitters = append(out.eligibleEmitters, h.ID)
			}
		}
		if len(batch.Rows) > 0 {
			last := batch.Rows[len(batch.Rows)-1].ID
			if batch.Last == nil || *batch.Last != last {
				return out, errors.New("ingest coverage: cursor is not raw tail")
			}
			after = &last
		} else if batch.Last != nil {
			return out, errors.New("ingest coverage: cursor on empty batch")
		}
		if batch.Done {
			break
		}

		// Estimate only the next complete batch from the one just completed.
		// This never certifies an unfinished prefix or masks a database/context
		// error, and never downgrades already-observed EOF. The original request
		// deadline is the only budget; there is no notebook-wide row cap.
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= time.Since(started) {
			out.state, out.reason = CoverageIncomplete, WorkBudget
			return out, nil
		}
	}
	if !rootSeen {
		return out, errors.New("ingest coverage: invalid source root")
	}
	out.state = CoverageComplete
	if unknown {
		out.state = CoverageUnknown
	}
	return out, ctx.Err()
}
