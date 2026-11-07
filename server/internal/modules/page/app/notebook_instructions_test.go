package app_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// An LLM Wiki's instructions name its source root as it is now, by a point
// read and one of the ancestors, after the AGENTS page; a standard notebook
// reads neither; each read's error keeps its cause (M10/P5 design 3.7).
func TestInstructionsReadTheSourceRoot(t *testing.T) {
	f := newFixture()
	folder := f.page("library", nil, 0)
	shelf := f.page("shelf", &folder.ID, 0)
	raw := f.page("raw", &shelf.ID, 0)
	f.write(f.page("AGENTS", nil, 1), "# Rules", 1)
	wiki := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}

	f.rec.calls = nil
	got, err := f.instructionsOf().Read(f.asAlice(), f.eng, wiki)
	want := app.NotebookInstructions{NotebookID: f.eng, Mode: domain.ModeLLMWiki, Agents: domain.Agents{Body: "# Rules", Title: "AGENTS"},
		HasAgents: true, SourceRootPath: "library/shelf/raw", SourceRootTitle: "raw"}
	if err != nil || got != want {
		t.Errorf("Read() = %+v, %v; want %+v", got, err, want)
	}
	calls := []string{"AgentsPage", "PageContent", "FrontmatterEnd", "FindNodeIn", "Ancestors"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}

	f.rec.calls = nil
	got, err = f.instructionsOf().Read(f.asAlice(), f.eng, domain.NotebookState{Mode: domain.ModeStandard})
	if err != nil || got.SourceRootPath != "" || got.SourceRootTitle != "" || got.Mode != domain.ModeStandard || !got.HasAgents {
		t.Errorf("a standard notebook's Read() = %+v, %v", got, err)
	}
	if slices.Contains(f.rec.calls, "FindNodeIn") || slices.Contains(f.rec.calls, "Ancestors") || slices.Contains(f.rec.calls, "ListNodes") {
		t.Errorf("a standard notebook's calls = %q, want no read of a root", f.rec.calls)
	}

	failure := errors.New("the database is gone")
	for _, port := range []string{"AgentsPage", "FindNodeIn", "Ancestors"} {
		f.store.errs = map[string]error{port: failure}
		if _, err := f.instructionsOf().Read(f.asAlice(), f.eng, wiki); !errors.Is(err, failure) {
			t.Errorf("%s failing: Read() = %v, want its cause", port, err)
		}
	}
	f.store.errs = nil

	// A root deleted between the reads answers the read's error: not the
	// built-in text, and not an LLM Wiki without its paragraph.
	if _, err := f.instructionsOf().Read(f.asAlice(), f.eng, domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7()}); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("a root gone = %v, want app.ErrNotFound", err)
	}
}
