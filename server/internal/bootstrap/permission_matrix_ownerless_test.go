package bootstrap

import (
	"net/http"
	"testing"
)

// The ownerless notebooks' rows (M3/P3 design 3.7), by the notebook
// columns, in lab, whose admins are the workspace admin outside priv and
// the default reader: they alone list the ownerless notebooks and the
// audit events, which lab's members and guests see the workspace to be
// refused; and they alone take over or delete, by id, an ownerless
// notebook: every other answer is notebook.not_found, the same for a
// notebook that is not ownerless or is gone.

// profileAnswer is an AccountProfile answer.
type profileAnswer struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// profileOf is c's profile as the workspace's admins see it.
func profileOf(c caller, s seeded) profileAnswer {
	l := listed(c, "", true)
	return profileAnswer{UserID: s.accounts[c].String(), DisplayName: l.DisplayName, Email: *l.Email}
}

// labAdminsOnly answers lab's admins, refuses its members and guests, and
// does not show lab to the column outside it.
func labAdminsOnly(answer cell) map[caller]cell {
	return everyNotebookColumn(cellForbidden(), map[caller]cell{
		callerOutsideAdmin: answer, callerDefaultReader: answer, callerOutsideWorkspace: {http.StatusNotFound, "workspace.not_found"},
	})
}

func ownerlessMatrixRows() []matrixRow {
	notFound := cell{http.StatusNotFound, "notebook.not_found"}
	orphan, formerOwner := matrixOwnerless()
	rows := []matrixRow{
		{
			op:      "listOwnerlessNotebooks",
			columns: notebookColumns(),
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/ownerless-notebooks", ""
			},
			cells: labAdminsOnly(cellOK()),
			// orphan alone, private as it is, with its editor left and its
			// former owner; no source of activity in M3: 0 bytes.
			check: func(t *testing.T, _ caller, s seeded, answer string) {
				t.Helper()
				var list struct {
					Data []struct {
						ID              string        `json:"id"`
						Name            string        `json:"name"`
						WorkspaceAccess string        `json:"workspace_access"`
						MemberCount     int           `json:"member_count"`
						FormerOwner     profileAnswer `json:"former_owner"`
						SizeBytes       int64         `json:"size_bytes"`
					}
				}
				decodeAnswer(t, answer, &list)
				if len(list.Data) != 1 {
					t.Fatalf("listed %+v, want orphan alone", list.Data)
				}
				o := list.Data[0]
				if o.ID != s.notebook(orphan).String() || o.Name != orphan || o.WorkspaceAccess != "none" || o.MemberCount != 1 ||
					o.FormerOwner != profileOf(formerOwner, s) || o.SizeBytes != 0 {
					t.Errorf("listed %+v, want orphan, private, its editor left, ownerless of %s, 0 bytes", o, formerOwner)
				}
			},
		},
		{
			op:      "listNotebookAuditEvents",
			columns: notebookColumns(),
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c) + "/notebook-audit-events", ""
			},
			cells: labAdminsOnly(cellOK()),
			check: func(t *testing.T, _ caller, s seeded, answer string) {
				t.Helper()
				var page struct {
					Data []struct {
						Action       string        `json:"action"`
						NotebookName string        `json:"notebook_name"`
						FormerOwner  profileAnswer `json:"former_owner"`
						Actor        profileAnswer `json:"actor"`
					}
					NextCursor *string `json:"next_cursor"`
				}
				decodeAnswer(t, answer, &page)
				gone, goneOwner, actor := matrixAuditEvent()
				if len(page.Data) != 1 || page.NextCursor != nil {
					t.Fatalf("listed %+v, want the seeded event alone, no next page", page)
				}
				if e := page.Data[0]; e.Action != "deleted" || e.NotebookName != gone || e.FormerOwner != profileOf(goneOwner, s) ||
					e.Actor != profileOf(actor, s) {
					t.Errorf("listed %+v, want %s deleted by %s, ownerless of %s", e, gone, actor, goneOwner)
				}
			},
		},
	}
	// By id, three targets: orphan; priv, private and owned; gone-nb,
	// deleted, as a notebook that never was answers.
	for _, target := range []struct {
		variant, notebook string
		ownerless         bool
	}{
		{"", orphan, true},
		{"a notebook not ownerless", "priv", false},
		{"a deleted notebook", "gone-nb", false},
	} {
		byID := func(answer cell) map[caller]cell {
			if !target.ownerless {
				return everyNotebookColumn(notFound, nil)
			}
			return everyNotebookColumn(notFound, map[caller]cell{callerOutsideAdmin: answer, callerDefaultReader: answer})
		}
		path := func(s seeded) string { return "/api/v0/ownerless-notebooks/" + s.notebook(target.notebook).String() }
		rows = append(rows,
			matrixRow{
				op:      "takeOverNotebook",
				variant: target.variant,
				columns: notebookColumns(),
				write:   true,
				request: func(_ caller, s seeded) (string, string, string) { return http.MethodPost, path(s) + "/take-over", "" },
				cells:   byID(cellOK()),
				check: func(t *testing.T, _ caller, _ seeded, answer string) {
					t.Helper()
					var n notebookAnswer
					decodeAnswer(t, answer, &n)
					if n != (notebookAnswer{orphan, "admin", 2}) {
						t.Errorf("took over %+v, want orphan, the caller its admin beside its editor", n)
					}
				},
			},
			matrixRow{
				op:      "deleteOwnerlessNotebook",
				variant: target.variant,
				columns: notebookColumns(),
				write:   true,
				request: func(_ caller, s seeded) (string, string, string) { return http.MethodDelete, path(s), "" },
				cells:   byID(cell{status: http.StatusNoContent}),
			})
	}
	return rows
}
