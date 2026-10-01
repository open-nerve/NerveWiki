package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// The workspace module's rows of the permission matrix for the invitations
// (M2/P3 design 3.10). The preview and the acceptance have none: see
// matrixExempt.

// invitationAnswer is a WorkspaceInvitation answer, as much as the rows
// check.
type invitationAnswer struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	Token string `json:"token"`
}

func invitationMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "workspace.not_found"}
	invitationNotFound := cell{http.StatusNotFound, "workspace.invitation_not_found"}
	adminsOnly := func(ok cell, hidden cell) map[caller]cell {
		return map[caller]cell{
			callerAdmin: ok, callerMember: cellForbidden(), callerGuest: cellForbidden(),
			callerNever: hidden, callerEnded: hidden, callerDeleted: hidden,
		}
	}
	isInvitation := func(t *testing.T, got invitationAnswer, email string) {
		t.Helper()
		if got.Email != email || got.Role != "member" || !strings.HasPrefix(got.Token, "nwk_inv_") {
			t.Errorf("invitation %+v, want %s as a member, with its token", got, email)
		}
	}
	return []matrixRow{
		{
			op: "listWorkspaceInvitations",
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/invitations", ""
			},
			cells: adminsOnly(cellOK(), notFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []invitationAnswer }
				decodeAnswer(t, answer, &list)
				if len(list.Data) != 1 {
					t.Fatalf("listed %+v, want acme's one invitation", list.Data)
				}
				isInvitation(t, list.Data[0], matrixInvitee)
			},
		},
		{
			op:    "createWorkspaceInvitation",
			write: true,
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/workspaces/" + workspaceOf(c) + "/invitations", `{"email":"newcomer@example.com","role":"member"}`
			},
			cells: adminsOnly(cellCreated(), notFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var got invitationAnswer
				decodeAnswer(t, answer, &got)
				isInvitation(t, got, "newcomer@example.com")
			},
		},
		{
			op:    "deleteWorkspaceInvitation",
			write: true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/workspace-invitations/" + s.invitation(workspaceOf(c)).String(), ""
			},
			cells: adminsOnly(cell{status: http.StatusNoContent}, invitationNotFound),
		},
	}
}
