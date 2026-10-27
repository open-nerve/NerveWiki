package mcpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// writeIn is write's arguments (M9/P2 design 3.11.1).
type writeIn struct {
	Path        string  `json:"path" jsonschema:"The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."`
	Content     string  `json:"content" jsonschema:"The page's whole content, as it will be stored: frontmatter, byte order mark and line breaks included."`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"The page's version, as read, list or write gave it, to replace its content. Without it, the page is created."`
}

// writeAnswer is write's answer (M9/P2 design 2.1 item 11): created, the
// nodes the write created from the root down, its page last, fitted;
// created_count all of them; changeset_id and at null when nothing changed.
type writeAnswer struct {
	Path         string        `json:"path"`
	NodeID       string        `json:"node_id"`
	Version      string        `json:"version"`
	Changed      bool          `json:"changed"`
	Created      []createdNode `json:"created"`
	CreatedCount int           `json:"created_count"`
	ChangesetID  *string       `json:"changeset_id"`
	At           *time.Time    `json:"at"`
}

// createdNode is a node a write created.
type createdNode struct {
	Path   string `json:"path"`
	NodeID string `json:"node_id"`
}

// write writes the page at the path: created with its missing parents, or
// its content replaced on base_version. The answer holds as many of the
// nodes created as fit, each a path; their count is all of them.
func (t tools) write(ctx context.Context, c mcpserver.Call, in writeIn) (mcpserver.Result, error) {
	w, err := t.uc.Write.Execute(ctx, c.Notebook, app.PathWrite{Path: in.Path, Content: in.Content, Base: in.BaseVersion},
		domain.MCPClient(c.Client))
	if err != nil {
		return mcpserver.Result{}, t.duplicateErr(ctx, c, writeErr(err, c, in.Path))
	}
	created := make([]createdNode, len(w.Created))
	for i, n := range w.Created {
		created[i] = createdNode{Path: n.Path, NodeID: n.Node.ID.String()}
	}
	changeset, at := changesetOf(w.Outcome)
	res, _, err := mcpserver.Fit(len(created), func(k int) any {
		return writeAnswer{Path: w.Path, NodeID: w.Node.ID.String(), Version: w.Version.String(), Changed: changeset != nil,
			Created: created[:k], CreatedCount: len(created), ChangesetID: changeset, At: at}
	})
	return res, err
}

// writeErr is write's err: a node holding the path's name told as a
// creation meets it, a page there or an attachment (M9/P2 design 3.11.5);
// any other as toolErr has it.
func writeErr(err error, c mcpserver.Call, path string) error {
	var taken *domain.PathTaken
	switch {
	case !errors.As(err, &taken):
		return toolErr(err, c, path)
	case taken.Asset:
		return &mcpserver.ToolError{Err: err, Text: "page.title_taken: An attachment holds the name of " + domain.Quoted(taken.Path) +
			" there: a page there needs another title."}
	}
	return &mcpserver.ToolError{Err: err, Text: "page.title_taken: " + domain.Quoted(taken.Path) +
		" is a page already. To replace its content, write it with base_version, the version read gave."}
}
