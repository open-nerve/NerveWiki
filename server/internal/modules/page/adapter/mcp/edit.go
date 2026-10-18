package mcpadapter

import (
	"context"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// editIn is edit's arguments (M9/P2 design 3.11.2).
type editIn struct {
	Path        string  `json:"path" jsonschema:"The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."`
	OldString   string  `json:"old_string" jsonschema:"The text to replace, copied from what read gave, byte for byte: 1 byte to 1 MiB."`
	NewString   string  `json:"new_string" jsonschema:"The text to put in its place, which differs from old_string."`
	ReplaceAll  *bool   `json:"replace_all,omitempty" jsonschema:"Replace every occurrence of old_string, left to right. Default false: old_string must occur exactly once."`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"The page's version, as read, list or write gave it: the edit is refused when the page changed since. Without it, the edit applies to the content as it is now."`
}

// editAnswer is edit's answer: changed false, and changeset_id and at
// null, when the content stayed as it was (M9/P2 design 3.11.2).
type editAnswer struct {
	Path         string     `json:"path"`
	NodeID       string     `json:"node_id"`
	Version      string     `json:"version"`
	Replacements int        `json:"replacements"`
	Changed      bool       `json:"changed"`
	ChangesetID  *string    `json:"changeset_id"`
	At           *time.Time `json:"at"`
}

// edit replaces old_string with new_string in the page at the path.
func (t tools) edit(ctx context.Context, c mcpserver.Call, in editIn) (mcpserver.Result, error) {
	e, err := t.uc.Edit.Execute(ctx, c.Notebook, app.PageEdit{Path: in.Path, Old: in.OldString, New: in.NewString,
		All: in.ReplaceAll != nil && *in.ReplaceAll, Base: in.BaseVersion}, domain.MCPClient(c.Client))
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, in.Path)
	}
	changeset, at := changesetOf(e.Outcome)
	return mcpserver.Result{Structured: editAnswer{Path: e.Path, NodeID: e.Node.ID.String(), Version: e.Version.String(),
		Replacements: e.Replacements, Changed: changeset != nil, ChangesetID: changeset, At: at}}, nil
}
