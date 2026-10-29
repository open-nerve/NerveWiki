package app

import (
	"iter"
	"slices"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// sourceFactReplay avoids rebuilding a small plan's tree simulation for each
// precheck pass. These are retention limits, never limits on an accepted plan:
// crossing one discards the copy and leaves the original streaming replay.
// Count node occurrences, including a node visited by several actions. Empty
// actions still consume a step, and keys/hashes have a separate payload cap.
type sourceFactReplay struct {
	steps          []domain.StepFacts
	nodes, payload int
	disabled       bool
}

const sourceFactReplayPayload = 1 << 20

func (r *sourceFactReplay) remember(step domain.StepFacts) {
	if r.disabled {
		return
	}
	if len(r.steps) >= domain.MaxRevertSteps || len(step.Nodes) > domain.MaxUnitNodes-r.nodes {
		r.discard()
		return
	}
	remaining := sourceFactReplayPayload - r.payload
	take := func(n int) bool {
		if n > remaining {
			return false
		}
		remaining -= n
		return true
	}
	if step.Content != nil && !take(len(step.Content.Hash)) {
		r.discard()
		return
	}
	for _, n := range step.Nodes {
		if n.Before != nil && !take(len(n.Before.Key)) || n.After != nil && !take(len(n.After.Key)) {
			r.discard()
			return
		}
	}
	// All bounds precede copying this step. Iterators may overwrite their
	// scratch slices and pointed-to values as soon as yield returns.
	step.Nodes = slices.Clone(step.Nodes)
	for i := range step.Nodes {
		step.Nodes[i].Before = copySourceFactPlace(step.Nodes[i].Before)
		step.Nodes[i].After = copySourceFactPlace(step.Nodes[i].After)
	}
	if step.Content != nil {
		step.Content = new(*step.Content)
		step.Content.Hash = slices.Clone(step.Content.Hash)
	}
	r.steps = append(r.steps, step)
	r.nodes += len(step.Nodes)
	r.payload = sourceFactReplayPayload - remaining
}

func copySourceFactPlace(p *domain.NodePlace) *domain.NodePlace {
	if p == nil {
		return nil
	}
	out := *p
	out.Key = strings.Clone(p.Key)
	if p.ParentID != nil {
		out.ParentID = new(*p.ParentID)
	}
	return &out
}

func (r *sourceFactReplay) discard() {
	r.steps, r.nodes, r.payload, r.disabled = nil, 0, 0, true
}

func (r *sourceFactReplay) sequence(original iter.Seq[domain.StepFacts]) iter.Seq[domain.StepFacts] {
	if r.disabled {
		return original
	}
	return slices.Values(r.steps)
}
