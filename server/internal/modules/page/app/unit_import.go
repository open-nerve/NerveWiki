package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// An import's unit (M7/P6 design 3.3): it creates pages and attachments
// alone, as CreatePage and CreateAsset do, but for three things. A name a
// sibling holds is numbered rather than refused; a page too deep is
// refused before anything is written, the unit going on; and the
// siblings and the line of each parent are read once in the unit, those
// it creates added to them: the unit holds the notebook's row FOR NO KEY
// UPDATE, so no other write of the tree changes them meanwhile.

// ErrNoParent is a parent of an import's node that is no page of the
// notebook: deleted since the import read it.
var ErrNoParent = errors.New("page: the parent is no page of the notebook")

// ImportUnit is an import's unit: Import's do calls its operations.
type ImportUnit struct {
	u     *Unit
	empty Facts
	// under are the children of each parent read in the unit, and lines
	// the line of each, by parent: uuid.Nil() for the root.
	under map[uuid.UUID]*children
	lines map[uuid.UUID][]domain.Ancestor
}

// children are a parent's children as the unit knows them, in order, and
// the keys of their names; next the number after the last tried of each
// family (domain.Family), by its key: the numbers before it are held, or
// were tried. The names that cut alike and the spellings of a key number
// alike, each number tried once (Codex M7 review R3). A number may be
// passed over that is free: a name reserved for a page the import drops
// too deep is not held.
type children struct {
	nodes []domain.Node
	keys  map[string]bool
	next  map[string]int
}

// ImportedPage is an import's page: under ParentID (nil: the notebook's
// root), named Name, holding Content, which Parsed parsed.
type DuplicateSource struct {
	ID   uuid.UUID
	Path string
}

// CreatedNode includes an import skip without pretending a missing leaf was created.
type CreatedNode struct {
	domain.Node
	Duplicate *DuplicateSource
	Skipped   bool
}

type ImportedPage struct {
	HasChildren bool
	ParentID    *uuid.UUID
	Name        string
	Content     string
	Parsed      Parsed
	// Reserved tells whether a later node of the import, a sibling, is
	// named with the key: a name numbered because its own is held takes
	// none of those, which keep theirs (nil: none).
	Reserved func(key string) bool
}

// ImportedAsset is an import's attachment: under ParentID (nil: the
// root), named Name, its file Meta.
type ImportedAsset struct {
	ParentID *uuid.UUID
	Name     string
	Meta     AssetMeta
	// Reserved tells whether a later node of the import, a sibling, is
	// named with the key: a name numbered because its own is held takes
	// none of those, which keep theirs (nil: none).
	Reserved func(key string) bool
}

// CreatePage creates the page p, last among its siblings, at revision 1
// of its content. Its name, numbered when a sibling holds it, must be a
// title (422 otherwise: the import mends it first); a parent that is no
// page of the notebook is ErrNoParent; a page deeper than domain.MaxDepth
// is domain.ErrTooDeep, nothing written.
func (iu *ImportUnit) CreatePage(ctx context.Context, p ImportedPage) (CreatedNode, error) {
	line, err := iu.line(ctx, p.ParentID)
	if err != nil {
		return CreatedNode{}, err
	}
	if domain.Depth(line) > domain.MaxDepth {
		return CreatedNode{}, domain.ErrTooDeep
	}
	kids, err := iu.children(ctx, p.ParentID)
	if err != nil {
		return CreatedNode{}, err
	}
	title, err := kids.free(p.Name, false, p.Reserved)
	if err != nil {
		return CreatedNode{}, err
	}
	var duplicate *DuplicateSource
	if len(p.Content) > 0 {
		hash := sha256.Sum256([]byte(p.Content))
		duplicate, err = iu.duplicate(ctx, line, domain.KindPage, hash[:])
		if err != nil {
			return CreatedNode{}, err
		}
	}
	if duplicate != nil {
		if !p.HasChildren {
			return CreatedNode{Duplicate: duplicate, Skipped: true}, nil
		}
		p.Content = ""
		p.Parsed = Parsed{Facts: iu.empty}
	}
	n, renumbered, err := iu.u.insertPage(ctx, PageDraft{ParentID: p.ParentID, Title: title.Name, Content: p.Content, Facts: p.Parsed.Facts},
		title, kids.nodes, len(kids.nodes)-1, line)
	if err != nil {
		return CreatedNode{}, err
	}
	kids.add(n, renumbered)
	return CreatedNode{Node: n, Duplicate: duplicate, Skipped: duplicate != nil}, nil
}

// CreateAsset creates the attachment a, last among its siblings; after
// runs in the unit's transaction once the node is written, and its error
// rolls the whole unit back. Its name, numbered when a sibling holds it,
// must be an attachment's (422 otherwise); a parent that is no page of
// the notebook is ErrNoParent. An attachment is no level of the tree.
func (iu *ImportUnit) CreateAsset(ctx context.Context, a ImportedAsset, after func(ctx context.Context, n domain.Node) error) (
	CreatedNode, error,
) {
	line, err := iu.line(ctx, a.ParentID)
	if err != nil {
		return CreatedNode{}, err
	}
	kids, err := iu.children(ctx, a.ParentID)
	if err != nil {
		return CreatedNode{}, err
	}
	title, err := kids.free(a.Name, true, a.Reserved)
	if err != nil {
		return CreatedNode{}, err
	}
	duplicate, err := iu.duplicate(ctx, line, domain.KindAsset, a.Meta.SHA256)
	if err != nil {
		return CreatedNode{}, err
	}
	if duplicate != nil {
		return CreatedNode{Duplicate: duplicate, Skipped: true}, nil
	}
	n, renumbered, err := iu.u.insertAsset(ctx, AssetDraft{ParentID: a.ParentID, Name: title.Name, Meta: a.Meta}, title, kids.nodes, &StepLines{After: line})
	if err != nil {
		return CreatedNode{}, err
	}
	kids.add(n, renumbered)
	if err := after(ctx, n); err != nil {
		return CreatedNode{}, err
	}
	return CreatedNode{Node: n}, nil
}

// line is the line of parentID, read once in the unit: the ancestors from
// the root down to the parent, none for the root. ErrNoParent for a parent
// that is no page of the notebook.
func (iu *ImportUnit) line(ctx context.Context, parentID *uuid.UUID) ([]domain.Ancestor, error) {
	key := keyOf(parentID)
	if line, ok := iu.lines[key]; ok {
		return line, nil
	}
	line, err := iu.u.lineUnder(ctx, parentID)
	switch {
	case errors.Is(err, errNotAPage):
		return nil, ErrNoParent
	case err != nil:
		return nil, err
	}
	iu.lines[key] = line
	return line, nil
}

// children are parentID's children, read once in the unit.
func (iu *ImportUnit) children(ctx context.Context, parentID *uuid.UUID) (*children, error) {
	key := keyOf(parentID)
	if kids, ok := iu.under[key]; ok {
		return kids, nil
	}
	nodes, err := iu.u.w.d.Nodes.Children(ctx, iu.u.write.NotebookID, parentID)
	if err != nil {
		return nil, err
	}
	kids := &children{nodes: nodes, keys: make(map[string]bool, len(nodes)), next: map[string]int{}}
	for _, n := range nodes {
		kids.keys[n.NameKey] = true
	}
	if parentID == nil && iu.u.write.Notebook.Mode == domain.ModeLLMWiki {
		for _, key := range domain.ReservedKeys() {
			kids.keys[key] = true
		}
	}
	iu.under[key] = kids
	return kids, nil
}

// free is name checked, a page's title or an attachment's name; when one
// of the children holds its key, numbered "name 2", "name 3"… until none
// does nor is it reserved, each family from the number after the last
// tried of it in the unit.
func (c *children) free(name string, asset bool, reserved func(key string) bool) (domain.Title, error) {
	check := domain.CheckTitle
	if asset {
		check = domain.CheckAssetName
	}
	title, err := check("name", name)
	if err != nil || !c.keys[title.Key] {
		return title, err
	}
	for n := 2; ; {
		at := shared.TitleKey(domain.Family(name, n, asset))
		if last := c.next[at]; last > n {
			n = last // of more digits, the name may be cut shorter
			continue
		}
		c.next[at] = n + 1
		title, err = check("name", domain.Numbered(name, n, asset))
		if err != nil || !c.keys[title.Key] && (reserved == nil || !reserved(title.Key)) {
			return title, err
		}
		n++
	}
}

// add adds n, created last among the children, whose orders are
// renumbered when that is set.
func (c *children) add(n domain.Node, renumbered []float64) {
	for i, o := range renumbered {
		c.nodes[i].SortOrder = o
	}
	c.nodes = append(c.nodes, n)
	c.keys[n.NameKey] = true
}

// keyOf is a parent's key among a unit's reads: uuid.Nil() for the root.
func keyOf(parentID *uuid.UUID) uuid.UUID {
	if parentID == nil {
		return uuid.Nil()
	}
	return *parentID
}

// Import skips are successful report facts; their target can be another node
// already created in this same transaction. No rollback problem ID escapes.
func (iu *ImportUnit) duplicate(ctx context.Context, line []domain.Ancestor, kind domain.Kind, hash []byte) (*DuplicateSource, error) {
	state := iu.u.write.Notebook
	if state.Mode != domain.ModeLLMWiki {
		return nil, nil
	}
	inside := false
	for _, a := range line {
		inside = inside || a.ID == state.SourceRootID
	}
	if !inside {
		return nil, nil
	}
	key, ok := sourceKey(kind, hash)
	if !ok {
		return nil, errors.New("invalid imported source hash")
	}
	witnesses, err := probeSourceWitnesses(ctx, iu.u.w.d.Sources, iu.u.write.NotebookID, []SourceKey{key}, nil)
	if err != nil {
		return nil, err
	}
	id, ok := witnesses[key]
	if !ok {
		return nil, nil
	}
	paths, err := iu.u.w.d.Paths.PathsOf(ctx, iu.u.write.NotebookID, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	path, ok := paths[id]
	if !ok {
		return nil, errors.New("import duplicate target has no current path")
	}
	return &DuplicateSource{ID: id, Path: path}, nil
}
