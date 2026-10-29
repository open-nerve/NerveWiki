package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceEntryContractCountsOnlyExistingOutsideMoves(t *testing.T) {
	for _, tt := range []struct {
		name          string
		mode          domain.Mode
		op            domain.Operation
		count         int
		before, after bool
		created       bool
		want          bool
	}{
		{"overflow", domain.ModeLLMWiki, domain.OpMove, 10001, false, true, false, true},
		{"exact", domain.ModeLLMWiki, domain.OpMove, 10000, false, true, false, false},
		{"standard", domain.ModeStandard, domain.OpMove, 10001, false, true, false, false},
		{"source out", domain.ModeLLMWiki, domain.OpMove, 10001, true, false, false, false},
		{"within source", domain.ModeLLMWiki, domain.OpMove, 10001, true, true, false, false},
		{"outside", domain.ModeLLMWiki, domain.OpMove, 10001, false, false, false, false},
		{"restore", domain.ModeLLMWiki, domain.OpRestore, 10001, false, true, true, false},
		{"import", domain.ModeLLMWiki, domain.OpCreate, 10001, false, true, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := domain.NotebookState{Mode: tt.mode, SourceRootID: uuid.NewV7()}
			facts := domain.StepFacts{Operation: tt.op, Nodes: make([]domain.NodeFacts, tt.count)}
			for i := range facts.Nodes {
				facts.Nodes[i] = domain.NodeFacts{ID: uuid.NewV7(), Kind: domain.KindPage,
					Before: &domain.NodePlace{InArea: tt.before}, After: &domain.NodePlace{ParentID: &state.SourceRootID, InArea: tt.after}}
				if tt.created {
					facts.Nodes[i].Before = nil
				}
			}
			facts.Nodes[len(facts.Nodes)-1].Kind = domain.KindAsset
			v := domain.CheckContract(state, facts)
			if errors.Is(v.Err, domain.ErrSourceTooLarge) != tt.want || tt.want && v.NodeID != facts.Nodes[0].ID {
				t.Errorf("verdict=%+v, want source_too_large=%t at first node", v, tt.want)
			}
		})
	}
}

func TestSourceEntryBatchCountsTheInstantaneousTree(t *testing.T) {
	paths := []string{"raw.md", "Other.md", "Folder.md"}
	for i := range domain.MaxUnitNodes - 1 {
		paths = append(paths, fmt.Sprintf("Folder/p%05d.md", i))
	}
	paths = append(paths, "Folder/zero.bin")
	for _, trim := range []bool{false, true} {
		t.Run(fmt.Sprint(trim), func(t *testing.T) {
			nb := newNotebook(paths...)
			state := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")}
			ops := []domain.BatchArgs{writeOp("Other.md", "changed"), moveOp("Folder.md", "raw/Folder.md")}
			if trim {
				ops[0] = deleteOp("Folder/p00000.md")
			}
			p, _ := planIn(t, nb, state, ops...)
			if trim {
				if p.Failed != nil {
					t.Fatalf("after removing one child, current 10000-node entry failed: %v", p.Failed)
				}
				return
			}
			var refused *domain.Refused
			if p.Failed == nil || p.Failed.Op != 1 || p.Failed.Tool != domain.OpKindMove || !errors.Is(p.Failed, domain.ErrSourceTooLarge) || !errors.As(p.Failed, &refused) || refused.NodeID != nb.id(t, "Folder.md") {
				t.Errorf("batch failed=%+v refusal=%+v, want ops[1] move source_too_large at Folder", p.Failed, refused)
			}
		})
	}
}

func TestSourceEntryBatchOverflowPrecedesUnseenDepth(t *testing.T) {
	paths := []string{"raw.md", "Folder/d1/d2/d3/d4/d5/d6/d7/d8/d9.md"}
	for i := range domain.MaxUnitNodes - 9 {
		paths = append(paths, fmt.Sprintf("Folder/p%05d.md", i))
	}
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			nb := newNotebook(paths...)
			state := domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: nb.id(t, "raw.md")}
			ops := []domain.BatchArgs{moveOp("Folder.md", "raw/Folder.md")}
			want := error(domain.ErrSourceTooLarge)
			if remove {
				ops = append([]domain.BatchArgs{deleteOp("Folder/p00000.md")}, ops...)
				want = domain.ErrTooDeep
			}
			p, _ := planIn(t, nb, state, ops...)
			if p.Failed == nil || p.Failed.Op != len(ops)-1 || !errors.Is(p.Failed, want) {
				t.Errorf("instantaneous size/depth refusal=%v, want %v at final move", p.Failed, want)
			}
		})
	}
}
