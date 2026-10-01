package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The workspace module's rows of the permission matrix for managing a
// workspace and its members (M2/P2).

// memberAnswer is a WorkspaceMember answer, as much as the rows check.
type memberAnswer struct {
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email"`
	Role        string  `json:"role"`
}

// listed is c as the member list shows it, with its email when withEmail.
func listed(c caller, role string, withEmail bool) memberAnswer {
	email := emailOf(c)
	name, _, _ := strings.Cut(email, "@")
	a := memberAnswer{DisplayName: name, Role: role}
	if withEmail {
		a.Email = &email
	}
	return a
}

func sameMember(a, b memberAnswer) bool {
	return a.DisplayName == b.DisplayName && a.Role == b.Role &&
		(a.Email == nil) == (b.Email == nil) && (a.Email == nil || *a.Email == *b.Email)
}

// ownOrAdmins is the membership a row about one's own aims at: the
// column's own in its workspace, or, for the column that never was a
// member of acme, its admin's.
func ownOrAdmins(c caller, s seeded) string {
	if c == callerNever {
		return s.adminMembership("acme").String()
	}
	return s.membership(workspaceOf(c), c).String()
}

func memberMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "workspace.not_found"}
	memberNotFound := cell{http.StatusNotFound, "workspace.member_not_found"}
	adminsOnly := func(ok cell, hidden cell) map[caller]cell {
		return map[caller]cell{
			callerAdmin: ok, callerMember: cellForbidden(), callerGuest: cellForbidden(),
			callerNever: hidden, callerEnded: hidden, callerDeleted: hidden,
		}
	}
	// membersPath aims at another column's membership in the column's
	// workspace: the member column's, or for that column the guest's, so
	// that no column aims at its own (a row of its own does).
	membersPath := func(c caller, s seeded) string {
		target := callerMember
		if c == callerMember {
			target = callerGuest
		}
		return "/api/v0/workspace-members/" + s.membership(workspaceOf(c), target).String()
	}
	return []matrixRow{
		{
			op:    "updateWorkspace",
			write: true,
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodPatch, "/api/v0/workspaces/" + workspaceOf(c), `{"name":"Renamed"}`
			},
			cells: adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var w struct {
					Name string `json:"name"`
					Role string `json:"role"`
				}
				decodeAnswer(t, answer, &w)
				if w.Name != "Renamed" || w.Role != "admin" {
					t.Errorf("renamed %+v, want Renamed, as its admin", w)
				}
			},
		},
		{
			op:    "deleteWorkspace",
			write: true,
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/workspaces/" + workspaceOf(c), ""
			},
			cells: adminsOnly(cell{status: http.StatusNoContent}, notFound),
		},
		{
			op: "listWorkspaceMembers",
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/members", ""
			},
			cells: map[caller]cell{
				callerAdmin: cellOK(), callerMember: cellOK(), callerGuest: cellOK(),
				callerNever: notFound, callerEnded: notFound, callerDeleted: notFound,
			},
			// The active members, by when they joined (the order of the
			// seeding); a guest sees no email.
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []memberAnswer }
				decodeAnswer(t, answer, &list)
				emails := c != callerGuest
				want := []memberAnswer{listed(callerAdmin, "admin", emails), listed(callerMember, "member", emails), listed(callerGuest, "guest", emails)}
				if !slices.EqualFunc(list.Data, want, sameMember) {
					t.Errorf("listed %+v, want %+v", list.Data, want)
				}
			},
		},
		{
			op:    "updateWorkspaceMember",
			write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, membersPath(c, s), `{"role":"guest"}`
			},
			cells: adminsOnly(cellOK(), memberNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var m memberAnswer
				decodeAnswer(t, answer, &m)
				if want := listed(callerMember, "guest", true); !sameMember(m, want) {
					t.Errorf("updated %+v, want %+v", m, want)
				}
			},
		},
		{
			op:      "updateWorkspaceMember",
			variant: "one's own",
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, "/api/v0/workspace-members/" + ownOrAdmins(c, s), `{"role":"guest"}`
			},
			cells: adminsOnly(cell{http.StatusConflict, "workspace.own_membership"}, memberNotFound),
		},
		{
			op:    "removeWorkspaceMember",
			write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, membersPath(c, s), ""
			},
			cells: adminsOnly(cell{status: http.StatusNoContent}, memberNotFound),
		},
		{
			op:      "removeWorkspaceMember",
			variant: "one's own",
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/workspace-members/" + ownOrAdmins(c, s), ""
			},
			cells: adminsOnly(cell{http.StatusConflict, "workspace.own_membership"}, memberNotFound),
		},
		{
			op:    "leaveWorkspace",
			write: true,
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/workspaces/" + workspaceOf(c) + "/leave", ""
			},
			// acme's admin is its only one.
			cells: map[caller]cell{
				callerAdmin: {http.StatusConflict, "workspace.sole_admin"}, callerMember: {status: http.StatusNoContent},
				callerGuest: {status: http.StatusNoContent}, callerNever: notFound, callerEnded: notFound, callerDeleted: notFound,
			},
		},
	}
}
