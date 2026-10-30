package app

import (
	"slices"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// RuleTiming measures one rule's exclusive branch, not SQL CPU time.
type RuleTiming struct {
	Rule     domain.Rule
	Duration time.Duration
}

// PreparationTimings records the shared sources work once. Nil means that
// phase was not called; an observed zero duration is still a present value.
type PreparationTimings struct {
	SharedSources *time.Duration
	Rules         []RuleTiming
}

// Timings returns owned observations without repeating any reads.
func (p *Prepared) Timings() PreparationTimings {
	out := p.timings
	out.Rules = slices.Clone(out.Rules)
	if out.SharedSources != nil {
		value := *out.SharedSources
		out.SharedSources = &value
	}
	return out
}
