package page

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// agentsOf is a notebook whose root page AGENTS holds content, at revision
// 1 and with no frontmatter, for the instructions' use case: the other
// reads are none of its.
type agentsOf struct {
	app.Nodes
	app.Markdown
	node    uuid.UUID
	content string
}

func (a agentsOf) AgentsPage(context.Context, uuid.UUID, string) (uuid.UUID, domain.Kind, string, int, bool, error) {
	return a.node, domain.KindPage, domain.AgentsTitle, 1, true, nil
}

func (a agentsOf) PageContent(context.Context, uuid.UUID) (app.PageContent, error) {
	return app.PageContent{Content: a.content, Revision: 1}, nil
}

func (agentsOf) FrontmatterEnd(string) int { return 0 }

// entryStates is the notebooks' state read of a handshake.
type entryStates struct {
	state domain.NotebookState
	found bool
	err   error
}

func (s entryStates) StateOf(context.Context, uuid.UUID) (domain.NotebookState, bool, error) {
	return s.state, s.found, s.err
}

// measured is a body that the handshake's text counts n bytes of: lines of
// "<", "漢" and "x", padded with "x".
func measured(n int) string {
	s := strings.Repeat("<漢 x\n", n/16)
	return s + strings.Repeat("x", n-mcpserver.TextBytes(s))
}

// The handshake's instructions hold an AGENTS page whole up to
// MaxInstructionsBytes as its text counts them, 32,768, and cut it one
// byte past, with the note, within the bound (M9/P1 design 3.15.2, 6.2,
// S3 review A-M1).
func TestTheHandshakesInstructionsAreWholeUpToTheirBound(t *testing.T) {
	const note = `(The rest of AGENTS.md is cut here: read it whole with read, path "AGENTS.md".)`
	for _, n := range []int{domain.MaxInstructionsBytes, domain.MaxInstructionsBytes + 1} {
		body := measured(n)
		if mcpserver.TextBytes(body) != n {
			t.Fatalf("a body of %d bytes measured %d", n, mcpserver.TextBytes(body))
		}
		page := agentsOf{node: uuid.NewV7(), content: body}
		entry := MCPEntry{notebooks: entryStates{found: true}, instructions: app.NewInstructions(page, page, page, mcpserver.TextBytes)}
		got, err := entry.Instructions(context.Background(), uuid.NewV7())
		switch {
		case err != nil:
			t.Fatalf("%d bytes: %v", n, err)
		case mcpserver.TextBytes(got) > domain.MaxInstructionsBytes:
			t.Errorf("%d bytes: the instructions are %d", n, mcpserver.TextBytes(got))
		case n <= domain.MaxInstructionsBytes && got != body:
			t.Errorf("%d bytes: cut to %d bytes, want the body whole", n, mcpserver.TextBytes(got))
		case n > domain.MaxInstructionsBytes &&
			(!strings.HasSuffix(got, "\n\n"+note) || !strings.HasPrefix(body, strings.TrimSuffix(got, "\n\n"+note))):
			t.Errorf("%d bytes: %d bytes not a prefix of the body and the note", n, mcpserver.TextBytes(got))
		}
	}
}

// The handshake's own failures (M10/P5 design 3.7): a notebook deleted
// since the Gate is notebook.not_found, never read as a standard one; a
// read that fails keeps its cause; a standard notebook is not asked for
// its root, and an LLM Wiki answers the system paragraph of its entry.
func TestMCPEntryAnswersItsState(t *testing.T) {
	failure := errors.New("the database is gone")
	page := agentsOf{node: uuid.NewV7(), content: "# Rules"}
	entry := func(states entryStates) MCPEntry {
		return MCPEntry{notebooks: states, instructions: app.NewInstructions(page, page, page, mcpserver.TextBytes)}
	}
	if _, err := entry(entryStates{}).Instructions(context.Background(), uuid.NewV7()); !errors.Is(err, domain.ErrNotebookNotFound) {
		t.Errorf("a notebook gone since the gate = %v, want notebook.not_found", err)
	}
	if _, err := entry(entryStates{err: failure}).Instructions(context.Background(), uuid.NewV7()); !errors.Is(err, failure) {
		t.Errorf("a state that fails = %v, want its cause", err)
	}
	// agentsOf's Nodes is nil: a read of the root would panic.
	if got, err := entry(entryStates{found: true, state: domain.NotebookState{Mode: domain.ModeStandard}}).Instructions(context.Background(), uuid.NewV7()); err != nil || got != "# Rules" {
		t.Errorf("a standard notebook = %q, %v", got, err)
	}
	root := rootOf{agentsOf: page, title: "raw", line: []domain.Ancestor{{Name: "sources"}}}
	wiki := MCPEntry{notebooks: entryStates{found: true, state: domain.NotebookState{Mode: domain.ModeLLMWiki}},
		instructions: app.NewInstructions(root, root, root, mcpserver.TextBytes)}
	got, err := wiki.Instructions(context.Background(), uuid.NewV7())
	want := "This notebook is an LLM Wiki: pages under `sources/raw/` are sources you add once and never change; index.md and log.md are generated; " +
		"run the prompts ingest, query and lint.\n\n# Rules"
	if err != nil || got != want {
		t.Errorf("an LLM Wiki = %q, %v; want %q", got, err, want)
	}
}

// The handshake's reads share no transaction (M10/P5 design 3.7, 7.2):
// each outcome of a change between them lasts one answer. A downgrade
// after the state was read answers one paragraph more; a move and a rename
// between the root's two reads, one path that never was; a root gone
// between them, an error no problem names, which mcpserver answers as a
// fault and logs at error.
func TestTheInstructionsShareNoTransaction(t *testing.T) {
	entry := func(root rootOf, mode domain.Mode) MCPEntry {
		return MCPEntry{notebooks: entryStates{found: true, state: domain.NotebookState{Mode: mode}},
			instructions: app.NewInstructions(root, root, root, mcpserver.TextBytes)}
	}
	root := rootOf{agentsOf: agentsOf{node: uuid.NewV7(), content: "# Rules"}, title: "raw"}

	// One entry, its state read first says LLM Wiki, the next standard.
	downgraded := entry(root, domain.ModeLLMWiki)
	downgraded.notebooks = &statesInTurn{turns: []domain.Mode{domain.ModeLLMWiki, domain.ModeStandard}}
	if got, err := downgraded.Instructions(context.Background(), uuid.NewV7()); err != nil || !strings.HasPrefix(got, "This notebook is an LLM Wiki: pages under `raw/`") {
		t.Errorf("a downgrade after the state = %.80q, %v; want one paragraph more", got, err)
	}
	if got, err := downgraded.Instructions(context.Background(), uuid.NewV7()); err != nil || got != "# Rules" {
		t.Errorf("the next handshake = %q, %v", got, err)
	}

	mixed := root
	mixed.title, mixed.line = "new", []domain.Ancestor{{Name: "old"}}
	if got, err := entry(mixed, domain.ModeLLMWiki).Instructions(context.Background(), uuid.NewV7()); err != nil || !strings.Contains(got, "pages under `old/new/`") {
		t.Errorf("a move between the reads = %.80q, %v; want the two reads joined", got, err)
	}

	for _, failure := range []error{app.ErrNotFound, errors.New("the database is gone")} {
		gone := root
		gone.err = failure
		_, err := entry(gone, domain.ModeLLMWiki).Instructions(context.Background(), uuid.NewV7())
		if !errors.Is(err, failure) {
			t.Fatalf("a root's read failing with %v = %v, want that error", failure, err)
		}
		if a := httpserver.ProblemOf(context.Background(), err); a.Fault != httpserver.FaultUnknown {
			t.Errorf("%v is classified %v, want an unknown fault: an internal error, logged", failure, a.Fault)
		}
	}
}

// statesInTurn answers each mode of turns in turn.
type statesInTurn struct{ turns []domain.Mode }

func (s *statesInTurn) StateOf(context.Context, uuid.UUID) (domain.NotebookState, bool, error) {
	mode := s.turns[0]
	s.turns = s.turns[1:]
	return domain.NotebookState{Mode: mode}, true, nil
}

// rootOf is agentsOf of a notebook whose source root is titled title,
// under line; err is its point read's.
type rootOf struct {
	agentsOf
	title string
	line  []domain.Ancestor
	err   error
}

func (r rootOf) FindNodeIn(context.Context, uuid.UUID, uuid.UUID) (domain.Node, error) {
	return domain.Node{Name: r.title, Kind: domain.KindPage}, r.err
}

func (r rootOf) Ancestors(context.Context, uuid.UUID) ([]domain.Ancestor, error) {
	return r.line, nil
}
