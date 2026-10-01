package bootstrap

import (
	"net/http"
	"slices"
	"testing"
)

// The notebook module's rows (M3/P1 design 3.11). The list and the
// creation name a workspace: the list by the notebook columns, which see
// lab's notebooks each by its role, the creation by the workspace columns.
// The rest name a notebook, each column its own (notebookOf).

// notebookAnswer is a Notebook answer, as much as the rows check.
type notebookAnswer struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	MemberCount int    `json:"member_count"`
}

func notebookMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "notebook.not_found"}
	path := func(c caller, s seeded) string { return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() }
	// adminsOnly answers a notebook's admin, refuses the other roles in it,
	// and does not show it to the rest.
	adminsOnly := func(answer cell) map[caller]cell {
		cells := map[caller]cell{}
		for _, c := range notebookColumns() {
			_, seen := roleIn(c)
			switch {
			case c == callerNotebookAdmin:
				cells[c] = answer
			case seen:
				cells[c] = cellForbidden()
			default:
				cells[c] = notFound
			}
		}
		return cells
	}
	return []matrixRow{
		{
			op:      "listNotebooks",
			columns: notebookColumns(),
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/notebooks", ""
			},
			cells: everyNotebookColumn(cellOK(), map[caller]cell{callerOutsideWorkspace: {http.StatusNotFound, "workspace.not_found"}}),
			// Each column sees the notebooks it has a role in, with that role,
			// by name: never a deleted one, nor one of an ended membership.
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []notebookAnswer }
				decodeAnswer(t, answer, &list)
				if want := listedNotebooks(c); !slices.Equal(list.Data, want) {
					t.Errorf("listed %+v, want %+v", list.Data, want)
				}
			},
		},
		{
			// The list's rule is the workspace level's: the workspace columns
			// see acme's, which has no notebook, and the rest not acme.
			op:      "listNotebooks",
			variant: "by the workspace columns",
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/notebooks", ""
			},
			cells: map[caller]cell{
				callerAdmin: cellOK(), callerMember: cellOK(), callerGuest: cellOK(),
				callerNever: {http.StatusNotFound, "workspace.not_found"}, callerEnded: {http.StatusNotFound, "workspace.not_found"},
				callerDeleted: {http.StatusNotFound, "workspace.not_found"},
			},
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []notebookAnswer }
				decodeAnswer(t, answer, &list)
				if len(list.Data) != 0 {
					t.Errorf("listed %+v, want none: acme has no notebook", list.Data)
				}
			},
		},
		{
			op:    "createNotebook",
			write: true,
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/workspaces/" + workspaceOf(c) + "/notebooks", `{"name":"New"}`
			},
			cells: map[caller]cell{
				callerAdmin: cellCreated(), callerMember: cellCreated(), callerGuest: cellForbidden(),
				callerNever: {http.StatusNotFound, "workspace.not_found"}, callerEnded: {http.StatusNotFound, "workspace.not_found"},
				callerDeleted: {http.StatusNotFound, "workspace.not_found"},
			},
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var n notebookAnswer
				decodeAnswer(t, answer, &n)
				if n != (notebookAnswer{"New", "admin", 1}) {
					t.Errorf("created %+v, want New with the caller as its one member and admin", n)
				}
			},
		},
		{
			op:      "getNotebook",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, path(c, s), "" },
			cells: func() map[caller]cell {
				cells := map[caller]cell{}
				for _, c := range notebookColumns() {
					if _, seen := roleIn(c); seen {
						cells[c] = cellOK()
					} else {
						cells[c] = notFound
					}
				}
				return cells
			}(),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var n notebookAnswer
				decodeAnswer(t, answer, &n)
				if role, _ := roleIn(c); n.Name != notebookOf(c) || n.Role != role {
					t.Errorf("read %+v, want %s as %s", n, notebookOf(c), role)
				}
			},
		},
		{
			op:      "updateNotebook",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, path(c, s), `{"name":"Renamed","workspace_access":"viewer"}`
			},
			cells: adminsOnly(cellOK()),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var n notebookAnswer
				decodeAnswer(t, answer, &n)
				if n != (notebookAnswer{"Renamed", "admin", 3}) {
					t.Errorf("updated %+v, want Renamed, as its admin, with its three members", n)
				}
			},
		},
		{
			op:      "deleteNotebook",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) { return http.MethodDelete, path(c, s), "" },
			cells:   adminsOnly(cell{status: http.StatusNoContent}),
		},
	}
}

// notebookOf is the notebook a notebook column's cells target.
func notebookOf(c caller) string {
	switch c {
	case callerDefaultEditor, callerOutsideGuest, callerGuestReaderOfOpen:
		return "team"
	case callerDefaultReader:
		return "wiki"
	case callerNotebookDeleted:
		return "gone-nb"
	}
	return "priv"
}

// roleIn is a notebook column's effective role in its notebook, and
// whether it has one: the higher of its membership's and the default the
// workspace access gives an admin or a member, never a guest.
func roleIn(c caller) (string, bool) {
	role, ok := map[caller]string{
		callerNotebookAdmin: "admin", callerNotebookEditor: "editor", callerNotebookReader: "reader",
		callerDefaultEditor: "editor", callerDefaultReader: "reader", callerGuestReaderOfOpen: "reader",
	}[c]
	return role, ok
}

// listedNotebooks are the notebooks of lab a notebook column sees, by
// name, each with its role and its active members: priv's ended member is
// not counted.
func listedNotebooks(c caller) []notebookAnswer {
	priv := func(role string) notebookAnswer { return notebookAnswer{"priv", role, 3} }
	team := func(role string) notebookAnswer { return notebookAnswer{"team", role, 2} }
	wiki := func(role string) notebookAnswer { return notebookAnswer{"wiki", role, 1} }
	open := []notebookAnswer{team("editor"), wiki("reader")}
	return map[caller][]notebookAnswer{
		callerNotebookAdmin:     {priv("admin"), team("editor"), wiki("reader")},
		callerNotebookEditor:    {priv("editor"), team("editor"), wiki("reader")},
		callerNotebookReader:    {priv("reader")},
		callerOutsideAdmin:      {team("admin"), wiki("admin")},
		callerOutsideMember:     open,
		callerDefaultEditor:     open,
		callerDefaultReader:     open,
		callerOutsideGuest:      {},
		callerNotebookEnded:     open,
		callerNotebookDeleted:   open,
		callerGuestReaderOfOpen: {team("reader")},
	}[c]
}

// everyNotebookColumn is answer in every notebook column but those of
// except.
func everyNotebookColumn(answer cell, except map[caller]cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range notebookColumns() {
		cells[c] = answer
		if other, ok := except[c]; ok {
			cells[c] = other
		}
	}
	return cells
}
