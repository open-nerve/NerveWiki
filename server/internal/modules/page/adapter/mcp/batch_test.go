package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// batchFunc is an ApplyBatch of a function.
type batchFunc func(notebookID uuid.UUID, p app.PathBatch, client domain.Client) (app.Batched, error)

func (f batchFunc) Execute(_ context.Context, id uuid.UUID, p app.PathBatch, c domain.Client) (app.Batched, error) {
	return f(id, p, c)
}

// batchOf is the tools with batch's use case f.
func batchOf(f batchFunc) tools {
	return tools{uc: UseCases{Batch: f}}
}

// batch passes its ops, each member to its argument, its message and the
// agent's client to its use case.
func TestTheBatchPassesItsOps(t *testing.T) {
	notebook := uuid.NewV7()
	var got app.PathBatch
	var client domain.Client
	stop := errors.New("stop")
	tools := batchOf(func(id uuid.UUID, p app.PathBatch, c domain.Client) (app.Batched, error) {
		if id != notebook {
			t.Errorf("the notebook passed is %s, want %s", id, notebook)
		}
		got, client = p, c
		return app.Batched{}, stop
	})
	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }
	in := batchIn{Message: "why", Ops: []batchOp{
		{Op: "write", Path: s("A.md"), Content: s("a"), BaseVersion: s("x:1")},
		{Op: "edit", Path: s("B.md"), OldString: s("o"), NewString: s("n"), ReplaceAll: b(true)},
		{Op: "move", From: s("C.md"), To: s("D.md"), UpdateLinks: b(false)},
		{Op: "delete", Path: s("E.md"), Recursive: b(true)},
	}}
	if _, err := tools.batch(context.Background(), mcpserver.Call{Notebook: notebook, Client: "Claude Code"}, in); !errors.Is(err, stop) {
		t.Fatalf("batch = %v, want its use case's error", err)
	}
	want := []domain.BatchArgs{
		{Op: "write", Path: s("A.md"), Content: s("a"), Base: s("x:1")},
		{Op: "edit", Path: s("B.md"), Old: s("o"), New: s("n"), All: b(true)},
		{Op: "move", From: s("C.md"), To: s("D.md"), UpdateLinks: b(false)},
		{Op: "delete", Path: s("E.md"), Recursive: b(true)},
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got.Ops)
	if string(gotJSON) != string(wantJSON) || got.Message != "why" || client != "mcp:Claude Code" {
		t.Errorf("the use case got %s, %q, %s; want %s, the message and the agent", gotJSON, got.Message, client, wantJSON)
	}
}

// batch answers its changeset and each op as its tool would, without the
// changeset (M9/P3 design 3.9.1): write's version and created, edit's
// version and replacements, move's path alone, delete's counts, merged
// into a later one with none of its own; a batch that changed nothing has
// no changeset.
func TestTheBatchAnswersEachOp(t *testing.T) {
	changeset, at := uuid.NewV7(), time.Date(2026, 10, 20, 9, 0, 0, 0, time.FixedZone("x", 3600))
	page, folder := domain.Node{ID: uuid.NewV7()}, domain.Node{ID: uuid.NewV7()}
	token := domain.VersionToken{Node: page.ID, Revision: 2}
	ops := []app.OpDone{
		{Kind: domain.OpKindWrite, Path: "F/P.md", Node: page, Version: token, Changed: true, MergedInto: -1,
			Created: []app.Placed{{Path: "F.md", Node: folder}, {Path: "F/P.md", Node: page}}},
		{Kind: domain.OpKindEdit, Path: "F/P.md", Node: page, Version: token, Changed: true, Replacements: 3, MergedInto: -1},
		{Kind: domain.OpKindMove, Path: "G.md", Node: folder, Changed: true, MergedInto: -1},
		{Kind: domain.OpKindDelete, Path: "H/X.md", Node: page, Changed: true, MergedInto: 4},
		{Kind: domain.OpKindDelete, Path: "H.md", Node: folder, Changed: true, Pages: 2, Assets: 1, MergedInto: -1},
	}
	tools := batchOf(func(uuid.UUID, app.PathBatch, domain.Client) (app.Batched, error) {
		return app.Batched{Ops: ops, Outcome: app.Outcome{ChangesetID: changeset, At: at}}, nil
	})
	res, err := tools.batch(context.Background(), mcpserver.Call{}, batchIn{})
	if err != nil {
		t.Fatal(err)
	}
	p, f := page.ID.String(), folder.ID.String()
	want := `{"changeset_id":"` + changeset.String() + `","at":"2026-10-20T08:00:00Z","changed":true,"results":[` +
		`{"op":"write","path":"F/P.md","node_id":"` + p + `","changed":true,"version":"` + token.String() + `",` +
		`"created":[{"path":"F.md","node_id":"` + f + `"},{"path":"F/P.md","node_id":"` + p + `"}],"created_count":2},` +
		`{"op":"edit","path":"F/P.md","node_id":"` + p + `","changed":true,"version":"` + token.String() + `","replacements":3},` +
		`{"op":"move","path":"G.md","node_id":"` + f + `","changed":true},` +
		`{"op":"delete","path":"H/X.md","node_id":"` + p + `","changed":true,"pages":0,"attachments":0,"merged_into":4},` +
		`{"op":"delete","path":"H.md","node_id":"` + f + `","changed":true,"pages":2,"attachments":1}],"results_count":5}`
	if got := rawOf(t, res); got != want {
		t.Errorf("the answer =\n%s\nwant\n%s", got, want)
	}

	tools = batchOf(func(uuid.UUID, app.PathBatch, domain.Client) (app.Batched, error) {
		return app.Batched{Ops: ops[2:3]}, nil
	})
	if res, err = tools.batch(context.Background(), mcpserver.Call{}, batchIn{}); err != nil ||
		!strings.HasPrefix(rawOf(t, res), `{"changeset_id":null,"at":null,"changed":false,"results":[{"op":"move"`) {
		t.Errorf("a batch that changed nothing = %s, %v; want no changeset", rawOf(t, res), err)
	}
}

// batch's answer holds its ops and the nodes they created as many as fit,
// in one column (M9/P3 design 3.9.1): 100 ops at the longest path stop at
// an op, a write of ten nodes at the longest paths within its nodes; the
// counts count them all, and the changeset is always there.
func TestTheBatchFitsItsResults(t *testing.T) {
	segment := strings.Repeat("&", 255)
	worst := strings.Repeat(segment+"/", 10) + segment + ".md"
	changeset := uuid.NewV7()
	var ops []app.OpDone
	for range 100 {
		ops = append(ops, app.OpDone{Kind: domain.OpKindMove, Path: worst, Node: domain.Node{ID: uuid.NewV7()}, MergedInto: -1})
	}
	var created []app.Placed
	path := ""
	for i := range 10 {
		if i > 0 {
			path += "/"
		}
		path += segment
		created = append(created, app.Placed{Path: path + ".md", Node: domain.Node{ID: uuid.NewV7()}})
	}
	write := app.OpDone{Kind: domain.OpKindWrite, Path: created[9].Path, Node: created[9].Node, Created: created, MergedInto: -1}
	for _, tt := range []struct {
		name string
		ops  []app.OpDone
		cut  func(a batchAnswer) bool
	}{
		{"100 ops of the longest path", ops, func(a batchAnswer) bool { return len(a.Results) > 0 && len(a.Results) < 100 }},
		{"a write of ten nodes of the longest paths", []app.OpDone{write}, func(a batchAnswer) bool {
			return len(a.Results) == 1 && *a.Results[0].CreatedCount == 10 && len(a.Results[0].Created) > 0 &&
				len(a.Results[0].Created) < 10
		}},
	} {
		tools := batchOf(func(uuid.UUID, app.PathBatch, domain.Client) (app.Batched, error) {
			return app.Batched{Ops: tt.ops, Outcome: app.Outcome{ChangesetID: changeset, At: time.Now()}}, nil
		})
		res, err := tools.batch(context.Background(), mcpserver.Call{}, batchIn{})
		if err != nil {
			t.Fatal(err)
		}
		var a batchAnswer
		if err := json.Unmarshal([]byte(rawOf(t, res)), &a); err != nil {
			t.Fatal(err)
		}
		if !tt.cut(a) || a.ResultsCount != len(tt.ops) || a.ChangesetID == nil || *a.ChangesetID != changeset.String() ||
			mcpserver.TextBytes(rawOf(t, res)) > mcpserver.MaxAnswerBytes {
			t.Errorf("%s: %d results of %d, changeset %v, in %d bytes", tt.name, len(a.Results), a.ResultsCount, a.ChangesetID,
				mcpserver.TextBytes(rawOf(t, res)))
		}
	}
}

// failedAt is the error of the op i, a tool's, of the use case.
func failedAt(i int, tool domain.OpKind, err error) error {
	return &domain.OpFailed{Op: i, Tool: tool, Err: err}
}

// batch words an op's error as the op's tool does, from the op's own
// path, at the op (M9/P3 design 3.9.2): the place "ops[i] (tool)" after
// the code (ToolError.At) and op in the error's member, in each of a fit's
// builds; the deferred rewrite's refusal of edit locks names the batch's
// renames and moves; an error of the batch as a whole is at none.
func TestTheBatchWordsItsErrors(t *testing.T) {
	s := func(v string) *string { return &v }
	ops := []batchOp{{Op: "write", Path: s("B.md")}, {Op: "move", From: s("A.md"), To: s("C.md")},
		{Op: "edit", Path: s("E.md")}, {Op: "delete", Path: s("D.md")}, {Op: "rename"}}
	me := mcpserver.Call{UserID: uuid.NewV7()}
	bob := shared.LockHolder{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: "Bob"}
	carol := shared.LockHolder{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: "Carol"}
	paths := map[uuid.UUID]string{bob.PageID: "P.md", carol.PageID: "Q.md"}
	sub := domain.Subtree{{Node: domain.Node{ID: uuid.NewV7(), Kind: domain.KindPage}, Level: 1},
		{Node: domain.Node{ID: uuid.NewV7(), Kind: domain.KindPage}, Level: 2}}
	recursive := domain.DeletionOf(domain.OpArgs(3), sub, false)
	token := domain.VersionToken{Node: uuid.NewV7(), Revision: 4}
	for _, tt := range []struct {
		name  string
		err   error
		at    string
		op    int
		words string
	}{
		{"a write's page there", failedAt(0, domain.OpKindWrite, &domain.PathTaken{Path: "B.md"}), "ops[0] (write)", 0,
			`page.title_taken: "B.md" is a page already.`},
		{"a move's to taken", failedAt(1, domain.OpKindMove, &domain.PathTaken{Path: "C.md"}), "ops[1] (move)", 1,
			`page.title_taken: "C.md" is taken: a page there has that title.`},
		{"an edit's version", failedAt(2, domain.OpKindEdit, &domain.VersionMismatch{Path: "E.md", Current: token,
			Why: domain.KeptChanging}), "ops[2] (edit)", 2, `page.revision_mismatch: "E.md" changed twice while the edit was made`},
		{"a delete without recursive", failedAt(3, domain.OpKindDelete, recursive), "ops[3] (delete)", 3,
			`validation_failed: "D.md" has 1 page under it: set ops[3].recursive to true`},
		{"a delete's locks", failedAt(3, domain.OpKindDelete, &domain.LockPaths{Err: domain.LockedAll([]shared.LockHolder{bob, carol}),
			Paths: paths}), "ops[3] (delete)", 3, `page.locked: "D.md" has 2 pages being edited in the web editor, by Bob and Carol`},
		{"the deferred rewrite's locks", failedAt(1, domain.OpKindMove, &domain.LockPaths{Err: pagesLocked(bob, carol), Paths: paths}),
			"ops[1] (move)", 1, "linking.pages_locked: The batch's renames and moves would rewrite links in 2 pages being edited"},
		{"a path not found", failedAt(2, domain.OpKindEdit, &domain.PathNotFound{Path: "E.md"}), "ops[2] (edit)", 2,
			`page.not_found: Nothing is at "E.md".`},
		{"an unknown op", failedAt(4, "", domain.NotAllowed(domain.OpArgs(4).Field("op"), "is one of write, edit, move and delete")),
			"ops[4]", 4, ""},
		{"the notebook changed twice", failedAt(1, domain.OpKindMove, domain.BatchChanged()), "ops[1] (move)", 1, ""},
	} {
		got := batchErr(tt.err, me, ops)
		var te *mcpserver.ToolError
		if !errors.As(got, &te) || te.At != tt.at {
			t.Errorf("%s: %#v, want at %s", tt.name, got, tt.at)
			continue
		}
		builds := [][2]any{}
		if te.Fit != nil {
			for k := 0; k <= te.Fit.N; k++ {
				text, extra := te.Fit.Build(k)
				builds = append(builds, [2]any{text, extra})
			}
		} else {
			builds = append(builds, [2]any{te.Text, te.Extra})
		}
		for _, b := range builds {
			text, extra := b[0].(string), b[1].(map[string]any)
			if extra["op"] != tt.op || !strings.HasPrefix(text, tt.words) {
				t.Errorf("%s: %q with %v, want op %d and words starting %q", tt.name, text, extra, tt.op, tt.words)
			}
		}
	}
	for _, err := range []error{domain.BatchTooLarge("ops", 101, 100, "split the batch"), shared.Invalid(shared.FieldError{
		Field: "ops", Code: shared.FieldTooShort, Message: "must hold at least one op"})} {
		var te *mcpserver.ToolError
		if got := batchErr(err, me, ops); errors.As(got, &te) {
			t.Errorf("an error of the batch as a whole, %v, = %+v; want it as it is, at no op", err, te)
		}
	}
}

// Each error of batch fits in an answer at its worst (M9/P3 design 3.9.2,
// as M9/P2 design 3.10.3): an op's place of three digits, the longest
// path of "&", the call's and the error's; 25 locks of the longest names
// and paths; the bounds' numbers at their largest; each measured as
// mcpserver answers it, At included, and answered with its code.
func TestEachBatchErrorFitsAtItsWorst(t *testing.T) {
	segment := strings.Repeat("&", 255)
	worst := strings.Repeat(segment+"/", 10) + segment + ".md"
	tenth := strings.Repeat(segment+"/", 9) + segment + ".md"
	name := strings.Repeat("&", 100)
	me := mcpserver.Call{UserID: uuid.NewV7()}
	token := domain.VersionToken{Node: uuid.NewV7(), Revision: domain.MaxRevision}
	holders := make([]shared.LockHolder, 25)
	paths := map[uuid.UUID]string{}
	for i := range holders {
		holders[i] = shared.LockHolder{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: name}
		paths[holders[i].PageID] = tenth
	}
	ops := make([]batchOp, 100)
	for i := range ops {
		ops[i] = batchOp{Op: "move", Path: &worst, From: &worst, To: &worst}
	}
	big := 1 << 30
	for _, tt := range []struct {
		name string
		tool domain.OpKind
		err  error
		code string
	}{
		{"a page's hint", domain.OpKindEdit, &domain.PathNotFound{Path: worst, Hint: domain.PageHint(worst)}, "page.not_found"},
		{"to's parent", domain.OpKindMove, &domain.PathNotFound{Path: worst, Hint: domain.MoveParentHint, ParentOf: "ops[99].to"},
			"page.not_found"},
		{"recursive", domain.OpKindDelete, &domain.Recursive{Field: "ops[99].recursive", Pages: big, Assets: big}, "validation_failed"},
		{"a write's page there", domain.OpKindWrite, &domain.PathTaken{Path: worst}, "page.title_taken"},
		{"a move's to taken", domain.OpKindMove, &domain.PathTaken{Path: worst, Asset: true}, "page.title_taken"},
		{"kept changing", domain.OpKindEdit, &domain.VersionMismatch{Path: worst, Current: token, Why: domain.KeptChanging},
			"page.revision_mismatch"},
		{"a lock", domain.OpKindEdit, &domain.LockPaths{Err: domain.Locked(holders[0].PageID, holders[0].UserID, name),
			Paths: map[uuid.UUID]string{holders[0].PageID: worst}}, "page.locked"},
		{"a deletion's locks", domain.OpKindDelete, &domain.LockPaths{Err: domain.LockedAll(holders), Paths: paths}, "page.locked"},
		{"the deferred rewrite's locks", domain.OpKindMove, &domain.LockPaths{Err: pagesLocked(holders...), Paths: paths},
			"linking.pages_locked"},
		{"the rewrite's bound", domain.OpKindMove, domain.BatchTooLarge("pages whose links the batch's renames and moves would rewrite",
			big, big, "move with update_links false, or in separate calls"), "page.batch_too_large"},
		{"the plan's parses", domain.OpKindEdit, domain.BatchTooLarge("bytes of contents the plan parses to rewrite links", big, big,
			"send the renames and moves in a call apart from the edits after them"), "page.batch_too_large"},
		{"B1", domain.OpKindDelete, domain.NotAllowed(domain.OpArgs(99).Op(), fmt.Sprintf("deletes a page that ops[%d] created: a batch "+
			"may not delete what it creates (the change would hold a node that is neither before nor after it); leave both out, "+
			"or delete it in a later call", 98)), "validation_failed"},
		{"changed twice", domain.OpKindMove, domain.BatchChanged(), "page.revision_mismatch"},
	} {
		code, n := mcpserver.ErrorAnswerBytes(batchErr(failedAt(99, tt.tool, tt.err), me, ops))
		if code != tt.code || n > mcpserver.MaxAnswerBytes {
			t.Errorf("%s: %s of %d bytes, want %s within %d", tt.name, code, n, tt.code, mcpserver.MaxAnswerBytes)
		}
		t.Logf("%s: %s, %d bytes", tt.name, code, n)
	}
}
