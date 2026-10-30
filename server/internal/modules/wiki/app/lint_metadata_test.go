package app

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintMetadataBundle(t *testing.T) {
	for _, failure := range []string{"partial-error", "cancelled-snapshot"} {
		t.Run(failure, func(t *testing.T) {
			deps, trace, _, _ := lintExecFixture(t)
			n := deps.Notebooks.(lintExecNotebooks)
			n.state = func(context.Context, uuid.UUID) (NotebookState, bool, error) {
				return NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: lintExecID(2)}, true, nil
			}
			deps.Notebooks = n
			var cancel context.CancelFunc
			deps.Snapshots = lintExecSnapshots(func(ctx context.Context, fn func(context.Context) error) error {
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				return fn(ctx)
			})
			readError := errors.New("bundle read failed")
			deps.Sources = lintExecSources(func(context.Context, uuid.UUID, uuid.UUID) (SourceData, error) {
				trace.add("sources")
				data := SourceData{Sources: []SourceNode{{ID: lintExecID(3), Kind: domain.NodePage}},
					Nodes: []CompactNode{{ID: lintExecID(3), Kind: domain.NodePage, NameKey: "partial"}}}
				if failure == "partial-error" {
					return data, readError
				}
				cancel()
				return data, nil
			})
			got, err := NewLint(deps).Execute(lintExecActor(t.Context(), 10), LintScope{NotebookID: lintExecID(1)}, LintQuery{})
			want := readError
			if failure == "cancelled-snapshot" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || !reflect.DeepEqual(got, domain.LintResult{}) {
				t.Fatalf("bundle error exposed a result or lost cause: result=%+v error=%v; want %v", got, err, want)
			}
			calls := trace.copy()
			if !slices.Contains(calls, "sources") || slices.Contains(calls, "nodes") || slices.Contains(calls, "prepare") || slices.Contains(calls, "paths") {
				t.Fatalf("failed bundle reached later consumers: %v", calls)
			}
		})
	}
}
