package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The page module's rows (M4/P1 design 3.12; M4/P2 design 3.6; M4/P4 design 3.9), by the notebook columns:
// each aims at its notebook (notebookOf) and its page in it (pageOf). Any
// role reads; the editors and admins write, the readers are refused; the
// rest do not see the notebook. The edit sessions' rows aim at the column's
// own session of its page, or at someone else's: a heartbeat is decided on
// the role the caller has now, an end only on whose the session is.

// treeNodeAnswer is a TreeNode answer, as much as the rows check.
type treeNodeAnswer struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

// pageAnswer is a Page answer, as much as the rows check.
type pageAnswer struct {
	Name      string `json:"name"`
	Revision  int    `json:"revision"`
	Ancestors []struct {
		Name string `json:"name"`
	} `json:"ancestors"`
}

// trashItemAnswer is a TrashItem answer, as much as the rows check.
type trashItemAnswer struct {
	Name      string `json:"name"`
	Pages     int    `json:"pages"`
	RestoreTo struct {
		Reason string `json:"reason"`
	} `json:"restore_to"`
}

// isRename reports whether the answer is the seeded rename id, of one
// node, its change a rename.
func (a changesetAnswer) isRename(id uuid.UUID) bool {
	return a.ID == id.String() && a.Kind == "edit" && a.NodeCount == 1 && len(a.Changes) == 1 && a.Changes[0].Kind == "renamed"
}

// renames reports whether the change renames its node from before to
// after.
func (a changeAnswer) renames(before, after string) bool {
	return a.Kind == "renamed" && a.Before != nil && a.Before.Name == before && a.After != nil && a.After.Name == after
}

// editSessionAnswer is an EditSession answer.
type editSessionAnswer struct {
	ID        string    `json:"id"`
	PageID    string    `json:"page_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// leased reports whether the session's lease runs a lease at most from
// now: from the call, not from the seeded session's hour.
func (e editSessionAnswer) leased() bool {
	left := time.Until(e.ExpiresAt)
	return left > 0 && left <= domain.EditSessionLease
}

// ancestorNames are a page answer's ancestors' names.
func (p pageAnswer) ancestorNames() []string {
	var out []string
	for _, a := range p.Ancestors {
		out = append(out, a.Name)
	}
	return out
}

func pageMatrixRows() []matrixRow {
	notebookNotFound := cell{http.StatusNotFound, "notebook.not_found"}
	pageNotFound := cell{http.StatusNotFound, "page.not_found"}
	changesetNotFound := cell{http.StatusNotFound, "page.changeset_not_found"}
	changesets := func(c caller, s seeded) string { return "/api/v0/changesets/" + s.changeset(notebookOf(c)).String() }
	nodes := func(c caller, s seeded) string { return "/api/v0/notebooks/" + s.notebook(notebookOf(c)).String() }
	readers := func(notFound cell) map[caller]cell {
		cells := map[caller]cell{}
		for _, c := range notebookColumns() {
			cells[c] = notFound
			if _, seen := roleIn(c); seen {
				cells[c] = cellOK()
			}
		}
		return cells
	}
	sessionNotFound := cell{http.StatusNotFound, "page.edit_session_not_found"}
	sessions := func(c caller, s seeded, owner caller) string {
		if owner == c {
			return "/api/v0/edit-sessions/" + s.session(draftOf(c), c).String()
		}
		return "/api/v0/edit-sessions/" + s.session(othersDraftIn(notebookOf(c)), owner).String()
	}
	everyColumn := func(answer cell) map[caller]cell {
		cells := map[caller]cell{}
		for _, c := range notebookColumns() {
			cells[c] = answer
		}
		return cells
	}
	return []matrixRow{
		{
			op:      "listNodes",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, nodes(c, s) + "/nodes", "" },
			cells:   readers(notebookNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var list struct{ Data []treeNodeAnswer }
				decodeAnswer(t, answer, &list)
				var names []string
				for _, n := range list.Data {
					names = append(names, n.Name)
				}
				if want := treeOf(notebookOf(c)); !slices.Equal(names, want) {
					t.Errorf("listed %v, want %v", names, want)
				}
			},
		},
		{
			op:      "getPage",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String(), ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var p pageAnswer
				decodeAnswer(t, answer, &p)
				if want := ancestorsOf(pageOf(c)); p.Name != pageOf(c) || !slices.Equal(p.ancestorNames(), want) {
					t.Errorf("read %+v, want %s under %v", p, pageOf(c), want)
				}
			},
		},
		{
			op:      "getPageView",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/view", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var v struct {
					HTML     string `json:"html"`
					Revision int    `json:"revision"`
				}
				decodeAnswer(t, answer, &v)
				if v.HTML != matrixMarkHTML || v.Revision != 1 {
					t.Errorf("read %+v, want the seeded content at revision 1", v)
				}
			},
		},
		{
			op:      "listPageVersions",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/versions", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var l struct {
					Data []struct {
						Revision  int `json:"revision"`
						Changeset struct {
							Kind string `json:"kind"`
						} `json:"changeset"`
					} `json:"data"`
				}
				decodeAnswer(t, answer, &l)
				if len(l.Data) != 1 || l.Data[0].Revision != 1 || l.Data[0].Changeset.Kind != "edit" {
					t.Errorf("listed %s, want the seeded creation's version 1", answer)
				}
			},
		},
		{
			op:      "getPageVersion",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/versions/1", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var v struct {
					Content  string `json:"content"`
					Revision int    `json:"revision"`
				}
				decodeAnswer(t, answer, &v)
				if v.Content != matrixMark+"\n" || v.Revision != 1 {
					t.Errorf("read %+v, want the seeded content at revision 1", v)
				}
			},
		},
		{
			op:      "getPageVersionView",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/versions/1/view", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var v struct {
					HTML     string `json:"html"`
					Revision int    `json:"revision"`
				}
				decodeAnswer(t, answer, &v)
				if v.HTML != matrixMarkHTML || v.Revision != 1 {
					t.Errorf("read %+v, want the seeded content at revision 1", v)
				}
			},
		},
		{
			// The seeded page's one version is its content: the restore is
			// decided and writes nothing.
			op:      "restorePageVersion",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/versions/1/restore", `{"base_revision":1}`
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var r struct {
					Revision    int     `json:"revision"`
					ChangesetID *string `json:"changeset_id"`
					Changed     bool    `json:"changed"`
				}
				decodeAnswer(t, answer, &r)
				if r.Revision != 1 || r.ChangesetID != nil || r.Changed {
					t.Errorf("restored %+v, want revision 1 unchanged", r)
				}
			},
		},
		{
			op:      "listTrash",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, nodes(c, s) + "/trash", "" },
			cells:   readers(notebookNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var l struct{ Data []trashItemAnswer }
				decodeAnswer(t, answer, &l)
				if len(l.Data) != 1 || l.Data[0].Name != trashedIn(notebookOf(c)) {
					t.Errorf("listed %s, want the seeded item %s", answer, trashedIn(notebookOf(c)))
				}
			},
		},
		{
			op:      "getTrashItem",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/nodes/" + s.page(trashedIn(notebookOf(c))).String() + "/trash-item", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var it trashItemAnswer
				decodeAnswer(t, answer, &it)
				if it.Name != trashedIn(notebookOf(c)) || it.Pages != 1 || it.RestoreTo.Reason != "original" {
					t.Errorf("read %s, want the seeded item, back at its root", answer)
				}
			},
		},
		{
			op:      "restoreNode",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/nodes/" + s.page(trashedIn(notebookOf(c))).String() + "/restore", ""
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var r struct {
					Name    string `json:"name"`
					Renamed bool   `json:"renamed"`
					Placed  struct {
						Reason string `json:"reason"`
					} `json:"placed"`
				}
				decodeAnswer(t, answer, &r)
				if r.Name != trashedIn(notebookOf(c)) || r.Renamed || r.Placed.Reason != "original" {
					t.Errorf("restored %s, want the seeded item back at its root as it was", answer)
				}
			},
		},
		{
			op:      "listChangesets",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, nodes(c, s) + "/changesets", ""
			},
			cells: readers(notebookNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var l struct{ Data []changesetAnswer }
				decodeAnswer(t, answer, &l)
				if len(l.Data) == 0 || !l.Data[0].isRename(s.changeset(notebookOf(c))) {
					t.Errorf("listed %s, want the seeded rename first", answer)
				}
			},
		},
		{
			op:      "getChangeset",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) { return http.MethodGet, changesets(c, s), "" },
			cells:   readers(changesetNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var d changesetAnswer
				decodeAnswer(t, answer, &d)
				if !d.isRename(s.changeset(notebookOf(c))) || !d.Renames || d.Moves {
					t.Errorf("read %s, want the seeded rename, renaming and moving nothing", answer)
				}
			},
		},
		{
			op:      "listChangesetChanges",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, changesets(c, s) + "/changes", ""
			},
			cells: readers(changesetNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var l struct{ Data []changeAnswer }
				decodeAnswer(t, answer, &l)
				if len(l.Data) != 1 || !l.Data[0].renames(wasIn(notebookOf(c)), renamedIn(notebookOf(c))) {
					t.Errorf("listed %s, want the seeded rename", answer)
				}
			},
		},
		{
			op:      "revertChangeset",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, changesets(c, s) + "/revert", `{"message":"Back"}`
			},
			cells: editorsOnly(cellOK(), changesetNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var r struct {
					ChangesetID *string `json:"changeset_id"`
					Changed     bool    `json:"changed"`
				}
				decodeAnswer(t, answer, &r)
				if r.ChangesetID == nil || !r.Changed {
					t.Errorf("reverted %s, want the rename put back in a changeset", answer)
				}
			},
			after: func(t *testing.T, c caller, s seeded, get func(string) (int, string)) {
				t.Helper()
				nb := notebookOf(c)
				status, answer := get("/api/v0/notebooks/" + s.notebook(nb).String() + "/nodes")
				type node struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				var l struct {
					Data []node `json:"data"`
				}
				decodeAnswer(t, answer, &l)
				page := s.page(renamedIn(nb)).String()
				if i := slices.IndexFunc(l.Data, func(n node) bool { return n.ID == page }); status != http.StatusOK || i < 0 ||
					l.Data[i].Name != wasIn(nb) {
					t.Errorf("the tree after the revert = %d %s, want the page named %s again", status, answer, wasIn(nb))
				}
			},
		},
		{
			op:      "getPageContent",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/content", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var c struct {
					Content  string `json:"content"`
					Revision int    `json:"revision"`
					Hash     string `json:"content_hash"`
				}
				decodeAnswer(t, answer, &c)
				if sum := sha256.Sum256([]byte(matrixMark + "\n")); c.Content != matrixMark+"\n" || c.Revision != 1 ||
					c.Hash != hex.EncodeToString(sum[:]) {
					t.Errorf("read %+v, want the seeded content at revision 1, with its hash", c)
				}
			},
		},
		{
			op:      "putPageContent",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPut, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/content", `{"content":"# Written","base_revision":1}`
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var p pageAnswer
				decodeAnswer(t, answer, &p)
				if p.Name != pageOf(c) || p.Revision != 2 {
					t.Errorf("wrote %+v, want %s at revision 2", p, pageOf(c))
				}
			},
		},
		{
			op:      "openEditSession",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/pages/" + s.page(pageOf(c)).String() + "/edit-sessions", ""
			},
			cells: editorsOnly(cellCreated(), pageNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var e editSessionAnswer
				decodeAnswer(t, answer, &e)
				if e.PageID != s.page(pageOf(c)).String() || !e.leased() {
					t.Errorf("opened %+v, want a new session of %s, its lease from now", e, pageOf(c))
				}
			},
		},
		{
			op:      "heartbeatEditSession",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, sessions(c, s, c) + "/heartbeat", ""
			},
			// gone-nb-page's session was deleted with its notebook.
			cells: editorsOnly(cellOK(), sessionNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var e editSessionAnswer
				decodeAnswer(t, answer, &e)
				if e.ID != s.session(draftOf(c), c).String() || e.PageID != s.page(draftOf(c)).String() || !e.leased() {
					t.Errorf("kept %+v, want its session of %s, its lease from now", e, draftOf(c))
				}
			},
		},
		{
			op:      "heartbeatEditSession",
			variant: "someone else's",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, sessions(c, s, someoneElse) + "/heartbeat", ""
			},
			cells: everyColumn(sessionNotFound),
		},
		{
			op:      "endEditSession",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, sessions(c, s, c), ""
			},
			cells: func() map[caller]cell {
				cells := everyColumn(cell{status: http.StatusNoContent})
				cells[callerNotebookDeleted] = sessionNotFound
				return cells
			}(),
		},
		{
			op:      "endEditSession",
			variant: "someone else's",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, sessions(c, s, someoneElse), ""
			},
			cells: everyColumn(sessionNotFound),
		},
		{
			op:      "getEditLock",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, "/api/v0/pages/" + s.page(draftOf(c)).String() + "/edit-lock", ""
			},
			cells: readers(pageNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var l struct {
					Holder *struct {
						UserID string `json:"user_id"`
					} `json:"holder"`
					ExpiresIn *int `json:"expires_in"`
				}
				decodeAnswer(t, answer, &l)
				if l.Holder == nil || l.Holder.UserID != s.accounts[c].String() || l.ExpiresIn == nil || *l.ExpiresIn <= 0 {
					t.Errorf("read %s, want the lock of its own session", answer)
				}
			},
		},
		{
			op:      "releaseEditLock",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/pages/" + s.page(draftOf(c)).String() + "/edit-lock", ""
			},
			cells: adminsOnly(cell{status: http.StatusNoContent}, pageNotFound),
		},
		{
			op:      "toggleTask",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/pages/" + s.page(tasksIn(notebookOf(c))).String() + "/toggle-task",
					`{"base_revision":1,"offset":3,"checked":true}`
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var p pageAnswer
				decodeAnswer(t, answer, &p)
				if p.Name != tasksIn(notebookOf(c)) || p.Revision != 2 {
					t.Errorf("toggled %+v, want %s at revision 2", p, tasksIn(notebookOf(c)))
				}
			},
		},
		{
			op:      "createPage",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, nodes(c, s) + "/pages", `{"parent_id":"` + s.page(pageOf(c)).String() + `","title":"New"}`
			},
			cells: editorsOnly(cellCreated(), notebookNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var p pageAnswer
				decodeAnswer(t, answer, &p)
				want := append(ancestorsOf(pageOf(c)), pageOf(c))
				if p.Name != "New" || p.Revision != 1 || !slices.Equal(p.ancestorNames(), want) {
					t.Errorf("created %+v, want New at revision 1 under %v", p, want)
				}
			},
		},
		{
			op:      "renameNode",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, "/api/v0/nodes/" + s.page(pageOf(c)).String(), `{"name":"Renamed"}`
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var n treeNodeAnswer
				decodeAnswer(t, answer, &n)
				if n.Name != "Renamed" {
					t.Errorf("renamed %+v, want Renamed", n)
				}
			},
		},
		{
			op:      "moveNode",
			columns: notebookColumns(),
			write:   true,
			// First at the root: priv's child changes its parent; a root
			// page already first stays where it is, decided on all the same.
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/nodes/" + s.page(pageOf(c)).String() + "/move", `{"parent_id":null,"after_id":null}`
			},
			cells: editorsOnly(cellOK(), pageNotFound),
			check: func(t *testing.T, c caller, _ seeded, answer string) {
				t.Helper()
				var n treeNodeAnswer
				decodeAnswer(t, answer, &n)
				if n.Name != pageOf(c) || n.ParentID != nil {
					t.Errorf("moved %+v, want %s at the root", n, pageOf(c))
				}
			},
		},
		{
			op:      "deleteNode",
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, "/api/v0/nodes/" + s.page(pageOf(c)).String(), ""
			},
			cells: editorsOnly(cell{status: http.StatusNoContent}, pageNotFound),
		},
	}
}

// editorsOnly answers a notebook's editors and admins, refuses its readers,
// and does not show it to the rest.
func editorsOnly(answer, notFound cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range notebookColumns() {
		switch role, seen := roleIn(c); {
		case role == "admin" || role == "editor":
			cells[c] = answer
		case seen:
			cells[c] = cellForbidden()
		default:
			cells[c] = notFound
		}
	}
	return cells
}

// adminsOnly answers a notebook's admins, refuses its other roles, and
// does not show it to the rest.
func adminsOnly(answer, notFound cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range notebookColumns() {
		switch role, seen := roleIn(c); {
		case role == "admin":
			cells[c] = answer
		case seen:
			cells[c] = cellForbidden()
		default:
			cells[c] = notFound
		}
	}
	return cells
}

// pageOf is the page a notebook column's cells target, in its notebook:
// priv's is a child, to show its ancestor.
func pageOf(c caller) string {
	if nb := notebookOf(c); nb != "priv" {
		return nb + "-page"
	}
	return "priv-child"
}

// pagesOf is a notebook's seeded pages, each parent before its children.
func pagesOf(notebook string) []string {
	var out []string
	for _, p := range matrixPages() {
		if p.notebook == notebook {
			out = append(out, p.name)
		}
	}
	return out
}

// treeOf is a notebook's seeded nodes, each parent before its children: a
// page, then its attachments, last among its children, before the pages
// after it, which are none of its children.
func treeOf(notebook string) []string {
	var out []string
	for _, p := range matrixPages() {
		if p.notebook == notebook {
			out = append(out, p.name)
			for _, a := range matrixAssets() {
				if a.parent == p.name {
					out = append(out, a.name)
				}
			}
		}
	}
	return out
}

// ancestorsOf is a seeded page's ancestors, from the root.
func ancestorsOf(name string) []string {
	var out []string
	for parent := name; ; {
		i := slices.IndexFunc(matrixPages(), func(p matrixPage) bool { return p.name == parent })
		if parent = matrixPages()[i].parent; parent == "" {
			break
		}
		out = append([]string{parent}, out...)
	}
	return out
}

// matrixMarkHTML is the reading view of a seeded page's content.
const matrixMarkHTML = "<p>" + matrixMark + "</p>\n"

// searchMatrixRows are the search's rows (M8/P5 design 3.8): any member
// searches a workspace, the rest do not see it. The workspace columns
// search acme, which has no notebook; the notebook columns lab, every
// notebook each sees, then the one it targets (notebookOf), which a column
// that does not see it is told is not found. Each finds by the mark
// (matrixMark) exactly the seeded nodes not deleted of what it searches,
// through the queue (queueSeeded): the reads' app runs no job.
func searchMatrixRows() []matrixRow {
	workspaceNotFound := cell{http.StatusNotFound, "workspace.not_found"}
	// finds checks that the answer holds the nodes of the notebooks, one
	// page of them, counted exactly.
	finds := func(t *testing.T, s seeded, answer string, notebooks ...string) {
		t.Helper()
		var a searchAnswer
		decodeAnswer(t, answer, &a)
		got, want := slices.Sorted(slices.Values(a.ids())), markedNodes(s, notebooks...)
		if !slices.Equal(got, want) || a.NextCursor != nil || a.Building || a.Count == nil || a.Count.Value != len(want) ||
			a.Count.Kind != "exact" {
			t.Errorf("found %s, want the seeded nodes of %q, %d of them, exactly", answer, notebooks, len(want))
		}
	}
	return []matrixRow{
		{
			op: "searchWorkspace",
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, searchPath(workspaceOf(c), matrixMark, nil), ""
			},
			cells: map[caller]cell{
				callerAdmin: cellOK(), callerMember: cellOK(), callerGuest: cellOK(),
				callerNever: workspaceNotFound, callerEnded: workspaceNotFound, callerDeleted: workspaceNotFound,
			},
			check: func(t *testing.T, _ caller, s seeded, answer string) {
				t.Helper()
				finds(t, s, answer)
			},
		},
		{
			op:      "searchWorkspace",
			variant: "by the notebook columns",
			columns: notebookColumns(),
			request: func(c caller, _ seeded) (string, string, string) {
				return http.MethodGet, searchPath(workspaceOf(c), matrixMark, url.Values{"limit": {"50"}}), ""
			},
			cells: everyNotebookColumn(cellOK(), map[caller]cell{callerOutsideWorkspace: workspaceNotFound}),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var seen []string
				for _, n := range listedNotebooks(c) {
					seen = append(seen, n.Name)
				}
				finds(t, s, answer, seen...)
			},
		},
		{
			op:      "searchWorkspace",
			variant: "in the column's notebook",
			columns: notebookColumns(),
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodGet, searchPath(workspaceOf(c), matrixMark,
					url.Values{"notebook_id": {s.notebook(notebookOf(c)).String()}, "limit": {"50"}}), ""
			},
			cells: func() map[caller]cell {
				cells := map[caller]cell{}
				for _, c := range notebookColumns() {
					_, seen := roleIn(c)
					switch {
					case c == callerOutsideWorkspace:
						cells[c] = workspaceNotFound
					case seen:
						cells[c] = cellOK()
					default:
						cells[c] = cell{http.StatusNotFound, "notebook.not_found"}
					}
				}
				return cells
			}(),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				finds(t, s, answer, notebookOf(c))
			},
		},
	}
}

// markedNodes are the ids of the seeded nodes not deleted of the notebooks
// that the search finds by the mark, sorted: each page, and the attachment
// markedIn; a deleted notebook's are deleted with it.
func markedNodes(s seeded, notebooks ...string) []string {
	var out []string
	for _, n := range matrixNotebooks() {
		if !slices.Contains(notebooks, n.name) || n.deleted {
			continue
		}
		for _, p := range matrixPages() {
			if p.notebook == n.name {
				out = append(out, s.page(p.name).String())
			}
		}
		out = append(out, s.asset(markedIn(n.name)).String())
	}
	slices.Sort(out)
	return out
}
