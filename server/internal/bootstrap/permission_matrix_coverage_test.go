package bootstrap

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// matrixViolations are the gaps between the matrix and the contract's
// operations: an operation without a row and not exempt; a row without a
// cell for a column, or whose request is not its operation's, or aims at
// another workspace than its column's; a row that sends a write without
// write, whose cells would share the reads' copy; an exemption that names
// nothing, or an operation of another kind, or one with a row.
func matrixViolations(ops []apitest.Operation, exempt matrixExemptions, rows []matrixRow, s seeded) []string {
	var found []string
	byID, inMatrix := map[string]apitest.Operation{}, map[string]bool{}
	for _, op := range ops {
		byID[op.ID] = op
	}
	for _, r := range rows {
		inMatrix[r.op] = true
		op, named := byID[r.op]
		if !named {
			found = append(found, fmt.Sprintf("row %s names no operation of the contract", r.name()))
		}
		unsafe := ""
		for _, c := range r.callers() {
			if _, ok := r.cells[c]; !ok {
				found = append(found, fmt.Sprintf("row %s has no cell for %s", r.name(), c))
			}
			method, path, _ := r.request(c, s)
			switch {
			case !named:
			case method != op.Method || !pathOf(op.Path, path):
				found = append(found, fmt.Sprintf("row %s, %s: %s %s is not %s", r.name(), c, method, path, op.Pattern()))
			default:
				if _, listed := exempt.notTargets[op.Path]; listed {
					break
				}
				if v := targetViolation(op.Path, path, c, s); v != "" {
					found = append(found, fmt.Sprintf("row %s, %s: %s", r.name(), c, v))
				}
			}
			if method != http.MethodGet && !r.write && unsafe == "" {
				unsafe = method
			}
		}
		if unsafe != "" {
			found = append(found, fmt.Sprintf("row %s sends %s without write: its cells would run on the reads' copy", r.name(), unsafe))
		}
	}
	for _, op := range ops {
		exempted := len(op.Tags) > 0 && !slices.ContainsFunc(op.Tags, func(tag string) bool { return !slices.Contains(exempt.modules, tag) })
		_, public := exempt.public[op.ID]
		if _, held := exempt.byCredential[op.ID]; !exempted && !public && !held && !inMatrix[op.ID] {
			found = append(found, fmt.Sprintf("operation %s, tagged %v, has no row", op.ID, op.Tags))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(exempt.public)) {
		op, named := byID[id]
		switch {
		case !named:
			found = append(found, fmt.Sprintf("the public exemption %s names no operation of the contract", id))
		case !op.Public:
			found = append(found, fmt.Sprintf("operation %s is exempt as public, but needs a token", id))
		case inMatrix[id]:
			found = append(found, fmt.Sprintf("operation %s is exempt as public, and has a row", id))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(exempt.byCredential)) {
		op, named := byID[id]
		switch {
		case !named:
			found = append(found, fmt.Sprintf("the by-credential exemption %s names no operation of the contract", id))
		case op.Public:
			found = append(found, fmt.Sprintf("operation %s is exempt by credential, but is public: exempt it as public", id))
		case inMatrix[id]:
			found = append(found, fmt.Sprintf("operation %s is exempt by credential, and has a row", id))
		}
	}
	for _, path := range slices.Sorted(maps.Keys(exempt.notTargets)) {
		if !slices.ContainsFunc(ops, func(op apitest.Operation) bool { return op.Path == path }) {
			found = append(found, fmt.Sprintf("the not-target path %s is no operation's", path))
		}
	}
	for _, module := range exempt.modules {
		if !slices.ContainsFunc(ops, func(op apitest.Operation) bool { return slices.Contains(op.Tags, module) }) {
			found = append(found, fmt.Sprintf("the exempt module %s has no operation", module))
		}
	}
	return found
}

// pathOf reports whether path, without its query, is an instance of the
// pattern.
func pathOf(pattern, path string) bool {
	path, _, _ = strings.Cut(path, "?")
	want, got := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(want) != len(got) {
		return false
	}
	for i, segment := range want {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			if got[i] == "" {
				return false
			}
		} else if segment != got[i] {
			return false
		}
	}
	return true
}

// targetViolation tells how path, an instance of pattern, aims elsewhere
// than column c's workspace: a {slug} after workspaces must be its slug, an
// {…_id} a row seeded under it. Any other parameter is unknown: its path is
// listed as not a target, with its reason, or the matrix learns it.
func targetViolation(pattern, path string, c caller, s seeded) string {
	path, _, _ = strings.Cut(path, "?")
	want, got := strings.Split(pattern, "/"), strings.Split(path, "/")
	for i, segment := range want {
		switch {
		case !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}"):
		case segment == "{slug}" && i > 0 && want[i-1] == "workspaces":
			if got[i] != workspaceOf(c) {
				return fmt.Sprintf("targets the workspace %s, not its column's %s", got[i], workspaceOf(c))
			}
		case segment == "{tag}" && i > 0 && want[i-1] == "tags":
			// A tag is no target: the notebook before it is.
		case strings.HasSuffix(segment, "_id}"):
			id, err := uuid.Parse(got[i])
			slug, isRow := s.workspaceOfRow(id)
			if err != nil || !isRow || slug != workspaceOf(c) {
				return fmt.Sprintf("%s %s is no row seeded under its column's workspace %s", segment, got[i], workspaceOf(c))
			}
		default:
			return fmt.Sprintf("%s is no target the matrix knows: list %s as not a target, with its reason", segment, pattern)
		}
	}
	return ""
}

func TestThePermissionMatrixCoversEveryOperation(t *testing.T) {
	for _, v := range matrixViolations(apitest.Load(t).Operations(), matrixExempt(), matrixRows(), newSeeded().in(t)) {
		t.Error(v)
	}
}

// Each kind of gap is reported: a synthetic contract and rows, one gap at a
// time.
func TestMatrixViolationsCatchesEachGap(t *testing.T) {
	get := apitest.Operation{ID: "getWorkspace", Tags: []string{"workspace"}, Method: http.MethodGet, Path: "/api/v0/workspaces/{slug}"}
	me := apitest.Operation{ID: "getMe", Tags: []string{"identity"}, Method: http.MethodGet, Path: "/api/v0/me"}
	preview := apitest.Operation{ID: "previewInvitation", Tags: []string{"workspace"}, Method: http.MethodPost,
		Path: "/api/v0/invitations/{invitation_id}", Public: true}
	slugs := apitest.Operation{ID: "checkWorkspaceSlug", Tags: []string{"workspace"}, Method: http.MethodGet, Path: "/api/v0/workspace-slugs/{slug}"}
	member := apitest.Operation{ID: "removeMember", Tags: []string{"workspace"}, Method: http.MethodDelete,
		Path: "/api/v0/workspace-members/{workspace_member_id}"}
	accept := apitest.Operation{ID: "acceptInvitation", Tags: []string{"workspace"}, Method: http.MethodPost,
		Path: "/api/v0/invitations/{invitation_id}/accept"}
	exempt := matrixExemptions{modules: []string{"identity"}, public: map[string]string{"previewInvitation": "the token decides"},
		byCredential: map[string]string{"acceptInvitation": "the token and the address decide"},
		notTargets:   map[string]string{"/api/v0/workspace-slugs/{slug}": "a slug"}}
	s := newSeeded().in(t)
	getRow := matrixRow{op: "getWorkspace", cells: every(cellOK()), request: func(c caller, _ seeded) (string, string, string) {
		return http.MethodGet, "/api/v0/workspaces/" + workspaceOf(c), ""
	}}
	slugRow := matrixRow{op: "checkWorkspaceSlug", cells: every(cellOK()), request: sameRequest(http.MethodGet, "/api/v0/workspace-slugs/acme", "")}
	removeRow := matrixRow{op: "removeMember", write: true, cells: every(cellForbidden()), request: func(c caller, s seeded) (string, string, string) {
		return http.MethodDelete, "/api/v0/workspace-members/" + s.adminMembership(workspaceOf(c)).String(), ""
	}}
	ops := []apitest.Operation{get, me, preview, slugs, member, accept}
	rows := []matrixRow{getRow, slugRow, removeRow}
	if found := matrixViolations(ops, exempt, rows, s); len(found) != 0 {
		t.Fatalf("a complete matrix: %q", found)
	}
	without := func(r matrixRow, change func(*matrixRow)) []matrixRow {
		changed := r
		change(&changed)
		return slices.Concat([]matrixRow{changed}, slices.DeleteFunc(slices.Clone(rows), func(x matrixRow) bool { return x.op == r.op }))
	}
	exemptWith := func(change func(*matrixExemptions)) matrixExemptions {
		e := matrixExemptions{modules: slices.Clone(exempt.modules), public: maps.Clone(exempt.public),
			byCredential: maps.Clone(exempt.byCredential), notTargets: maps.Clone(exempt.notTargets)}
		change(&e)
		return e
	}
	tests := []struct {
		name   string
		ops    []apitest.Operation
		exempt matrixExemptions
		rows   []matrixRow
		want   string
	}{
		{"an operation without a row", ops, exempt, rows[1:], "operation getWorkspace, tagged [workspace], has no row"},
		{"a row without a cell", ops, exempt, without(getRow, func(r *matrixRow) { delete(r.cells, callerGuest) }), "row getWorkspace has no cell for guest"},
		{"a row of the notebook columns without a cell", ops, exempt, without(getRow, func(r *matrixRow) {
			r.columns, r.cells = notebookColumns(), everyNotebookColumn(cellOK(), nil)
			delete(r.cells, callerOutsideGuest)
		}), "row getWorkspace has no cell for guest outside"},
		{"a row of the notebook columns with the workspace's cells", ops, exempt, without(getRow, func(r *matrixRow) {
			r.columns = notebookColumns()
		}), "row getWorkspace has no cell for notebook admin"},
		{"a request of another operation", ops, exempt,
			without(getRow, func(r *matrixRow) { r.request = sameRequest(http.MethodGet, "/api/v0/workspaces", "") }), "is not GET /api/v0/workspaces/{slug}"},
		{"another workspace", ops, exempt,
			without(getRow, func(r *matrixRow) { r.request = sameRequest(http.MethodGet, "/api/v0/workspaces/acme", "") }),
			"targets the workspace acme, not its column's gone"},
		{"a row of another workspace", ops, exempt, without(removeRow, func(r *matrixRow) {
			r.request = func(_ caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/workspace-members/" + s.adminMembership("other").String(), ""
			}
		}), "is no row seeded under its column's workspace"},
		{"an id that is no uuid", ops, exempt, without(removeRow, func(r *matrixRow) {
			r.request = sameRequest(http.MethodDelete, "/api/v0/workspace-members/not-a-uuid", "")
		}), "not-a-uuid is no row seeded under its column's workspace"},
		{"an id of no seeded row", ops, exempt, without(removeRow, func(r *matrixRow) {
			r.request = sameRequest(http.MethodDelete, "/api/v0/workspace-members/"+uuid.NewV7().String(), "")
		}), "is no row seeded under its column's workspace"},
		{"a write without write", ops, exempt, without(removeRow, func(r *matrixRow) { r.write = false }), "sends DELETE without write"},
		{"an unknown parameter", ops, exemptWith(func(e *matrixExemptions) { delete(e.notTargets, slugs.Path) }), rows,
			"{slug} is no target the matrix knows"},
		{"a row of no operation", ops, exempt, append(slices.Clone(rows), matrixRow{op: "nothing", cells: every(cellOK()),
			request: sameRequest(http.MethodGet, "/", "")}), "row nothing names no operation of the contract"},
		{"a public exemption of no operation", ops, exemptWith(func(e *matrixExemptions) { e.public["nothing"] = "?" }), rows,
			"the public exemption nothing names no operation of the contract"},
		{"a public exemption that needs a token", ops, exemptWith(func(e *matrixExemptions) { e.public["getWorkspace"] = "?" }),
			rows, "operation getWorkspace is exempt as public, but needs a token"},
		{"a public exemption that has a row", ops, exempt, append(slices.Clone(rows), matrixRow{op: "previewInvitation", write: true,
			cells: every(cellOK()), request: sameRequest(http.MethodPost, "/api/v0/invitations/x", "")}),
			"operation previewInvitation is exempt as public, and has a row"},
		{"an operation held by credential, not exempt", ops, exemptWith(func(e *matrixExemptions) { delete(e.byCredential, "acceptInvitation") }),
			rows, "operation acceptInvitation, tagged [workspace], has no row"},
		{"a by-credential exemption of no operation", ops, exemptWith(func(e *matrixExemptions) { e.byCredential["nothing"] = "?" }), rows,
			"the by-credential exemption nothing names no operation of the contract"},
		{"a by-credential exemption that is public", ops, exemptWith(func(e *matrixExemptions) { e.byCredential["previewInvitation"] = "?" }),
			rows, "operation previewInvitation is exempt by credential, but is public"},
		{"a by-credential exemption that has a row", ops, exempt, append(slices.Clone(rows), matrixRow{op: "acceptInvitation", write: true,
			cells: every(cellOK()), request: sameRequest(http.MethodPost, "/api/v0/invitations/x/accept", "")}),
			"operation acceptInvitation is exempt by credential, and has a row"},
		{"a not-target path of no operation", ops, exemptWith(func(e *matrixExemptions) { e.notTargets["/api/v0/nothing"] = "?" }),
			rows, "the not-target path /api/v0/nothing is no operation's"},
		{"an exempt module of no operation", ops, exemptWith(func(e *matrixExemptions) { e.modules = append(e.modules, "nothing") }),
			rows, "the exempt module nothing has no operation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := matrixViolations(tt.ops, tt.exempt, tt.rows, s)
			if !slices.ContainsFunc(found, func(v string) bool { return strings.Contains(v, tt.want) }) {
				t.Errorf("violations = %q, want one with %q", found, tt.want)
			}
		})
	}
}
