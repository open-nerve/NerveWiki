package page

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

type VirtualTreeNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Name     string
	Asset    bool
}

type VirtualTreeReads interface {
	Nodes(context.Context, uuid.UUID) ([]VirtualTreeNode, error)
	ChangedAt(context.Context, []uuid.UUID) (map[uuid.UUID]time.Time, error)
}

type virtualTreeReads struct{ store *postgresadapter.Store }

func NewVirtualTreeReads(pool *pgxpool.Pool) VirtualTreeReads {
	return virtualTreeReads{postgresadapter.New(pool)}
}

func (r virtualTreeReads) Nodes(ctx context.Context, notebook uuid.UUID) ([]VirtualTreeNode, error) {
	rows, err := r.store.ListNodes(ctx, notebook)
	if err != nil {
		return nil, err
	}
	out := make([]VirtualTreeNode, len(rows))
	for i, row := range rows {
		out[i] = VirtualTreeNode{ID: row.ID, ParentID: row.ParentID, Name: row.Name, Asset: row.Kind == domain.KindAsset}
	}
	return out, nil
}

func (r virtualTreeReads) ChangedAt(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	rows, err := r.store.PageStates(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]time.Time, len(rows))
	for id, row := range rows {
		out[id] = row.UpdatedAt
	}
	return out, nil
}

type VirtualLogChange struct {
	NodeID uuid.UUID
	Name   string
	Path   *string
	Kind   string
	Asset  bool
}

type VirtualLogSource struct {
	NodeID *uuid.UUID
	Path   *string
	Asset  bool
}

type VirtualLogChangeset struct {
	ID                                   uuid.UUID
	Kind, Client, ByName                 string
	CreatedAt                            time.Time
	Message                              *string
	Changes                              []VirtualLogChange
	NodeCount, ContentCount, IngestCount int
	IngestSources                        []VirtualLogSource
}

type VirtualLogReads interface {
	Recent(context.Context, uuid.UUID, int) ([]VirtualLogChangeset, error)
}

// virtualLogStore keeps the bounded reads used by Recent together. The
// production value is the existing page store, using the caller's context.
type virtualLogStore interface {
	ListChangesets(context.Context, uuid.UUID, domain.LogFilter, domain.ChangesetCursor, int) ([]domain.Changeset, error)
	ChangesetFacts(context.Context, []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error)
	FirstChanges(context.Context, []uuid.UUID, int) (map[uuid.UUID][]domain.ChangeRow, error)
	FirstIngestSources(context.Context, []uuid.UUID, int) (map[uuid.UUID][]*uuid.UUID, error)
	LinkTargetsByIDs(context.Context, uuid.UUID, []uuid.UUID) ([]postgresadapter.LinkPath, error)
}

type virtualLogReads struct {
	store virtualLogStore
	names Names
}

func NewVirtualLogReads(pool *pgxpool.Pool, names Names) VirtualLogReads {
	return virtualLogReads{store: postgresadapter.New(pool), names: names}
}

func (r virtualLogReads) Recent(ctx context.Context, notebook uuid.UUID, limit int) ([]VirtualLogChangeset, error) {
	if limit < 1 || limit > 1000 {
		return nil, errors.New("virtual log: invalid limit")
	}
	rows, err := r.store.ListChangesets(ctx, notebook, domain.LogFilter{}, domain.ChangesetFrom(), limit)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	ids, actors := make([]uuid.UUID, len(rows)), make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for i, row := range rows {
		ids[i] = row.ID
		if !seen[row.By] {
			actors = append(actors, row.By)
			seen[row.By] = true
		}
	}
	facts, err := r.store.ChangesetFacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	changes, err := r.store.FirstChanges(ctx, ids, 3)
	if err != nil {
		return nil, err
	}
	sources, err := r.store.FirstIngestSources(ctx, ids, 5)
	if err != nil {
		return nil, err
	}
	names, err := r.names.DisplayNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	nodes := []uuid.UUID{}
	seenNodes := map[uuid.UUID]bool{}
	add := func(id uuid.UUID) {
		if !seenNodes[id] {
			nodes = append(nodes, id)
			seenNodes[id] = true
		}
	}
	for _, id := range ids {
		for _, c := range changes[id] {
			if c.Alive {
				add(c.NodeID)
			}
		}
		for _, source := range sources[id] {
			if source != nil {
				add(*source)
			}
		}
	}
	targets, err := r.store.LinkTargetsByIDs(ctx, notebook, nodes)
	if err != nil {
		return nil, err
	}
	paths := make(map[uuid.UUID]VirtualLogSource, len(targets))
	for _, target := range targets {
		names := make([]string, len(target.Steps))
		for i, step := range target.Steps {
			names[i] = step.Name
		}
		kind := domain.KindPage
		if target.Asset {
			kind = domain.KindAsset
		}
		path := domain.PathOfNames(names, kind)
		paths[target.ID] = VirtualLogSource{NodeID: &target.ID, Path: &path, Asset: target.Asset}
	}
	out := make([]VirtualLogChangeset, len(rows))
	for i, row := range rows {
		cs := VirtualLogChangeset{ID: row.ID, Kind: string(row.Kind), Client: string(row.Client), ByName: names[row.By],
			CreatedAt: row.CreatedAt, Message: row.Message, NodeCount: facts[row.ID].Nodes, ContentCount: facts[row.ID].Contents,
			IngestCount: row.IngestCount,
			Changes:     []VirtualLogChange{}, IngestSources: []VirtualLogSource{}}
		for _, c := range changes[row.ID] {
			change := VirtualLogChange{NodeID: c.NodeID, Name: c.Name, Kind: string(c.Change()), Asset: c.NodeKind == domain.KindAsset}
			if path, ok := paths[c.NodeID]; ok {
				change.Path = path.Path
			}
			cs.Changes = append(cs.Changes, change)
		}
		for _, source := range sources[row.ID] {
			var preview VirtualLogSource
			if source != nil {
				preview = paths[*source]
			}
			cs.IngestSources = append(cs.IngestSources, preview)
		}
		out[i] = cs
	}
	return out, nil
}
