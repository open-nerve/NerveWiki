package bootstrap

import (
	"net/http"
	"slices"
	"testing"
)

// The notebook member operations' rows (M3/P2 design 3.8), by the notebook
// columns, each before its own notebook (notebookOf): a notebook's admin
// manages its members, its other roles are refused, and the rest do not
// see it.

// notebookMemberAnswer is a NotebookMember answer, as much as the rows
// check.
type notebookMemberAnswer struct {
	UserID      string  `json:"user_id"`
	Role        string  `json:"role"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email"`
}

// seededMembers are the active members of the notebook name, in the order
// the list gives them: seeded at one time, so by id, which newSeeded gave
// in this order.
func seededMembers(name string) []matrixNotebookMember {
	var list []matrixNotebookMember
	for _, m := range matrixNotebookMembers() {
		if m.notebook == name && !m.ended {
			list = append(list, m)
		}
	}
	return list
}

// targetMember is the membership the update and removal rows of c aim at:
// one of its notebook, which for priv's columns is the editor's, the
// guest reader's for team's, the admin's for wiki's and gone-nb's.
func targetMember(c caller, s seeded) string {
	n := notebookOf(c)
	target := map[string]caller{
		"priv": callerNotebookEditor, "team": callerGuestReaderOfOpen, "wiki": callerOutsideAdmin, "gone-nb": callerNotebookDeleted,
	}[n]
	return s.notebookMember(n, target).String()
}

// ownNotebookMembership is c's own membership of its notebook, ended or
// not, or, for a column with none, the notebook's admin's.
func ownNotebookMembership(c caller, s seeded) string {
	n := notebookOf(c)
	if id, ok := s.notebookMembers[n+"/"+string(c)]; ok {
		return id.String()
	}
	for _, m := range matrixNotebookMembers() {
		if m.notebook == n && m.role == "admin" {
			return s.notebookMember(n, m.c).String()
		}
	}
	s.t.Helper()
	s.t.Fatalf("no admin of %s is seeded", n)
	return ""
}

func notebookMemberMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "notebook.not_found"}
	memberNotFound := cell{http.StatusNotFound, "notebook.member_not_found"}
	// adminsOnly answers a notebook's admin, refuses its other roles, and
	// answers the rest hidden.
	adminsOnly := func(answer, hidden cell) map[caller]cell {
		cells := map[caller]cell{}
		for _, c := range notebookColumns() {
			_, seen := roleIn(c)
			switch {
			case c == callerNotebookAdmin:
				cells[c] = answer
			case seen:
				cells[c] = cellForbidden()
			default:
				cells[c] = hidden
			}
		}
		return cells
	}
	membersPath := func(c caller, s seeded) string {
		return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() + "/members"
	}
	return []matrixRow{
		{
			op:      "listNotebookMembers",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, membersPath(c, s), "" },
			cells: everyNotebookColumn(cellOK(), map[caller]cell{
				callerOutsideAdmin: notFound, callerOutsideMember: notFound, callerOutsideGuest: notFound,
				callerNotebookEnded: notFound, callerNotebookDeleted: notFound, callerOutsideWorkspace: notFound,
			}),
			// The notebook's active members, by id; the emails to the
			// workspace's admins and members, not to its guests.
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var list struct{ Data []notebookMemberAnswer }
				decodeAnswer(t, answer, &list)
				shown := c != callerNotebookReader && c != callerGuestReaderOfOpen
				var want []notebookMemberAnswer
				for _, m := range seededMembers(notebookOf(c)) {
					l := listed(m.c, string(m.role), shown)
					want = append(want, notebookMemberAnswer{UserID: s.accounts[m.c].String(), Role: l.Role, DisplayName: l.DisplayName, Email: l.Email})
				}
				if !slices.EqualFunc(list.Data, want, sameMemberAnswer) {
					t.Errorf("listed %+v, want %+v", list.Data, want)
				}
			},
		},
		{
			op:      "addNotebookMember",
			columns: notebookColumns(),
			write:   true,
			// The workspace member outside every notebook, as a reader.
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, membersPath(c, s), `{"user_id":"` + s.accounts[callerOutsideMember].String() + `","role":"reader"}`
			},
			cells: adminsOnly(cellCreated(), notFound),
			check: func(t *testing.T, _ caller, s seeded, answer string) {
				t.Helper()
				var m notebookMemberAnswer
				decodeAnswer(t, answer, &m)
				if l := listed(callerOutsideMember, "reader", true); m.UserID != s.accounts[callerOutsideMember].String() || m.Role != "reader" ||
					m.DisplayName != l.DisplayName || m.Email == nil || *m.Email != *l.Email {
					t.Errorf("added %+v, want the outside member as a reader, with its profile", m)
				}
			},
		},
		{
			op:      "updateNotebookMember",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, "/api/v0/notebook-members/" + targetMember(c, s), `{"role":"reader"}`
			},
			cells: adminsOnly(cellOK(), memberNotFound),
			check: func(t *testing.T, _ caller, s seeded, answer string) {
				t.Helper()
				var m notebookMemberAnswer
				decodeAnswer(t, answer, &m)
				if m.UserID != s.accounts[callerNotebookEditor].String() || m.Role != "reader" || m.DisplayName != listed(callerNotebookEditor, "", false).DisplayName {
					t.Errorf("updated %+v, want priv's editor as a reader, with its name", m)
				}
			},
		},
		{
			// The decision comes before rule one: only an admin learns its
			// own membership is refused.
			op:      "updateNotebookMember",
			variant: "one's own",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, "/api/v0/notebook-members/" + ownNotebookMembership(c, s), `{"role":"reader"}`
			},
			cells: adminsOnly(cell{http.StatusConflict, "notebook.own_membership"}, memberNotFound),
		},
		{
			op:      "removeNotebookMember",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/notebook-members/" + targetMember(c, s), ""
			},
			cells: adminsOnly(cell{status: http.StatusNoContent}, memberNotFound),
		},
		{
			op:      "removeNotebookMember",
			variant: "one's own",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/notebook-members/" + ownNotebookMembership(c, s), ""
			},
			cells: adminsOnly(cell{http.StatusConflict, "notebook.own_membership"}, memberNotFound),
		},
		{
			// priv's admin is its only one (rule one); the default roles
			// have no membership to end.
			op:      "leaveNotebook",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() + "/leave", ""
			},
			cells: everyNotebookColumn(notFound, map[caller]cell{
				callerNotebookAdmin: {http.StatusConflict, "notebook.sole_admin"}, callerNotebookEditor: {status: http.StatusNoContent},
				callerNotebookReader: {status: http.StatusNoContent}, callerGuestReaderOfOpen: {status: http.StatusNoContent},
				callerDefaultEditor: memberNotFound, callerDefaultReader: memberNotFound,
			}),
		},
	}
}

// sameMemberAnswer compares two member answers, the emails by value.
func sameMemberAnswer(a, b notebookMemberAnswer) bool {
	emailsAlike := (a.Email == nil) == (b.Email == nil) && (a.Email == nil || *a.Email == *b.Email)
	return a.UserID == b.UserID && a.Role == b.Role && a.DisplayName == b.DisplayName && emailsAlike
}
