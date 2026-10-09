package page

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The tree's writes the other modules make, in units of the wired module:
// the asset module's attachments, whose files' rows join their units (M7/P2
// design 3.3), and the transfer module's imports (M7/P6 design 3.3). Only
// the wired module gives them: the command line's composition, which
// builds no module, cannot reach them.
type (
	// NewAsset is an attachment to create.
	NewAsset = app.NewAsset
	// AssetMeta is an attachment's file as the guards see it in a step.
	AssetMeta = app.AssetMeta
	// Client is where a write came from: web, api or cli.
	Client = domain.Client
	// ImportSpec is an import's unit.
	ImportSpec = app.ImportSpec
	// Parsed is a page's content parsed for an import's unit.
	Parsed = app.Parsed
)

// ErrTooDeep is an import's page deeper than pages go: nothing of it was
// written, and the unit goes on.
var ErrTooDeep = domain.ErrTooDeep

// ErrNoParent is an import's parent that is no page of the notebook.
var ErrNoParent = app.ErrNoParent

// TreeWrites creates the attachments' nodes, and an import's.
type TreeWrites interface {
	// Check decides as CreateAsset would, unlocked: notebook.not_found,
	// forbidden, then the name's rules and a parent that is no page of
	// the notebook (validation_failed), and page.title_taken for a name a
	// sibling holds now.
	Check(ctx context.Context, a NewAsset) error
	// CreateAsset creates the attachment's node in a unit that changes
	// the tree, last among its siblings; after runs in the unit's
	// transaction once the node is written, and its error rolls the unit
	// back.
	CreateAsset(ctx context.Context, a NewAsset, after func(ctx context.Context, n NodeInfo) error) (NodeInfo, error)
	// CheckContent checks a page's content as a unit would, reading
	// nothing: 422 past 5 MiB, not UTF-8, or holding NUL.
	CheckContent(content string) error
	// Parse parses a page's content, which CheckContent passed, for an
	// import's unit, outside it: 503 server_busy when the parse budget
	// does not free up in time. The import calls Release once the unit is
	// over.
	Parse(ctx context.Context, content string) (Parsed, error)
	// Import runs do in an import's unit, of the import kind, merged into
	// spec's changeset when it is set, and answers the unit's changeset,
	// none when it wrote nothing. notebook.not_found for a notebook the
	// job cannot see, forbidden for a reader.
	Import(ctx context.Context, spec ImportSpec, do func(ctx context.Context, u ImportUnit) error) (uuid.UUID, error)
}

// ImportUnit is an import's unit (M7/P6 design 3.3): each node created
// last among its siblings, its name numbered when a sibling holds it.
type ImportUnit interface {
	// CreatePage creates a page at revision 1 of its content: ErrTooDeep,
	// nothing written, for one deeper than pages go; ErrNoParent; 422 for
	// a name that is no title.
	CreatePage(ctx context.Context, p ImportedPage) (NodeInfo, error)
	// CreateAsset creates an attachment; after runs in the unit's
	// transaction once the node is written. ErrNoParent; 422 for a name
	// that is no attachment's.
	CreateAsset(ctx context.Context, a ImportedAsset, after func(ctx context.Context, n NodeInfo) error) (NodeInfo, error)
}

// ImportedPage is an import's page: under ParentID (nil: the root), named
// Name, holding Content, which Parsed parsed.
type ImportedPage struct {
	ParentID *uuid.UUID
	Name     string
	Content  string
	Parsed   Parsed
}

// ImportedAsset is an import's attachment: under ParentID (nil: the
// root), named Name, its file Meta.
type ImportedAsset struct {
	ParentID *uuid.UUID
	Name     string
	Meta     AssetMeta
}

// NodeInfo is a node of a notebook's tree, as the asset module reads it.
type NodeInfo struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	// Asset is an attachment's node; a page's otherwise.
	Asset     bool
	Name      string
	NameKey   string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func nodeInfo(n domain.Node) NodeInfo {
	return NodeInfo{ID: n.ID, NotebookID: n.NotebookID, ParentID: n.ParentID, Asset: n.Kind == domain.KindAsset, Name: n.Name,
		NameKey: n.NameKey, CreatedBy: n.CreatedBy, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}
}

// TreeWrites are the wired module's writes of attachments' nodes and of
// imports.
func (m *Module) TreeWrites() TreeWrites {
	return treeWrites{assets: m.assets, imports: m.imports}
}

type treeWrites struct {
	assets  *app.AssetWrites
	imports *app.ImportWrites
}

func (t treeWrites) CheckContent(content string) error {
	return t.imports.CheckContent(content)
}

func (t treeWrites) Parse(ctx context.Context, content string) (Parsed, error) {
	return t.imports.Parse(ctx, content)
}

func (t treeWrites) Import(ctx context.Context, spec ImportSpec, do func(ctx context.Context, u ImportUnit) error) (uuid.UUID, error) {
	return t.imports.Import(ctx, spec, func(ctx context.Context, u *app.ImportUnit) error {
		return do(ctx, importUnit{u: u})
	})
}

// importUnit is app's ImportUnit as the root gives it.
type importUnit struct {
	u *app.ImportUnit
}

func (i importUnit) CreatePage(ctx context.Context, p ImportedPage) (NodeInfo, error) {
	n, err := i.u.CreatePage(ctx, app.ImportedPage(p))
	return nodeInfo(n), err
}

func (i importUnit) CreateAsset(ctx context.Context, a ImportedAsset, after func(ctx context.Context, n NodeInfo) error) (NodeInfo, error) {
	n, err := i.u.CreateAsset(ctx, app.ImportedAsset(a), func(ctx context.Context, n domain.Node) error {
		return after(ctx, nodeInfo(n))
	})
	return nodeInfo(n), err
}

func (t treeWrites) Check(ctx context.Context, a NewAsset) error {
	return t.assets.Check(ctx, a)
}

func (t treeWrites) CreateAsset(ctx context.Context, a NewAsset, after func(ctx context.Context, n NodeInfo) error) (NodeInfo, error) {
	n, err := t.assets.CreateAsset(ctx, a, func(ctx context.Context, n domain.Node) error {
		return after(ctx, nodeInfo(n))
	})
	return nodeInfo(n), err
}
