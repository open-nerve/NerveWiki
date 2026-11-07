package app

import (
	"context"
	"errors"
	"strconv"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// referencePages consumes complete bounded streams and releases each page's raw
// representation after its callback. Bodies is one stream for the entire role,
// including source F: never one body query per source page.
func referencePages(ctx context.Context, reads *LintReads, notebook uuid.UUID, ids []uuid.UUID, properties bool,
	emit func(uuid.UUID, []QualifiedReference) error,
) error {
	return referencePagesUsingBody(ctx, reads, notebook, ids, properties, reads.Bodies, emit)
}

func referencePagesUsingBody(ctx context.Context, reads *LintReads, notebook uuid.UUID, ids []uuid.UUID, properties bool,
	openBodies func(uuid.UUID, []uuid.UUID) *LintBatches[BodyReference], emit func(uuid.UUID, []QualifiedReference) error,
) error {
	if len(ids) == 0 {
		return nil
	}
	bodies := openBodies(notebook, ids)
	var body []BodyReference
	bodyDone := false
	nextBody := func() error {
		if len(body) == 0 && !bodyDone {
			var err error
			body, err = bodies.Next(ctx)
			if err != nil {
				return err
			}
			bodyDone = len(body) == 0
		}
		return nil
	}
	var values *LintBatches[SourceProperty]
	if properties {
		values = reads.SourceValues(notebook, ids)
	}
	var value []SourceProperty
	valueDone := !properties
	nextValue := func() error {
		if len(value) == 0 && !valueDone {
			var err error
			value, err = values.Next(ctx)
			if err != nil {
				return err
			}
			valueDone = len(value) == 0
		}
		return nil
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		var refs []QualifiedReference
		if err := nextValue(); err != nil {
			return err
		}
		if len(value) > 0 && value[0].PageID.Compare(id) < 0 {
			return errors.New("ingest: unexpected source property page")
		}
		if len(value) > 0 && value[0].PageID == id {
			p := value[0]
			value[0] = SourceProperty{}
			value = value[1:]
			slots, err := domain.NewSourceSlots(ctx, p.Key, p.JSON)
			if err != nil {
				return err
			}
			array, items, has := slots.QueryShape()
			starts := map[string]int32{}
			if has {
				stream := reads.SourceSlots(notebook, []SourceSpec{{PageID: id, Key: p.Key, Array: array, Items: items}})
				for {
					batch, err := stream.Next(ctx)
					if err != nil {
						return err
					}
					if len(batch) == 0 {
						break
					}
					for _, edge := range batch {
						if edge.SourceID != id {
							return errors.New("ingest: unexpected slot page")
						}
						if err := slots.Observe(edge.PropertyKey, edge.Target, edge.ResolvedID, edge.ResolvedAsset); err != nil {
							return err
						}
						if edge.PropertyKey != nil {
							starts[*edge.PropertyKey] = edge.Start
						}
					}
				}
			}
			judgments, err := slots.Finish(ctx)
			if err != nil {
				return err
			}
			for i, j := range judgments {
				if j.NotLink || j.ResolvedID == nil {
					continue
				}
				key := p.Key
				if array {
					key += "." + strconv.Itoa(i)
				}
				refs = append(refs, QualifiedReference{TargetID: *j.ResolvedID, TargetIsAsset: j.ResolvedAsset, Origin: FirstSourcesSlot, Start: starts[key]})
			}
		}
		for {
			if err := nextBody(); err != nil {
				return err
			}
			if len(body) == 0 || body[0].SourceID != id {
				break
			}
			r := body[0]
			body = body[1:]
			refs = append(refs, QualifiedReference{TargetID: r.ResolvedID, TargetIsAsset: r.ResolvedAsset, Origin: ReferenceBody, Start: r.Start})
		}
		if len(body) > 0 && body[0].SourceID.Compare(id) < 0 {
			return errors.New("ingest: unexpected body page")
		}
		if err := emit(id, refs); err != nil {
			return err
		}
	}
	if err := nextBody(); err != nil {
		return err
	}
	if err := nextValue(); err != nil {
		return err
	}
	if !bodyDone || !valueDone {
		return errors.New("ingest: references outside requested pages")
	}
	return ctx.Err()
}
