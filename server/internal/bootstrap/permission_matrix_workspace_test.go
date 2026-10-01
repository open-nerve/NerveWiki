package bootstrap

import (
	"net/http"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The workspace module's rows of the permission matrix.

// workspaceAnswer is a Workspace answer, as much as the rows check.
type workspaceAnswer struct {
	Slug string `json:"slug"`
	Role string `json:"role"`
}

func workspaceMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "workspace.not_found"}
	return []matrixRow{
		{
			op:      "listWorkspaces",
			request: sameRequest(http.MethodGet, "/api/v0/workspaces", ""),
			cells:   every(cellOK()),
			// Each column sees the workspaces of its active memberships:
			// not the deleted one, not the one it left.
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []workspaceAnswer }
				decodeAnswer(t, answer, &list)
				want := map[caller][]workspaceAnswer{
					callerAdmin:   {{"acme", "admin"}},
					callerMember:  {{"acme", "member"}},
					callerGuest:   {{"acme", "guest"}},
					callerNever:   {{"other", "admin"}},
					callerEnded:   {{"other", "member"}},
					callerDeleted: {},
				}[c]
				if !slices.Equal(list.Data, want) {
					t.Errorf("listed %+v, want %+v", list.Data, want)
				}
			},
		},
		{
			op:      "createWorkspace",
			write:   true,
			request: sameRequest(http.MethodPost, "/api/v0/workspaces", `{"name":"New","slug":"new"}`),
			cells:   every(cellCreated()),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var w workspaceAnswer
				decodeAnswer(t, answer, &w)
				if w != (workspaceAnswer{"new", "admin"}) {
					t.Errorf("created %+v, want new with the caller as its admin", w)
				}
			},
		},
		{
			op:      "createWorkspace",
			variant: "creation disabled",
			config:  func(cfg *config.Config) { cfg.Workspace.CreationEnabled = false },
			write:   true,
			request: sameRequest(http.MethodPost, "/api/v0/workspaces", `{"name":"New","slug":"new"}`),
			cells:   every(cell{http.StatusForbidden, "workspace.creation_disabled"}),
		},
		{
			op: "getWorkspace",
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c), ""
			},
			cells: map[caller]cell{
				callerAdmin: cellOK(), callerMember: cellOK(), callerGuest: cellOK(),
				callerNever: notFound, callerEnded: notFound, callerDeleted: notFound,
			},
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var w workspaceAnswer
				decodeAnswer(t, answer, &w)
				if w != (workspaceAnswer{"acme", string(c)}) {
					t.Errorf("got %+v, want acme with the role %s", w, c)
				}
			},
		},
		{
			op:      "checkWorkspaceSlug",
			request: sameRequest(http.MethodGet, "/api/v0/workspace-slugs/acme", ""),
			cells:   every(cellOK()),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				if answer != `{"available":false,"reason":"taken"}`+"\n" {
					t.Errorf("answered %s, want acme taken", answer)
				}
			},
		},
	}
}
