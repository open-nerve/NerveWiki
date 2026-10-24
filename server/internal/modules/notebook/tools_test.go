package notebook_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
)

// noAccountWorkspaces is an account of no workspace, for the tests that
// call no MCP tool.
type noAccountWorkspaces struct{}

func (noAccountWorkspaces) WorkspacesOf(context.Context, uuid.UUID) ([]notebook.AccountWorkspace, error) {
	return nil, nil
}

// noSeed writes nothing and answers no source root: the seeder of the
// tests that create no LLM Wiki.
type noSeed struct{}

func (noSeed) NotebookCreated(context.Context, notebook.NotebookCreation) (uuid.UUID, error) {
	return uuid.UUID{}, nil
}

// A module without the account's workspaces is a fault of its wiring:
// New panics, as page's New does without its ports (M9/P1 design 3.17.1).
func TestTheModuleRefusesNoAccountWorkspaces(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "no account workspaces") {
			t.Errorf("New without AccountWorkspaces panicked with %q", r)
		}
	}()
	notebook.New(notebook.Deps{})
}

// A module without a seeder is a fault of its wiring: an LLM Wiki would be
// created without its template (M10/P1 design 3.5).
func TestTheModuleRefusesNoSeeders(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "no seeders") {
			t.Errorf("New without Seeders panicked with %q", r)
		}
	}()
	notebook.New(notebook.Deps{AccountWorkspaces: noAccountWorkspaces{}})
}

// The module's MCP tools are notebooks, of the account entry alone.
func TestTheModulesToolIsNotebooks(t *testing.T) {
	tools := notebook.New(notebook.Deps{AccountWorkspaces: noAccountWorkspaces{}, Seeders: []notebook.Seeder{noSeed{}}}).Tools()
	if len(tools) != 1 || tools[0].Spec().Name != "notebooks" || tools[0].Spec().Notebook {
		t.Errorf("Tools() = %+v, want notebooks alone", tools)
	}
}
