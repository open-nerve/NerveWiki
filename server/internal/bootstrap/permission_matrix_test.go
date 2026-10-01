package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The permission matrix (v0.1 design 10, 13.1 item 3; M2/P1 design 3.10):
// each operation of the contract but the exempt ones, called over HTTP on
// the wired app and a real database by each kind of caller, its status and
// problem code asserted cell by cell, and where a row says so, what the
// answer holds. The data is prepared once (prepareMatrix). The cells that
// only read share one copy of it, and each cell that writes gets a copy of
// its own (pgtest.NewDatabaseFrom), so no cell sees another's writes. Each
// module's rows are in a file of their own (permission_matrix_<module>_test.go):
// a phase that adds an operation adds its row there, and what the row needs
// prepared in permission_matrix_seeded_test.go.

// matrixExempt is what has no row, each entry for its reason.
func matrixExempt() matrixExemptions {
	return matrixExemptions{
		modules: []string{
			// Account-level: each operation acts on the caller's own account,
			// sessions or tokens, and no workspace role decides it.
			"identity",
			// Public: it describes this instance to anyone.
			"instance",
		},
		public: map[string]string{
			"previewWorkspaceInvitation": "the link's token decides, for anyone holding it: no column's role does",
		},
		byCredential: map[string]string{
			"acceptWorkspaceInvitation": "the caller is no member yet: the link's token and the caller's address decide (M2 design 9)",
		},
		notTargets: map[string]string{
			"/api/v0/workspace-slugs/{slug}": "a slug asked about, not a workspace: every caller gets the same answer",
		},
	}
}

// matrixExemptions are the operations without a row. modules exempts every
// operation of a module: every other operation has a row, so a module that
// adds operations is in the matrix unless it is listed, and an entry no
// operation carries is reported, so a misspelled one fails. public exempts
// one public operation of a module the matrix covers, by its operationId,
// with its reason: its route runs no authentication, so the columns could
// not be told apart. byCredential exempts an operation that needs a token
// but is decided by what the caller holds, not by a role in the workspace,
// with its reason. notTargets are the paths whose parameters name nothing
// a column's cell must aim at its workspace, each with its reason.
type matrixExemptions struct {
	modules      []string
	public       map[string]string // operationId → why it has no row
	byCredential map[string]string // operationId → what decides it instead of a role
	notTargets   map[string]string // path → why its parameters are no column's target
}

// caller is a column: an account, and how it stands to the workspace a row
// targets.
type caller string

const (
	callerAdmin   caller = "admin"
	callerMember  caller = "member"
	callerGuest   caller = "guest"
	callerNever   caller = "never a member"
	callerEnded   caller = "membership ended"
	callerDeleted caller = "workspace deleted"
)

// workspaceColumns are the columns of the workspace level.
func workspaceColumns() []caller {
	return []caller{callerAdmin, callerMember, callerGuest, callerNever, callerEnded, callerDeleted}
}

// The columns of the notebook level (M3/P1 design 3.11): each an account
// and how it stands to the notebook its cells target (notebookOf), in lab.
const (
	callerNotebookAdmin     caller = "notebook admin"
	callerNotebookEditor    caller = "notebook editor"
	callerNotebookReader    caller = "notebook reader guest"
	callerOutsideAdmin      caller = "workspace admin outside"
	callerOutsideMember     caller = "workspace member outside"
	callerDefaultEditor     caller = "default editor"
	callerDefaultReader     caller = "default reader admin"
	callerOutsideGuest      caller = "guest outside"
	callerNotebookEnded     caller = "notebook membership ended"
	callerNotebookDeleted   caller = "notebook deleted"
	callerOutsideWorkspace  caller = "outside the workspace"
	callerGuestReaderOfOpen caller = "guest reader open"
	// An editor of orphan, a notebook without an admin (M3/P3 design 3.7).
	callerOwnerlessMember caller = "ownerless notebook member"
)

// notebookColumns are the columns of the notebook level.
func notebookColumns() []caller {
	return []caller{
		callerNotebookAdmin, callerNotebookEditor, callerNotebookReader, callerOutsideAdmin, callerOutsideMember,
		callerDefaultEditor, callerDefaultReader, callerOutsideGuest, callerNotebookEnded, callerNotebookDeleted,
		callerOutsideWorkspace, callerGuestReaderOfOpen, callerOwnerlessMember,
	}
}

// allColumns are every column: each has an account of its own.
func allColumns() []caller { return slices.Concat(workspaceColumns(), notebookColumns()) }

// workspaceOf is the slug of the workspace a column's cells target: the
// prepared workspace, or the deleted one its caller was the admin of; lab
// for the notebook columns, whose memberships stay out of acme's member
// list.
func workspaceOf(c caller) string {
	switch {
	case c == callerDeleted:
		return "gone"
	case slices.Contains(notebookColumns(), c):
		return "lab"
	}
	return "acme"
}

// cell is an answer: the status and, for a problem, its code.
type cell struct {
	status int
	code   string
}

// String is the answer as a failure prints it.
func (c cell) String() string {
	if c.code == "" {
		return fmt.Sprint(c.status)
	}
	return fmt.Sprintf("%d %s", c.status, c.code)
}

// The answers rows use most.
func cellOK() cell        { return cell{status: http.StatusOK} }
func cellCreated() cell   { return cell{status: http.StatusCreated} }
func cellForbidden() cell { return cell{http.StatusForbidden, "forbidden"} }

// matrixRow is an operation's row: the request each caller sends, which can
// name what prepareMatrix seeded, and the answer each gets.
type matrixRow struct {
	op      string // operationId
	variant string // what sets the row apart from the operation's other rows
	// columns are the row's columns: the workspace level's when nil.
	columns []caller
	write   bool // each cell on a copy of its own
	config  func(*config.Config)
	request func(c caller, s seeded) (method, path, body string)
	cells   map[caller]cell
	// check, when set, runs on each answer that is not a problem: what the
	// answer holds for that caller.
	check func(t *testing.T, c caller, s seeded, answer string)
}

// callers are the row's columns.
func (r matrixRow) callers() []caller {
	if r.columns == nil {
		return workspaceColumns()
	}
	return r.columns
}

func (r matrixRow) name() string {
	if r.variant == "" {
		return r.op
	}
	return r.op + ", " + r.variant
}

// every is the same answer in every column.
func every(answer cell) map[caller]cell {
	cells := map[caller]cell{}
	for _, c := range workspaceColumns() {
		cells[c] = answer
	}
	return cells
}

// sameRequest is the request of a row whose callers all send the same.
func sameRequest(method, path, body string) func(caller, seeded) (string, string, string) {
	return func(caller, seeded) (string, string, string) { return method, path, body }
}

// decodeAnswer decodes a cell's answer into v for a row's check.
func decodeAnswer(t *testing.T, answer string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(answer), v); err != nil {
		t.Fatalf("the answer %s: %v", answer, err)
	}
}

// matrixRows are the rows, each module's from its file.
func matrixRows() []matrixRow {
	return slices.Concat(workspaceMatrixRows(), memberMatrixRows(), invitationMatrixRows(), notebookMatrixRows(),
		notebookMemberMatrixRows(), ownerlessMatrixRows())
}

// matrixApps is how many cells may run an app of their own at once: each
// cell that writes, or whose row has a config, takes a slot while its app
// runs. Each app's pool opens up to testConfig's MaxConns (4) connections,
// against the container's max_connections of 100.
const matrixApps = 8

// Each cell of the matrix, in parallel: a reading cell on the reads' copy,
// a cell with an app of its own (a writing cell, or one whose row has a
// config) on its copy, at most matrixApps of those at once. Every answer a
// row's check is for is checked, and counted: a harness that skipped the
// checks would fail.
func TestPermissionMatrix(t *testing.T) {
	d := prepareMatrix(t)
	contract := apitest.Load(t)
	reads := startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), nil), migrations.FS())
	apps := make(chan struct{}, matrixApps)
	var checked, toCheck atomic.Int64
	t.Cleanup(func() {
		if !t.Failed() && checked.Load() != toCheck.Load() {
			t.Errorf("%d answers checked, want %d", checked.Load(), toCheck.Load())
		}
	})
	for _, r := range matrixRows() {
		for _, c := range r.callers() {
			want, ok := r.cells[c]
			if !ok {
				continue // TestThePermissionMatrixCoversEveryOperation reports it
			}
			t.Run(r.name()+"/"+string(c), func(t *testing.T) {
				t.Parallel()
				// Counted in the cell: a -run of some cells expects only
				// their checks.
				if r.check != nil && want.code == "" {
					toCheck.Add(1)
				}
				base := reads
				if r.write || r.config != nil {
					apps <- struct{}{}
					// Registered before the app's: cleanups run last first,
					// so the slot is freed once the app is closed.
					t.Cleanup(func() { <-apps })
					base = startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), r.config), migrations.FS())
				}
				method, path, body := r.request(c, d.seeded.in(t))
				status, answer := ask(t, contract, method, base+path, d.tokens[c], body)
				got := cell{status: status}
				if status >= http.StatusBadRequest {
					got.code = problemCode(t, answer)
				}
				if got != want {
					t.Errorf("%s %s = %d %s, want %s", method, path, status, strings.TrimSpace(answer), want)
					return
				}
				if r.check != nil && got.code == "" {
					r.check(t, c, d.seeded.in(t), answer)
					checked.Add(1)
				}
			})
		}
	}
}

// ask sends method url with token and body, checks the answer against the
// contract, and returns its status and body.
func ask(t *testing.T, contract *apitest.Contract, method, url, token, body string) (int, string) {
	t.Helper()
	var payload []byte
	if body != "" {
		payload = []byte(body)
	}
	req := newRequest(t, method, url, token, payload)
	res, answer := sendRequest(t, req)
	contract.CheckResponse(t, req, res)
	return res.StatusCode, string(answer)
}

// problemCode is the code of a problem answer.
func problemCode(t *testing.T, answer string) string {
	t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	decodeAnswer(t, answer, &p)
	return p.Code
}
