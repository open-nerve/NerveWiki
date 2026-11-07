package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// playbookCalls is what a get reached, in its order.
type playbookCalls []string

// playbooksOf is the use case over ports that record their calls and
// answer as the arguments say.
func playbooksOf(calls *playbookCalls, found bool, mode domain.Mode, stateFound bool, auth error) *Playbooks {
	workspace := lintExecID(1)
	return NewPlaybooks(lintExecNotebooks{
		workspace: func(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
			*calls = append(*calls, "workspace")
			return workspace, found, nil
		},
		state: func(context.Context, uuid.UUID) (NotebookState, bool, error) {
			*calls = append(*calls, "state")
			return NotebookState{Mode: mode}, stateFound, nil
		},
	}, lintExecAuth(func(_ context.Context, _ shared.Actor, action shared.Action, target shared.Target) (shared.Grant, error) {
		*calls = append(*calls, "authorize "+string(action))
		if target.WorkspaceID != workspace || target.NotebookID != lintExecID(2) {
			return shared.Grant{}, fmt.Errorf("the target %+v", target)
		}
		return shared.Grant{}, auth
	}))
}

// samePlaybookError tells that got is want: shared's constructors build a
// new error each call, so those compare by their codes.
func samePlaybookError(got, want error) bool {
	var g, w *shared.Error
	if errors.As(got, &g) && errors.As(want, &w) {
		return g.Code == w.Code
	}
	return errors.Is(got, want)
}

// A playbook's text is the reader's of an LLM Wiki alone: each refusal
// comes where lint's does, before the next port is asked.
func TestPlaybooksAuthorizeAndRequireMode(t *testing.T) {
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: lintExecID(3), SessionID: lintExecID(4)})
	failure := errors.New("the database is gone")
	for _, tt := range []struct {
		name       string
		ctx        context.Context
		found      bool
		stateFound bool
		mode       domain.Mode
		auth       error
		want       error
		calls      playbookCalls
	}{
		{name: "no actor", ctx: context.Background(), found: true, want: shared.Unauthenticated()},
		{name: "no notebook", ctx: ctx, want: domain.ErrNotebookNotFound, calls: playbookCalls{"workspace"}},
		{name: "not visible", ctx: ctx, found: true, auth: fmt.Errorf("wrapped: %w", shared.ErrNotVisible), want: domain.ErrNotebookNotFound,
			calls: playbookCalls{"workspace", "authorize lint.read"}},
		{name: "forbidden", ctx: ctx, found: true, auth: shared.Forbidden(), want: shared.Forbidden(),
			calls: playbookCalls{"workspace", "authorize lint.read"}},
		{name: "an authorizer that fails", ctx: ctx, found: true, auth: failure, want: failure,
			calls: playbookCalls{"workspace", "authorize lint.read"}},
		{name: "gone since", ctx: ctx, found: true, want: domain.ErrNotebookNotFound,
			calls: playbookCalls{"workspace", "authorize lint.read", "state"}},
		{name: "standard", ctx: ctx, found: true, stateFound: true, mode: domain.ModeStandard, want: domain.ErrNotLLMWiki,
			calls: playbookCalls{"workspace", "authorize lint.read", "state"}},
		{name: "LLM Wiki", ctx: ctx, found: true, stateFound: true, mode: domain.ModeLLMWiki,
			calls: playbookCalls{"workspace", "authorize lint.read", "state"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls playbookCalls
			text, err := playbooksOf(&calls, tt.found, tt.mode, tt.stateFound, tt.auth).Execute(tt.ctx, lintExecID(2), domain.PlaybookLint, nil)
			if !samePlaybookError(err, tt.want) {
				t.Errorf("Execute() = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(calls, tt.calls) {
				t.Errorf("the calls = %v, want %v", calls, tt.calls)
			}
			if (text != "") != (tt.want == nil) {
				t.Errorf("Execute() text = %.60q with the error %v", text, err)
			}
		})
	}

	t.Run("a workspace read that fails", func(t *testing.T) {
		p := NewPlaybooks(lintExecNotebooks{
			workspace: func(context.Context, uuid.UUID) (uuid.UUID, bool, error) { return uuid.UUID{}, false, failure },
		}, lintExecAuth(func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
			return shared.Grant{}, nil
		}))
		if _, err := p.Execute(ctx, lintExecID(2), domain.PlaybookLint, nil); !errors.Is(err, failure) {
			t.Errorf("Execute() = %v, want its cause", err)
		}
	})

	t.Run("a state that fails", func(t *testing.T) {
		p := NewPlaybooks(lintExecNotebooks{
			workspace: func(context.Context, uuid.UUID) (uuid.UUID, bool, error) { return lintExecID(1), true, nil },
			state: func(context.Context, uuid.UUID) (NotebookState, bool, error) {
				return NotebookState{}, false, failure
			},
		}, lintExecAuth(func(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
			return shared.Grant{}, nil
		}))
		if _, err := p.Execute(ctx, lintExecID(2), domain.PlaybookLint, nil); !errors.Is(err, failure) {
			t.Errorf("Execute() = %v, want its cause", err)
		}
	})

	var calls playbookCalls
	p := playbooksOf(&calls, true, domain.ModeLLMWiki, true, nil)
	texts := []struct {
		kind      domain.Playbook
		first     string
		lead      string
		sentences []string
	}{
		{domain.PlaybookIngest, "Ingest a source into this LLM Wiki.", "Source path supplied by the caller:\n", []string{
			"call pending_sources", "until next_offset is null", `"ingest: <source path>"`, `sources: ["[[raw/Note.md]]"]`, "Quote a summary that holds a colon.",
			"Leave them empty", "until next_column is null", "partial_lines have no column", "changeset_id", "invalid_frontmatter or invalid_sources",
		}},
		{domain.PlaybookQuery, "Answer a question from this LLM Wiki.", "Question supplied by the caller:\n", []string{
			"grep's incomplete or building", "until next_column is null", "create one only when the requested work asks for it",
		}},
		{domain.PlaybookLint, "Check and maintain this LLM Wiki.", "", []string{
			"stale_pages", "count_capped", "target_cut", "missing_type, missing_summary and invalid_sources", "leave it empty", "with changeset", "until next_column is null",
		}},
	}
	for _, tt := range texts {
		t.Run(string(tt.kind), func(t *testing.T) {
			text, err := p.Execute(ctx, lintExecID(2), tt.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			if first, _, _ := strings.Cut(text, "\n"); first != tt.first {
				t.Errorf("the first line = %q, want %q", first, tt.first)
			}
			if len(text) > domain.MaxPlaybookBytes {
				t.Errorf("the text is %d bytes, over %d", len(text), domain.MaxPlaybookBytes)
			}
			if text != strings.TrimSpace(text) {
				t.Error("the text has white space around it")
			}
			for _, s := range tt.sentences {
				if !strings.Contains(text, s) {
					t.Errorf("the text does not say %q", s)
				}
			}
			if tt.lead == "" {
				return
			}
			if strings.Contains(text, tt.lead) {
				t.Errorf("no argument, yet the text says %q", tt.lead)
			}
			for _, argument := range []string{"  raw/A b.md\t", "", "line one\nline two <&>"} {
				got, err := p.Execute(ctx, lintExecID(2), tt.kind, &argument)
				if err != nil {
					t.Fatal(err)
				}
				if want := text + "\n\n" + tt.lead + argument; got != want {
					t.Errorf("with %q the text ends %q, want the argument as it is, once", argument, got[len(text):])
				}
			}
		})
	}
}
