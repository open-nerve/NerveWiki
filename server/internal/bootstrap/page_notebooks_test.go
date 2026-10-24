package bootstrap

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// states is the notebook module's Notebooks answering StateOf, ShareState
// and LockState from a table, or err, each its own read: by names, the
// calls made. The test calls nothing else of it.
type states struct {
	notebook.Notebooks
	of    map[uuid.UUID]notebook.State
	err   error
	calls *[]string
}

func (s states) StateOf(_ context.Context, id uuid.UUID) (notebook.State, bool, error) {
	return s.read("StateOf", id)
}

func (s states) ShareState(_ context.Context, id uuid.UUID) (notebook.State, bool, error) {
	return s.read("ShareState", id)
}

func (s states) LockState(_ context.Context, id uuid.UUID) (notebook.State, bool, error) {
	return s.read("LockState", id)
}

func (s states) read(call string, id uuid.UUID) (notebook.State, bool, error) {
	*s.calls = append(*s.calls, call)
	if s.err != nil {
		return notebook.State{}, false, s.err
	}
	st, ok := s.of[id]
	return st, ok, nil
}

// pageNotebooks converts the notebook module's state into the page
// module's field by field (M10/P1 design 3.12, 3.18), unlocked and under
// each lock, each through the notebook module's read of its own: an LLM
// Wiki's mode and source root, a standard notebook's mode and none; a
// notebook not found, as one deleted after open_notebook's gate is, is not
// found, with the zero state, never a found state of no mode; an error is
// answered as it is.
func TestPageNotebooksConvertTheState(t *testing.T) {
	wiki, plain, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	root := uuid.NewV7()
	var calls []string
	n := pageNotebooks{states{of: map[uuid.UUID]notebook.State{
		wiki:  {Mode: notebook.ModeLLMWiki, SourceRootID: root},
		plain: {Mode: notebook.ModeStandard},
	}, calls: &calls}}
	failure := errors.New("the database is gone")
	failing := pageNotebooks{states{err: failure, calls: &calls}}
	reads := []struct {
		name, call string
		of         func(pageNotebooks) func(context.Context, uuid.UUID) (page.NotebookState, bool, error)
	}{
		{"StateOf", "StateOf", func(n pageNotebooks) func(context.Context, uuid.UUID) (page.NotebookState, bool, error) {
			return n.StateOf
		}},
		{"ShareByID", "ShareState", func(n pageNotebooks) func(context.Context, uuid.UUID) (page.NotebookState, bool, error) {
			return n.ShareByID
		}},
		{"LockByID", "LockState", func(n pageNotebooks) func(context.Context, uuid.UUID) (page.NotebookState, bool, error) {
			return n.LockByID
		}},
	}
	for _, r := range reads {
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
			calls = nil
			if got, ok, err := r.of(n)(context.Background(), tt.id); err != nil || ok != tt.ok || got != tt.want ||
				!slices.Equal(calls, []string{r.call}) {
				t.Errorf("%s: %s() = %+v, %v, %v by %v; want %+v, %v by %s", tt.name, r.name, got, ok, err, calls, tt.want, tt.ok, r.call)
			}
		}
		if got, ok, err := r.of(failing)(context.Background(), wiki); !errors.Is(err, failure) || ok || got != (page.NotebookState{}) {
			t.Errorf("a failing read: %s() = %+v, %v, %v; want its error", r.name, got, ok, err)
		}
	}
}
