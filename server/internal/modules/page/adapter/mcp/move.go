package mcpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// moveIn is move's arguments (M9/P2 design 3.11.3): update_links true when
// left out or null.
type moveIn struct {
	From        string `json:"from" jsonschema:"The path of the page or attachment to move, from the notebook's root."`
	To          string `json:"to" jsonschema:"The full path it will have, its new name last: never a page to move it into. The page above it must exist."`
	UpdateLinks *bool  `json:"update_links,omitempty" jsonschema:"Rewrite the links to what moves, and to its pages, so that they follow it. Default true."`
}

// moveAnswer is move's answer: changeset_id and at null when nothing
// moved.
type moveAnswer struct {
	Path        string     `json:"path"`
	NodeID      string     `json:"node_id"`
	Changed     bool       `json:"changed"`
	ChangesetID *string    `json:"changeset_id"`
	At          *time.Time `json:"at"`
}

// move moves, renames or both the node at from to its full new path, to.
func (t tools) move(ctx context.Context, c mcpserver.Call, in moveIn) (mcpserver.Result, error) {
	m, err := t.uc.Move.Execute(ctx, c.Notebook, app.PathMove{From: in.From, To: in.To,
		UpdateLinks: in.UpdateLinks == nil || *in.UpdateLinks}, domain.MCPClient(c.Client))
	if err != nil {
		return mcpserver.Result{}, t.duplicateErr(ctx, c, moveErr(err, c, in.From))
	}
	changeset, at := changesetOf(m.Outcome)
	return mcpserver.Result{Structured: moveAnswer{Path: m.Path, NodeID: m.Node.ID.String(), Changed: changeset != nil,
		ChangesetID: changeset, At: at}}, nil
}

// moveErr is move's err: to taken, by a page or an attachment, told with
// what to is (M9/P2 design 3.11.5); any other as toolErr has it, from the
// path the call moves.
func moveErr(err error, c mcpserver.Call, from string) error {
	var taken *domain.PathTaken
	if !errors.As(err, &taken) {
		return toolErr(err, c, from)
	}
	holder := "a page there has that title"
	if taken.Asset {
		holder = "an attachment there has that name"
	}
	return &mcpserver.ToolError{Err: err, Text: "page.title_taken: " + domain.Quoted(taken.Path) + " is taken: " + holder +
		". to is the full new path of what moves, never a page to move it into."}
}
