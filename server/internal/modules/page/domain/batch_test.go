package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// str and flag are an op's optional member, given.
func str(s string) *string { return &s }
func flag(b bool) *bool    { return &b }

// writeOp, editOp, moveOp and deleteOp are ops of a batch with their
// tool's required members.
func writeOp(path, content string) domain.BatchArgs {
	return domain.BatchArgs{Op: "write", Path: str(path), Content: str(content)}
}

func editOp(path, old, new string) domain.BatchArgs {
	return domain.BatchArgs{Op: "edit", Path: str(path), Old: str(old), New: str(new)}
}

func moveOp(from, to string) domain.BatchArgs {
	return domain.BatchArgs{Op: "move", From: str(from), To: str(to)}
}

func deleteOp(path string) domain.BatchArgs { return domain.BatchArgs{Op: "delete", Path: str(path)} }

// with is o with set's change.
func with(o domain.BatchArgs, set func(*domain.BatchArgs)) domain.BatchArgs {
	set(&o)
	return o
}

// refusal is what a check refused: the problem's code, and its field and
// field code when it names one; at, the op an *OpFailed names, -1 for
// none.
type refusal struct {
	code, field, fieldCode string
	at                     int
}

// refusalOf is err as a refusal.
func refusalOf(t *testing.T, err error) refusal {
	t.Helper()
	r := refusal{at: -1}
	if failed := (*domain.OpFailed)(nil); errors.As(err, &failed) {
		r.at = failed.Op
	}
	var e *shared.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v is no problem", err)
	}
	r.code = e.Code
	if len(e.Fields) > 0 {
		r.field, r.fieldCode = e.Fields[0].Field, e.Fields[0].Code
	}
	return r
}

// CheckBatch, its value 422s in their order (M9/P3 design 3.3.2, 3.9.3
// step 2): the number of ops, the strings together, the contents and
// new_strings together, the message; then each op in their order: its
// tool, the members it requires, those it takes not, and its values by
// its tool's rules, the first that fails at its op.
func TestCheckBatch(t *testing.T) {
	const tooLarge = "page.batch_too_large"
	many := func(n int, o domain.BatchArgs) []domain.BatchArgs {
		ops := make([]domain.BatchArgs, n)
		for i := range ops {
			ops[i] = o
		}
		return ops
	}
	// The strings together at their bound: a write of a content one byte
	// short of the contents' bound, and an edit whose old_string brings
	// the rest, so that the contents and new_strings are at theirs too.
	content := strings.Repeat("c", domain.MaxContentBytes-1)
	old := strings.Repeat("o", domain.MaxBatchStringBytes-len(content)-len("write")-len("A.md")-len("edit")-len("B.md")-len("n")-len("m"))
	atStrings := []domain.BatchArgs{writeOp("A.md", content), editOp("B.md", old, "n")}
	pastStrings := []domain.BatchArgs{writeOp("A.md", content), editOp("B.md", old+"o", "n")}
	pastContents := []domain.BatchArgs{writeOp("A.md", content), editOp("B.md", "o", "nn")}
	badPath := writeOp("A/../B.md", "x")
	badOp := domain.BatchArgs{Op: "rename", Path: str("A.md")}
	token := domain.VersionToken{Node: uuid.NewV7(), Revision: 3}.String()
	for _, tt := range []struct {
		name    string
		ops     []domain.BatchArgs
		message string
		want    refusal // code "" for none
	}{
		{"no op", nil, "m", refusal{"validation_failed", "ops", "too_short", -1}},
		{"one op", many(1, writeOp("A.md", "x")), "m", refusal{}},
		{"100 ops", many(domain.MaxBatchOps, writeOp("A.md", "x")), "m", refusal{}},
		{"101 ops", many(domain.MaxBatchOps+1, writeOp("A.md", "x")), "m", refusal{tooLarge, "", "", -1}},
		{"the strings at their bound", atStrings, "m", refusal{}},
		{"a byte of strings more", pastStrings, "m", refusal{tooLarge, "", "", -1}},
		{"a byte of contents and new_strings more", pastContents, "m", refusal{tooLarge, "", "", -1}},
		{"no message", many(1, writeOp("A.md", "x")), "", refusal{"validation_failed", "message", "too_short", -1}},
		{"a message at its bound", many(1, writeOp("A.md", "x")), strings.Repeat("m", domain.MaxMessageBytes), refusal{}},
		{"a message a byte past", many(1, writeOp("A.md", "x")), strings.Repeat("m", domain.MaxMessageBytes+1),
			refusal{"validation_failed", "message", "too_long", -1}},
		{"an op of no tool", []domain.BatchArgs{badOp}, "m", refusal{"validation_failed", "ops[0].op", "not_allowed", 0}},
		{"no op named", []domain.BatchArgs{{Path: str("A.md")}}, "m", refusal{"validation_failed", "ops[0].op", "not_allowed", 0}},
		// The totals and the message come before any op's values (M9/P3
		// review C-M1).
		{"101 ops, the first of no tool", append([]domain.BatchArgs{badOp}, many(domain.MaxBatchOps, writeOp("A.md", "x"))...), "m",
			refusal{tooLarge, "", "", -1}},
		{"strings past, the first op's path bad", append([]domain.BatchArgs{badPath}, pastStrings...), "m",
			refusal{tooLarge, "", "", -1}},
		{"the message bad, the first op of no tool", []domain.BatchArgs{badOp}, "", refusal{"validation_failed", "message", "too_short", -1}},
		// The first op that fails decides.
		{"ops[3]'s path bad, ops[5] of no tool", []domain.BatchArgs{writeOp("A.md", "x"), writeOp("B.md", "x"),
			writeOp("C.md", "x"), badPath, writeOp("D.md", "x"), badOp}, "m", refusal{"validation_failed", "ops[3].path", "invalid_format", 3}},
		// Each tool's values in its use case's order.
		{"write's path", []domain.BatchArgs{badPath}, "m", refusal{"validation_failed", "ops[0].path", "invalid_format", 0}},
		{"write's attachment path", []domain.BatchArgs{writeOp("a.png", "x")}, "m", refusal{"validation_failed", "ops[0].path", "not_allowed", 0}},
		{"write's content", []domain.BatchArgs{writeOp("A.md", "a\x00b")}, "m",
			refusal{"validation_failed", "ops[0].content", "invalid_format", 0}},
		{"write's token", []domain.BatchArgs{with(writeOp("A.md", "x"), func(o *domain.BatchArgs) { o.Base = str("nope") })}, "m",
			refusal{"validation_failed", "ops[0].base_version", "invalid_format", 0}},
		{"write's path before its content", []domain.BatchArgs{writeOp("A/../B.md", "a\x00b")}, "m",
			refusal{"validation_failed", "ops[0].path", "invalid_format", 0}},
		{"edit's path", []domain.BatchArgs{editOp("a.png", "o", "n")}, "m", refusal{"validation_failed", "ops[0].path", "not_allowed", 0}},
		{"edit's strings", []domain.BatchArgs{editOp("A.md", "", "n")}, "m", refusal{"validation_failed", "ops[0].old_string", "too_short", 0}},
		{"edit's strings the same", []domain.BatchArgs{editOp("A.md", "o", "o")}, "m",
			refusal{"validation_failed", "ops[0].new_string", "not_allowed", 0}},
		{"edit's token", []domain.BatchArgs{with(editOp("A.md", "o", "n"), func(o *domain.BatchArgs) { o.Base = str("x:1") })}, "m",
			refusal{"validation_failed", "ops[0].base_version", "invalid_format", 0}},
		{"move's from", []domain.BatchArgs{moveOp("", "B.md")}, "m", refusal{"validation_failed", "ops[0].from", "required", 0}},
		{"move's to", []domain.BatchArgs{moveOp("A.md", "B/")}, "m", refusal{"validation_failed", "ops[0].to", "invalid_format", 0}},
		{"move's kinds", []domain.BatchArgs{moveOp("A.md", "b.png")}, "m", refusal{"validation_failed", "ops[0].to", "not_allowed", 0}},
		{"delete's path", []domain.BatchArgs{deleteOp("/")}, "m", refusal{"validation_failed", "ops[0].path", "required", 0}},
		{"a token well formed", []domain.BatchArgs{with(writeOp("A.md", "x"), func(o *domain.BatchArgs) { o.Base = &token })}, "m", refusal{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			steps, err := domain.CheckBatch(tt.ops, tt.message)
			switch {
			case tt.want.code == "" && err != nil:
				t.Fatalf("CheckBatch: %v", err)
			case tt.want.code == "":
				if len(steps) != len(tt.ops) {
					t.Errorf("CheckBatch's steps = %d, want %d", len(steps), len(tt.ops))
				}
				return
			case err == nil:
				t.Fatalf("CheckBatch passed, want %+v", tt.want)
			}
			if got := refusalOf(t, err); got != tt.want {
				t.Errorf("CheckBatch = %+v (%v), want %+v", got, err, tt.want)
			}
		})
	}
}

// Each tool's members in a batch's op (M9/P3 design 3.3.2): one it
// requires left out is 422 required at it, before one it takes not, given,
// 422 not_allowed; a row a member.
func TestCheckBatchTakesEachToolsMembers(t *testing.T) {
	full := domain.BatchArgs{Path: str("A.md"), Content: str("x"), Base: str(domain.VersionToken{Node: uuid.NewV7(), Revision: 1}.String()),
		Old: str("o"), New: str("n"), All: flag(true), From: str("A.md"), To: str("B.md"), UpdateLinks: flag(false), Recursive: flag(true)}
	members := []string{"path", "content", "base_version", "old_string", "new_string", "replace_all", "from", "to", "update_links", "recursive"}
	set := func(o *domain.BatchArgs, member string, give bool) {
		f := map[string]func(){
			"path":         func() { o.Path = give0(give, full.Path) },
			"content":      func() { o.Content = give0(give, full.Content) },
			"base_version": func() { o.Base = give0(give, full.Base) },
			"old_string":   func() { o.Old = give0(give, full.Old) },
			"new_string":   func() { o.New = give0(give, full.New) },
			"replace_all":  func() { o.All = give0(give, full.All) },
			"from":         func() { o.From = give0(give, full.From) },
			"to":           func() { o.To = give0(give, full.To) },
			"update_links": func() { o.UpdateLinks = give0(give, full.UpdateLinks) },
			"recursive":    func() { o.Recursive = give0(give, full.Recursive) },
		}
		f[member]()
	}
	for _, tool := range []struct {
		op                 string
		required, optional []string
	}{
		{"write", []string{"path", "content"}, []string{"base_version"}},
		{"edit", []string{"path", "old_string", "new_string"}, []string{"replace_all", "base_version"}},
		{"move", []string{"from", "to"}, []string{"update_links"}},
		{"delete", []string{"path"}, []string{"recursive"}},
	} {
		base := domain.BatchArgs{Op: tool.op}
		for _, m := range slices.Concat(tool.required, tool.optional) {
			set(&base, m, true)
		}
		if _, err := domain.CheckBatch([]domain.BatchArgs{base}, "m"); err != nil {
			t.Fatalf("%s with its members: %v", tool.op, err)
		}
		for _, m := range tool.required {
			o := base
			set(&o, m, false)
			o2 := o
			set(&o2, notTaken(members, tool.required, tool.optional), true) // a member it takes not, given too
			want := refusal{"validation_failed", "ops[1]." + m, "required", 1}
			for _, op := range []domain.BatchArgs{o, o2} {
				_, err := domain.CheckBatch([]domain.BatchArgs{base, op}, "m")
				if got := refusalOf(t, err); got != want || fieldMessage(err) != "is required for "+tool.op {
					t.Errorf("%s without %s = %+v (%v), want %+v", tool.op, m, got, err, want)
				}
			}
		}
		for _, m := range members {
			if slices.Contains(tool.required, m) || slices.Contains(tool.optional, m) {
				continue
			}
			o := base
			set(&o, m, true)
			want := refusal{"validation_failed", "ops[1]." + m, "not_allowed", 1}
			_, err := domain.CheckBatch([]domain.BatchArgs{base, o}, "m")
			if got := refusalOf(t, err); got != want || fieldMessage(err) != "is no argument of "+tool.op {
				t.Errorf("%s with %s = %+v (%v), want %+v", tool.op, m, got, err, want)
			}
		}
	}
}

// fieldMessage is the message of err's first field, "" for none.
func fieldMessage(err error) string {
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) == 0 {
		return ""
	}
	return e.Fields[0].Message
}

// give0 is v when give, nil otherwise.
func give0[T any](give bool, v *T) *T {
	if give {
		return v
	}
	return nil
}

// notTaken is a member of members neither required nor optional.
func notTaken(members, required, optional []string) string {
	for _, m := range members {
		if !slices.Contains(required, m) && !slices.Contains(optional, m) {
			return m
		}
	}
	return ""
}

// CheckBatch's steps are the ops parsed, each tool's defaults set: an
// edit replaces once, a move rewrites links, a delete takes no children.
func TestCheckBatchParsesEachOp(t *testing.T) {
	node := uuid.NewV7()
	token := domain.VersionToken{Node: node, Revision: 4}.String()
	steps, err := domain.CheckBatch([]domain.BatchArgs{
		with(writeOp("/A/B.md", "body"), func(o *domain.BatchArgs) { o.Base = &token }),
		editOp("A.md", "o", "n"),
		with(editOp("A.md", "o", "n"), func(o *domain.BatchArgs) { o.All = flag(true) }),
		moveOp("A.md", "C/D.md"),
		with(moveOp("A.md", "E.md"), func(o *domain.BatchArgs) { o.UpdateLinks = flag(false) }),
		deleteOp("x.png"),
		with(deleteOp("A.md"), func(o *domain.BatchArgs) { o.Recursive = flag(true) }),
	}, "m")
	if err != nil {
		t.Fatal(err)
	}
	if s := steps[0]; s.Kind != domain.OpKindWrite || s.Args.Field("path") != "ops[0].path" || !slices.Equal(s.Path.Titles, []string{"A", "B"}) ||
		s.Content != "body" || s.Base == nil || *s.Base != (domain.VersionToken{Node: node, Revision: 4}) {
		t.Errorf("write = %+v", s)
	}
	if s := steps[1]; s.Kind != domain.OpKindEdit || s.Old != "o" || s.New != "n" || s.All || s.Base != nil {
		t.Errorf("edit = %+v", s)
	}
	if !steps[2].All {
		t.Error("replace_all was not taken")
	}
	if s := steps[3]; s.Kind != domain.OpKindMove || s.From.Written != "A.md" || s.To.Written != "C/D.md" || !s.UpdateLinks {
		t.Errorf("move = %+v", s)
	}
	if steps[4].UpdateLinks {
		t.Error("update_links false was not taken")
	}
	if s := steps[5]; s.Kind != domain.OpKindDelete || s.Path.Kind != domain.KindAsset || s.Recursive || !steps[6].Recursive {
		t.Errorf("delete = %+v, %+v", s, steps[6])
	}
}
