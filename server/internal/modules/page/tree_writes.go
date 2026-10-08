package page

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The tree's writes the asset module makes (M7/P2 design 3.3): an
// attachment's node, created in a unit of the wired module, which the
// file's row joins. Only the wired module gives them: the command line's
// composition, which builds no module, cannot reach them.
type (
	// NewAsset is an attachment to create.
	NewAsset = app.NewAsset
	// AssetMeta is an attachment's file as the guards see it in a step.
	AssetMeta = app.AssetMeta
	// Client is where a write came from: web, api or cli.
	Client = domain.Client
)

// TreeWrites creates the attachments' nodes.
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

// TreeWrites are the wired module's writes of attachments' nodes.
func (m *Module) TreeWrites() TreeWrites {
	return treeWrites{assets: m.assets}
}

type treeWrites struct {
	assets *app.AssetWrites
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
