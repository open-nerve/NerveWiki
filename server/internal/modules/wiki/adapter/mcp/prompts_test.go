package mcpadapter

import (
	"context"
	"errors"
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
			for _, p := range Prompts(seen) {
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
	if _, messages, err := Prompts(&playbooksSeen{err: failure})[0].Get(context.Background(), mcpserver.Call{}, nil); !errors.Is(err, failure) || messages != nil {
		t.Errorf("a refusal = %q, %v; want the error, no message", messages, err)
	}
}
