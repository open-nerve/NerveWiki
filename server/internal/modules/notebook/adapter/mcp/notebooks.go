// Package mcpadapter is the notebook module's MCP adapter (M9 design 4.1,
// 4.6): the tool notebooks, its answer within mcpserver's bound. It
// imports mcpserver, never the SDK.
package mcpadapter

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// ListAccountNotebooksUseCase lists the caller's notebooks.
type ListAccountNotebooksUseCase interface {
	Execute(ctx context.Context) (app.AccountNotebooks, error)
}

// Tools are the module's MCP tools: notebooks, of the account entry alone.
func Tools(notebooks ListAccountNotebooksUseCase) []mcpserver.Tool {
	t := tools{notebooks: notebooks}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "notebooks", Title: "List your notebooks", Description: notebooksDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.AccountEntry}, t.list),
	}
}

// notebooksDescription is the tool's description, as api/mcp/tools.json
// has it (M9/P1 design 3.18).
const notebooksDescription = `List the notebooks you can see in every workspace you belong to, each with its id, its ` +
	`name, its workspace's slug and name, your role (admin, editor or reader) and its mode (standard, or llm_wiki for an ` +
	`LLM Wiki), by workspace, then by name. Open one with open_notebook, then pass its id as notebook to the other ` +
	`tools. truncated is true when there were more than one answer holds.`

// tools are the handlers of the module's tools.
type tools struct {
	notebooks ListAccountNotebooksUseCase
}

// notebooksAnswer is notebooks' answer (M9/P1 design 3.17.2).
type notebooksAnswer struct {
	Notebooks []notebookItem `json:"notebooks"`
	Truncated bool           `json:"truncated"`
}

type notebookItem struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Workspace workspaceItem `json:"workspace"`
	Role      string        `json:"role"`
	Mode      string        `json:"mode"`
}

type workspaceItem struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// errNoNotebookFits is a first notebook over the answer's bound, which the
// names' bounds do not let happen: one is at most about 4 KiB.
var errNoNotebookFits = errors.New("notebook: the first notebook does not fit in an answer")

// list lists the caller's notebooks: as many as the answer holds,
// truncated when some are left out, by the answer or by the use case.
func (t tools) list(ctx context.Context, _ mcpserver.Call, _ struct{}) (mcpserver.Result, error) {
	listed, err := t.notebooks.Execute(ctx)
	if err != nil {
		return mcpserver.Result{}, err
	}
	items := make([]notebookItem, len(listed.Notebooks))
	for i, n := range listed.Notebooks {
		items[i] = notebookItem{ID: n.Notebook.ID.String(), Name: n.Notebook.Name, Role: string(n.Role), Mode: string(n.Notebook.Mode),
			Workspace: workspaceItem{Slug: n.Workspace.Slug, Name: n.Workspace.Name}}
	}
	n := len(items)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return notebooksAnswer{Notebooks: items[:k], Truncated: k < n || listed.Truncated}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoNotebookFits
	}
	return res, nil
}
