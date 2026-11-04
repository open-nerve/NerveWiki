package postgresadapter

import (
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Values are borrowed from this unit's immutable facts. Only the two complete
// Keys arrays are owned here; an eligible plan retains no borrowed facts slice.
type preparedLinkFacts struct {
	source, notebook uuid.UUID
	values           []domain.Link
	keys, alt        []string
}

func newPreparedLinkFacts(page app.Page, values []domain.Link) preparedLinkFacts {
	out := preparedLinkFacts{source: page.ID, notebook: page.NotebookID, values: values}
	if n := len(values); n > 0 {
		out.keys, out.alt = make([]string, n), make([]string, n)
	}
	for i, link := range values {
		if keys := link.Keys(); len(keys) > 0 {
			out.keys[i] = keys[0]
			if len(keys) > 1 {
				out.alt[i] = keys[1]
			}
		}
	}
	return out
}

func (p *preparedLinkFacts) at(i int) retentionLinkValues {
	link := p.values[i]
	return retentionLinkValues{notebook: p.notebook, end: int32(link.End), kind: link.Kind,
		property: link.Property, frontmatter: link.InFrontmatter(), target: link.Target,
		anchor: link.Anchor, display: link.Display, key: p.keys[i], alt: p.alt[i], aliases: link.Aliases}
}

// Called only after the complete eligible read. Sparse plans have exact
// capacity; a fully changed plan takes ownership of the two complete Keys arrays.
func (p *preparedLinkFacts) changedParams(changed []bool) gen.InsertLinksParams {
	out := gen.InsertLinksParams{SourceID: p.source, NotebookID: p.notebook}
	if len(p.values) == 0 {
		return out
	} // Original zero-input parameters are nil, unlike zero changed of N>0.
	n := 0
	for _, yes := range changed {
		if yes {
			n++
		}
	}
	out.RangeStarts, out.RangeEnds = make([]int32, n), make([]int32, n)
	out.Kinds, out.PropertyKeys = make([]string, n), make([]string, n)
	out.InFrontmatter, out.Aliases = make([]bool, n), make([]bool, n)
	out.Targets, out.Anchors, out.Displays = make([]string, n), make([]string, n), make([]string, n)
	if n == len(p.values) {
		out.TargetKeys, out.TargetAltKeys = p.keys, p.alt
	} else {
		out.TargetKeys, out.TargetAltKeys = make([]string, n), make([]string, n)
	}
	j := 0
	for i, yes := range changed {
		if !yes {
			continue
		}
		link := p.values[i]
		out.RangeStarts[j], out.RangeEnds[j] = int32(link.Start), int32(link.End)
		out.Kinds[j], out.PropertyKeys[j] = link.Kind, link.Property
		out.InFrontmatter[j], out.Aliases[j] = link.InFrontmatter(), link.Aliases
		out.Targets[j], out.Anchors[j], out.Displays[j] = link.Target, link.Anchor, link.Display
		out.TargetKeys[j], out.TargetAltKeys[j] = p.keys[i], p.alt[i]
		j++
	}
	return out
}

// Strictly increasing input starts need no map. Other caller orders retain
// the old last-index-per-start map without sorting or modifying the input.
type preparedStartMatcher struct {
	values   []domain.Link
	byStart  map[int32]int
	next     int
	last     int32
	haveLast bool
}

func newPreparedStartMatcher(values []domain.Link) preparedStartMatcher {
	out := preparedStartMatcher{values: values}
	for i := 1; i < len(values); i++ {
		if int32(values[i-1].Start) >= int32(values[i].Start) {
			out.useMap()
			break
		}
	}
	return out
}

func (m *preparedStartMatcher) useMap() {
	m.byStart = make(map[int32]int, len(m.values))
	for i, link := range m.values {
		m.byStart[int32(link.Start)] = i
	}
}

func (m *preparedStartMatcher) find(start int32) (int, bool) {
	if m.byStart == nil {
		if m.haveLast && start < m.last {
			m.useMap()
		} // A backwards native row keeps the original map behavior thereafter.
		m.last, m.haveLast = start, true
	}
	if m.byStart != nil {
		i, ok := m.byStart[start]
		return i, ok
	}
	for m.next < len(m.values) && int32(m.values[m.next].Start) < start {
		m.next++
	}
	if m.next < len(m.values) && int32(m.values[m.next].Start) == start {
		return m.next, true
	}
	return 0, false
}
