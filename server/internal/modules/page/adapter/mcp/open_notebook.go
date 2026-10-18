package mcpadapter

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// OpenNotebookUseCase opens a notebook.
type OpenNotebookUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) (app.Opened, error)
}

// openNotebookDescription is the tool's description, as api/mcp/tools.json
// has it (M9/P1 design 3.18).
const openNotebookDescription = `Open a notebook: its instructions for agents (its root page AGENTS without its ` +
	`frontmatter, or the server's built-in ones when it has none) and an overview: how many pages and attachments it ` +
	`holds, when it last changed (null when nothing was ever written in it), and the nodes at its root. Follow its ` +
	`instructions while you work in it, and pass its id as notebook to the other tools. truncated is true when the ` +
	`root's nodes were cut short.`

// openNotebookRootReserve is what of the answer the instructions leave the
// root's nodes at least: 100 nodes of short names, 28 of 255 bytes.
const openNotebookRootReserve = 8 << 10

// openAnswer is open_notebook's answer (M9/P1 design 3.17.3):
// instructions_from is the AGENTS page's path now, null for the built-in
// instructions; truncated tells that the root's nodes were cut short.
type openAnswer struct {
	Notebook         string   `json:"notebook"`
	Instructions     string   `json:"instructions"`
	InstructionsFrom *string  `json:"instructions_from"`
	Overview         overview `json:"overview"`
	Truncated        bool     `json:"truncated"`
}

// overview is a notebook's overview: last_changed_at null when nothing
// was ever written in it.
type overview struct {
	Pages         int64      `json:"pages"`
	Attachments   int64      `json:"attachments"`
	LastChangedAt *time.Time `json:"last_changed_at"`
	Root          []rootItem `json:"root"`
}

type rootItem struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// openNotebook opens the call's notebook, its answer filled against what
// is left of the bound, the instructions first (M9/P1 design 3.17.3): the
// answer without instructions and nodes measured, the instructions cut to
// fit with openNotebookRootReserve left, as a member of the answer counts
// them, and within the handshake's 32 KiB; then as many of the root's
// nodes as the rest holds.
func (t tools) openNotebook(ctx context.Context, c mcpserver.Call, _ struct{}) (mcpserver.Result, error) {
	o, err := t.uc.Open.Execute(ctx, c.Notebook)
	if err != nil {
		return mcpserver.Result{}, err
	}
	answer := openAnswer{Notebook: c.Notebook.String(), Instructions: NotebookInstructions,
		Overview: overview{Pages: o.Pages, Attachments: o.Attachments, Root: []rootItem{}}}
	if o.LastChange != nil {
		at := o.LastChange.UTC()
		answer.Overview.LastChangedAt = &at
	}
	if o.HasAgents {
		from := o.Agents.Title + ".md"
		answer.InstructionsFrom = &from
		skeleton := answer
		skeleton.Instructions = ""
		text, err := line(skeleton)
		if err != nil {
			return mcpserver.Result{}, err
		}
		budget := mcpserver.MaxAnswerBytes - mcpserver.TextBytes(text) - openNotebookRootReserve
		answer.Instructions = domain.AgentsText(o.Agents, func(s string) bool {
			return mcpserver.TextBytes(s) <= domain.MaxInstructionsBytes && mcpserver.MemberBytes(s) <= budget
		})
	}
	root := make([]rootItem, len(o.Root))
	for i, n := range o.Root {
		root[i] = rootItem{Path: n.Path, Kind: kindOf(n.Kind)}
	}
	n := len(root)
	res, _, err := mcpserver.Fit(n, func(k int) any {
		a := answer
		a.Overview.Root, a.Truncated = root[:k], k < n || o.MoreRoot
		return a
	})
	return res, err
}

// kindOf is a node's kind as the tools tell it: "attachment", not the
// store's "asset".
func kindOf(k domain.Kind) string {
	if k == domain.KindPage {
		return "page"
	}
	return "attachment"
}
