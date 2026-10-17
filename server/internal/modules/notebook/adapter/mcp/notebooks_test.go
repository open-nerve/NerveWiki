package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// listed is a ListAccountNotebooks of an answer.
type listed struct {
	answer app.AccountNotebooks
	err    error
}

func (l listed) Execute(context.Context) (app.AccountNotebooks, error) { return l.answer, l.err }

// call is the tool's answer to the use case's l, decoded.
func call(t *testing.T, l listed) (notebooksAnswer, string, error) {
	t.Helper()
	res, err := tools{notebooks: l}.list(context.Background(), mcpserver.Call{}, struct{}{})
	if err != nil {
		return notebooksAnswer{}, "", err
	}
	raw, _ := res.Structured.(json.RawMessage)
	var a notebooksAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("the answer %s: %v", raw, err)
	}
	return a, string(raw), nil
}

// notebookOf is a notebook named name of the workspace acme, as role.
func notebookOf(id uuid.UUID, name string, role shared.NotebookRole) app.AccountNotebook {
	return app.AccountNotebook{Notebook: domain.Notebook{ID: id, Name: name}, Role: role,
		Workspace: app.AccountWorkspace{Slug: "acme", Name: "Acme <&>"}}
}

// Each notebook comes with its id, its name, its workspace's slug and
// name, and the caller's role there, as they are; truncated false when all
// came (M9/P1 design 3.17.2).
func TestTheNotebooksAnswerTheirItems(t *testing.T) {
	id := uuid.MustParse("01920000-0000-7000-8000-0000000000b0")
	_, raw, err := call(t, listed{answer: app.AccountNotebooks{Notebooks: []app.AccountNotebook{notebookOf(id, "Handbook", shared.NotebookEditor)}}})
	want := `{"notebooks":[{"id":"01920000-0000-7000-8000-0000000000b0","name":"Handbook","workspace":{"slug":"acme","name":"Acme <&>"},` +
		`"role":"editor"}],"truncated":false}`
	if err != nil || raw != want {
		t.Errorf("answer = %s, %v; want %s", raw, err, want)
	}
	a, raw, err := call(t, listed{})
	if err != nil || raw != `{"notebooks":[],"truncated":false}` || len(a.Notebooks) != 0 {
		t.Errorf("no notebooks = %s, %v", raw, err)
	}
	if _, _, err := call(t, listed{err: errors.New("the database is gone")}); err == nil {
		t.Error("the use case's error was lost")
	}
}

// The answer holds as many notebooks as mcpserver's bound does, truncated
// when some are left out: by the answer, or by the use case's own bound.
func TestTheNotebooksFitTheirAnswer(t *testing.T) {
	var many []app.AccountNotebook
	for range 1000 {
		many = append(many, notebookOf(uuid.NewV7(), strings.Repeat(`"`, 255), shared.NotebookReader))
	}
	a, raw, err := call(t, listed{answer: app.AccountNotebooks{Notebooks: many}})
	switch {
	case err != nil:
		t.Fatal(err)
	case mcpserver.TextBytes(raw) > mcpserver.MaxAnswerBytes:
		t.Errorf("the answer is %d bytes, over the bound", mcpserver.TextBytes(raw))
	case !a.Truncated || len(a.Notebooks) < 10 || len(a.Notebooks) >= len(many):
		t.Errorf("%d notebooks, truncated %v; want some, at least 10, truncated", len(a.Notebooks), a.Truncated)
	}
	a, _, err = call(t, listed{answer: app.AccountNotebooks{Notebooks: many[:3], Truncated: true}})
	if err != nil || len(a.Notebooks) != 3 || !a.Truncated {
		t.Errorf("the use case's truncated list: %d, %v, %v; want 3 truncated", len(a.Notebooks), a.Truncated, err)
	}
}
