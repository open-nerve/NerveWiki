package app

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NewGrepParsesWith is a queue of n turns whose parse is parse: a test's
// fake, which counts the parses in flight.
func NewGrepParsesWith(n int, parse func(pattern string, ignoreCase bool) (domain.GrepPattern, error)) *GrepParses {
	q := NewGrepParses(n)
	q.parse = parse
	return q
}

// NewGrepSlotsWaiting is n slots, a call waiting wait for one: a test's
// few milliseconds, not GrepSlotWait.
func NewGrepSlotsWaiting(n int, wait time.Duration) *GrepSlots {
	s := NewGrepSlots(n)
	s.wait = wait
	return s
}

// WithLines is g splitting a content's lines by lines: a test's, which
// wraps domain.GrepLines to advance its clock as it scans for each line's
// end.
func (g *Grep) WithLines(lines func(content string, from int, yield func(line int, text string, column int) bool)) *Grep {
	c := *g
	c.lines = lines
	return &c
}

// WithMaxWork is l with its globs' work a call at most maxWork: a test's
// bound, less than domain.MaxGlobWork.
func (l *ListByPattern) WithMaxWork(maxWork int64) *ListByPattern {
	c := *l
	c.maxWork = maxWork
	return &c
}

// RevertStep is a background revert's step as its job keeps it (M9/P3
// design 3.22.1), as a test reads it: its kind and seq; a place's or a
// step aside's node, parent (the zero id for the root), name and order; a
// write back's page, base and hash; a deletion's top and its subtree at
// its moment; a restore's node, group and member.
type RevertStep struct {
	Kind          domain.RevertOpKind
	Seq           int
	Node, Parent  uuid.UUID
	Name          string
	Order         float64
	Base          int
	Hash          []byte
	Bytes         int
	Rewrite       bool
	Nodes         []uuid.UUID
	Group, Member int
}

// RevertSteps are a revert's job plan's steps as the job keeps them, and
// its groups, each its members in pre-order.
func (p JobPlan) RevertSteps() ([]RevertStep, [][]domain.GroupNode) {
	if p.revert == nil {
		return nil, nil
	}
	steps := make([]RevertStep, len(p.revert.steps))
	for i, s := range p.revert.steps {
		steps[i] = RevertStep{Kind: s.kind, Seq: int(s.seq), Node: s.node, Parent: s.parent, Name: s.name, Order: s.order,
			Base: int(s.base), Hash: s.hash, Bytes: s.bytes, Rewrite: s.rewrite, Nodes: s.nodes, Group: int(s.group), Member: int(s.member)}
	}
	groups := make([][]domain.GroupNode, len(p.revert.groups))
	for i, g := range p.revert.groups {
		groups[i] = g.nodes
	}
	return steps, groups
}
