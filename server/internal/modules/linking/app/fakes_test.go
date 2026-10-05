package app_test

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// tree is a notebook's tree, as the fake Pages reads it, and how many
// reads of candidates and paths it answered.
type tree struct {
	nodes map[uuid.UUID]*node
	names map[string]uuid.UUID
	reads int
}

type node struct {
	parent *uuid.UUID
	name   string
	gone   bool
}

func newTree() *tree {
	return &tree{nodes: map[uuid.UUID]*node{}, names: map[string]uuid.UUID{}}
}

// add adds the page at path, "A/B" under A, with an id after every other.
func (t *tree) add(path string) uuid.UUID {
	id := uuid.NewV7()
	n := &node{name: path[strings.LastIndex(path, "/")+1:]}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		parent := t.names[path[:i]]
		n.parent = &parent
	}
	t.nodes[id], t.names[path] = n, id
	return id
}

// path is id's path from the root, itself last.
func (t *tree) path(id uuid.UUID) []domain.Step {
	var out []domain.Step
	for cur := &id; cur != nil; cur = t.nodes[*cur].parent {
		out = append(out, domain.Step{ID: *cur, Key: shared.TitleKey(t.nodes[*cur].name), Name: t.nodes[*cur].name})
	}
	slices.Reverse(out)
	return out
}

func (t *tree) ByKeys(_ context.Context, _ uuid.UUID, keys []string) ([]domain.Node, error) {
	t.reads++
	var out []domain.Node
	for id, n := range t.nodes {
		if !n.gone && slices.Contains(keys, shared.TitleKey(n.name)) {
			out = append(out, domain.Node{ID: id, Path: t.path(id)})
		}
	}
	return out, nil
}

func (t *tree) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	t.reads++
	var out []domain.Node
	for _, id := range ids {
		if n, ok := t.nodes[id]; ok && !n.gone {
			out = append(out, domain.Node{ID: id, Path: t.path(id)})
		}
	}
	return out, nil
}

func (t *tree) Subtree(_ context.Context, _ uuid.UUID, id uuid.UUID) ([]domain.Step, error) {
	if n, ok := t.nodes[id]; !ok || n.gone {
		return nil, fmt.Errorf("no page %s", id)
	}
	out := []domain.Step{{ID: id, Key: shared.TitleKey(t.nodes[id].name), Name: t.nodes[id].name}}
	for i := 0; i < len(out); i++ {
		for child, n := range t.nodes {
			if !n.gone && n.parent != nil && *n.parent == out[i].ID {
				out = append(out, domain.Step{ID: child, Key: shared.TitleKey(n.name), Name: n.name})
			}
		}
	}
	return out, nil
}

// store is the index's tables in memory, the extractor its rows are of
// (0 for this one), the last reach it read and the views it answered.
type store struct {
	locked    []uuid.UUID
	facts     map[uuid.UUID]domain.Facts
	revisions map[uuid.UUID]int
	links     []app.Link
	missing   bool // SetResolutions finds no link
	extractor int
	reached   domain.Reach
	viewed    int
}

func newStore() *store {
	return &store{facts: map[uuid.UUID]domain.Facts{}, revisions: map[uuid.UUID]int{}}
}

func (s *store) Lock(_ context.Context, notebookID uuid.UUID) error {
	s.locked = append(s.locked, notebookID)
	return nil
}

func (s *store) ReplacePage(ctx context.Context, p app.Page, f domain.Facts) (app.Dropped, error) {
	dropped, err := s.DeletePages(ctx, []uuid.UUID{p.ID})
	if err == nil {
		err = s.AddPage(ctx, p, f)
	}
	return dropped, err
}

func (s *store) AddPage(_ context.Context, p app.Page, f domain.Facts) error {
	if _, ok := s.facts[p.ID]; ok {
		return fmt.Errorf("the page %s has rows", p.ID)
	}
	s.facts[p.ID], s.revisions[p.ID] = f, p.Revision
	for _, l := range f.Links {
		s.links = append(s.links, app.Link{SourceID: p.ID, Start: l.Start, Target: l.Target, Aliases: l.Aliases})
	}
	return nil
}

func (s *store) DeletePages(_ context.Context, ids []uuid.UUID) (app.Dropped, error) {
	var d app.Dropped
	for _, id := range ids {
		for _, a := range s.facts[id].Aliases {
			d.AliasKeys = append(d.AliasKeys, a.Key)
		}
		delete(s.facts, id)
		delete(s.revisions, id)
	}
	s.links = slices.DeleteFunc(s.links, func(l app.Link) bool {
		if !slices.Contains(ids, l.SourceID) {
			return false
		}
		if l.Resolution.ID != uuid.Nil() && !slices.Contains(d.Targets, l.Resolution.ID) {
			d.Targets = append(d.Targets, l.Resolution.ID)
		}
		return true
	})
	return d, nil
}

// DeleteNotebooks drops every row: the store holds one notebook's.
func (s *store) DeleteNotebooks(context.Context, []uuid.UUID) error {
	s.facts, s.links = map[uuid.UUID]domain.Facts{}, nil
	return nil
}

func (s *store) Links(_ context.Context, _ uuid.UUID, r domain.Reach) ([]app.Link, error) {
	s.reached = r
	var out []app.Link
	for _, l := range s.links {
		keys := domain.Link{Target: l.Target}.Keys()
		if slices.ContainsFunc(keys, func(k string) bool { return slices.Contains(r.Keys, k) }) ||
			slices.Contains(r.Targets, l.Resolution.ID) || slices.Contains(r.Sources, l.SourceID) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (s *store) Aliases(_ context.Context, _ uuid.UUID, keys []string) ([]app.Alias, error) {
	var out []app.Alias
	for id, f := range s.facts {
		for _, a := range f.Aliases {
			if slices.Contains(keys, a.Key) {
				out = append(out, app.Alias{PageID: id, Key: a.Key})
			}
		}
	}
	return out, nil
}

func (s *store) AliasKeys(_ context.Context, ids []uuid.UUID) ([]string, error) {
	var out []string
	for _, id := range ids {
		for _, a := range s.facts[id].Aliases {
			out = append(out, a.Key)
		}
	}
	return out, nil
}

func (s *store) SetResolutions(_ context.Context, links []app.Link) error {
	for _, l := range links {
		i := slices.IndexFunc(s.links, func(m app.Link) bool { return m.SourceID == l.SourceID && m.Start == l.Start })
		if i < 0 || s.missing {
			return fmt.Errorf("no link %s at %d", l.SourceID, l.Start)
		}
		s.links[i].Resolution = l.Resolution
	}
	return nil
}

func (s *store) IndexedOf(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.Indexed, error) {
	out := make(map[uuid.UUID]app.Indexed)
	for _, id := range ids {
		if _, ok := s.facts[id]; ok {
			out[id] = app.Indexed{Revision: s.revisions[id], Extractor: cmp.Or(s.extractor, domain.Extractor)}
		}
	}
	return out, nil
}

func (s *store) View(_ context.Context, id uuid.UUID) (app.Indexed, bool, error) {
	s.viewed++
	if _, ok := s.facts[id]; !ok {
		return app.Indexed{}, false, nil
	}
	out := app.Indexed{Revision: s.revisions[id], Extractor: s.extractor, Resolutions: map[int]domain.Resolution{}}
	if out.Extractor == 0 {
		out.Extractor = domain.Extractor
	}
	for _, l := range s.links {
		if l.SourceID == id {
			out.Resolutions[l.Start] = l.Resolution
		}
	}
	return out, true, nil
}

// resolution is where the link of source at start resolves.
func (s *store) resolution(source uuid.UUID, start int) domain.Resolution {
	for _, l := range s.links {
		if l.SourceID == source && l.Start == start {
			return l.Resolution
		}
	}
	panic(fmt.Sprintf("no link %s at %d", source, start))
}

// publisher records the links events.
type publisher struct {
	events []app.LinksChanged
}

func (p *publisher) LinksChanged(_ context.Context, e app.LinksChanged) error {
	p.events = append(p.events, e)
	return nil
}
