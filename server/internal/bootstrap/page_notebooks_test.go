package bootstrap

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// states is the notebook module's Notebooks answering StateOf from a table,
// or err; the test calls nothing else of it.
type states struct {
	notebook.Notebooks
	of  map[uuid.UUID]notebook.State
	err error
}

func (s states) StateOf(_ context.Context, id uuid.UUID) (notebook.State, bool, error) {
	if s.err != nil {
		return notebook.State{}, false, s.err
	}
	st, ok := s.of[id]
	return st, ok, nil
}

// pageNotebooks converts the notebook module's state into the page
// module's field by field (M10/P1 design 3.12): an LLM Wiki's mode and
// source root, a standard notebook's mode and none; a notebook not found,
// as one deleted after open_notebook's gate is, is not found, with the zero
// state, never a found state of no mode; an error is answered as it is.
func TestPageNotebooksConvertTheState(t *testing.T) {
	wiki, plain, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	root := uuid.NewV7()
	n := pageNotebooks{states{of: map[uuid.UUID]notebook.State{
		wiki:  {Mode: notebook.ModeLLMWiki, SourceRootID: root},
		plain: {Mode: notebook.ModeStandard},
	}}}
	for _, tt := range []struct {
		name string
		id   uuid.UUID
		want page.NotebookState
		ok   bool
	}{
		{"an LLM Wiki", wiki, page.NotebookState{Mode: page.ModeLLMWiki, SourceRootID: root}, true},
		{"a standard notebook", plain, page.NotebookState{Mode: page.ModeStandard}, true},
		{"none", gone, page.NotebookState{}, false},
	} {
		if got, ok, err := n.StateOf(context.Background(), tt.id); err != nil || ok != tt.ok || got != tt.want {
			t.Errorf("%s: StateOf() = %+v, %v, %v; want %+v, %v", tt.name, got, ok, err, tt.want, tt.ok)
		}
	}
	failure := errors.New("the database is gone")
	if got, ok, err := (pageNotebooks{states{err: failure}}).StateOf(context.Background(), wiki); !errors.Is(err, failure) || ok ||
		got != (page.NotebookState{}) {
		t.Errorf("a failing read: StateOf() = %+v, %v, %v; want its error", got, ok, err)
	}
}
