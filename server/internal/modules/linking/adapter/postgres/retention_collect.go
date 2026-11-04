package postgresadapter

import (
	"bytes"
	"context"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type retentionPlan struct {
	canRetain bool
	targets   []uuid.UUID
	missing   []int32
}

// Use the original execution mode on the caller's maintenance transaction.
// SQL and column order belong only to the generated retention query.
func inspectPageLinksForRetention(ctx context.Context, q postgres.Querier, arg gen.InspectPageLinksForRetentionParams, links *gen.InsertLinksParams) (retentionPlan, error) {
	rows, err := q.Query(ctx, gen.InspectPageLinksForRetention, arg.SourceID, arg.NotebookID, arg.Extractor)
	if err != nil {
		return retentionPlan{}, err
	}
	return collectPageLinksForRetention(rows, links)
}

// links owns all eleven arrays produced by the complete linksParams call.
// Do not compact any of them until every old row and the native terminal Err
// have been read. Every return closes Rows before Store can fall back or write.
func collectPageLinksForRetention(rows pgx.Rows, links *gen.InsertLinksParams) (retentionPlan, error) {
	defer rows.Close()
	byStart := make(map[int32]int, len(links.RangeStarts))
	changed := make([]bool, len(links.RangeStarts))
	for i, start := range links.RangeStarts {
		byStart[start] = i
		changed[i] = true
	}
	targetSet := make(map[uuid.UUID]struct{})
	var missing []int32
	seen, canRetain := false, true
	var row gen.InspectPageLinksForRetentionRow
	dest := [...]any{&row.CanRetain, &row.RangeStart, &row.RangeEnd, &row.NotebookID,
		&row.Kind, &row.PropertyKey, &row.Target, &row.Anchor, &row.Display,
		&row.TargetKey, &row.TargetAltKey, &row.Aliases, &row.ResolvedID}
	for rows.Next() {
		if err := rows.Scan(dest[:]...); err != nil {
			return retentionPlan{}, err
		}
		seen = true
		canRetain = canRetain && row.CanRetain
		if row.RangeStart == nil { // The empty-source metadata row; start=0 is real.
			continue
		}
		if row.NotebookID != links.NotebookID {
			canRetain = false
		}
		if row.ResolvedID != nil {
			if *row.ResolvedID == uuid.Nil() {
				canRetain = false
			} else if len(targetSet) <= app.MaxEventPages {
				// Dropped.Targets is only an event overflow witness. The
				// complete row stream and eligibility checks still run.
				targetSet[*row.ResolvedID] = struct{}{}
			}
		}
		if i, ok := byStart[*row.RangeStart]; ok {
			changed[i] = !retentionFactsEqual(&row, links, i)
		} else {
			missing = append(missing, *row.RangeStart)
		}
	}
	if err := rows.Err(); err != nil {
		return retentionPlan{}, err
	}
	if !seen {
		return retentionPlan{}, pgx.ErrNoRows
	}
	if !canRetain {
		return retentionPlan{}, nil
	}
	var targets []uuid.UUID
	if len(targetSet) != 0 {
		targets = make([]uuid.UUID, len(targetSet))
		i := 0
		for target := range targetSet {
			targets[i] = target
			i++
		}
		slices.SortFunc(targets, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	}
	compactRetentionLinks(links, changed)
	return retentionPlan{canRetain: true, targets: targets, missing: missing}, nil
}

// Only the incoming value is normalized as the SQL normalizes it. An old
// empty string is distinct from NULL; property_key may be a valid empty key.
func retentionNullableEqual(old *string, value string, valid bool) bool {
	if old == nil {
		return !valid
	}
	return valid && *old == value
}

// SQL's explicit C comparisons are byte equality. Resolution is not an
// extracted fact: retained rows keep it and changed/new rows use the SQL reset.
func retentionFactsEqual(old *gen.InspectPageLinksForRetentionRow, links *gen.InsertLinksParams, i int) bool {
	return retentionValuesEqual(old, retentionLinkValues{notebook: links.NotebookID, end: links.RangeEnds[i],
		kind: links.Kinds[i], property: links.PropertyKeys[i], frontmatter: links.InFrontmatter[i], target: links.Targets[i],
		anchor: links.Anchors[i], display: links.Displays[i], key: links.TargetKeys[i], alt: links.TargetAltKeys[i], aliases: links.Aliases[i]})
}

type retentionLinkValues struct {
	notebook                                          uuid.UUID
	end                                               int32
	kind, property, target, anchor, display, key, alt string
	frontmatter, aliases                              bool
}

// Both collectors project into the same pure comparison; only incoming
// values are normalized, so an old empty value never becomes NULL.
func retentionValuesEqual(old *gen.InspectPageLinksForRetentionRow, value retentionLinkValues) bool {
	return old.RangeEnd == value.end && old.NotebookID == value.notebook && old.Kind == value.kind &&
		retentionNullableEqual(old.PropertyKey, value.property, value.frontmatter) && old.Target == value.target &&
		retentionNullableEqual(old.Anchor, value.anchor, value.anchor != "") &&
		retentionNullableEqual(old.Display, value.display, value.display != "") &&
		retentionNullableEqual(old.TargetKey, value.key, value.key != "") &&
		retentionNullableEqual(old.TargetAltKey, value.alt, value.alt != "") && old.Aliases == value.aliases
}

// Retain input order and the same aligned value in every owned parameter
// array. Nothing aliases the caller's facts slice and no Keys work is skipped.
func compactRetentionLinks(p *gen.InsertLinksParams, changed []bool) {
	n := 0
	for i, yes := range changed {
		if !yes {
			continue
		}
		p.RangeStarts[n] = p.RangeStarts[i]
		p.RangeEnds[n] = p.RangeEnds[i]
		p.Kinds[n] = p.Kinds[i]
		p.PropertyKeys[n] = p.PropertyKeys[i]
		p.InFrontmatter[n] = p.InFrontmatter[i]
		p.Targets[n] = p.Targets[i]
		p.Anchors[n] = p.Anchors[i]
		p.Displays[n] = p.Displays[i]
		p.TargetKeys[n] = p.TargetKeys[i]
		p.TargetAltKeys[n] = p.TargetAltKeys[i]
		p.Aliases[n] = p.Aliases[i]
		n++
	}
	p.RangeStarts = p.RangeStarts[:n]
	p.RangeEnds = p.RangeEnds[:n]
	p.Kinds = p.Kinds[:n]
	p.PropertyKeys = p.PropertyKeys[:n]
	p.InFrontmatter = p.InFrontmatter[:n]
	p.Targets = p.Targets[:n]
	p.Anchors = p.Anchors[:n]
	p.Displays = p.Displays[:n]
	p.TargetKeys = p.TargetKeys[:n]
	p.TargetAltKeys = p.TargetAltKeys[:n]
	p.Aliases = p.Aliases[:n]
}
