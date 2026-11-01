package app

import (
	"cmp"
	"context"
	"errors"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

type FileName string

const (
	IndexFile FileName = "index.md"
	LogFile   FileName = "log.md"
)

type FileProfile uint8

const (
	ReadFile FileProfile = iota
	ExportFile
)

const (
	MaxIndexFileItems   = 200
	maxReadFileBytes    = 40 << 10
	maxExportIndexItems = 10000
	maxExportIndexBytes = 2 << 20
	maxReadLogItems     = 50
	maxExportLogItems   = 1000
	maxExportLogBytes   = 1 << 20
)

type FileIndexHead struct {
	ID        uuid.UUID
	Typed     bool
	TypeStart string
	TypeKey   string
}

type FileIndexFacts struct {
	Type, Summary *string
	Tags          []string
}

type IndexFileReads interface {
	Heads(context.Context, uuid.UUID) ([]FileIndexHead, error)
	Items(context.Context, []uuid.UUID) (map[uuid.UUID]FileIndexFacts, error)
	Inbound(context.Context, []uuid.UUID) (map[uuid.UUID]int, error)
}

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

type VirtualSourceNode struct {
	ID    uuid.UUID
	Asset bool
	Bytes int64
}

type VirtualSourceReads interface {
	Nodes(context.Context, uuid.UUID, uuid.UUID) ([]VirtualSourceNode, error)
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

type ExportPath struct {
	ID    uuid.UUID
	Asset bool
	Path  string
}

type VirtualExportScope struct{ Paths []ExportPath }

type VirtualScope struct {
	NotebookID, SourceRootID uuid.UUID
	Export                   *VirtualExportScope
}

type VirtualFileDeps struct {
	Index   IndexFileReads
	Tree    VirtualTreeReads
	Sources VirtualSourceReads
	Log     VirtualLogReads
}

// VirtualFiles is a read-only service. It borrows the caller's authorized
// snapshot, with no lint runtime, transaction, or cross-call cache of its own.
type VirtualFiles struct{ deps VirtualFileDeps }

func NewVirtualFiles(d VirtualFileDeps) *VirtualFiles {
	if d.Index == nil || d.Tree == nil || d.Sources == nil || d.Log == nil {
		panic("wiki: incomplete virtual file dependencies")
	}
	return &VirtualFiles{deps: d}
}

func (v *VirtualFiles) GenerateInSnapshot(ctx context.Context, scope VirtualScope, name FileName, profile FileProfile) ([]byte, error) {
	if profile != ReadFile && profile != ExportFile || (profile == ExportFile) != (scope.Export != nil) {
		return nil, errors.New("virtual file: invalid profile or export scope")
	}
	if name != IndexFile && name != LogFile {
		return nil, errors.New("virtual file: unknown name")
	}
	// Read log needs only its own bounded history and path batches. Export
	// validates the complete mapping before either file can be generated.
	var nodes map[uuid.UUID]VirtualTreeNode
	var paths map[uuid.UUID]domain.FilePath
	if name == IndexFile || profile == ExportFile {
		rows, err := v.deps.Tree.Nodes(ctx, scope.NotebookID)
		if err != nil {
			return nil, err
		}
		nodes, paths, err = virtualPaths(rows, scope.Export)
		if err != nil {
			return nil, err
		}
		if root, ok := nodes[scope.SourceRootID]; !ok || root.Asset {
			return nil, errors.New("virtual file: missing source root")
		}
	}
	if name == IndexFile {
		return v.index(ctx, scope, profile, nodes, paths)
	}
	return v.log(ctx, scope, profile, paths)
}

// virtualPaths uses the actual export plan when supplied. In read mode it
// builds full paths once from the complete live tree, never per selected page.
func virtualPaths(rows []VirtualTreeNode, export *VirtualExportScope) (map[uuid.UUID]VirtualTreeNode, map[uuid.UUID]domain.FilePath, error) {
	nodes := make(map[uuid.UUID]VirtualTreeNode, len(rows))
	for _, n := range rows {
		if _, exists := nodes[n.ID]; exists {
			return nil, nil, errors.New("virtual file: duplicate tree node")
		}
		nodes[n.ID] = n
	}
	paths := make(map[uuid.UUID]domain.FilePath, len(rows))
	if export != nil {
		seen := map[string]bool{}
		for _, e := range export.Paths {
			n, ok := nodes[e.ID]
			_, duplicate := paths[e.ID]
			if !ok || duplicate || seen[e.Path] || n.Asset != e.Asset || !validExportPath(e.Path, e.Asset) {
				return nil, nil, errors.New("virtual file: invalid export mapping")
			}
			p := domain.FilePath{Label: e.Path}
			if !strings.HasSuffix(e.Path, "/") {
				p.Destination = &e.Path
			}
			paths[e.ID], seen[e.Path] = p, true
		}
		if len(paths) != len(nodes) {
			return nil, nil, errors.New("virtual file: incomplete export mapping")
		}
		return nodes, paths, nil
	}
	visiting, stems := map[uuid.UUID]bool{}, map[uuid.UUID]string{}
	var stem func(uuid.UUID) (string, error)
	stem = func(id uuid.UUID) (string, error) {
		if s, ok := stems[id]; ok {
			return s, nil
		}
		n, ok := nodes[id]
		if !ok || visiting[id] {
			return "", errors.New("virtual file: incomplete or cyclic tree")
		}
		visiting[id] = true
		s := n.Name
		if n.ParentID != nil {
			parent, err := stem(*n.ParentID)
			if err != nil {
				return "", err
			}
			s = parent + "/" + s
		}
		stems[id], visiting[id] = s, false
		return s, nil
	}
	for id, n := range nodes {
		s, err := stem(id)
		if err != nil {
			return nil, nil, err
		}
		if !n.Asset {
			s += ".md"
		}
		paths[id] = domain.FilePath{Label: s, Destination: &s}
	}
	return nodes, paths, nil
}

func validExportPath(s string, asset bool) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.Contains(s, `\`) || !utf8.ValidString(s) ||
		strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return false
	}
	if strings.HasSuffix(s, "/") {
		if asset {
			return false
		}
		s = strings.TrimSuffix(s, "/")
	} else if !asset && !strings.HasSuffix(s, ".md") {
		return false
	}
	for _, segment := range strings.Split(s, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

type fileCandidate struct {
	id                uuid.UUID
	typed             bool
	key, parent, path string
}

func (v *VirtualFiles) index(ctx context.Context, scope VirtualScope, profile FileProfile,
	nodes map[uuid.UUID]VirtualTreeNode, paths map[uuid.UUID]domain.FilePath,
) ([]byte, error) {
	heads, err := v.deps.Index.Heads(ctx, scope.NotebookID)
	if err != nil {
		return nil, err
	}
	sources, err := v.deps.Sources.Nodes(ctx, scope.NotebookID, scope.SourceRootID)
	if err != nil {
		return nil, err
	}
	excluded := make(map[uuid.UUID]bool, len(sources))
	var count int64
	for _, n := range sources {
		excluded[n.ID] = true
		if n.Asset || n.Bytes > 0 {
			count++
		}
	}
	candidates := make([]fileCandidate, 0, len(heads))
	for _, h := range heads {
		n, ok := nodes[h.ID]
		if !ok || n.Asset || excluded[h.ID] {
			continue
		}
		p := paths[h.ID].Label
		parent := path.Dir(strings.TrimSuffix(p, "/"))
		if parent == "." {
			parent = "/"
		}
		candidates = append(candidates, fileCandidate{id: h.ID, typed: h.Typed, key: h.TypeKey, parent: parent, path: p})
	}
	slices.SortFunc(candidates, func(a, b fileCandidate) int {
		if a.typed != b.typed {
			if a.typed {
				return -1
			}
			return 1
		}
		if a.typed {
			if c := cmp.Compare(a.key, b.key); c != 0 {
				return c
			}
		} else if c := cmp.Compare(a.parent, b.parent); c != 0 {
			return c
		}
		return cmp.Compare(a.path, b.path)
	})
	total, limit, budget := len(candidates), MaxIndexFileItems, maxReadFileBytes
	if profile == ExportFile {
		limit, budget = maxExportIndexItems, maxExportIndexBytes
	}
	candidates = candidates[:min(len(candidates), limit)]
	ids := make([]uuid.UUID, len(candidates))
	for i, c := range candidates {
		ids[i] = c.id
	}
	facts, err := v.deps.Index.Items(ctx, ids)
	if err != nil {
		return nil, err
	}
	changed, err := v.deps.Tree.ChangedAt(ctx, ids)
	if err != nil {
		return nil, err
	}
	var inbound map[uuid.UUID]int
	if profile == ReadFile {
		inbound, err = v.deps.Index.Inbound(ctx, ids)
		if err != nil {
			return nil, err
		}
	}
	items := make([]domain.IndexFileItem, len(candidates))
	for i, c := range candidates {
		f := facts[c.id]
		items[i] = domain.IndexFileItem{ID: c.id, Path: paths[c.id], ParentPath: c.parent, Title: nodes[c.id].Name, TypeKey: c.key,
			Type: f.Type, Summary: f.Summary, Tags: f.Tags, ChangedAt: changed[c.id]}
		if profile == ReadFile {
			n := inbound[c.id]
			items[i].Inbound = &n
		}
	}
	return domain.RenderIndex(domain.RenderIndexInput{Items: items, Total: total, SourcePath: paths[scope.SourceRootID], Sources: count}, budget)
}

func (v *VirtualFiles) log(ctx context.Context, scope VirtualScope, profile FileProfile, paths map[uuid.UUID]domain.FilePath) ([]byte, error) {
	limit, budget := maxReadLogItems, maxReadFileBytes
	if profile == ExportFile {
		limit, budget = maxExportLogItems, maxExportLogBytes
	}
	rows, err := v.deps.Log.Recent(ctx, scope.NotebookID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RenderLogChangeset, len(rows))
	for i, r := range rows {
		it := domain.RenderLogChangeset{ID: r.ID, Kind: r.Kind, Client: r.Client, ByName: r.ByName, CreatedAt: r.CreatedAt,
			Message: r.Message, NodeCount: r.NodeCount, ContentCount: r.ContentCount, IngestCount: r.IngestCount}
		for _, c := range r.Changes {
			p := domain.FilePath{Label: c.Name}
			if profile == ExportFile {
				if mapped, ok := paths[c.NodeID]; ok {
					p = mapped
				}
			} else if c.Path != nil {
				p = domain.FilePath{Label: *c.Path, Destination: c.Path}
			}
			it.Changes = append(it.Changes, domain.RenderLogChange{Path: p, Kind: c.Kind, Asset: c.Asset})
		}
		for _, s := range r.IngestSources {
			p := domain.FilePath{Label: "Missing source"}
			if profile == ExportFile {
				if s.NodeID != nil {
					if mapped, ok := paths[*s.NodeID]; ok {
						p = mapped
					}
				}
			} else if s.Path != nil {
				p = domain.FilePath{Label: *s.Path, Destination: s.Path}
			}
			it.IngestSources = append(it.IngestSources, p)
		}
		items[i] = it
	}
	return domain.RenderLog(domain.RenderLogInput{Items: items, Total: len(rows)}, budget)
}
