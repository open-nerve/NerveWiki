package mcpadapter

import (
	"context"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// deleteIn is delete's arguments (M9/P2 design 3.11.4).
type deleteIn struct {
	Path      string `json:"path" jsonschema:"The path of the page or attachment to delete, from the notebook's root."`
	Recursive *bool  `json:"recursive,omitempty" jsonschema:"Delete a page with its children, its pages and attachments. Default false: a page with children is refused."`
}

// deleteAnswer is delete's answer: the pages and attachments that went to
// the trash, the node deleted counted; a deletion always writes.
type deleteAnswer struct {
	Path        string    `json:"path"`
	NodeID      string    `json:"node_id"`
	Pages       int       `json:"pages"`
	Attachments int       `json:"attachments"`
	ChangesetID string    `json:"changeset_id"`
	At          time.Time `json:"at"`
}

// delete puts the node at the path in the trash, with its subtree when
// recursive.
func (t tools) delete(ctx context.Context, c mcpserver.Call, in deleteIn) (mcpserver.Result, error) {
	d, err := t.uc.Delete.Execute(ctx, c.Notebook, app.PathDelete{Path: in.Path, Recursive: in.Recursive != nil && *in.Recursive},
		domain.MCPClient(c.Client))
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, in.Path)
	}
	return mcpserver.Result{Structured: deleteAnswer{Path: d.Path, NodeID: d.Node.ID.String(), Pages: d.Pages, Attachments: d.Assets,
		ChangesetID: d.Outcome.ChangesetID.String(), At: d.Outcome.At.UTC()}}, nil
}
