package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// An import's unit (M7/P6 design 3.3): it creates pages and attachments
// alone, as CreatePage and CreateAsset do, but for three things. A name a
// sibling holds is numbered rather than refused; a page too deep is
// refused before anything is written, the unit going on; and the
// siblings and the line of each parent are read once in the unit, those
// it creates added to them: the unit holds the notebook's row FOR NO KEY
// UPDATE, so no other write of the tree changes them meanwhile.

// ErrNoParent is a parent of an import's node that is no page of the
// notebook: deleted, or moved away, since the import read it.
var ErrNoParent = errors.New("page: the parent is no page of the notebook")

// ImportUnit is an import's unit: Import's do calls its operations.
type ImportUnit struct {
	u *Unit
	// under are the children of each parent read in the unit, and lines
	// the line of each, by parent: uuid.Nil() for the root.
	under map[uuid.UUID]*children
	lines map[uuid.UUID][]domain.Ancestor
}

// children are a parent's children as the unit knows them, in order, and
// the keys of their names; next the number after the last a name took,
// by its key and whether it is an attachment's: the numbers before it are
// held, or were tried. A number may then be passed over that is free: a
// cut long name numbers otherwise in another spelling, and a name
// reserved for a page the import drops too deep is not held.
type children struct {
	nodes []domain.Node
	keys  map[string]bool
	next  map[numbering]int
}

// numbering is a name numbered, a page's or an attachment's, by its key:
// the spellings of a key number alike.
type numbering struct {
	key   string
	asset bool
}

// ImportedPage is an import's page: under ParentID (nil: the notebook's
// root), named Name, holding Content, which Parsed parsed.
type ImportedPage struct {
	ParentID *uuid.UUID
	Name     string
	Content  string
	Parsed   Parsed
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
func (iu *ImportUnit) CreatePage(ctx context.Context, p ImportedPage) (domain.Node, error) {
	line, err := iu.line(ctx, p.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	if domain.Depth(line) > domain.MaxDepth {
		return domain.Node{}, domain.ErrTooDeep
	}
	kids, err := iu.children(ctx, p.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	title, err := kids.free(p.Name, false, p.Reserved)
	if err != nil {
		return domain.Node{}, err
	}
	n, renumbered, err := iu.u.insertPage(ctx, PageDraft{ParentID: p.ParentID, Title: title.Name, Content: p.Content, Facts: p.Parsed.Facts},
		title, kids.nodes, len(kids.nodes)-1)
	if err != nil {
		return domain.Node{}, err
	}
	kids.add(n, renumbered)
	return n, nil
}

// CreateAsset creates the attachment a, last among its siblings; after
// runs in the unit's transaction once the node is written, and its error
// rolls the whole unit back. Its name, numbered when a sibling holds it,
// must be an attachment's (422 otherwise); a parent that is no page of
// the notebook is ErrNoParent. An attachment is no level of the tree.
func (iu *ImportUnit) CreateAsset(ctx context.Context, a ImportedAsset, after func(ctx context.Context, n domain.Node) error) (
	domain.Node, error,
) {
	if _, err := iu.line(ctx, a.ParentID); err != nil {
		return domain.Node{}, err
	}
	kids, err := iu.children(ctx, a.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	title, err := kids.free(a.Name, true, a.Reserved)
	if err != nil {
		return domain.Node{}, err
	}
	n, renumbered, err := iu.u.insertAsset(ctx, AssetDraft{ParentID: a.ParentID, Name: title.Name, Meta: a.Meta}, title, kids.nodes)
	if err != nil {
		return domain.Node{}, err
	}
	kids.add(n, renumbered)
	if err := after(ctx, n); err != nil {
		return domain.Node{}, err
	}
	return n, nil
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
	kids := &children{nodes: nodes, keys: make(map[string]bool, len(nodes)), next: map[numbering]int{}}
	for _, n := range nodes {
		kids.keys[n.NameKey] = true
	}
	iu.under[key] = kids
	return kids, nil
}

// free is name checked, a page's title or an attachment's name; when one
// of the children holds its key, numbered "name 2", "name 3"… until none
// does nor is it reserved, from the number after the last a name of its
// key took in the unit.
func (c *children) free(name string, asset bool, reserved func(key string) bool) (domain.Title, error) {
	check := domain.CheckTitle
	if asset {
		check = domain.CheckAssetName
	}
	title, err := check("name", name)
	if err != nil || !c.keys[title.Key] {
		return title, err
	}
	at := numbering{key: title.Key, asset: asset}
	for n := max(2, c.next[at]); ; n++ {
		title, err = check("name", domain.Numbered(name, n, asset))
		if err != nil {
			return title, err
		}
		if !c.keys[title.Key] && (reserved == nil || !reserved(title.Key)) {
			c.next[at] = n + 1
			return title, nil
		}
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
