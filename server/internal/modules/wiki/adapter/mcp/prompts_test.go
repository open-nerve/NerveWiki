package mcpadapter

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// playbooksSeen records a get: the kind, the notebook and the argument.
type playbooksSeen struct {
	kind     domain.Playbook
	notebook uuid.UUID
	argument *string
	err      error
}

func (p *playbooksSeen) Execute(_ context.Context, notebook uuid.UUID, kind domain.Playbook, argument *string) (string, error) {
	p.kind, p.notebook, p.argument = kind, notebook, argument
	return "the text", p.err
}

// upgradeSeen records the upgrade prompt's get: the notebook, how many
// times.
type upgradeSeen struct {
	notebook uuid.UUID
	gets     int
	err      error
}

func (u *upgradeSeen) Execute(_ context.Context, notebook uuid.UUID) (string, error) {
	u.notebook = notebook
	u.gets++
	return "the upgrade's text", u.err
}

// promptNamed is the prompt name of prompts, failing t when there is none.
func promptNamed(t *testing.T, prompts []mcpserver.Prompt, name string) mcpserver.Prompt {
	t.Helper()
	for _, p := range prompts {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no prompt %s", name)
	return mcpserver.Prompt{}
}

// The four prompts, each in the notebook entry alone, in the order of
// api/mcp/prompts.json (M11/P3 design 3.16): the LLM Wiki's three, then
// the upgrade's, which declares no argument.
func TestThePromptsAreTheFour(t *testing.T) {
	prompts := Prompts(&playbooksSeen{}, &upgradeSeen{})
	var names []string
	for _, p := range prompts {
		names = append(names, p.Name)
		if p.Entry != mcpserver.NotebookEntry {
			t.Errorf("%s is listed in %v, want the notebook entry alone", p.Name, p.Entry)
		}
	}
	if !slices.Equal(names, []string{"ingest", "lint", "query", "upgrade"}) {
		t.Errorf("the prompts = %v", names)
	}
	u := promptNamed(t, prompts, "upgrade")
	if u.Title != "Upgrade to an LLM Wiki" || u.Description != upgradePlaybookDescription || len(u.Arguments) != 0 {
		t.Errorf("the upgrade prompt = %q %q %v", u.Title, u.Description, u.Arguments)
	}
}

// The upgrade prompt's get hands its use case the call's notebook, once,
// whatever arguments came, and answers its text as the one message, the
// prompt's description with it; a refusal comes back as it is, with no
// message.
func TestTheUpgradePromptPassesItsNotebook(t *testing.T) {
	nb := uuid.UUID{7}
	seen := &upgradeSeen{}
	get := promptNamed(t, Prompts(&playbooksSeen{}, seen), "upgrade").Get
	description, messages, err := get(context.Background(), mcpserver.Call{Notebook: nb}, map[string]string{"source": "raw/A.md"})
	if err != nil || description != upgradePlaybookDescription || !slices.Equal(messages, []string{"the upgrade's text"}) {
		t.Errorf("Get() = %q, %q, %v", description, messages, err)
	}
	if seen.notebook != nb || seen.gets != 1 {
		t.Errorf("the use case had %v, %d times", seen.notebook, seen.gets)
	}
	failure := errors.New("refused")
	description, messages, err = promptNamed(t, Prompts(&playbooksSeen{}, &upgradeSeen{err: failure}), "upgrade").
		Get(context.Background(), mcpserver.Call{Notebook: nb}, nil)
	if !errors.Is(err, failure) || messages != nil || description != "" {
		t.Errorf("a refusal = %q, %q, %v; want the error, no message", description, messages, err)
	}
}

// A prompt hands its own argument to the use case, a pointer only when the
// caller gave it, and no other: lint declares none, so a key named "" is
// nobody's.
func TestThePromptsPassTheirArgument(t *testing.T) {
	nb := uuid.UUID{9}
	empty, source, question := "", "raw/A.md", "why?"
	for _, tt := range []struct {
		name, prompt string
		args         map[string]string
		kind         domain.Playbook
		want         *string
	}{
		{"a source", "ingest", map[string]string{"source": source}, domain.PlaybookIngest, &source},
		{"no source", "ingest", nil, domain.PlaybookIngest, nil},
		{"an empty source", "ingest", map[string]string{"source": ""}, domain.PlaybookIngest, &empty},
		{"another argument", "ingest", map[string]string{"question": question}, domain.PlaybookIngest, nil},
		{"a question", "query", map[string]string{"question": question}, domain.PlaybookQuery, &question},
		{"lint", "lint", nil, domain.PlaybookLint, nil},
		{"lint and a key that is empty", "lint", map[string]string{"": "x"}, domain.PlaybookLint, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			seen := &playbooksSeen{}
			for _, p := range Prompts(seen, &upgradeSeen{}) {
				if p.Name != tt.prompt {
					continue
				}
				description, messages, err := p.Get(context.Background(), mcpserver.Call{Notebook: nb}, tt.args)
				if err != nil || description != p.Description || len(messages) != 1 || messages[0] != "the text" {
					t.Errorf("Get() = %q, %q, %v", description, messages, err)
				}
			}
			switch {
			case seen.kind != tt.kind || seen.notebook != nb:
				t.Errorf("the use case had %q of %v", seen.kind, seen.notebook)
			case (seen.argument == nil) != (tt.want == nil):
				t.Errorf("the argument = %v, want %v", seen.argument, tt.want)
			case tt.want != nil && *seen.argument != *tt.want:
				t.Errorf("the argument = %q, want %q", *seen.argument, *tt.want)
			}
		})
	}
	failure := errors.New("refused")
	if _, messages, err := promptNamed(t, Prompts(&playbooksSeen{err: failure}, &upgradeSeen{}), "lint").Get(context.Background(), mcpserver.Call{}, nil); !errors.Is(err, failure) || messages != nil {
		t.Errorf("a refusal = %q, %v; want the error, no message", messages, err)
	}
}
