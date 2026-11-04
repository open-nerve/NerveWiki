package postgresadapter

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type preparedLinkRead struct {
	canRetain       bool
	links           gen.InsertLinksParams
	targets, assets []uuid.UUID
	missing         []int32
	inputs          []app.Link
}

func inspectPageLinksForPreparation(ctx context.Context, q postgres.Querier, arg gen.InspectPageLinksForPreparationParams, links *preparedLinkFacts) (preparedLinkRead, error) {
	rows, err := q.Query(ctx, gen.InspectPageLinksForPreparation, arg.SourceID, arg.NotebookID, arg.Extractor)
	if err != nil {
		return preparedLinkRead{}, err
	}
	return collectPreparedSource(rows, links)
}

// One fixed native Scan scratch, including fixed nullable codecs. The DTO's
// nullable pointers refer only to this scratch, are reset after every Scan,
// and are never retained in the result. Non-null fields keep their native
// destinations/errors. No statement or metadata work occurs before Close.
func collectPreparedSource(rows pgx.Rows, links *preparedLinkFacts) (preparedLinkRead, error) {
	defer rows.Close()
	byStart := newPreparedStartMatcher(links.values)
	changed := make([]bool, len(links.values))
	inputs := make([]app.Link, len(links.values))
	for i, link := range links.values {
		changed[i] = true
		inputs[i] = app.Link{SourceID: links.source, Start: link.Start, Target: link.Target, Aliases: link.Aliases}
	}
	out := preparedLinkRead{canRetain: true, assets: make([]uuid.UUID, 0)}
	targetSet := make(map[uuid.UUID]struct{})
	var row gen.InspectPageLinksForPreparationRow
	var start pgtype.Int4
	var property, anchor, display, key, alt pgtype.Text
	var resolved pgtype.UUID
	var resolvedID uuid.UUID
	var aliases, ambiguous, resolvedAsset bool
	dest := [...]any{&row.CanRetain, &start, &row.RangeEnd, &row.NotebookID,
		&row.Kind, &property, &row.Target, &anchor, &display, &key, &alt,
		&aliases, &resolved, &ambiguous, &resolvedAsset}
	seen, haveBody := false, false
	var lastBody int32
	var bodyErr error
	for rows.Next() {
		if err := rows.Scan(dest[:]...); err != nil {
			return preparedLinkRead{}, err
		}
		seen = true
		out.canRetain = out.canRetain && row.CanRetain
		row.RangeStart, row.ResolvedID = nil, nil
		if start.Valid {
			row.RangeStart = &start.Int32
		}
		if resolved.Valid {
			resolvedID = uuid.UUID(resolved.Bytes)
			row.ResolvedID = &resolvedID
		}
		row.PropertyKey, row.Anchor, row.Display = preparedText(&property), preparedText(&anchor), preparedText(&display)
		row.TargetKey, row.TargetAltKey = preparedText(&key), preparedText(&alt)
		if row.RangeStart == nil {
			continue
		} // sentinel only, not range zero
		if row.NotebookID != links.notebook {
			out.canRetain = false
		}
		if row.ResolvedID != nil {
			if *row.ResolvedID == uuid.Nil() {
				out.canRetain = false
			} else if len(targetSet) <= app.MaxEventPages {
				targetSet[*row.ResolvedID] = struct{}{}
			}
		}
		// F has its own qualification; storage eligibility never suppresses it.
		if row.NotebookID == links.notebook && row.PropertyKey == nil && row.ResolvedID != nil {
			if *row.RangeStart < 0 || haveBody && *row.RangeStart <= lastBody {
				bodyErr = errors.New("lint reads: body references do not advance")
			}
			lastBody, haveBody = *row.RangeStart, true
			if resolvedAsset {
				if len(out.assets) == 0 {
					// Incoming size is a bounded hint, never a limit on old F.
					out.assets = make([]uuid.UUID, 0, min(len(links.values), 10_000))
				}
				out.assets = append(out.assets, *row.ResolvedID)
			}
		}
		if i, ok := byStart.find(*row.RangeStart); ok {
			// Keep one equality implementation: this value projection borrows
			// fixed scratch pointers only for the duration of this comparison.
			old := gen.InspectPageLinksForRetentionRow{CanRetain: row.CanRetain, RangeStart: row.RangeStart,
				RangeEnd: row.RangeEnd, NotebookID: row.NotebookID, Kind: row.Kind, PropertyKey: row.PropertyKey,
				Target: row.Target, Anchor: row.Anchor, Display: row.Display, TargetKey: row.TargetKey,
				TargetAltKey: row.TargetAltKey, Aliases: aliases, ResolvedID: row.ResolvedID}
			changed[i] = !retentionValuesEqual(&old, links.at(i))
			if !changed[i] && row.ResolvedID != nil {
				inputs[i].Resolution = domain.Resolution{ID: *row.ResolvedID, Ambiguous: ambiguous, Asset: resolvedAsset}
			}
		} else {
			out.missing = append(out.missing, *row.RangeStart)
		}
	}
	if err := rows.Err(); err != nil {
		return preparedLinkRead{}, err
	}
	if !seen {
		return preparedLinkRead{}, pgx.ErrNoRows
	}
	if bodyErr != nil {
		return preparedLinkRead{}, bodyErr
	}
	if !out.canRetain {
		return preparedLinkRead{assets: out.assets}, nil
	}
	if len(targetSet) > 0 {
		out.targets = make([]uuid.UUID, len(targetSet))
		i := 0
		for id := range targetSet {
			out.targets[i] = id
			i++
		}
		slices.SortFunc(out.targets, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	}
	out.links = links.changedParams(changed)
	sortPreparedInputs(inputs)
	out.inputs = inputs
	return out, nil
}

func preparedText(v *pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
