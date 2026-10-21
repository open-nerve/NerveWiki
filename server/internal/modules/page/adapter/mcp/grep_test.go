package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// grepFunc is a Grep of a function.
type grepFunc func(notebookID uuid.UUID, r app.GrepRequest) (app.Grepped, error)

func (f grepFunc) Execute(_ context.Context, notebookID uuid.UUID, r app.GrepRequest) (app.Grepped, error) {
	return f(notebookID, r)
}

// grep's answer (M9/P4 design 3.10.1): the hits and the partial lines in
// their lists, each in the walk's order, a hit's column only when its text
// does not start at the line's first byte and cut only when it is cut;
// next_cursor the use case's, null without one; building and incomplete
// the use case's. The call passes its notebook and arguments, ignore_case
// false without it; a use case's error is the tool's as it is.
func TestTheGrepAnswersItsShape(t *testing.T) {
	g := app.Grepped{Found: []app.GrepFound{
		{Path: "A.md", Line: 1, Hit: &domain.GrepHit{Text: "a needle", Column: 3}, Cursor: "c1"},
		{Path: "A.md", Line: 4, Cursor: "c2"},
		{Path: "B/C.md", Line: 9, Hit: &domain.GrepHit{Text: "needle cut", Column: 4000, Cut: true}, Cursor: "c3"},
	}, Next: "c3", Building: true, Incomplete: true}
	var passed []any
	tools := tools{uc: UseCases{Grep: grepFunc(func(id uuid.UUID, r app.GrepRequest) (app.Grepped, error) {
		passed = []any{id, r}
		return g, nil
	})}}
	notebook, glob, ignore, limit, cursor := uuid.NewV7(), "B/**", true, 3, "c0"
	res, err := tools.grep(context.Background(), mcpserver.Call{Notebook: notebook}, grepIn{Pattern: "needle", Glob: &glob,
		IgnoreCase: &ignore, Limit: &limit, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := passed[1].(app.GrepRequest); passed[0] != notebook || !ok || r.Pattern != "needle" || r.Glob != &glob || !r.IgnoreCase ||
		r.Limit != &limit || r.Cursor != &cursor {
		t.Errorf("grep passed %v", passed)
	}
	want := `{"hits":[{"path":"A.md","line":1,"text":"a needle","column":3},` +
		`{"path":"B/C.md","line":9,"text":"needle cut","column":4000,"cut":true}],` +
		`"partial_lines":[{"path":"A.md","line":4}],"next_cursor":"c3","building":true,"incomplete":true}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	g = app.Grepped{Found: []app.GrepFound{{Path: "A.md", Line: 2, Hit: &domain.GrepHit{Text: "needle"}, Cursor: "c1"}}}
	res, err = tools.grep(context.Background(), mcpserver.Call{}, grepIn{Pattern: "needle"})
	if raw := rawOf(t, res); err != nil || raw != `{"hits":[{"path":"A.md","line":2,"text":"needle"}],"partial_lines":[],`+
		`"next_cursor":null,"building":false,"incomplete":false}` {
		t.Errorf("a last page = %s, %v", raw, err)
	}
	if r, _ := passed[1].(app.GrepRequest); r.IgnoreCase || r.Glob != nil || r.Limit != nil || r.Cursor != nil {
		t.Errorf("grep without its optional arguments passed %+v", r)
	}
	g = app.Grepped{}
	res, err = tools.grep(context.Background(), mcpserver.Call{}, grepIn{Pattern: "needle"})
	if raw := rawOf(t, res); err != nil || raw != `{"hits":[],"partial_lines":[],"next_cursor":null,"building":false,"incomplete":false}` {
		t.Errorf("no lines = %s, %v", raw, err)
	}
	refused := shared.Invalid(shared.FieldError{Field: "pattern", Code: "invalid_format", Message: "invalid_format"})
	tools.uc.Grep = grepFunc(func(uuid.UUID, app.GrepRequest) (app.Grepped, error) { return app.Grepped{}, refused })
	if _, err := tools.grep(context.Background(), mcpserver.Call{}, grepIn{Pattern: "("}); !errors.Is(err, refused) {
		t.Errorf("a refused pattern = %v, want the use case's error", err)
	}
}

// worstGrepLine is grep's largest line (M9/P4 design 3.10.1, review A-n6):
// a path of ten pages and a page, each title 255 bytes of "&", 2,818
// bytes, six in JSON each; a hit of 1,000 bytes of C0 controls, six each;
// its cursor of the path's key, each "&" a JSON escape before base64.
func worstGrepLine(t *testing.T, line int) app.GrepFound {
	t.Helper()
	title := strings.Repeat("&", 255)
	path := strings.Repeat(title+"/", 10) + title + ".md"
	cursor, err := domain.GrepCursor{K: path, N: line + 1}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return app.GrepFound{Path: path, Line: line, Hit: &domain.GrepHit{Text: strings.Repeat("\x01", domain.MaxGrepLineText),
		Column: domain.MaxContentBytes, Cut: true}, Cursor: cursor}
}

// An answer holds as many lines as it can (M9/P4 design 3.10.1): the
// largest line fits alone, at a line number of seven digits; 500 of them are cut at the bound, next_cursor
// the last kept's, incomplete false although the walk said true, as an
// answer cut short is not incomplete (mutant 170); every line kept, the
// walk's incomplete and next_cursor stand.
func TestTheGrepFitsItsAnswer(t *testing.T) {
	one := worstGrepLine(t, domain.MaxContentBytes) // the last line of the largest content's most
	tools := tools{uc: UseCases{Grep: grepFunc(func(uuid.UUID, app.GrepRequest) (app.Grepped, error) {
		return app.Grepped{Found: []app.GrepFound{one}, Next: one.Cursor, Incomplete: true}, nil
	})}}
	res, err := tools.grep(context.Background(), mcpserver.Call{}, grepIn{Pattern: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var a grepAnswer
	raw := rawOf(t, res)
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	n := mcpserver.TextBytes(raw)
	t.Logf("the largest line: %d bytes of %d, its cursor %d", n, mcpserver.MaxAnswerBytes, len(one.Cursor))
	if len(a.Hits) != 1 || !a.Incomplete || a.NextCursor == nil || *a.NextCursor != one.Cursor || n > mcpserver.MaxAnswerBytes {
		t.Errorf("the largest line alone: %d hits, incomplete %v, next %v, %d bytes", len(a.Hits), a.Incomplete, a.NextCursor, n)
	}
	found := make([]app.GrepFound, domain.MaxGrepLimit)
	for i := range found {
		found[i] = worstGrepLine(t, i+1)
	}
	tools.uc.Grep = grepFunc(func(uuid.UUID, app.GrepRequest) (app.Grepped, error) {
		return app.Grepped{Found: found, Next: "after", Incomplete: true}, nil
	})
	if res, err = tools.grep(context.Background(), mcpserver.Call{}, grepIn{Pattern: "x"}); err != nil {
		t.Fatal(err)
	}
	raw = rawOf(t, res)
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	k := len(a.Hits)
	if n := mcpserver.TextBytes(raw); k == 0 || k >= len(found) || n > mcpserver.MaxAnswerBytes {
		t.Fatalf("%d lines in %d bytes, want some, cut at %d", k, n, mcpserver.MaxAnswerBytes)
	}
	if a.Incomplete || a.NextCursor == nil || *a.NextCursor != found[k-1].Cursor || a.Hits[k-1].Line != k {
		t.Errorf("cut at %d lines: incomplete %v, next %v; want false and the %dth's cursor", k, a.Incomplete, a.NextCursor, k)
	}
}
