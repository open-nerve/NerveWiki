package mcpadapter

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The history's answers as MCP writes them (M9/P2 design 3.16): REST's
// shapes, member for member (page/adapter/http's history.go and
// versions.go), but for a node's kind, "attachment" where REST writes
// "asset", as every tool says it; times in UTC.

// nextCursor is the next_cursor of an answer of the first k of n items,
// each with the cursor that goes on after it in cursors: next, the list's,
// when all fit; else the cursor after the last that fits; null when
// nothing follows, and when no item fits, which the tool answers itself.
func nextCursor(k, n int, cursors []string, next string) *string {
	switch {
	case k == 0 && n > 0:
		return nil
	case k < n:
		next = cursors[k-1]
	}
	return cursorOf(next)
}

// cursorOf is a cursor as an answer writes it: null for none.
func cursorOf(cursor string) *string {
	if cursor == "" {
		return nil
	}
	return &cursor
}

// userRef is an account by its id and its display name, as of the
// answer.
type userRef struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
}

// versionRow is a page's version, as REST's PageVersion.
type versionRow struct {
	ID           uuid.UUID        `json:"id"`
	Revision     int              `json:"revision"`
	BaseRevision *int             `json:"base_revision"`
	ByteSize     int              `json:"byte_size"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	Editing      bool             `json:"editing"`
	Changeset    versionChangeset `json:"changeset"`
}

// versionChangeset is the changeset a version is of, as REST's
// VersionChangeset.
type versionChangeset struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Client    string    `json:"client"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy userRef   `json:"created_by"`
}

func versionOf(v domain.Version) versionRow {
	c := v.Changeset
	return versionRow{ID: v.ID, Revision: v.Revision, BaseRevision: v.Base, ByteSize: v.ByteSize, CreatedAt: v.CreatedAt.UTC(),
		UpdatedAt: v.UpdatedAt.UTC(), Editing: v.Editing, Changeset: versionChangeset{ID: c.ID, Kind: string(c.Kind),
			Client: string(c.Client), CreatedAt: c.CreatedAt.UTC(), CreatedBy: userRef{UserID: c.By, DisplayName: c.ByName}}}
}

// changesetFields are a changeset's members as REST's Changeset has them,
// its changes aside.
type changesetFields struct {
	ID           uuid.UUID       `json:"id"`
	NotebookID   uuid.UUID       `json:"notebook_id"`
	Kind         string          `json:"kind"`
	Client       string          `json:"client"`
	Message      *string         `json:"message"`
	CreatedBy    userRef         `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Reverts      *changesetBrief `json:"reverts"`
	RevertedBy   []uuid.UUID     `json:"reverted_by"`
	Trimmed      bool            `json:"trimmed"`
	NodeCount    int             `json:"node_count"`
	ContentCount int             `json:"content_count"`
}

// changesetBrief is the changeset a revert reverts, as REST's
// ChangesetBrief.
type changesetBrief struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	CreatedBy userRef   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// changesetRow is a changeset as log lists it: REST's Changeset, its first
// changes without their nodes' paths (a summary, M9/P2 design 2.1 item 8);
// changes_cut true when the answer holds only some of them (log's
// fallback), absent otherwise.
type changesetRow struct {
	changesetFields
	Changes    []change[changeNode] `json:"changes"`
	ChangesCut bool                 `json:"changes_cut,omitempty"`
}

// changesetHeader is a changeset as changeset reads it: REST's
// getChangeset without its first changes, which its page of changes
// holds, and whether it renames or moves a node.
type changesetHeader struct {
	changesetFields
	Renames bool `json:"renames"`
	Moves   bool `json:"moves"`
}

func fieldsOf(l app.LoggedChangeset) changesetFields {
	c := l.Changeset
	out := changesetFields{ID: c.ID, NotebookID: c.NotebookID, Kind: string(c.Kind), Client: string(c.Client), Message: c.Message,
		CreatedBy: userRef{UserID: c.By, DisplayName: c.ByName}, CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC(),
		RevertedBy: append([]uuid.UUID{}, l.Facts.RevertedBy...), Trimmed: c.Trimmed, NodeCount: l.Facts.Nodes,
		ContentCount: l.Facts.Contents}
	if r := c.Reverts; r != nil {
		out.Reverts = &changesetBrief{ID: r.ID, Kind: string(r.Kind), CreatedBy: userRef{UserID: r.By, DisplayName: r.ByName},
			CreatedAt: r.CreatedAt.UTC()}
	}
	return out
}

func changesetRowOf(l app.LoggedChangeset) changesetRow {
	row := changesetRow{changesetFields: fieldsOf(l), Changes: make([]change[changeNode], len(l.Changes))}
	for i, r := range l.Changes {
		row.Changes[i] = changeOf(r, nodeOf(r))
	}
	return row
}

// change is what a changeset did to a node, as REST's ChangesetChange, its
// node N: log's without a path, changeset's with one.
type change[N any] struct {
	Node    N              `json:"node"`
	Kind    string         `json:"kind"`
	Before  *changeState   `json:"before"`
	After   *changeState   `json:"after"`
	Content *changeContent `json:"content"`
}

// changeNode is a changed node as it is now, as REST's ChangeNode, its
// kind "page" or "attachment".
type changeNode struct {
	ID    uuid.UUID `json:"id"`
	Kind  string    `json:"kind"`
	Name  string    `json:"name"`
	Alive bool      `json:"alive"`
}

// pathNode is a changed node with its path now: null for a node in the
// trash or gone, which no tool reaches by a path (M9/P2 design 2.1 item 8).
type pathNode struct {
	changeNode
	Path *string `json:"path"`
}

// changeState is where a node was, as REST's ChangeState.
type changeState struct {
	ParentID   *uuid.UUID `json:"parent_id"`
	ParentName *string    `json:"parent_name"`
	Name       string     `json:"name"`
}

// changeContent is the version of a page's content a change wrote, as
// REST's ChangeContent.
type changeContent struct {
	BaseRevision *int `json:"base_revision"`
	Revision     int  `json:"revision"`
}

func nodeOf(r domain.ChangeRow) changeNode {
	return changeNode{ID: r.NodeID, Kind: kindOf(r.NodeKind), Name: r.Name, Alive: r.Alive}
}

func changeOf[N any](r domain.ChangeRow, node N) change[N] {
	state := func(p *domain.ChangePlace) *changeState {
		if p == nil {
			return nil
		}
		return &changeState{ParentID: p.ParentID, ParentName: p.ParentName, Name: p.Name}
	}
	out := change[N]{Node: node, Kind: string(r.Change()), Before: state(r.Before), After: state(r.After)}
	if v := r.Content; v != nil {
		out.Content = &changeContent{BaseRevision: v.Base, Revision: v.Revision}
	}
	return out
}
