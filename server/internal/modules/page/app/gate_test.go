package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The MCP entry's gate decides as listNodes does, unlocked: the
// notebook's workspace, then node.list on it.
func TestTheNotebookGateDecidesAsTheTreeDoes(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	if err := app.NewNotebookGate(f.notebooks, f.auth).Execute(f.asAlice(), f.eng); err != nil {
		t.Fatalf("the gate of a notebook alice sees = %v, want nil", err)
	}
	if !slices.Equal(f.rec.calls, []string{"WorkspaceOf", "Authorize node.list"}) {
		t.Errorf("calls = %v, want the workspace and the decision, outside a transaction", f.rec.calls)
	}
	if f.auth.targets[0] != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
		t.Errorf("decided on %+v, want acme's eng", f.auth.targets[0])
	}
}

// A notebook that does not exist, or the caller cannot see, is
// notebook.not_found; the zero id, an entry's address that is no UUID, is
// answered without a read; without an actor the gate is a wiring fault.
func TestTheNotebookGateOfANotebookNotSeen(t *testing.T) {
	f := newFixture()
	gate := app.NewNotebookGate(f.notebooks, f.auth)
	for _, id := range []uuid.UUID{uuid.NewV7(), f.eng} {
		if err := gate.Execute(f.asAlice(), id); codeOf(err) != "notebook.not_found" {
			t.Errorf("the gate of %v = %v, want notebook.not_found", id, err)
		}
	}
	f.rec.calls = nil
	if err := gate.Execute(f.asAlice(), uuid.UUID{}); codeOf(err) != "notebook.not_found" || len(f.rec.calls) != 0 {
		t.Errorf("the gate of the zero id = %v after %v, want notebook.not_found without a read", err, f.rec.calls)
	}
	if err := gate.Execute(context.Background(), f.eng); codeOf(err) != "unauthorized" {
		t.Errorf("the gate without an actor = %v, want unauthorized", err)
	}
}

// A failure reading the notebook's workspace is answered as it is, not as
// the notebook not found: by the gate, list and read alike, which decide
// the notebook by one helper.
func TestTheNotebooksDecisionAnswersAFailure(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList, domain.ActionRead)
	f.page("Guide", nil, 0)
	boom := errors.New("boom")
	f.notebooks.err = boom
	for name, decide := range map[string]func() error{
		"the gate": func() error { return app.NewNotebookGate(f.notebooks, f.auth).Execute(f.asAlice(), f.eng) },
		"list": func() error {
			_, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "**", nil, nil)
			return err
		},
		"read": func() error {
			_, err := app.NewReadPath(f.notebooks, f.store, f.store, f.auth).Execute(f.asAlice(), f.eng, "Guide.md", nil, nil, nil,
				textMeasure(100))
			return err
		},
	} {
		if err := decide(); !errors.Is(err, boom) {
			t.Errorf("%s with WorkspaceOf failing = %v, want its failure", name, err)
		}
	}
}
