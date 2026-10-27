package domain_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Each case changes one input that can otherwise silently bypass the source
// contract. Expectations are literal, independent of SourceFacts' construction.
func TestSourceContractUsesEachFact(t *testing.T) {
	raw, page := uuid.NewV7(), uuid.NewV7()
	for _, tt := range []struct {
		name string
		edit func(*domain.NotebookState, *domain.StepFacts)
		deny bool
	}{
		{"nonempty source", nil, true},
		{"empty first write", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Content.BeforeBytes = 0 }, false},
		{"first creation", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Operation, s.Nodes[0].Before, s.Content.BeforeBytes = domain.OpCreate, nil, 0
		}, false},
		{"restore into source", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Operation, s.Nodes[0].Before, s.Content.BeforeBytes = domain.OpRestore, nil, 0
		}, false},
		{"system rewrite", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Rewrite = true }, false},
		{"rewrite undo", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Kind, s.Rewrite = domain.ChangesetRevert, true }, false},
		{"ordinary undo", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Kind = domain.ChangesetRevert }, true},
		{"version restore", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Kind, s.Content.BeforeBytes = domain.ChangesetVersionRestore, 0
		}, true},
		{"version restore of rewrite", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Kind, s.Rewrite, s.Content.BeforeBytes = domain.ChangesetVersionRestore, true, 0
		}, true},
		{"valid edit session", func(_ *domain.NotebookState, s *domain.StepFacts) { s.EditSession, s.Content.BeforeBytes = true, 0 }, true},
		{"session cannot use rewrite", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.EditSession, s.Rewrite, s.Content.BeforeBytes = true, true, 0
		}, true},
		{"merged second change", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Content.BeforeBytes, s.Content.Merged = 0, 2 }, true},
		{"one merged first change", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Content.BeforeBytes, s.Content.Merged = 0, 1 }, false},
		{"tree pass has no body", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Content = nil }, false},
		{"opening a session has no body", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Content, s.EditSession = nil, true }, true},
		{"asset has no page body", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Nodes[0].Kind = domain.KindAsset }, false},
		{"outside", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Nodes[0].Before.InArea, s.Nodes[0].After.InArea = false, false
		}, false},
		{"source root excluded even with marked facts", func(_ *domain.NotebookState, s *domain.StepFacts) { s.Nodes[0].ID = raw }, false},
		{"AGENTS body", func(_ *domain.NotebookState, s *domain.StepFacts) {
			s.Nodes[0].Before, s.Nodes[0].After = &domain.NodePlace{Key: "agents"}, &domain.NodePlace{Key: "agents"}
		}, false},
		{"standard", func(n *domain.NotebookState, _ *domain.StepFacts) { n.Mode = domain.ModeStandard }, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}
			s := domain.StepFacts{Operation: domain.OpContent, Kind: domain.ChangesetEdit,
				Nodes: []domain.NodeFacts{{ID: page, Kind: domain.KindPage,
					Before: &domain.NodePlace{ParentID: &raw, Key: "page", InArea: true},
					After:  &domain.NodePlace{ParentID: &raw, Key: "page", InArea: true}}},
				Content: &domain.ContentFacts{BeforeBytes: 4, Bytes: 3, Merged: 1}}
			if tt.edit != nil {
				tt.edit(&n, &s)
			}
			v := domain.CheckContract(n, s)
			if !tt.deny {
				if v != (domain.Verdict{}) {
					t.Fatalf("verdict = %+v, want permitted", v)
				}
				return
			}
			var problem *shared.Error
			if !errors.As(v.Err, &problem) || problem.Code != "page.source_immutable" || problem.Kind != shared.KindConflict || v.NodeID != page || v.Needs != "" {
				t.Fatalf("verdict = %+v, want source_immutable conflict of page %s without an admin exception", v, page)
			}
		})
	}
}

// An existing source node, including an attachment, needs manage_source.
// New arrivals do not, and a source root's outside ancestor keeps P1's
// rename/reorder exception even when its facts include source descendants.
func TestSourceContractManagementNamesTheChangedNode(t *testing.T) {
	raw, page, outside, parent := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	n := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}
	source := &domain.NodePlace{ParentID: &raw, Key: "page", InArea: true}
	root := &domain.NodePlace{Key: "raw"}
	outer := &domain.NodePlace{Key: "outside"}
	for _, kind := range []domain.Kind{domain.KindPage, domain.KindAsset} {
		for _, tt := range []struct {
			name          string
			op            domain.Operation
			before, after *domain.NodePlace
			needs         bool
		}{
			{"rename", domain.OpRename, source, source, true},
			{"reorder", domain.OpMove, source, source, true},
			{"move out", domain.OpMove, source, outer, true},
			{"move within", domain.OpMove, source, &domain.NodePlace{ParentID: &parent, Key: "page", InArea: true}, true},
			{"delete", domain.OpDelete, source, nil, true},
			{"move in", domain.OpMove, outer, source, false},
			{"create in", domain.OpCreate, nil, source, false},
			{"restore in", domain.OpRestore, nil, source, false},
		} {
			t.Run(string(kind)+" "+tt.name, func(t *testing.T) {
				v := domain.CheckContract(n, domain.StepFacts{Operation: tt.op, Nodes: []domain.NodeFacts{{ID: page, Kind: kind, Before: tt.before, After: tt.after}}})
				want := domain.Verdict{}
				if tt.needs {
					want = domain.Verdict{Needs: domain.ActionManageSource, NodeID: page}
				}
				if v != want {
					t.Fatalf("verdict = %+v, want %+v", v, want)
				}
			})
		}
	}
	for _, tt := range []struct {
		name  string
		op    domain.Operation
		after *domain.NodePlace
		needs bool
	}{
		{"ancestor rename", domain.OpRename, outer, false},
		{"ancestor reorder", domain.OpMove, outer, false},
		{"ancestor changes parent", domain.OpMove, &domain.NodePlace{ParentID: &parent, Key: "outside"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := domain.CheckContract(n, domain.StepFacts{Operation: tt.op, Nodes: []domain.NodeFacts{
				{ID: outside, Kind: domain.KindPage, Before: outer, After: tt.after},
				{ID: raw, Kind: domain.KindPage, Before: root, After: root},
				{ID: page, Kind: domain.KindPage, Before: source, After: source},
			}})
			want := domain.Verdict{}
			if tt.needs {
				want = domain.Verdict{Needs: domain.ActionManageSource, NodeID: raw}
			}
			if v != want {
				t.Fatalf("verdict = %+v, want %+v", v, want)
			}
		})
	}
}

// Conflicting facts deliberately exercise the global code order across nodes,
// so a per-node early admin decision cannot hide a later protected violation.
func TestSourceContractKeepsGlobalCodeOrder(t *testing.T) {
	raw, page, other := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	n := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}
	for _, tt := range []struct {
		name string
		op   domain.Operation
		tail domain.NodeFacts
		code string
		id   uuid.UUID
	}{
		{"protected before immutable and admin", domain.OpDelete, domain.NodeFacts{ID: raw, Before: &domain.NodePlace{Key: "raw"}}, "page.protected", raw},
		{"virtual before immutable and admin", domain.OpMove, domain.NodeFacts{ID: other, Before: &domain.NodePlace{ParentID: &raw}, After: &domain.NodePlace{Key: "index"}}, "page.virtual_file", other},
		{"immutable before admin", domain.OpRename, domain.NodeFacts{ID: other, Before: &domain.NodePlace{Key: "other"}, After: &domain.NodePlace{Key: "other"}}, "page.source_immutable", page},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := domain.StepFacts{Operation: tt.op, Content: &domain.ContentFacts{BeforeBytes: 2, Bytes: 3, Merged: 1}, Nodes: []domain.NodeFacts{
				{ID: page, Kind: domain.KindPage, Before: &domain.NodePlace{ParentID: &raw, InArea: true}, After: &domain.NodePlace{ParentID: &raw, InArea: true}}, tt.tail,
			}}
			v := domain.CheckContract(n, s)
			var problem *shared.Error
			if !errors.As(v.Err, &problem) || problem.Code != tt.code || v.NodeID != tt.id || v.Needs != "" {
				t.Fatalf("verdict = %+v, want %s of %s", v, tt.code, tt.id)
			}
		})
	}
}
