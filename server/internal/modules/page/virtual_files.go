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

type virtualLogReads struct {
	store *postgresadapter.Store
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
	names, err := r.names.DisplayNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	nodes := []uuid.UUID{}
	for _, cs := range changes {
		for _, c := range cs {
			if c.Alive {
				nodes = append(nodes, c.NodeID)
			}
		}
	}
	paths, err := r.store.PathsOf(ctx, notebook, nodes)
	if err != nil {
		return nil, err
	}
	out := make([]VirtualLogChangeset, len(rows))
	for i, row := range rows {
		cs := VirtualLogChangeset{ID: row.ID, Kind: string(row.Kind), Client: string(row.Client), ByName: names[row.By],
			CreatedAt: row.CreatedAt, Message: row.Message, NodeCount: facts[row.ID].Nodes, ContentCount: facts[row.ID].Contents,
			Changes: []VirtualLogChange{}, IngestSources: []VirtualLogSource{}}
		for _, c := range changes[row.ID] {
			change := VirtualLogChange{NodeID: c.NodeID, Name: c.Name, Kind: string(c.Change()), Asset: c.NodeKind == domain.KindAsset}
			if path, ok := paths[c.NodeID]; ok {
				change.Path = &path
			}
			cs.Changes = append(cs.Changes, change)
		}
		out[i] = cs
	}
	return out, nil
}
