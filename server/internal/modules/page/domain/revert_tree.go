package domain

import "uuid"

// RevertTreeNode is one live node's structural state for revert planning.
// The complete tree includes pages and assets, without per-node audit data.
type RevertTreeNode struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Kind      Kind
	Name      string
	NameKey   string
	SortOrder float64
}

// RevertTreeNodeOf projects the structural state without normalizing its stored
// name key. Content-only plans still read their complete current nodes.
func RevertTreeNodeOf(n Node) RevertTreeNode {
	return RevertTreeNode{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind,
		Name: n.Name, NameKey: n.NameKey, SortOrder: n.SortOrder}
}
