package mcpadapter

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// changesetIn is changeset's arguments (M9/P2 design 3.16.3): id a
// string, as mcpserver takes no UUID's field.
type changesetIn struct {
	ID     string  `json:"id" jsonschema:"The changeset's id, as log gives it."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many changes, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// changesetAnswer is changeset's answer (M9/P2 design 3.16.3): the
// changeset on the first page, null with a cursor; a page of its changes
// by node id, each node with its path; next_cursor null when nothing
// follows.
type changesetAnswer struct {
	Changeset  *changesetHeader   `json:"changeset"`
	Changes    []change[pathNode] `json:"changes"`
	NextCursor *string            `json:"next_cursor"`
}

// errNoChangeFits is a change over the answer's bound on a page without
// the changeset, which no path's bound lets happen (M9/P2 design 3.16.4).
var errNoChangeFits = errors.New("page: changeset's first change does not fit in an answer")

// changeset reads the notebook's changeset id: as many of a page of its
// changes as the answer holds, next_cursor going on after the last of
// them. A first page whose changeset leaves its first change no room
// answers the changeset alone, next_cursor its first change's page: each
// call goes on (M9/P2 design 3.16.3, 3.16.4).
func (t tools) changeset(ctx context.Context, c mcpserver.Call, in changesetIn) (mcpserver.Result, error) {
	id, err := uuid.Parse(in.ID)
	if err != nil {
		id = uuid.UUID{} // no changeset's: the use case finds none, unread
	}
	p, err := t.uc.Changeset.InNotebook(ctx, c.Notebook, id, in.Limit, in.Cursor)
	if err != nil {
		return mcpserver.Result{}, err
	}
	var header *changesetHeader
	if d := p.Detail; d != nil {
		header = &changesetHeader{changesetFields: fieldsOf(d.LoggedChangeset), Renames: d.Moves.Renames, Moves: d.Moves.Moves}
	}
	changes := make([]change[pathNode], len(p.Changes))
	for i, r := range p.Changes {
		node := pathNode{changeNode: nodeOf(r)}
		if path, ok := p.Paths[r.NodeID]; ok {
			node.Path = &path
		}
		changes[i] = changeOf(r, node)
	}
	n := len(changes)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		next := nextCursor(k, n, p.Cursors, p.NextCursor)
		if k == 0 && n > 0 {
			next = &p.Start
		}
		return changesetAnswer{Changeset: header, Changes: changes[:k], NextCursor: next}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0 && header == nil:
		return mcpserver.Result{}, errNoChangeFits
	}
	return res, nil
}
