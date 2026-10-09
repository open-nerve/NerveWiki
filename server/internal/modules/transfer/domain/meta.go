package domain

import (
	"time"
	"uuid"
)

// MetaPath is where an export writes its meta.json in the vault (M7 design
// 4.10): Obsidian ignores the folder; an import reads it for the siblings'
// order.
const MetaPath = ".nerve/meta.json"

// MetaFormat is the version of meta.json's format.
const MetaFormat = 1

// Meta is an export's meta.json.
type Meta struct {
	Format     int       `json:"format"`
	ExportedAt time.Time `json:"exported_at"`
	Notebook   MetaNamed `json:"notebook"`
	// Root is the page exported with its subtree; null for the whole
	// notebook.
	Root  *MetaNamed `json:"root"`
	Nodes []MetaNode `json:"nodes"`
	// Contributed are the files the export's contributors added.
	Contributed []string `json:"contributed"`
}

// MetaNamed is a notebook or a page in meta.json.
type MetaNamed struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// MetaNode is a node written in the archive: its path in the vault (a
// page's file, or its folder ending in "/"; an attachment's file), its
// kind, its id, only for reference, and its order among its siblings.
type MetaNode struct {
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	ID        uuid.UUID `json:"id"`
	SortOrder float64   `json:"sort_order"`
}

// Meta is the plan's meta.json as of at, the entries written listed: those
// for which written is true, in the plan's order.
func (p *Plan) Meta(at time.Time, notebook Named, root *Named, written func(Entry) bool) Meta {
	m := Meta{Format: MetaFormat, ExportedAt: at.UTC().Truncate(time.Second), Notebook: MetaNamed(notebook),
		Nodes: []MetaNode{}, Contributed: p.Contributed()}
	if root != nil {
		r := MetaNamed(*root)
		m.Root = &r
	}
	if m.Contributed == nil {
		m.Contributed = []string{}
	}
	for _, e := range p.Entries {
		if !written(e) {
			continue
		}
		kind := "page"
		if e.Node.Asset {
			kind = "asset"
		}
		m.Nodes = append(m.Nodes, MetaNode{Path: e.Path, Kind: kind, ID: e.Node.ID, SortOrder: e.Node.SortOrder})
	}
	return m
}
